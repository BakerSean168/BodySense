package service

import (
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/model"
)

func readyTreatmentDecisionFacts() TreatmentDecisionFacts {
	return TreatmentDecisionFacts{
		DiagnosisStatus:          "completed",
		CandidateCount:           1,
		FreshnessState:           model.DiagnosisFreshnessFresh,
		SafetyState:              json.RawMessage(`{}`),
		CandidateAssessmentReady: true,
		ProposalAcceptanceState:  model.TreatmentAcceptanceProposed,
		SourceBodyStateRevision:  7,
		CurrentBodyStateRevision: 7,
		MaterialReviewStatus:     model.TreatmentStatusActive,
		MaterialReviewReasons:    []TreatmentReviewReason{},
	}
}

func TestTreatmentDecisionPolicyAllowsGenerationAndAcceptance(t *testing.T) {
	facts := readyTreatmentDecisionFacts()
	generation := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionGeneration, facts)
	if generation.Outcome != TreatmentAllowProposal || len(generation.Reasons) != 0 {
		t.Fatalf("unexpected generation decision: %#v", generation)
	}
	acceptance := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionAcceptance, facts)
	if acceptance.Outcome != TreatmentAllowAcceptance || len(acceptance.Reasons) != 0 {
		t.Fatalf("unexpected acceptance decision: %#v", acceptance)
	}
}

func TestTreatmentDecisionPolicyDenyOverrides(t *testing.T) {
	cases := []struct {
		name   string
		phase  string
		mutate func(*TreatmentDecisionFacts)
		reason string
	}{
		{"diagnosis", TreatmentDecisionGeneration, func(f *TreatmentDecisionFacts) { f.DiagnosisStatus = "insufficient_information" }, "diagnosis_not_ready"},
		{"freshness", TreatmentDecisionGeneration, func(f *TreatmentDecisionFacts) { f.FreshnessState = model.DiagnosisFreshnessStale }, "diagnosis_not_fresh"},
		{"safety", TreatmentDecisionGeneration, func(f *TreatmentDecisionFacts) {
			f.SafetyState = json.RawMessage(`{"has_red_flags":true,"status":"requires_review"}`)
		}, "active_body_state_safety_concern"},
		{"assessment", TreatmentDecisionGeneration, func(f *TreatmentDecisionFacts) { f.CandidateAssessmentReady = false }, "candidate_assessment_required"},
		{"proposal-state", TreatmentDecisionAcceptance, func(f *TreatmentDecisionFacts) { f.ProposalAcceptanceState = model.TreatmentAcceptanceAccepted }, "proposal_not_proposed"},
		{"revision-regressed", TreatmentDecisionAcceptance, func(f *TreatmentDecisionFacts) { f.CurrentBodyStateRevision = 6 }, "body_state_revision_regressed"},
		{"material-change", TreatmentDecisionAcceptance, func(f *TreatmentDecisionFacts) {
			f.CurrentBodyStateRevision = 8
			f.MaterialReviewStatus = model.TreatmentStatusReviewRecommended
			f.MaterialReviewReasons = []TreatmentReviewReason{{Code: "material_related_body_state_change"}}
		}, "material_related_body_state_change"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := readyTreatmentDecisionFacts()
			tc.mutate(&facts)
			decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, tc.phase, facts)
			if decision.Outcome != TreatmentBlock || len(decision.Reasons) == 0 || decision.Reasons[0] != tc.reason {
				t.Fatalf("unexpected decision: %#v", decision)
			}
		})
	}
}

func TestTreatmentDecisionPolicyMalformedSafetyFailsClosed(t *testing.T) {
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"has_red_flags":"yes","status":"requires_review"}`),
		json.RawMessage(`{"has_red_flags":false,"status":"requires_review"}`),
		json.RawMessage(`{"has_red_flags":true,"status":"mystery"}`),
	} {
		facts := readyTreatmentDecisionFacts()
		facts.SafetyState = raw
		decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionGeneration, facts)
		if decision.Outcome != TreatmentBlock || len(decision.Reasons) == 0 || decision.Reasons[0] != "malformed_or_unknown_safety_state" {
			t.Fatalf("malformed safety must fail closed: %#v", decision)
		}
	}
}

func TestTreatmentDecisionPolicyUnknownRevisionAndPhaseFailClosed(t *testing.T) {
	facts := readyTreatmentDecisionFacts()
	if decision := EvaluateTreatmentDecision("treatment-policy-v999", TreatmentDecisionGeneration, facts); decision.Outcome != TreatmentBlock || decision.Reasons[0] != "unsupported_decision_policy_revision" {
		t.Fatalf("unknown policy must fail closed: %#v", decision)
	}
	if decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, "surprise", facts); decision.Outcome != TreatmentBlock || decision.Reasons[0] != "unsupported_decision_phase" {
		t.Fatalf("unknown phase must fail closed: %#v", decision)
	}
}
