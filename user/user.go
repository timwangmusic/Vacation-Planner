package user

import (
	"time"
)

type Level uint8

const (
	JWTExpirationTime         = time.Hour * 24 * 10 // 10 days
	LevelStringAdmin   string = "admin"
	LevelStringRegular string = "regular"
	LevelRegular       Level  = 0
	LevelAdmin         Level  = 1
)

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Email     string `json:"email"`
	UserLevel string `json:"user_level"`
}

type View struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	UserLevel string `json:"user_level"`
}

type Credential struct {
	Username string
	Password string
}

type Profile struct {
	ID               string   `json:"id"`
	UserID           string   `json:"user_id"`
	SavedTravelPlans []string `json:"saved_travel_plans"`
}

// TravelPlaceView reflect what users see on Front-end result tables
type TravelPlaceView struct {
	TimePeriod string `json:"time_period"`
	PlaceName  string `json:"place_name"`
	Address    string `json:"address"`
	URL        string `json:"url"`
}

type TravelPlanView struct {
	ID          string            `json:"id"`
	CreatedAt   string            `json:"created_at"`
	TravelDate  string            `json:"travel_date"`
	Destination string            `json:"destination"`
	Places      []TravelPlaceView `json:"places"`
}
