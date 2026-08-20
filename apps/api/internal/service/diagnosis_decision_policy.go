package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	DiagnosisDecisionPolicyPreEnvelope = "diagnosis-authority-pre-envelope-v0"
	DiagnosisDecisionPolicyV1          = "diagnosis-decision-policy-v1"
)

type DiagnosisDecisionOutcome string

const (
	DiagnosisAllowNormal   DiagnosisDecisionOutcome = "allow-normal"
	DiagnosisAllowDegraded DiagnosisDecisionOutcome = "allow-degraded"
	DiagnosisAbstain       DiagnosisDecisionOutcome = "abstain"
	DiagnosisEscalate      DiagnosisDecisionOutcome = "escalate"
	DiagnosisBlock         DiagnosisDecisionOutcome = "block"
)

// DiagnosisDecision is the Go-owned final authority result. Agent confidence is
// intentionally absent: confidence may describe a candidate but cannot authorize
// delivery across a hard business or safety boundary.
type DiagnosisDecision struct {
	PolicyRevision string                   `json:"policy_revision"`
	Outcome        DiagnosisDecisionOutcome `json:"outcome"`
	Reasons        []string                 `json:"reasons"`
}

type diagnosisDecisionFacts struct {
	activeSafetyReview bool
	newRedFlag         bool
	status             string
	governanceVerdict  string
	candidateCount     int
	criticalGapCount   int
}

// EvaluateDiagnosisDecision is a pure, deterministic deny-overrides policy.
// Unknown policy revisions and malformed policy facts fail closed.
func EvaluateDiagnosisDecision(
	policyRevision string,
	bodyStateSafetyState json.RawMessage,
	agentPayload map[string]any,
) DiagnosisDecision {
	decision := DiagnosisDecision{
		PolicyRevision: policyRevision,
		Outcome:        DiagnosisBlock,
		Reasons:        []string{},
	}
	if policyRevision != DiagnosisDecisionPolicyV1 {
		decision.Reasons = []string{"unsupported_decision_policy_revision"}
		return decision
	}

	facts, err := extractDiagnosisDecisionFacts(bodyStateSafetyState, agentPayload)
	if err != nil {
		decision.Reasons = []string{"malformed_policy_facts", err.Error()}
		return decision
	}

	// Deny-overrides order is deliberate. A lower-strength outcome can never
	// overwrite a stronger reason discovered earlier in the policy.
	if facts.activeSafetyReview {
		decision.Reasons = []string{"active_body_state_safety_concern"}
		return decision
	}
	if facts.governanceVerdict == "rejected" || facts.status == "safety_blocked" {
		decision.Reasons = []string{"agent_output_failed_safety_governance"}
		return decision
	}
	if facts.newRedFlag {
		decision.Outcome = DiagnosisEscalate
		decision.Reasons = []string{"new_runtime_safety_signal"}
		return decision
	}
	if facts.criticalGapCount > 0 {
		decision.Outcome = DiagnosisAbstain
		decision.Reasons = []string{"critical_evidence_gap_unresolved"}
		return decision
	}
	if facts.status == "insufficient_information" {
		decision.Outcome = DiagnosisAbstain
		decision.Reasons = []string{"insufficient_information"}
		return decision
	}
	if facts.status == "partial" || facts.governanceVerdict == "degraded" {
		decision.Outcome = DiagnosisAllowDegraded
		decision.Reasons = []string{"partial_or_degraded_analysis"}
		return decision
	}
	if facts.status == "completed" && facts.governanceVerdict == "accepted" && facts.candidateCount > 0 {
		decision.Outcome = DiagnosisAllowNormal
		return decision
	}

	decision.Reasons = []string{"unrecognized_or_inconsistent_decision_state"}
	return decision
}

