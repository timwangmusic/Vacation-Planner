package user

import (
	"time"
)

const (
	LevelAdmin        = "Admin"
	LevelRegular      = "Regular"
	JWTExpirationTime = time.Hour * 240 // 10 days
)

type User struct {
	Username  string `bson:"_id"`
	Password  string `bson:"password"`
	Email     string `bson:"email"`
	UserLevel string `bson:"user_level"`
}

type Credential struct {
	Username string
	Password string
}

type RemoveUserRequest struct {
	CurrentUser  string `json:"current_user"`
	UserToRemove string `json:"user_to_remove"`
}
