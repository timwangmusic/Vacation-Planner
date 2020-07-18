package planner

import (
	"encoding/json"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/globalsign/mgo"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/user"
	"net/http"
	"os"
	"strings"
)

type UserLoginResponse struct {
	Username string `json:"username"`
	Jwt      string `json:"jwt"`
}

// user signup POST request handler
// user submit username/password/email and user is created
// return bad request if creation fails
func (planner MyPlanner) UserSignup(w http.ResponseWriter, r *http.Request) {
	u := user.User{}

	decodeErr := json.NewDecoder(r.Body).Decode(&u)
	if decodeErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(decodeErr)
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
		w.WriteHeader(http.StatusBadRequest)
		if mgo.IsDup(createErr) {
			_ = json.NewEncoder(w).Encode("User already exists")
		} else {
			_ = json.NewEncoder(w).Encode(createErr.Error())
		}
		return
	}
	_ = json.NewEncoder(w).Encode("user created")
}

// user login POST request handler
// user submit credentials and return JWT if login is successful
func (planner MyPlanner) UserLogin(w http.ResponseWriter, r *http.Request) {
	c := user.Credential{}

	decodeErr := json.NewDecoder(r.Body).Decode(&c)
	if decodeErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(decodeErr)
		return
	}

	token, tokenExpirationTime, loginErr := planner.RedisClient.Authenticate(c)
	if loginErr != nil {
		log.Debug(loginErr)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(UserLoginResponse{
			Username: c.Username,
			Jwt:      "",
		})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "JWT",
		Value:   token,
		Expires: tokenExpirationTime,
	})

	http.SetCookie(w, &http.Cookie{
		Name:  "Username",
		Value: c.Username,
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
