package planner

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/modern-go/reflect2"
	"github.com/ulule/limiter/v3"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
	"golang.org/x/oauth2"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	gogeonames "github.com/timwangmusic/go-geonames"

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

	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
)

const (
	SolverTimeout      = time.Second * 10
	jobQueueBufferSize = 1000
	PhotoApiBaseURL    = "https://maps.googleapis.com/maps/api/place/photo?maxwidth=400&photo_reference=%s&key=%s"
	requestIdKey       = "request_id"
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
	PhotoClient        iowrappers.PhotoClient
	Solver             Solver
	ResultHTMLTemplate *template.Template
	TripHTMLTemplate   *template.Template
	PlanningEvents     chan iowrappers.PlanningEvent
	Environment        Environment
	Configs            map[string]interface{}
	OAuth2Config       *oauth2.Config
	Mailer             *iowrappers.Mailer
	GeonamesApiKey     string
	MapsClientApiKey   string
	Dispatcher         *Dispatcher
}

type TimeSectionPlace struct {
	ID        string            `json:"id"`
	PlaceName string            `json:"place_name"`
	Category  POI.PlaceCategory `json:"category"`
	StartTime POI.Hour          `json:"start_time"`
	EndTime   POI.Hour          `json:"end_time"`
	Address   string            `json:"address"`
	URL       string            `json:"url"`
	PlaceIcon string            `json:"place_icon_css_class"`
}

