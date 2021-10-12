package redis_client_mocks

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/user"
	"testing"
)

func TestUserAuthentication(t *testing.T) {
	// create a user
	username := "johnny_depp"
	password := "33521"
	userEmail := "johnny_depp@gmail.com"
	if _, err := RedisClient.CreateUser(RedisContext, user.View{Username: username, Password: password, Email: userEmail}); err != nil {
		t.Error(err)
		return
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
	userLevel := user.LevelStringRegular

	expectedUserView := user.View{
		Username:  username,
		Email:     userEmail,
		Password:  "",
		UserLevel: userLevel,
	}

	var err error
	_, err = RedisClient.CreateUser(RedisContext, expectedUserView)

	if err != nil {
		t.Error(err)
		return
	}

	usr, err := RedisClient.FindUser(RedisContext, username)
	if err != nil {
		t.Error(err)
	}

	// ignore comparing password in this test
	usr.Password = ""
	assert.Equal(t, expectedUserView, usr)
}
