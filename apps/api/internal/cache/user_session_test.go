package cache

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// fakeRedis is an in-memory redisOp that tracks string keys and set members,
// with a failure flag to simulate Redis outages.
type fakeRedis struct {
	strings map[string]string
	sets    map[string]map[string]struct{}
	fail    bool
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		strings: map[string]string{},
		sets:    map[string]map[string]struct{}{},
	}
}

func (f *fakeRedis) Set(_ context.Context, key string, value any, _ time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(context.Background())
	if f.fail {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}
	f.strings[key] = value.(string)
	cmd.SetVal("OK")
	return cmd
}

func (f *fakeRedis) Del(_ context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	if f.fail {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}
	var n int64
	for _, key := range keys {
		if _, ok := f.strings[key]; ok {
			delete(f.strings, key)
			n++
		}
		if members, ok := f.sets[key]; ok {
			delete(f.sets, key)
			n += int64(len(members))
		}
	}
	cmd.SetVal(n)
	return cmd
}

func (f *fakeRedis) Exists(_ context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	if f.fail {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}
	var n int64
	for _, key := range keys {
		if _, ok := f.strings[key]; ok {
			n++
		}
		if members, ok := f.sets[key]; ok {
			n += int64(len(members))
		}
	}
	cmd.SetVal(n)
	return cmd
}

func (f *fakeRedis) SAdd(_ context.Context, key string, members ...any) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	if f.fail {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}
	if f.sets[key] == nil {
		f.sets[key] = map[string]struct{}{}
	}
	var added int64
	for _, m := range members {
		member := m.(string)
		if _, ok := f.sets[key][member]; !ok {
			f.sets[key][member] = struct{}{}
			added++
		}
	}
	cmd.SetVal(added)
	return cmd
}

func (f *fakeRedis) SRem(_ context.Context, key string, members ...any) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	if f.fail {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}
	var removed int64
	for _, m := range members {
		member := m.(string)
		if _, ok := f.sets[key][member]; ok {
			delete(f.sets[key], member)
			removed++
		}
	}
	cmd.SetVal(removed)
	return cmd
}

func (f *fakeRedis) SMembers(_ context.Context, key string) *redis.StringSliceCmd {
	cmd := redis.NewStringSliceCmd(context.Background())
	if f.fail {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}
	var out []string
	for member := range f.sets[key] {
		out = append(out, member)
	}
	cmd.SetVal(out)
	return cmd
}

func newTestCache(f *fakeRedis) *UserSessionCache {
	return &UserSessionCache{redis: f, ttl: 30 * 24 * time.Hour}
}

func TestUserSessionCacheSetExistsDelete(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache(newFakeRedis())
	userID := uuid.New()
	sessionID := uuid.New()

	if err := cache.Set(ctx, userID, sessionID); err != nil {
		t.Fatalf("Set: %v", err)
	}
	exists, err := cache.Exists(ctx, sessionID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("session should exist after Set")
	}

	if err := cache.Delete(ctx, userID, sessionID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	exists, err = cache.Exists(ctx, sessionID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Fatal("session should be gone after Delete")
	}
}

func TestUserSessionCacheDeleteAllForUser(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache(newFakeRedis())
	userID := uuid.New()
	sessionA := uuid.New()
	sessionB := uuid.New()

	if err := cache.Set(ctx, userID, sessionA); err != nil {
		t.Fatalf("Set A: %v", err)
	}
	if err := cache.Set(ctx, userID, sessionB); err != nil {
		t.Fatalf("Set B: %v", err)
	}

	if err := cache.DeleteAllForUser(ctx, userID); err != nil {
		t.Fatalf("DeleteAllForUser: %v", err)
	}
	for _, sid := range []uuid.UUID{sessionA, sessionB} {
		exists, err := cache.Exists(ctx, sid)
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if exists {
			t.Fatalf("session %v should be revoked", sid)
		}
	}
}

func TestUserSessionCacheSessionIsolatedPerUser(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache(newFakeRedis())
	userA := uuid.New()
	userB := uuid.New()
	sessionA := uuid.New()

	if err := cache.Set(ctx, userA, sessionA); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// Deleting all of user B's sessions must not touch user A's session.
	if err := cache.DeleteAllForUser(ctx, userB); err != nil {
		t.Fatalf("DeleteAllForUser B: %v", err)
	}
	exists, err := cache.Exists(ctx, sessionA)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("user A session must survive revoking user B")
	}
}

func TestUserSessionCacheRedisDownSurfacesError(t *testing.T) {
	ctx := context.Background()
	f := newFakeRedis()
	f.fail = true
	cache := newTestCache(f)

	if _, err := cache.Exists(ctx, uuid.New()); err == nil {
		t.Fatal("expected error when Redis is down")
	}
	if err := cache.Set(ctx, uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error when Redis is down")
	}
	if err := cache.Delete(ctx, uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error when Redis is down")
	}
	if err := cache.DeleteAllForUser(ctx, uuid.New()); err == nil {
		t.Fatal("expected error when Redis is down")
	}
}
