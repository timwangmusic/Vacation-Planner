package planner

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/user"
	"net/http"
	"os"
	"sort"
	"strings"
)

type UserLoginResponse struct {
	Username string `json:"username"`
	Jwt      string `json:"jwt"`
	Status   string `json:"status"`
}

type ProfileView struct {
	Username    string
	TravelPlans []user.TravelPlanView
}

func (planner *MyPlanner) profile(context *gin.Context) {
	userView, authErr := planner.UserAuthentication(context, user.LevelRegular)
	iowrappers.Logger.Debugf("fetching user profile for %s", userView.Username)

	if userView.Username != context.Param("username") {
		context.JSON(http.StatusBadRequest, gin.H{"error": "only logged-in users can view their saved plans"})
		return
	}

	if authErr != nil {
		context.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	userTravelPlans, err := planner.RedisClient.FindUserPlans(context, userView)
	sort.Sort(user.ByCreatedAt(userTravelPlans))
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userProfile := ProfileView{
		Username:    userView.Username,
		TravelPlans: userTravelPlans}

	if templateExecutionErr := planner.ProfileHTMLTemplate.Execute(context.Writer, userProfile); templateExecutionErr != nil {
		context.Status(http.StatusInternalServerError)
		return
	}
}

func (planner MyPlanner) UserSignup(context *gin.Context) {
	userView := user.View{}

	decodeErr := context.ShouldBindJSON(&userView)
	if decodeErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
		return
	}

	userLevel := user.LevelStringRegular
	adminUsers := strings.Split(os.Getenv("ADMIN_USERS"), ",")
	for _, username := range adminUsers {
		if userView.Username == username {
			userLevel = user.LevelStringAdmin
		}
	}

	userView.UserLevel = userLevel

	view, createErr := planner.RedisClient.CreateUser(context, userView)
	if createErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": createErr.Error()})
		return
	}
	log.Debugf("created user with ID %s", view.ID)
	context.JSON(http.StatusCreated, gin.H{"user creation success": view.Username})
}

func (planner MyPlanner) UserLogin(context *gin.Context) {
	c := user.Credential{}

	decodeErr := context.ShouldBindJSON(&c)
	if decodeErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
		return
	}

	token, tokenExpirationTime, loginErr := planner.RedisClient.Authenticate(context, c)
	if loginErr != nil {
		log.Debug(loginErr)
		context.JSON(http.StatusUnauthorized, UserLoginResponse{
			Username: c.Username,
			Jwt:      "",
			Status:   "unauthorized",
		})
		return
	}

	http.SetCookie(context.Writer, &http.Cookie{
		Name:    "JWT",
		Value:   token,
		Expires: tokenExpirationTime,
	})

	context.JSON(http.StatusOK, UserLoginResponse{
		Username: c.Username,
		Jwt:      token,
		Status:   "you are logged in",
	})
}

func (planner MyPlanner) UserAuthentication(context *gin.Context, minimumUserLevel user.Level) (user.View, error) {
	request := context.Request

	var userView user.View
	cookie, cookieErr := request.Cookie("JWT")
	if cookieErr != nil {
		return userView, cookieErr
	}

	jwtKey := []byte(os.Getenv("JWT_SIGNING_SECRET"))
	token, tokenErr := jwt.Parse(cookie.Value, func(tkn *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if tokenErr != nil {
		return userView, tokenErr
	}

	if !token.Valid {
		return userView, errors.New("invalid token")
	}

	var username string
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		username = claims["username"].(string)
	} else {
		return userView, errors.New("failed to parse JWT claims")
	}

	iowrappers.Logger.Debugf("the current logged-in user is %s", username)

	userView, findUserErr := planner.RedisClient.FindUser(context, iowrappers.FindUserByName, user.View{Username: username})
	if findUserErr != nil {
		return userView, findUserErr
	}

	var userLevel user.Level
	switch userView.UserLevel {
	case user.LevelStringRegular:
		userLevel = user.LevelRegular
	case user.LevelStringAdmin:
		userLevel = user.LevelAdmin
	}
	if userLevel < minimumUserLevel {
		log.Debugf("user level is %d, required %d", userLevel, minimumUserLevel)
		return userView, errors.New("does not meet minimum user level requirement")
	}
	return userView, nil
}

func (planner *MyPlanner) UserSavedPlansPostHandler(context *gin.Context) {
	var planView user.TravelPlanView
	bindErr := context.ShouldBindJSON(&planView)
	if bindErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	userView, authErr := planner.UserAuthentication(context, user.LevelRegular)
	if userView.Username != context.Param("username") {
		context.JSON(http.StatusBadRequest, gin.H{"error": "only logged-in users can view their saved plans"})
		return
	}

	if authErr != nil {
		context.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	// TODO: differentiate between internal plan saving errors against duplicated plan saving requests errors
	if err := planner.RedisClient.SaveUserPlan(context, userView, &planView); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	context.JSON(http.StatusOK, gin.H{"results": "save user plan succeeded."})
}

func (planner *MyPlanner) UserSavedPlansGetHandler(context *gin.Context) {
	userView, authErr := planner.UserAuthentication(context, user.LevelRegular)
	if userView.Username != context.Param("username") {
		context.JSON(http.StatusBadRequest, gin.H{"error": "only logged-in users can view their saved plans"})
		return
	}

	if authErr != nil {
		context.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	iowrappers.Logger.Debugf("current USER ID: %s", userView.ID)
	plans, err := planner.RedisClient.FindUserPlans(context.Request.Context(), userView)
	if err != nil {
		context.Status(http.StatusInternalServerError)
		iowrappers.Logger.Error(err)
		return
	}

	context.JSON(http.StatusOK, gin.H{"travel_plans": plans})
}

func (planner *MyPlanner) UserPlanDeleteHandler(context *gin.Context) {
	userView, authErr := planner.UserAuthentication(context, user.LevelRegular)
	if userView.Username != context.Param("username") {
		context.JSON(http.StatusBadRequest, gin.H{"error": "only authorized users can delete plans"})
		return
	}

	if authErr != nil {
		context.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	err := planner.RedisClient.DeleteUserPlan(context, userView, user.TravelPlanView{ID: context.Param("id")})
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}
