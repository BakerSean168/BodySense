package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/cache"
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
	userRepo     *repository.UserRepository
	jwtConfig    auth.JWTConfig
	sessionCache *cache.UserSessionCache
}

// NewAuthService creates a new AuthService.
func NewAuthService(userRepo *repository.UserRepository, jwtConfig auth.JWTConfig, sessionCache *cache.UserSessionCache) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		jwtConfig:    jwtConfig,
		sessionCache: sessionCache,
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

	// Generate tokens (also writes session cache)
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
		log.Printf("Warning: failed to update last login time: %v", err)
	}

	// Generate tokens (also writes session cache)
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

	// Find user — if user no longer exists, clean up and reject
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User was deleted — clear stale session cache
			_ = s.sessionCache.Delete(ctx, userID)
			return nil, errors.New("user no longer exists")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Generate new tokens (also writes session cache)
	return s.generateTokens(ctx, user)
}

// Logout invalidates a refresh token and clears the user's session cache.
// Looks up the user ID from the refresh token in Redis before cleaning up.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	redisClient := database.RedisClient
	refreshKey := fmt.Sprintf("refresh_token:%s", refreshToken)

	// Look up user ID from refresh token (before deleting it)
	userIDStr, err := redisClient.Get(ctx, refreshKey).Result()
	if err != nil {
		// Refresh token not found or expired — nothing to clean up
		return nil
	}

	// Delete refresh token from Redis
	redisClient.Del(ctx, refreshKey)

	// Delete session cache if we have a valid user ID
	if userID, parseErr := uuid.Parse(userIDStr); parseErr == nil {
		if err := s.sessionCache.Delete(ctx, userID); err != nil {
			log.Printf("[AuthService] Failed to delete session cache on logout: %v", err)
		}
	}

	return nil
}

// DeleteUser deletes a user from the DB and invalidates all their sessions.
// Any future request with the user's JWT will be rejected by the middleware.
func (s *AuthService) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	// Delete from DB first
	if err := s.userRepo.DeleteByID(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Clear session cache — middleware will now reject this user's tokens
	if err := s.sessionCache.Delete(ctx, userID); err != nil {
		log.Printf("[AuthService] Failed to delete session cache for deleted user %s: %v", userID, err)
	}

	return nil
}

// generateTokens generates access and refresh tokens for a user,
// and writes the user session cache entry.
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

	// Write user session cache (best-effort, don't fail token generation)
	if err := s.sessionCache.Set(ctx, user.ID); err != nil {
		log.Printf("[AuthService] Failed to cache user session for %s: %v", user.ID, err)
	}

	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwtConfig.AccessTokenTTL.Seconds()),
	}, nil
}
