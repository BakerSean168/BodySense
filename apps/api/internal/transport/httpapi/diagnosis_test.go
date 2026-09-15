package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeDiagnosisApplication struct {
	payload map[string]any
	err     *service.DiagnosisApplicationError
	calls   int
}

func (f *fakeDiagnosisApplication) Analyze(context.Context, uuid.UUID, uuid.UUID) (map[string]any, *service.DiagnosisApplicationError) {
	f.calls++
	return f.payload, f.err
}

func newDiagnosisRouteTestRouter(t *testing.T, application diagnosisApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	server := NewPublicServer(nil).WithDiagnosis(application, nil, nil, nil)
	RegisterRoutes(r, StrictHandler(server), RouteSecurity{
		Auth: func(c *gin.Context) {
			c.Set("user_id", uuid.NewString())
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return r
}

func TestAnalyzeDiagnosisMapsBodyStateNotReadyToConflict(t *testing.T) {
	application := &fakeDiagnosisApplication{err: &service.DiagnosisApplicationError{
		Code: "BODY_STATE_NOT_READY", Message: "body state is not ready",
	}}
	r := newDiagnosisRouteTestRouter(t, application)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/consultations/11111111-1111-4111-8111-111111111111/diagnosis", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "BODY_STATE_NOT_READY")
	if application.calls != 1 {
		t.Fatalf("application calls=%d", application.calls)
	}
}

func TestProjectDiagnosisPayloadRemovesNestedUserIdentity(t *testing.T) {
	projected := projectDiagnosisPayload(map[string]any{
		"analysis_id": "11111111-1111-4111-8111-111111111111",
		"freshness": map[string]any{
			"analysis_id": "11111111-1111-4111-8111-111111111111",
			"user_id":     "private-user",
			"state":       "fresh",
		},
		"candidate_assessments": []any{
			map[string]any{
				"id":           "22222222-2222-4222-8222-222222222222",
				"analysis_id":  "11111111-1111-4111-8111-111111111111",
				"candidate_id": "33333333-3333-4333-8333-333333333333",
				"user_id":      "private-user",
				"state":        "confirmed",
			},
		},
	})

	freshness, ok := projected["freshness"].(map[string]any)
	if !ok {
		t.Fatalf("freshness projection=%#v", projected["freshness"])
	}
	if _, leaked := freshness["user_id"]; leaked {
		t.Fatalf("freshness leaked user_id: %#v", freshness)
	}
	assessments, ok := projected["candidate_assessments"].([]any)
	if !ok || len(assessments) != 1 {
		t.Fatalf("assessment projection=%#v", projected["candidate_assessments"])
	}
	assessment, ok := assessments[0].(map[string]any)
	if !ok {
		t.Fatalf("assessment=%#v", assessments[0])
	}
	if _, leaked := assessment["user_id"]; leaked {
		t.Fatalf("assessment leaked user_id: %#v", assessment)
	}
}

func TestStrictDiagnosisAssessmentsDropsPersistenceUserID(t *testing.T) {
	assessment := model.DiagnosisCandidateAssessment{
		ID:          uuid.New(),
		AnalysisID:  uuid.New(),
		CandidateID: uuid.New(),
		UserID:      uuid.New(),
		State:       "confirmed",
		AssessedAt:  time.Now().UTC(),
	}
	projected, err := strictDiagnosisAssessments([]model.DiagnosisCandidateAssessment{assessment})
	if err != nil {
		t.Fatalf("strict assessment projection: %v", err)
	}
	if len(projected) != 1 || projected[0].Id != assessment.ID || projected[0].CandidateId != assessment.CandidateID {
		t.Fatalf("unexpected assessment projection: %+v", projected)
	}
}
