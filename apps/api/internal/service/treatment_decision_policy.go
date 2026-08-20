package service

import (
	"encoding/json"
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
	DiagnosisStatus          string                  `json:"diagnosis_status"`
	CandidateCount           int                     `json:"candidate_count"`
	FreshnessState           string                  `json:"freshness_state,omitempty"`
	SafetyState              json.RawMessage         `json:"safety_state"`
	CandidateAssessmentReady bool                    `json:"candidate_assessment_ready"`
	ProposalAcceptanceState  string                  `json:"proposal_acceptance_state,omitempty"`
	SourceBodyStateRevision  int64                   `json:"source_body_state_revision,omitempty"`
	CurrentBodyStateRevision int64                   `json:"current_body_state_revision"`
	MaterialReviewStatus     string                  `json:"material_review_status,omitempty"`
	MaterialReviewReasons    []TreatmentReviewReason `json:"material_review_reasons,omitempty"`
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

	activeSafety, err := strictTreatmentSafetyReview(facts.SafetyState)
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
func strictTreatmentSafetyReview(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return false, nil
	}
	var state struct {
		HasRedFlags *bool   `json:"has_red_flags"`
		Status      *string `json:"status"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return false, fmt.Errorf("invalid BodyState safety state: %w", err)
	}
	if state.HasRedFlags == nil {
		return false, fmt.Errorf("BodyState safety state is missing has_red_flags")
	}
	status := ""
	if state.Status != nil {
		status = strings.TrimSpace(*state.Status)
	}
	allowed := map[string]bool{
		"": true, "requires_review": true, "active": true,
		"monitoring": true, "resolved": true, "cleared_by_review": true,
	}
	if !allowed[status] {
		return false, fmt.Errorf("unknown BodyState safety status %q", status)
	}
	if *state.HasRedFlags && (status == "resolved" || status == "cleared_by_review" || status == "") {
		return false, fmt.Errorf("inconsistent BodyState safety state")
	}
	if !*state.HasRedFlags && (status == "requires_review" || status == "active" || status == "monitoring") {
		return false, fmt.Errorf("inconsistent BodyState safety state")
	}
	return *state.HasRedFlags && (status == "requires_review" || status == "active"), nil
}
