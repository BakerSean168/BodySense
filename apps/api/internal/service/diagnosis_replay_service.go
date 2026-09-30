package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

var (
	ErrDiagnosisReplayUnavailable = errors.New("diagnosis replay input is unavailable")
	ErrDiagnosisReplayNotFound    = errors.New("diagnosis analysis not found for replay")
)

const DiagnosisRegressionExportSchema = "diagnosis_qualification_v1"

type DiagnosisReplayInput struct {
	BodyStateRevision int64             `json:"body_state_revision"`
	BodyState         json.RawMessage   `json:"body_state"`
	RelevantHistory   json.RawMessage   `json:"relevant_history"`
	Profile           json.RawMessage   `json:"profile"`
	SafetyEnvelope    *SafetyEnvelopeV2 `json:"safety_envelope,omitempty"`
}

type DiagnosisReplayCheck struct {
	Name      string `json:"name"`
	Match     bool   `json:"match"`
	Baseline  string `json:"baseline,omitempty"`
	Candidate string `json:"candidate,omitempty"`
}

type DiagnosisReplayLayer struct {
	Match  bool                   `json:"match"`
	Checks []DiagnosisReplayCheck `json:"checks"`
}

const (
	LegacyProseSafetySourcePostAgent = "post_agent_red_flag_governance"
	LegacyProseSafetySourcePreAgent  = "pre_agent_red_flag_gate"
)

type DiagnosisReplayAuthorityEndpoint struct {
	DecisionPolicyRevision      string   `json:"decision_policy_revision"`
	GovernanceVerdict           string   `json:"governance_verdict"`
	LegacyProseGovernanceOnly   bool     `json:"legacy_prose_governance_only"`
	LegacyProseSafetySource     string   `json:"legacy_prose_safety_source,omitempty"`
	LegacyProseSafetyCategories []string `json:"legacy_prose_safety_categories,omitempty"`
	SafetyFindingCount          int      `json:"safety_finding_count"`
	ForbiddenSideEffectsPresent bool     `json:"forbidden_side_effects_present"`
}

type DiagnosisReplayAuthorityEvidence struct {
	SafetyEnvelopePresent   bool                             `json:"safety_envelope_present"`
	CoverageComplete        bool                             `json:"coverage_complete"`
	ActiveBlockerCount      int                              `json:"active_blocker_count"`
	RequiresReview          bool                             `json:"requires_review"`
	ConfirmedAbsentConcepts []string                         `json:"confirmed_absent_concepts,omitempty"`
	Baseline                DiagnosisReplayAuthorityEndpoint `json:"baseline"`
	Replay                  DiagnosisReplayAuthorityEndpoint `json:"replay"`
}

type DiagnosisReplayAuthorityComparison struct {
	PolicyRevision  string                           `json:"policy_revision,omitempty"`
	PromotionRecord string                           `json:"promotion_record,omitempty"`
	Classification  string                           `json:"classification,omitempty"`
	GateEquivalent  bool                             `json:"gate_equivalent"`
	ReasonCodes     []string                         `json:"reason_codes,omitempty"`
	Evidence        DiagnosisReplayAuthorityEvidence `json:"evidence"`
}

type DiagnosisReplayComparison struct {
	Hard         DiagnosisReplayLayer               `json:"hard"`
	Semantic     DiagnosisReplayLayer               `json:"semantic"`
	Presentation DiagnosisReplayLayer               `json:"presentation"`
	Authority    DiagnosisReplayAuthorityComparison `json:"authority,omitempty"`
}

type DiagnosisReplaySnapshot struct {
	Status          string   `json:"status"`
	DecisionOutcome string   `json:"decision_outcome"`
	CandidateCount  int      `json:"candidate_count"`
	ConcernKeys     []string `json:"concern_keys"`
	SupportIDs      []string `json:"support_ids"`
	Summary         string   `json:"summary"`
	CandidateNames  []string `json:"candidate_names"`
}

type DiagnosisReplayReport struct {
	Mode                  string                           `json:"mode"`
	SourceAnalysisID      uuid.UUID                        `json:"source_analysis_id"`
	SourceConfigurationID string                           `json:"source_configuration_id"`
	TargetConfigurationID string                           `json:"target_configuration_id"`
	InputFingerprint      string                           `json:"input_fingerprint"`
	ArtifactIntegrity     DiagnosisReplayLayer             `json:"artifact_integrity"`
	Baseline              DiagnosisReplaySnapshot          `json:"baseline"`
	Replay                DiagnosisReplaySnapshot          `json:"replay"`
	Comparison            DiagnosisReplayComparison        `json:"comparison"`
	AuthorityEvidence     DiagnosisReplayAuthorityEvidence `json:"authority_evidence"`
	Output                json.RawMessage                  `json:"output"`
}

