package redis_client_mocks

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/user"
)

func TestRedisClient_NewPAT(t *testing.T) {
	// Setup: Create a test user first
	testUser := user.View{
		Username: "pat_test_user",
		Email:    "pat_test@example.com",
		Password: "test_password",
	}
	createdUser, err := RedisClient.CreateUser(RedisContext, testUser, false)
	if err != nil {
		t.Fatal("Failed to create test user:", err)
	}

	type args struct {
		ctx      context.Context
		name     string
		userId   string
		token    string
		validity time.Duration
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Should create PAT successfully with valid parameters",
			args: args{
				ctx:      context.Background(),
				name:     "test-token",
				userId:   createdUser.ID,
				token:    "secure_token_hash_123",
				validity: 24 * time.Hour,
			},
			wantErr: false,
		},
		{
			name: "Should create PAT with long validity period",
			args: args{
				ctx:      context.Background(),
				name:     "long-term-token",
				userId:   createdUser.ID,
				token:    "long_term_hash_456",
				validity: 365 * 24 * time.Hour, // 1 year
			},
			wantErr: false,
		},
		{
			name: "Should handle empty token name",
			args: args{
				ctx:      context.Background(),
				name:     "",
				userId:   createdUser.ID,
				token:    "empty_name_hash_789",
				validity: 24 * time.Hour,
			},
			wantErr: false, // Empty name is allowed
		},
		{
			name: "Should reject duplicate token names",
			args: args{
				ctx:      context.Background(),
				name:     "test-token", // Same name as first test
				userId:   createdUser.ID,
				token:    "duplicate_name_hash",
				validity: 24 * time.Hour,
			},
			wantErr: true, // Should fail due to duplicate name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := RedisClient.NewPAT(tt.args.ctx, tt.args.name, tt.args.userId, tt.args.token, tt.args.validity)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)

				// Verify response contains expected data
				assert.NotEmpty(t, response.TokenID)
				assert.Equal(t, tt.args.token, response.TokenHash, "Hash should be returned once during creation")
				assert.Equal(t, tt.args.name, response.Name)
				assert.NotEmpty(t, response.CreatedAt)
				assert.NotEmpty(t, response.ExpiresAt)
				assert.NotEmpty(t, response.ExpiresIn)

				// Verify human-readable formats
				if tt.args.validity == 24*time.Hour {
					assert.Equal(t, "24 hours", response.ExpiresIn)
				} else if tt.args.validity == 365*24*time.Hour {
					assert.Equal(t, "1 year", response.ExpiresIn)
				}

				// Verify token was added to user's metadata list (secure way)
				userTokens, err := RedisClient.ListUserPATMetadata(tt.args.ctx, tt.args.userId)
				assert.NoError(t, err)

				found := false
				for _, userToken := range userTokens {
					if userToken.Id == response.TokenID {
						assert.Equal(t, tt.args.name, userToken.Name)
						assert.Equal(t, tt.args.userId, userToken.UserId)
						assert.True(t, userToken.IsActive)
						// Hash should NOT be accessible in metadata
						found = true
						break
					}
				}
				assert.True(t, found, "Token should be in user's metadata list")
			}
		})
	}
}

func TestRedisClient_NewPAT_ReuseExpiredTokenName(t *testing.T) {
	// Setup: Create a test user
	testUser := user.View{
		Username: "expired_token_test_user",
		Email:    "expired_test@example.com",
		Password: "test_password",
	}
	createdUser, err := RedisClient.CreateUser(RedisContext, testUser, false)
	if err != nil {
		t.Fatal("Failed to create test user:", err)
	}

	ctx := context.Background()
	tokenName := "reusable-token"

	// Step 1: Create an expired token
	expiredResponse, err := RedisClient.NewPAT(ctx, tokenName, createdUser.ID, "expired_token_hash", -1*time.Hour) // Already expired
	assert.NoError(t, err)
	assert.NotNil(t, expiredResponse)

	// Step 2: Try to create a new token with the same name - should succeed because the previous one is expired
	newResponse, err := RedisClient.NewPAT(ctx, tokenName, createdUser.ID, "new_token_hash", 24*time.Hour)
	assert.NoError(t, err)
	assert.NotNil(t, newResponse)
	assert.Equal(t, tokenName, newResponse.Name)
	assert.Equal(t, "new_token_hash", newResponse.TokenHash)
	assert.NotEqual(t, expiredResponse.TokenID, newResponse.TokenID, "Should create a new token with different ID")

	// Step 3: Verify that trying to create another token with the same name fails (because the new one is still valid)
	failResponse, err := RedisClient.NewPAT(ctx, tokenName, createdUser.ID, "should_fail_hash", 24*time.Hour)
	assert.Error(t, err)
	assert.Nil(t, failResponse)
	assert.Contains(t, err.Error(), "already exists")

	// Step 4: Verify only one token with this name exists in metadata (the new one)
	metadata, err := RedisClient.ListUserPATMetadata(ctx, createdUser.ID)
	assert.NoError(t, err)

	activeTokensWithName := 0
	for _, token := range metadata {
		if token.Name == tokenName && token.IsActive {
			activeTokensWithName++
		}
	}
	assert.Equal(t, 1, activeTokensWithName, "Should have exactly one active token with the reused name")
}

