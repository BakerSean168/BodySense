package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRefreshFamilyRealRedis8(t *testing.T) {
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
	oldToken, newToken := uuid.NewString(), uuid.NewString()
	oldKey, newKey := refreshTokenKey(oldToken), refreshTokenKey(newToken)
	replayKey, familyKey := refreshReplayKey(oldToken), refreshFamilyKey(sessionID)
	t.Cleanup(func() { client.Del(context.Background(), oldKey, newKey, replayKey, familyKey) })
	expected := fmt.Sprintf("%s:%s", userID, sessionID)
	if err := client.Set(ctx, oldKey, expected, time.Minute).Err(); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	if err := client.SAdd(ctx, familyKey, oldKey).Err(); err != nil {
		t.Fatalf("seed family: %v", err)
	}
	svc := &AuthService{redisClient: client}
	svc.jwtConfig.RefreshTokenTTL = time.Minute
	result, err := svc.rotateRefreshToken(ctx, oldToken, expected, newToken, userID, sessionID)
	if err != nil || result != 1 {
		t.Fatalf("rotate = %d, error=%v", result, err)
	}
	result, err = svc.rotateRefreshToken(ctx, oldToken, expected, uuid.NewString(), userID, sessionID)
	if err != nil || result != 2 {
		t.Fatalf("replay = %d, error=%v", result, err)
	}
	if n, err := client.SIsMember(ctx, familyKey, newKey).Result(); err != nil || !n {
		t.Fatalf("replacement absent from family: member=%v error=%v", n, err)
	}
	if err := svc.revokeRefreshFamilyOnly(ctx, sessionID); err != nil {
		t.Fatalf("revoke family: %v", err)
	}
	if count, err := client.Exists(ctx, newKey, familyKey).Result(); err != nil || count != 0 {
		t.Fatalf("family remains: count=%d error=%v", count, err)
	}
}
