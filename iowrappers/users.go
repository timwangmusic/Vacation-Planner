package iowrappers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/weihesdlegend/Vacation-planner/user"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	UserKeyPrefix       = "user"
	UserPlanWorkerCount = 5
)

type GoogleOAuthResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	PictureURL    string `json:"picture"`
}

type PlanningEvent struct {
	User              string `json:"user"`
	City              string `json:"city"`
	Country           string `json:"country"`
	AdminAreaLevelOne string `json:"admin_area_level_one"`
	Timestamp         string `json:"timestamp"`
}

type VerificationResult struct {
	Message    error
	HttpStatus int
}

type FindUserBy string

const (
	UserSavedTravelPlansPrefix = "user_saved_travel_plans"
	UserSavedTravelPlanPrefix  = "user_saved_travel_plan"
	UserSearchHistoryPrefix    = "users:search_history"

	//UserNamesKey maps usernames to IDs
	UserNamesKey = "user_names"

	//UserEmailsKey maps emails to IDs
	UserEmailsKey = "user_emails"

	//EmailVerificationCodes maps email to verification code
	EmailVerificationCodes = "email_verification_codes"

	FindUserByName  FindUserBy = "FindUserByName"
	FindUserByID    FindUserBy = "FindUserByID"
	FindUserByEmail FindUserBy = "FindUserByEmail"
)

func (r *RedisClient) UpdateSearchHistory(ctx context.Context, location string, userView *user.View, preciseLocation bool) error {
	// do not update search history for precise location
	if preciseLocation {
		return nil
	}
	if userView.ID == "" {
		if view, err := r.FindUser(ctx, FindUserByName, *userView); err != nil {
			return err
		} else {
			userView.ID = view.ID
		}
	}

	redisKey := strings.Join([]string{UserSearchHistoryPrefix, "user", userView.ID}, ":")
	err := r.Get().Watch(ctx, func(tx *redis.Tx) error {
		record := &user.LastSearchRecord{}
		val, err := tx.HGet(ctx, redisKey, location).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}

		if val != "" {
			if err = json.Unmarshal([]byte(val), record); err != nil {
				return fmt.Errorf("failed to unmarshal user search history: %w", err)
			}
		}

		// Update the record
		record.Location = location
		record.LastSearchTimestamp = time.Now().Format(time.RFC3339)
		record.Count++

		// Serialize the record
		serialized, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to serialize user search history: %w", err)
		}

		// Execute the transaction
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.HSet(ctx, redisKey, location, serialized)
			return nil
		})
		return err
	}, redisKey)

	if err != nil {
		return fmt.Errorf("failed to update search history: %w", err)
	}

	return nil
}

type Favorites struct {
	MostFrequentSearch string `json:"most_frequent_search"`
}

func (r *RedisClient) UserFavorites(ctx context.Context, view *user.View) (*Favorites, error) {
	if view.ID == "" {
		return nil, errors.New("user id is required")
	}

	redisKey := strings.Join([]string{UserSearchHistoryPrefix, "user", view.ID}, ":")
	searchHistory, err := r.Get().HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user search history: %w", err)
	}

	topLocation, err := findTopLocation(searchHistory)
	if err != nil {
		return nil, fmt.Errorf("failed to find favorite location: %w", err)
	}

	return &Favorites{MostFrequentSearch: topLocation}, nil
}

func findTopLocation(locationData map[string]string) (string, error) {
	var (
		topLocation string
		curMax      int
	)

	for location, data := range locationData {
		record := &user.LastSearchRecord{}
		if err := json.Unmarshal([]byte(data), record); err != nil {
			Logger.Errorf("failed to unmarshal user search history for location %s: %v", location, err)
			continue
		}

		if record.Count > curMax {
			topLocation = record.Location
			curMax = record.Count
			Logger.Infof("current top location is %s with count %d", topLocation, record.Count)
		}
	}

	return topLocation, nil
}