func TestRedisClient_RevokePAT(t *testing.T) {
	// Setup: Create a test user and token
	testUser := user.View{
		Username: "revoke_test_user",
		Email:    "revoke_test@example.com",
		Password: "test_password",
	}
	createdUser, err := RedisClient.CreateUser(RedisContext, testUser, false)
	if err != nil {
		t.Fatal("Failed to create test user:", err)
	}

	// Create a token to revoke
	response, err := RedisClient.NewPAT(RedisContext, "token-to-revoke", createdUser.ID, "revoke_test_hash", 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create test token:", err)
	}
	tokenId := response.TokenID

	type args struct {
		ctx     context.Context
		userId  string
		tokenId string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Should revoke PAT successfully",
			args: args{
				ctx:     context.Background(),
				userId:  createdUser.ID,
				tokenId: tokenId,
			},
			wantErr: false,
		},
		{
			name: "Should return error for non-existent token",
			args: args{
				ctx:     context.Background(),
				userId:  createdUser.ID,
				tokenId: "non-existent-token-id",
			},
			wantErr: true,
		},
		{
			name: "Should return error for empty token ID",
			args: args{
				ctx:     context.Background(),
				userId:  createdUser.ID,
				tokenId: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RedisClient.RevokePAT(tt.args.ctx, tt.args.userId, tt.args.tokenId)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify token validation fails (secure way to check revocation)
				validToken, err := RedisClient.ValidatePAT(tt.args.ctx, tt.args.tokenId)
				assert.Error(t, err)
				assert.Equal(t, redis.Nil, err)
				assert.Nil(t, validToken)

				// Verify token is removed from user's metadata list
				userTokens, err := RedisClient.ListUserPATMetadata(tt.args.ctx, tt.args.userId)
				assert.NoError(t, err)

				for _, userToken := range userTokens {
					assert.NotEqual(t, tt.args.tokenId, userToken.Id, "Revoked token should be removed from user's metadata list")
				}
			}
		})
	}
}

func TestRedisClient_RevokePATByName(t *testing.T) {
	// Setup: Create a test user and token
	testUser := user.View{
		Username: "revoke_by_name_test_user",
		Email:    "revoke_by_name_test@example.com",
		Password: "test_password",
	}
	createdUser, err := RedisClient.CreateUser(RedisContext, testUser, false)
	if err != nil {
		t.Fatal("Failed to create test user:", err)
	}

	// Create a token to revoke
	tokenName := "test-token-to-revoke"
	response, err := RedisClient.NewPAT(context.Background(), tokenName, createdUser.ID, "token_hash_123", 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create test token:", err)
	}

	type args struct {
		ctx       context.Context
		userId    string
		tokenName string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Should revoke PAT by name successfully",
			args: args{
				ctx:       context.Background(),
				userId:    createdUser.ID,
				tokenName: tokenName,
			},
			wantErr: false,
		},
		{
			name: "Should return error for non-existent token name",
			args: args{
				ctx:       context.Background(),
				userId:    createdUser.ID,
				tokenName: "non-existent-token",
			},
			wantErr: true,
		},
		{
			name: "Should return error for empty token name",
			args: args{
				ctx:       context.Background(),
				userId:    createdUser.ID,
				tokenName: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RedisClient.RevokePATByName(tt.args.ctx, tt.args.userId, tt.args.tokenName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify the token is revoked by trying to validate it
				validatedToken, err := RedisClient.ValidatePAT(tt.args.ctx, response.TokenID)
				assert.Error(t, err)
				assert.Nil(t, validatedToken)

				// Verify token is no longer in user's active metadata
				metadata, err := RedisClient.ListUserPATMetadata(tt.args.ctx, tt.args.userId)
				assert.NoError(t, err)

				for _, token := range metadata {
					if token.Name == tt.args.tokenName {
						assert.False(t, token.IsActive, "Token should be inactive after revocation")
					}
				}
			}
		})
	}
}

