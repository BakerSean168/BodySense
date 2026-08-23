package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimitPolicy describes a fixed-window abuse boundary.
type RateLimitPolicy struct {
	Limit  int64
	Window time.Duration
}

// RateLimitDecision is deterministic so HTTP handlers can expose a stable 429.
type RateLimitDecision struct {
	Allowed    bool
	Count      int64
	RetryAfter time.Duration
}

// RateLimiter is the smallest boundary needed by public auth handlers.
type RateLimiter interface {
	Allow(ctx context.Context, key string, policy RateLimitPolicy) (RateLimitDecision, error)
}

// RedisRateLimiter keeps abuse counters outside the application process so
// multiple API replicas share the same decision boundary.
type RedisRateLimiter struct {
	client *redis.Client
	prefix string
}

func NewRedisRateLimiter(client *redis.Client, prefix string) *RedisRateLimiter {
	if prefix == "" {
		prefix = "bodysense:auth:rate"
	}
	return &RedisRateLimiter{client: client, prefix: prefix}
}

var rateLimitScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('TTL', KEYS[1])
return {current, ttl}
`)

func (r *RedisRateLimiter) Allow(ctx context.Context, key string, policy RateLimitPolicy) (RateLimitDecision, error) {
	if r == nil || r.client == nil {
		return RateLimitDecision{}, errors.New("rate limiter unavailable")
	}
	if policy.Limit <= 0 || policy.Window <= 0 {
		return RateLimitDecision{}, errors.New("invalid rate limit policy")
	}

	windowSeconds := int64(math.Ceil(policy.Window.Seconds()))
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	digest := sha256.Sum256([]byte(key))
	redisKey := fmt.Sprintf("%s:%x", r.prefix, digest[:])
	result, err := rateLimitScript.Run(ctx, r.client, []string{redisKey}, windowSeconds).Slice()
	if err != nil {
		return RateLimitDecision{}, fmt.Errorf("evaluate rate limit: %w", err)
	}
	if len(result) != 2 {
		return RateLimitDecision{}, errors.New("invalid rate limit response")
	}
	count, ok := result[0].(int64)
	if !ok {
		return RateLimitDecision{}, errors.New("invalid rate limit count")
	}
	ttlSeconds, ok := result[1].(int64)
	if !ok {
		return RateLimitDecision{}, errors.New("invalid rate limit ttl")
	}
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}

	return RateLimitDecision{
		Allowed:    count <= policy.Limit,
		Count:      count,
		RetryAfter: time.Duration(ttlSeconds) * time.Second,
	}, nil
}
