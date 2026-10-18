package service

import (
	"testing"
)

func readyTreatmentDecisionFacts() TreatmentDecisionFacts {
	return TreatmentDecisionFacts{
		DiagnosisRevision:        3,
		AdoptedConditionCount:    1,
		SafetyEnvelope:           testCompleteSafetyEnvelope(7),
		CurrentBodyStateRevision: 7,
	}
}

func TestTreatmentDecisionPolicyAllowsGeneration(t *testing.T) {
	facts := readyTreatmentDecisionFacts()
	generation := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionGeneration, facts)
	if generation.Outcome != TreatmentAllowProposal || len(generation.Reasons) != 0 {
		t.Fatalf("unexpected generation decision: %#v", generation)
	}
	// Acceptance left the policy with Treatment (TP-002 slice 3).
	if acceptance := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, "acceptance", facts); acceptance.Outcome != TreatmentBlock ||
		acceptance.Reasons[0] != "unsupported_decision_phase" {
		t.Fatalf("acceptance must fail closed: %#v", acceptance)
	}
}

func TestTreatmentDecisionPolicyDenyOverrides(t *testing.T) {
	cases := []struct {
		name   string
		phase  string
		mutate func(*TreatmentDecisionFacts)
		reason string
	}{
		{"no adopted condition", TreatmentDecisionGeneration, func(f *TreatmentDecisionFacts) { f.AdoptedConditionCount = 0 }, "adopted_condition_required"},
		{"no diagnosis revision", TreatmentDecisionGeneration, func(f *TreatmentDecisionFacts) { f.DiagnosisRevision = 0 }, "adopted_condition_required"},
		{"safety", TreatmentDecisionGeneration, func(f *TreatmentDecisionFacts) {
			f.SafetyEnvelope = testBlockedSafetyEnvelope(7)
		}, "active_body_state_safety_concern"},
		{"incomplete safety coverage", TreatmentDecisionGeneration, func(f *TreatmentDecisionFacts) {
			f.SafetyEnvelope = testNativeSafetyEnvelope(7)
		}, "safety_coverage_incomplete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := readyTreatmentDecisionFacts()
			tc.mutate(&facts)
			facts.SafetyEnvelope.BodyStateRevision = facts.CurrentBodyStateRevision
			decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, tc.phase, facts)
			if decision.Outcome != TreatmentBlock || len(decision.Reasons) == 0 || decision.Reasons[0] != tc.reason {
				t.Fatalf("unexpected decision: %#v", decision)
			}
		})
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

func TestTreatmentDecisionPolicyUnavailableSafetyFailsClosed(t *testing.T) {
	facts := readyTreatmentDecisionFacts()
	for _, envelope := range []*SafetyEnvelopeV2{nil, {}, testCompleteSafetyEnvelope(8)} {
		facts.SafetyEnvelope = envelope
		decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionGeneration, facts)
		if decision.Outcome != TreatmentBlock || decision.Reasons[0] != "malformed_or_unknown_safety_state" {
			t.Fatalf("unavailable Safety allowed: %+v", decision)
		}
	}
}
