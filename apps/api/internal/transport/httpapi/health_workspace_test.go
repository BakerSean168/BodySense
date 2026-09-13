package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bodysense/api/internal/dto"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeHealthWorkspaceService struct {
	calls     int
	workspace *dto.HealthWorkspace
	err       error
}

func (f *fakeHealthWorkspaceService) Get(_ context.Context, _ uuid.UUID) (*dto.HealthWorkspace, error) {
	f.calls++
	return f.workspace, f.err
}

func minimalWorkspace() *dto.HealthWorkspace {
	return &dto.HealthWorkspace{
		GeneratedAt:  time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC),
		ProfileReady: true,
		BodyState: &dto.HealthWorkspaceBodyState{
			CurrentRevision:     0,
			SafetyState:         json.RawMessage(`{}`),
			Facts:               []model.BodyStateFact{},
			PendingFacts:        []model.BodyStateFact{},
			Observations:        []model.BodyStateObservation{},
			PendingObservations: []model.BodyStateObservation{},
			Hypotheses:          []model.BodyStateHypothesis{},
			RecentRevisions:     []model.BodyStateRevision{},
		},
		TreatmentRevisions: []model.TreatmentRevision{},
		RecentOutcomes:     []model.Outcome{},
		Trends:             []dto.HealthWorkspaceTrend{},
		Actions:            []dto.HealthWorkspaceAction{},
	}
}

func newWorkspaceOpenAPIRouter(t *testing.T, service healthWorkspaceService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	g := r.Group("")
	g.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.NewString())
		c.Next()
	})
	g.Use(RequestValidator(spec))
	openapiv1.RegisterHandlers(g, StrictHandler(NewPublicServer(&fakeBodyStateFactService{}).WithHealthWorkspace(service)))
	return r
}

func TestGetHealthWorkspaceMatchesOpenAPIResponseSchema(t *testing.T) {
	service := &fakeHealthWorkspaceService{workspace: minimalWorkspace()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health-workspace", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newWorkspaceOpenAPIRouter(t, service).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	if service.calls != 1 {
		t.Fatalf("service calls=%d want=1", service.calls)
	}
	assertOpenAPIResponse(t, req, rec)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	bodyState, ok := body["body_state"].(map[string]any)
	if !ok {
		t.Fatalf("body_state=%#v", body["body_state"])
	}
	if _, exists := bodyState["user_id"]; exists {
		t.Fatal("workspace body_state must not invent the consultation snapshot user_id field")
	}
}

func TestGetHealthWorkspaceServiceFailureUsesCanonicalError(t *testing.T) {
	service := &fakeHealthWorkspaceService{err: errors.New("database unavailable")}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health-workspace", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newWorkspaceOpenAPIRouter(t, service).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want=500 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INTERNAL_ERROR")
	assertOpenAPIResponse(t, req, rec)
}

func assertOpenAPIResponse(t *testing.T, req *http.Request, rec *httptest.ResponseRecorder) {
	t.Helper()
	ctx := context.Background()
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	spec.Servers = nil
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatalf("create OpenAPI router: %v", err)
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("find OpenAPI route: %v", err)
	}
	input := &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route}
	response := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: input,
		Status:                 rec.Code,
		Header:                 rec.Header(),
	}
	response.SetBodyBytes(rec.Body.Bytes())
	if err := openapi3filter.ValidateResponse(ctx, response); err != nil {
		t.Fatalf("response violates OpenAPI: %v body=%s", err, rec.Body.String())
	}
}
