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

func (p *MyPlanner) UserEmailVerify(ctx *gin.Context) {
	userView := user.View{}

	decodeErr := ctx.ShouldBindJSON(&userView)
	if decodeErr != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
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

	if err := p.Mailer.Send(ctx, iowrappers.EmailVerification, userView, string(p.Environment)); err != nil {
		iowrappers.Logger.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Email sent. Please check the inbox to verify your email address."})
}

func (p *MyPlanner) UserSignup(ctx *gin.Context) {
	userView := user.View{}

	decodeErr := ctx.ShouldBindJSON(&userView)
	if decodeErr != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
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

	view, createErr := p.RedisClient.CreateUser(ctx, userView, false)
	if createErr != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": createErr.Error()})
		return
	}
	log.Debugf("created user with ID %s", view.ID)
	ctx.JSON(http.StatusCreated, gin.H{"user creation success": view.Username})
}

func (p *MyPlanner) userLogin(ctx *gin.Context) {
	c := user.Credential{}

	decodeErr := ctx.ShouldBindJSON(&c)
	if decodeErr != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
		return
	}

	p.loginHelper(ctx, c, true)
}

func (p *MyPlanner) loginHelper(ctx *gin.Context, c user.Credential, frontEndLogin bool) (loggedIn bool) {
	logger := iowrappers.Logger

	u, token, tokenExpirationTime, loginErr := p.RedisClient.Authenticate(ctx, c)
	err := p.RedisClient.UpdateUser(ctx, &u)
	if err != nil {
		logger.Errorf("failed to update u %s: %v", u.Username, err)
	}
	if loginErr != nil {
		logger.Error(loginErr)

		if frontEndLogin {
			ctx.JSON(http.StatusUnauthorized, UserLoginResponse{
				Username: c.Username,
				Jwt:      "",
				Status:   "Unauthorized",
			})
		}
		return false
	} else {
		logger.Infof("u is logged in: %+v", u)
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:    "JWT",
		Value:   token,
		Expires: tokenExpirationTime,
	})
	return true
}

func (p *MyPlanner) UserAuthentication(ctx *gin.Context, minimumUserLevel user.Level) (user.View, error) {
	request := ctx.Request

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

	userView, findUserErr := p.RedisClient.FindUser(ctx, iowrappers.FindUserByName, user.View{Username: username})
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

func (p *MyPlanner) userSavedPlansPostHandler(ctx *gin.Context) {
	var planView user.TravelPlanView
	bindErr := ctx.ShouldBindJSON(&planView)
	if bindErr != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	userView, authErr := p.UserAuthentication(ctx, user.LevelRegular)
	if userView.Username != ctx.Param("username") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "only logged-in users can view their saved plans"})
		return
	}

	if authErr != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	// TODO: differentiate between internal plan saving errors against duplicated plan saving requests errors
	if err := p.RedisClient.SaveUserPlan(ctx, userView, &planView); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"results": "save user plan succeeded."})
}

func (p *MyPlanner) userSavedPlansGetHandler(context *gin.Context) {
	userView, authErr := p.UserAuthentication(context, user.LevelRegular)
	if userView.Username != context.Param("username") {
		context.JSON(http.StatusBadRequest, gin.H{"error": "only logged-in users can view their saved plans"})
		return
	}

	if authErr != nil {
		context.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	iowrappers.Logger.Debugf("current USER ID: %s", userView.ID)
	plans := p.RedisClient.FindUserPlans(context.Request.Context(), userView)

	sort.Sort(user.ByCreatedAt(plans))
	context.JSON(http.StatusOK, gin.H{"travel_plans": plans})
}

func (p *MyPlanner) userPlanDeleteHandler(ctx *gin.Context) {
	userView, authErr := p.UserAuthentication(ctx, user.LevelRegular)
	if userView.Username != ctx.Param("username") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "only authorized users can delete plans"})
		return
	}

	if authErr != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	err := p.RedisClient.DeleteUserPlan(ctx, userView, user.TravelPlanView{ID: ctx.Param("id")})
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

func (p *MyPlanner) userFavoritesHandler(ctx *gin.Context) {
	userView, authErr := p.UserAuthentication(ctx, user.LevelRegular)

	if authErr != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	iowrappers.Logger.Debugf("->userFavoritesHandler: user favorites %+v", *userView.Favorites)
	ctx.JSON(http.StatusOK, userView.Favorites)
}
