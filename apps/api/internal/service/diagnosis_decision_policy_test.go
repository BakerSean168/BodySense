package service

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
)

type diagnosisDecisionPolicyFixture struct {
	Name            string          `json:"name"`
	SafetyState     json.RawMessage `json:"safety_state"`
	Payload         map[string]any  `json:"payload"`
	ExpectedOutcome string          `json:"expected_outcome"`
}

func TestDiagnosisDecisionPolicyV1Fixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/diagnosis_decision_policy_v1.json")
	if err != nil {
		t.Fatalf("read decision policy fixture: %v", err)
	}
	var fixtures []diagnosisDecisionPolicyFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("decode decision policy fixture: %v", err)
	}
	if len(fixtures) < 8 {
		t.Fatalf("expected broad decision policy fixture, got %d cases", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			decision := EvaluateDiagnosisDecision(
				DiagnosisDecisionPolicyV1,
				fixture.SafetyState,
				fixture.Payload,
			)
			if string(decision.Outcome) != fixture.ExpectedOutcome {
				t.Fatalf("expected %s, got %#v", fixture.ExpectedOutcome, decision)
			}
		})
	}
}

func TestDiagnosisDecisionPolicyUnknownRevisionFailsClosed(t *testing.T) {
	decision := EvaluateDiagnosisDecision(
		"diagnosis-decision-policy-does-not-exist",
		json.RawMessage(`{}`),
		map[string]any{
			"status":     "completed",
			"candidates": []any{map[string]any{"name": "candidate", "confidence": "高"}},
			"governance": map[string]any{"verdict": "accepted"},
		},
	)
	if decision.Outcome != DiagnosisBlock {
		t.Fatalf("unknown policy revision must fail closed, got %#v", decision)
	}
}

func TestApplyDiagnosisDecisionSuppressesCandidatesWhenAuthorityDeniesNormalDelivery(t *testing.T) {
	payload := map[string]any{
		"status":                 "completed",
		"scope":                  "full_body",
		"summary":                "model summary",
		"candidates":             []any{map[string]any{"name": "candidate", "confidence": "高"}},
		"cross_concern_patterns": []any{"pattern"},
		"citations":              []any{map[string]any{"evidence_id": "e1"}},
		"governance":             map[string]any{"verdict": "accepted"},
	}
	decision := DiagnosisDecision{
		PolicyRevision: DiagnosisDecisionPolicyV1,
		Outcome:        DiagnosisBlock,
		Reasons:        []string{"hard_block"},
	}
	result := ApplyDiagnosisDecision(payload, decision)
	if result["status"] != "safety_blocked" {
		t.Fatalf("block must become safety_blocked, got %#v", result["status"])
	}
	if got := len(result["candidates"].([]any)); got != 0 {
		t.Fatalf("blocked output must not expose candidates, got %d", got)
	}
	if got := len(result["citations"].([]any)); got != 0 {
		t.Fatalf("blocked output must not expose citations, got %d", got)
	}
	if _, ok := result["decision_authority"].(DiagnosisDecision); !ok {
		t.Fatalf("decision authority metadata missing: %#v", result["decision_authority"])
	}
	if len(payload["candidates"].([]any)) != 1 {
		t.Fatal("ApplyDiagnosisDecision must not mutate original model payload")
	}
}

func TestDecisionAuthorityAbstainBecomesDurableNonCandidateAnalysis(t *testing.T) {
	payload := map[string]any{
		"status":  "completed",
		"scope":   "full_body",
		"summary": "model wanted to emit a candidate",
		"candidates": []any{
			map[string]any{"name": "candidate", "confidence": "高"},
		},
		"governance": map[string]any{"verdict": "accepted"},
		"evidence_acquisition": map[string]any{
			"unresolved_critical_gaps": []any{map[string]any{"gap_id": "critical"}},
		},
	}
	decision := EvaluateDiagnosisDecision(DiagnosisDecisionPolicyV1, json.RawMessage(`{}`), payload)
	if decision.Outcome != DiagnosisAbstain {
		t.Fatalf("expected abstain, got %#v", decision)
	}
	authorized := ApplyDiagnosisDecision(payload, decision)
	raw, err := json.Marshal(authorized)
	if err != nil {
		t.Fatalf("marshal authorized Diagnosis: %v", err)
	}
	repo := &fakeDiagnosisAnalysisRepository{}
	analysis, err := NewDiagnosisAnalysisService(repo).PersistAIResult(
		context.Background(), uuid.New(), 42, raw,
	)
	if err != nil {
		t.Fatalf("persist authorized Diagnosis: %v", err)
	}
	if analysis.Status != "insufficient_information" || len(analysis.Candidates) != 0 {
		t.Fatalf("abstain must persist without ordinary candidates: %#v", analysis)
	}
	public := NewDiagnosisAnalysisService(repo).PublicPayload(analysis)
	decisionRaw, ok := public["decision_authority"].(json.RawMessage)
	if !ok || !json.Valid(decisionRaw) {
		t.Fatalf("durable read model lost decision authority: %#v", public["decision_authority"])
	}
}

func TestPythonRejectedStrippedPayloadStillBecomesHardBlock(t *testing.T) {
	decision := EvaluateDiagnosisDecision(
		DiagnosisDecisionPolicyV1,
		json.RawMessage(`{}`),
		map[string]any{
			"governance":      map[string]any{"verdict": "rejected"},
			"safety_fallback": "blocked by Python runtime guard",
		},
	)
	if decision.Outcome != DiagnosisBlock {
		t.Fatalf("Python hard rejection must be a Go hard block, got %#v", decision)
	}
	if len(decision.Reasons) == 0 || decision.Reasons[0] != "agent_output_failed_safety_governance" {
		t.Fatalf("unexpected hard-block reason: %#v", decision.Reasons)
	}
}
