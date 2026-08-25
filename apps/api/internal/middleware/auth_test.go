package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type fakeSessionCache struct {
	live map[uuid.UUID]bool
	down bool
}

func (f *fakeSessionCache) Exists(_ context.Context, sessionID uuid.UUID) (bool, error) {
	if f.down {
		return false, context.DeadlineExceeded
	}
	live, ok := f.live[sessionID]
	return ok && live, nil
}

func (f *fakeSessionCache) Set(_ context.Context, _, _ uuid.UUID) error { return nil }

func (f *fakeSessionCache) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }

func (f *fakeSessionCache) RevokeAllForUser(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func newMiddlewareTestDB(t *testing.T) (*repository.UserRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return repository.NewUserRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func performRequest(mw func(*gin.Context), token string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", mw, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAuthMiddlewareSessionLiveAllows(t *testing.T) {
	_, _, cleanup := newMiddlewareTestDB(t)
	defer cleanup()

	cfg := auth.JWTConfig{SecretKey: "test-secret", AccessTokenTTL: 15 * time.Minute}
	sessionCache := &fakeSessionCache{live: map[uuid.UUID]bool{}}
	sessionID := uuid.New()
	token, err := auth.GenerateAccessToken(cfg, uuid.New(), sessionID, "test@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	sessionCache.live[sessionID] = true

	mw := AuthMiddleware(cfg, nil, sessionCache)
	if rec := performRequest(mw, token); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddlewareSessionRevokedRejects(t *testing.T) {
	cfg := auth.JWTConfig{SecretKey: "test-secret", AccessTokenTTL: 15 * time.Minute}
	sessionCache := &fakeSessionCache{live: map[uuid.UUID]bool{}}
	token, err := auth.GenerateAccessToken(cfg, uuid.New(), uuid.New(), "test@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	mw := AuthMiddleware(cfg, nil, sessionCache)
	if rec := performRequest(mw, token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMiddlewareRedisDownFailsClosed(t *testing.T) {
	cfg := auth.JWTConfig{SecretKey: "test-secret", AccessTokenTTL: 15 * time.Minute}
	token, err := auth.GenerateAccessToken(cfg, uuid.New(), uuid.New(), "test@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	mw := AuthMiddleware(cfg, nil, &fakeSessionCache{down: true})
	if rec := performRequest(mw, token); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddlewareLegacyTokenNoSessionCacheHitForcesDB(t *testing.T) {
	userRepo, mock, cleanup := newMiddlewareTestDB(t)
	defer cleanup()

	cfg := auth.JWTConfig{SecretKey: "test-secret", AccessTokenTTL: 15 * time.Minute}
	userID := uuid.New()

	// Legacy token without session_id: must still check the user exists.
	token, err := auth.GenerateAccessToken(cfg, userID, uuid.Nil, "test@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE id = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))

	sessionCache := &fakeSessionCache{down: false}
	mw := AuthMiddleware(cfg, userRepo, sessionCache)
	if rec := performRequest(mw, token); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAuthMiddlewareLegacyTokenMissingUserRejects(t *testing.T) {
	userRepo, mock, cleanup := newMiddlewareTestDB(t)
	defer cleanup()

	cfg := auth.JWTConfig{SecretKey: "test-secret", AccessTokenTTL: 15 * time.Minute}
	userID := uuid.New()
	token, err := auth.GenerateAccessToken(cfg, userID, uuid.Nil, "test@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE id = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mw := AuthMiddleware(cfg, userRepo, &fakeSessionCache{})
	if rec := performRequest(mw, token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%s)", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAuthMiddlewareLegacyTokenDBFailureFailsClosed(t *testing.T) {
	userRepo, mock, cleanup := newMiddlewareTestDB(t)
	defer cleanup()

	cfg := auth.JWTConfig{SecretKey: "test-secret", AccessTokenTTL: 15 * time.Minute}
	userID := uuid.New()
	token, err := auth.GenerateAccessToken(cfg, userID, uuid.Nil, "test@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE id = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(errors.New("database unavailable"))

	mw := AuthMiddleware(cfg, userRepo, &fakeSessionCache{})
	if rec := performRequest(mw, token); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAuthMiddlewareMissingHeaderRejects(t *testing.T) {
	cfg := auth.JWTConfig{SecretKey: "test-secret", AccessTokenTTL: 15 * time.Minute}
	mw := AuthMiddleware(cfg, nil, &fakeSessionCache{live: map[uuid.UUID]bool{}})
	if rec := performRequest(mw, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func performKnowledgeOperatorRequest(userRepo *repository.UserRepository, userID uuid.UUID) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/knowledge", func(c *gin.Context) {
		c.Set("user_id", userID.String())
		c.Next()
	}, RequireKnowledgeOperator(userRepo), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req := httptest.NewRequest(http.MethodGet, "/knowledge", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestRequireKnowledgeOperatorAllowsOperator(t *testing.T) {
	userRepo, mock, cleanup := newMiddlewareTestDB(t)
	defer cleanup()
	userID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE id = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role"}).AddRow(userID, "operator"))

	rec := performKnowledgeOperatorRequest(userRepo, userID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireKnowledgeOperatorRejectsMember(t *testing.T) {
	userRepo, mock, cleanup := newMiddlewareTestDB(t)
	defer cleanup()
	userID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE id = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role"}).AddRow(userID, "member"))

	rec := performKnowledgeOperatorRequest(userRepo, userID)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=403 body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