type DiagnosisReplayService struct {
	diagnosis *DiagnosisAnalysisService
	ai        *AIClient
}

func NewDiagnosisReplayService(diagnosis *DiagnosisAnalysisService, ai *AIClient) *DiagnosisReplayService {
	return &DiagnosisReplayService{diagnosis: diagnosis, ai: ai}
}

func EncodeDiagnosisReplayInput(
	bodyStateRevision int64,
	bodyState json.RawMessage,
	relevantHistory json.RawMessage,
	profile json.RawMessage,
	safetyEnvelope *SafetyEnvelopeV2,
) (json.RawMessage, error) {
	if bodyStateRevision <= 0 || len(bodyState) == 0 || !json.Valid(bodyState) {
		return nil, errors.New("valid replay BodyState and revision are required")
	}
	if len(relevantHistory) == 0 {
		relevantHistory = json.RawMessage(`[]`)
	}
	if len(profile) == 0 {
		profile = json.RawMessage(`{}`)
	}
	if safetyEnvelope != nil && (safetyEnvelope.BodyStateRevision != bodyStateRevision || safetyEnvelope.SchemaRevision != SafetyEnvelopeSchemaV2 || safetyEnvelope.PolicyRevision != SafetyEnvelopePolicyV1) {
		return nil, errors.New("replay safety envelope does not match pinned BodyState revision or policy")
	}
	return json.Marshal(DiagnosisReplayInput{
		BodyStateRevision: bodyStateRevision,
		BodyState:         bodyState,
		RelevantHistory:   relevantHistory,
		Profile:           profile,
		SafetyEnvelope:    safetyEnvelope,
	})
}

func (s *DiagnosisReplayService) HistoricalReplay(
	ctx context.Context,
	userID uuid.UUID,
	analysisID uuid.UUID,
) (*DiagnosisReplayReport, error) {
	analysis, input, baseline, err := s.loadReplayCase(ctx, userID, analysisID)
	if err != nil {
		return nil, err
	}
	policyRevision, err := replayPolicyRevision(analysis)
	if err != nil {
		return nil, err
	}
	recomputed := cloneDiagnosisPayload(baseline)
	if policyRevision == DiagnosisDecisionPolicyV1 {
		decision := EvaluateDiagnosisDecision(policyRevision, replaySafetyState(input.BodyState), recomputed)
		recomputed = ApplyDiagnosisDecision(recomputed, decision)
		recomputed = normalizedDiagnosisReplayPayload(recomputed)
	} else if policyRevision == DiagnosisDecisionPolicyV2 {
		decision := EvaluateDiagnosisDecisionV2(input.SafetyEnvelope, input.BodyStateRevision, recomputed)
		recomputed = ApplyDiagnosisDecision(recomputed, decision)
		recomputed = normalizedDiagnosisReplayPayload(recomputed)
	}
	replayRaw, _ := json.Marshal(recomputed)
	return buildDiagnosisReplayReport(
		"historical", analysis, input, analysis.AgentConfigurationID,
		baseline, recomputed, replayRaw,
	), nil
}

func (s *DiagnosisReplayService) CounterfactualReplay(
	ctx context.Context,
	userID uuid.UUID,
	analysisID uuid.UUID,
	targetConfigurationID string,
) (*DiagnosisReplayReport, error) {
	analysis, input, baseline, err := s.loadReplayCase(ctx, userID, analysisID)
	if err != nil {
		return nil, err
	}
	return s.counterfactualCompare(ctx, userID, analysis, input, baseline, targetConfigurationID)
}

