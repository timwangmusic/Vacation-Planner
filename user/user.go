package user

import (
	"time"
)

const (
	JWTExpirationTime = time.Hour * 240 // 10 days
)

type User struct {
	Username string `bson:"_id"`
	Password string `bson:"password"`
	Email    string `bson:"email"`
}

type Credential struct {
	Username string
	Password string
}