func (r *RedisClient) VerifyPasswordResetRequest(ctx context.Context, req *user.PasswordResetRequest) VerificationResult {
	if strings.TrimSpace(req.VerificationCode) == "" {
		return VerificationResult{
			Message:    errors.New("password reset verification code should not be empty"),
			HttpStatus: http.StatusBadRequest,
		}
	}
	key := "password_reset:" + req.VerificationCode
	if claimed, err := r.Get().HGet(ctx, key, "claimed").Result(); err != nil {
		return VerificationResult{
			Message:    errors.New("cannot determine if the verification code is claimed"),
			HttpStatus: http.StatusInternalServerError,
		}
	} else if parsedClaimed, claimedParsingErr := strconv.ParseBool(claimed); claimedParsingErr != nil {
		return VerificationResult{
			Message:    errors.New("cannot determine if the verification code is claimed"),
			HttpStatus: http.StatusInternalServerError,
		}
	} else if parsedClaimed {
		return VerificationResult{
			Message:    errors.New("the verification code is already claimed"),
			HttpStatus: http.StatusBadRequest,
		}
	}

	if err := r.Get().HSet(ctx, key, "claimed", true).Err(); err != nil {
		return VerificationResult{
			Message:    errors.New("fails to claim password reset verification code"),
			HttpStatus: http.StatusInternalServerError,
		}
	}

	if email, err := r.Get().HGet(ctx, key, "user_email").Result(); err != nil {
		return VerificationResult{
			Message:    errors.New("fails to fetch user email with verification code"),
			HttpStatus: http.StatusInternalServerError,
		}
	} else if email != req.Email {
		return VerificationResult{
			Message:    errors.New("email address and verification code do not match"),
			HttpStatus: http.StatusForbidden,
		}
	}
	return VerificationResult{
		Message:    nil,
		HttpStatus: http.StatusOK,
	}
}

func (r *RedisClient) SetPassword(ctx context.Context, req *user.PasswordResetRequest) error {
	view, err := r.FindUser(ctx, FindUserByEmail, user.View{Email: req.Email})
	if err != nil {
		return err
	}
	Logger.Debugf("->SetPassword: user view %+v", view)
	passwordEncrypted, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	view.Password = string(passwordEncrypted)
	Logger.Debugf("->SetPassword: resetting password for user %s", view.ID)
	return r.UpdateUser(ctx, &view)
}

// UpdateUser should only accept a complete view of the user so that no user information is lost after update.
func (r *RedisClient) UpdateUser(ctx context.Context, view *user.View) error {
	redisUserKey := strings.Join([]string{UserKeyPrefix, view.ID}, ":")

	// username is required
	if _, err := r.Get().HSet(ctx, UserNamesKey, view.Username, view.ID).Result(); err != nil {
		return err
	}

	if view.Email != "" {
		if _, err := r.Get().HSet(ctx, UserEmailsKey, view.Email, view.ID).Result(); err != nil {
			return err
		}
	}

	if _, err := r.Get().HMSet(ctx, redisUserKey, toRedisUserData(view)).Result(); err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) FindUser(context context.Context, findUserBy FindUserBy, userView user.View) (user.View, error) {
	client := r.client
	redisKey := ""
	switch findUserBy {
	case FindUserByID:
		redisKey = strings.Join([]string{UserKeyPrefix, userView.ID}, ":")
	case FindUserByName:
		userId, err := client.HGet(context, UserNamesKey, userView.Username).Result()
		if err != nil {
			return user.View{}, fmt.Errorf("cannot find user name %s", userView.Username)
		}
		redisKey = strings.Join([]string{UserKeyPrefix, userId}, ":")
	case FindUserByEmail:
		userId, err := client.HGet(context, UserEmailsKey, userView.Email).Result()
		if err != nil {
			return user.View{}, fmt.Errorf("cannot find user email %s", userView.Email)
		}
		redisKey = strings.Join([]string{UserKeyPrefix, userId}, ":")
	}

	if client.Exists(context, redisKey).Val() == 0 {
		return userView, errors.New("user does not exist")
	}

	u := client.HGetAll(context, redisKey).Val()
	var view user.View
	var err error
	if view, err = toUserView(u); err != nil {
		return view, err
	}

	return view, nil
}

