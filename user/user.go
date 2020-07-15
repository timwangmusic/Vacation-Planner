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
	Username      string    `json:"username"`
	Password      string    `json:"password"`
	Email         string    `json:"email"`
	UserLevel     string    `json:"user_level"`
}

type Credential struct {
	Username string
	Password string
}

type RemoveUserRequest struct {
	CurrentUser         string `json:"current_user"`
	CurrentUserPassword string `json:"current_user_password"`
	UserToRemove        string `json:"user_to_remove"`
}
