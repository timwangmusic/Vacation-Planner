package planner

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
	"Vacation-planner/solution"
	"Vacation-planner/utils"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

type Planner interface {
	Planning(req *solution.PlanningRequest) (resp PlanningResponse)
}

type MyPlanner struct {
	RedisLogger iowrappers.RedisClient
	RedisStreamName string
	Solver solution.Solver
}

type TimeSectionPlace struct {
	PlaceName string `json:"place_name"`
	StartTime POI.Hour `json:"start_time"`
	EndTime	POI.Hour `json:"end_time"`
}

type TimeSectionPlaces struct {
	Places []TimeSectionPlace `json:"places"`
}

type PlanningResponse struct {
	Places []TimeSectionPlaces `json:"time_section_places"`
}

func (planner *MyPlanner) Planning(req *solution.PlanningRequest) (resp PlanningResponse) {
	planningResp, err := planner.Solver.Solve(*req, planner.RedisLogger)
	utils.CheckErr(err)
	if len(planningResp.Solution) == 0 {
		return
	}
	topSolution := planningResp.Solution[0]
	for idx, slotSol := range topSolution.SlotSolutions {
		timeSectionPlaces := TimeSectionPlaces{
			Places:    make([]TimeSectionPlace, 0),
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
}

// API definitions
func (planner *MyPlanner) welcome_api(w http.ResponseWriter, r *http.Request) {
	_, err := fmt.Fprint(w, "Welcome to use the Vacation Planner system!")
	utils.CheckErr(err)
}

// Return top planning result to user
func (planner *MyPlanner) planning_api(w http.ResponseWriter, r *http.Request) {
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
	tmpl := template.Must(template.ParseFiles("templates/plan_layout.html"))
	planningResp := planner.Planning(&planningReq)
	utils.CheckErr(tmpl.Execute(w, planningResp))
}

func (planner *MyPlanner) HandlingRequests() {
	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.HandleFunc("/", planner.welcome_api)


	//myRouter.HandleFunc("/planning/{location}/{radius}", planner.planning_api)
	myRouter.HandleFunc("/planning/{country}/{city}/{radius}", planner.planning_api)

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}
