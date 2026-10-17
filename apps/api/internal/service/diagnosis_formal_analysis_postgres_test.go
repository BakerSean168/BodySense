package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

type fakeFormalDiagnosisAgent struct {
	calls  int
	output func(DiagnosisRequest) (json.RawMessage, error)
}

func (f *fakeFormalDiagnosisAgent) AnalyzeDiagnosis(_ context.Context, req DiagnosisRequest) (json.RawMessage, error) {
	f.calls++
	return f.output(req)
}

func formalAgentResponse(req DiagnosisRequest, formal any) json.RawMessage {
	payload := map[string]any{
		"status": "completed", "scope": "full_body", "summary": "synthetic",
		"candidates":      []any{map[string]any{"name": "Synthetic A syndrome", "confidence": "中"}},
		"governance":      map[string]any{"kind": "diagnosis", "verdict": "accepted", "reasons": []any{}, "issues": []any{}},
		"safety_findings": []any{}, "information_gaps": []any{},
		"agent_configuration":  map[string]any{"id": req.ConfigurationID, "role": "diagnosis"},
		"execution_provenance": map[string]any{"status": "completed", "runtime": "synthetic"},
	}
	if formal != nil {
		payload["formal_proposals"] = formal
	}
	encoded, _ := json.Marshal(payload)
	return encoded
}

func newFormalAnalysisForTest(t *testing.T, agent diagnosisFormalAgent) *DiagnosisFormalAnalysisService {
	t.Helper()
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	return NewDiagnosisFormalAnalysisService(openOnboardingContextIntegrationDB(t), nil, agent, policy)
}

func TestDiagnosisFormalAnalysisAgentOutputBecomesProposalsPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f := seedDiagnosisDecisionFixture(t, db)
	var seen DiagnosisRequest
	agent := &fakeFormalDiagnosisAgent{output: func(req DiagnosisRequest) (json.RawMessage, error) {
		seen = req
		return formalAgentResponse(req, diagnosisAgentOutputFor(f, "Synthetic A syndrome")), nil
	}}
	svc := newFormalAnalysisForTest(t, agent)
	cmd := DiagnosisFormalAnalysisCommand{RequestEvidenceID: f.request.ID, Trigger: model.ConditionNewAnalysis, ScopeDescription: "Synthetic", ExpectedRevision: 0, IdempotencyKey: uuid.NewString()}
	r, err := svc.Start(ctx, f.owner, cmd)
	if err != nil || r.Outcome != "proposals" || len(r.ProposalIDs) != 1 || agent.calls != 1 {
		t.Fatalf("start %+v calls=%d %v", r, agent.calls, err)
	}
	if seen.BodyStateRevision != 1 || seen.SafetyEnvelope == nil || seen.UserID != f.owner.String() {
		t.Fatalf("agent input not pinned: %+v", seen)
	}
	run, err := repository.NewDiagnosisRepository(db).GetAnalysisRun(ctx, f.owner, r.RunID)
	if err != nil {
		t.Fatal(err)
	}
	// The frozen replay input is exactly what Go sent; replay never reads live state.
	var replay DiagnosisReplayInput
	if err := json.Unmarshal(run.ReplayInput, &replay); err != nil || replay.BodyStateRevision != 1 || !diagnosisJSONEqual(replay.BodyState, seen.BodyState) {
		t.Fatalf("replay input %+v %v", replay.BodyStateRevision, err)
	}
	if len(run.InputEvidenceRefs) != 1 || run.InputEvidenceRefs[0].EvidenceID != f.fact.ID {
		t.Fatalf("frozen evidence %+v", run.InputEvidenceRefs)
	}
	// The legacy per-inference analysis store is not written by the formal flow.
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_analyses", 0)
	assertNoAgentAuthority(t, db, f.owner, 1)
	// A lost response is replayed without re-running the Agent.
	again, err := svc.Start(ctx, f.owner, cmd)
	if err != nil || !again.Replayed || again.RunID != r.RunID || agent.calls != 1 {
		t.Fatalf("replay %+v calls=%d %v", again, agent.calls, err)
	}
}

func TestDiagnosisFormalAnalysisFailsClosedPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	cases := map[string]struct {
		output func(diagnosisDecisionFixture, DiagnosisRequest) (json.RawMessage, error)
		want   error
	}{
		"agent error": {func(diagnosisDecisionFixture, DiagnosisRequest) (json.RawMessage, error) {
			return nil, errors.New("synthetic outage")
		}, ErrDiagnosisAgentUnavailable},
		"not json": {func(diagnosisDecisionFixture, DiagnosisRequest) (json.RawMessage, error) {
			return json.RawMessage(`[1]`), nil
		}, ErrDiagnosisAgentOutputInvalid},
		"legacy output without formal proposals": {func(_ diagnosisDecisionFixture, r DiagnosisRequest) (json.RawMessage, error) {
			return formalAgentResponse(r, nil), nil
		}, ErrDiagnosisAgentOutputInvalid},
		"foreign configuration": {func(f diagnosisDecisionFixture, r DiagnosisRequest) (json.RawMessage, error) {
			r.ConfigurationID = "diagnosis-other"
			return formalAgentResponse(r, diagnosisAgentOutputFor(f, "Synthetic A syndrome")), nil
		}, ErrDiagnosisAgentOutputInvalid},
		"malformed formal proposals": {func(_ diagnosisDecisionFixture, r DiagnosisRequest) (json.RawMessage, error) {
			return formalAgentResponse(r, map[string]any{"schemaRevision": DiagnosisAgentProposalsSchemaV1, "outcome": "proposals", "summary": "", "proposals": []any{map[string]any{"changeKind": "create"}}}), nil
		}, ErrDiagnosisAgentOutputInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := seedDiagnosisDecisionFixture(t, db)
			agent := &fakeFormalDiagnosisAgent{output: func(r DiagnosisRequest) (json.RawMessage, error) { return tc.output(f, r) }}
			cmd := DiagnosisFormalAnalysisCommand{RequestEvidenceID: f.request.ID, Trigger: model.ConditionNewAnalysis, IdempotencyKey: uuid.NewString()}
			if _, err := newFormalAnalysisForTest(t, agent).Start(ctx, f.owner, cmd); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if n := diagnosisRunsForKey(t, db, f.owner, cmd.IdempotencyKey); n != 0 {
				t.Fatal("failed formal analysis left a run")
			}
			diagnosisDecisionCount(t, db, f.owner, "diagnosis_analyses", 0)
		})
	}
	t.Run("foreign request never reaches the agent", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		other := seedDiagnosisDecisionFixture(t, db)
		agent := &fakeFormalDiagnosisAgent{output: func(r DiagnosisRequest) (json.RawMessage, error) {
			return formalAgentResponse(r, diagnosisAgentOutputFor(other, "Synthetic A syndrome")), nil
		}}
		cmd := DiagnosisFormalAnalysisCommand{RequestEvidenceID: f.request.ID, Trigger: model.ConditionNewAnalysis, IdempotencyKey: uuid.NewString()}
		if _, err := newFormalAnalysisForTest(t, agent).Start(ctx, other.owner, cmd); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatalf("foreign request: %v", err)
		}
		if agent.calls != 0 {
			t.Fatal("foreign request reached the Agent")
		}
		if n := diagnosisRunsForKey(t, db, other.owner, cmd.IdempotencyKey); n != 0 {
			t.Fatal("foreign request wrote a run")
		}
	})
}
