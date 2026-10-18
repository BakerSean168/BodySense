package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/domain/trainingplan"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type planGenerationFake struct {
	owner    uuid.UUID
	input    service.TreatmentPlanGenerationInput
	replayed bool
	kind     trainingplan.ProposalKind
	err      error
	calls    int
}

func (f *planGenerationFake) GeneratePlanProposal(_ context.Context, owner uuid.UUID, in service.TreatmentPlanGenerationInput) (*service.TreatmentPlanGenerationResult, error) {
	f.calls++
	f.owner, f.input = owner, in
	if f.err != nil {
		return nil, f.err
	}
	kind := f.kind
	if kind == "" {
		kind = trainingplan.ProposalCreate
	}
	now := time.Now().UTC()
	return &service.TreatmentPlanGenerationResult{Replayed: f.replayed, Proposal: &model.PlanProposal{
		ID: uuid.New(), UserID: owner, Kind: kind, State: trainingplan.ProposalPending, InitiatedBy: trainingplan.InitiatorAgent,
		SourceID: "treatment_agent:test", Draft: []byte(`{"title":"Neck plan","goal":"g","relatedConditionIds":[],"relatedBodyRefs":[],"content":{"sessions":[{"templateId":"t1","title":"A","stages":[{"phase":"main","exercises":[{"planExerciseId":"e1","exerciseId":"chin_tuck","nameSnapshot":"Chin tuck","sets":2,"target":{"kind":"repetitions","min":8}}]}]}],"frequency":{},"schedule":{"mode":"frequency_only"},"constraints":[]}}`),
		IdempotencyKey: in.IdempotencyKey, ContentFingerprint: strings.Repeat("c", 64), CreatedAt: now, ExpiresAt: now.Add(trainingplan.ProposalTTL),
	}}, nil
}

func planGenerationRoute(t *testing.T, owner uuid.UUID, app *planGenerationFake) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	server := NewPublicServer()
	if app != nil {
		server = server.WithPlanGeneration(app)
	}
	RegisterRoutes(router, StrictHandler(server), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return router
}

const planGenerationPath = "/api/v1/plan-proposals/generations"

func TestGeneratePlanProposalReturnsPendingAgentProposal(t *testing.T) {
	owner := uuid.New()
	fake := &planGenerationFake{}
	body := map[string]any{"diagnosisRevision": 3, "userConstraints": map[string]any{"minutes": 20}, "changeReason": " first ", "idempotencyKey": "gen-1"}
	rec := doPlanRequest(t, planGenerationRoute(t, owner, fake), http.MethodPost, planGenerationPath, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.owner != owner || fake.input.ExpectedDiagnosisRevision == nil || *fake.input.ExpectedDiagnosisRevision != 3 || fake.input.IdempotencyKey != "gen-1" ||
		fake.input.ChangeReason != "first" || fake.input.UserConstraints["minutes"] != float64(20) {
		t.Fatalf("command: %+v", fake.input)
	}
	for _, want := range []string{`"initiatedBy":"agent"`, `"state":"pending"`, `"kind":"create"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %s in %s", want, rec.Body.String())
		}
	}

	fake.replayed = true
	if rec = doPlanRequest(t, planGenerationRoute(t, owner, fake), http.MethodPost, planGenerationPath, map[string]any{"idempotencyKey": "gen-1"}); rec.Code != http.StatusOK {
		t.Fatalf("replay status=%d", rec.Code)
	}
	if fake.input.ExpectedDiagnosisRevision != nil {
		t.Fatal("an omitted revision must generate from the current Diagnosis without a pin")
	}
}

func TestGeneratePlanProposalRejectsCallerAuthorityInBody(t *testing.T) {
	owner := uuid.New()
	for name, body := range map[string]map[string]any{
		"no key":          {"changeReason": "x"},
		"owner":           {"idempotencyKey": "k", "userId": uuid.NewString()},
		"agent draft":     {"idempotencyKey": "k", "draft": map[string]any{"title": "t"}},
		"initiator":       {"idempotencyKey": "k", "initiatedBy": "user"},
		"decision":        {"idempotencyKey": "k", "decisionId": uuid.NewString()},
		"zero revision":   {"idempotencyKey": "k", "diagnosisRevision": 0},
		"padded key":      {"idempotencyKey": " k "},
		"bad revision":    {"idempotencyKey": "k", "diagnosisRevision": "r1"},
		"legacy analysis": {"idempotencyKey": "k", "diagnosisAnalysisId": uuid.NewString()},
	} {
		fake := &planGenerationFake{}
		rec := doPlanRequest(t, planGenerationRoute(t, owner, fake), http.MethodPost, planGenerationPath, body)
		if rec.Code != http.StatusBadRequest || fake.calls != 0 {
			t.Fatalf("%s: status=%d calls=%d body=%s", name, rec.Code, fake.calls, rec.Body.String())
		}
	}
	if rec := doPlanRequest(t, planGenerationRoute(t, uuid.Nil, &planGenerationFake{}), http.MethodPost, planGenerationPath, map[string]any{"idempotencyKey": "k"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", rec.Code)
	}
	if rec := doPlanRequest(t, planGenerationRoute(t, owner, nil), http.MethodPost, planGenerationPath, map[string]any{"idempotencyKey": "k"}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured: %d", rec.Code)
	}
}

func TestGeneratePlanProposalErrorMapping(t *testing.T) {
	owner := uuid.New()
	cases := []struct {
		err  error
		code int
		key  string
	}{
		{service.ErrTreatmentDiagnosisNotReady, 409, "DIAGNOSIS_NOT_READY"},
		{service.ErrTreatmentDiagnosisStale, 409, "DIAGNOSIS_STALE"},
		{fmt.Errorf("%w: governance", service.ErrTreatmentSafetyBlocked), 409, "SAFETY_BLOCKED"},
		{service.ErrTreatmentPlanUnavailable, 502, "AGENT_OUTPUT_REJECTED"},
		{fmt.Errorf("%w: agent id", service.ErrTreatmentPlanOutputInvalid), 502, "AGENT_OUTPUT_REJECTED"},
		{service.ErrTreatmentConfigurationMismatch, 502, "AGENT_OUTPUT_REJECTED"},
		{service.ErrTreatmentPlanGenerationUnavailable, 503, "PLAN_GENERATION_UNAVAILABLE"},
		{service.ErrPlanConflict, 409, "REVISION_CONFLICT"},
		{service.ErrPlanContextUnavailable, 503, "PLAN_CONTEXT_UNAVAILABLE"},
		{service.ErrPlanInvalid, 400, "INVALID_REQUEST"},
		{errors.New("database down"), 500, "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		fake := &planGenerationFake{err: tc.err}
		rec := doPlanRequest(t, planGenerationRoute(t, owner, fake), http.MethodPost, planGenerationPath, map[string]any{"idempotencyKey": "k"})
		if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.key) {
			t.Fatalf("%v -> %d %s", tc.err, rec.Code, rec.Body.String())
		}
	}
	// A service result that is not a pending agent create is a contract violation.
	fake := &planGenerationFake{kind: trainingplan.ProposalUpdate}
	if rec := doPlanRequest(t, planGenerationRoute(t, owner, fake), http.MethodPost, planGenerationPath, map[string]any{"idempotencyKey": "k"}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("inconsistent result: %d", rec.Code)
	}
}
