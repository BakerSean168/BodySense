package cache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrUserSessionRevoked = errors.New("user session authority revoked")

// SessionCache is the revocation authority for short-lived access credentials.
type SessionCache interface {
	Exists(ctx context.Context, sessionID uuid.UUID) (bool, error)
	Set(ctx context.Context, userID, sessionID uuid.UUID) error
	Delete(ctx context.Context, userID, sessionID uuid.UUID) error
	// RevokeAllForUser atomically prevents new sessions and removes every
	// current session. The returned IDs let AuthService delete refresh families.
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

// UserSessionCache is a Redis-backed SessionCache.
//
// Keys:
//
//	session:<sid>            string, TTL = refresh TTL
//	user_sessions:<userID>   set of live session ids, TTL = refresh TTL
//	user_auth_revoked:<uid>  tombstone, TTL = refresh TTL
//
// The tombstone closes the account-erasure race: session creation and user
// revocation are both Lua-atomic in Redis, so a login that started just before
// erasure cannot re-arm a session after revocation wins.
type UserSessionCache struct {
	redis *redis.Client
	ttl   time.Duration
}

func NewUserSessionCache(client *redis.Client, ttl time.Duration) *UserSessionCache {
	return &UserSessionCache{redis: client, ttl: ttl}
}

func sessionIDKey(sessionID uuid.UUID) string { return fmt.Sprintf("session:%s", sessionID) }
func userSessionsKey(userID uuid.UUID) string { return fmt.Sprintf("user_sessions:%s", userID) }
func userRevokedKey(userID uuid.UUID) string  { return fmt.Sprintf("user_auth_revoked:%s", userID) }

func sessionTTLSeconds(ttl time.Duration) int64 {
	seconds := int64(ttl.Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

var setSessionScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 1 then
  return 0
end
redis.call('SET', KEYS[1], '1', 'EX', ARGV[2])
redis.call('SADD', KEYS[2], ARGV[1])
redis.call('EXPIRE', KEYS[2], ARGV[2])
return 1
`)

var deleteSessionScript = redis.NewScript(`
redis.call('SREM', KEYS[2], ARGV[1])
redis.call('DEL', KEYS[1])
return 1
`)

var revokeUserSessionsScript = redis.NewScript(`
redis.call('SET', KEYS[2], '1', 'EX', ARGV[1])
local members = redis.call('SMEMBERS', KEYS[1])
for _, sid in ipairs(members) do
  redis.call('DEL', 'session:' .. sid)
end
redis.call('DEL', KEYS[1])
return members
`)

func (c *UserSessionCache) Exists(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	val, err := c.redis.Exists(ctx, sessionIDKey(sessionID)).Result()
	if err != nil {
		log.Printf("[UserSessionCache] Redis EXISTS failed for %s: %v", sessionID, err)
		return false, err
	}
	return val > 0, nil
}

func (c *UserSessionCache) Set(ctx context.Context, userID, sessionID uuid.UUID) error {
	result, err := setSessionScript.Run(
		ctx,
		c.redis,
		[]string{sessionIDKey(sessionID), userSessionsKey(userID), userRevokedKey(userID)},
		sessionID.String(), sessionTTLSeconds(c.ttl),
	).Int64()
	if err != nil {
		return fmt.Errorf("set session authority: %w", err)
	}
	if result != 1 {
		return ErrUserSessionRevoked
	}
	return nil
}

func (c *UserSessionCache) Delete(ctx context.Context, userID, sessionID uuid.UUID) error {
	if _, err := deleteSessionScript.Run(
		ctx,
		c.redis,
		[]string{sessionIDKey(sessionID), userSessionsKey(userID)},
		sessionID.String(),
	).Result(); err != nil {
		return fmt.Errorf("delete session authority: %w", err)
	}
	return nil
}

func (c *UserSessionCache) RevokeAllForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	values, err := revokeUserSessionsScript.Run(
		ctx,
		c.redis,
		[]string{userSessionsKey(userID), userRevokedKey(userID)},
		sessionTTLSeconds(c.ttl),
	).StringSlice()
	if err != nil {
		return nil, fmt.Errorf("revoke user session authority: %w", err)
	}

	sessionIDs := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		sessionID, parseErr := uuid.Parse(value)
		if parseErr != nil {
			return nil, fmt.Errorf("parse revoked session id %q: %w", value, parseErr)
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	return sessionIDs, nil
}
