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
	BodyStateRevision int64           `json:"body_state_revision"`
	BodyState         json.RawMessage `json:"body_state"`
	RelevantHistory   json.RawMessage `json:"relevant_history"`
	Profile           json.RawMessage `json:"profile"`
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

type DiagnosisReplayComparison struct {
	Hard         DiagnosisReplayLayer `json:"hard"`
	Semantic     DiagnosisReplayLayer `json:"semantic"`
	Presentation DiagnosisReplayLayer `json:"presentation"`
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
	Mode                  string                    `json:"mode"`
	SourceAnalysisID      uuid.UUID                 `json:"source_analysis_id"`
	SourceConfigurationID string                    `json:"source_configuration_id"`
	TargetConfigurationID string                    `json:"target_configuration_id"`
	InputFingerprint      string                    `json:"input_fingerprint"`
	ArtifactIntegrity     DiagnosisReplayLayer      `json:"artifact_integrity"`
	Baseline              DiagnosisReplaySnapshot   `json:"baseline"`
	Replay                DiagnosisReplaySnapshot   `json:"replay"`
	Comparison            DiagnosisReplayComparison `json:"comparison"`
	Output                json.RawMessage           `json:"output"`
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
	return json.Marshal(DiagnosisReplayInput{
		BodyStateRevision: bodyStateRevision,
		BodyState:         bodyState,
		RelevantHistory:   relevantHistory,
		Profile:           profile,
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

// CounterfactualFrozen compares a transient served result with another immutable
// configuration without forcing the served result into DiagnosisAnalysis
// persistence. This is required for legacy pre-envelope rejected responses:
// rollout can still detect unsafe Challenger relaxation without changing the
// user's characterized Champion response contract.
func (s *DiagnosisReplayService) CounterfactualFrozen(
	ctx context.Context,
	userID uuid.UUID,
	replayInput json.RawMessage,
	baselineRaw json.RawMessage,
	sourceConfigurationID string,
	targetConfigurationID string,
) (*DiagnosisReplayReport, error) {
	input, err := decodeDiagnosisReplayInput(replayInput)
	if err != nil {
		return nil, err
	}
	var baseline map[string]any
	if len(baselineRaw) == 0 || json.Unmarshal(baselineRaw, &baseline) != nil {
		return nil, errors.New("transient Diagnosis baseline is not valid JSON")
	}
	status, _ := baseline["status"].(string)
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.Nil, BodyStateRevision: input.BodyStateRevision,
		Status: status, AgentConfigurationID: sourceConfigurationID,
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
		result, _ = json.Marshal(replayed)
	}
	return buildDiagnosisReplayReport(
		"counterfactual", analysis, input, targetConfigurationID,
		baseline, replayed, result,
	), nil
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
		Output:                replayRaw,
	}
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
