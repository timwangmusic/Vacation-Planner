package iowrappers

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/weihesdlegend/Vacation-planner/user"
	"golang.org/x/crypto/bcrypt"
	"os"
	"strings"
	"time"
)

const UserKeyPrefix = "user"

type PlanningEvent struct {
	User      string `json:"user"`
	City      string `json:"city"`
	Country   string `json:"country"`
	Timestamp string `json:"timestamp"`
}

func (redisClient *RedisClient) FindUser(context context.Context, username string) (user.View, error) {
	userView := user.View{Username: "guest"}
	redisKey := strings.Join([]string{UserKeyPrefix, username}, ":")
	if redisClient.client.Exists(context, redisKey).Val() == 0 {
		return userView, errors.New("user does not exist")
	}

	u := redisClient.client.HGetAll(context, redisKey).Val()
	userView.Username = u["username"]
	userView.Password = u["password"]
	userView.Email = u["email"]
	userView.UserLevel = u["user_level"]
	return userView, nil
}

func (redisClient *RedisClient) CreateUser(context context.Context, userView user.View) (user.View, error) {
	redisKey := strings.Join([]string{UserKeyPrefix, userView.Username}, ":")
	if redisClient.client.Exists(context, redisKey).Val() == 1 {
		return userView, fmt.Errorf("user %s already exists", userView.Username)
	}

	psw, _ := bcrypt.GenerateFromPassword([]byte(userView.Password), bcrypt.DefaultCost)

	userData := map[string]interface{}{
		"username":   userView.Username,
		"user_level": userView.UserLevel,
		"password":   string(psw),
		"email":      userView.Email,
	}
	_, err := redisClient.client.HMSet(context, redisKey, userData).Result()
	return userView, err
}

// Authenticate a user when a new user that holds no JWT or an existing user with expired JWT
func (redisClient *RedisClient) Authenticate(context context.Context, credential user.Credential) (string, time.Time, error) {
	u, err := redisClient.FindUser(context, credential.Username)
	if err != nil {
		return "", time.Now(), err
	}

	pswCompErr := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(credential.Password))
	if pswCompErr != nil { // wrong password
		err = errors.New("wrong password")
		return "", time.Now(), err
	}

	lastLoginTime := time.Now()
	tokenExpirationTime := lastLoginTime.Add(user.JWTExpirationTime)
	expiresAt := tokenExpirationTime.Unix() // expires after 10 days
	jwtSigningSecret := os.Getenv("JWT_SIGNING_SECRET")

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username":       u.Username,
		"StandardClaims": jwt.StandardClaims{ExpiresAt: expiresAt},
	})

	token, jwtSignErr := jwtToken.SignedString([]byte(jwtSigningSecret))
	return token, tokenExpirationTime, jwtSignErr
}
