package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
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
	"github.com/weihesdlegend/Vacation-planner/user"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"golang.org/x/oauth2"
)

const (
	SolverTimeout      = time.Second * 10
	jobQueueBufferSize = 1000
	PhotoApiBaseURL    = "https://maps.googleapis.com/maps/api/place/photo?maxwidth=400&photo_reference=%s&key=%s"
)

type Environment string

const (
	ProductionEnvironment  Environment = "production"
	StagingEnvironment     Environment = "staging"
	TestingEnvironment     Environment = "testing"
	DevelopmentEnvironment Environment = "development"
)

var geocodes map[string]string

var placeTypeToIcon = map[POI.PlaceCategory]POI.PlaceIcon{
	POI.PlaceCategoryEatery: POI.PlaceIconEatery,
	POI.PlaceCategoryVisit:  POI.PlaceIconVisit,
}

type MyPlanner struct {
	RedisClient        *iowrappers.RedisClient
	RedisStreamName    string
	PhotoClient        iowrappers.PhotoHttpClient
	Solver             Solver
	ResultHTMLTemplate *template.Template
	TripHTMLTemplate   *template.Template
	PlanningEvents     chan iowrappers.PlanningEvent
	Environment        Environment
	Configs            map[string]interface{}
	OAuth2Config       *oauth2.Config
	Mailer             *iowrappers.Mailer
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

func (p *MyPlanner) Init(mapsClientApiKey string, redisURL *url.URL, redisStreamName string, configs map[string]interface{}, oauthClientID string, oauthClientSecret string, domain string) {
	p.PlanningEvents = make(chan iowrappers.PlanningEvent, jobQueueBufferSize)
	p.RedisClient = iowrappers.CreateRedisClient(redisURL)
	p.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		p.RedisStreamName = "stream:planning_api_usage"
	}
	p.PhotoClient = iowrappers.CreatePhotoHttpClient(mapsClientApiKey, PhotoApiBaseURL)

	PoiSearcher := iowrappers.CreatePoiSearcher(mapsClientApiKey, redisURL)

	p.Solver.Init(PoiSearcher)

	p.ResultHTMLTemplate = template.Must(template.ParseFiles("templates/search_results_layout_template.html"))
	p.TripHTMLTemplate = template.Must(template.ParseFiles("templates/trip_plan_details_template.html"))
	switch strings.ToLower(os.Getenv("ENVIRONMENT")) {
	case "production":
		p.Environment = ProductionEnvironment
	case "staging":
		p.Environment = StagingEnvironment
	case "testing":
		p.Environment = TestingEnvironment
	case "development":
		p.Environment = DevelopmentEnvironment
	}
	p.Configs = configs
	if v, exists := p.Configs["server:google_maps:detailed_search_fields"]; exists {
		p.Solver.Searcher.GetMapsClient().SetDetailedSearchFields(v.([]string))
	}
	var err error
	geocodes, err = p.RedisClient.GetCities(context.Background())
	if err != nil {
		log.Errorf("failed to load city geocodes: %v", err.Error())
	}
	p.OAuth2Config = &oauth2.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		RedirectURL:  domain + "/v1/callback-google",
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	p.Mailer = &iowrappers.Mailer{}
	if err = p.Mailer.Init(p.RedisClient); err != nil {
		log.Fatalf("p failed to create a Mailer: %s", err.Error())
	}
}

func (p *MyPlanner) Destroy() {
	iowrappers.DestroyLogger()
	p.RedisClient.Destroy()
}

func (p *MyPlanner) reverseGeocodingHandler(ctx *gin.Context) {
	latitude, _ := strconv.ParseFloat(ctx.Query("lat"), 64)
	longitude, _ := strconv.ParseFloat(ctx.Query("lng"), 64)
	result, err := p.Solver.Searcher.GetMapsClient().ReverseGeocode(ctx, latitude, longitude)
	if err != nil {
		log.Error(err)
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"results": result,
	})
}

func (p *MyPlanner) UserRatingsTotalMigrationHandler(ctx *gin.Context) {
	_, authenticationErr := p.UserAuthentication(ctx, user.LevelAdmin)
	if authenticationErr != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	if err := p.Solver.Searcher.AddUserRatingsTotal(ctx.Request.Context()); err != nil {
		log.Error(err)
	}
}

func (p *MyPlanner) UrlMigrationHandler(ctx *gin.Context) {
	_, authenticationErr := p.UserAuthentication(ctx, user.LevelAdmin)
	if authenticationErr != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	if err := p.Solver.Searcher.AddUrl(ctx.Request.Context()); err != nil {
		log.Error(err)
	}
}