type TravelPlan struct {
	ID           string             `json:"id"`
	Places       []TimeSectionPlace `json:"places"`
	Saved        bool               `json:"saved"`
	PlanningSpec string             `json:"planning_spec"`
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

// TODO: deprecate Score and ScoreOld fields
type TripDetailResp struct {
	OriginalPlanID    string
	LatLongs          [][2]float64
	PlaceCategories   []POI.PlaceCategory
	PlaceDetails      []PlaceDetailsResp
	ShownActive       []bool
	TravelDestination string
	TravelDate        string
	Score             float64
	ScoreOld          float64
	ApiKey            string
}

type PlaceDetailsResp struct {
	ID               string
	Name             string
	URL              string
	FormattedAddress string
	PhotoURL         string
	Summary          string
}

type RequestIdKey string

func (p *MyPlanner) Init(mapsClientApiKey string, redisURL *url.URL, redisStreamName string, configs map[string]interface{}, oauthClientID string, oauthClientSecret string, domain string, geonamesApiKey string) {
	logger := iowrappers.Logger
	p.PlanningEvents = make(chan iowrappers.PlanningEvent, jobQueueBufferSize)
	p.RedisClient = iowrappers.CreateRedisClient(redisURL)
	p.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		p.RedisStreamName = "stream:planning_api_usage"
	}

	p.ResultHTMLTemplate = template.Must(template.ParseFiles("assets/templates/search_results_layout_template.html"))
	p.TripHTMLTemplate = template.Must(template.ParseFiles("assets/templates/trip_plan_details_template.html"))
	switch strings.ToLower(os.Getenv("ENVIRONMENT")) {
	case "production":
		p.Environment = ProductionEnvironment
	case "staging":
		p.Environment = StagingEnvironment
	case "testing":
		p.Environment = TestingEnvironment
	default:
		p.Environment = DevelopmentEnvironment
	}
	p.Configs = configs

	var err error
	// initialize photo client
	var enableMapsPhotoClient = false
	if flagEnableMapsPhotoClient, exists := p.Configs["server:plan_solver:enable_maps_photo_client"]; exists {
		enableMapsPhotoClient = flagEnableMapsPhotoClient.(bool)
		logger.Debugf("flag server:plan_solver:enableMapsPhotoClient: %v\n", enableMapsPhotoClient)
	} else {
		logger.Errorf("failed to load flag server:plan_solver:enableMapsPhotoClient!")
	}

	p.MapsClientApiKey = mapsClientApiKey
	// initialize poi searcher
	PoiSearcher := iowrappers.CreatePoiSearcher(mapsClientApiKey, redisURL)
	if v, exists := p.Configs["server:plan_solver:same_place_dedupe_count_limit"]; exists {
		if c, exists := p.Configs["server:plan_solver:nearby_cities_count_limit"]; exists {
			p.Solver.Init(PoiSearcher, v.(int), c.(int))
		}
	} else {
		logger.Fatal("failed to initialize the planner")
	}

	var placeDetailsFields []string
	if v, exists := p.Configs["server:google_maps:detailed_search_fields"]; exists {
		placeDetailsFields = v.([]string)
		p.Solver.Searcher.GetMapsClient().SetDetailedSearchFields(placeDetailsFields)
	}

	p.PhotoClient, err = iowrappers.CreatePhotoClient(mapsClientApiKey, PhotoApiBaseURL, enableMapsPhotoClient, placeDetailsFields, p.RedisClient)
	if err != nil {
		log.Fatalf("failed to initialize photo client, err:%v\n", err)
	}

	p.GeonamesApiKey = geonamesApiKey

	geocodes, err = p.RedisClient.GetCities(context.Background())
	if err != nil {
		logger.Errorf("failed to load city geocodes: %v", err.Error())
	}
	p.OAuth2Config = &oauth2.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		RedirectURL:  domain + "/v1/callback-google",
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	if p.Environment == ProductionEnvironment || p.Environment == TestingEnvironment {
		p.Mailer = &iowrappers.Mailer{}
		if err = p.Mailer.Init(p.RedisClient); err != nil {
			logger.Fatalf("p failed to create a Mailer: %s", err.Error())
		}
	}
	p.Dispatcher = NewDispatcher(&p.Solver, p.RedisClient)
	logger.Info("The planner initialization process completes")
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
	if err := p.Solver.Searcher.RemovePlaces(ctx, []iowrappers.PlaceDetailsFields{iowrappers.PlaceDetailsFieldURL, iowrappers.PlaceDetailsFieldPhoto, iowrappers.PlaceDetailsFieldUserRatingsCount}); err != nil {
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

func (p *MyPlanner) Planning(ctx context.Context, planningRequest *PlanningRequest, user string) (resp PlanningResponse) {
	logger := iowrappers.Logger
	logger.Debugf("->MyPlanner.Planning: handling planning request %+v for user: %s", *planningRequest, ctx.Value(iowrappers.ContextRequestUserId))

	primaryLocationPlanningResponse := p.Solver.Solve(ctx, planningRequest)

	if err := p.queuePlanningJobsForAllPriceLevels(planningRequest); err != nil {
		logger.Error(err)
	}

	resp = p.processPlanningResp(ctx, planningRequest, primaryLocationPlanningResponse, user)
	if !planningRequest.WithNearbyCities {
		return resp
	}

	// prioritize planning results from the primary location
	if resp.Err == nil && len(resp.TravelPlans) >= planningRequest.NumPlans {
		return resp
	}

	var err error
	lat, lng := planningRequest.Location.Latitude, planningRequest.Location.Longitude
	if !planningRequest.PreciseLocation {
		lat, lng, err = p.Solver.Searcher.Geocode(ctx, &iowrappers.GeocodeQuery{
			City:              planningRequest.Location.City,
			AdminAreaLevelOne: planningRequest.Location.AdminAreaLevelOne,
			Country:           planningRequest.Location.Country,
		})
		if err != nil {
			return PlanningResponse{Err: err}
		}
		logger.Debugf("->Planning: lat, lng from Geocode: %.4f, %.4f", lat, lng)
	}

	nearbyCityResponse, err := p.Solver.Searcher.NearbyCities(ctx,
		&iowrappers.NearbyCityRequest{
			ApiKey: p.GeonamesApiKey,
			Location: POI.Location{
				Latitude:          lat,
				Longitude:         lng,
				City:              planningRequest.Location.City,
				AdminAreaLevelOne: planningRequest.Location.AdminAreaLevelOne,
				Country:           planningRequest.Location.Country,
			},
			// convert km to m for nearbyCityResponse search query
			Radius: float64(planningRequest.SearchRadius / 1000),
			Filter: gogeonames.CityWithPopulationGreaterThan15000,
		})
	if err != nil {
		return PlanningResponse{Err: err}
	}

	// sort cities by population descending
	slices.SortFunc(nearbyCityResponse.Cities, func(a, b iowrappers.City) int { return cmp.Compare(b.Population, a.Population) })
	locations := MapSlice[iowrappers.City, POI.Location](nearbyCityResponse.Cities[:min(p.Solver.nearbyCitiesCountLimit, len(nearbyCityResponse.Cities))], toLocation)
	logger.Debugf("->Planning: found %d nearby nearbyCityResponse: %+v", len(locations), locations)

	requests, err := deepCopyAnything(planningRequest, len(locations))
	if err != nil {
		return PlanningResponse{Err: err}
	}

	for idx, req := range requests {
		req.Location = locations[idx]
	}
	nearbyCitiesPlanningResponse := p.Solver.SolveWithNearbyCities(ctx, &MultiPlanningReq{requests: requests, numPlans: planningRequest.NumPlans})
	// fall back to planning results for the primary city when nearby cities results have error
	if nearbyCitiesPlanningResponse.Err != nil {
		return resp
	}
	return p.processPlanningResp(ctx, planningRequest, primaryLocationPlanningResponse, user)
}

func (p *MyPlanner) processPlanningResp(ctx context.Context, request *PlanningRequest, resp *PlanningResp, user string) PlanningResponse {
	response := PlanningResponse{}
	if resp.Err != nil {
		response.Err = resp.Err
		response.StatusCode = resp.ErrorCode
		return response
	}

	// logging planning API usage for valid requests
	event := iowrappers.PlanningEvent{
		User:      user,
		Country:   request.Location.Country,
		City:      request.Location.City,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	p.PlanningEvents <- event
	p.planningEventLogging(event)

	if len(resp.Solutions) == 0 {
		response.Err = errors.New("cannot find a valid solution")
		response.StatusCode = NoValidSolution
		return response
	}

	s := resp.Solutions
	response.TravelPlans = make([]TravelPlan, len(s))
	response.TripDetailsURL = make([]string, len(s))

	for idx, solution := range s {
		travelPlan := TravelPlan{
			Places:       make([]TimeSectionPlace, 0),
			PlanningSpec: solution.PlanSpec,
		}
		for pIdx, placeName := range solution.PlaceNames {
			travelPlan.Places = append(travelPlan.Places, TimeSectionPlace{
				ID:        solution.PlaceIDS[pIdx],
				PlaceName: placeName,
				Category:  solution.PlaceCategories[pIdx],
				StartTime: request.Slots[pIdx].TimeSlot.Slot.Start,
				EndTime:   request.Slots[pIdx].TimeSlot.Slot.End,
				Address:   solution.PlaceAddresses[pIdx],
				URL:       solution.PlaceURLs[pIdx],
				PlaceIcon: getPlaceIcon(solution.PlaceCategories, pIdx),
			})
		}
		travelPlan.ID = solution.ID
		response.TravelPlans[idx] = travelPlan
		response.TripDetailsURL[idx] = "/v1/plans/" + travelPlan.ID + "?date=" + request.TravelDate
	}

	response.StatusCode = ValidSolutionFound
	if len(request.Location.City) > 0 {
		c := cases.Title(language.English)
		response.TravelDestination = c.String(request.Location.City)
	} else {
		geocodeResp, err := p.Solver.Searcher.ReverseGeocode(ctx, request.Location.Latitude, request.Location.Longitude)
		if err != nil {
			response.TravelDestination = "Dream Vacation Destination"
			return response
		}
		response.TravelDestination = geocodeResp.City
	}
	return response
}

func (p *MyPlanner) searchPageHandler(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "search_page.html", gin.H{})
}

func (p *MyPlanner) homePageHandler(ctx *gin.Context) {
	ctx.Redirect(http.StatusMovedPermanently, "/v1/")
}

func (p *MyPlanner) getOptimalPlan(ctx *gin.Context) {
	req := &PlanningRequest{}

	if err := ctx.ShouldBindJSON(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
	}

	if plan, err := p.Solver.SolveHungarianOptimal(ctx, req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err})
	} else {
		ctx.JSON(http.StatusOK, gin.H{"plan": plan})
	}
}

