package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateAccessToken(t *testing.T) {
	cfg := JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  7 * 24 * time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}

	userID := uuid.New()
	email := "test@example.com"

	token, err := GenerateAccessToken(cfg, userID, email)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("Generated token is empty")
	}

	t.Logf("Generated token: %s", token[:20]+"...")
}

func TestValidateAccessToken(t *testing.T) {
	cfg := JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  7 * 24 * time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}

	userID := uuid.New()
	email := "test@example.com"

	// Generate token
	token, err := GenerateAccessToken(cfg, userID, email)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	// Validate token
	claims, err := ValidateAccessToken(cfg, token)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	// Check claims
	if claims.UserID != userID {
		t.Errorf("Expected UserID %v, got %v", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("Expected Email %s, got %s", email, claims.Email)
	}
}

func TestValidateAccessToken_InvalidToken(t *testing.T) {
	cfg := JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  7 * 24 * time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}

	// Try to validate invalid token
	_, err := ValidateAccessToken(cfg, "invalid-token")
	if err == nil {
		t.Fatal("Expected error for invalid token, got nil")
	}

	t.Logf("Got expected error: %v", err)
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
	cfg1 := JWTConfig{
		SecretKey:       "secret-key-1",
		AccessTokenTTL:  7 * 24 * time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}
	cfg2 := JWTConfig{
		SecretKey:       "secret-key-2",
		AccessTokenTTL:  7 * 24 * time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}

	userID := uuid.New()
	email := "test@example.com"

	// Generate with key 1
	token, err := GenerateAccessToken(cfg1, userID, email)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	// Validate with key 2 should fail
	_, err = ValidateAccessToken(cfg2, token)
	if err == nil {
		t.Fatal("Expected error for wrong secret key, got nil")
	}

	t.Logf("Got expected error: %v", err)
}

func TestGenerateRefreshToken(t *testing.T) {
	token1 := GenerateRefreshToken()
	token2 := GenerateRefreshToken()

	if token1 == "" {
		t.Fatal("Generated refresh token is empty")
	}
	if token2 == "" {
		t.Fatal("Generated refresh token is empty")
	}
	if token1 == token2 {
		t.Fatal("Two refresh tokens should be different")
	}

	t.Logf("Generated refresh tokens: %s, %s", token1[:8], token2[:8])
}
