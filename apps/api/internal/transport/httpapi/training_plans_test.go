package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/domain/trainingplan"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type trainingPlanFake struct {
	owner    uuid.UUID
	views    []service.TrainingPlanView
	proposal *model.PlanProposal
	propose  service.PlanProposalInput
	status   service.PlanStatusCommand
	decision service.PlanDecision
	err      error
}

func (f *trainingPlanFake) ListViews(_ context.Context, owner uuid.UUID) ([]service.TrainingPlanView, error) {
	f.owner = owner
	return f.views, f.err
}
func (f *trainingPlanFake) View(_ context.Context, owner, plan uuid.UUID) (*service.TrainingPlanView, error) {
	f.owner = owner
	if f.err != nil {
		return nil, f.err
	}
	for i := range f.views {
		if f.views[i].Plan.ID == plan {
			return &f.views[i], nil
		}
	}
	return nil, service.ErrPlanNotFound
}
func (f *trainingPlanFake) Revision(_ context.Context, owner, _, _ uuid.UUID) (*model.TrainingPlanRevision, error) {
	f.owner = owner
	return nil, service.ErrPlanNotFound
}
func (f *trainingPlanFake) ChangeStatus(_ context.Context, owner uuid.UUID, cmd service.PlanStatusCommand) (*model.TrainingPlanStatusEvent, error) {
	f.owner, f.status = owner, cmd
	if f.err != nil {
		return nil, f.err
	}
	return &model.TrainingPlanStatusEvent{ID: uuid.New(), UserID: owner, PlanID: cmd.PlanID, FromStatus: "active", ToStatus: cmd.Status,
		StatusRevision: cmd.ExpectedStatusRevision + 1, IdempotencyKey: cmd.IdempotencyKey, CreatedAt: time.Now()}, nil
}
func (f *trainingPlanFake) ListProposals(_ context.Context, owner uuid.UUID, _ trainingplan.ProposalState) ([]model.PlanProposal, error) {
	f.owner = owner
	return nil, f.err
}
func (f *trainingPlanFake) GetProposal(_ context.Context, owner, _ uuid.UUID) (*model.PlanProposal, error) {
	f.owner = owner
	return f.proposal, f.err
}
func (f *trainingPlanFake) Propose(_ context.Context, owner uuid.UUID, in service.PlanProposalInput) (*model.PlanProposal, error) {
	f.owner, f.propose = owner, in
	if f.err != nil {
		return nil, f.err
	}
	raw, _ := json.Marshal(in.Draft)
	now := time.Now().UTC()
	return &model.PlanProposal{ID: uuid.New(), UserID: owner, Kind: trainingplan.ProposalCreate, State: trainingplan.ProposalPending,
		InitiatedBy: in.InitiatedBy, SourceID: in.SourceID, Draft: raw, IdempotencyKey: in.IdempotencyKey,
		ContentFingerprint: strings.Repeat("a", 64), CreatedAt: now, ExpiresAt: now.Add(trainingplan.ProposalTTL)}, nil
}
func (f *trainingPlanFake) Accept(_ context.Context, owner uuid.UUID, d service.PlanDecision) (*service.PlanAcceptance, error) {
	f.owner, f.decision = owner, d
	return nil, f.err
}
func (f *trainingPlanFake) Reject(_ context.Context, owner uuid.UUID, d service.PlanDecision) (*model.PlanDecisionReceipt, error) {
	f.owner, f.decision = owner, d
	return nil, f.err
}

