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
	"strings"
)

type UserLoginResponse struct {
	Username string `json:"username"`
	Jwt      string `json:"jwt"`
	Status   string `json:"status"`
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
