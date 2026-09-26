package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiterRealRedis8(t *testing.T) {
	addr := os.Getenv("BODYSENSE_REDIS8_ADDR")
	if addr == "" {
		t.Skip("set BODYSENSE_REDIS8_ADDR to run against Redis 8")
	}
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: addr, Password: os.Getenv("BODYSENSE_REDIS8_PASSWORD")})
	defer client.Close()
	info, err := client.Info(ctx, "server").Result()
	if err != nil || !strings.Contains(info, "redis_version:8.") {
		t.Fatalf("Redis 8 required: info error=%v", err)
	}

	prefix := "test:b081:rate:" + uuid.NewString()
	key := "login:" + uuid.NewString()
	digest := sha256.Sum256([]byte(key))
	redisKey := fmt.Sprintf("%s:%x", prefix, digest[:])
	t.Cleanup(func() { client.Del(context.Background(), redisKey) })
	limiter := NewRedisRateLimiter(client, prefix)
	policy := RateLimitPolicy{Limit: 2, Window: time.Second}
	for i := int64(1); i <= 3; i++ {
		decision, err := limiter.Allow(ctx, key, policy)
		if err != nil || decision.Allowed != (i <= 2) || decision.Count != i || decision.RetryAfter <= 0 || decision.RetryAfter > policy.Window {
			t.Fatalf("Allow #%d = %+v, error=%v", i, decision, err)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		exists, err := client.Exists(ctx, redisKey).Result()
		if err != nil {
			t.Fatalf("check expiry: %v", err)
		}
		if exists == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("rate limit key did not expire")
		}
		time.Sleep(25 * time.Millisecond)
	}
	decision, err := limiter.Allow(ctx, key, policy)
	if err != nil || !decision.Allowed || decision.Count != 1 {
		t.Fatalf("Allow after expiry = %+v, error=%v", decision, err)
	}
}
