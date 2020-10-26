package redis_client_mocks

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/user"
	"testing"
)

func TestUserAuthentication(t *testing.T) {
	// create an user
	username := "johnny_depp"
	password := "33521"
	userEmail := "johnny_depp@gmail.com"
	if err := RedisClient.CreateUser(RedisContext, user.User{Username: username, Password: password, Email: userEmail}); err != nil {
		t.Error(err)
	}

	// authenticate the user
	_, _, err := RedisClient.Authenticate(RedisContext, user.Credential{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Error(err)
	}
}

func TestUserFind(t *testing.T) {
	username := "jenny"

	var err error
	_, err = RedisClient.FindUser(RedisContext, username)
	expectedErr := errors.New("user does not exist")

	if assert.Error(t, err, "an error was expected") {
		assert.Equal(t, err, expectedErr)
	}
}

func TestUserCreation(t *testing.T) {
	username := "tom_cruise"
	userEmail := "tom_cruise@gmail.com"
	userLevel := user.LevelRegularString

	expectedUser := user.User{
		Username:  username,
		Email:     userEmail,
		UserLevel: userLevel,
	}

	var err error
	err = RedisClient.CreateUser(RedisContext, expectedUser)

	if err != nil {
		t.Error(err)
	}

	usr, err := RedisClient.FindUser(RedisContext, username)
	if err != nil {
		t.Error(err)
	}

	// ignore comparing password in this test
	usr.Password = ""
	assert.Equal(t, expectedUser, usr)
}
