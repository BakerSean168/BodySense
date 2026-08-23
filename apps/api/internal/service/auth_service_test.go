package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type recordingSessionCache struct {
	mu           sync.Mutex
	live         map[uuid.UUID]bool
	setErr       error
	deleteErr    error
	deleteAllErr error
}

func newRecordingSessionCache() *recordingSessionCache {
	return &recordingSessionCache{live: map[uuid.UUID]bool{}}
}

func (f *recordingSessionCache) Exists(_ context.Context, sessionID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.live[sessionID], nil
}

func (f *recordingSessionCache) Set(_ context.Context, _ uuid.UUID, sessionID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return f.setErr
	}
	f.live[sessionID] = true
	return nil
}

func (f *recordingSessionCache) Delete(_ context.Context, _ uuid.UUID, sessionID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.live, sessionID)
	return nil
}

func (f *recordingSessionCache) RevokeAllForUser(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteAllErr != nil {
		return nil, f.deleteAllErr
	}
	revoked := make([]uuid.UUID, 0, len(f.live))
	for sessionID := range f.live {
		revoked = append(revoked, sessionID)
	}
	f.live = map[uuid.UUID]bool{}
	return revoked, nil
}

func newAuthRedisTestService(t *testing.T) (*AuthService, *miniredis.Miniredis, *redis.Client, *recordingSessionCache) {
	t.Helper()
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	sessions := newRecordingSessionCache()
	return &AuthService{
		jwtConfig: auth.JWTConfig{
			SecretKey:       "test-secret",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 30 * 24 * time.Hour,
		},
		sessionCache: sessions,
		redisClient:  client,
	}, mini, client, sessions
}

func TestRefreshTokenKeyDoesNotExposeBearerCredential(t *testing.T) {
	token := "this-is-a-bearer-secret"
	key := refreshTokenKey(token)
	if strings.Contains(key, token) {
		t.Fatalf("refresh Redis key exposes bearer credential: %q", key)
	}
	if key == refreshTokenKey(token+"-other") {
		t.Fatal("different refresh credentials must not share a Redis key")
	}
}

func TestRotateRefreshTokenSingleWinnerAndReplayRevokesFamily(t *testing.T) {
	ctx := context.Background()
	svc, _, client, sessions := newAuthRedisTestService(t)
	userID := uuid.New()
	sessionID := uuid.New()
	oldToken := "old-refresh-token"
	newTokens := []string{"new-refresh-token-a", "new-refresh-token-b"}
	expected := fmt.Sprintf("%s:%s", userID, sessionID)

	if err := client.Set(ctx, refreshTokenKey(oldToken), expected, svc.jwtConfig.RefreshTokenTTL).Err(); err != nil {
		t.Fatalf("seed refresh token: %v", err)
	}
	if err := client.SAdd(ctx, refreshFamilyKey(sessionID), refreshTokenKey(oldToken)).Err(); err != nil {
		t.Fatalf("seed refresh family: %v", err)
	}
	if err := sessions.Set(ctx, userID, sessionID); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	results := make(chan int64, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, token := range newTokens {
		token := token
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := svc.rotateRefreshToken(ctx, oldToken, expected, token, userID, sessionID)
			results <- result
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("rotate refresh token: %v", err)
		}
	}
	got := make([]int, 0, 2)
	for result := range results {
		got = append(got, int(result))
	}
	sort.Ints(got)
	if fmt.Sprint(got) != "[1 2]" {
		t.Fatalf("rotation results = %v, want one winner and one replay", got)
	}

	// A replay of the consumed credential must revoke the whole family, including
	// whichever replacement won the race.
	if _, err := svc.RefreshToken(ctx, dto.RefreshRequest{RefreshToken: oldToken}); err == nil || !strings.Contains(err.Error(), "reuse") {
		t.Fatalf("RefreshToken(replay) error = %v, want reuse detection", err)
	}
	for _, token := range newTokens {
		if n, err := client.Exists(ctx, refreshTokenKey(token)).Result(); err != nil || n != 0 {
			t.Fatalf("replacement %q still exists after replay revocation: exists=%d err=%v", token, n, err)
		}
	}
	if members, err := client.SMembers(ctx, refreshFamilyKey(sessionID)).Result(); err != nil || len(members) != 0 {
		t.Fatalf("refresh family not cleared: members=%v err=%v", members, err)
	}
	if live, _ := sessions.Exists(ctx, sessionID); live {
		t.Fatal("session authority remained live after refresh replay")
	}
}