func (s *DiagnosisReplayService) counterfactualCompare(
	ctx context.Context,
	userID uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
	input DiagnosisReplayInput,
	baseline map[string]any,
	targetConfigurationID string,
) (*DiagnosisReplayReport, error) {
	targetConfigurationID = strings.TrimSpace(targetConfigurationID)
	policyRevision, err := DiagnosisDecisionPolicyRevisionForConfiguration(targetConfigurationID)
	if err != nil {
		return nil, err
	}
	if policyRevision == DiagnosisDecisionPolicyV1 {
		probe := map[string]any{
			"status":     "completed",
			"candidates": []any{map[string]any{"name": "counterfactual-preflight", "confidence": "n/a"}},
			"governance": map[string]any{"verdict": "accepted"},
		}
		preflight := EvaluateDiagnosisDecision(policyRevision, replaySafetyState(input.BodyState), probe)
		if preflight.Outcome == DiagnosisBlock {
			replayed := ApplyDiagnosisDecision(map[string]any{
				"status": "safety_blocked", "scope": "full_body",
				"summary":    "counterfactual Go pre-agent safety gate blocked ordinary Diagnosis",
				"candidates": []any{}, "cross_concern_patterns": []any{},
				"information_gaps": []any{}, "citations": []any{},
				"safety_summary":       replaySafetyState(input.BodyState),
				"governance":           map[string]any{"kind": "diagnosis", "verdict": "rejected", "reasons": []string{"active_body_state_safety_concern"}, "issues": []any{}},
				"agent_configuration":  map[string]any{"id": targetConfigurationID, "role": "diagnosis", "decision_policy_revision": policyRevision},
				"execution_provenance": map[string]any{"status": "bypassed", "runtime": "go", "reason": "counterfactual_pre_agent_safety_gate"},
			}, preflight)
			result, _ := json.Marshal(replayed)
			return buildDiagnosisReplayReport(
				"counterfactual", analysis, input, targetConfigurationID,
				baseline, replayed, result,
			), nil
		}
	} else if policyRevision == DiagnosisDecisionPolicyV2 {
		if input.SafetyEnvelope == nil {
			return nil, errors.New("counterfactual Diagnosis v8 requires frozen structured safety envelope")
		}
		probe := map[string]any{"status": "completed", "candidates": []any{map[string]any{"name": "counterfactual-preflight"}}, "governance": map[string]any{"verdict": "accepted"}, "safety_findings": []any{}}
		preflight := EvaluateDiagnosisDecisionV2(input.SafetyEnvelope, input.BodyStateRevision, probe)
		if preflight.Outcome != DiagnosisAllowNormal {
			status := "safety_blocked"
			if preflight.Outcome == DiagnosisAbstain {
				status = "insufficient_information"
			}
			replayed := ApplyDiagnosisDecision(map[string]any{
				"status": status, "scope": "full_body", "summary": "structured safety preflight bypassed the agent",
				"candidates": []any{}, "cross_concern_patterns": []any{}, "information_gaps": []any{}, "citations": []any{},
				"governance":           map[string]any{"kind": "diagnosis", "verdict": "rejected", "reasons": preflight.Reasons, "issues": []any{}},
				"agent_configuration":  map[string]any{"id": targetConfigurationID, "role": "diagnosis", "decision_policy_revision": policyRevision},
				"execution_provenance": map[string]any{"status": "bypassed", "runtime": "go", "reason": preflight.Reasons[0]},
			}, preflight)
			result, _ := json.Marshal(replayed)
			replayed = normalizedDiagnosisReplayPayload(replayed)
			return buildDiagnosisReplayReport("counterfactual", analysis, input, targetConfigurationID, baseline, replayed, result), nil
		}
	}
	if s.ai == nil {
		return nil, errors.New("Diagnosis replay AI client is not configured")
	}
	result, err := s.ai.AnalyzeDiagnosis(ctx, DiagnosisRequest{
		UserID:            userID.String(),
		ConfigurationID:   targetConfigurationID,
		BodyStateRevision: input.BodyStateRevision,
		BodyState:         input.BodyState,
		RelevantHistory:   input.RelevantHistory,
		Profile:           input.Profile,
		SafetyEnvelope:    input.SafetyEnvelope,
	})
	if err != nil {
		return nil, fmt.Errorf("counterfactual Diagnosis replay: %w", err)
	}
	var replayed map[string]any
	if err := json.Unmarshal(result, &replayed); err != nil {
		return nil, fmt.Errorf("decode counterfactual Diagnosis replay: %w", err)
	}
	if !replayConfigurationMatches(replayed, targetConfigurationID) {
		return nil, errors.New("counterfactual Diagnosis replay returned the wrong Agent configuration")
	}
	if policyRevision == DiagnosisDecisionPolicyV1 {
		decision := EvaluateDiagnosisDecision(policyRevision, replaySafetyState(input.BodyState), replayed)
		replayed = ApplyDiagnosisDecision(replayed, decision)
		replayed = normalizedDiagnosisReplayPayload(replayed)
		result, _ = json.Marshal(replayed)
	} else if policyRevision == DiagnosisDecisionPolicyV2 {
		decision := EvaluateDiagnosisDecisionV2(input.SafetyEnvelope, input.BodyStateRevision, replayed)
		replayed = ApplyDiagnosisDecision(replayed, decision)
		result, _ = json.Marshal(replayed)
		replayed = normalizedDiagnosisReplayPayload(replayed)
	}
	return buildDiagnosisReplayReport(
		"counterfactual", analysis, input, targetConfigurationID,
		baseline, replayed, result,
	), nil
}

