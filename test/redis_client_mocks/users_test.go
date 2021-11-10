package redis_client_mocks

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/user"
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
	userView := user.View{Username: "mickey_mouse"}
	planView := user.TravelPlanView{
		ID:             "33521",
		OriginalPlanID: "09201989",
		Destination:    "Los Angeles, USA",
		TravelDate:     "2022-01-29",
		Places: []user.TravelPlaceView{
			{
				TimePeriod: "10 - 12",
				PlaceName:  "Philippe The Original",
				Address:    "1001 N Alameda St, Los Angeles, CA 90012, USA",
				URL:        "https://maps.google.com/?cid=7772213039771900053",
			},
		},
	}

	planView2 := user.TravelPlanView{
		ID:             "33522",
		OriginalPlanID: "06191990",
		Destination:    "Mountain View, USA",
		TravelDate:     "2022-01-31",
		Places: []user.TravelPlaceView{
			{
				TimePeriod: "16 - 17",
				PlaceName:  "GooglePlex",
				Address:    "1600 Amphitheatre Pkwy, Mountain View, CA 94043",
				URL:        "https://maps.google.com/?cid=7772213039771900011",
			},
		},
	}

	var err error

	userView, err = RedisClient.CreateUser(RedisContext, userView)
	if err != nil {
		t.Error(err)
		return
	}

	err = RedisClient.SaveUserPlan(RedisContext, userView, &planView)
	if err != nil {
		t.Error(err)
		return
	}

	err = RedisClient.SaveUserPlan(RedisContext, userView, &planView2)
	if err != nil {
		t.Error(err)
		return
	}

	plans, err := RedisClient.FindUserPlans(RedisContext, userView)
	if err != nil {
		t.Error(err)
		return
	}

	expectedNumberOfPlans := 2
	if len(plans) != expectedNumberOfPlans {
		t.Errorf("expected number of plans to be %d, got %d", expectedNumberOfPlans, len(plans))
		return
	}

	log.Debugf("plan details: %+v", plans)
}

func TestDeleteUserPlan(t *testing.T) {
	userView := user.View{Username: "daisy_duck"}
	planView := user.TravelPlanView{
		ID:             "33521",
		OriginalPlanID: "09201989",
		Destination:    "Los Angeles, USA",
		TravelDate:     "2022-01-29",
		Places: []user.TravelPlaceView{
			{
				TimePeriod: "10 - 12",
				PlaceName:  "Philippe The Original",
				Address:    "1001 N Alameda St, Los Angeles, CA 90012, USA",
				URL:        "https://maps.google.com/?cid=7772213039771900053",
			},
		},
	}

	var err error

	userView, err = RedisClient.CreateUser(RedisContext, userView)
	if err != nil {
		t.Error(err)
		return
	}

	err = RedisClient.SaveUserPlan(RedisContext, userView, &planView)
	if err != nil {
		t.Error(err)
		return
	}

	err = RedisClient.DeleteUserPlan(RedisContext, userView, planView)
	if err != nil {
		t.Error(err)
		return
	}

	var plans []user.TravelPlanView
	plans, err = RedisClient.FindUserPlans(RedisContext, userView)
	if err != nil {
		t.Error(err)
		return
	}

	if len(plans) != 0 {
		t.Errorf("expected no plan remains after deletion, got %d plans remained", len(plans))
		return
	}
	t.Log("test user plan deletion passed")
}
