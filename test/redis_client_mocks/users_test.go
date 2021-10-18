package redis_client_mocks

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
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
	userView := user.View{Username: "Jenny"}

	var err error
	_, err = RedisClient.FindUser(RedisContext, iowrappers.FindUserByName, userView)
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
		ID:        "",
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

	actualUserView, err := RedisClient.FindUser(RedisContext, iowrappers.FindUserByName, expectedUserView)
	if err != nil {
		t.Error(err)
		return
	}

	// ignore comparing ID and password in this test
	actualUserView.Password = ""
	actualUserView.ID = ""
	assert.Equal(t, expectedUserView, actualUserView)
}

func TestSaveUserPlan(t *testing.T) {
	userView := user.View{Username: "tom_cruise"}
	planView := user.TravelPlanView{
		ID: "33521",
		Places: []user.TravelPlaceView{
			{
				Category:   string(POI.PlaceCategoryEatery),
				TimePeriod: "10 - 12",
				PlaceName:  "Philippe The Original",
				Address:    "1001 N Alameda St, Los Angeles, CA 90012, USA",
			},
		},
	}

	userView, _ = RedisClient.CreateUser(RedisContext, userView)

	err := RedisClient.SaveUserPlan(RedisContext, userView, planView)
	if err != nil {
		t.Error(err)
		return
	}

	savedUserPlans, err := RedisClient.FindUserPlans(RedisContext, userView)
	if err != nil {
		t.Error(err)
		return
	}
	var expectedNumSavedUserPlans = 1
	if len(savedUserPlans) != expectedNumSavedUserPlans {
		t.Errorf("Expected to find %d saved plans, got %d", expectedNumSavedUserPlans, len(savedUserPlans))
		return
	}
	// avoid comparing ID
	planView.ID = savedUserPlans[0].ID
	assert.Equal(t, planView, savedUserPlans[0])
}
