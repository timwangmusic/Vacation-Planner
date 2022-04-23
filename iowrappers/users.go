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
	"net/mail"
	"os"
	"strings"
	"sync"
	"time"
)

const UserKeyPrefix = "user"

type GoogleOAuthResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	PictureURL    string `json:"picture"`
}

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

	//UserNamesKey maps usernames to IDs
	UserNamesKey = "user_names"

	//UserEmailsKey maps emails to IDs
	UserEmailsKey = "user_emails"

	FindUserByName  FindUserBy = "FindUserByName"
	FindUserByID    FindUserBy = "FindUserByID"
	FindUserByEmail FindUserBy = "FindUserByEmail"
)

func (redisClient *RedisClient) FindUser(context context.Context, findUserBy FindUserBy, userView user.View) (user.View, error) {
	client := redisClient.client
	redisKey := ""
	switch findUserBy {
	case FindUserByID:
		redisKey = strings.Join([]string{UserKeyPrefix, userView.ID}, ":")
	case FindUserByName:
		userId, err := client.HGet(context, UserNamesKey, userView.Username).Result()
		if err != nil {
			return user.View{}, err
		}
		redisKey = strings.Join([]string{UserKeyPrefix, userId}, ":")
	case FindUserByEmail:
		userId, err := client.HGet(context, UserEmailsKey, userView.Email).Result()
		if err != nil {
			return user.View{}, err
		}
		redisKey = strings.Join([]string{UserKeyPrefix, userId}, ":")
	}

	if client.Exists(context, redisKey).Val() == 0 {
		return userView, errors.New("user does not exist")
	}

	u := client.HGetAll(context, redisKey).Val()
	userView.ID = u["id"]
	userView.Username = u["username"]
	userView.Password = u["password"]
	userView.Email = u["email"]
	userView.UserLevel = u["user_level"]

	return userView, nil
}

func (redisClient *RedisClient) CreateUser(context context.Context, userView user.View) (user.View, error) {
	client := redisClient.client

	if client.HExists(context, UserNamesKey, userView.Username).Val() {
		return userView, fmt.Errorf("user %s already exists", userView.Username)
	}

	// email addresses are not case sensitive
	userView.Email = strings.ToLower(userView.Email)

	if client.HExists(context, UserEmailsKey, userView.Email).Val() {
		return userView, fmt.Errorf("user %s already exists", userView.Email)
	}

	if _, err := mail.ParseAddress(userView.Email); err != nil {
		return userView, fmt.Errorf("invalid email: %v", err)
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

	// username is required
	if _, err := client.HSet(context, UserNamesKey, userView.Username, userID).Result(); err != nil {
		return userView, err
	}

	if userView.Email != "" {
		if _, err := client.HSet(context, UserEmailsKey, userView.Email, userID).Result(); err != nil {
			return userView, err
		}
	}

	redisKeyUserID := strings.Join([]string{UserKeyPrefix, userID}, ":")
	_, err := client.HMSet(context, redisKeyUserID, userData).Result()
	userView.ID = userID
	return userView, err
}

func (redisClient *RedisClient) Authenticate(context context.Context, credential user.Credential) (user.View, string, time.Time, error) {
	userView := user.View{Username: credential.Username, Email: strings.ToLower(credential.Email)}
	Logger.Debugf("->Authenticate: user view is %v", userView)
	var u user.View
	var err error
	var loggedInByEmail bool
	u, err = redisClient.FindUser(context, FindUserByName, userView)
	if err != nil {
		Logger.Debugf("cannot find user by username %s, error: %v", credential.Username, err)
		loggedInByEmail = true
	}

	if loggedInByEmail {
		Logger.Debugf("->Authenticate: email from credential is %s", credential.Email)
		userView.Email = credential.Email
		if strings.TrimSpace(credential.Email) == "" {
			userView.Email = strings.ToLower(credential.Username)
		}
		u, err = redisClient.FindUser(context, FindUserByEmail, userView)
		Logger.Debugf("cannot find user by email %s, error: %v", credential.Email, err)
		if err != nil {
			return u, "", time.Now(), err
		}
	}

	if !credential.WithOAuth {
		pswCompErr := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(credential.Password))
		if pswCompErr != nil { // wrong password
			err = errors.New("wrong password")
			return u, "", time.Now(), err
		}
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
	return u, token, tokenExpirationTime, jwtSignErr
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