func TestRedisClient_ValidatePAT(t *testing.T) {
	// Setup: Create a test user
	testUser := user.View{
		Username: "validate_test_user",
		Email:    "validate_test@example.com",
		Password: "test_password",
	}
	createdUser, err := RedisClient.CreateUser(RedisContext, testUser, false)
	if err != nil {
		t.Fatal("Failed to create test user:", err)
	}

	// Create valid token
	validResponse, err := RedisClient.NewPAT(RedisContext, "valid-token", createdUser.ID, "valid_hash", 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create valid token:", err)
	}
	validTokenId := validResponse.TokenID

	// Create expired token
	expiredResponse, err := RedisClient.NewPAT(RedisContext, "expired-token", createdUser.ID, "expired_hash", -1*time.Hour)
	if err != nil {
		t.Fatal("Failed to create expired token:", err)
	}
	expiredTokenId := expiredResponse.TokenID

	// Create and revoke token
	revokedResponse, err := RedisClient.NewPAT(RedisContext, "revoked-token", createdUser.ID, "revoked_hash", 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create revoked token:", err)
	}
	revokedTokenId := revokedResponse.TokenID
	err = RedisClient.RevokePAT(RedisContext, createdUser.ID, revokedTokenId)
	if err != nil {
		t.Fatal("Failed to revoke token:", err)
	}

	type args struct {
		ctx     context.Context
		tokenId string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Should validate active token successfully",
			args: args{
				ctx:     context.Background(),
				tokenId: validTokenId,
			},
			wantErr: false,
		},
		{
			name: "Should reject expired token",
			args: args{
				ctx:     context.Background(),
				tokenId: expiredTokenId,
			},
			wantErr: true,
		},
		{
			name: "Should reject revoked token",
			args: args{
				ctx:     context.Background(),
				tokenId: revokedTokenId,
			},
			wantErr: true,
		},
		{
			name: "Should reject non-existent token",
			args: args{
				ctx:     context.Background(),
				tokenId: "non-existent-token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := RedisClient.ValidatePAT(tt.args.ctx, tt.args.tokenId)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, token)
				assert.True(t, token.Valid())
				assert.Equal(t, tt.args.tokenId, token.Id)
			}
		})
	}
}

func TestRedisClient_ListUserPATMetadata(t *testing.T) {
	// Setup: Create a test user
	testUser := user.View{
		Username: "list_test_user",
		Email:    "list_test@example.com",
		Password: "test_password",
	}
	createdUser, err := RedisClient.CreateUser(RedisContext, testUser, false)
	if err != nil {
		t.Fatal("Failed to create test user:", err)
	}

	// Create multiple tokens
	token1Response, err := RedisClient.NewPAT(RedisContext, "token-1", createdUser.ID, "hash_1", 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create token 1:", err)
	}
	token1Id := token1Response.TokenID

	token2Response, err := RedisClient.NewPAT(RedisContext, "token-2", createdUser.ID, "hash_2", 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create token 2:", err)
	}
	token2Id := token2Response.TokenID

	token3Response, err := RedisClient.NewPAT(RedisContext, "token-3", createdUser.ID, "hash_3", 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create token 3:", err)
	}
	token3Id := token3Response.TokenID

	// Revoke one token
	err = RedisClient.RevokePAT(RedisContext, createdUser.ID, token2Id)
	if err != nil {
		t.Fatal("Failed to revoke token 2:", err)
	}

	type args struct {
		ctx    context.Context
		userId string
	}
	tests := []struct {
		name          string
		args          args
		expectedCount int
		wantErr       bool
	}{
		{
			name: "Should list user's tokens correctly",
			args: args{
				ctx:    context.Background(),
				userId: createdUser.ID,
			},
			expectedCount: 2, // token1 and token3 (token2 was revoked)
			wantErr:       false,
		},
		{
			name: "Should return empty list for user with no tokens",
			args: args{
				ctx:    context.Background(),
				userId: "non-existent-user",
			},
			expectedCount: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := RedisClient.ListUserPATMetadata(tt.args.ctx, tt.args.userId)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, tokens, tt.expectedCount)

				if tt.expectedCount > 0 {
					// Verify the returned tokens are the expected ones
					tokenIds := make([]string, len(tokens))
					for i, token := range tokens {
						tokenIds[i] = token.Id
						assert.Equal(t, createdUser.ID, token.UserId)
						assert.True(t, token.IsActive) // All remaining tokens should be active
					}

					if tt.expectedCount == 2 {
						assert.Contains(t, tokenIds, token1Id)
						assert.Contains(t, tokenIds, token3Id)
						assert.NotContains(t, tokenIds, token2Id) // Revoked token should not be listed
					}
				}
			}
		})
	}
}

