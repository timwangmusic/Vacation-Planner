package planner

import (
	"Vacation-planner/POI"
	"Vacation-planner/matching"
	"Vacation-planner/solution"
	"Vacation-planner/utils"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
)

type Planner interface {
	Planning(req *solution.PlanningRequest) (resp *solution.PlanningResponse)
}

type MyPlanner struct {
	Solver solution.Solver
}

func (planner *MyPlanner) Planning(req *solution.PlanningRequest) (resp *solution.PlanningResponse) {
	planningResp, err := planner.Solver.Solve(*req)
	utils.CheckErr(err)
	return &planningResp
}

func (planner *MyPlanner) Init(mapsClientApiKey string, dbUrl string, redisAddr string) {
	dbName := "VacationPlanner"
	planner.Solver.Init(mapsClientApiKey, dbName, dbUrl, redisAddr, "", 0)
}

// Generate a standard request while we seek a better way to represent complex REST requests
func GetStandardRequest() (req solution.PlanningRequest) {
	slot11 := matching.TimeSlot{POI.TimeInterval{8, 9}}
	slot12 := matching.TimeSlot{POI.TimeInterval{9, 11}}
	slot13 := matching.TimeSlot{POI.TimeInterval{11, 12}}
	stayTimes1 := []matching.TimeSlot{slot11, slot12, slot13}
	timeslot_1 := matching.TimeSlot{POI.TimeInterval{8, 12}}
	slotReq1 := solution.SlotRequest{
		Location:     "",
		TimeInterval: timeslot_1,
		EvOption:     "EVV",
		StayTimes:    stayTimes1,
	}

	slot21 := matching.TimeSlot{POI.TimeInterval{12, 13}}
	slot22 := matching.TimeSlot{POI.TimeInterval{13, 17}}
	slot23 := matching.TimeSlot{POI.TimeInterval{17, 19}}
	stayTimes2 := []matching.TimeSlot{slot21, slot22, slot23}
	timeslot2 := matching.TimeSlot{POI.TimeInterval{12, 19}}
	slotReq2 := solution.SlotRequest{
		Location:     "",
		TimeInterval: timeslot2,
		EvOption:     "EVV",
		StayTimes:    stayTimes2,
	}

	slot31 := matching.TimeSlot{POI.TimeInterval{19, 21}}
	slot32 := matching.TimeSlot{POI.TimeInterval{21, 23}}
	stayTimes3 := []matching.TimeSlot{slot31, slot32}
	timeslot3 := matching.TimeSlot{POI.TimeInterval{19, 23}}
	slotReq3 := solution.SlotRequest{
		Location:     "",
		TimeInterval: timeslot3,
		EvOption:     "EV",
		StayTimes:    stayTimes3,
	}

	req.SlotRequests = append(req.SlotRequests, []solution.SlotRequest{slotReq1, slotReq2, slotReq3}...)
	req.Weekday = POI.DATE_FRIDAY
	return
}

// API definitions
func (planner *MyPlanner) welcome_api(w http.ResponseWriter, r *http.Request) {
	_, err := fmt.Fprint(w, "Welcome to use the Vacation Planner system!")
	utils.CheckErr(err)
}

func (planner *MyPlanner) planning_api(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	location := vars["location"]
	radius := vars["radius"]

	planningReq := GetStandardRequest()
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)
	planningReq.SearchRadius = uint(searchRadius_)

	for slotReqIdx := range planningReq.SlotRequests {
		planningReq.SlotRequests[slotReqIdx].Location = location // set to the same location from URL
	}

	utils.CheckErr(json.NewEncoder(w).Encode(planner.Planning(&planningReq)))
}

func (planner *MyPlanner) HandlingRequests() {
	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.HandleFunc("/", planner.welcome_api)

	myRouter.HandleFunc("/planning/{location}/{radius}", planner.planning_api)

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}
