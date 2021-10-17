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
