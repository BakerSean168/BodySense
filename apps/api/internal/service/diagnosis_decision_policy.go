package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const DiagnosisDecisionPolicyV1 = "diagnosis-decision-policy-v1"
const DiagnosisDecisionPolicyV2 = "diagnosis-decision-policy-v2-structured-safety"

// EvaluateDiagnosisDecisionV2 uses only the frozen structured envelope and typed
// output fields. Prose and legacy red_flags are never authority inputs.
func EvaluateDiagnosisDecisionV2(envelope *SafetyEnvelopeV2, bodyStateRevision int64, payload map[string]any) DiagnosisDecision {
	decision := DiagnosisDecision{PolicyRevision: DiagnosisDecisionPolicyV2, Outcome: DiagnosisBlock, Reasons: []string{"invalid_structured_safety_envelope"}}
	if envelope == nil || envelope.SchemaRevision != SafetyEnvelopeSchemaV2 || envelope.PolicyRevision != SafetyEnvelopePolicyV1 || envelope.BodyStateRevision != bodyStateRevision || envelope.Coverage.Revision != SafetyCoverageRevisionV1 || envelope.Coverage.CaptureRevision != SafetyCaptureRevisionV1 || len(envelope.Coverage.RequiredConcepts) != len(structuredSafetyDetails) {
		return decision
	}
	for i, field := range structuredSafetyDetails {
		if envelope.Coverage.RequiredConcepts[i] != field.key {
			return decision
		}
	}
	if envelope.RequiresReview != (len(envelope.ActiveBlockers) > 0) || envelope.Coverage.Complete != (len(envelope.Coverage.CoveredSourceRefs) > 0 && len(envelope.Coverage.IncompleteSourceRefs) == 0) {
		return decision
	}
	if envelope.RequiresReview {
		decision.Reasons = []string{"active_structured_safety_blocker"}
		return decision
	}
	if !envelope.Coverage.Complete {
		decision.Outcome = DiagnosisAbstain
		decision.Reasons = []string{"structured_safety_capture_incomplete"}
		return decision
	}
	governance, ok := payload["governance"].(map[string]any)
	if !ok {
		decision.Reasons = []string{"malformed_policy_facts"}
		return decision
	}
	verdict, ok := governance["verdict"].(string)
	if !ok || (verdict != "accepted" && verdict != "degraded" && verdict != "rejected") {
		decision.Reasons = []string{"malformed_policy_facts"}
		return decision
	}
	if verdict == "rejected" {
		decision.Reasons = []string{"agent_output_failed_safety_governance"}
		return decision
	}
	status, ok := payload["status"].(string)
	if !ok || (status != "completed" && status != "partial" && status != "insufficient_information" && status != "safety_blocked") {
		decision.Reasons = []string{"malformed_policy_facts"}
		return decision
	}
	candidates, ok := payload["candidates"].([]any)
	if !ok {
		decision.Reasons = []string{"malformed_policy_facts"}
		return decision
	}
	if findingsRaw, exists := payload["safety_findings"]; exists {
		findings, valid := findingsRaw.([]any)
		if !valid {
			decision.Reasons = []string{"malformed_structured_safety_findings"}
			return decision
		}
		for _, raw := range findings {
			finding, valid := raw.(map[string]any)
			if !valid || !validDiagnosisSafetyFinding(finding) {
				decision.Reasons = []string{"malformed_structured_safety_findings"}
				return decision
			}
		}
		if len(findings) > 0 {
			decision.Outcome = DiagnosisEscalate
			decision.Reasons = []string{"new_structured_runtime_safety_signal"}
			return decision
		}
	} else {
		decision.Reasons = []string{"malformed_structured_safety_findings"}
		return decision
	}
	if acquisition, exists := payload["evidence_acquisition"]; exists {
		item, valid := acquisition.(map[string]any)
		if !valid {
			decision.Reasons = []string{"malformed_policy_facts"}
			return decision
		}
		gaps, valid := item["unresolved_critical_gaps"].([]any)
		if !valid {
			decision.Reasons = []string{"malformed_policy_facts"}
			return decision
		}
		if len(gaps) > 0 {
			decision.Outcome = DiagnosisAbstain
			decision.Reasons = []string{"critical_evidence_gap_unresolved"}
			return decision
		}
	}
	if status == "insufficient_information" {
		decision.Outcome = DiagnosisAbstain
		decision.Reasons = []string{"insufficient_information"}
		return decision
	}
	if status == "partial" || verdict == "degraded" {
		decision.Outcome = DiagnosisAllowDegraded
		decision.Reasons = []string{"partial_or_degraded_analysis"}
		return decision
	}
	if status == "completed" && verdict == "accepted" && len(candidates) > 0 {
		decision.Outcome = DiagnosisAllowNormal
		decision.Reasons = []string{}
		return decision
	}
	decision.Reasons = []string{"unrecognized_or_inconsistent_decision_state"}
	return decision
}

func validDiagnosisSafetyFinding(finding map[string]any) bool {
	concept, ok := finding["concept"].(string)
	if !ok || !validSafetyConcept(SafetyConceptV1(concept)) || finding["temporality"] != "current" {
		return false
	}
	if finding["polarity"] != "present" && finding["polarity"] != "uncertain" {
		return false
	}
	refs, ok := finding["source_refs"].([]any)
	if !ok || len(refs) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, raw := range refs {
		ref, ok := raw.(string)
		if !ok || (!strings.HasPrefix(ref, "body-state:fact:") && !strings.HasPrefix(ref, "body-state:observation:")) || seen[ref] {
			return false
		}
		if _, err := uuid.Parse(ref[strings.LastIndex(ref, ":")+1:]); err != nil {
			return false
		}
		seen[ref] = true
	}
	return true
}

func validSafetyConcept(concept SafetyConceptV1) bool {
	switch concept {
	case SafetyTrauma, SafetyRadiatingPain, SafetyNumbness, SafetyWeakness, SafetyDizziness, SafetyGaitInstability, SafetySeverePain, SafetyWorsening, SafetyInfection, SafetySystemic, SafetyUnknownRedFlag:
		return true
	default:
		return false
	}
}

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
