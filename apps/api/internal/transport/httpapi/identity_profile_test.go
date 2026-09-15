package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeProfileApplication struct {
	profile *model.UserProfile
	getErr  error
	putErr  error
	updates []*model.UserProfile
}

func (f *fakeProfileApplication) GetProfile(_ context.Context, _ uuid.UUID) (*model.UserProfile, error) {
	return f.profile, f.getErr
}

func (f *fakeProfileApplication) CreateOrUpdateProfile(_ context.Context, userID uuid.UUID, profile *model.UserProfile) error {
	profile.UserID = userID
	f.updates = append(f.updates, profile)
	if f.putErr == nil {
		f.profile = profile
	}
	return f.putErr
}

func newProfileOpenAPIRouter(t *testing.T, profile profileApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	g := r.Group("")
	g.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.NewString())
		c.Set("email", "user@example.com")
		c.Next()
	})
	g.Use(RequestValidator(spec))
	openapiv1.RegisterHandlers(g, StrictHandler(NewPublicServer(&fakeBodyStateFactService{}).WithProfile(profile)))
	return r
}

func TestGetCurrentUserUsesAuthenticatedContextAndMatchesOpenAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newProfileOpenAPIRouter(t, &fakeProfileApplication{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	assertOpenAPIResponse(t, req, rec)
	var body openapiv1.CurrentUser
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode current user: %v", err)
	}
	if string(body.Email) != "user@example.com" || body.Id == uuid.Nil {
		t.Fatalf("current user=%+v", body)
	}
}

func TestGetUserProfileReturnsExplicitNullEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newProfileOpenAPIRouter(t, &fakeProfileApplication{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	assertOpenAPIResponse(t, req, rec)
	var body openapiv1.NullableUserProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile envelope: %v", err)
	}
	if body.Profile != nil {
		t.Fatalf("profile=%+v want=nil", body.Profile)
	}
}

func TestUpdateUserProfileRejectsPersistenceFieldsBeforeApplication(t *testing.T) {
	profile := &fakeProfileApplication{}
	rec := performJSONAt(
		newProfileOpenAPIRouter(t, profile),
		http.MethodPut,
		"/api/v1/profile",
		`{"gender":"male","age_years":22}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	if len(profile.updates) != 0 {
		t.Fatalf("application updates=%d want=0", len(profile.updates))
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestUpdateUserProfileReturnsCanonicalStoredProjection(t *testing.T) {
	profile := &fakeProfileApplication{}
	// Return the same object after the write, but provide durable fields that a
	// real repository load owns rather than trusting client-supplied values.
	profileID := uuid.New()
	userID := uuid.New()
	now := time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC)
	profile.profile = &model.UserProfile{ID: profileID, UserID: userID, CreatedAt: now, UpdatedAt: now}

	// The fake update replaces profile.profile, so emulate repository canonicalization
	// through a wrapper that fills durable identity on write.
	profile.putErr = nil
	router := newProfileOpenAPIRouter(t, &canonicalProfileApplication{inner: profile, id: profileID, createdAt: now})
	rec := performJSONAt(router, http.MethodPut, "/api/v1/profile", `{"gender":"female","birth_date":"2000-01-02"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	assertOpenAPIResponse(t, httptest.NewRequest(http.MethodPut, "/api/v1/profile", nil), rec)
	var body openapiv1.UserProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if body.Profile.Id != profileID || body.Profile.Gender == nil || string(*body.Profile.Gender) != "female" {
		t.Fatalf("profile response=%+v", body.Profile)
	}
}

type canonicalProfileApplication struct {
	inner     *fakeProfileApplication
	id        uuid.UUID
	createdAt time.Time
}

func (c *canonicalProfileApplication) GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error) {
	profile, err := c.inner.GetProfile(ctx, userID)
	if profile != nil {
		profile.ID = c.id
		profile.UserID = userID
		profile.CreatedAt = c.createdAt
		profile.UpdatedAt = c.createdAt
	}
	return profile, err
}
func (c *canonicalProfileApplication) CreateOrUpdateProfile(ctx context.Context, userID uuid.UUID, profile *model.UserProfile) error {
	return c.inner.CreateOrUpdateProfile(ctx, userID, profile)
}
