package iowrappers

import (
	"context"
	"errors"
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

// lookup an user
func (redisClient *RedisClient) FindUser(context context.Context, username string) (user.User, error) {
	usr := user.User{Username: "guest"}
	redisKey := strings.Join([]string{UserKeyPrefix, username}, ":")
	if redisClient.client.Exists(context, redisKey).Val() == 0 {
		return usr, errors.New("user does not exist")
	}

	u := redisClient.client.HGetAll(context, redisKey).Val()
	usr.Username = u["username"]
	usr.Email = u["email"]
	usr.UserLevel = u["user_level"]
	usr.Password = u["password"]
	return usr, nil
}

// create a new user
func (redisClient *RedisClient) CreateUser(context context.Context, usr user.User) error {
	redisKey := strings.Join([]string{UserKeyPrefix, usr.Username}, ":")
	if redisClient.client.Exists(context, redisKey).Val() == 1 {
		return errors.New("user already exists")
	}

	psw, _ := bcrypt.GenerateFromPassword([]byte(usr.Password), bcrypt.DefaultCost)
	if usr.UserLevel == "" {
		usr.UserLevel = user.LevelRegularString
	}

	userData := map[string]interface{}{
		"username":   usr.Username,
		"user_level": usr.UserLevel,
		"password":   string(psw),
		"email":      usr.Email,
	}
	_, err := redisClient.client.HMSet(context, redisKey, userData).Result()
	return err
}

// authenticate an user when a new user that holds no JWT or an existing user with expired JWT
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
