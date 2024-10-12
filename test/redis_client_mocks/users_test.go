package redis_client_mocks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/user"
)

func TestUserAuthentication_shouldPass_forAuthenticatedUser(t *testing.T) {
	// create a user
	username := "johnny_depp"
	password := "33521"
	userEmail := "johnny_depp@gmail.com"
	if _, err := RedisClient.CreateUser(RedisContext, user.View{Username: username, Password: password, Email: userEmail}, false); err != nil {
		t.Error(err)
		return
	}

	// authenticate the user
	_, _, _, err := RedisClient.Authenticate(RedisContext, user.Credential{
		Email:    userEmail,
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Error(err)
	}
}

func TestUserFind_ShouldReturnNotFound_whenUserDoesNotExist(t *testing.T) {
	userView := user.View{Username: "Jenny"}

	var err error
	_, err = RedisClient.FindUser(RedisContext, iowrappers.FindUserByName, userView)
	expectedErr := errors.New("cannot find user name Jenny")

	if assert.Error(t, err, "an error was expected") {
		assert.Equal(t, expectedErr, err)
	}
}

func TestUpdateUser(t *testing.T) {
	view := user.View{Username: "Teddy Bear", Email: "teddy@hotmail.com"}

	var createdView user.View
	var err error
	if createdView, err = RedisClient.CreateUser(RedisContext, view, false); err != nil {
		t.Fatalf("failed to update user %v", err)
	}

	createdView.Username = "Teddy"
	err = RedisClient.UpdateUser(RedisContext, &createdView)
	if err != nil {
		t.Fatalf("failed to update user %v", err)
	}

	updatedView, err := RedisClient.FindUser(RedisContext, iowrappers.FindUserByName, createdView)
	if err != nil {
		t.Fatalf("failed to find user %v", err)
	}

	if updatedView.Username != "Teddy" {
		t.Errorf("username failed to update")
	}
}

func TestUserCreation(t *testing.T) {
	username := "teddy_cruise"
	userEmail := "teddy_cruise@gmail.com"
	userLevel := user.LevelStringRegular

	expectedUserView := user.View{
		ID:        "",
		Username:  username,
		Email:     userEmail,
		Password:  "",
		UserLevel: userLevel,
		Favorites: &user.PersonalFavorites{},
	}

	var err error
	_, err = RedisClient.CreateUser(RedisContext, expectedUserView, false)

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
	userView := user.View{Username: "mickey_mouse", Email: "micky_mouse@google.com"}
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

	userView, err = RedisClient.CreateUser(RedisContext, userView, false)
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

	plans := RedisClient.FindUserPlans(RedisContext, userView)

	expectedNumberOfPlans := 2
	if len(plans) != expectedNumberOfPlans {
		t.Errorf("expected number of plans to be %d, got %d", expectedNumberOfPlans, len(plans))
		return
	}

	t.Logf("plan details: %+v", plans)
}

func TestDeleteUserPlan(t *testing.T) {
	userView := user.View{Username: "daisy_duck", Email: "daisy_duck@disney.com"}
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

	userView, err = RedisClient.CreateUser(RedisContext, userView, false)
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

	plans := RedisClient.FindUserPlans(RedisContext, userView)

	if len(plans) != 0 {
		t.Errorf("expected no plan remains after deletion, got %d plans remained", len(plans))
		return
	}
	t.Log("test user plan deletion passed")
}
