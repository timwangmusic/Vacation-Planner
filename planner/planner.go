package planner

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/user"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"golang.org/x/oauth2"
)

const (
	ServerTimeout      = time.Second * 15
	jobQueueBufferSize = 1000
	PhotoApiBaseURL    = "https://maps.googleapis.com/maps/api/place/photo?maxwidth=400&photo_reference=%s&key=%s"
)

var geocodes map[string]string

var placeTypeToIcon = map[POI.PlaceCategory]POI.PlaceIcon{
	POI.PlaceCategoryEatery: POI.PlaceIconEatery,
	POI.PlaceCategoryVisit:  POI.PlaceIconVisit,
}

type MyPlanner struct {
	RedisClient         *iowrappers.RedisClient
	RedisStreamName     string
	PhotoClient         iowrappers.PhotoHttpClient
	Solver              Solver
	ResultHTMLTemplate  *template.Template
	TripHTMLTemplate    *template.Template
	ProfileHTMLTemplate *template.Template
	PlanningEvents      chan iowrappers.PlanningEvent
	Environment         string
	Configs             map[string]interface{}
	OAuth2Config        *oauth2.Config
	Mailer              *iowrappers.Mailer
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
	ID     string             `json:"id"`
	Places []TimeSectionPlace `json:"places"`
}

type PlanningResponse struct {
	TravelDestination string       `json:"travel_destination"`
	TravelPlans       []TravelPlan `json:"travel_plans"`
	TripDetailsURL    []string     `json:"trip_details_url"`
	Err               error        `json:"error"`
	StatusCode        int          `json:"status_code"`
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

type TripDetailResp struct {
	LatLongs          [][2]float64
	PlaceCategories   []POI.PlaceCategory
	PlaceDetails      []PlaceDetailsResp
	ShownActive       []bool
	TravelDestination string
	TravelDate        string
	Score             float64
	ScoreOld          float64
}

type PlaceDetailsResp struct {
	Name             string
	URL              string
	FormattedAddress string
	PhotoURL         string
}

type RequestIdKey string

func (planner *MyPlanner) Init(mapsClientApiKey string, redisURL *url.URL, redisStreamName string, configs map[string]interface{}, oauthClientID string, oauthClientSecret string, domain string) {
	planner.PlanningEvents = make(chan iowrappers.PlanningEvent, jobQueueBufferSize)
	planner.RedisClient = iowrappers.CreateRedisClient(redisURL)
	planner.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		planner.RedisStreamName = "stream:planning_api_usage"
	}
	planner.PhotoClient = iowrappers.CreatePhotoHttpClient(mapsClientApiKey, PhotoApiBaseURL)

	PoiSearcher := iowrappers.CreatePoiSearcher(mapsClientApiKey, redisURL)

	planner.Solver.Init(PoiSearcher)

	planner.ResultHTMLTemplate = template.Must(template.ParseFiles("templates/search_results_layout_template.html"))
	planner.TripHTMLTemplate = template.Must(template.ParseFiles("templates/trip_plan_details_template.html"))
	planner.ProfileHTMLTemplate = template.Must(template.ParseFiles("templates/profile_page.html"))
	planner.Environment = strings.ToLower(os.Getenv("ENVIRONMENT"))
	planner.Configs = configs
	if v, exists := planner.Configs["server:google_maps:detailed_search_fields"]; exists {
		planner.Solver.Searcher.GetMapsClient().SetDetailedSearchFields(v.([]string))
	}
	var err error
	geocodes, err = planner.RedisClient.GetCities(context.Background())
	if err != nil {
		log.Errorf("failed to load city geocodes: %v", err.Error())
	}
	planner.OAuth2Config = &oauth2.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		RedirectURL:  domain + "/v1/callback-google",
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	planner.Mailer = &iowrappers.Mailer{}
	if err = planner.Mailer.Init(planner.RedisClient); err != nil {
		log.Fatalf("planner failed to create a Mailer: %s", err.Error())
	}
}

