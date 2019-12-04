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
	"github.com/weihesdlegend/Vacation-planner/matching"
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
	"time"
)

const (
	MaxPlacesPerSlot         = 4
	MaxPlacesPerDay          = 12
	MaxGetRequestsPerSecond  = 10.0 // max GET QPS
	MaxPostRequestsPerSecond = 8.0  // max POST QPS
	ServerTimeout            = time.Second * 15
)

type Planner interface {
	Planning(req *solution.PlanningRequest) (resp PlanningResponse)
}

type MyPlanner struct {
	RedisLogger        iowrappers.RedisClient
	RedisStreamName    string
	Solver             solution.Solver
	ResultHTMLTemplate *template.Template
}

type TimeSectionPlace struct {
	PlaceName string   `json:"place_name"`
	StartTime POI.Hour `json:"start_time"`
	EndTime   POI.Hour `json:"end_time"`
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

// single-day, single-city planning method
func (planner *MyPlanner) Planning(req *solution.PlanningRequest) (resp PlanningResponse) {
	planningResp, err := planner.Solver.Solve(*req, planner.RedisLogger)
	utils.CheckErrImmediate(err, utils.LogInfo)
	if err != nil {
		resp.Err = err.Error()
		resp.StatusCode = planningResp.Errcode
		return
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

func (planner *MyPlanner) Init(mapsClientApiKey string, dbName string, dbUrl string, redisUrl *url.URL, redisStreamName string) {
	planner.Solver.Init(mapsClientApiKey, dbName, dbUrl, redisUrl)

	planner.RedisLogger.Init(redisUrl)
	planner.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		planner.RedisStreamName = "stream:planning_api_usage"
	}
	planner.ResultHTMLTemplate = template.Must(template.ParseFiles("templates/plan_layout.html"))
}

// API definitions
func (planner *MyPlanner) welcomeApi(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprint(w, "Welcome to use the Vacation Planner system!")
	utils.CheckErrImmediate(err, utils.LogError)
}

// HTTP POST API end-point
func (planner *MyPlanner) postPlanningApi(w http.ResponseWriter, r *http.Request) {
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

func processPlanningPostRequest(req *PlanningPostRequest) (planningRequest solution.PlanningRequest, err error) {
	if req.Weekday > POI.DATE_SUNDAY || req.Weekday < POI.DATE_MONDAY {
		err = errors.New("invalid weekday in the request")
		return
	}

	planningRequest.Weekday = req.Weekday
	planningRequest.SearchRadius = 10000
	// basic POST parameter validations
	if req.StartTime == 0 || req.EndTime == 0 {
		req.StartTime = 9
		req.EndTime = 22
	}

	if req.NumEatery == 0 {
		req.NumEatery = 1
	}

	if req.NumVisit == 0 {
		req.NumVisit = 2
	}

	err = checkPostReqTimePlaceNum(req)
	if err != nil {
		return
	}

	planningRequest.SlotRequests = GenSlotRequests(*req)
	return
}

func GenSlotRequests(req PlanningPostRequest) []solution.SlotRequest {
	// grouping
	numGroups := uint(1)
	numVisit, numEatery := req.NumVisit, req.NumEatery
	if req.NumEatery > req.NumVisit {
		numGroups = req.NumEatery
	} else {
		numGroups = req.NumVisit
	}
	groups := make([][]string, numGroups)
	for idx := range groups {
		groups[idx] = make([]string, 0)
	}
	// construct groups, make sure eatery appear before visit locations
	// depends on the location type ratio, some groups might only has 1 location
	if req.NumVisit > req.NumEatery {
		ratio := int(req.NumVisit / req.NumEatery)
		for idx := range groups {
			groups[idx] = append(groups[idx], "V")
			if idx%ratio == 0 && numEatery > 0 {
				groups[idx] = append([]string{"E"}, groups[idx]...)
				numEatery--
			}
		}
	} else {
		ratio := int(req.NumEatery / req.NumVisit)
		for idx := range groups {
			groups[idx] = append(groups[idx], "E")
			if idx%ratio == 0 && numVisit > 0 {
				groups[idx] = append(groups[idx], "V")
				numVisit--
			}
		}
	}

	// time allocation
	numHours := int(req.EndTime - req.StartTime)
	hours := make([]int, numGroups)

	for idx := range hours {
		hours[idx] = len(groups[idx])
		numHours -= hours[idx]
	}

	groupIdx := 0
	for numHours > 0 {
		hours[groupIdx] += 1
		groupIdx++
		numHours--
		if groupIdx == len(groups) {
			groupIdx = 0
		}
	}

	slotRequests := make([]solution.SlotRequest, numGroups)
	cityCountry := req.City + "," + req.Country

	curTime := req.StartTime

	for groupIdx := range slotRequests {
		slotRequests[groupIdx].Location = cityCountry
		slotRequests[groupIdx].EvOption = strings.Join(groups[groupIdx], "")
		slotRequests[groupIdx].StayTimes = make([]matching.TimeSlot, len(groups[groupIdx]))
		allocatedTime := hours[groupIdx]
		for placeIdx, placeType := range groups[groupIdx] {
			curSlot := matching.TimeSlot{}
			if placeType == "E" {
				curSlot.Slot.Start = curTime
				curSlot.Slot.End = curTime + 1
				allocatedTime -= 1
				curTime += 1
			} else {
				curSlot.Slot.Start = curTime
				curTime += POI.Hour(allocatedTime)
				curSlot.Slot.End = curTime
			}
			slotRequests[groupIdx].StayTimes[placeIdx] = curSlot
		}
	}

	groupIdx = 0
	curGroupIdx := 1
	excludedGroupIndexes := make(map[int]bool)

	// combine groups and limit maximum number of groups
	for numGroups > 3 && curGroupIdx < len(slotRequests) {
		if len(slotRequests[groupIdx].EvOption)+len(slotRequests[curGroupIdx].EvOption) <= MaxPlacesPerSlot {
			slotRequests[groupIdx].EvOption = slotRequests[groupIdx].EvOption + slotRequests[curGroupIdx].EvOption
			slotRequests[groupIdx].StayTimes = append(slotRequests[groupIdx].StayTimes, slotRequests[curGroupIdx].StayTimes...)
			excludedGroupIndexes[curGroupIdx] = true
			curGroupIdx++
			numGroups--
		} else {
			groupIdx = curGroupIdx
			curGroupIdx++
		}
	}

	finalRes := make([]solution.SlotRequest, 0)
	for idx, slotReq := range slotRequests {
		if _, exist := excludedGroupIndexes[idx]; !exist {
			finalRes = append(finalRes, slotReq)
		}
	}
	return finalRes
}

func checkPostReqTimePlaceNum(req *PlanningPostRequest) (err error) {
	if req.StartTime > 24 || req.EndTime > 24 {
		err = errors.New("invalid time, valid times are chosen from 1-24")
		return
	}
	if req.StartTime >= req.EndTime {
		err = errors.New("start time cannot be later than end time")
		return
	}

	if req.NumEatery+req.NumVisit > MaxPlacesPerDay {
		err = fmt.Errorf("total number of places cannot exceed %d", MaxPlacesPerDay)
		return
	}

	if req.NumEatery+req.NumVisit > uint(req.EndTime-req.StartTime) {
		err = errors.New("not enough time for visiting all the places")
	}
	return
}

// HTTP GET API end-point
// Return top planning result to user
func (planner *MyPlanner) planningApi(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	country := vars["country"]
	city := vars["city"]
	radius := vars["radius"]
	weekday := vars["weekday"]

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

	// logging planning API usage
	planner.PlanningEventLogging(PlanningEvent{
		City:    city,
		Country: country,
	})

	cityCountry := city + "," + country

	planningReq := solution.GetStandardRequest(POI.Weekday(weekdayUint))
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
			_, _ = w.Write([]byte("No valid solution is found"))
		}
		return
	}

	utils.CheckErrImmediate(planner.ResultHTMLTemplate.Execute(w, planningResp), utils.LogError)
}

func (planner *MyPlanner) HandlingRequests(serverPort string) {
	myRouter := mux.NewRouter().StrictSlash(true)

	getLimiter := tollbooth.NewLimiter(MaxGetRequestsPerSecond, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Second})
	getLimiter.SetMethods([]string{"GET"})
	getLimiter.SetMessage("You have reached maximum GET API limit")

	myRouter.HandleFunc("/", planner.welcomeApi)

	myRouter.Path("/planning/v1").Queries("country", "{country}", "city", "{city}",
		"radius", "{radius}", "weekday", "{weekday}").Handler(tollbooth.LimitFuncHandler(getLimiter, planner.planningApi)).Methods("GET")

	postLimiter := tollbooth.NewLimiter(MaxPostRequestsPerSecond, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Second})
	postLimiter.SetMethods([]string{"POST"})
	postLimiter.SetMessage("You have reached maximum POST API limit")

	myRouter.Handle("/planning/v1", tollbooth.LimitFuncHandler(postLimiter, planner.postPlanningApi)).Methods("POST")

	svr := &http.Server{
		Addr:         serverPort,
		Handler:      myRouter,
		ReadTimeout:  ServerTimeout,
		WriteTimeout: ServerTimeout,
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

	// create a deadline for other connections to complete IO
	ctx, cancel := context.WithTimeout(context.Background(), ServerTimeout)
	defer cancel()

	utils.CheckErrImmediate(svr.Shutdown(ctx), utils.LogError)

	log.Info("Server gracefully shut down")
	os.Exit(0)
}