func TestTokenRecord_Valid(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		token       iowrappers.TokenRecord
		wantIsValid bool
	}{
		{
			name: "Should be valid for active token",
			token: iowrappers.TokenRecord{
				Id:        "valid-token",
				ExpiresAt: &[]time.Time{now.Add(24 * time.Hour)}[0],
				RevokedAt: nil,
			},
			wantIsValid: true,
		},
		{
			name: "Should be invalid for expired token",
			token: iowrappers.TokenRecord{
				Id:        "expired-token",
				ExpiresAt: &[]time.Time{now.Add(-1 * time.Hour)}[0],
				RevokedAt: nil,
			},
			wantIsValid: false,
		},
		{
			name: "Should be invalid for revoked token",
			token: iowrappers.TokenRecord{
				Id:        "revoked-token",
				ExpiresAt: &[]time.Time{now.Add(24 * time.Hour)}[0],
				RevokedAt: &[]time.Time{now.Add(-1 * time.Hour)}[0],
			},
			wantIsValid: false,
		},
		{
			name: "Should be invalid for token with nil expiration",
			token: iowrappers.TokenRecord{
				Id:        "nil-expiry-token",
				ExpiresAt: nil,
				RevokedAt: nil,
			},
			wantIsValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.token.Valid()
			assert.Equal(t, tt.wantIsValid, isValid)
		})
	}
}

func TestPATSecurity_TokenHashNotExposed(t *testing.T) {
	// Setup: Create a test user and token
	testUser := user.View{
		Username: "security_test_user",
		Email:    "security@example.com",
		Password: "test_password",
	}
	createdUser, err := RedisClient.CreateUser(RedisContext, testUser, false)
	if err != nil {
		t.Fatal("Failed to create test user:", err)
	}

	// Create a token with a known hash
	secretHash := "this_should_never_be_exposed_12345"
	response, err := RedisClient.NewPAT(RedisContext, "security-test-token", createdUser.ID, secretHash, 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create test token:", err)
	}
	tokenId := response.TokenID

	// Verify the token hash is returned once during creation
	assert.Equal(t, secretHash, response.TokenHash, "Hash should be returned during token creation")
	assert.Equal(t, "security-test-token", response.Name)
	assert.Equal(t, "24 hours", response.ExpiresIn)

	t.Run("Token hash should not be accessible through metadata API", func(t *testing.T) {
		// Get user's token metadata
		metadata, err := RedisClient.ListUserPATMetadata(RedisContext, createdUser.ID)
		assert.NoError(t, err)
		assert.Len(t, metadata, 1)

		token := metadata[0]
		assert.Equal(t, tokenId, token.Id)
		assert.Equal(t, "security-test-token", token.Name)
		assert.Equal(t, createdUser.ID, token.UserId)
		assert.True(t, token.IsActive)

		// Verify the hash is NOT exposed in the metadata struct
		// TokenMetadata should not have a Hash field at all
		assert.NotNil(t, token.CreatedAt)
		assert.NotNil(t, token.ExpiresAt)
		assert.Nil(t, token.RevokedAt)
	})

	t.Run("Token should only be retrievable for server-side validation", func(t *testing.T) {
		// ValidatePAT is the only way to access token details (for server auth)
		token, err := RedisClient.ValidatePAT(RedisContext, tokenId)
		assert.NoError(t, err)
		assert.NotNil(t, token)
		assert.Equal(t, secretHash, token.Hash) // Hash accessible only for validation
		assert.True(t, token.Valid())
	})

	t.Run("Token becomes inaccessible after revocation", func(t *testing.T) {
		// Revoke the token
		err := RedisClient.RevokePAT(RedisContext, createdUser.ID, tokenId)
		assert.NoError(t, err)

		// Should not be validatable
		token, err := RedisClient.ValidatePAT(RedisContext, tokenId)
		assert.Error(t, err)
		assert.Equal(t, redis.Nil, err)
		assert.Nil(t, token)

		// Should not appear in metadata list
		metadata, err := RedisClient.ListUserPATMetadata(RedisContext, createdUser.ID)
		assert.NoError(t, err)
		assert.Len(t, metadata, 0) // Empty list after revocation
	})
}