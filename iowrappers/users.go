package iowrappers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/weihesdlegend/Vacation-planner/user"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"golang.org/x/crypto/bcrypt"
	"os"
	"strings"
	"sync"
	"time"
)

const UserKeyPrefix = "user"

type PlanningEvent struct {
	User      string `json:"user"`
	City      string `json:"city"`
	Country   string `json:"country"`
	Timestamp string `json:"timestamp"`
}

type FindUserBy string

const (
	UserSavedTravelPlansPrefix = "user_saved_travel_plans"
	UserSavedTravelPlanPrefix  = "user_saved_travel_plan"

	FindUserByName FindUserBy = "FindUserByName"
	FindUserByID   FindUserBy = "FindUserByID"
)

func (redisClient *RedisClient) FindUser(context context.Context, findUserBy FindUserBy, userView user.View) (user.View, error) {
	redisKey := ""
	switch findUserBy {
	case FindUserByID:
		redisKey = strings.Join([]string{UserKeyPrefix, userView.ID}, ":")
	case FindUserByName:
		redisKey = strings.Join([]string{UserKeyPrefix, userView.Username}, ":")
	}

	if redisClient.client.Exists(context, redisKey).Val() == 0 {
		return userView, errors.New("user does not exist")
	}

	u := redisClient.client.HGetAll(context, redisKey).Val()
	userView.ID = u["id"]
	userView.Username = u["username"]
	userView.Password = u["password"]
	userView.Email = u["email"]
	userView.UserLevel = u["user_level"]

	return userView, nil
}

func (redisClient *RedisClient) CreateUser(context context.Context, userView user.View) (user.View, error) {
	// users can only provide username, instead of an ID
	redisKeyUsername := strings.Join([]string{UserKeyPrefix, userView.Username}, ":")
	if redisClient.client.Exists(context, redisKeyUsername).Val() == 1 {
		return userView, fmt.Errorf("user %s already exists", userView.Username)
	}

	passwordEncrypted, _ := bcrypt.GenerateFromPassword([]byte(userView.Password), bcrypt.DefaultCost)

	userID := uuid.NewString()
	userData := map[string]interface{}{
		"id":         userID,
		"username":   userView.Username,
		"user_level": userView.UserLevel,
		"password":   string(passwordEncrypted),
		"email":      userView.Email,
	}
	_, err := redisClient.client.HMSet(context, redisKeyUsername, userData).Result()
	if err != nil {
		return userView, err
	}

	redisKeyUserID := strings.Join([]string{UserKeyPrefix, userID}, ":")
	_, err = redisClient.client.HMSet(context, redisKeyUserID, userData).Result()
	userView.ID = userID
	return userView, err
}

func (redisClient *RedisClient) Authenticate(context context.Context, credential user.Credential) (string, time.Time, error) {
	userView := user.View{Username: credential.Username}
	u, err := redisClient.FindUser(context, FindUserByName, userView)
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

func (redisClient *RedisClient) SaveUserPlan(context context.Context, userView user.View, planView *user.TravelPlanView) error {
	userView, findUserErr := redisClient.FindUser(context, FindUserByName, userView)
	if findUserErr != nil {
		return findUserErr
	}

	planView.ID = uuid.NewString()
	json_, planSerializationErr := json.Marshal(planView)
	if planSerializationErr != nil {
		return planSerializationErr
	}

	travelPlanRedisKey := strings.Join([]string{TravelPlanRedisCacheKeyPrefix, planView.OriginalPlanID}, ":")
	userSavedPlansRedisKey := strings.Join([]string{UserSavedTravelPlansPrefix, "user", userView.ID, "plans"}, ":")
	if exists, getPlanErr := redisClient.client.SIsMember(context, userSavedPlansRedisKey, travelPlanRedisKey).Result(); getPlanErr != nil || exists {
		if getPlanErr != nil && getPlanErr != redis.Nil {
			return getPlanErr
		}
		if exists {
			return fmt.Errorf("travel plan %s is already saved to profile for user %s", planView.ID, userView.ID)
		}
	}

	var err error
	_, err = redisClient.client.SAdd(context, userSavedPlansRedisKey, travelPlanRedisKey).Result()
	if err != nil {
		return err
	}

	redisKey := strings.Join([]string{UserSavedTravelPlanPrefix, "user", userView.ID, "plan", planView.ID}, ":")
	_, err = redisClient.client.Set(context, redisKey, json_, 0).Result()
	return err
}

func (redisClient *RedisClient) DeleteUserPlan(context context.Context, userView user.View, planView user.TravelPlanView) error {
	userView, findUserErr := redisClient.FindUser(context, FindUserByName, userView)
	if findUserErr != nil {
		return findUserErr
	}

	redisKey := strings.Join([]string{UserSavedTravelPlanPrefix, "user", userView.ID, "plan", planView.ID}, ":")
	Logger.Debugf("plan to be deleted: %s", redisKey)
	wg := sync.WaitGroup{}
	wg.Add(1)

	go redisClient.findUserPlan(context, redisKey, &planView, &wg)

	wg.Wait()

	userSavedPlansRedisKey := strings.Join([]string{UserSavedTravelPlansPrefix, "user", userView.ID, "plans"}, ":")
	travelPlanRedisKey := strings.Join([]string{TravelPlanRedisCacheKeyPrefix, planView.OriginalPlanID}, ":")
	if res, originalPlanIdRemovalErr := redisClient.client.SRem(context, userSavedPlansRedisKey, travelPlanRedisKey).Result(); originalPlanIdRemovalErr != nil && originalPlanIdRemovalErr != redis.Nil {
		Logger.Infof("result from removing original key from hash set is %d", res)
		return originalPlanIdRemovalErr
	}

	return redisClient.RemoveKeys(context, []string{redisKey})
}

func (redisClient *RedisClient) FindUserPlans(context context.Context, userView user.View) ([]user.TravelPlanView, error) {
	var cursor uint64 = 0
	travelPlanKeys := make([]string, 0)

	redisKeysPrefix := strings.Join([]string{UserSavedTravelPlanPrefix, "user", userView.ID, "plan"}, ":")
	for {
		var err error
		var keys []string
		keys, cursor, err = redisClient.client.Scan(context, cursor, redisKeysPrefix+"*", 100).Result()
		if err != nil {
			break
		}
		travelPlanKeys = append(travelPlanKeys, keys...)
		if cursor == 0 {
			break
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(len(travelPlanKeys))

	result := make([]user.TravelPlanView, len(travelPlanKeys))

	for idx, key := range travelPlanKeys {
		go redisClient.findUserPlan(context, key, &result[idx], &wg)
	}
	wg.Wait()

	return result, nil
}

func (redisClient *RedisClient) findUserPlan(context context.Context, redisKey string, view *user.TravelPlanView, wg *sync.WaitGroup) {
	defer wg.Done()
	cachedPlan, err := redisClient.client.Get(context, redisKey).Result()
	if err != nil {
		Logger.Error(err)
		return
	}
	utils.LogErrorWithLevel(json.Unmarshal([]byte(cachedPlan), view), utils.LogError)
}