func (p *MyPlanner) SetPlanSavedStatusForUser(ctx *gin.Context, numResults int, resp PlanningResponse, uv user.View) error {
	var err error
	var resultsAvail = min(numResults, len(resp.TravelPlans))
	var userSavedPlansRedisKey = strings.Join([]string{iowrappers.UserSavedTravelPlansPrefix, "user", uv.ID, "plans"}, ":")
	savedPlanIds, err := p.RedisClient.FetchSingleRecordTypeSet(ctx, userSavedPlansRedisKey)
	if err != nil {
		return fmt.Errorf("cannot find user saved plans with key %s: %v", userSavedPlansRedisKey, err)
	}

	for idx := 0; idx < resultsAvail; idx++ {
		var targetPlanId = strings.Join([]string{"travel_plan", resp.TravelPlans[idx].ID}, ":")
		if isPlanIdSaved(targetPlanId, savedPlanIds) {
			resp.TravelPlans[idx].Saved = true
		}
	}
	return nil
}

func (p *MyPlanner) queuePlanningJobsForAllPriceLevels(req *PlanningRequest) error {
	curTime := time.Now()
	for l := range []POI.PriceLevel{POI.PriceLevelZero, POI.PriceLevelOne, POI.PriceLevelTwo, POI.PriceLevelThree, POI.PriceLevelFour} {
		newReq := *req
		newReq.PriceLevel = POI.PriceLevel(l)

		p.Dispatcher.JobQueue <- &iowrappers.Job{
			ID:          uuid.New().String(),
			Name:        "Planning",
			Description: "Compute Planning Solutions",
			Parameters:  &newReq,
			Status:      iowrappers.JobStatusNew,
			CreatedAt:   curTime,
			UpdatedAt:   curTime,
		}
	}
	return nil
}

