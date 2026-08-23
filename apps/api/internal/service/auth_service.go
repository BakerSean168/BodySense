package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/cache"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrRegistrationFailed = errors.New("registration failed")
	ErrInvalidRefresh     = errors.New("invalid or expired refresh token")
	ErrRefreshReuse       = errors.New("refresh token reuse detected")
	ErrAuthUnavailable    = errors.New("authentication service unavailable")
)

// AuthService handles authentication business logic.
type AuthService struct {
	userRepo     *repository.UserRepository
	jwtConfig    auth.JWTConfig
	sessionCache cache.SessionCache
	redisClient  *redis.Client
}

// NewAuthService creates a new AuthService.

func NewAuthService(userRepo *repository.UserRepository, jwtConfig auth.JWTConfig, sessionCache cache.SessionCache, redisClients ...*redis.Client) *AuthService {
	redisClient := database.RedisClient
	if len(redisClients) > 0 {
		redisClient = redisClients[0]
	}
	return &AuthService{
		userRepo:     userRepo,
		jwtConfig:    jwtConfig,
		sessionCache: sessionCache,
		redisClient:  redisClient,
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
		return nil, ErrRegistrationFailed
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
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
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
	if s.redisClient == nil {
		return nil, ErrAuthUnavailable
	}
	key := refreshTokenKey(req.RefreshToken)
	stored, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
		}
		replayValue, replayErr := s.redisClient.Get(ctx, refreshReplayKey(req.RefreshToken)).Result()
		if replayErr == nil {
			if userID, sessionID, parseErr := parseRefreshValue(replayValue); parseErr == nil {
				if revokeErr := s.revokeSessionFamily(ctx, userID, sessionID); revokeErr != nil {
					return nil, fmt.Errorf("revoke replayed refresh family: %w", revokeErr)
				}
			}
			return nil, ErrRefreshReuse
		}
		if !errors.Is(replayErr, redis.Nil) {
			return nil, fmt.Errorf("%w: %v", ErrAuthUnavailable, replayErr)
		}
		return nil, ErrInvalidRefresh
	}

	// Parse "userID:sessionID"
	userID, sessionID, err := parseRefreshValue(stored)
	if err != nil {
		return nil, ErrInvalidRefresh
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

	// Generate the replacement before consuming the old token. The Lua compare
	// and delete below makes concurrent refresh requests single-winner.
	newRefreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	accessToken, err := auth.GenerateAccessToken(s.jwtConfig, user.ID, sessionID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}
	result, err := s.rotateRefreshToken(ctx, req.RefreshToken, stored, newRefreshToken, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if result == 2 {
		_ = s.revokeSessionFamily(ctx, userID, sessionID)
		return nil, ErrRefreshReuse
	}
	if result != 1 {
		return nil, ErrInvalidRefresh
	}
	if err := s.sessionCache.Set(ctx, user.ID, sessionID); err != nil {
		return nil, fmt.Errorf("failed to re-arm session: %w", err)
	}
	return &dto.AuthResponse{AccessToken: accessToken, RefreshToken: newRefreshToken, ExpiresIn: int64(s.jwtConfig.AccessTokenTTL.Seconds())}, nil
}

// Logout invalidates a refresh token and revokes its session.
// Looks up the user ID and session ID from the refresh token in Redis first.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if s.redisClient == nil {
		return ErrAuthUnavailable
	}
	refreshKey := refreshTokenKey(refreshToken)

	stored, err := s.redisClient.Get(ctx, refreshKey).Result()
	if errors.Is(err, redis.Nil) {
		stored, err = s.redisClient.Get(ctx, refreshReplayKey(refreshToken)).Result()
	}
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup refresh session: %w", err)
	}

	userID, sessionID, err := parseRefreshValue(stored)
	if err != nil {
		_ = s.redisClient.Del(ctx, refreshKey).Err()
		return nil
	}
	return s.revokeSessionFamily(ctx, userID, sessionID)
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

	if s.redisClient == nil {
		return nil, ErrAuthUnavailable
	}

	// Store only a digest-derived Redis key for the opaque refresh credential.
	// A Redis key dump must not reveal bearer credentials that can be replayed.
	redisClient := s.redisClient
	key := refreshTokenKey(refreshToken)
	familyKey := refreshFamilyKey(sessionID)
	value := fmt.Sprintf("%s:%s", user.ID.String(), sessionID.String())
	if err := redisClient.Set(ctx, key, value, s.jwtConfig.RefreshTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}
	if err := redisClient.SAdd(ctx, familyKey, key).Err(); err != nil {
		_ = redisClient.Del(ctx, key).Err()
		return nil, fmt.Errorf("failed to index refresh token family: %w", err)
	}
	if err := redisClient.Expire(ctx, familyKey, s.jwtConfig.RefreshTokenTTL).Err(); err != nil {
		_ = redisClient.SRem(ctx, familyKey, key).Err()
		_ = redisClient.Del(ctx, key).Err()
		return nil, fmt.Errorf("failed to bound refresh family TTL: %w", err)
	}

	// Session authority is mandatory: never issue credentials that cannot be
	// revoked immediately through the session boundary.
	if err := s.sessionCache.Set(ctx, user.ID, sessionID); err != nil {
		_ = redisClient.SRem(ctx, familyKey, key).Err()
		_ = redisClient.Del(ctx, key).Err()
		return nil, fmt.Errorf("failed to establish session authority: %w", err)
	}

	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwtConfig.AccessTokenTTL.Seconds()),
	}, nil
}

