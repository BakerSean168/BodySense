package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// fakeProfileApplication mimics the service: it canonicalizes durable fields
// (id, timestamps, revision) itself instead of trusting the request.
type fakeProfileApplication struct {
	profile  *model.UserProfile
	getErr   error
	putErr   error
	commands []model.ReplaceProfileCommand
}

func (f *fakeProfileApplication) GetProfile(_ context.Context, _ uuid.UUID) (*model.UserProfile, error) {
	return f.profile, f.getErr
}

func (f *fakeProfileApplication) ReplaceProfile(_ context.Context, userID uuid.UUID, command model.ReplaceProfileCommand) (*model.UserProfile, error) {
	f.commands = append(f.commands, command)
	if f.putErr != nil {
		return nil, f.putErr
	}
	now := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
	stored := &model.UserProfile{
		ID: uuid.New(), UserID: userID, DisplayName: command.DisplayName, AvatarAssetID: command.AvatarAssetID,
		Gender: command.Gender, BirthDate: command.BirthDate, Locale: command.Locale, TimeZone: command.TimeZone,
		UnitPreference: command.UnitPreference, Revision: command.ExpectedRevision + 1, CreatedAt: now, UpdatedAt: now,
	}
	stored.ProjectPreferences()
	f.profile = stored
	return stored, nil
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
	openapiv1.RegisterHandlers(g, StrictHandler(NewPublicServer().WithProfile(profile)))
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

func TestGetUserProfileProjectsPreferencesWithoutStorageColumns(t *testing.T) {
	zone, unit := "Asia/Tokyo", "imperial"
	stored := &model.UserProfile{
		ID: uuid.New(), UserID: uuid.New(), TimeZone: &zone, UnitPreference: &unit, Revision: 3,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	stored.ProjectPreferences()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newProfileOpenAPIRouter(t, &fakeProfileApplication{profile: stored}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertOpenAPIResponse(t, req, rec)
	var raw struct {
		Profile map[string]json.RawMessage `json:"profile"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	for _, column := range []string{"time_zone", "unit_preference", "locale"} {
		if _, leaked := raw.Profile[column]; leaked {
			t.Fatalf("storage column %s leaked to the top level: %s", column, rec.Body.String())
		}
	}
	var body openapiv1.NullableUserProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Profile == nil || body.Profile.Revision != 3 || body.Profile.Preferences.TimeZone == nil ||
		*body.Profile.Preferences.TimeZone != zone || body.Profile.Preferences.Locale != nil {
		t.Fatalf("profile=%+v", body.Profile)
	}
}

const validProfilePut = `{"expected_revision":2,"display_name":"  BakerSean ","avatar_asset_id":null,` +
	`"gender":"female","birth_date":"2000-01-02",` +
	`"preferences":{"locale":"zh-CN","time_zone":"Asia/Shanghai","unit_preference":"metric"}}`

func TestUpdateUserProfileMapsTheFullReplacementCommand(t *testing.T) {
	profile := &fakeProfileApplication{}
	rec := performJSONAt(newProfileOpenAPIRouter(t, profile), http.MethodPut, "/api/v1/profile", validProfilePut)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	assertOpenAPIResponse(t, httptest.NewRequest(http.MethodPut, "/api/v1/profile", nil), rec)
	if len(profile.commands) != 1 {
		t.Fatalf("commands=%d", len(profile.commands))
	}
	command := profile.commands[0]
	if command.ExpectedRevision != 2 || command.DisplayName == nil || command.Gender == nil || *command.Gender != "female" ||
		command.BirthDate == nil || command.BirthDate.String() != "2000-01-02" || command.TimeZone == nil ||
		*command.TimeZone != "Asia/Shanghai" || command.UnitPreference == nil || *command.UnitPreference != "metric" ||
		command.Locale == nil || command.AvatarAssetID != nil {
		t.Fatalf("command=%+v", command)
	}
	var body openapiv1.UserProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if body.Profile.Revision != 3 || body.Profile.Gender == nil || string(*body.Profile.Gender) != "female" {
		t.Fatalf("profile response=%+v", body.Profile)
	}
}

func TestUpdateUserProfileRejectsPartialOrPersistenceFieldsBeforeApplication(t *testing.T) {
	for name, payload := range map[string]string{
		"derived age":       strings.Replace(validProfilePut, `"gender"`, `"age_years":22,"gender"`, 1),
		"revision field":    strings.Replace(validProfilePut, `"gender"`, `"revision":4,"gender"`, 1),
		"missing field":     `{"expected_revision":0,"gender":"male"}`,
		"unknown gender":    strings.Replace(validProfilePut, `"female"`, `"unspecified"`, 1),
		"unknown unit":      strings.Replace(validProfilePut, `"metric"`, `"stone"`, 1),
		"negative version":  strings.Replace(validProfilePut, `"expected_revision":2`, `"expected_revision":-1`, 1),
		"height in profile": strings.Replace(validProfilePut, `"gender"`, `"height_cm":180,"gender"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			profile := &fakeProfileApplication{}
			rec := performJSONAt(newProfileOpenAPIRouter(t, profile), http.MethodPut, "/api/v1/profile", payload)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
			}
			if len(profile.commands) != 0 {
				t.Fatalf("application commands=%d want=0", len(profile.commands))
			}
			assertErrorCode(t, rec, "INVALID_REQUEST")
		})
	}
}

func TestUpdateUserProfileMapsDomainErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{model.ErrProfileRevisionConflict, http.StatusConflict, "PROFILE_REVISION_CONFLICT"},
		{model.ErrProfileAvatarUnavailable, http.StatusBadRequest, "PROFILE_AVATAR_UNAVAILABLE"},
		{model.ErrInvalidProfile, http.StatusBadRequest, "INVALID_PROFILE"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			profile := &fakeProfileApplication{putErr: tc.err}
			rec := performJSONAt(newProfileOpenAPIRouter(t, profile), http.MethodPut, "/api/v1/profile", validProfilePut)
			if rec.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.status, rec.Body.String())
			}
			assertErrorCode(t, rec, tc.code)
			assertOpenAPIResponse(t, httptest.NewRequest(http.MethodPut, "/api/v1/profile", nil), rec)
		})
	}
}
