package planner

import (
	"encoding/json"
	"github.com/globalsign/mgo"
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

	createErr := planner.LoginHandler.CreateUser(u)
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

	token, loginErr := planner.LoginHandler.UserLogin(c)
	if loginErr != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(UserLoginResponse{
			Username: c.Username,
			Jwt:      "",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(UserLoginResponse{
		Username: c.Username,
		Jwt:      token,
	})
}

// remove user handler
func (planner MyPlanner) RemoveUser(w http.ResponseWriter, r *http.Request) {
	removeReq := user.RemoveUserRequest{}

	decodeErr := json.NewDecoder(r.Body).Decode(&removeReq)
	if decodeErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(decodeErr)
		return
	}

	err := planner.LoginHandler.RemoveUser(removeReq.CurrentUser, removeReq.UserToRemove)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(err.Error())
		return
	}

	_ = json.NewEncoder(w).Encode("Removal success")
}