// Return top planning results to user
func (p *MyPlanner) getPlanningApi(ctx *gin.Context) {
	logger := iowrappers.Logger
	requestId := requestid.Get(ctx)
	ctx.Set(requestIdKey, requestId)

	var userView user.View
	var authenticationErr error
	userView, authenticationErr = p.UserAuthentication(ctx, user.LevelRegular)
	if authenticationErr != nil {
		logger.Debug(authenticationErr)
		p.login(ctx)
		return
	}

	logger.Debugf("->getPlanningApi: user view: %+v", userView)

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

	searchWithNearbyCities := ctx.DefaultQuery("nearby", "false")
	var enableNearbyCities bool
	if enableNearbyCities, err = strconv.ParseBool(searchWithNearbyCities); err != nil {
		logger.Errorf("failed to parse search with nearby cities flag")
	}

	planningReq := standardRequest(date, toWeekday(date), numResultsInt, toPriceLevel(priceLevel))
	planningReq.WithNearbyCities = enableNearbyCities
	planningReq.SearchRadius = DefaultPlaceSearchRadius
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
	c = context.WithValue(c, iowrappers.ContextRequestUserId, userView.ID)
	planningResp := p.Planning(c, &planningReq, userView.Username)
	if err = p.RedisClient.UpdateSearchHistory(c, location, &userView); err != nil {
		logger.Debug(err)
	}

	if planningResp.Err != nil {
		if planningResp.StatusCode == InvalidRequestLocation {
			ctx.String(http.StatusBadRequest, planningResp.Err.Error())
		} else if planningResp.StatusCode == NoValidSolution {
			ctx.Redirect(http.StatusPermanentRedirect, "/v1")
		} else if planningResp.StatusCode == InternalError {
			ctx.Redirect(http.StatusPermanentRedirect, "/v1")
		}
		return
	}
	if err = p.SetPlanSavedStatusForUser(ctx, numResultsInt, planningResp, userView); err != nil {
		logger.Error(err)
	}

	jsonOnly := ctx.DefaultQuery("json_only", "false")
	if jsonOnly != "false" {
		ctx.JSON(http.StatusOK, planningResp.TravelPlans)
		return
	}
	utils.LogErrorWithLevel(p.ResultHTMLTemplate.Execute(ctx.Writer, planningResp), utils.LogError)
}

