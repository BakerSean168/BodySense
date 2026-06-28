package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService handles authentication business logic.
type AuthService struct {
	userRepo  *repository.UserRepository
	jwtConfig auth.JWTConfig
}

// NewAuthService creates a new AuthService.
func NewAuthService(userRepo *repository.UserRepository, jwtConfig auth.JWTConfig) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtConfig: jwtConfig,
	}
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	// Check if email already exists
	exists, err := s.userRepo.EmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		return nil, errors.New("registration failed")
	}

	// Hash password with bcrypt (cost >= 12)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &model.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate tokens
	return s.generateTokens(ctx, user)
}

// Login authenticates a user and returns tokens.
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid email or password")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	// Update last login time
	if err := s.userRepo.UpdateLastLoginAt(ctx, user.ID); err != nil {
		// Log error but don't fail login
		fmt.Printf("Warning: failed to update last login time: %v\n", err)
	}

	// Generate tokens
	return s.generateTokens(ctx, user)
}

// RefreshToken refreshes an access token using a refresh token.
func (s *AuthService) RefreshToken(ctx context.Context, req dto.RefreshRequest) (*dto.AuthResponse, error) {
	// Get user ID from Redis
	redisClient := database.RedisClient
	key := fmt.Sprintf("refresh_token:%s", req.RefreshToken)

	userIDStr, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	// Delete old refresh token
	redisClient.Del(ctx, key)

	// Parse user ID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Find user
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Generate new tokens
	return s.generateTokens(ctx, user)
}

// generateTokens generates access and refresh tokens for a user.
func (s *AuthService) generateTokens(ctx context.Context, user *model.User) (*dto.AuthResponse, error) {
	// Generate access token
	accessToken, err := auth.GenerateAccessToken(s.jwtConfig, user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store refresh token in Redis
	redisClient := database.RedisClient
	key := fmt.Sprintf("refresh_token:%s", refreshToken)
	if err := redisClient.Set(ctx, key, user.ID.String(), s.jwtConfig.RefreshTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwtConfig.AccessTokenTTL.Seconds()),
	}, nil
}

// InvalidateRefreshToken invalidates a refresh token.
func (s *AuthService) InvalidateRefreshToken(ctx context.Context, refreshToken string) error {
	redisClient := database.RedisClient
	key := fmt.Sprintf("refresh_token:%s", refreshToken)
	return redisClient.Del(ctx, key).Err()
}

// CleanupExpiredTokens removes expired refresh tokens (can be called periodically).
func (s *AuthService) CleanupExpiredTokens(ctx context.Context) error {
	// Redis automatically removes expired keys, but this can be used for manual cleanup
	return nil
}

// getRefreshTokenTTL returns the TTL for refresh tokens.
func (s *AuthService) getRefreshTokenTTL() time.Duration {
	return s.jwtConfig.RefreshTokenTTL
}
