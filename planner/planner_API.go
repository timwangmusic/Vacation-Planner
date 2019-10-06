package planner

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
	"Vacation-planner/solution"
	"Vacation-planner/utils"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"
)

type Planner interface {
	Planning(req *solution.PlanningRequest) (resp PlanningResponse)
}

type MyPlanner struct {
	RedisLogger     iowrappers.RedisClient
	RedisStreamName string
	Solver          solution.Solver
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
	Places     []TimeSectionPlaces `json:"time_section_places"`
	Err        string              `json:"error"`
	StatusCode uint                `json:"status_code"`
}

// validate REST API input
func validateSearchRadius(searchRadius string) bool {
	searchRadiusPattern := "^[1-9][0-9]{2,5}$" // limit range to 100 -- 99999
	if matched, _ := regexp.Match(searchRadiusPattern, []byte(searchRadius)); !matched {
		return false
	}
	return true
}

func (planner *MyPlanner) Planning(req *solution.PlanningRequest) (resp PlanningResponse) {
	planningResp, err := planner.Solver.Solve(*req, planner.RedisLogger)
	utils.CheckErrImmediate(err, utils.LogError)
	if err != nil {
		resp.Err = err.Error()
		resp.StatusCode = planningResp.Errcode
		return
	}

	if len(planningResp.Solution) == 0 {
		resp.Err = errors.New("cannot find a solution").Error()
		resp.StatusCode = solution.NoValidSolution
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
	resp.StatusCode = solution.ValidSolutionFound
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
func (planner *MyPlanner) welcomeApi(w http.ResponseWriter, r *http.Request) {
	_, err := fmt.Fprint(w, "Welcome to use the Vacation Planner system!")
	utils.CheckErrImmediate(err, utils.LogError)
}

// Return top planning result to user
func (planner *MyPlanner) planningApi(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	country := vars["country"]
	city := vars["city"]
	radius := vars["radius"]

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

	planningReq := solution.GetStandardRequest()
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)
	planningReq.SearchRadius = uint(searchRadius_)

	for slotReqIdx := range planningReq.SlotRequests {
		planningReq.SlotRequests[slotReqIdx].Location = cityCountry // set to the same location from URL
	}
	tmpl := template.Must(template.ParseFiles("templates/plan_layout.html"))

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

	utils.CheckErrImmediate(tmpl.Execute(w, planningResp), utils.LogError)
}

func (planner *MyPlanner) HandlingRequests() {
	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.HandleFunc("/", planner.welcomeApi)

	//myRouter.HandleFunc("/planning/{location}/{radius}", planner.planning_api)
	myRouter.HandleFunc("/planning/{country}/{city}/{radius}", planner.planningApi)

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}
