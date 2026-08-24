package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// JWTConfigFromEnv reads JWT config from environment variables.
// Panics if JWT_SECRET_KEY is not set, to prevent running with a publicly known key.
func JWTConfigFromEnv() JWTConfig {
	accessTTL := getEnvAsDuration("JWT_ACCESS_TTL_HOURS", 0.25) // 15 minutes default for a health app
	refreshTTL := getEnvAsDuration("JWT_REFRESH_TTL_HOURS", 30*24) // 30 days

	secret := os.Getenv("JWT_SECRET_KEY")
	if secret == "" {
		log.Fatal("JWT_SECRET_KEY environment variable is required")
	}

	return JWTConfig{
		SecretKey:       secret,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
	}
}

// Claims represents the JWT claims.
type Claims struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	SessionID uuid.UUID `json:"session_id,omitempty"`
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a new access token for the user.
// sessionID ties the token to a single login session so logout (or account
// deletion) can revoke the whole session family via the session cache.
func GenerateAccessToken(cfg JWTConfig, userID, sessionID uuid.UUID, email string) (string, error) {
	claims := Claims{
		UserID:    userID,
		Email:     email,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.SecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return tokenString, nil
}

// GenerateRefreshToken creates a cryptographically secure refresh token.
func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// ValidateAccessToken validates and parses an access token.
func ValidateAccessToken(cfg JWTConfig, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.SecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvAsDuration(key string, defaultHours float64) time.Duration {
	if v := os.Getenv(key); v != "" {
		if hours, err := strconv.ParseFloat(v, 64); err == nil {
			return time.Duration(hours * float64(time.Hour))
		}
	}
	return time.Duration(defaultHours * float64(time.Hour))
}
