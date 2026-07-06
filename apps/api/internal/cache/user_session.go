package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// UserSessionCache uses Redis to cache user session validity.
//
// Key pattern: user_session:<uuid>  Value: "1"  TTL: configurable
//
// Design:
//   - Hot path (middleware): check Redis first, hit → allow, miss → fallback to DB
//   - Cold path (DB miss): user doesn't exist → 401
//   - If Redis is temporarily unavailable, degrade to DB-only (don't reject all requests)
type UserSessionCache struct {
	redis *redis.Client
	ttl   time.Duration
}

// NewUserSessionCache creates a new UserSessionCache.
// ttl should be set to 2x the access token TTL to ensure the cache
// doesn't expire while the token is still valid.
func NewUserSessionCache(redis *redis.Client, ttl time.Duration) *UserSessionCache {
	return &UserSessionCache{
		redis: redis,
		ttl:   ttl,
	}
}

func sessionKey(userID uuid.UUID) string {
	return fmt.Sprintf("user_session:%s", userID.String())
}

// Exists checks whether the user session exists in Redis.
//
// Returns:
//
//	(true, nil)  — cache hit, user is valid
//	(false, nil) — cache miss, caller should fallback to DB
//	(false, err) — Redis unavailable, caller should fallback to DB
func (c *UserSessionCache) Exists(ctx context.Context, userID uuid.UUID) (bool, error) {
	key := sessionKey(userID)

	val, err := c.redis.Exists(ctx, key).Result()
	if err != nil {
		// Redis unavailable — log and degrade gracefully
		log.Printf("[UserSessionCache] Redis EXISTS failed for %s: %v", userID, err)
		return false, err
	}

	return val > 0, nil
}

// Set writes a user session entry into Redis.
// Called after successful login, register, or token refresh.
func (c *UserSessionCache) Set(ctx context.Context, userID uuid.UUID) error {
	key := sessionKey(userID)

	if err := c.redis.Set(ctx, key, "1", c.ttl).Err(); err != nil {
		log.Printf("[UserSessionCache] Redis SET failed for %s: %v", userID, err)
		return fmt.Errorf("failed to set user session cache: %w", err)
	}

	return nil
}

// Delete removes a user session entry from Redis.
// Called on logout or when a user is found to no longer exist.
func (c *UserSessionCache) Delete(ctx context.Context, userID uuid.UUID) error {
	key := sessionKey(userID)

	if err := c.redis.Del(ctx, key).Err(); err != nil {
		log.Printf("[UserSessionCache] Redis DEL failed for %s: %v", userID, err)
		return fmt.Errorf("failed to delete user session cache: %w", err)
	}

	return nil
}
