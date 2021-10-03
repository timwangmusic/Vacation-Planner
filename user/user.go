package user

import (
	"github.com/google/uuid"
	"time"
)

type Level uint8

const (
	JWTExpirationTime        = time.Hour * 240 // 10 days
	LevelAdminString         = "admin"
	LevelRegularString       = "regular"
	LevelRegular       Level = 0
	LevelAdmin         Level = 1
)

type User struct {
	ID        uuid.UUID
	Username  string `json:"username"`
	Password  string `json:"password"`
	Email     string `json:"email"`
	UserLevel string `json:"user_level"`
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