func extractDiagnosisDecisionFacts(
	bodyStateSafetyState json.RawMessage,
	payload map[string]any,
) (diagnosisDecisionFacts, error) {
	facts := diagnosisDecisionFacts{}

	activeSafety, err := strictDiagnosisSafetyReview(bodyStateSafetyState)
	if err != nil {
		return facts, err
	}
	facts.activeSafetyReview = activeSafety

	governance, ok := payload["governance"].(map[string]any)
	if !ok {
		return facts, fmt.Errorf("diagnosis governance object is required")
	}
	verdict, ok := governance["verdict"].(string)
	if !ok {
		return facts, fmt.Errorf("diagnosis governance verdict is required")
	}
	verdict = strings.TrimSpace(verdict)
	switch verdict {
	case "accepted", "degraded", "rejected":
		facts.governanceVerdict = verdict
	default:
		return facts, fmt.Errorf("unknown governance verdict %q", verdict)
	}

	// Python's safety gate intentionally strips rejected model content. Once an
	// explicit rejection is present, Go needs no candidate/status payload to deny
	// normal delivery; requiring stripped fields here would turn a valid hard deny
	// into an unrelated malformed-payload reason.
	if verdict == "rejected" {
		return facts, nil
	}

	status, ok := payload["status"].(string)
	if !ok || strings.TrimSpace(status) == "" {
		return facts, fmt.Errorf("diagnosis status is required")
	}
	status = strings.TrimSpace(status)
	switch status {
	case "completed", "partial", "insufficient_information", "safety_blocked":
		facts.status = status
	default:
		return facts, fmt.Errorf("unknown diagnosis status %q", status)
	}

	candidates, ok := payload["candidates"].([]any)
	if !ok {
		return facts, fmt.Errorf("diagnosis candidates must be an array")
	}
	facts.candidateCount = len(candidates)
	if status == "completed" && facts.candidateCount == 0 {
		return facts, fmt.Errorf("completed diagnosis requires at least one candidate")
	}

	if redFlagsRaw, exists := payload["red_flags"]; exists && redFlagsRaw != nil {
		redFlags, ok := redFlagsRaw.(map[string]any)
		if !ok {
			return facts, fmt.Errorf("red_flags must be an object")
		}
		hasRedFlags, ok := redFlags["has_red_flags"].(bool)
		if !ok {
			return facts, fmt.Errorf("red_flags.has_red_flags must be boolean")
		}
		facts.newRedFlag = hasRedFlags
	}

	if acquisitionRaw, exists := payload["evidence_acquisition"]; exists && acquisitionRaw != nil {
		acquisition, ok := acquisitionRaw.(map[string]any)
		if !ok {
			return facts, fmt.Errorf("evidence_acquisition must be an object")
		}
		gapsRaw, ok := acquisition["unresolved_critical_gaps"]
		if !ok {
			return facts, fmt.Errorf("evidence_acquisition.unresolved_critical_gaps is required")
		}
		gaps, ok := gapsRaw.([]any)
		if !ok {
			return facts, fmt.Errorf("evidence_acquisition.unresolved_critical_gaps must be an array")
		}
		facts.criticalGapCount = len(gaps)
	}

	return facts, nil
}

func strictDiagnosisSafetyReview(raw json.RawMessage) (bool, error) {
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

// ApplyDiagnosisDecision makes the Go authority outcome part of the durable raw
// payload and removes model-proposed candidates when ordinary delivery is denied.
func ApplyDiagnosisDecision(payload map[string]any, decision DiagnosisDecision) map[string]any {
	result := cloneDiagnosisPayload(payload)
	result["decision_authority"] = decision

	switch decision.Outcome {
	case DiagnosisAllowNormal, DiagnosisAllowDegraded:
		return result
	case DiagnosisAbstain:
		result["status"] = "insufficient_information"
		result["summary"] = "现有证据不足以支持普通候选下发，请补充关键信息后重新分析。"
		result["candidates"] = []any{}
		result["cross_concern_patterns"] = []any{}
		return result
	case DiagnosisEscalate, DiagnosisBlock:
		result["status"] = "safety_blocked"
		result["summary"] = "当前存在需要优先审核的安全或治理信号，暂不下发普通可能性候选。"
		result["candidates"] = []any{}
		result["cross_concern_patterns"] = []any{}
		result["citations"] = []any{}
		return result
	default:
		// Unknown outcomes are impossible from the typed policy today, but keep
		// serialization fail-closed if a future caller constructs one manually.
		result["status"] = "safety_blocked"
		result["candidates"] = []any{}
		result["cross_concern_patterns"] = []any{}
		result["citations"] = []any{}
		return result
	}
}

func cloneDiagnosisPayload(payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		result[key] = value
	}
	return result
}
