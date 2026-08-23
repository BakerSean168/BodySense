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

	token, err := GenerateAccessToken(cfg, userID, uuid.New(), email)
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
	sessionID := uuid.New()
	email := "test@example.com"

	// Generate token
	token, err := GenerateAccessToken(cfg, userID, sessionID, email)
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
	if claims.SessionID != sessionID {
		t.Errorf("Expected SessionID %v, got %v", sessionID, claims.SessionID)
	}
}

func TestGenerateAccessTokenCarriesSessionID(t *testing.T) {
	cfg := JWTConfig{
		SecretKey:      "test-secret-key",
		AccessTokenTTL: 15 * time.Minute,
	}
	userID := uuid.New()
	sessionID := uuid.New()

	token, err := GenerateAccessToken(cfg, userID, sessionID, "test@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}
	claims, err := ValidateAccessToken(cfg, token)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if claims.SessionID != sessionID {
		t.Fatalf("SessionID = %v, want %v", claims.SessionID, sessionID)
	}
}

func TestGetEnvAsDurationParsesFractionalHours(t *testing.T) {
	t.Setenv("JWT_ACCESS_TTL_HOURS", "0.25")
	ttl := getEnvAsDuration("JWT_ACCESS_TTL_HOURS", 2)
	if want := 15 * time.Minute; ttl != want {
		t.Fatalf("ttl = %v, want %v", ttl, want)
	}
}

func TestGetEnvAsDurationDefaultIsQuarterHour(t *testing.T) {
	t.Setenv("JWT_ACCESS_TTL_HOURS", "")
	ttl := getEnvAsDuration("JWT_ACCESS_TTL_HOURS", 0.25)
	if want := 15 * time.Minute; ttl != want {
		t.Fatalf("ttl = %v, want %v", ttl, want)
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
	token, err := GenerateAccessToken(cfg1, userID, uuid.New(), email)
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
	token1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}
	token2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

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
