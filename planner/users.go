package planner

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/user"
)

type UserLoginResponse struct {
	Email    string `json:"email"`
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

	// only verifies user emails in test and production environments, consider add staging environment later
	if p.Environment == ProductionEnvironment || p.Environment == TestingEnvironment {
		if err := p.Mailer.Send(ctx, iowrappers.EmailVerification, userView, strings.ToLower(string(p.Environment))); err != nil {
			iowrappers.Logger.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"message": "Email sent. Please check the inbox to verify your email address."})
		return
	}

	createdUser, err := p.RedisClient.CreateUser(ctx, userView, false)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Created user %s with ID %s", createdUser.Username, createdUser.ID)})
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
		if frontEndLogin {
			ctx.JSON(http.StatusUnauthorized, UserLoginResponse{
				Email:  c.Email,
				Jwt:    "",
				Status: "Unauthorized",
			})
		}
		return false
	} else {
		logger.Infof("user is logged in: %+v", u)
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:    "JWT",
		Value:   token,
		Expires: tokenExpirationTime,
		Secure:  true,
	})
	return true
}

type AuthErrorType string

const (
	AuthErrorTypeToken  AuthErrorType = "token"
	AuthErrorTypeCookie AuthErrorType = "cookie"
)

type AuthError struct {
	ErrType AuthErrorType
	ErrMsg  error
}

func (e *AuthError) Error() string {
	return e.ErrMsg.Error()
}

// IsTokenError returns true if the auth error is related to PAT token authentication
func (e *AuthError) IsTokenError() bool {
	return e.ErrType == AuthErrorTypeToken
}

// IsCookieError returns true if the auth error is related to JWT cookie authentication
func (e *AuthError) IsCookieError() bool {
	return e.ErrType == AuthErrorTypeCookie
}

// GetErrorMessage returns a user-friendly error message based on the auth type
func (e *AuthError) GetErrorMessage() string {
	switch e.ErrType {
	case AuthErrorTypeToken:
		return "Invalid or expired access token. Please check your authorization header."
	case AuthErrorTypeCookie:
		return "Please login with your credentials"
	default:
		return "Authentication failed"
	}
}

func (p *MyPlanner) UserAuthentication(ctx *gin.Context, minimumUserLevel user.Level) (user.View, *AuthError) {
	request := ctx.Request

	// Priority 1: Check for Personal Access Token (Authorization header)
	authHeader := request.Header.Get("Authorization")
	if authHeader != "" {
		return p.authenticateWithPAT(ctx, authHeader, minimumUserLevel)
	}

	// Priority 2: Check for JWT in cookies (fallback for web browsers)
	return p.authenticateWithJWT(ctx, minimumUserLevel)
}

// authenticateWithPAT handles Personal Access Token authentication
func (p *MyPlanner) authenticateWithPAT(ctx *gin.Context, authHeader string, minimumUserLevel user.Level) (user.View, *AuthError) {
	var userView user.View

	// Parse Authorization header: "Bearer pat_..."
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return userView, &AuthError{
			ErrType: AuthErrorTypeToken,
			ErrMsg:  errors.New("authorization header must use Bearer scheme"),
		}
	}

	tokenHash := strings.TrimPrefix(authHeader, bearerPrefix)
	if tokenHash == "" {
		return userView, &AuthError{
			ErrType: AuthErrorTypeToken,
			ErrMsg:  errors.New("empty token in authorization header"),
		}
	}

	// Validate the PAT by hash
	tokenRecord, err := p.RedisClient.ValidatePATByHash(ctx, tokenHash)
	if err != nil {
		iowrappers.Logger.Debugf("[request ID: %s] PAT validation failed: %v", ctx.Value(requestIdKey), err)
		return userView, &AuthError{
			ErrType: AuthErrorTypeToken,
			ErrMsg:  errors.New("invalid or expired personal access token"),
		}
	}

	// Get user by ID from token
	userView, findUserErr := p.RedisClient.FindUser(ctx, iowrappers.FindUserByID, user.View{ID: tokenRecord.UserId})
	if findUserErr != nil {
		return userView, &AuthError{
			ErrType: AuthErrorTypeToken,
			ErrMsg:  fmt.Errorf("user not found for token: %w", findUserErr),
		}
	}

	// Check user level
	if err := p.checkUserLevel(userView, minimumUserLevel); err != nil {
		return userView, &AuthError{
			ErrType: AuthErrorTypeToken,
			ErrMsg:  err,
		}
	}

	iowrappers.Logger.Debugf("[request ID: %s] User authenticated via PAT: %s", ctx.Value(requestIdKey), userView.Username)
	return userView, nil
}

