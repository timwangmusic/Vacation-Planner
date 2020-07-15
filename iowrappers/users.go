package iowrappers

import (
	"errors"
	"github.com/weihesdlegend/Vacation-planner/user"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

const UserKeyPrefix = "user"

// lookup an user
func (redisClient *RedisClient) FindUser(username string) (user.User, error) {
	usr := user.User{Username: "guest"}
	redisKey := strings.Join([]string{UserKeyPrefix, username}, ":")
	if redisClient.client.Exists(redisKey).Val() == 0 {
		return usr, errors.New("user does not exist")
	}

	u := redisClient.client.HGetAll(redisKey).Val()
	usr.Username = u["username"]
	usr.Email = u["email"]
	usr.UserLevel = u["user_level"]
	return usr, nil
}

// create a new user
func (redisClient *RedisClient) CreateUser(user user.User) error {
	redisKey := strings.Join([]string{UserKeyPrefix, user.Username}, ":")
	if redisClient.client.Exists(redisKey).Val() == 1 {
		return errors.New("user already exists")
	}

	psw, _ := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	userData := map[string]interface{}{
		"username":   user.Username,
		"user_level": user.UserLevel,
		"password":   string(psw),
		"email":      user.Email,
	}
	_, err := redisClient.client.HMSet(redisKey, userData).Result()
	return err
}