const refreshReplayTTL = 10 * time.Minute

func refreshTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}

func refreshTokenKey(token string) string {
	return fmt.Sprintf("refresh_token:%s", refreshTokenDigest(token))
}
func refreshReplayKey(token string) string {
	return fmt.Sprintf("refresh_replay:%s", refreshTokenDigest(token))
}
func refreshFamilyKey(sessionID uuid.UUID) string { return fmt.Sprintf("refresh_family:%s", sessionID) }

var rotateRefreshScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current == ARGV[1] then
  redis.call('DEL', KEYS[1])
  redis.call('SET', KEYS[2], current, 'EX', ARGV[5])
  redis.call('SET', KEYS[3], ARGV[3], 'EX', ARGV[4])
  redis.call('SREM', KEYS[4], KEYS[1])
  redis.call('SADD', KEYS[4], KEYS[3])
  redis.call('EXPIRE', KEYS[4], ARGV[4])
  return 1
end
local replay = redis.call('GET', KEYS[2])
if replay == ARGV[1] then
  return 2
end
return 0
`)

var revokeRefreshFamilyScript = redis.NewScript(`
local members = redis.call('SMEMBERS', KEYS[1])
for _, key in ipairs(members) do
  redis.call('DEL', key)
end
redis.call('DEL', KEYS[1])
return #members
`)

func (s *AuthService) rotateRefreshToken(ctx context.Context, oldToken, expected, newToken string, userID, sessionID uuid.UUID) (int64, error) {
	result, err := rotateRefreshScript.Run(ctx, s.redisClient,
		[]string{refreshTokenKey(oldToken), refreshReplayKey(oldToken), refreshTokenKey(newToken), refreshFamilyKey(sessionID)},
		expected, newToken, fmt.Sprintf("%s:%s", userID, sessionID), int64(s.jwtConfig.RefreshTokenTTL.Seconds()), int64(refreshReplayTTL.Seconds()),
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("rotate refresh token: %w", err)
	}
	return result, nil
}

func (s *AuthService) revokeSessionFamily(ctx context.Context, userID, sessionID uuid.UUID) error {
	if s.redisClient == nil {
		return ErrAuthUnavailable
	}
	if _, err := revokeRefreshFamilyScript.Run(ctx, s.redisClient, []string{refreshFamilyKey(sessionID)}).Int64(); err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	if err := s.sessionCache.Delete(ctx, userID, sessionID); err != nil {
		return fmt.Errorf("revoke session authority: %w", err)
	}
	return nil
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