func normalizedDiagnosisReplayPayload(payload map[string]any) map[string]any {
	raw, _ := json.Marshal(payload)
	var normalized map[string]any
	_ = json.Unmarshal(raw, &normalized)
	return normalized
}

func (s *DiagnosisReplayService) ExportRegressionCase(
	ctx context.Context,
	userID uuid.UUID,
	analysisID uuid.UUID,
) (map[string]any, error) {
	analysis, input, baseline, err := s.loadReplayCase(ctx, userID, analysisID)
	if err != nil {
		return nil, err
	}
	snapshot := diagnosisReplaySnapshot(baseline)
	executed := replayAgentExecuted(json.RawMessage(analysis.ExecutionProvenance))
	critical := snapshot.Status == "safety_blocked" || snapshot.DecisionOutcome == string(DiagnosisBlock) || snapshot.DecisionOutcome == string(DiagnosisEscalate)
	maxCandidates := snapshot.CandidateCount
	caseName := "historical-" + strings.ReplaceAll(analysis.ID.String()[:13], "-", "")
	exportBodyState := sanitizeRegressionReplayJSON(input.BodyState, false)
	exportHistory := sanitizeRegressionReplayJSON(input.RelevantHistory, false)
	exportProfile := sanitizeRegressionReplayJSON(input.Profile, true)
	return map[string]any{
		"schema_target":      DiagnosisRegressionExportSchema,
		"source_analysis_id": analysis.ID,
		"case": map[string]any{
			"name": caseName,
			"inputs": map[string]any{
				"user_id":             "historical-regression",
				"body_state_revision": input.BodyStateRevision,
				"body_state":          exportBodyState,
				"relevant_history":    exportHistory,
				"profile":             exportProfile,
			},
			"metadata": map[string]any{
				"scenario_family_id":      "historical-" + analysis.ID.String(),
				"case_category":           "historical-regression",
				"split":                   "regression",
				"slices":                  []string{"historical-replay"},
				"critical":                critical,
				"expected_status":         snapshot.Status,
				"expected_agent_executed": executed,
				"max_tool_calls":          replayEvidenceAttemptCount(json.RawMessage(analysis.EvidenceAcquisitionTrace)),
				"min_candidates":          snapshot.CandidateCount,
				"max_candidates":          maxCandidates,
				"required_concern_keys":   snapshot.ConcernKeys,
				"forbidden_output_fields": []string{"treatment", "training_plan"},
			},
		},
	}, nil
}

func (s *DiagnosisReplayService) loadReplayCase(
	ctx context.Context,
	userID uuid.UUID,
	analysisID uuid.UUID,
) (*model.DiagnosisAnalysisRecord, DiagnosisReplayInput, map[string]any, error) {
	if s == nil || s.diagnosis == nil {
		return nil, DiagnosisReplayInput{}, nil, errors.New("Diagnosis replay service is not configured")
	}
	analysis, err := s.diagnosis.GetByID(ctx, analysisID, userID)
	if err != nil {
		return nil, DiagnosisReplayInput{}, nil, err
	}
	if analysis == nil {
		return nil, DiagnosisReplayInput{}, nil, ErrDiagnosisReplayNotFound
	}
	input, err := decodeDiagnosisReplayInput(json.RawMessage(analysis.ReplayInput))
	if err != nil {
		return nil, DiagnosisReplayInput{}, nil, err
	}
	var baseline map[string]any
	if len(analysis.RawOutput) == 0 || json.Unmarshal(analysis.RawOutput, &baseline) != nil {
		return nil, DiagnosisReplayInput{}, nil, errors.New("stored Diagnosis raw output is not replayable JSON")
	}
	return analysis, input, baseline, nil
}

func decodeDiagnosisReplayInput(raw json.RawMessage) (DiagnosisReplayInput, error) {
	var input DiagnosisReplayInput
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return input, ErrDiagnosisReplayUnavailable
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return input, fmt.Errorf("decode Diagnosis replay input: %w", err)
	}
	if input.BodyStateRevision <= 0 || len(input.BodyState) == 0 || !json.Valid(input.BodyState) {
		return input, ErrDiagnosisReplayUnavailable
	}
	return input, nil
}

