package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SessionCache bounds the Redis operations AuthService and AuthMiddleware depend
// on so tests can substitute a fake without a real Redis instance.
type SessionCache interface {
	// Exists reports whether the session is still valid.
	// (true, nil) active, (false, nil) definitively revoked/unknown, (false, err) Redis unavailable.
	Exists(ctx context.Context, sessionID uuid.UUID) (bool, error)
	// Set registers a new session under the user and adds sessionID to the user's family index.
	Set(ctx context.Context, userID, sessionID uuid.UUID) error
	// Delete ends one session: removes it from the user's index and drops the session key.
	Delete(ctx context.Context, userID, sessionID uuid.UUID) error
	// DeleteAllForUser ends every session for a user (used on account deletion).
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

// UserSessionCache is a Redis-backed SessionCache.
//
// Keys:
//
//	session:<sid>                       redis string, value "1", TTL = refresh TTL
//	user_sessions:<userID>             redis set of live session ids
//
// Design:
//   - Access tokens are short-lived and carry the session id. The middleware
//     rejects tokens whose session was revoked (logout / global sign-out).
//   - TTL is the refresh-token TTL because the session family lives exactly as
//     long as its refresh token (refresh re-arms both).
//   - Redis unavailable => set/membership errors surface to the caller, which
//     degrades to a DB user check with no write-back.
type UserSessionCache struct {
	redis redisOp
	ttl   time.Duration
}

// redisOp narrows the go-redis surface to the commands this cache uses. The
// concrete redis.Client satisfies it; tests can provide a fake.
type redisOp interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	SAdd(ctx context.Context, key string, members ...any) *redis.IntCmd
	SRem(ctx context.Context, key string, members ...any) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
}

// NewUserSessionCache creates a Redis-backed SessionCache.
// ttl is the refresh-token TTL, which bounds how long a session stays live.
func NewUserSessionCache(client *redis.Client, ttl time.Duration) *UserSessionCache {
	return &UserSessionCache{redis: client, ttl: ttl}
}

func sessionIDKey(sessionID uuid.UUID) string {
	return fmt.Sprintf("session:%s", sessionID.String())
}

func userSessionsKey(userID uuid.UUID) string {
	return fmt.Sprintf("user_sessions:%s", userID.String())
}

// Exists reports whether the session key is present in Redis.
func (c *UserSessionCache) Exists(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	key := sessionIDKey(sessionID)
	val, err := c.redis.Exists(ctx, key).Result()
	if err != nil {
		log.Printf("[UserSessionCache] Redis EXISTS failed for %s: %v", sessionID, err)
		return false, err
	}
	return val > 0, nil
}

// Set writes the session key with the refresh TTL and records the session id
// in the user's family index.
func (c *UserSessionCache) Set(ctx context.Context, userID, sessionID uuid.UUID) error {
	if err := c.redis.Set(ctx, sessionIDKey(sessionID), "1", c.ttl).Err(); err != nil {
		log.Printf("[UserSessionCache] Redis SET failed for %s: %v", sessionID, err)
		return fmt.Errorf("set session cache: %w", err)
	}
	if err := c.redis.SAdd(ctx, userSessionsKey(userID), sessionID.String()).Err(); err != nil {
		log.Printf("[UserSessionCache] Redis SADD failed for user %s: %v", userID, err)
		return fmt.Errorf("failed to index user session: %w", err)
	}
	return nil
}

// Delete removes the session key and drops the session id from the user's index.
func (c *UserSessionCache) Delete(ctx context.Context, userID, sessionID uuid.UUID) error {
	if err := c.redis.SRem(ctx, userSessionsKey(userID), sessionID.String()).Err(); err != nil {
		log.Printf("[UserSessionCache] Redis SREM failed for user %s: %v", userID, err)
		return fmt.Errorf("failed to deindex user session: %w", err)
	}
	if err := c.redis.Del(ctx, sessionIDKey(sessionID)).Err(); err != nil {
		log.Printf("[UserSessionCache] Redis DEL failed for %s: %v", sessionID, err)
		return fmt.Errorf("failed to delete session cache: %w", err)
	}
	return nil
}

// DeleteAllForUser revokes every live session belonging to the user (account deletion).
func (c *UserSessionCache) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	indexKey := userSessionsKey(userID)
	sessionIDs, err := c.redis.SMembers(ctx, indexKey).Result()
	if err != nil {
		log.Printf("[UserSessionCache] Redis SMEMBERS failed for user %s: %v", userID, err)
		return fmt.Errorf("failed to list user sessions: %w", err)
	}
	keys := make([]string, 0, len(sessionIDs)+1)
	for _, sid := range sessionIDs {
		keys = append(keys, fmt.Sprintf("session:%s", sid))
	}
	keys = append(keys, indexKey)
	if err := c.redis.Del(ctx, keys...).Err(); err != nil {
		log.Printf("[UserSessionCache] Redis DEL failed for user %s: %v", userID, err)
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}
	return nil
}
