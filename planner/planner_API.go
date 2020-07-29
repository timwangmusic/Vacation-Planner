package planner

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/solution"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MaxPlacesPerSlot         = 4
	MaxPlacesPerDay          = 12
	ServerTimeout            = time.Second * 15
	jobQueueBufferSize       = 1000
)

type Planner interface {
	Planning(req *solution.PlanningRequest, user string) (resp PlanningResponse)
}

type MyPlanner struct {
	RedisClient        iowrappers.RedisClient
	RedisStreamName    string
	Solver             solution.Solver
	HomeHTMLTemplate   *template.Template
	ResultHTMLTemplate *template.Template
	PlanningEvents     chan iowrappers.PlanningEvent
	Environment        string
}

type TimeSectionPlace struct {
	PlaceName string   `json:"place_name"`
	StartTime POI.Hour `json:"start_time"`
	EndTime   POI.Hour `json:"end_time"`
	Address   string   `json:"address"`
	URL       string   `json:"url"`
}

type TimeSectionPlaces struct {
	Places []TimeSectionPlace `json:"places"`
}

type PlanningResponse struct {
	TravelDestination string                `json:"travel_destination"`
	Places            [][]TimeSectionPlaces `json:"time_section_places"`
	Err               string                `json:"error"`
	StatusCode        uint                  `json:"status_code"`
}

// validate REST API input
func validateSearchRadius(searchRadius string) bool {
	searchRadiusPattern := "^[1-9][0-9]{2,5}$" // limit range to 100 -- 99999
	if matched, _ := regexp.Match(searchRadiusPattern, []byte(searchRadius)); !matched {
		return false
	}
	return true
}

type PlanningPostRequest struct {
	Country   string      `json:"country"`
	City      string      `json:"city"`
	Weekday   POI.Weekday `json:"weekday"`
	StartTime POI.Hour    `json:"start_time"`
	EndTime   POI.Hour    `json:"end_time"`
	NumVisit  uint        `json:"num_visit"`
	NumEatery uint        `json:"num_eatery"`
}

func (planner *MyPlanner) Init(mapsClientApiKey string, redisURL *url.URL, redisStreamName string) {
	planner.PlanningEvents = make(chan iowrappers.PlanningEvent, jobQueueBufferSize)
	planner.RedisClient.Init(redisURL)
	planner.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		planner.RedisStreamName = "stream:planning_api_usage"
	}

	PoiSearcher := &iowrappers.PoiSearcher{}
	PoiSearcher.Init(mapsClientApiKey, redisURL)

	planner.Solver.Init(PoiSearcher)

	planner.HomeHTMLTemplate = template.Must(template.ParseFiles("templates/index.html"))
	planner.ResultHTMLTemplate = template.Must(template.ParseFiles("templates/plan_layout.html"))
	planner.Environment = os.Getenv("ENVIRONMENT")
}

func (planner *MyPlanner) Destroy() {
	iowrappers.DestroyLogger()
	planner.RedisClient.Destroy()
}

// single-day, single-city planning method
func (planner *MyPlanner) Planning(req *solution.PlanningRequest, user string) (resp PlanningResponse) {
	planningResp, err := planner.Solver.Solve(*req, planner.RedisClient)
	utils.CheckErrImmediate(err, utils.LogError)
	if err != nil {
		resp.Err = err.Error()
		resp.StatusCode = planningResp.Errcode
		return
	}

	// logging planning API usage for valid requests
	if len(req.SlotRequests) > 0 {
		countryCity := req.SlotRequests[0].Location
		countryAndCity := strings.Split(countryCity, ",")
		event := iowrappers.PlanningEvent{
			User:      user,
			Country:   countryAndCity[1],
			City:      countryAndCity[0],
			Timestamp: time.Now().Format(time.RFC3339),
		}
		planner.PlanningEvents <- event
		planner.PlanningEventLogging(event)
	}

	if len(planningResp.Solutions) == 0 {
		resp.Err = errors.New("cannot find a valid solution").Error()
		resp.StatusCode = solution.NoValidSolution
		return
	}

	topSolutions := planningResp.Solutions
	resp.Places = make([][]TimeSectionPlaces, len(topSolutions))
	for sIdx, topSolution := range topSolutions {
		for idx, slotSol := range topSolution.SlotSolutions {
			timeSectionPlaces := TimeSectionPlaces{
				Places: make([]TimeSectionPlace, 0),
			}
			for pIdx, placeName := range slotSol.PlaceNames {
				timeSectionPlaces.Places = append(timeSectionPlaces.Places, TimeSectionPlace{
					PlaceName: placeName,
					StartTime: req.SlotRequests[idx].StayTimes[pIdx].Slot.Start,
					EndTime:   req.SlotRequests[idx].StayTimes[pIdx].Slot.End,
					Address:   slotSol.PlaceAddresses[pIdx],
					URL:       slotSol.PlaceURLs[pIdx],
				})
			}
			resp.Places[sIdx] = append(resp.Places[sIdx], timeSectionPlaces)
		}
	}

	resp.StatusCode = solution.ValidSolutionFound
	if len(req.SlotRequests) > 0 {
		resp.TravelDestination = strings.Title(strings.Split(req.SlotRequests[0].Location, ",")[0])
	} else {
		resp.TravelDestination = "Dream Vacation Destination"
	}
	return
}