func replayPolicyRevision(analysis *model.DiagnosisAnalysisRecord) (string, error) {
	var config struct {
		DecisionPolicyRevision string `json:"decision_policy_revision"`
	}
	_ = json.Unmarshal(analysis.AgentConfiguration, &config)
	if strings.TrimSpace(config.DecisionPolicyRevision) != "" {
		return config.DecisionPolicyRevision, nil
	}
	return DiagnosisDecisionPolicyRevisionForConfiguration(analysis.AgentConfigurationID)
}

func replaySafetyState(bodyState json.RawMessage) json.RawMessage {
	var payload map[string]json.RawMessage
	if json.Unmarshal(bodyState, &payload) != nil {
		return json.RawMessage(`{}`)
	}
	if raw := payload["safety_state"]; len(raw) > 0 {
		return raw
	}
	return json.RawMessage(`{}`)
}

func replayConfigurationMatches(payload map[string]any, expectedID string) bool {
	configuration, ok := payload["agent_configuration"].(map[string]any)
	if !ok {
		return false
	}
	id, idOK := configuration["id"].(string)
	role, roleOK := configuration["role"].(string)
	return idOK && roleOK && id == expectedID && role == "diagnosis"
}

func buildDiagnosisReplayReport(
	mode string,
	analysis *model.DiagnosisAnalysisRecord,
	input DiagnosisReplayInput,
	targetConfigurationID string,
	baseline map[string]any,
	replayed map[string]any,
	replayRaw json.RawMessage,
) *DiagnosisReplayReport {
	integrity := diagnosisReplayArtifactIntegrity(analysis, input, baseline)
	return &DiagnosisReplayReport{
		Mode:                  mode,
		SourceAnalysisID:      analysis.ID,
		SourceConfigurationID: analysis.AgentConfigurationID,
		TargetConfigurationID: targetConfigurationID,
		InputFingerprint:      diagnosisReplayInputFingerprint(input),
		ArtifactIntegrity:     integrity,
		Baseline:              diagnosisReplaySnapshot(baseline),
		Replay:                diagnosisReplaySnapshot(replayed),
		Comparison:            compareDiagnosisReplayOutputs(baseline, replayed),
		AuthorityEvidence:     diagnosisReplayAuthorityEvidence(input, baseline, replayed),
		Output:                replayRaw,
	}
}

func diagnosisReplayAuthorityEvidence(input DiagnosisReplayInput, baseline, replayed map[string]any) DiagnosisReplayAuthorityEvidence {
	evidence := DiagnosisReplayAuthorityEvidence{
		SafetyEnvelopePresent: input.SafetyEnvelope != nil,
		Baseline:              diagnosisReplayAuthorityEndpoint(baseline),
		Replay:                diagnosisReplayAuthorityEndpoint(replayed),
	}
	if input.SafetyEnvelope != nil {
		evidence.CoverageComplete = input.SafetyEnvelope.Coverage.Complete
		evidence.ActiveBlockerCount = len(input.SafetyEnvelope.ActiveBlockers)
		evidence.RequiresReview = input.SafetyEnvelope.RequiresReview
		evidence.ConfirmedAbsentConcepts = replayConfirmedAbsentConcepts(input.SafetyEnvelope)
	}
	return evidence
}

func diagnosisReplayAuthorityEndpoint(payload map[string]any) DiagnosisReplayAuthorityEndpoint {
	categories, source, legacyOnly := replayLegacyProseGovernanceEvidence(payload)
	return DiagnosisReplayAuthorityEndpoint{
		DecisionPolicyRevision:      replayPayloadDecisionPolicyRevision(payload),
		GovernanceVerdict:           replayPayloadGovernanceVerdict(payload),
		LegacyProseGovernanceOnly:   legacyOnly,
		LegacyProseSafetySource:     source,
		LegacyProseSafetyCategories: categories,
		SafetyFindingCount:          replayPayloadSafetyFindingCount(payload),
		ForbiddenSideEffectsPresent: replayHasForbiddenSideEffects(payload),
	}
}