func (p *MyPlanner) getUserSavedPlanDetails(ctx *gin.Context) {
	logger := iowrappers.Logger
	if reflect2.IsNil(logger) {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	view, err := p.UserAuthentication(ctx, user.LevelRegular)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if reflect2.IsNil(view) {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	planId := ctx.Param("id")
	logger.Debugf("looking for plan %s saved by user %s", planId, view.ID)

	planDetailsRedisKey := strings.Join([]string{iowrappers.UserSavedTravelPlanPrefix, "user", view.ID, "plan", planId}, ":")
	planDetails := &user.TravelPlanView{}
	cacheErr := p.RedisClient.FetchSingleRecord(ctx, planDetailsRedisKey, planDetails)
	if cacheErr != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("plan %s does not exist or is not owned by %s", planId, view.Username)})
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(planDetails.Places))
	placesCount := len(planDetails.Places)

	resp := TripDetailResp{
		OriginalPlanID:    planDetails.OriginalPlanID,
		LatLongs:          make([][2]float64, placesCount),
		PlaceCategories:   make([]POI.PlaceCategory, placesCount),
		PlaceDetails:      make([]PlaceDetailsResp, placesCount),
		ShownActive:       make([]bool, placesCount),
		TravelDestination: planDetails.Destination,
		TravelDate:        planDetails.TravelDate,
	}

	const fixedPlaceKeyPrefix = "place_details:place_ID:"
	for idx, view := range planDetails.Places {
		go func(i int, v user.TravelPlaceView) {
			defer wg.Done()
			placeRedisKey := fixedPlaceKeyPrefix + v.ID
			var place POI.Place
			if err := p.RedisClient.FetchSingleRecord(ctx, placeRedisKey, &place); err != nil {
				logger.Error(err)
				return
			}
			resp.LatLongs[i] = [2]float64{place.Location.Latitude, place.Location.Longitude}
			resp.ShownActive[i] = i == 0
			resp.PlaceCategories[i] = POI.GetPlaceCategory(place.LocationType)

			details, err := p.placeDetailsResp(ctx, place)
			if err != nil {
				logger.Error(err)
				return
			}
			resp.PlaceDetails[i] = details
		}(idx, view)
	}

	wg.Wait()

	resp.ApiKey = p.MapsClientApiKey
	jsonOnly, _ := strconv.ParseBool(strings.ToLower(ctx.DefaultQuery("json_only", "false")))
	if jsonOnly {
		ctx.JSON(http.StatusOK, resp)
		return
	}

	utils.LogErrorWithLevel(p.TripHTMLTemplate.Execute(ctx.Writer, resp), utils.LogError)
}

