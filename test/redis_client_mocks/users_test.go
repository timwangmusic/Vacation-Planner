package redis_client_mocks

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/user"
	"testing"
)

func TestUserFind(t *testing.T) {
	username := "johnny_depp"

	var err error
	_, err = RedisClient.FindUser(username)
	expectedErr := errors.New("user does not exist")

	if assert.Error(t, err, "an error was expected") {
		assert.Equal(t, err, expectedErr)
	}
}

func TestUserCreation(t *testing.T) {
	username := "johnny_depp"
	userEmail := "johnny_depp@gmail.com"
	userLevel := user.LevelRegular

	expectedUser := user.User{
		Username:  username,
		Email:     userEmail,
		UserLevel: userLevel,
	}

	var err error
	err = RedisClient.CreateUser(expectedUser)

	if err != nil {
		t.Error(err)
	}

	usr, err := RedisClient.FindUser(username)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, expectedUser, usr)
}
