package planner

import (
	"context"
	"errors"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/solution"
	"github.com/weihesdlegend/Vacation-planner/user"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

const (
	ServerTimeout      = time.Second * 15
	jobQueueBufferSize = 1000
)

var placeTypeToIcon = map[POI.PlaceCategory]POI.PlaceIcon{
	POI.PlaceCategoryEatery: POI.PlaceIconEatery,
	POI.PlaceCategoryVisit:  POI.PlaceIconVisit,
}

type MyPlanner struct {
	RedisClient        iowrappers.RedisClient
	RedisStreamName    string
	Solver             solution.Solver
	ResultHTMLTemplate *template.Template
	PlanningEvents     chan iowrappers.PlanningEvent
	Environment        string
	Configs            map[string]interface{}
}

type TimeSectionPlace struct {
	PlaceName string   `json:"place_name"`
	StartTime POI.Hour `json:"start_time"`
	EndTime   POI.Hour `json:"end_time"`
	Address   string   `json:"address"`
	URL       string   `json:"url"`
	PlaceIcon string   `json:"place_icon_css_class"`
}

type TravelPlan struct {
	Places []TimeSectionPlace `json:"places"`
}

type PlanningResponse struct {
	TravelDestination string       `json:"travel_destination"`
	TravelPlans       []TravelPlan `json:"travel_plans"`
	Err               error        `json:"error"`
	StatusCode        uint         `json:"status_code"`
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

type RequestIdKey string

func (planner *MyPlanner) Init(mapsClientApiKey string, redisURL *url.URL, redisStreamName string, configs map[string]interface{}) {
	planner.PlanningEvents = make(chan iowrappers.PlanningEvent, jobQueueBufferSize)
	planner.RedisClient = iowrappers.CreateRedisClient(redisURL)
	planner.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		planner.RedisStreamName = "stream:planning_api_usage"
	}

	PoiSearcher := iowrappers.CreatePoiSearcher(mapsClientApiKey, redisURL)

	planner.Solver.Init(PoiSearcher)

	planner.ResultHTMLTemplate = template.Must(template.ParseFiles("templates/search_results_layout_template.html"))
	planner.Environment = strings.ToLower(os.Getenv("ENVIRONMENT"))
	planner.Configs = configs
	if v, exists := planner.Configs["server:google_maps:detailed_search_fields"]; exists {
		planner.Solver.Searcher.GetMapsClient().SetDetailedSearchFields(v.([]string))
	}
}

func (planner *MyPlanner) UserSavedPlansPostHandler(context *gin.Context) {
	planView := user.TravelPlanView{}
	bindErr := context.ShouldBindJSON(&planView)
	if bindErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
	}

	username, authErr := planner.UserAuthentication(context, user.LevelRegular)
	if authErr != nil {
		context.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
	}

	if err := planner.RedisClient.SaveUserPlan(context, user.View{Username: username}, planView); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (planner *MyPlanner) SingleDayNearbySearchHandler(context *gin.Context) {
	country := context.DefaultQuery("country", "USA")
	city := context.DefaultQuery("city", "San Diego")
	radius := context.DefaultQuery("radius", "10000")
	weekday := context.DefaultQuery("weekday", "5") // Saturday
	category := strings.ToLower(context.DefaultQuery("category", "visit"))

	weekdayUint, weekdayParsingErr := strconv.ParseUint(weekday, 10, 8)
	if weekdayParsingErr != nil || weekdayUint > 6 {
		context.String(http.StatusBadRequest, "invalid weekday of %d", weekdayUint)
		return
	}
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)

	var placeCategory POI.PlaceCategory
	switch category {
	case "visit":
		placeCategory = POI.PlaceCategoryVisit
	case "eatery":
		placeCategory = POI.PlaceCategoryEatery
	}

	location := POI.Location{City: city, Country: country}
	places, err := solution.NearbySearchWithPlaceView(context, planner.Solver.TimeMatcher, location, POI.Weekday(weekdayUint), uint(searchRadius_), matching.TimeSlot{Slot: POI.TimeInterval{
		Start: 8,
		End:   21,
	}}, placeCategory)
	if err != nil {
		context.JSON(http.StatusInternalServerError, "sorry please try later")
		return
	}
	context.JSON(http.StatusOK, gin.H{"places": places})
}

func (planner *MyPlanner) Destroy() {
	iowrappers.DestroyLogger()
	planner.RedisClient.Destroy()
}