func (p *MyPlanner) removePlacesMigrationHandler(ctx *gin.Context) {
	_, authenticationErr := p.UserAuthentication(ctx, user.LevelAdmin)
	if authenticationErr != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	if err := p.Solver.Searcher.RemovePlaces(ctx, []iowrappers.PlaceDetailsFields{iowrappers.PlaceDetailsFieldURL, iowrappers.PlaceDetailsFieldPhoto}); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func (p *MyPlanner) placeStatsHandler(ctx *gin.Context) {
	var placeCount int
	var err error
	if _, placeCount, err = p.RedisClient.GetPlaceCountInRedis(ctx); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var eateryCount int64
	if eateryCount, err = p.RedisClient.GetPlaceCountByCategory(ctx, POI.PlaceCategoryEatery); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var visitCount int64
	if visitCount, err = p.RedisClient.GetPlaceCountByCategory(ctx, POI.PlaceCategoryVisit); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"place count":  placeCount,
		"eatery count": eateryCount,
		"visit count":  visitCount,
	})
}

type GeocodeCityView struct {
	Count  int
	Cities map[string]string
}

func (p *MyPlanner) getCitiesHandler(ctx *gin.Context) {
	views := toCityViews(geocodes)
	term := ctx.DefaultQuery("term", "")
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
	ctx.JSON(http.StatusOK, gin.H{"results": results})
}

func (p *MyPlanner) cityStatsHandler(context *gin.Context) {
	cityCount := len(geocodes)
	view := GeocodeCityView{
		Count:  cityCount,
		Cities: geocodes,
	}
	context.JSON(http.StatusOK, view)
}

func (p *MyPlanner) Planning(ctx context.Context, planningRequest *PlanningReq, user string) (resp PlanningResponse) {
	planningResponse := p.Solver.Solve(ctx, planningRequest)

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
	p.PlanningEvents <- event
	p.planningEventLogging(event)

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
		geocodes, err := p.Solver.Searcher.ReverseGeocode(ctx, planningRequest.Location.Latitude, planningRequest.Location.Longitude)
		if err != nil {
			resp.TravelDestination = "Dream Vacation Destination"
			return
		}
		resp.TravelDestination = geocodes.City
	}
	return
}

func (p *MyPlanner) searchPageHandler(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "search_page.html", gin.H{})
}

func (p *MyPlanner) homePageHandler(ctx *gin.Context) {
	ctx.Redirect(http.StatusMovedPermanently, "/v1/")
}

// Return top planning results to user
func (p *MyPlanner) getPlanningApi(ctx *gin.Context) {
	logger := iowrappers.Logger

	var userView user.View
	var authenticationErr error
	userView, authenticationErr = p.UserAuthentication(ctx, user.LevelRegular)
	if authenticationErr != nil {
		logger.Debug(authenticationErr)
		p.login(ctx)
		return
	}

	iowrappers.Logger.Debugf("->getPlanningApi: user view: %+v", userView)
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

	numResultsInt, numResultsParsingErr := strconv.Atoi(numResults)
	if numResultsParsingErr != nil {
		ctx.String(http.StatusBadRequest, "number of planning results of %d is invalid", numResultsInt)
		return
	}
	logger.Debugf("[request_id: %s] The number of requested planning results is %s.", requestId, numResults)

	priceLevel := ctx.DefaultQuery("price", "2")
	logger.Debugf("Requested price range is %s", priceLevel)

	planningReq := standardRequest(date, toWeekday(date), numResultsInt, toPriceLevel(priceLevel))
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
	planningResp := p.Planning(c, &planningReq, userView.Username)
	if err = p.RedisClient.UpdateSearchHistory(c, location, &userView); err != nil {
		iowrappers.Logger.Debug(err)
	}

	if planningResp.Err != nil {
		if planningResp.StatusCode == InvalidRequestLocation {
			ctx.String(http.StatusBadRequest, planningResp.Err.Error())
		} else if planningResp.StatusCode == NoValidSolution {
			ctx.Redirect(http.StatusPermanentRedirect, "/v1/")
		}
		return
	}

	jsonOnly := ctx.DefaultQuery("json_only", "false")
	if jsonOnly != "false" {
		ctx.JSON(http.StatusOK, planningResp.TravelPlans)
		return
	}
	utils.LogErrorWithLevel(p.ResultHTMLTemplate.Execute(ctx.Writer, planningResp), utils.LogError)
}