func replayConfirmedAbsentConcepts(envelope *SafetyEnvelopeV2) []string {
	if envelope == nil || !envelope.Coverage.Complete || len(envelope.Coverage.CoveredSourceRefs) == 0 {
		return nil
	}
	covered := map[string]struct{}{}
	for _, sourceRef := range envelope.Coverage.CoveredSourceRefs {
		if value := strings.TrimSpace(sourceRef); value != "" {
			covered[value] = struct{}{}
		}
	}
	if len(covered) == 0 {
		return nil
	}
	required := map[string]struct{}{}
	for _, concept := range envelope.Coverage.RequiredConcepts {
		if value := strings.TrimSpace(concept); value != "" {
			required[value] = struct{}{}
		}
	}
	absentByConcept := map[string]map[string]struct{}{}
	for _, assertion := range envelope.Assertions {
		concept := string(assertion.Concept)
		if _, ok := required[concept]; !ok || assertion.Polarity != SafetyAbsent || assertion.Temporality != SafetyCurrent || assertion.ReviewState != SafetyConfirmed {
			continue
		}
		if _, ok := covered[assertion.SourceRef]; !ok {
			continue
		}
		if absentByConcept[concept] == nil {
			absentByConcept[concept] = map[string]struct{}{}
		}
		absentByConcept[concept][assertion.SourceRef] = struct{}{}
	}
	result := []string{}
	for concept := range required {
		if len(absentByConcept[concept]) == len(covered) {
			result = append(result, concept)
		}
	}
	sort.Strings(result)
	return result
}

func replayPayloadDecisionPolicyRevision(payload map[string]any) string {
	if configuration, ok := payload["agent_configuration"].(map[string]any); ok {
		if revision, ok := configuration["decision_policy_revision"].(string); ok && strings.TrimSpace(revision) != "" {
			return strings.TrimSpace(revision)
		}
	}
	if authority, ok := payload["decision_authority"].(map[string]any); ok {
		if revision, ok := authority["policy_revision"].(string); ok {
			return strings.TrimSpace(revision)
		}
	}
	return ""
}

func replayPayloadGovernanceVerdict(payload map[string]any) string {
	governance, _ := payload["governance"].(map[string]any)
	verdict, _ := governance["verdict"].(string)
	return strings.TrimSpace(verdict)
}

func replayPayloadSafetyFindingCount(payload map[string]any) int {
	findings, _ := payload["safety_findings"].([]any)
	return len(findings)
}

func replayLegacyProseGovernanceOnly(payload map[string]any) bool {
	_, _, ok := replayLegacyProseGovernanceEvidence(payload)
	return ok
}

func replayLegacyProseGovernanceCategories(payload map[string]any) ([]string, bool) {
	categories, _, ok := replayLegacyProseGovernanceEvidence(payload)
	return categories, ok
}

func replayLegacyProseGovernanceEvidence(payload map[string]any) ([]string, string, bool) {
	status, _ := payload["status"].(string)
	if status != "safety_blocked" || replayPayloadDecisionPolicyRevision(payload) != DiagnosisDecisionPolicyV1 {
		return nil, "", false
	}
	authority, _ := payload["decision_authority"].(map[string]any)
	outcome, _ := authority["outcome"].(string)
	if outcome != string(DiagnosisBlock) || !replayStringListExactly(authority["reasons"], []string{"agent_output_failed_safety_governance"}) {
		return nil, "", false
	}
	if categories, ok := replayLegacyPostAgentGovernanceCategories(payload); ok {
		return categories, LegacyProseSafetySourcePostAgent, true
	}
	if categories, ok := replayLegacyPreAgentSafetyCategories(payload); ok {
		return categories, LegacyProseSafetySourcePreAgent, true
	}
	return nil, "", false
}

func replayLegacyPostAgentGovernanceCategories(payload map[string]any) ([]string, bool) {
	governance, _ := payload["governance"].(map[string]any)
	verdict, _ := governance["verdict"].(string)
	issues, _ := governance["issues"].([]any)
	if verdict != "rejected" || len(issues) == 0 {
		return nil, false
	}
	categories := map[string]struct{}{}
	for _, raw := range issues {
		issue, ok := raw.(map[string]any)
		if !ok {
			return nil, false
		}
		policy, _ := issue["policy"].(string)
		if policy != "red_flag_safety" {
			return nil, false
		}
		details, ok := issue["details"].(map[string]any)
		if !ok {
			return nil, false
		}
		category, _ := details["category"].(string)
		category = strings.TrimSpace(category)
		if category == "" {
			return nil, false
		}
		categories[category] = struct{}{}
	}
	return sortedLegacySafetyCategories(categories)
}