// authenticateWithJWT handles JWT cookie authentication (fallback)
func (p *MyPlanner) authenticateWithJWT(ctx *gin.Context, minimumUserLevel user.Level) (user.View, *AuthError) {
	request := ctx.Request
	var userView user.View

	cookie, cookieErr := request.Cookie("JWT")
	if cookieErr != nil {
		return userView, &AuthError{
			ErrType: AuthErrorTypeCookie,
			ErrMsg:  fmt.Errorf("no authentication provided: %w", cookieErr),
		}
	}

	jwtKey := []byte(os.Getenv("JWT_SIGNING_SECRET"))
	token, tokenErr := jwt.Parse(cookie.Value, func(tkn *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if tokenErr != nil {
		return userView, &AuthError{
			ErrType: AuthErrorTypeCookie,
			ErrMsg:  fmt.Errorf("invalid JWT token: %w", tokenErr),
		}
	}

	if !token.Valid {
		return userView, &AuthError{
			ErrType: AuthErrorTypeCookie,
			ErrMsg:  errors.New("invalid JWT token"),
		}
	}

	var username string
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		username = claims["username"].(string)
	} else {
		return userView, &AuthError{
			ErrType: AuthErrorTypeCookie,
			ErrMsg:  errors.New("failed to parse JWT claims"),
		}
	}

	userView, findUserErr := p.RedisClient.FindUser(ctx, iowrappers.FindUserByName, user.View{Username: username})
	if findUserErr != nil {
		return userView, &AuthError{
			ErrType: AuthErrorTypeCookie,
			ErrMsg:  fmt.Errorf("user not found: %w", findUserErr),
		}
	}

	// Check user level
	if err := p.checkUserLevel(userView, minimumUserLevel); err != nil {
		return userView, &AuthError{
			ErrType: AuthErrorTypeCookie,
			ErrMsg:  err,
		}
	}

	iowrappers.Logger.Debugf("[request ID: %s] User authenticated via JWT: %s", ctx.Value(requestIdKey), username)
	return userView, nil
}

// checkUserLevel validates that the user meets the minimum level requirement
func (p *MyPlanner) checkUserLevel(userView user.View, minimumUserLevel user.Level) error {
	var userLevel user.Level
	switch userView.UserLevel {
	case user.LevelStringRegular:
		userLevel = user.LevelRegular
	case user.LevelStringAdmin:
		userLevel = user.LevelAdmin
	}
	if userLevel < minimumUserLevel {
		log.Debugf("user level is %d, required %d", userLevel, minimumUserLevel)
		return errors.New("does not meet minimum user level requirement")
	}
	return nil
}

func (p *MyPlanner) userSavedPlansPostHandler(ctx *gin.Context) {
	var planView user.TravelPlanView
	bindErr := ctx.ShouldBindJSON(&planView)
	if bindErr != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	userView, authErr := p.UserAuthentication(ctx, user.LevelRegular)
	if authErr != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	if userView.Username != ctx.Param("username") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "only logged-in users can view their saved plans"})
		return
	}

	if err := p.RedisClient.SaveUserPlan(ctx, userView, &planView); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"results": "save user plan succeeded."})
}

func (p *MyPlanner) userSavedPlansGetHandler(context *gin.Context) {
	userView, authErr := p.UserAuthentication(context, user.LevelRegular)
	if authErr != nil {
		context.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	if userView.Username != context.Param("username") {
		context.JSON(http.StatusBadRequest, gin.H{"error": "only logged-in users can view their saved plans"})
		return
	}

	iowrappers.Logger.Debugf("current USER ID: %s", userView.ID)
	plans := p.RedisClient.FindUserPlans(context.Request.Context(), userView)

	sort.Sort(user.ByCreatedAt(plans))
	context.JSON(http.StatusOK, gin.H{"travel_plans": plans})
}

func (p *MyPlanner) userPlanDeleteHandler(ctx *gin.Context) {
	userView, authErr := p.UserAuthentication(ctx, user.LevelRegular)
	if authErr != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	if userView.Username != ctx.Param("username") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "only authorized users can delete plans"})
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

	userFav, err := p.RedisClient.UserFavorites(ctx, &userView)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, userFav)
}

func (p *MyPlanner) userFeedbackHandler(ctx *gin.Context) {
	userView, authErr := p.UserAuthentication(ctx, user.LevelRegular)

	if authErr != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": authErr.Error()})
		return
	}

	fb := iowrappers.UserFeedback{UserId: userView.ID}
	err := ctx.ShouldBind(&fb)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = p.RedisClient.UserFeedback(ctx, &fb)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"msg": "plan is updated"})
}
