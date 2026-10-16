package service

import (
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
)

const (
	TreatmentDecisionPolicyV1   = "treatment-go-acceptance-v1"
	TreatmentDecisionTraceV1    = "treatment-decision-trace-v1"
	TreatmentDecisionGeneration = "generation"
	TreatmentDecisionAcceptance = "acceptance"
)

type TreatmentDecisionOutcome string

const (
	TreatmentAllowProposal   TreatmentDecisionOutcome = "allow-proposal"
	TreatmentAllowAcceptance TreatmentDecisionOutcome = "allow-acceptance"
	TreatmentBlock           TreatmentDecisionOutcome = "block"
)

type TreatmentDecision struct {
	PolicyRevision string                   `json:"policy_revision"`
	Phase          string                   `json:"phase"`
	Outcome        TreatmentDecisionOutcome `json:"outcome"`
	Reasons        []string                 `json:"reasons"`
}

type TreatmentDecisionFacts struct {
	DiagnosisStatus          string                         `json:"diagnosis_status"`
	CandidateCount           int                            `json:"candidate_count"`
	FreshnessState           string                         `json:"freshness_state,omitempty"`
	SafetyEnvelope           *SafetyEnvelopeV2              `json:"safety_envelope"`
	CandidateAssessmentReady bool                           `json:"candidate_assessment_ready"`
	ProposalAcceptanceState  model.TreatmentAcceptanceState `json:"proposal_acceptance_state,omitempty"`
	SourceBodyStateRevision  int64                          `json:"source_body_state_revision,omitempty"`
	CurrentBodyStateRevision int64                          `json:"current_body_state_revision"`
	MaterialReviewStatus     model.TreatmentStatus          `json:"material_review_status,omitempty"`
	MaterialReviewReasons    []TreatmentReviewReason        `json:"material_review_reasons,omitempty"`
}

// EvaluateTreatmentDecision is the Go-owned deny-overrides authority for
// proposal generation and acceptance. Model confidence and Agent prose are
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
	if phase != TreatmentDecisionGeneration && phase != TreatmentDecisionAcceptance {
		decision.Reasons = []string{"unsupported_decision_phase"}
		return decision
	}

	// Preserve the established command-boundary ordering. Acceptance first
	// verifies that the artifact is still a proposal; generation has no such fact.
	if phase == TreatmentDecisionAcceptance && facts.ProposalAcceptanceState != model.TreatmentAcceptanceProposed {
		decision.Reasons = []string{"proposal_not_proposed"}
		return decision
	}

	if phase == TreatmentDecisionGeneration {
		if !treatmentDiagnosisFactsReady(facts) {
			decision.Reasons = []string{"diagnosis_not_ready"}
			return decision
		}
		if treatmentFreshnessBlocks(facts.FreshnessState) {
			decision.Reasons = []string{"diagnosis_not_fresh"}
			return decision
		}
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

	if phase == TreatmentDecisionAcceptance && facts.CurrentBodyStateRevision < facts.SourceBodyStateRevision {
		decision.Reasons = []string{"body_state_revision_regressed"}
		return decision
	}
	if phase == TreatmentDecisionAcceptance {
		if !treatmentDiagnosisFactsReady(facts) {
			decision.Reasons = []string{"diagnosis_not_ready"}
			return decision
		}
		if treatmentFreshnessBlocks(facts.FreshnessState) {
			decision.Reasons = []string{"diagnosis_not_fresh"}
			return decision
		}
	}

	if !facts.CandidateAssessmentReady {
		decision.Reasons = []string{"candidate_assessment_required"}
		return decision
	}

	if phase == TreatmentDecisionAcceptance && facts.CurrentBodyStateRevision > facts.SourceBodyStateRevision {
		if facts.MaterialReviewStatus != model.TreatmentStatusActive || len(facts.MaterialReviewReasons) > 0 {
			decision.Reasons = []string{"material_related_body_state_change"}
			return decision
		}
	}

	if phase == TreatmentDecisionGeneration {
		decision.Outcome = TreatmentAllowProposal
	} else {
		decision.Outcome = TreatmentAllowAcceptance
	}
	return decision
}

func treatmentDiagnosisFactsReady(facts TreatmentDecisionFacts) bool {
	return (facts.DiagnosisStatus == "completed" || facts.DiagnosisStatus == "partial") && facts.CandidateCount > 0
}

func treatmentFreshnessBlocks(state string) bool {
	state = strings.TrimSpace(state)
	return state != "" && state != model.DiagnosisFreshnessFresh
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
