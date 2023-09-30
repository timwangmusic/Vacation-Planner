package iowrappers

import (
	"context"
	"errors"
	"sync"

	"github.com/modern-go/reflect2"
	"github.com/weihesdlegend/Vacation-planner/user"
)

func scanRedisKeys(context context.Context, redisClient *RedisClient, redisKeyPrefix string) ([]string, error) {
	var redisKeys []string
	if reflect2.IsNil(redisClient) {
		return redisKeys, errors.New("redis client pointer is nil")
	}

	var cursor uint64
	for {
		var err error
		var keys []string
		keys, cursor, err = redisClient.client.Scan(context, cursor, redisKeyPrefix+"*", 100).Result()
		if err != nil {
			return redisKeys, err
		}

		redisKeys = append(redisKeys, keys...)

		if cursor == 0 {
			break
		}
	}
	return redisKeys, nil
}

// Filter places that meet certain condition
func Filter[T any](places []T, condition func(place T) bool) []T {
	var results []T
	for _, place := range places {
		if condition(place) {
			results = append(results, place)
		}
	}
	return results
}

func toUserView(userData map[string]string) (user.View, error) {
	var view user.View
	view.ID = userData["id"]
	view.Username = userData["username"]
	view.Email = userData["email"]
	view.Password = userData["password"]
	view.UserLevel = userData["user_level"]
	view.Favorites = &user.PersonalFavorites{SearchHistory: make(map[string]user.LastSearchRecord)}
	if userData["favorites"] != "" {
		if err := view.Favorites.UnmarshalBinary([]byte(userData["favorites"])); err != nil {
			return user.View{}, err
		}
	}
	view.LastLoginTime = userData["lastLoginTime"]
	return view, nil
}

type View interface {
	user.View | user.TravelPlanView
}

func merge[V View](cs ...chan V) chan V {
	var wg sync.WaitGroup
	out := make(chan V)

	output := func(c <-chan V) {
		for view := range c {
			out <- view
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
