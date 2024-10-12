package redis_client_mocks

import (
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/user"
	"testing"
)

func TestUserFavorite_shouldReturnCorrectResult(t *testing.T) {
	username := "tom_cruise"
	userEmail := "tom_cruise@gmail.com"
	userLevel := user.LevelStringRegular

	expectedUserView := user.View{
		ID:        "",
		Username:  username,
		Email:     userEmail,
		Password:  "",
		UserLevel: userLevel,
		Favorites: &user.PersonalFavorites{},
	}

	view, err := RedisClient.CreateUser(RedisContext, expectedUserView, false)

	if err != nil {
		t.Error(err)
		return
	}

	const SanDiegoSearchCount = 10
	for i := 0; i < SanDiegoSearchCount; i++ {
		err = RedisClient.UpdateSearchHistory(RedisContext, "San Diego, CA, USA", &view, false)
		if err != nil {
			t.Error(err.Error())
			return
		}
	}

	const NewYorkSearchCount = 15
	for i := 0; i < NewYorkSearchCount; i++ {
		err = RedisClient.UpdateSearchHistory(RedisContext, "New York, New York, USA", &view, false)
		if err != nil {
			t.Error(err.Error())
			return
		}
	}

	userFavRedisKey := iowrappers.UserSearchHistoryPrefix + ":user:" + view.ID
	t.Log(RedisClient.Get().HGetAll(RedisContext, userFavRedisKey).Val())
	favorites, err := RedisClient.UserFavorites(RedisContext, &view)
	if err != nil {
		t.Errorf("UserFavorites returned error: %v", err)
		return
	}

	if favorites.MostFrequentSearch != "New York, New York, USA" {
		t.Errorf("UserFavorites returned incorrect result: %s, expected New York, New York, USA",
			favorites.MostFrequentSearch)
	}
}
