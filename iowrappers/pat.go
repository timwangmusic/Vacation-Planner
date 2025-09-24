package iowrappers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type TokenRecord struct {
	Id        string     `json:"id"`
	Name      string     `json:"name"`
	Hash      string     `json:"hash"`
	UserId    string     `json:"user_id"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}

// TokenMetadata contains non-sensitive token information for user management
type TokenMetadata struct {
	Id        string     `json:"id"`
	Name      string     `json:"name"`
	UserId    string     `json:"user_id"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	IsActive  bool       `json:"is_active"`
}

// NewPATResponse contains the token information shown once during creation
type NewPATResponse struct {
	TokenID   string `json:"token_id"`
	TokenHash string `json:"token_hash"` // Only shown once!
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"` // Human-readable format
	ExpiresAt string `json:"expires_at"` // Human-readable format
	ExpiresIn string `json:"expires_in"` // Human-readable duration (e.g., "24 hours")
}

func (tr *TokenRecord) Valid() bool {
	now := time.Now()
	return tr.ExpiresAt != nil && now.Before(*tr.ExpiresAt) && tr.RevokedAt == nil
}

// formatDuration converts a time.Duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}

	// For less than 48 hours, show in hours
	if d < 48*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}

	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	if days < 365 {
		return fmt.Sprintf("%d days", days)
	}

	years := days / 365
	remainingDays := days % 365
	if years == 1 && remainingDays == 0 {
		return "1 year"
	}
	if remainingDays == 0 {
		return fmt.Sprintf("%d years", years)
	}
	return fmt.Sprintf("%d years, %d days", years, remainingDays)
}

func (r *RedisClient) NewPAT(ctx context.Context, name, userId, token string, valid time.Duration) (*NewPATResponse, error) {
	now := time.Now()
	expiresAt := now.Add(valid)
	record := TokenRecord{
		Id:        uuid.NewString(),
		Name:      name,
		Hash:      token,
		UserId:    userId,
		Scopes:    nil,
		CreatedAt: now,
		ExpiresAt: &expiresAt,
		RevokedAt: nil,
	}

	tokenKey := strings.Join([]string{"pat", record.Id}, ":")
	// maps token name to ID
	userTokensKey := strings.Join([]string{"user_pats", userId}, ":")

	save, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	// Use transaction to atomically create token and add to user's token list
	err = r.Get().Watch(ctx, func(tx *redis.Tx) error {
		existingTokenId, err := tx.HGet(ctx, userTokensKey, name).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		if existingTokenId != "" {
			existingTokenKey := strings.Join([]string{"pat", existingTokenId}, ":")
			oldRecord := TokenRecord{}
			if recordStr, err := tx.Get(ctx, existingTokenKey).Result(); err != nil {
				return err
			} else if err := json.Unmarshal([]byte(recordStr), &oldRecord); err != nil {
				return err
			}

			if oldRecord.Valid() {
				return fmt.Errorf("personal access token with same name %s already exists", name)
			}
		}
		// Store the token record
		if err := tx.Set(ctx, tokenKey, string(save), 0).Err(); err != nil {
			return err
		}

		// Add token ID to user's token set
		return tx.HSet(ctx, userTokensKey, name, record.Id).Err()
	}, tokenKey, userTokensKey)

	if err != nil {
		return nil, err
	}

	// Create user-friendly response with readable formats
	response := &NewPATResponse{
		TokenID:   record.Id,
		TokenHash: token, // This is the only time the user sees the token!
		Name:      name,
		CreatedAt: now.Format("January 2, 2006 at 3:04 PM MST"),
		ExpiresAt: expiresAt.Format("January 2, 2006 at 3:04 PM MST"),
		ExpiresIn: formatDuration(valid),
	}

	return response, nil
}

func (r *RedisClient) RevokePAT(ctx context.Context, userId, tokenId string) error {
	tokenKey := strings.Join([]string{"pat", tokenId}, ":")
	userTokensKey := strings.Join([]string{"user_pats", userId}, ":")

	val, err := r.Get().Get(ctx, tokenKey).Result()
	if err != nil {
		return err
	}
	var token TokenRecord
	if err = json.Unmarshal([]byte(val), &token); err != nil {
		return err
	}

	if err = r.Get().Watch(ctx, func(tx *redis.Tx) error {
		t := time.Now()
		token.RevokedAt = &t

		// Marshal the updated token record
		updatedToken, err := json.Marshal(token)
		if err != nil {
			return err
		}

		if err = tx.Set(ctx, tokenKey, string(updatedToken), 0).Err(); err != nil {
			return err
		}

		return tx.HDel(ctx, userTokensKey, token.Name).Err()
	}, tokenKey, userTokensKey); err != nil {
		return err
	}
	return nil
}

// RevokePATByName revokes a personal access token by name (user-friendly approach)
func (r *RedisClient) RevokePATByName(ctx context.Context, userId, tokenName string) error {
	userTokensKey := strings.Join([]string{"user_pats", userId}, ":")

	// Get token ID from the name->ID hash
	tokenId, err := r.Get().HGet(ctx, userTokensKey, tokenName).Result()
	if err == redis.Nil {
		return fmt.Errorf("token with name '%s' not found", tokenName)
	} else if err != nil {
		return err
	}

	// Use existing RevokePAT method with the found token ID
	return r.RevokePAT(ctx, userId, tokenId)
}

// validatePATInternal is a private method for internal validation (server-side auth)
func (r *RedisClient) validatePATInternal(ctx context.Context, tokenId string) (*TokenRecord, error) {
	tokenKey := strings.Join([]string{"pat", tokenId}, ":")
	val, err := r.Get().Get(ctx, tokenKey).Result()
	if err != nil {
		return nil, err
	}

	var token TokenRecord
	if err = json.Unmarshal([]byte(val), &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// ValidatePAT validates a personal access token for authentication (server-side only)
func (r *RedisClient) ValidatePAT(ctx context.Context, tokenId string) (*TokenRecord, error) {
	token, err := r.validatePATInternal(ctx, tokenId)
	if err != nil {
		return nil, err
	}

	if !token.Valid() {
		return nil, redis.Nil // Token is expired or revoked
	}

	return token, nil
}

// ListUserPATMetadata retrieves metadata (no hash) for all user's tokens
func (r *RedisClient) ListUserPATMetadata(ctx context.Context, userId string) ([]*TokenMetadata, error) {
	userTokensKey := strings.Join([]string{"user_pats", userId}, ":")
	// Get name->ID mapping from hash
	tokenNameToId, err := r.Get().HGetAll(ctx, userTokensKey).Result()
	if err != nil {
		return nil, err
	}

	var metadata []*TokenMetadata
	for tokenName, tokenId := range tokenNameToId {
		// Get token by ID using existing method
		token, err := r.validatePATInternal(ctx, tokenId)
		if err != nil {
			// Skip tokens that can't be retrieved (maybe deleted)
			Logger.Warnf("Failed to retrieve token %s for user %s: %v", tokenName, userId, err)
			continue
		}

		// Convert to metadata (no hash exposed)
		meta := &TokenMetadata{
			Id:        token.Id,
			Name:      token.Name,
			UserId:    token.UserId,
			Scopes:    token.Scopes,
			CreatedAt: token.CreatedAt,
			ExpiresAt: token.ExpiresAt,
			RevokedAt: token.RevokedAt,
			IsActive:  token.Valid(),
		}
		metadata = append(metadata, meta)
	}

	return metadata, nil
}
