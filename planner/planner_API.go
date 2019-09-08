package planner

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
	"Vacation-planner/solution"
	"Vacation-planner/utils"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

const (
	MaxPlacesPerSlot = 4
	MaxPlacesPerDay = 12
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
	Places []TimeSectionPlaces `json:"time_section_places"`
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
	utils.CheckErr(err)
	if len(planningResp.Solution) == 0 {
		return
	}
	topSolution := planningResp.Solution[0]
	for idx, slotSol := range topSolution.SlotSolutions {
		timeSectionPlaces := TimeSectionPlaces{
			Places: make([]TimeSectionPlace, 0),
		}
		for pidx, placeName := range slotSol.PlaceNames {
			timeSectionPlaces.Places = append(timeSectionPlaces.Places, TimeSectionPlace{
				PlaceName: placeName,
				StartTime: req.SlotRequests[idx].StayTimes[pidx].Slot.Start,
				EndTime:   req.SlotRequests[idx].StayTimes[pidx].Slot.End,
			})
		}
		resp.Places = append(resp.Places, timeSectionPlaces)
	}
	return
}

func (planner *MyPlanner) Init(mapsClientApiKey string, dbUrl string, redisAddr string, redisStreamName string) {
	dbName := "VacationPlanner"
	planner.Solver.Init(mapsClientApiKey, dbName, dbUrl, redisAddr, "", 0)

	planner.RedisLogger.Init(redisAddr, "", 0)
	planner.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		planner.RedisStreamName = "planning_api_usage"
	}
	planner.ResultHTMLTemplate = template.Must(template.ParseFiles("templates/plan_layout.html"))
}

// API definitions
func (planner *MyPlanner) welcomeApi(w http.ResponseWriter, r *http.Request) {
	_, err := fmt.Fprint(w, "Welcome to use the Vacation Planner system!")
	utils.CheckErr(err)
}

// HTTP POST API end-point
func (planner *MyPlanner) postPlanningApi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	req := PlanningPostRequest{}
	utils.CheckErr(json.NewDecoder(r.Body).Decode(&req))
	planningReq, err := processPlanningPostRequest(&req)
	utils.CheckErr(err)
	planningResp := planner.Planning(&planningReq)
	utils.CheckErr(planner.ResultHTMLTemplate.Execute(w, planningResp))
}

func processPlanningPostRequest(req *PlanningPostRequest) (planningRequest solution.PlanningRequest, err error) {
	planningRequest.Weekday = req.Weekday
	planningRequest.SearchRadius = 10000
	// basic POST parameter validations
	if req.StartTime == 0 || req.EndTime == 0 {
		req.StartTime = 9
		req.EndTime = 22
	}

	if req.NumVisit == 0 && req.NumEatery == 0 {
		req.NumVisit = 4
		req.NumEatery = 3
	}

	err = checkPostReqTimePlaceNum(req)
	if err != nil {
		return
	}

	return
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

	if req.NumEatery + req.NumVisit > MaxPlacesPerDay {
		err = fmt.Errorf("total number of places cannot exceed %d", MaxPlacesPerDay)
		return
	}

	if req.NumEatery + req.NumVisit > uint(req.EndTime - req.StartTime) {
		err = errors.New("not enough time for visiting all the places")
	}
	return
}

// HTTP GET API end-point
// Return top planning result to user
func (planner *MyPlanner) getPlanningApi(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	//location := vars["location"]
	country := vars["country"]
	city := vars["city"]
	radius := vars["radius"]

	// TODO: Validate user inputs
	// logging planning API usage
	planner.PlanningEventLogging(PlanningEvent{
		City:    city,
		Country: country,
	})

	cityCountry := city + "," + country

	planningReq := solution.GetStandardRequest()
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)
	planningReq.SearchRadius = uint(searchRadius_)

	for slotReqIdx := range planningReq.SlotRequests {
		planningReq.SlotRequests[slotReqIdx].Location = cityCountry // set to the same location from URL
	}
	planningResp := planner.Planning(&planningReq)
	utils.CheckErr(planner.ResultHTMLTemplate.Execute(w, planningResp))
}

func (planner *MyPlanner) HandlingRequests(serverPort string) {
	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.HandleFunc("/", planner.welcomeApi)

	myRouter.HandleFunc("/planning/{country}/{city}/{radius}", planner.getPlanningApi).Methods("GET")

	myRouter.HandleFunc("/planning/v1", planner.postPlanningApi).Methods("POST")

	log.Fatal(http.ListenAndServe(serverPort, myRouter))
}