func (p *MyPlanner) getPlanDetails(ctx *gin.Context) {
	logger := iowrappers.Logger
	id := ctx.Param("id")

	var record iowrappers.PlanningSolutionRecord
	var planRecordRedisKey = strings.Join([]string{iowrappers.TravelPlanRedisCacheKeyPrefix, id}, ":")
	cacheErr := p.RedisClient.FetchSingleRecord(ctx, planRecordRedisKey, &record)
	if cacheErr != nil {
		logger.Errorf("Error while fetching plan with key %s: %v", planRecordRedisKey, cacheErr)
		ctx.String(http.StatusInternalServerError, cacheErr.Error())
		return
	}

	const fixedPlaceKeyPrefix = "place_details:place_ID:"
	var placeKey string
	destination := "Dream Place"
	var today = time.Now()
	if record.Destination != (POI.Location{}) {
		c := cases.Title(language.English)
		destination = c.String(record.Destination.City) + ", " + c.String(record.Destination.Country)
	}
	travelDate := ctx.DefaultQuery("date", today.Format("2006-01-02")) // yyyy-mm-dd
	var tripResp = TripDetailResp{
		OriginalPlanID:    record.ID,
		LatLongs:          record.PlaceLocations,
		PlaceCategories:   record.PlaceCategories,
		PlaceDetails:      make([]PlaceDetailsResp, len(record.PlaceIDs)),
		ShownActive:       make([]bool, 0),
		TravelDestination: destination,
		TravelDate:        travelDate,
		Score:             record.Score,
		ScoreOld:          record.ScoreOld,
		ApiKey:            p.MapsClientApiKey,
	}

	wg := sync.WaitGroup{}
	wg.Add(len(record.PlaceIDs))
	for idx, placeId := range record.PlaceIDs {
		placeKey = fixedPlaceKeyPrefix + placeId
		var place POI.Place
		cacheErr = p.RedisClient.FetchSingleRecord(ctx, placeKey, &place)
		if cacheErr != nil {
			logger.Error(cacheErr)
		}

		// Show the first place by default
		tripResp.ShownActive = append(tripResp.ShownActive, idx == 0)

		go p.asyncGetTripRespPlaceDetails(ctx, &wg, &tripResp.PlaceDetails[idx], place)
	}
	wg.Wait()

	jsonOnly, _ := strconv.ParseBool(strings.ToLower(ctx.DefaultQuery("json_only", "false")))
	if jsonOnly {
		ctx.JSON(http.StatusOK, tripResp)
		return
	}

	logger.Debugf("lat/lng are: %+v", tripResp.LatLongs)
	utils.LogErrorWithLevel(p.TripHTMLTemplate.Execute(ctx.Writer, tripResp), utils.LogError)
}

func (p *MyPlanner) asyncGetTripRespPlaceDetails(ctx context.Context, wg *sync.WaitGroup, resp *PlaceDetailsResp, place POI.Place) {
	var err error
	*resp, err = p.placeDetailsResp(ctx, place)
	if err != nil {
		iowrappers.Logger.Error(err)
	}
	wg.Done()
}

func (p *MyPlanner) placeDetailsResp(ctx context.Context, place POI.Place) (PlaceDetailsResp, error) {
	photoURL, err := p.PhotoClient.GetPhotoURL(ctx, place.Photo.Reference, place.GetID())
	return PlaceDetailsResp{
		ID:               place.GetID(),
		Name:             place.GetName(),
		URL:              place.GetURL(),
		FormattedAddress: place.GetFormattedAddress(),
		PhotoURL:         string(photoURL),
		Summary:          place.GetSummary(),
	}, err
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
	logger.Debugf("received date in request: %s", date)

	priceLevel := ctx.DefaultQuery("price", "2")
	logger.Debugf("Requested price range is %s", priceLevel)

	pageSize, err := strconv.Atoi(ctx.DefaultQuery("size", "5"))
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	request := &PlanningRequest{
		NumPlans:     pageSize,
		SearchRadius: 10000,
		PriceLevel:   toPriceLevel(priceLevel),
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		logger.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}

	weekday := toWeekday(date)
	for _, slot := range request.Slots {
		slot.Weekday = weekday
	}

	c := context.WithValue(ctx, iowrappers.ContextRequestIdKey, requestid.Get(ctx))
	planningResp := p.Planning(c, request, "guest")
	logger.Debugf("response status code is: %d", planningResp.StatusCode)
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

	if verificationErr := p.RedisClient.VerifyPasswordResetRequest(ctx, r); verificationErr.HttpStatus != http.StatusOK {
		ctx.JSON(verificationErr.HttpStatus, gin.H{"error": verificationErr.Message.Error()})
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

func (p *MyPlanner) fourZeroFourPage(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "404.html", gin.H{})
}

func (p *MyPlanner) aboutPage(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "about.html", gin.H{})
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

	if err = p.Mailer.Send(ctx, iowrappers.PasswordReset, view, strings.ToLower(string(p.Environment))); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (p *MyPlanner) getNearbyCities(ctx *gin.Context) {
	req := &iowrappers.NearbyCityRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
	resp, err := p.Solver.Searcher.NearbyCities(ctx, req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	ctx.JSON(http.StatusOK, gin.H{"cities": resp.Cities})
}

func (p *MyPlanner) GetPlaceDetails(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, "place ID is missing")
	}

	var place POI.Place
	requestId := requestid.Get(ctx)
	ctx_ := context.WithValue(ctx, iowrappers.ContextRequestIdKey, requestId)
	err := p.RedisClient.FetchSingleRecord(ctx_, iowrappers.PlaceDetailsRedisKeyPrefix+id, &place)
	if err != nil {
		if strings.Contains(err.Error(), "redis server find no result for key") {
			ctx.JSON(http.StatusBadRequest, "the requested place does not exist in the system")
			return
		}
		ctx.JSON(http.StatusInternalServerError, "failed to find place details")
	}
	ctx.JSON(http.StatusOK, place)
}