func replayLegacyPreAgentSafetyCategories(payload map[string]any) ([]string, bool) {
	provenance, _ := payload["execution_provenance"].(map[string]any)
	status, _ := provenance["status"].(string)
	reason, _ := provenance["reason"].(string)
	if status != "bypassed" || reason != "python_pre_agent_safety_gate" {
		return nil, false
	}
	governance, _ := payload["governance"].(map[string]any)
	verdict, _ := governance["verdict"].(string)
	issues, _ := governance["issues"].([]any)
	if verdict != "accepted" || len(issues) != 0 {
		return nil, false
	}
	safetySummary, _ := payload["safety_summary"].(map[string]any)
	redFlags, _ := safetySummary["red_flags"].(map[string]any)
	hasRedFlags, _ := redFlags["has_red_flags"].(bool)
	flags, _ := redFlags["flags"].([]any)
	if !hasRedFlags || len(flags) == 0 {
		return nil, false
	}
	categories := map[string]struct{}{}
	for _, raw := range flags {
		flag, ok := raw.(map[string]any)
		if !ok {
			return nil, false
		}
		category, _ := flag["category"].(string)
		category = strings.TrimSpace(category)
		if category == "" {
			return nil, false
		}
		categories[category] = struct{}{}
	}
	return sortedLegacySafetyCategories(categories)
}

func sortedLegacySafetyCategories(categories map[string]struct{}) ([]string, bool) {
	result := make([]string, 0, len(categories))
	for category := range categories {
		result = append(result, category)
	}
	sort.Strings(result)
	return result, len(result) > 0
}

func replayStringListExactly(raw any, want []string) bool {
	items := []string{}
	switch values := raw.(type) {
	case []any:
		for _, item := range values {
			value, ok := item.(string)
			if !ok {
				return false
			}
			items = append(items, value)
		}
	case []string:
		items = append(items, values...)
	default:
		return false
	}
	if len(items) != len(want) {
		return false
	}
	for i := range want {
		if items[i] != want[i] {
			return false
		}
	}
	return true
}

func replayStringListContains(raw any, want string) bool {
	items, ok := raw.([]any)
	if !ok {
		if strings, ok := raw.([]string); ok {
			for _, item := range strings {
				if item == want {
					return true
				}
			}
		}
		return false
	}
	for _, item := range items {
		if value, ok := item.(string); ok && value == want {
			return true
		}
	}
	return false
}

func diagnosisReplayArtifactIntegrity(
	analysis *model.DiagnosisAnalysisRecord,
	input DiagnosisReplayInput,
	baseline map[string]any,
) DiagnosisReplayLayer {
	configID := replayPayloadConfigurationID(baseline)
	status, _ := baseline["status"].(string)
	checks := []DiagnosisReplayCheck{
		replayCheck("body_state_revision", fmt.Sprint(analysis.BodyStateRevision), fmt.Sprint(input.BodyStateRevision)),
		replayCheck("agent_configuration_id", analysis.AgentConfigurationID, configID),
		replayCheck("durable_status", analysis.Status, status),
	}
	return replayLayer(checks)
}

func compareDiagnosisReplayOutputs(baseline, candidate map[string]any) DiagnosisReplayComparison {
	base := diagnosisReplaySnapshot(baseline)
	cand := diagnosisReplaySnapshot(candidate)
	hard := replayLayer([]DiagnosisReplayCheck{
		replayCheck("status", base.Status, cand.Status),
		replayCheck("decision_outcome", base.DecisionOutcome, cand.DecisionOutcome),
		replayCheck("forbidden_side_effects", fmt.Sprint(replayHasForbiddenSideEffects(baseline)), fmt.Sprint(replayHasForbiddenSideEffects(candidate))),
	})
	semantic := replayLayer([]DiagnosisReplayCheck{
		replayCheck("candidate_count", fmt.Sprint(base.CandidateCount), fmt.Sprint(cand.CandidateCount)),
		replayCheck("concern_keys", strings.Join(base.ConcernKeys, "|"), strings.Join(cand.ConcernKeys, "|")),
		replayCheck("support_ids", strings.Join(base.SupportIDs, "|"), strings.Join(cand.SupportIDs, "|")),
	})
	presentation := replayLayer([]DiagnosisReplayCheck{
		replayCheck("summary", base.Summary, cand.Summary),
		replayCheck("candidate_names", strings.Join(base.CandidateNames, "|"), strings.Join(cand.CandidateNames, "|")),
	})
	return DiagnosisReplayComparison{Hard: hard, Semantic: semantic, Presentation: presentation}
}

