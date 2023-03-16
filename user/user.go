package user

import (
	"github.com/vmihailenco/msgpack"
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

type View struct {
	ID            string             `json:"id"`
	Username      string             `json:"username"`
	Email         string             `json:"email"`
	Password      string             `json:"password"`
	UserLevel     string             `json:"user_level"`
	Favorites     *PersonalFavorites `json:"favorites"`
	LastLoginTime string             `json:"lastLoginTime"`
}

type Credential struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Email     string `json:"email"`
	WithOAuth bool   `json:"withOAuth"`
}

type PasswordResetRequest struct {
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
	OldPassword      string `json:"old_password"`
	NewPassword      string `json:"new_password"`
}

type PersonalFavorites struct {
	SearchHistory map[string]LastSearchRecord `json:"searchHistory"`
}

type LastSearchRecord struct {
	Location            string `json:"location"`
	Count               int    `json:"count"`
	LastSearchTimestamp string `json:"lastSearchTimestamp"`
}

func (p *PersonalFavorites) MarshalBinary() ([]byte, error) {
	return msgpack.Marshal(p)
}

func (p *PersonalFavorites) UnmarshalBinary(data []byte) error {
	return msgpack.Unmarshal(data, p)
}

// TravelPlaceView reflect what users see on Front-end result tables
type TravelPlaceView struct {
	TimePeriod string `json:"time_period"`
	PlaceName  string `json:"place_name"`
	Address    string `json:"address"`
	URL        string `json:"url"`
}

type TravelPlanView struct {
	ID             string            `json:"id"`
	OriginalPlanID string            `json:"original_plan_id"`
	CreatedAt      string            `json:"created_at"`
	TravelDate     string            `json:"travel_date"`
	Destination    string            `json:"destination"`
	Places         []TravelPlaceView `json:"places"`
}

type ByCreatedAt []TravelPlanView

func (plans ByCreatedAt) Len() int { return len(plans) }

func (plans ByCreatedAt) Swap(i, j int) { plans[i], plans[j] = plans[j], plans[i] }

func (plans ByCreatedAt) Less(i, j int) bool {
	createdAtI, _ := time.Parse(time.RFC3339, plans[i].CreatedAt)
	createdAtJ, _ := time.Parse(time.RFC3339, plans[j].CreatedAt)

	return createdAtI.After(createdAtJ)
}