func (planner *MyPlanner) ReverseGeocodingHandler(context *gin.Context) {
	latitude, _ := strconv.ParseFloat(context.Query("lat"), 64)
	longitude, _ := strconv.ParseFloat(context.Query("lng"), 64)
	result, err := planner.Solver.Searcher.GetMapsClient().ReverseGeocoding(context, latitude, longitude)
	if err != nil {
		log.Error(err)
		context.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	context.JSON(http.StatusOK, gin.H{
		"results": result,
	})
}

func (planner *MyPlanner) UserRatingsTotalMigrationHandler(context *gin.Context) {
	_, authenticationErr := planner.UserAuthentication(context, user.LevelAdmin)
	if authenticationErr != nil {
		context.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	if err := planner.Solver.Searcher.AddUserRatingsTotal(context.Request.Context()); err != nil {
		log.Error(err)
	}
}

func (planner *MyPlanner) UrlMigrationHandler(context *gin.Context) {
	_, authenticationErr := planner.UserAuthentication(context, user.LevelAdmin)
	if authenticationErr != nil {
		context.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	if err := planner.Solver.Searcher.AddUrl(context.Request.Context()); err != nil {
		log.Error(err)
	}
}

func (planner *MyPlanner) PlaceStatsHandler(context *gin.Context) {
	var placeCount int
	var err error
	if _, placeCount, err = planner.RedisClient.GetPlaceCountInRedis(context); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var eateryCount int64
	if eateryCount, err = planner.RedisClient.GetPlaceCountByCategory(context, POI.PlaceCategoryEatery); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var visitCount int64
	if visitCount, err = planner.RedisClient.GetPlaceCountByCategory(context, POI.PlaceCategoryVisit); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	context.JSON(http.StatusOK, gin.H{
		"place count":  placeCount,
		"eatery count": eateryCount,
		"visit count":  visitCount,
	})
}

type GeocodeCityView struct {
	Count  int
	Cities map[string]string
}

func (planner *MyPlanner) CityStatsHandler(context *gin.Context) {
	var cityCount int
	var err error
	var geocodes map[string]string

	if geocodes, err = planner.RedisClient.GetCityCountInRedis(context); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cityCount = len(geocodes)
	view := GeocodeCityView{
		Count:  cityCount,
		Cities: geocodes,
	}
	context.JSON(http.StatusOK, view)
}

func (planner *MyPlanner) Planning(ctx context.Context, planningRequest *solution.PlanningRequest, user string) (resp PlanningResponse) {
	var planningResponse solution.PlanningResponse

	planner.Solver.Solve(ctx, planner.RedisClient, planningRequest, &planningResponse)

	if planningResponse.Err != nil {
		resp.Err = planningResponse.Err
		resp.StatusCode = planningResponse.ErrorCode
		return
	}

	// logging planning API usage for valid requests
	event := iowrappers.PlanningEvent{
		User:      user,
		Country:   planningRequest.Location.Country,
		City:      planningRequest.Location.City,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	planner.PlanningEvents <- event
	planner.PlanningEventLogging(event)

	if len(planningResponse.Solutions) == 0 {
		resp.Err = errors.New("cannot find a valid solution")
		resp.StatusCode = solution.NoValidSolution
		return
	}

	topSolutions := planningResponse.Solutions
	resp.TravelPlans = make([]TravelPlan, len(topSolutions))

	for sIdx, topSolution := range topSolutions {
		timeSectionPlaces := TravelPlan{
			Places: make([]TimeSectionPlace, 0),
		}
		for pIdx, placeName := range topSolution.PlaceNames {
			timeSectionPlaces.Places = append(timeSectionPlaces.Places, TimeSectionPlace{
				PlaceName: placeName,
				StartTime: planningRequest.Slots[pIdx].TimeSlot.Slot.Start,
				EndTime:   planningRequest.Slots[pIdx].TimeSlot.Slot.End,
				Address:   topSolution.PlaceAddresses[pIdx],
				URL:       topSolution.PlaceURLs[pIdx],
				PlaceIcon: getPlaceIcon(topSolution.PlaceCategories, pIdx),
			})
		}
		resp.TravelPlans[sIdx] = timeSectionPlaces
	}

	resp.StatusCode = solution.ValidSolutionFound
	if len(planningRequest.Location.City) > 0 {
		resp.TravelDestination = strings.Title(planningRequest.Location.City)
	} else {
		resp.TravelDestination = "Dream Vacation Destination"
	}
	return
}

func (planner *MyPlanner) searchPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "search_page.html", gin.H{})
}

func (planner *MyPlanner) homePageHandler(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, "/v1/")
}

// validate date is in the format of yyyy-mm-dd
func validateDate(date string) error {
	if len(date) == 0 {
		return errors.New("date cannot be empty")
	}

	datePattern := `(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`
	if matched, _ := regexp.Match(datePattern, []byte(date)); !matched {
		return errors.New("date format must be yyyy-mm-dd")
	}
	return nil
}

func dateToWeekday(date string) time.Weekday {
	datePattern := regexp.MustCompile(`(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`)
	dateFields := datePattern.FindStringSubmatch(date)
	year, _ := strconv.Atoi(dateFields[1])
	month, _ := strconv.Atoi(dateFields[2])
	day, _ := strconv.Atoi(dateFields[3])
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Weekday()
}

// validate location is in the format of city,country
func validateLocation(location string) error {
	if len(location) == 0 {
		return errors.New("location cannot be empty")
	}

	locationPattern := `[a-zA-Z]+,\s[a-zA-Z]+`
	if matched, _ := regexp.Match(locationPattern, []byte(location)); !matched {
		return errors.New("location format must be city, country")
	}
	return nil
}

// HTTP GET API end-point handler
// Return top planning result to user
func (planner *MyPlanner) getPlanningApi(ctx *gin.Context) {
	var username = "guest" // default username
	if strings.ToLower(planner.Environment) == "production" {
		var authenticationErr error
		username, authenticationErr = planner.UserAuthentication(ctx, user.LevelRegular)
		if authenticationErr != nil {
			utils.LogErrorWithLevel(authenticationErr, utils.LogDebug)
			planner.login(ctx)
			return
		}
	}

	requestId := requestid.Get(ctx)
	location := ctx.DefaultQuery("location", "San Jose, USA")
	locationFields := strings.Split(location, ", ")
	if err := validateLocation(location); err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	// date is in the format of yyyy-mm-dd
	date := ctx.DefaultQuery("date", "")
	if err := validateDate(date); err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	weekday := dateToWeekday(date)
	iowrappers.Logger.Debugf("Decoded weekday is %q.", weekday)

	numResults := ctx.DefaultQuery("numberResults", "5")

	numResultsInt, numResultsParsingErr := strconv.ParseInt(numResults, 10, 64)
	if numResultsParsingErr != nil {
		ctx.String(http.StatusBadRequest, "number of planning results of %d is invalid", numResultsInt)
		return
	}
	iowrappers.Logger.Debugf("[%s] The number of requested planning results is %s.", requestId, numResults)

	planningReq := solution.GetStandardRequest(POI.Weekday(weekday), numResultsInt)
	planningReq.SearchRadius = 10000 // default to 10km
	planningReq.Location = POI.Location{City: locationFields[0], Country: locationFields[1]}

	c := context.WithValue(ctx, "request_id", requestId)
	planningResp := planner.Planning(c, &planningReq, username)

	err := planningResp.Err
	if err != nil {
		if planningResp.StatusCode == solution.InvalidRequestLocation {
			ctx.String(http.StatusBadRequest, err.Error())
		} else if planningResp.StatusCode == solution.NoValidSolution {
			errString := "No valid solution is found.\n Please try to search with larger radius."
			ctx.String(http.StatusBadRequest, errString)
		}
		return
	}

	utils.LogErrorWithLevel(planner.ResultHTMLTemplate.Execute(ctx.Writer, planningResp), utils.LogError)
}

func (planner *MyPlanner) login(c *gin.Context) {
	c.HTML(http.StatusOK, "login_page.html", gin.H{})
}

func (planner *MyPlanner) signup(c *gin.Context) {
	c.HTML(http.StatusOK, "signup_page.html", gin.H{})
}

func (planner MyPlanner) SetupRouter(serverPort string) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	if planner.Environment == "debug" {
		gin.SetMode(gin.DebugMode)
	}
	gin.DefaultWriter = ioutil.Discard

	myRouter := gin.Default()
	myRouter.LoadHTMLGlob("templates/*")
	myRouter.Static("/v1/assets", "assets")
	// trace ID
	myRouter.Use(requestid.New())

	// cors settings
	// TODO: change to front-end domain once front-end server is deployed
	myRouter.Use(cors.Default())

	myRouter.GET("/", planner.homePageHandler)

	v1 := myRouter.Group("/v1")
	{
		v1.GET("/", planner.searchPageHandler)
		v1.GET("/plans", planner.getPlanningApi)
		v1.POST("/signup", planner.UserSignup)
		v1.POST("/login", planner.UserLogin)
		v1.GET("/reverse-geocoding", planner.ReverseGeocodingHandler)
		v1.GET("/single-day-nearby-search", planner.SingleDayNearbySearchHandler)
		v1.GET("/log-in", planner.login)
		v1.GET("/sign-up", planner.signup)
		v1.POST("/users/:username/plans", planner.UserSavedPlansPostHandler)
		migrations := v1.Group("/migrate")
		{
			migrations.GET("/user-ratings-total", planner.UserRatingsTotalMigrationHandler)
			migrations.GET("/url", planner.UrlMigrationHandler)
		}
	}

	// API endpoints for collecting database statistics
	stats := myRouter.Group("/stats")
	{
		stats.GET("places", planner.PlaceStatsHandler)
		stats.GET("cities", planner.CityStatsHandler)
	}

	svr := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      myRouter,
		ReadTimeout:  ServerTimeout,
		WriteTimeout: ServerTimeout,
	}

	return svr
}

func getPlaceIcon(placeTypes []POI.PlaceCategory, pIdx int) string {
	if pIdx >= len(placeTypes) {
		return string(POI.PlaceIconEmpty)
	}
	return string(placeTypeToIcon[placeTypes[pIdx]])
}