// API definitions
func (planner *MyPlanner) indexPageHandler(c *gin.Context) {
	utils.CheckErrImmediate(planner.HomeHTMLTemplate.Execute(c.Writer, nil), utils.LogError)
}

// HTTP POST API end-point
func (planner *MyPlanner) postPlanningApi(w http.ResponseWriter, r *http.Request) {
	var username = "guest" // default username
	if planner.Environment == "production" {
		var authenticationErr error
		username, authenticationErr = planner.UserAuthentication(r)
		if authenticationErr != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(authenticationErr.Error())
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")

	req := PlanningPostRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	utils.CheckErrImmediate(err, utils.LogInfo)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	planningReq, err := processPlanningPostRequest(&req)
	utils.CheckErrImmediate(err, utils.LogInfo)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(err.Error())
		return
	}

	planningResp := planner.Planning(&planningReq, username)
	if planningResp.Err != "" && planningResp.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("No solution is found"))
		return
	}
	// generate valid solution
	utils.CheckErrImmediate(planner.ResultHTMLTemplate.Execute(w, planningResp), utils.LogError)
}

// HTTP GET API end-point
// Return top planning result to user
func (planner *MyPlanner) getPlanningApi(c *gin.Context) {
	var username = "guest" // default username
	if strings.ToLower(planner.Environment) == "production" {
		var authenticationErr error
		username, authenticationErr = planner.UserAuthentication(c.Request)
		if authenticationErr != nil {
			c.JSON(http.StatusUnauthorized, gin.H{ "error": authenticationErr.Error()})
			return
		}
	}

	country := c.DefaultQuery("country", "USA")
	city := c.DefaultQuery("city", "San Diego")
	radius := c.DefaultQuery("radius", "10000")
	weekday := c.DefaultQuery("weekday", "5") // Saturday
	numResults := c.DefaultQuery("numberResults", "5")

	numResultsInt, numResultsParsingErr := strconv.ParseUint(numResults, 10, 64)
	if numResultsParsingErr != nil {
		c.String(http.StatusBadRequest, "number of planning results of %d is invalid", numResultsInt)
		return
	}
	iowrappers.Logger.Debugf("number of requested planning results is %s", numResults)

	weekdayUint, weekdayParsingErr := strconv.ParseUint(weekday, 10, 8)
	if weekdayParsingErr != nil || weekdayUint < 0 || weekdayUint > 6 {
		c.String(http.StatusBadRequest, "invalid weekday %d", weekdayUint)
		return
	}

	if !validateSearchRadius(radius) {
		c.String(http.StatusBadRequest, "invalid search radius of %s", radius)
		return
	}

	cityCountry := city + "," + country

	planningReq := solution.GetStandardRequest(POI.Weekday(weekdayUint), numResultsInt)
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)
	planningReq.SearchRadius = uint(searchRadius_)

	for slotReqIdx := range planningReq.SlotRequests {
		planningReq.SlotRequests[slotReqIdx].Location = cityCountry // set to the same location from URL
	}

	planningResp := planner.Planning(&planningReq, username)

	err := planningResp.Err
	if err != "" {
		if planningResp.StatusCode == solution.InvalidRequestLocation {
			c.String(http.StatusBadRequest, err)
		} else if planningResp.StatusCode == solution.NoValidSolution {
			errString := "No valid solution is found.\n Please try to search with larger radius."
			c.String(http.StatusBadRequest, errString)
		}
		return
	}

	utils.CheckErrImmediate(planner.ResultHTMLTemplate.Execute(c.Writer, planningResp), utils.LogError)
}

func (planner MyPlanner) SetupRouter(serverPort string) *http.Server {
	myRouter := gin.Default()

	v1 := myRouter.Group("/v1")
	{
		v1.GET("", planner.indexPageHandler)
		v1.GET("/plans", planner.getPlanningApi)
		//v1.POST("", planner.postPlanningApi)
	}

	//riceBox := rice.MustFindBox("../statics/scripts")
	//jsServingPath := "/statics/scripts"
	//jsFileServer := http.StripPrefix(jsServingPath, http.FileServer(riceBox.HTTPBox()))
	//myRouter.PathPrefix(jsServingPath).Handler(jsFileServer)
	//myRouter.HandleFunc("/", planner.indexPageHandler)
	//

	//myRouter.Path("/signup").HandlerFunc(planner.UserSignup).Methods("POST")
	//myRouter.Path("/login").HandlerFunc(planner.UserLogin).Methods("POST")

	svr := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      myRouter,
		ReadTimeout:  ServerTimeout,
		WriteTimeout: ServerTimeout,
	}

	return svr
}
