package iowrappers

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
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

type FindUserBy string

const (
	FindUserByName FindUserBy = "FindUserByName"
	FindUserByID   FindUserBy = "FindUserByID"
)

func (redisClient *RedisClient) FindUser(context context.Context, findUserBy FindUserBy, userView user.View) (user.View, error) {
	redisKey := ""
	switch findUserBy {
	case FindUserByID:
		redisKey = strings.Join([]string{UserKeyPrefix, userView.ID}, ":")
	case FindUserByName:
		redisKey = strings.Join([]string{UserKeyPrefix, userView.Username}, ":")
	}

	if redisClient.client.Exists(context, redisKey).Val() == 0 {
		return userView, errors.New("user does not exist")
	}

	u := redisClient.client.HGetAll(context, redisKey).Val()
	userView.ID = u["id"]
	userView.Username = u["username"]
	userView.Password = u["password"]
	userView.Email = u["email"]
	userView.UserLevel = u["user_level"]

	return userView, nil
}

func (redisClient *RedisClient) CreateUser(context context.Context, userView user.View) (user.View, error) {
	// users can only provide username, instead of an ID
	redisKeyUsername := strings.Join([]string{UserKeyPrefix, userView.Username}, ":")
	if redisClient.client.Exists(context, redisKeyUsername).Val() == 1 {
		return userView, fmt.Errorf("user %s already exists", userView.Username)
	}

	passwordEncrypted, _ := bcrypt.GenerateFromPassword([]byte(userView.Password), bcrypt.DefaultCost)

	userID := uuid.NewString()
	userData := map[string]interface{}{
		"id":         userID,
		"username":   userView.Username,
		"user_level": userView.UserLevel,
		"password":   string(passwordEncrypted),
		"email":      userView.Email,
	}
	_, err := redisClient.client.HMSet(context, redisKeyUsername, userData).Result()
	if err != nil {
		return userView, err
	}

	redisKeyUserID := strings.Join([]string{UserKeyPrefix, userID}, ":")
	_, err = redisClient.client.HMSet(context, redisKeyUserID, userData).Result()
	userView.ID = userID
	return userView, err
}

func (redisClient *RedisClient) Authenticate(context context.Context, credential user.Credential) (string, time.Time, error) {
	userView := user.View{Username: credential.Username}
	u, err := redisClient.FindUser(context, FindUserByName, userView)
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
