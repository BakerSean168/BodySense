package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newTestSessionCache(t *testing.T) (*UserSessionCache, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewUserSessionCache(client, 30*24*time.Hour), mini, client
}

func TestUserSessionCacheSetExistsDelete(t *testing.T) {
	ctx := context.Background()
	cache, _, _ := newTestSessionCache(t)
	userID := uuid.New()
	sessionID := uuid.New()

	if err := cache.Set(ctx, userID, sessionID); err != nil {
		t.Fatalf("Set: %v", err)
	}
	exists, err := cache.Exists(ctx, sessionID)
	if err != nil || !exists {
		t.Fatalf("Exists after Set = %v,%v", exists, err)
	}
	if err := cache.Delete(ctx, userID, sessionID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	exists, err = cache.Exists(ctx, sessionID)
	if err != nil || exists {
		t.Fatalf("Exists after Delete = %v,%v", exists, err)
	}
}

func TestUserSessionCacheRevokeAllReturnsSessionsAndBlocksRearm(t *testing.T) {
	ctx := context.Background()
	cache, mini, _ := newTestSessionCache(t)
	userID := uuid.New()
	sessionA := uuid.New()
	sessionB := uuid.New()
	for _, sid := range []uuid.UUID{sessionA, sessionB} {
		if err := cache.Set(ctx, userID, sid); err != nil {
			t.Fatalf("Set %s: %v", sid, err)
		}
	}

	revoked, err := cache.RevokeAllForUser(ctx, userID)
	if err != nil {
		t.Fatalf("RevokeAllForUser: %v", err)
	}
	if len(revoked) != 2 {
		t.Fatalf("revoked=%v, want 2 sessions", revoked)
	}
	for _, sid := range []uuid.UUID{sessionA, sessionB} {
		exists, err := cache.Exists(ctx, sid)
		if err != nil || exists {
			t.Fatalf("session %s remains live: exists=%v err=%v", sid, exists, err)
		}
	}
	if !mini.Exists(userRevokedKey(userID)) {
		t.Fatal("user revocation tombstone missing")
	}
	if err := cache.Set(ctx, userID, uuid.New()); !errors.Is(err, ErrUserSessionRevoked) {
		t.Fatalf("Set after user revocation = %v, want ErrUserSessionRevoked", err)
	}
}

func TestUserSessionCacheRevokingOtherUserIsIsolated(t *testing.T) {
	ctx := context.Background()
	cache, _, _ := newTestSessionCache(t)
	userA := uuid.New()
	userB := uuid.New()
	sessionA := uuid.New()
	if err := cache.Set(ctx, userA, sessionA); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := cache.RevokeAllForUser(ctx, userB); err != nil {
		t.Fatalf("RevokeAllForUser B: %v", err)
	}
	exists, err := cache.Exists(ctx, sessionA)
	if err != nil || !exists {
		t.Fatalf("user A session affected by user B revocation: exists=%v err=%v", exists, err)
	}
}

func TestUserSessionCacheRedisDownSurfacesError(t *testing.T) {
	ctx := context.Background()
	cache, mini, _ := newTestSessionCache(t)
	mini.Close()

	if _, err := cache.Exists(ctx, uuid.New()); err == nil {
		t.Fatal("expected Exists error when Redis is down")
	}
	if err := cache.Set(ctx, uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected Set error when Redis is down")
	}
	if err := cache.Delete(ctx, uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected Delete error when Redis is down")
	}
	if _, err := cache.RevokeAllForUser(ctx, uuid.New()); err == nil {
		t.Fatal("expected RevokeAllForUser error when Redis is down")
	}
}
