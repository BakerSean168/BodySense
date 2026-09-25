package cache

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestUserSessionCacheRealRedis8(t *testing.T) {
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

	userID, sessionID := uuid.New(), uuid.New()
	t.Cleanup(func() {
		client.Del(context.Background(), sessionIDKey(sessionID), userSessionsKey(userID), userRevokedKey(userID))
	})
	const lifetime = 30 * time.Second
	cache := NewUserSessionCache(client, lifetime)
	if err := cache.Set(ctx, userID, sessionID); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if exists, err := cache.Exists(ctx, sessionID); err != nil || !exists {
		t.Fatalf("Exists after Set: %v, %v", exists, err)
	}
	for _, key := range []string{sessionIDKey(sessionID), userSessionsKey(userID)} {
		ttl, err := client.TTL(ctx, key).Result()
		if err != nil || ttl <= 0 || ttl > lifetime {
			t.Fatalf("TTL for %s = %s, error=%v", key, ttl, err)
		}
	}
	if err := cache.Delete(ctx, userID, sessionID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if exists, err := cache.Exists(ctx, sessionID); err != nil || exists {
		t.Fatalf("Exists after Delete: %v, %v", exists, err)
	}
}
