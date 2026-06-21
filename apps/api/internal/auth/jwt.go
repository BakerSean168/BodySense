package auth

import (
	"fmt"
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
func JWTConfigFromEnv() JWTConfig {
	accessTTL := getEnvAsDuration("JWT_ACCESS_TTL_HOURS", 7*24) // 7 days
	refreshTTL := getEnvAsDuration("JWT_REFRESH_TTL_HOURS", 30*24) // 30 days

	return JWTConfig{
		SecretKey:       getEnv("JWT_SECRET_KEY", "your-secret-key-change-in-production"),
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
	}
}

// Claims represents the JWT claims.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a new access token for the user.
func GenerateAccessToken(cfg JWTConfig, userID uuid.UUID, email string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
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

// GenerateRefreshToken creates a new refresh token.
func GenerateRefreshToken() string {
	return uuid.New().String()
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

func getEnvAsDuration(key string, defaultHours int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if hours, err := strconv.Atoi(v); err == nil {
			return time.Duration(hours) * time.Hour
		}
	}
	return time.Duration(defaultHours) * time.Hour
}
