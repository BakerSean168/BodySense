package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiterEnforcesAndResetsWindow(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer client.Close()
	limiter := NewRedisRateLimiter(client, "test:auth:rate")
	policy := RateLimitPolicy{Limit: 2, Window: time.Minute}

	for i := 1; i <= 3; i++ {
		decision, err := limiter.Allow(context.Background(), "login|ip=203.0.113.7|account=user@example.com", policy)
		if err != nil {
			t.Fatalf("Allow #%d: %v", i, err)
		}
		if got, want := decision.Allowed, i <= 2; got != want {
			t.Fatalf("Allow #%d allowed=%v, want %v", i, got, want)
		}
		if decision.RetryAfter <= 0 {
			t.Fatalf("Allow #%d retry_after=%s, want positive", i, decision.RetryAfter)
		}
	}

	mini.FastForward(time.Minute + time.Second)
	decision, err := limiter.Allow(context.Background(), "login|ip=203.0.113.7|account=user@example.com", policy)
	if err != nil {
		t.Fatalf("Allow after reset: %v", err)
	}
	if !decision.Allowed || decision.Count != 1 {
		t.Fatalf("after reset = %+v, want allowed count=1", decision)
	}
}

func TestRedisRateLimiterHashesSensitiveDimensions(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer client.Close()
	limiter := NewRedisRateLimiter(client, "test:auth:rate")
	secret := "refresh-token-secret"

	if _, err := limiter.Allow(context.Background(), "refresh|credential="+secret, RateLimitPolicy{Limit: 1, Window: time.Minute}); err != nil {
		t.Fatalf("Allow: %v", err)
	}
	for _, key := range mini.Keys() {
		if strings.Contains(key, secret) {
			t.Fatalf("rate-limit key exposes sensitive dimension: %s", key)
		}
	}
}
