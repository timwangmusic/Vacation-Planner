package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/solution"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MaxPlacesPerSlot         = 4
	MaxPlacesPerDay          = 12
	MaxGetRequestsPerSecond  = 10.0 // max GET QPS
	MaxPostRequestsPerSecond = 8.0  // max POST QPS
	ServerTimeout            = time.Second * 15
	jobQueueBufferSize       = 1000
	numWorkers               = 5
)

type Planner interface {
	Planning(req *solution.PlanningRequest) (resp PlanningResponse)
}

type MyPlanner struct {
	RedisClient        iowrappers.RedisClient
	RedisStreamName    string
	Solver             solution.Solver
	ResultHTMLTemplate *template.Template
	LoginHandler       *iowrappers.DbHandler
	PlanningEvents     chan iowrappers.PlanningEvent
}

type TimeSectionPlace struct {
	PlaceName string   `json:"place_name"`
	StartTime POI.Hour `json:"start_time"`
	EndTime   POI.Hour `json:"end_time"`
	Address   string   `json:"address"`
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

func (planner *MyPlanner) Init(mapsClientApiKey string, dbUrl string, redisURL *url.URL, redisStreamName string, dbName string) {
	planner.PlanningEvents = make(chan iowrappers.PlanningEvent, jobQueueBufferSize)
	planner.RedisClient.Init(redisURL)
	planner.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		planner.RedisStreamName = "stream:planning_api_usage"
	}

	PoiSearcher := &iowrappers.PoiSearcher{}
	PoiSearcher.Init(mapsClientApiKey, dbUrl, redisURL, dbName)

	planner.LoginHandler = &iowrappers.DbHandler{}
	planner.LoginHandler.Init(dbName, dbUrl)

	planner.Solver.Init(PoiSearcher)

	planner.ResultHTMLTemplate = template.Must(template.ParseFiles("templates/plan_layout.html"))
}

func (planner *MyPlanner) Destroy() {
	iowrappers.DestroyLogger()
}

// single-day, single-city planning method
func (planner *MyPlanner) Planning(req *solution.PlanningRequest) (resp PlanningResponse) {
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
			User:      "",
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
func (planner *MyPlanner) welcomeApi(w http.ResponseWriter, r *http.Request) {
	_, authenticationErr := planner.UserAuthentication(r)
	if authenticationErr != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(authenticationErr.Error())
		return
	}
	_, err := fmt.Fprint(w, "Welcome to use the Vacation Planner system!")
	utils.CheckErrImmediate(err, utils.LogError)
}

// HTTP POST API end-point
func (planner *MyPlanner) postPlanningApi(w http.ResponseWriter, r *http.Request) {
	_, authenticationErr := planner.UserAuthentication(r)
	if authenticationErr != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(authenticationErr.Error())
		return
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

	planningResp := planner.Planning(&planningReq)
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
func (planner *MyPlanner) getPlanningApi(w http.ResponseWriter, r *http.Request) {
	_, authenticationErr := planner.UserAuthentication(r)
	if authenticationErr != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(authenticationErr.Error())
		return
	}
	vars := mux.Vars(r)
	country := vars["country"]
	city := vars["city"]
	radius := vars["radius"]
	weekday := vars["weekday"]
	numResults := vars["numberResults"]

	numResultsInt, numResultsParsingErr := strconv.ParseUint(numResults, 10, 64)
	if numResultsParsingErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid number of planning results"))
		return
	}
	iowrappers.Logger.Debugf("number of requested planning results is %s", numResults)

	weekdayUint, weekdayParsingErr := strconv.ParseUint(weekday, 10, 8)
	if weekdayParsingErr != nil || weekdayUint < 0 || weekdayUint > 6 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid weekday"))
		return
	}

	if !validateSearchRadius(radius) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid search radius"))
		return
	}

	cityCountry := city + "," + country

	planningReq := solution.GetStandardRequest(POI.Weekday(weekdayUint), numResultsInt)
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)
	planningReq.SearchRadius = uint(searchRadius_)

	for slotReqIdx := range planningReq.SlotRequests {
		planningReq.SlotRequests[slotReqIdx].Location = cityCountry // set to the same location from URL
	}

	planningResp := planner.Planning(&planningReq)

	err := planningResp.Err
	if err != "" {
		if planningResp.StatusCode == solution.InvalidRequestLocation {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err))
		} else if planningResp.StatusCode == solution.NoValidSolution {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("No valid solution is found.\n"))
			_, _ = w.Write([]byte("Please try to search with larger radius."))
		}
		return
	}

	utils.CheckErrImmediate(planner.ResultHTMLTemplate.Execute(w, planningResp), utils.LogError)
}

func (planner MyPlanner) HandlingRequests(serverPort string) {
	myRouter := mux.NewRouter().StrictSlash(true)

	getLimiter := tollbooth.NewLimiter(MaxGetRequestsPerSecond, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Second})
	getLimiter.SetMethods([]string{"GET"})
	getLimiter.SetMessage("You have reached maximum GET API limit")

	myRouter.HandleFunc("/", planner.welcomeApi).Methods("GET")

	myRouter.Path("/planning/v1").Queries("country", "{country}", "city", "{city}",
		"radius", "{radius}", "weekday", "{weekday}", "numberResults", "{numberResults}").Handler(tollbooth.LimitFuncHandler(getLimiter, planner.getPlanningApi)).Methods("GET")

	postLimiter := tollbooth.NewLimiter(MaxPostRequestsPerSecond, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Second})
	postLimiter.SetMethods([]string{"POST"})
	postLimiter.SetMessage("You have reached maximum POST API limit")

	myRouter.Handle("/planning/v1", tollbooth.LimitFuncHandler(postLimiter, planner.postPlanningApi)).Methods("POST")

	myRouter.Path("/signup").HandlerFunc(planner.UserSignup).Methods("POST")
	myRouter.Path("/login").HandlerFunc(planner.UserLogin).Methods("POST")
	myRouter.Path("/removeuser").HandlerFunc(planner.RemoveUser).Methods("POST")

	svr := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      myRouter,
		ReadTimeout:  ServerTimeout,
		WriteTimeout: ServerTimeout,
	}

	wg := &sync.WaitGroup{}
	wg.Add(numWorkers)
	// dispatch workers
	for worker := 0; worker < numWorkers; worker++ {
		go planner.ProcessPlanningEvent(worker, wg)
	}

	go func() {
		if err := svr.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// block until receiving interrupting signal
	<-c

	// closing event channel after server shuts down
	close(planner.PlanningEvents)
	wg.Wait()

	defer planner.Destroy()

	// create a deadline for other connections to complete IO
	ctx, cancel := context.WithTimeout(context.Background(), ServerTimeout)
	defer cancel()

	utils.CheckErrImmediate(svr.Shutdown(ctx), utils.LogError)

	log.Info("Server gracefully shut down")
	os.Exit(0)
}
