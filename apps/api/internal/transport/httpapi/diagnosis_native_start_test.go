package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeNativeStarter struct {
	receipt *service.DiagnosisProposalIngestionReceipt
	err     error
	owner   uuid.UUID
	cmd     service.DiagnosisFormalAnalysisCommand
	called  int
}

func (f *fakeNativeStarter) Start(_ context.Context, owner uuid.UUID, cmd service.DiagnosisFormalAnalysisCommand) (*service.DiagnosisProposalIngestionReceipt, error) {
	f.owner, f.cmd = owner, cmd
	f.called++
	return f.receipt, f.err
}

func nativeStartRoute(t *testing.T, owner uuid.UUID, starter nativeDiagnosisStartApplication, reader nativeDiagnosisRunReadApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer().WithNativeDiagnosisAnalysis(starter, reader)), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator: func(c *gin.Context) { c.Next() }, Validator: RequestValidator(spec),
	})
	return r
}

func startedRunProjection(owner, run, proposal uuid.UUID, outcome *string) *service.NativeDiagnosisRunProjection {
	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := model.DiagnosisEvidenceRef{EvidenceType: model.DiagnosisBodyFact, EvidenceID: uuid.New(), Version: "1"}
	p := &service.NativeDiagnosisRunProjection{
		OwnerID: owner,
		NativeDiagnosisRunSummary: service.NativeDiagnosisRunSummary{
			RunID: run, Trigger: model.ConditionNewAnalysis, BasedOnBodyStateRevision: 1, StartedAt: now, State: "open", Outcome: outcome,
		},
		RequestMessageID: uuid.New(), RequestEvidenceID: uuid.New(), RequestSourceVersion: 1,
		Scope:             model.ConditionAnalysisScope{ConditionIDs: []uuid.UUID{}, EvidenceRefs: []model.DiagnosisEvidenceRef{}, Description: ""},
		InputEvidenceRefs: []model.DiagnosisEvidenceRef{ref}, AgentHypotheses: []model.ConditionAgentHypothesis{},
		Events: []service.NativeDiagnosisRunEvent{{Sequence: 1, Kind: "run_started", RecordedAt: now}}, Proposals: []service.NativeDiagnosisRunProposal{},
	}
	if outcome != nil {
		p.State = "closed_without_change"
		return p
	}
	p.Proposals = append(p.Proposals, service.NativeDiagnosisRunProposal{
		ProposalID: proposal, RunID: run, ChangeKind: model.DiagnosisCreate,
		Content:            json.RawMessage(`{"proposedEntry":{"medicalName":"Synthetic named syndrome"}}`),
		ContentFingerprint: strings.Repeat("e", 64), EvidenceRefs: []model.DiagnosisEvidenceRef{ref},
		CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: now, State: "pending",
	})
	return p
}

func TestNativeDiagnosisStartHTTP(t *testing.T) {
	owner, run, proposal, request := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	body := map[string]any{"requestEvidenceId": request, "trigger": "new_analysis", "expectedRevision": 0, "idempotencyKey": "start-1", "scopeDescription": "knee"}
	const path = "/api/v1/diagnosis/analysis-runs"

	starter := &fakeNativeStarter{receipt: &service.DiagnosisProposalIngestionReceipt{RunID: run, Outcome: "proposals", ProposalIDs: []uuid.UUID{proposal}}}
	reader := &nativeRunReaderFake{read: startedRunProjection(owner, run, proposal, nil)}
	r := postNativeDiagnosis(t, nativeStartRoute(t, owner, starter, reader), path, body)
	if r.Code != 201 || starter.owner != owner || starter.cmd.RequestEvidenceID != request || starter.cmd.ScopeDescription != "knee" || reader.owner != owner || reader.run != run {
		t.Fatalf("start %d %s %+v", r.Code, r.Body.String(), starter.cmd)
	}
	starter.receipt.Replayed = true
	if r := postNativeDiagnosis(t, nativeStartRoute(t, owner, starter, reader), path, body); r.Code != 200 {
		t.Fatalf("replay %d", r.Code)
	}

	blocked := "safety_blocked"
	safety := &fakeNativeStarter{receipt: &service.DiagnosisProposalIngestionReceipt{RunID: run, Outcome: "safety_blocked", ProposalIDs: []uuid.UUID{}}, err: service.ErrDiagnosisSafetyBlocked}
	if r := postNativeDiagnosis(t, nativeStartRoute(t, owner, safety, &nativeRunReaderFake{read: startedRunProjection(owner, run, proposal, &blocked)}), path, body); r.Code != 201 || !strings.Contains(r.Body.String(), "safety_blocked") {
		t.Fatalf("safety terminal %d %s", r.Code, r.Body.String())
	}

	// A receipt whose persisted projection disagrees is not reported as success.
	mismatch := &fakeNativeStarter{receipt: &service.DiagnosisProposalIngestionReceipt{RunID: run, Outcome: "proposals", ProposalIDs: []uuid.UUID{proposal, uuid.New()}}}
	if r := postNativeDiagnosis(t, nativeStartRoute(t, owner, mismatch, reader), path, body); r.Code != 503 {
		t.Fatalf("inconsistent projection %d", r.Code)
	}

	for name, tc := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"foreign request":  {model.ErrDiagnosisNotFound, 404, "NOT_FOUND"},
		"stale diagnosis":  {service.ErrDiagnosisDecisionConflict, 409, "REVISION_CONFLICT"},
		"stale body state": {service.ErrDiagnosisIngestionStale, 409, "REVISION_CONFLICT"},
		"not ready":        {service.ErrDiagnosisBodyStateNotReady, 409, "BODY_STATE_NOT_READY"},
		"malformed agent":  {service.ErrDiagnosisAgentOutputInvalid, 503, "AGENT_OUTPUT_REJECTED"},
		"agent down":       {service.ErrDiagnosisAgentUnavailable, 503, "AI_SERVICE_ERROR"},
		"safety down":      {service.ErrDiagnosisSafetyUnavailable, 503, "SAFETY_UNAVAILABLE"},
	} {
		t.Run(name, func(t *testing.T) {
			r := postNativeDiagnosis(t, nativeStartRoute(t, owner, &fakeNativeStarter{err: tc.err}, reader), path, body)
			if r.Code != tc.status || !strings.Contains(r.Body.String(), tc.code) {
				t.Fatalf("%d %s", r.Code, r.Body.String())
			}
		})
	}

	never := &fakeNativeStarter{}
	if r := postNativeDiagnosis(t, nativeStartRoute(t, uuid.Nil, never, reader), path, body); r.Code != 401 {
		t.Fatalf("unauthenticated %d", r.Code)
	}
	for name, bad := range map[string]map[string]any{
		"caller owner":      {"requestEvidenceId": request, "trigger": "new_analysis", "expectedRevision": 0, "idempotencyKey": "k", "userId": uuid.New()},
		"agent content":     {"requestEvidenceId": request, "trigger": "new_analysis", "expectedRevision": 0, "idempotencyKey": "k", "proposals": []any{}},
		"unsupported scope": {"requestEvidenceId": request, "trigger": "full_reanalysis", "expectedRevision": 0, "idempotencyKey": "k"},
		"missing request":   {"trigger": "new_analysis", "expectedRevision": 0, "idempotencyKey": "k"},
	} {
		if r := postNativeDiagnosis(t, nativeStartRoute(t, owner, never, reader), path, bad); r.Code != 400 {
			t.Fatalf("%s accepted: %d", name, r.Code)
		}
	}
	if never.called != 0 {
		t.Fatal("invalid request reached the formal analysis service")
	}
}