func (p *MyPlanner) getPlanDetails(ctx *gin.Context) {
	id := ctx.Param("id")
	iowrappers.Logger.Debugf("GET Route /plans/%s", id)

	var cachePlanSolution iowrappers.PlanningSolutionRecord
	var planRecordRedisKey = strings.Join([]string{iowrappers.TravelPlanRedisCacheKeyPrefix, id}, ":")
	cacheErr := p.RedisClient.FetchSingleRecord(ctx, planRecordRedisKey, &cachePlanSolution)
	if cacheErr != nil {
		iowrappers.Logger.Debugf("Error occurs in fetching plan with key %s\n", planRecordRedisKey)
		ctx.String(http.StatusBadRequest, cacheErr.Error())
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
	travelDate := ctx.DefaultQuery("date", today.Format("2006-01-02")) // yyyy-mm-dd
	var tripResp = TripDetailResp{
		LatLongs:          cachePlanSolution.PlaceLocations,
		PlaceCategories:   cachePlanSolution.PlaceCategories,
		PlaceDetails:      make([]PlaceDetailsResp, len(cachePlanSolution.PlaceIDs)),
		ShownActive:       make([]bool, 0),
		TravelDestination: destination,
		TravelDate:        travelDate,
		Score:             cachePlanSolution.Score,
		ScoreOld:          cachePlanSolution.ScoreOld,
	}

	wg := sync.WaitGroup{}
	wg.Add(len(cachePlanSolution.PlaceIDs))
	for idx, placeId := range cachePlanSolution.PlaceIDs {
		placeKey = fixedPlaceKeyPrefix + placeId
		cacheErr := p.RedisClient.FetchSingleRecord(ctx, placeKey, &cachePlaceDetails)
		if cacheErr != nil {
			ctx.String(http.StatusBadRequest, cacheErr.Error())
			return
		}

		// Show the first place by default
		tripResp.ShownActive = append(tripResp.ShownActive, (idx == 0))

		// Run Goroutines to retrieve place details
		go p.asyncGetTripRespPlaceDetails(&wg, &tripResp.PlaceDetails[idx], cachePlaceDetails)
	}
	wg.Wait()
	iowrappers.Logger.Debugf("Trip Details:\n%v\n", tripResp.PlaceDetails)

	jsonOnly := strings.ToLower(ctx.DefaultQuery("json_only", "false"))
	if jsonOnly != "false" {
		ctx.JSON(http.StatusOK, tripResp)
		return
	}
	// send data
	utils.LogErrorWithLevel(p.TripHTMLTemplate.Execute(ctx.Writer, tripResp), utils.LogError)
}

func (p *MyPlanner) asyncGetTripRespPlaceDetails(wg *sync.WaitGroup, resp *PlaceDetailsResp, place POI.Place) {
	*resp = p.getTripFromPlace(place)
	wg.Done()
}

func (p *MyPlanner) getTripFromPlace(place POI.Place) PlaceDetailsResp {
	return PlaceDetailsResp{
		Name:             place.Name,
		URL:              place.URL,
		FormattedAddress: place.FormattedAddress,
		PhotoURL:         string(p.PhotoClient.GetPhotoURL(place.Photo.Reference)),
	}
}

func (p *MyPlanner) login(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "login_page.html", gin.H{})
}

func (p *MyPlanner) signup(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "signup_page.html", gin.H{})
}

func (p *MyPlanner) planTemplate(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "plan_template.html", gin.H{})
}

func (p *MyPlanner) userProfile(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "user_profile.html", gin.H{})
}

