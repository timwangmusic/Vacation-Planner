package planner

import (
	"encoding/json"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
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

// user signup POST request handler
// user submit username/password/email and user is created
// return bad request if creation fails
func (planner MyPlanner) UserSignup(c *gin.Context) {
	u := user.User{}

	decodeErr := c.ShouldBindJSON(&u)
	if decodeErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
		return
	}

	userLevel := user.LevelRegular
	adminUsers := strings.Split(os.Getenv("ADMIN_USERS"), ",")
	for _, username := range adminUsers {
		if username == u.Username {
			userLevel = user.LevelAdmin
		}
	}

	u.UserLevel = userLevel

	createErr := planner.RedisClient.CreateUser(u)
	if createErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": createErr.Error()})
		return
	}
	_ = json.NewEncoder(c.Writer).Encode("user created")
}

// user login POST request handler
// user submit credentials and return JWT if login is successful
func (planner MyPlanner) UserLogin(ctx *gin.Context) {
	c := user.Credential{}

	decodeErr := ctx.ShouldBindJSON(&c)
	if decodeErr != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
		return
	}

	token, tokenExpirationTime, loginErr := planner.RedisClient.Authenticate(c)
	if loginErr != nil {
		log.Debug(loginErr)
		ctx.JSON(http.StatusUnauthorized, UserLoginResponse{
			Username: c.Username,
			Jwt:      "",
			Status:   "unauthorized",
		})
		return
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:    "JWT",
		Value:   token,
		Expires: tokenExpirationTime,
	})

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:  "Username",
		Value: c.Username,
	})

	ctx.JSON(http.StatusOK, UserLoginResponse{
		Username: c.Username,
		Jwt:      token,
		Status:   "you are logged in",
	})
}

func (planner MyPlanner) UserAuthentication(r *http.Request) (username string, err error) {
	cookie, cookieErr := r.Cookie("JWT")
	if cookieErr != nil {
		return "", cookieErr
	}

	jwtKey := []byte(os.Getenv("JWT_SIGNING_SECRET"))
	token, tokenErr := jwt.Parse(cookie.Value, func(tkn *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if tokenErr != nil {
		return "", tokenErr
	}

	if !token.Valid {
		return "", errors.New("invalid token")
	}

	userCookie, _ := r.Cookie("Username")
	username = userCookie.Value
	log.Debugf("the current user is %s", username)
	return username, nil
}