func diagnosisReplaySnapshot(payload map[string]any) DiagnosisReplaySnapshot {
	status, _ := payload["status"].(string)
	summary, _ := payload["summary"].(string)
	concerns := map[string]struct{}{}
	support := map[string]struct{}{}
	names := []string{}
	count := 0
	if candidates, ok := payload["candidates"].([]any); ok {
		count = len(candidates)
		for _, raw := range candidates {
			candidate, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if value := strings.TrimSpace(fmt.Sprint(candidate["concern_key"])); value != "" && value != "<nil>" {
				concerns[value] = struct{}{}
			}
			if value, ok := candidate["name"].(string); ok && strings.TrimSpace(value) != "" {
				names = append(names, strings.TrimSpace(value))
			}
			for _, field := range []string{"basis_fact_ids", "basis_observation_ids", "supporting_evidence_ids", "counterevidence_ids"} {
				if values, ok := candidate[field].([]any); ok {
					for _, item := range values {
						value := strings.TrimSpace(fmt.Sprint(item))
						if value != "" && value != "<nil>" {
							support[value] = struct{}{}
						}
					}
				}
			}
		}
	}
	return DiagnosisReplaySnapshot{
		Status:          status,
		DecisionOutcome: replayDecisionOutcome(payload),
		CandidateCount:  count,
		ConcernKeys:     replaySortedKeys(concerns),
		SupportIDs:      replaySortedKeys(support),
		Summary:         summary,
		CandidateNames:  names,
	}
}

func replayDecisionOutcome(payload map[string]any) string {
	if authority, ok := payload["decision_authority"].(map[string]any); ok {
		if outcome, ok := authority["outcome"].(string); ok && strings.TrimSpace(outcome) != "" {
			return outcome
		}
	}
	status, _ := payload["status"].(string)
	verdict := ""
	if governance, ok := payload["governance"].(map[string]any); ok {
		verdict, _ = governance["verdict"].(string)
	}
	switch {
	case verdict == "rejected" || status == "safety_blocked":
		return string(DiagnosisBlock)
	case status == "insufficient_information":
		return string(DiagnosisAbstain)
	case status == "partial" || verdict == "degraded":
		return string(DiagnosisAllowDegraded)
	case status == "completed" && verdict == "accepted":
		return string(DiagnosisAllowNormal)
	default:
		return "unknown"
	}
}

func replayPayloadConfigurationID(payload map[string]any) string {
	configuration, _ := payload["agent_configuration"].(map[string]any)
	id, _ := configuration["id"].(string)
	return id
}

func replayHasForbiddenSideEffects(payload map[string]any) bool {
	_, treatment := payload["treatment"]
	_, training := payload["training_plan"]
	return treatment || training
}

func replayCheck(name, baseline, candidate string) DiagnosisReplayCheck {
	return DiagnosisReplayCheck{Name: name, Match: baseline == candidate, Baseline: baseline, Candidate: candidate}
}

func replayLayer(checks []DiagnosisReplayCheck) DiagnosisReplayLayer {
	match := true
	for _, check := range checks {
		match = match && check.Match
	}
	return DiagnosisReplayLayer{Match: match, Checks: checks}
}

func replaySortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func diagnosisReplayInputFingerprint(input DiagnosisReplayInput) string {
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func replayAgentExecuted(raw json.RawMessage) bool {
	var execution struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(raw, &execution)
	return execution.Status == "executed"
}

func replayEvidenceAttemptCount(raw json.RawMessage) int {
	var trace struct {
		Attempts []json.RawMessage `json:"attempts"`
	}
	_ = json.Unmarshal(raw, &trace)
	return len(trace.Attempts)
}

func sanitizeRegressionReplayJSON(raw json.RawMessage, profileRoot bool) json.RawMessage {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return raw
	}
	var walk func(any, bool) any
	walk = func(current any, isProfileRoot bool) any {
		switch typed := current.(type) {
		case map[string]any:
			result := make(map[string]any, len(typed))
			for key, item := range typed {
				normalized := strings.ToLower(strings.TrimSpace(key))
				if normalized == "user_id" {
					result[key] = "historical-regression"
					continue
				}
				if isProfileRoot {
					switch normalized {
					case "id", "email", "phone", "phone_number", "full_name", "display_name", "nickname", "avatar_url":
						continue
					}
				}
				result[key] = walk(item, false)
			}
			return result
		case []any:
			result := make([]any, 0, len(typed))
			for _, item := range typed {
				result = append(result, walk(item, false))
			}
			return result
		default:
			return current
		}
	}
	sanitized, _ := json.Marshal(walk(value, profileRoot))
	return json.RawMessage(sanitized)
}
