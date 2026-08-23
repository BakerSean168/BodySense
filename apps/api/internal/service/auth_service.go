package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

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
	sessionCache cache.SessionCache
}

// NewAuthService creates a new AuthService.
func NewAuthService(userRepo *repository.UserRepository, jwtConfig auth.JWTConfig, sessionCache cache.SessionCache) *AuthService {
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
	return s.generateTokens(ctx, user, uuid.New())
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
	return s.generateTokens(ctx, user, uuid.New())
}

// RefreshToken refreshes an access token using a refresh token.
// The refresh token value is stored as "userID:sessionID" so rotation keeps the
// same session alive, re-arms its TTL, and issues a new access token for it.
func (s *AuthService) RefreshToken(ctx context.Context, req dto.RefreshRequest) (*dto.AuthResponse, error) {
	// Get user ID and session ID from Redis
	redisClient := database.RedisClient
	key := fmt.Sprintf("refresh_token:%s", req.RefreshToken)

	stored, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	// Delete old refresh token
	redisClient.Del(ctx, key)

	// Parse "userID:sessionID"
	userID, sessionID, err := parseRefreshValue(stored)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Find user — if user no longer exists, clean up and reject
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User was deleted — clear stale session cache (no session id to revoke
			// individually, so clear by user against the family index).
			_ = s.sessionCache.DeleteAllForUser(ctx, userID)
			return nil, errors.New("user no longer exists")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Re-arm the same session and issue a fresh access token for it.
	return s.generateTokens(ctx, user, sessionID)
}

// Logout invalidates a refresh token and revokes its session.
// Looks up the user ID and session ID from the refresh token in Redis first.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	redisClient := database.RedisClient
	refreshKey := fmt.Sprintf("refresh_token:%s", refreshToken)

	// Look up user ID and session ID from refresh token (before deleting it)
	stored, err := redisClient.Get(ctx, refreshKey).Result()
	if err != nil {
		// Refresh token not found or expired — nothing to clean up
		return nil
	}

	// Delete refresh token from Redis
	redisClient.Del(ctx, refreshKey)

	// Revoke the session so every access token minted for it is rejected.
	if userID, sessionID, parseErr := parseRefreshValue(stored); parseErr == nil {
		if err := s.sessionCache.Delete(ctx, userID, sessionID); err != nil {
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

	// Revoke every session — middleware will now reject this user's tokens.
	if err := s.sessionCache.DeleteAllForUser(ctx, userID); err != nil {
		log.Printf("[AuthService] Failed to delete session cache for deleted user %s: %v", userID, err)
	}

	return nil
}

// generateTokens generates access and refresh tokens for a user within a session,
// and writes the session cache entry. When sessionID is uuid.Nil a new session is
// created; otherwise the existing session is re-armed (refresh rotation).
func (s *AuthService) generateTokens(ctx context.Context, user *model.User, sessionID uuid.UUID) (*dto.AuthResponse, error) {
	// Generate access token bound to the session
	accessToken, err := auth.GenerateAccessToken(s.jwtConfig, user.ID, sessionID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store refresh token in Redis as "userID:sessionID"
	redisClient := database.RedisClient
	key := fmt.Sprintf("refresh_token:%s", refreshToken)
	value := fmt.Sprintf("%s:%s", user.ID.String(), sessionID.String())
	if err := redisClient.Set(ctx, key, value, s.jwtConfig.RefreshTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Index + arm the session (best-effort, don't fail token generation)
	if err := s.sessionCache.Set(ctx, user.ID, sessionID); err != nil {
		log.Printf("[AuthService] Failed to cache user session for %s: %v", user.ID, err)
	}

	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwtConfig.AccessTokenTTL.Seconds()),
	}, nil
}

func parseRefreshValue(stored string) (userID, sessionID uuid.UUID, err error) {
	parts := strings.Split(stored, ":")
	if len(parts) < 2 {
		return uuid.Nil, uuid.Nil, errors.New("malformed refresh token value")
	}
	userID, err = uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	sessionID, err = uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return userID, sessionID, nil
}