// travel plan customization handler
func (p *MyPlanner) customize(ctx *gin.Context) {
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

	pageSize, err := strconv.Atoi(ctx.DefaultQuery("size", "5"))
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	request := PlanningReq{
		NumPlans:     pageSize,
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
	planningResp := p.Planning(c, &request, "guest")
	iowrappers.Logger.Debugf("response status code is: %d", planningResp.StatusCode)
	if planningResp.StatusCode == RequestTimeOut {
		ctx.JSON(http.StatusRequestTimeout, nil)
	}
	ctx.JSON(http.StatusOK, planningResp)
}

func (p *MyPlanner) handleLogin(ctx *gin.Context) {
	oauthConfig := p.OAuth2Config
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

func (p *MyPlanner) oauthCallback(ctx *gin.Context) {
	logger := iowrappers.Logger
	r := ctx.Request
	code := r.FormValue("code")

	if code == "" {
		logger.Warn("OAuth code not found")
		ctx.String(http.StatusBadRequest, "OAuth code not found")
		return
	} else {
		token, err := p.OAuth2Config.Exchange(ctx, code)
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
		if p.loginHelper(ctx, u, false) {
			ctx.Redirect(http.StatusPermanentRedirect, "/v1")
		} else {
			ctx.Redirect(http.StatusPermanentRedirect, "/v1/log-in")
		}
	}
}

func (p *MyPlanner) userClickOnEmailVerification(ctx *gin.Context) {
	code := ctx.DefaultQuery("code", "")
	if err := p.RedisClient.CreateUserOnEmailVerified(ctx, code); err != nil {
		ctx.String(http.StatusBadRequest, "failed to verify user email, please contact Vacation Planner")
	}
	ctx.Redirect(http.StatusMovedPermanently, "/v1/log-in")
}

func (p *MyPlanner) userResetPassword(ctx *gin.Context) {
	r := &user.PasswordResetRequest{}
	if err := ctx.ShouldBindJSON(r); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	if err := p.RedisClient.SetPassword(ctx, r); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (p *MyPlanner) forgotPasswordPage(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "forgot_password_page.html", gin.H{})
}

func (p *MyPlanner) resetPasswordPage(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "reset_password_page.html", gin.H{})
}

// when users click on the reset password button this handler requests mailer to send password reset emails
func (p *MyPlanner) resetPasswordHandler(ctx *gin.Context) {
	logger := iowrappers.Logger
	email := ctx.DefaultQuery("email", "")
	if email != "" {
		logger.Infof("resetting password for view email %s", email)
	}
	var view user.View
	var err error
	if view, err = p.RedisClient.FindUser(ctx, iowrappers.FindUserByEmail, user.View{Email: email}); err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("no user is found with email %s", email)})
	}

	if err = p.Mailer.Send(ctx, iowrappers.PasswordReset, view, string(p.Environment)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// FIXME: implement this method
// uses email verification code to find user ID and update its password from request payload
func (p *MyPlanner) updateUserPassword(ctx *gin.Context) {
}

func (p *MyPlanner) SetupRouter(serverPort string) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	if p.Environment == "debug" {
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

	myRouter.GET("/", p.homePageHandler)

	v1 := myRouter.Group("/v1")
	{
		v1.GET("/", p.searchPageHandler)
		v1.GET("/plans", p.getPlanningApi)
		v1.POST("/signup", p.UserEmailVerify)
		v1.GET("/verify", p.userClickOnEmailVerification)
		v1.POST("/login", p.userLogin)
		v1.POST("/reset-password", p.userResetPassword)
		v1.GET("/reverse-geocoding", p.reverseGeocodingHandler)
		v1.GET("/log-in", p.login)
		v1.GET("/sign-up", p.signup)
		v1.GET("/plans/:id", p.getPlanDetails)
		v1.GET("/cities", p.getCitiesHandler)
		v1.POST("/customize", p.customize)
		v1.GET("/template", p.planTemplate)
		v1.GET("/login-google", p.handleLogin)
		v1.GET("/callback-google", p.oauthCallback)
		v1.GET("/forgot-password", p.forgotPasswordPage)
		v1.GET("/reset-password", p.resetPasswordPage)
		v1.GET("/send-password-reset-email", p.resetPasswordHandler)
		migrations := v1.Group("/migrate")
		{
			migrations.GET("/user-ratings-total", p.UserRatingsTotalMigrationHandler)
			migrations.GET("/url", p.UrlMigrationHandler)
			migrations.GET("/remove-places", p.removePlacesMigrationHandler)
		}

		v1.GET("/profile", p.userProfile)
		users := v1.Group("/users")
		{
			users.PUT("/:id/password", p.updateUserPassword)
			users.GET("/:username/favorites", p.userFavoritesHandler)
			users.POST("/:username/plans", p.userSavedPlansPostHandler)
			users.GET("/:username/plans", p.userSavedPlansGetHandler)
			users.DELETE("/:username/plan/:id", p.userPlanDeleteHandler)
		}
	}

	// API endpoints for collecting database statistics
	stats := myRouter.Group("/stats")
	{
		stats.GET("places", p.placeStatsHandler)
		stats.GET("cities", p.cityStatsHandler)
	}

	svr := &http.Server{
		Addr:    ":" + serverPort,
		Handler: myRouter,
	}

	return svr
}

func getPlaceIcon(placeTypes []POI.PlaceCategory, pIdx int) string {
	if pIdx >= len(placeTypes) {
		return string(POI.PlaceIconEmpty)
	}
	return string(placeTypeToIcon[placeTypes[pIdx]])
}