func (planner *MyPlanner) singleDayNearbySearchHandler(context *gin.Context) {
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
	places, err := NearbySearchWithPlaceView(context, planner.Solver.TimeMatcher, location, POI.Weekday(weekdayUint), uint(searchRadius_), matching.TimeSlot{Slot: POI.TimeInterval{
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

func (planner *MyPlanner) reverseGeocodingHandler(context *gin.Context) {
	latitude, _ := strconv.ParseFloat(context.Query("lat"), 64)
	longitude, _ := strconv.ParseFloat(context.Query("lng"), 64)
	result, err := planner.Solver.Searcher.GetMapsClient().ReverseGeocode(context, latitude, longitude)
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

func (planner *MyPlanner) removePlacesMigrationHandler(context *gin.Context) {
	_, authenticationErr := planner.UserAuthentication(context, user.LevelAdmin)
	if authenticationErr != nil {
		context.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	if err := planner.Solver.Searcher.RemovePlaces(context, []iowrappers.PlaceDetailsFields{iowrappers.PlaceDetailsFieldURL, iowrappers.PlaceDetailsFieldPhoto}); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func (planner *MyPlanner) placeStatsHandler(context *gin.Context) {
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

func (planner *MyPlanner) getCitiesHandler(context *gin.Context) {
	views := toCityViews(geocodes)
	term := context.DefaultQuery("term", "")
	term = strings.ToLower(term)
	iowrappers.Logger.Debugf("Reveived the prefix of %s", term)
	var results []CityView
	deduplicateMap := make(map[string]bool)
	for _, view := range views {
		viewString := toString(view)
		if _, exists := deduplicateMap[viewString]; exists {
			continue
		}
		deduplicateMap[viewString] = true
		if strings.HasPrefix(viewString, term) {
			results = append(results, view)
		}
	}
	context.JSON(http.StatusOK, gin.H{"results": results})
}

func (planner *MyPlanner) cityStatsHandler(context *gin.Context) {
	cityCount := len(geocodes)
	view := GeocodeCityView{
		Count:  cityCount,
		Cities: geocodes,
	}
	context.JSON(http.StatusOK, view)
}

func (planner *MyPlanner) Planning(ctx context.Context, planningRequest *PlanningReq, user string) (resp PlanningResponse) {
	var planningResponse PlanningResp

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
	planner.planningEventLogging(event)

	if len(planningResponse.Solutions) == 0 {
		resp.Err = errors.New("cannot find a valid solution")
		resp.StatusCode = NoValidSolution
		return
	}

	topSolutions := planningResponse.Solutions
	resp.TravelPlans = make([]TravelPlan, len(topSolutions))
	resp.TripDetailsURL = make([]string, len(topSolutions))

	for sIdx, topSolution := range topSolutions {
		travelPlan := TravelPlan{
			Places: make([]TimeSectionPlace, 0),
		}
		for pIdx, placeName := range topSolution.PlaceNames {
			travelPlan.Places = append(travelPlan.Places, TimeSectionPlace{
				PlaceName: placeName,
				StartTime: planningRequest.Slots[pIdx].TimeSlot.Slot.Start,
				EndTime:   planningRequest.Slots[pIdx].TimeSlot.Slot.End,
				Address:   topSolution.PlaceAddresses[pIdx],
				URL:       topSolution.PlaceURLs[pIdx],
				PlaceIcon: getPlaceIcon(topSolution.PlaceCategories, pIdx),
			})
		}
		travelPlan.ID = topSolution.ID
		resp.TravelPlans[sIdx] = travelPlan
		resp.TripDetailsURL[sIdx] = "/v1/plans/" + travelPlan.ID + "?" + "date=" + planningRequest.TravelDate
	}

	resp.StatusCode = ValidSolutionFound
	if len(planningRequest.Location.City) > 0 {
		c := cases.Title(language.English)
		resp.TravelDestination = c.String(planningRequest.Location.City)
	} else {
		geocodes, err := planner.Solver.Searcher.ReverseGeocode(ctx, planningRequest.Location.Latitude, planningRequest.Location.Longitude)
		if err != nil {
			resp.TravelDestination = "Dream Vacation Destination"
			return
		}
		resp.TravelDestination = geocodes.City
	}
	return
}

func (planner *MyPlanner) searchPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "search_page.html", gin.H{})
}

func (planner *MyPlanner) homePageHandler(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, "/v1/")
}

// HTTP GET API end-point handler
// Return top planning result to user
func (planner *MyPlanner) getPlanningApi(ctx *gin.Context) {
	logger := iowrappers.Logger
	var userView user.View
	if strings.ToLower(planner.Environment) == "production" {
		var authenticationErr error
		userView, authenticationErr = planner.UserAuthentication(ctx, user.LevelRegular)
		if authenticationErr != nil {
			logger.Debug(authenticationErr)
			planner.login(ctx)
			return
		}
	}

	requestId := requestid.Get(ctx)

	var err error
	var preciseLocation bool
	preciseLocation, err = strconv.ParseBool(ctx.DefaultQuery("precise", "false"))
	if err != nil {
		logger.Errorf("failed to parse Precise Location query parameter: %v", err)
	}

	location := ctx.DefaultQuery("location", "San Jose, CA, USA")
	locationFields := strings.Split(location, ", ")
	if err = validateLocation(location, preciseLocation); err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	// date is in the format of yyyy-mm-dd
	date := ctx.DefaultQuery("date", "")
	if err = validateDate(date); err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	logger.Debugf("Requested weekday is %s.", date)

	numResults := ctx.DefaultQuery("numberResults", "5")

	numResultsInt, numResultsParsingErr := strconv.ParseInt(numResults, 10, 64)
	if numResultsParsingErr != nil {
		ctx.String(http.StatusBadRequest, "number of planning results of %d is invalid", numResultsInt)
		return
	}
	logger.Debugf("[request_id: %s] The number of requested planning results is %s.", requestId, numResults)

	priceLevel := ctx.DefaultQuery("price", "2")
	logger.Debugf("Requested price range is %s", priceLevel)

	planningReq := GetStandardRequest(date, toWeekday(date), numResultsInt, toPriceLevel(priceLevel))
	planningReq.SearchRadius = 10000 // default to 10km
	planningReq.PreciseLocation = preciseLocation
	logger.Debugf("use precise location: %t", preciseLocation)
	if preciseLocation {
		planningReq.Location = POI.Location{}
		planningReq.Location.Longitude, _ = strconv.ParseFloat(locationFields[0], 64)
		planningReq.Location.Latitude, _ = strconv.ParseFloat(locationFields[1], 64)
	} else {
		switch len(locationFields) {
		case 2:
			planningReq.Location = POI.Location{City: locationFields[0], Country: locationFields[1]}
		case 3:
			planningReq.Location = POI.Location{City: locationFields[0], AdminAreaLevelOne: locationFields[1], Country: locationFields[2]}
		default:
			ctx.String(http.StatusBadRequest, "wrong location input")
			return
		}
	}

	c := context.WithValue(ctx, iowrappers.ContextRequestIdKey, requestId)
	planningResp := planner.Planning(c, &planningReq, userView.Username)

	if planningResp.Err != nil {
		if planningResp.StatusCode == InvalidRequestLocation {
			ctx.String(http.StatusBadRequest, planningResp.Err.Error())
		} else if planningResp.StatusCode == NoValidSolution {
			errString := "No valid travel solution is found.\nPlease try searching with a larger radius or a different price level."
			ctx.String(http.StatusBadRequest, errString)
		}
		return
	}

	jsonOnly := ctx.DefaultQuery("json_only", "false")
	if jsonOnly != "false" {
		ctx.JSON(http.StatusOK, planningResp.TravelPlans)
		return
	}
	utils.LogErrorWithLevel(planner.ResultHTMLTemplate.Execute(ctx.Writer, planningResp), utils.LogError)
}

func (planner *MyPlanner) getPlanDetails(c *gin.Context) {
	id := c.Param("id")
	iowrappers.Logger.Debugf("GET Route /plans/%s", id)

	var cachePlanSolution iowrappers.PlanningSolutionRecord
	var planRecordRedisKey = strings.Join([]string{iowrappers.TravelPlanRedisCacheKeyPrefix, id}, ":")
	cacheErr := planner.RedisClient.FetchSingleRecord(c, planRecordRedisKey, &cachePlanSolution)
	if cacheErr != nil {
		iowrappers.Logger.Debugf("Error occurs in fetching plan with key %s\n", planRecordRedisKey)
		c.String(http.StatusBadRequest, cacheErr.Error())
		return
	}

	const fixedPlaceKeyPrefix = "place_details:place_ID:"
	var placeKey string
	var cachePlaceDetails POI.Place
	destination := "Dream Place"
	var today = time.Now()
	if cachePlanSolution.Destination != (POI.Location{}) {
		c := cases.Title(language.English)
		destination = c.String(cachePlanSolution.Destination.City) + ", " + c.String(cachePlanSolution.Destination.Country)
	}
	travelDate := c.DefaultQuery("date", today.Format("2006-01-02")) // yyyy-mm-dd
	var tripResp = TripDetailResp{
		LatLongs:          cachePlanSolution.PlaceLocations,
		PlaceCategories:   cachePlanSolution.PlaceCategories,
		PlaceDetails:      make([]PlaceDetailsResp, 0),
		ShownActive:       make([]bool, 0),
		TravelDestination: destination,
		TravelDate:        travelDate,
		Score:             cachePlanSolution.Score,
		ScoreOld:          cachePlanSolution.ScoreOld,
	}
	for idx, placeId := range cachePlanSolution.PlaceIDs {
		placeKey = fixedPlaceKeyPrefix + placeId
		cacheErr := planner.RedisClient.FetchSingleRecord(c, placeKey, &cachePlaceDetails)
		if cacheErr != nil {
			c.String(http.StatusBadRequest, cacheErr.Error())
			return
		}

		placeDetails := planner.getTripFromPlace(cachePlaceDetails)
		tripResp.PlaceDetails = append(tripResp.PlaceDetails, placeDetails)
		var isActive = false
		if idx == 0 {
			isActive = true
		}
		tripResp.ShownActive = append(tripResp.ShownActive, isActive)
	}
	iowrappers.Logger.Debugf("Trip Details:\n%v\n", tripResp.PlaceDetails)

	jsonOnly := strings.ToLower(c.DefaultQuery("json_only", "false"))
	if jsonOnly != "false" {
		c.JSON(http.StatusOK, tripResp)
		return
	}
	// send data
	utils.LogErrorWithLevel(planner.TripHTMLTemplate.Execute(c.Writer, tripResp), utils.LogError)
}

func (planner *MyPlanner) getTripFromPlace(place POI.Place) PlaceDetailsResp {
	return PlaceDetailsResp{
		Name:             place.Name,
		URL:              place.URL,
		FormattedAddress: place.FormattedAddress,
		PhotoURL:         string(planner.PhotoClient.GetPhotoURL(place.Photo.Reference)),
	}
}

func (planner *MyPlanner) login(c *gin.Context) {
	c.HTML(http.StatusOK, "login_page.html", gin.H{})
}

func (planner *MyPlanner) signup(c *gin.Context) {
	c.HTML(http.StatusOK, "signup_page.html", gin.H{})
}

func (planner *MyPlanner) customizedTemplate(context *gin.Context) {
	context.HTML(http.StatusOK, "plan_template.html", gin.H{})
}

// travel plan customization handler
func (planner *MyPlanner) customize(ctx *gin.Context) {
	logger := iowrappers.Logger
	// date is in the format of yyyy-mm-dd
	date := ctx.DefaultQuery("date", "")
	if err := validateDate(date); err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	iowrappers.Logger.Debugf("received date in request: %s", date)

	priceLevel := ctx.DefaultQuery("price", "2")
	logger.Debugf("Requested price range is %s", priceLevel)

	request := PlanningReq{
		NumPlans:     1,
		Weekday:      toWeekday(date),
		SearchRadius: 10000,
		PriceLevel:   toPriceLevel(priceLevel),
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		iowrappers.Logger.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}

	c := context.WithValue(ctx, iowrappers.ContextRequestIdKey, requestid.Get(ctx))
	planningResp := planner.Planning(c, &request, "guest")
	ctx.JSON(http.StatusOK, planningResp)
}

func (planner *MyPlanner) handleLogin(ctx *gin.Context) {
	oauthConfig := planner.OAuth2Config
	logger := iowrappers.Logger
	URL, err := url.Parse(oauthConfig.Endpoint.AuthURL)
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Infof("Parsed authorization URL is %s", URL.String())
	parameters := url.Values{}
	parameters.Add("client_id", oauthConfig.ClientID)
	parameters.Add("scope", strings.Join(oauthConfig.Scopes, " "))
	parameters.Add("redirect_uri", oauthConfig.RedirectURL)
	parameters.Add("response_type", "code")
	// use empty string for state value
	parameters.Add("state", "")
	URL.RawQuery = parameters.Encode()

	logger.Debugf("Parameters in authorization URL is %s", URL.RawQuery)

	ctx.Redirect(http.StatusTemporaryRedirect, URL.String())
}

func (planner *MyPlanner) oauthCallback(ctx *gin.Context) {
	logger := iowrappers.Logger
	r := ctx.Request
	code := r.FormValue("code")

	if code == "" {
		logger.Warn("OAuth code not found")
		ctx.String(http.StatusBadRequest, "OAuth code not found")
		return
	} else {
		token, err := planner.OAuth2Config.Exchange(ctx, code)
		if err != nil {
			ctx.String(http.StatusBadRequest, err.Error())
			return
		}
		resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + url.QueryEscape(token.AccessToken))
		if err != nil {
			logger.Errorf("failed to get response from Google: %v", err)
			ctx.String(http.StatusBadGateway, err.Error())
			return
		}
		defer resp.Body.Close()

		response, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Errorf("failed to read response body: %v", err)
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		userCredentials := &iowrappers.GoogleOAuthResponse{}
		if err := json.Unmarshal(response, userCredentials); err != nil {
			logger.Errorf("failed to unmarshal response to JSON: %v", err)
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		u := user.Credential{Email: userCredentials.Email, WithOAuth: true}
		if planner.loginHelper(ctx, u, false) {
			ctx.Redirect(http.StatusPermanentRedirect, "/v1")
		} else {
			ctx.Redirect(http.StatusPermanentRedirect, "/v1/log-in")
		}
	}
}

func (planner *MyPlanner) userClickOnEmailVerification(ctx *gin.Context) {
	code := ctx.DefaultQuery("code", "")
	if err := planner.RedisClient.CreateUserOnEmailVerified(ctx, code); err != nil {
		ctx.String(http.StatusBadRequest, "failed to verify user email, please contact Vacation Planner")
	}
	ctx.Redirect(http.StatusMovedPermanently, "/v1/log-in")
}

func (planner *MyPlanner) SetupRouter(serverPort string) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	if planner.Environment == "debug" {
		gin.SetMode(gin.DebugMode)
	}
	gin.DefaultWriter = io.Discard

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
		v1.POST("/signup", planner.UserEmailVerify)
		v1.GET("/verify", planner.userClickOnEmailVerification)
		v1.POST("/login", planner.userLogin)
		v1.GET("/reverse-geocoding", planner.reverseGeocodingHandler)
		v1.GET("/single-day-nearby-search", planner.singleDayNearbySearchHandler)
		v1.GET("/log-in", planner.login)
		v1.GET("/sign-up", planner.signup)
		v1.GET("/plans/:id", planner.getPlanDetails)
		v1.GET("/cities", planner.getCitiesHandler)
		v1.POST("/customize", planner.customize)
		v1.GET("/template", planner.customizedTemplate)
		v1.GET("/login-google", planner.handleLogin)
		v1.GET("/callback-google", planner.oauthCallback)
		migrations := v1.Group("/migrate")
		{
			migrations.GET("/user-ratings-total", planner.UserRatingsTotalMigrationHandler)
			migrations.GET("/url", planner.UrlMigrationHandler)
			migrations.GET("/remove-places", planner.removePlacesMigrationHandler)
		}

		v1.GET("/profile", planner.profile)
		users := v1.Group("/users")
		{
			users.POST("/:username/plans", planner.userSavedPlansPostHandler)
			users.GET("/:username/plans", planner.userSavedPlansGetHandler)
			users.DELETE("/:username/plan/:id", planner.userPlanDeleteHandler)
		}
	}

	// API endpoints for collecting database statistics
	stats := myRouter.Group("/stats")
	{
		stats.GET("places", planner.placeStatsHandler)
		stats.GET("cities", planner.cityStatsHandler)
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