func TestLogoutWithConsumedRefreshTokenRevokesCurrentFamily(t *testing.T) {
	ctx := context.Background()
	svc, _, client, sessions := newAuthRedisTestService(t)
	userID := uuid.New()
	sessionID := uuid.New()
	oldToken := "logout-old-token"
	newToken := "logout-current-token"
	expected := fmt.Sprintf("%s:%s", userID, sessionID)

	if err := client.Set(ctx, refreshTokenKey(oldToken), expected, svc.jwtConfig.RefreshTokenTTL).Err(); err != nil {
		t.Fatalf("seed refresh token: %v", err)
	}
	if err := client.SAdd(ctx, refreshFamilyKey(sessionID), refreshTokenKey(oldToken)).Err(); err != nil {
		t.Fatalf("seed refresh family: %v", err)
	}
	if err := sessions.Set(ctx, userID, sessionID); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	result, err := svc.rotateRefreshToken(ctx, oldToken, expected, newToken, userID, sessionID)
	if err != nil || result != 1 {
		t.Fatalf("rotate result=%d err=%v, want 1,nil", result, err)
	}

	if err := svc.Logout(ctx, oldToken); err != nil {
		t.Fatalf("Logout(consumed token): %v", err)
	}
	if n, err := client.Exists(ctx, refreshTokenKey(newToken)).Result(); err != nil || n != 0 {
		t.Fatalf("current refresh survived logout via replay marker: exists=%d err=%v", n, err)
	}
	if live, _ := sessions.Exists(ctx, sessionID); live {
		t.Fatal("session authority survived logout")
	}
}

func TestGenerateTokensRequiresRevocableSessionAuthority(t *testing.T) {
	ctx := context.Background()
	svc, mini, _, sessions := newAuthRedisTestService(t)
	sessions.setErr = errors.New("session store unavailable")
	user := &model.User{ID: uuid.New(), Email: "test@example.com"}

	if _, err := svc.generateTokens(ctx, user, uuid.New()); err == nil || !strings.Contains(err.Error(), "session authority") {
		t.Fatalf("generateTokens error = %v, want mandatory session-authority failure", err)
	}
	for _, key := range mini.Keys() {
		if strings.HasPrefix(key, "refresh_token:") {
			t.Fatalf("orphan refresh credential left behind after session failure: %s", key)
		}
	}
}

func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "valid password", password: "password123", wantErr: false},
		{name: "too short", password: "1234567", wantErr: true},
		{name: "minimum length", password: "12345678", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := len(tt.password) >= 8
			if isValid == tt.wantErr {
				t.Errorf("Password %q: expected valid=%v, got %v", tt.password, !tt.wantErr, isValid)
			}
		})
	}
}

func TestEmailValidation(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{name: "valid email", email: "test@example.com", wantErr: false},
		{name: "missing @", email: "testexample.com", wantErr: true},
		{name: "missing domain", email: "test@", wantErr: true},
		{name: "missing local part", email: "@example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atIndex := -1
			for i, c := range tt.email {
				if c == '@' {
					atIndex = i
					break
				}
			}
			isValid := atIndex > 0 && atIndex < len(tt.email)-1
			if isValid == tt.wantErr {
				t.Errorf("Email %q: expected valid=%v, got %v", tt.email, !tt.wantErr, isValid)
			}
		})
	}
}
