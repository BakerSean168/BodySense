package service

import (
	"fmt"
)

// TreatmentDecisionPolicyV1 keeps its historical identity: Treatment Agent
// configurations pin it. Since TP-002 slice 3 only the generation phase
// remains; acceptance is the user's TrainingPlan decision.
const (
	TreatmentDecisionPolicyV1   = "treatment-go-acceptance-v1"
	TreatmentDecisionTraceV1    = "treatment-decision-trace-v1"
	TreatmentDecisionGeneration = "generation"
)

type TreatmentDecisionOutcome string

const (
	TreatmentAllowProposal TreatmentDecisionOutcome = "allow-proposal"
	TreatmentBlock         TreatmentDecisionOutcome = "block"
)

type TreatmentDecision struct {
	PolicyRevision string                   `json:"policy_revision"`
	Phase          string                   `json:"phase"`
	Outcome        TreatmentDecisionOutcome `json:"outcome"`
	Reasons        []string                 `json:"reasons"`
}

// TreatmentDecisionFacts are read from the native Diagnosis head and the
// current BodyState Safety projection (DX-RETIRE-001): generation qualifies on
// the user's adopted Conditions at one Diagnosis revision, never on an Agent
// analysis or its candidates.
type TreatmentDecisionFacts struct {
	DiagnosisRevision        int64             `json:"diagnosis_revision"`
	AdoptedConditionCount    int               `json:"adopted_condition_count"`
	SafetyEnvelope           *SafetyEnvelopeV2 `json:"safety_envelope"`
	CurrentBodyStateRevision int64             `json:"current_body_state_revision"`
}

// EvaluateTreatmentDecision is the Go-owned deny-overrides authority for
// Treatment Agent generation. Model confidence and Agent prose are
// intentionally absent from the policy facts.
func EvaluateTreatmentDecision(
	policyRevision string,
	phase string,
	facts TreatmentDecisionFacts,
) TreatmentDecision {
	decision := TreatmentDecision{
		PolicyRevision: policyRevision,
		Phase:          phase,
		Outcome:        TreatmentBlock,
		Reasons:        []string{},
	}
	if policyRevision != TreatmentDecisionPolicyV1 {
		decision.Reasons = []string{"unsupported_decision_policy_revision"}
		return decision
	}
	if phase != TreatmentDecisionGeneration {
		decision.Reasons = []string{"unsupported_decision_phase"}
		return decision
	}
	if !treatmentDiagnosisFactsReady(facts) {
		decision.Reasons = []string{"adopted_condition_required"}
		return decision
	}

	activeSafety, err := strictTreatmentSafetyReview(facts.SafetyEnvelope, facts.CurrentBodyStateRevision)
	if err != nil {
		decision.Reasons = []string{"malformed_or_unknown_safety_state", err.Error()}
		return decision
	}
	if activeSafety {
		decision.Reasons = []string{"active_body_state_safety_concern"}
		return decision
	}
	// Complete Safety coverage is required, not merely the absence of blockers:
	// an unscreened BodyState is never treated as safe for plan generation.
	if !facts.SafetyEnvelope.Coverage.Complete {
		decision.Reasons = []string{"safety_coverage_incomplete"}
		return decision
	}
	decision.Outcome = TreatmentAllowProposal
	return decision
}

// treatmentDiagnosisFactsReady requires at least one adopted Condition at a
// committed Diagnosis revision. Inactive Conditions, pending proposals and
// recorded-but-unsettled decisions never qualify.
func treatmentDiagnosisFactsReady(facts TreatmentDecisionFacts) bool {
	return facts.DiagnosisRevision > 0 && facts.AdoptedConditionCount > 0
}

// strictTreatmentSafetyReview deliberately treats malformed or internally
// inconsistent durable safety facts as an authority failure rather than as safe.
func strictTreatmentSafetyReview(envelope *SafetyEnvelopeV2, revision int64) (bool, error) {
	if envelope == nil || envelope.SchemaRevision != SafetyEnvelopeSchemaV2 || envelope.PolicyRevision != SafetyEnvelopePolicyV1 || envelope.BodyStateRevision != revision || envelope.Coverage.Revision != SafetyCoverageRevisionV1 || envelope.Coverage.CaptureRevision != SafetyCaptureRevisionV1 || envelope.RequiresReview != (len(envelope.ActiveBlockers) > 0) {
		return false, fmt.Errorf("native Safety unavailable or not pinned")
	}
	if envelope.Coverage.Complete != (len(envelope.Coverage.CoveredSourceRefs) > 0 && len(envelope.Coverage.IncompleteSourceRefs) == 0) || len(envelope.Coverage.RequiredConcepts) != len(structuredSafetyDetails) {
		return false, fmt.Errorf("inconsistent native safety coverage")
	}
	for i, field := range structuredSafetyDetails {
		if envelope.Coverage.RequiredConcepts[i] != field.key {
			return false, fmt.Errorf("unknown native safety coverage concept")
		}
	}

	if len(envelope.Coverage.IncompleteSourceRefs) > 0 {
		return true, nil
	}
	for _, blocker := range envelope.ActiveBlockers {
		switch blocker.RiskLevel {
		case "restricted", "urgent":
			return true, nil
		case "advisory":
		default:
			return false, fmt.Errorf("unknown native safety risk level")
		}
	}
	return false, nil
}