func trainingPlanRoute(t *testing.T, owner uuid.UUID, app trainingPlanApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer().WithTrainingPlans(app)), RouteSecurity{
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

func doPlanRequest(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func httpTestDraft() map[string]any {
	return map[string]any{
		"title": "Knee plan", "goal": "User goal", "relatedConditionIds": []string{}, "relatedBodyRefs": []any{},
		"content": map[string]any{
			"sessions": []any{map[string]any{"templateId": "t1", "title": "A", "stages": []any{map[string]any{"phase": "main",
				"exercises": []any{map[string]any{"planExerciseId": "e1", "exerciseId": "squat", "nameSnapshot": "Squat", "sets": 3,
					"target": map[string]any{"kind": "repetitions", "min": 8, "max": 12.5}}}}}}},
			"frequency": map[string]any{"sessionsPerWeek": 3}, "schedule": map[string]any{"mode": "frequency_only"}, "constraints": []string{},
		},
	}
}

func planView(owner uuid.UUID, gate safety.Decision) service.TrainingPlanView {
	plan, rev := uuid.New(), uuid.New()
	draft := model.TrainingPlanDraft{Title: "A", Goal: "g", Content: model.TrainingPlanContent{
		Sessions: []model.PlanSessionTemplate{{TemplateID: "t", Title: "S", Stages: []model.PlanStage{{Phase: "main", Exercises: []model.PlanExercise{{PlanExerciseID: "e", ExerciseID: "squat", NameSnapshot: "Squat", Sets: 1, Target: model.PlanTarget{Kind: "duration", Seconds: 30}}}}}}},
		Schedule: model.PlanSchedule{Mode: "frequency_only"}}}
	return service.TrainingPlanView{
		Plan:     model.TrainingPlan{ID: plan, UserID: owner, Title: "A", Goal: "g", Status: "active", CurrentAcceptedRevisionID: &rev, CreatedAt: time.Now()},
		Revision: model.TrainingPlanRevision{ID: rev, UserID: owner, PlanID: plan, SourceID: uuid.NewString()},
		Draft:    draft, Review: service.PlanReview{Recommended: true, Reason: service.PlanReviewContextChanged}, Gate: gate,
	}
}

func TestTrainingPlanRoutesRequireAuthenticatedOwner(t *testing.T) {
	fake := &trainingPlanFake{}
	r := trainingPlanRoute(t, uuid.Nil, fake)
	if rec := doPlanRequest(t, r, http.MethodGet, "/api/v1/training-plans", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	if fake.owner != uuid.Nil {
		t.Fatal("service reached without owner")
	}
}

func TestTrainingPlanListProjectsFailClosedGateAndReview(t *testing.T) {
	owner := uuid.New()
	// An unknown Go gate result must reach the client as unavailable, never allow.
	fake := &trainingPlanFake{views: []service.TrainingPlanView{
		planView(owner, safety.Decision{Result: "maybe", Reasons: []safety.Reason{safety.ReasonStateUnavailable}}),
		planView(owner, safety.Decision{Result: safety.ResultBlock, MatchedRestrictionIDs: []uuid.UUID{uuid.New()}}),
	}}
	rec := doPlanRequest(t, trainingPlanRoute(t, owner, fake), http.MethodGet, "/api/v1/training-plans", nil)
	if rec.Code != http.StatusOK || fake.owner != owner {
		t.Fatalf("status=%d owner=%s body=%s", rec.Code, fake.owner, rec.Body.String())
	}
	var body openapiv1.TrainingPlanRecordListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Plans) != 2 {
		t.Fatalf("decode: %v %s", err, rec.Body.String())
	}
	if g := body.Plans[0].ExecutionGate; g.Result != "unavailable" || g.PermitsExecution {
		t.Fatalf("unknown gate leaked as permit: %+v", g)
	}
	if g := body.Plans[1].ExecutionGate; g.Result != "block" || g.PermitsExecution || len(g.MatchedRestrictionIds) != 1 {
		t.Fatalf("block projection: %+v", g)
	}
	if body.Plans[0].Status != "active" || !body.Plans[0].Review.Recommended || body.Plans[0].RelatedConditionIds == nil {
		t.Fatalf("status/review projection: %+v", body.Plans[0])
	}
	if strings.Contains(rec.Body.String(), "user_id") || strings.Contains(rec.Body.String(), "review_recommended") {
		t.Fatalf("leaked owner field or fake status: %s", rec.Body.String())
	}
}

func TestTrainingPlanForeignProjectionIsNeverServed(t *testing.T) {
	owner := uuid.New()
	fake := &trainingPlanFake{views: []service.TrainingPlanView{planView(uuid.New(), safety.Decision{Result: safety.ResultAllow})}}
	rec := doPlanRequest(t, trainingPlanRoute(t, owner, fake), http.MethodGet, "/api/v1/training-plans/"+fake.views[0].Plan.ID.String(), nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("anomalous cross-owner row must error, got %d %s", rec.Code, rec.Body.String())
	}
	fake.err = service.ErrPlanNotFound
	rec = doPlanRequest(t, trainingPlanRoute(t, owner, fake), http.MethodGet, "/api/v1/training-plans/"+uuid.NewString(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign/missing must be 404, got %d", rec.Code)
	}
}

func TestCreatePlanProposalIsAlwaysUserInitiated(t *testing.T) {
	owner := uuid.New()
	fake := &trainingPlanFake{}
	body := map[string]any{"draft": httpTestDraft(), "idempotencyKey": "k1"}
	rec := doPlanRequest(t, trainingPlanRoute(t, owner, fake), http.MethodPost, "/api/v1/plan-proposals", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.owner != owner || fake.propose.InitiatedBy != trainingplan.InitiatorUser || fake.propose.SourceID != "user_api:k1" ||
		fake.propose.PlanID != uuid.Nil || fake.propose.Draft.Content.Sessions[0].Stages[0].Exercises[0].Target.Max == nil ||
		*fake.propose.Draft.Content.Sessions[0].Stages[0].Exercises[0].Target.Max != 12.5 {
		t.Fatalf("proposal input: %+v", fake.propose)
	}
	// A caller cannot claim agent authorship or smuggle an owner.
	body["initiatedBy"] = "agent"
	if rec = doPlanRequest(t, trainingPlanRoute(t, owner, fake), http.MethodPost, "/api/v1/plan-proposals", body); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field accepted: %d", rec.Code)
	}
	delete(body, "initiatedBy")
	body["planId"] = uuid.NewString()
	if rec = doPlanRequest(t, trainingPlanRoute(t, owner, fake), http.MethodPost, "/api/v1/plan-proposals", body); rec.Code != http.StatusBadRequest {
		t.Fatalf("update without expected revision: %d", rec.Code)
	}
}

func TestPlanDecisionErrorMapping(t *testing.T) {
	owner := uuid.New()
	cases := []struct {
		err  error
		code int
		key  string
	}{
		{&service.PlanAdmissionError{Verdict: trainingplan.ContextChanged}, 409, "CONTEXT_CHANGED"},
		{&service.PlanAdmissionError{Verdict: trainingplan.ProposalTimedOut}, 409, "PROPOSAL_EXPIRED"},
		{&service.PlanAdmissionError{Verdict: trainingplan.PlanRevisionChanged}, 409, "REVISION_CONFLICT"},
		{&service.PlanAdmissionError{Verdict: trainingplan.ContextUnavailable}, 503, "PLAN_CONTEXT_UNAVAILABLE"},
		{service.ErrPlanContextUnavailable, 503, "PLAN_CONTEXT_UNAVAILABLE"},
		{service.ErrPlanNotFound, 404, "NOT_FOUND"},
		{service.ErrPlanConflict, 409, "REVISION_CONFLICT"},
	}
	for _, tc := range cases {
		fake := &trainingPlanFake{err: tc.err}
		body := map[string]any{"decisionId": uuid.NewString(), "presentedFingerprint": strings.Repeat("b", 64), "idempotencyKey": "d1"}
		rec := doPlanRequest(t, trainingPlanRoute(t, owner, fake), http.MethodPost, "/api/v1/plan-proposals/"+uuid.NewString()+"/accept", body)
		if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.key) || fake.owner != owner {
			t.Fatalf("%v -> %d %s", tc.err, rec.Code, rec.Body.String())
		}
	}
}

func TestChangeTrainingPlanStatusContract(t *testing.T) {
	owner, plan := uuid.New(), uuid.New()
	fake := &trainingPlanFake{}
	r := trainingPlanRoute(t, owner, fake)
	rec := doPlanRequest(t, r, http.MethodPost, "/api/v1/training-plans/"+plan.String()+"/status",
		map[string]any{"status": "paused", "expectedStatusRevision": 4, "idempotencyKey": "s1"})
	if rec.Code != http.StatusOK || fake.status.PlanID != plan || fake.status.ExpectedStatusRevision != 4 || fake.owner != owner {
		t.Fatalf("status=%d cmd=%+v body=%s", rec.Code, fake.status, rec.Body.String())
	}
	// review_recommended is not a status and is rejected at the contract.
	rec = doPlanRequest(t, r, http.MethodPost, "/api/v1/training-plans/"+plan.String()+"/status",
		map[string]any{"status": "review_recommended", "expectedStatusRevision": 5, "idempotencyKey": "s2"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("review_recommended accepted: %d", rec.Code)
	}
	fake.err = service.ErrPlanConflict
	rec = doPlanRequest(t, r, http.MethodPost, "/api/v1/training-plans/"+plan.String()+"/status",
		map[string]any{"status": "active", "expectedStatusRevision": 0, "idempotencyKey": "s3"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale expected status revision: %d", rec.Code)
	}
}