func (r *RedisClient) CreateUser(context context.Context, userView user.View, skipPasswordGeneration bool) (user.View, error) {
	client := r.client

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
	if skipPasswordGeneration {
		passwordEncrypted = []byte(userView.Password)
	}

	userID := uuid.NewString()
	userData := map[string]interface{}{
		"id":         userID,
		"username":   userView.Username,
		"user_level": userView.UserLevel,
		"password":   string(passwordEncrypted),
		"email":      userView.Email,
		"favorites":  userView.Favorites,
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

	redisUserKey := strings.Join([]string{UserKeyPrefix, userID}, ":")
	_, err := client.HMSet(context, redisUserKey, userData).Result()
	userView.ID = userID
	return userView, err
}

func (r *RedisClient) Authenticate(context context.Context, credential user.Credential) (user.View, string, time.Time, error) {
	userView := user.View{Email: strings.ToLower(credential.Email)}
	Logger.Infof("->Authenticate: user view is %+v", userView)
	var u user.View
	var err error

	Logger.Infof("->Authenticate: by email from credential is %s", credential.Email)
	u, err = r.FindUser(context, FindUserByEmail, userView)
	if err != nil {
		Logger.Errorf("cannot find user by email %s, error: %v", credential.Email, err)
		return u, "", time.Now(), err
	}

	if !credential.WithOAuth {
		pswCompErr := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(credential.Password))
		if pswCompErr != nil { // wrong password
			err = errors.New("wrong password")
			return u, "", time.Now(), err
		}
	}

	lastLoginTime := time.Now()
	u.LastLoginTime = lastLoginTime.Format(time.RFC3339)
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

func (r *RedisClient) SaveUserPlan(context context.Context, userView user.View, planView *user.TravelPlanView) error {
	var findUserErr error
	userView, findUserErr = r.FindUser(context, FindUserByName, userView)
	if findUserErr != nil {
		return findUserErr
	}

	planView.ID = uuid.NewString()
	json_, planSerializationErr := json.Marshal(planView)
	if planSerializationErr != nil {
		return planSerializationErr
	}

	userSavedPlansRedisKey := strings.Join([]string{UserSavedTravelPlansPrefix, "user", userView.ID, "plans"}, ":")
	travelPlanRedisKey := strings.Join([]string{TravelPlanRedisCacheKeyPrefix, planView.OriginalPlanID}, ":")

	if _, err := r.Get().TxPipelined(context, func(pipeliner redis.Pipeliner) error {
		if exists, getPlanErr := pipeliner.SIsMember(context, userSavedPlansRedisKey, travelPlanRedisKey).Result(); getPlanErr != nil || exists {
			if getPlanErr != nil && !errors.Is(getPlanErr, redis.Nil) {
				return getPlanErr
			}
			if exists {
				return fmt.Errorf("travel plan %s is already saved to profile for user %s", planView.ID, userView.ID)
			}
		}

		_, err := pipeliner.SAdd(context, userSavedPlansRedisKey, travelPlanRedisKey).Result()
		if err != nil {
			return err
		}

		savedPlanKey := strings.Join([]string{UserSavedTravelPlanPrefix, "user", userView.ID, "plan", planView.ID}, ":")
		_, err = pipeliner.Set(context, savedPlanKey, json_, 0).Result()
		return err
	}); err != nil {
		return fmt.Errorf("failed to save travel plan for user %s with err: %v", userView.ID, err)
	}

	return nil
}

func (r *RedisClient) DeleteUserPlan(context context.Context, userView user.View, planView user.TravelPlanView) error {
	userView, findUserErr := r.FindUser(context, FindUserByName, userView)
	if findUserErr != nil {
		return findUserErr
	}

	redisKey := strings.Join([]string{UserSavedTravelPlanPrefix, "user", userView.ID, "plan", planView.ID}, ":")
	Logger.Debugf("plan to be deleted: %s", redisKey)
	wg := sync.WaitGroup{}
	wg.Add(1)

	go r.findUserPlan(context, redisKey, &planView, &wg)

	wg.Wait()

	userSavedPlansRedisKey := strings.Join([]string{UserSavedTravelPlansPrefix, "user", userView.ID, "plans"}, ":")
	travelPlanRedisKey := strings.Join([]string{TravelPlanRedisCacheKeyPrefix, planView.OriginalPlanID}, ":")
	if res, originalPlanIdRemovalErr := r.client.SRem(context, userSavedPlansRedisKey, travelPlanRedisKey).Result(); originalPlanIdRemovalErr != nil && originalPlanIdRemovalErr != redis.Nil {
		Logger.Infof("result from removing original key from hash set is %d", res)
		return originalPlanIdRemovalErr
	}

	return r.RemoveKeys(context, []string{redisKey})
}

func (r *RedisClient) userPlanKeysFinder(ctx context.Context, view user.View) chan string {
	out := make(chan string)
	var cursor uint64 = 0
	redisKeysPrefix := strings.Join([]string{UserSavedTravelPlanPrefix, "user", view.ID, "plan"}, ":")
	go func() {
		for {
			var err error
			var keys []string
			keys, cursor, err = r.client.Scan(ctx, cursor, redisKeysPrefix+"*", 100).Result()
			if err != nil {
				break
			}
			for _, key := range keys {
				out <- key
			}
			if cursor == 0 {
				break
			}
		}
		close(out)
	}()
	return out
}

func (r *RedisClient) userPlanFinder(ctx context.Context, in chan string) chan user.TravelPlanView {
	out := make(chan user.TravelPlanView)
	go func() {
		for key := range in {
			plan, err := r.Get().Get(ctx, key).Result()
			if err != nil {
				Logger.Debug(err)
				continue
			}
			view := user.TravelPlanView{}
			if err = json.Unmarshal([]byte(plan), &view); err != nil {
				Logger.Debug(err)
				continue
			}
			out <- view
		}
		close(out)
	}()
	return out
}

func (r *RedisClient) FindUserPlans(ctx context.Context, userView user.View) []user.TravelPlanView {
	in := r.userPlanKeysFinder(ctx, userView)

	var workers [UserPlanWorkerCount]chan user.TravelPlanView
	for idx := range workers {
		workers[idx] = make(chan user.TravelPlanView)
		workers[idx] = r.userPlanFinder(ctx, in)
	}

	var result []user.TravelPlanView
	for view := range merge(workers[:]...) {
		result = append(result, view)
	}
	return result
}

func (r *RedisClient) findUserPlan(context context.Context, redisKey string, view *user.TravelPlanView, wg *sync.WaitGroup) {
	defer wg.Done()
	cachedPlan, err := r.client.Get(context, redisKey).Result()
	if err != nil {
		Logger.Error(err)
		return
	}
	utils.LogErrorWithLevel(json.Unmarshal([]byte(cachedPlan), view), utils.LogError)
}

// generates a code for backend to find user ID when user clicks on the link
func (r *RedisClient) saveEmailPasswordResetCode(ctx context.Context, view user.View) (string, error) {
	code := uuid.NewString()
	key := "password_reset:" + code
	if _, err := r.client.HSet(ctx, key, "user_email", view.Email, "claimed", false).Result(); err != nil {
		return "", err
	}
	// set 2 hour expiration time
	r.client.Expire(ctx, key, 2*time.Hour)
	return code, nil
}

func (r *RedisClient) saveUserEmailVerificationCode(ctx context.Context, view user.View) (string, error) {
	if len(view.Email) == 0 {
		return "", errors.New("email address cannot be empty")
	}
	c := r.client
	// overwrites existing verification code
	// the code serves as a temporary user ID
	code := uuid.NewString()
	if _, err := c.HSet(ctx, EmailVerificationCodes, view.Email, code).Result(); err != nil {
		return "", err
	}
	passwordEncrypted, _ := bcrypt.GenerateFromPassword([]byte(view.Password), bcrypt.DefaultCost)
	if _, err := c.HSet(ctx, "temp_user:"+code, "id", code, "email", view.Email, "username", view.Username, "password", passwordEncrypted, "user_level", view.UserLevel).Result(); err != nil {
		return "", err
	}
	// set 6 hour expiration time
	c.Expire(ctx, "temp_user:"+code, 6*time.Hour)
	return code, nil
}

func (r *RedisClient) CreateUserOnEmailVerified(ctx context.Context, tmpUserID string) error {
	c := r.client
	var tmpUserData map[string]string
	var err error
	if tmpUserData, err = c.HGetAll(ctx, "temp_user:"+tmpUserID).Result(); err != nil {
		return err
	}
	var view user.View
	view, err = toUserView(tmpUserData)
	if err != nil {
		Logger.Errorf("error converting temp user data to view %s", err.Error())
	}
	if _, err = r.CreateUser(ctx, view, true); err != nil {
		return err
	}
	return nil
}

type UserFeedback struct {
	UserId   string `json:"user_id"`
	PlanSpec string `json:"plan_spec"`
	PlanId   string `json:"plan_id"`
	Like     bool   `json:"like"`
}

func (r *RedisClient) UserFeedback(ctx context.Context, fb *UserFeedback) error {
	if fb.Like {
		Logger.Debugf("->RedisClient.UserFeedback: user %s likes plan %s", fb.UserId, fb.PlanId)
		return nil
	}

	userPlansSSKey := strings.Join([]string{"user", fb.UserId, fb.PlanSpec}, ":")
	Logger.Debugf("->RedisClient.UserFeedback: looking for key %s", userPlansSSKey)
	if exists, err := r.Get().Exists(ctx, userPlansSSKey).Result(); err != nil {
		return err
	} else if exists == 0 {
		return fmt.Errorf("failed to find user plans with key: %s", userPlansSSKey)
	}

	if err := r.Get().ZRem(ctx, userPlansSSKey, fb.PlanId).Err(); err != nil {
		return err
	}
	return nil
}