func (p *MyPlanner) rateLimiter() gin.HandlerFunc {
	logger := iowrappers.Logger
	// 100 requests per hour
	rate, err := limiter.NewRateFromFormatted("100-H")
	if err != nil {
		logger.Fatal(err)
	}

	store, err := sredis.NewStore(p.RedisClient.Get())
	if err != nil {
		logger.Fatal(err)
	}

	return mgin.NewMiddleware(limiter.New(store, rate))
}

func (p *MyPlanner) SetupRouter(serverPort string) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	if p.Environment == "debug" {
		gin.SetMode(gin.DebugMode)
	}
	gin.DefaultWriter = io.Discard

	myRouter := gin.Default()
	myRouter.Use(func(c *gin.Context) {
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Next()
	})
	myRouter.LoadHTMLGlob("assets/templates/*")
	myRouter.Static("/v1/assets", "assets")
	// trace ID
	myRouter.Use(requestid.New())

	middleware := p.rateLimiter()
	// cors settings
	// TODO: change to front-end domain once front-end server is deployed
	myRouter.ForwardedByClientIP = true
	myRouter.Use(cors.Default())
	myRouter.Use(middleware)

	myRouter.GET("/", p.homePageHandler)

	v1 := myRouter.Group("/v1")
	{
		v1.GET("/", p.searchPageHandler)
		v1.GET("/plans", p.getPlanningApi)
		v1.POST("/signup", p.UserEmailVerify)
		v1.GET("/verify", p.userClickOnEmailVerification)
		v1.POST("/login", p.userLogin)
		v1.PUT("/reset-password-backend", p.userResetPassword)
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
		v1.GET("/404", p.fourZeroFourPage)
		v1.GET("/about", p.aboutPage)
		v1.GET("/send-password-reset-email", p.resetPasswordHandler)
		v1.POST("/nearby-cities", p.getNearbyCities)
		v1.POST("/optimal-plan", p.getOptimalPlan)
		migrations := v1.Group("/migrate")
		{
			migrations.GET("/user-ratings-total", p.UserRatingsTotalMigrationHandler)
			migrations.GET("/url", p.UrlMigrationHandler)
			migrations.GET("/remove-places", p.removePlacesMigrationHandler)
		}

		v1.GET("/profile", p.userProfile)
		users := v1.Group("/users")
		{
			users.GET("/:username/favorites", p.userFavoritesHandler)
			users.POST("/:username/plans", p.userSavedPlansPostHandler)
			users.GET("/:username/plans", p.userSavedPlansGetHandler)
			users.DELETE("/:username/plan/:id", p.userPlanDeleteHandler)
			users.GET("/plan/:id", p.getUserSavedPlanDetails)
			users.POST("/:username/feedback", p.userFeedbackHandler)
		}

		places := v1.Group("/places")
		{
			places.GET("/:id", p.GetPlaceDetails)
		}

		admins := v1.Group("/admins")
		{
			admins.POST("/announce", p.announce)
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

func isPlanIdSaved(planID string, userSavedPlans []string) bool {
	for _, savePlaceID := range userSavedPlans {
		if savePlaceID == planID {
			return true
		}
	}
	return false
}
