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
	ErrTreatmentReplayUnavailable = errors.New("treatment replay input is unavailable")
	ErrTreatmentReplayNotFound    = errors.New("treatment revision not found for replay")
)

const TreatmentRegressionExportSchema = "treatment_qualification_v1"

type TreatmentReplayInput struct {
	BodyStateRevision    int64                  `json:"body_state_revision"`
	BodyState            json.RawMessage        `json:"body_state"`
	DiagnosisAnalysis    json.RawMessage        `json:"diagnosis_analysis"`
	CandidateAssessments json.RawMessage        `json:"candidate_assessments"`
	Profile              json.RawMessage        `json:"profile"`
	UserConstraints      json.RawMessage        `json:"user_constraints"`
	Evidence             json.RawMessage        `json:"evidence"`
	GenerationFacts      TreatmentDecisionFacts `json:"generation_facts"`
}

type TreatmentReplayCheck struct {
	Name      string `json:"name"`
	Match     bool   `json:"match"`
	Baseline  string `json:"baseline,omitempty"`
	Candidate string `json:"candidate,omitempty"`
}

type TreatmentReplayLayer struct {
	Match  bool                   `json:"match"`
	Checks []TreatmentReplayCheck `json:"checks"`
}

type TreatmentReplayComparison struct {
	Hard         TreatmentReplayLayer `json:"hard"`
	Semantic     TreatmentReplayLayer `json:"semantic"`
	Presentation TreatmentReplayLayer `json:"presentation"`
}

type TreatmentReplaySnapshot struct {
	Status             string   `json:"status"`
	GovernanceVerdict  string   `json:"governance_verdict"`
	InterventionCount  int      `json:"intervention_count"`
	InterventionKinds  []string `json:"intervention_kinds"`
	EvidenceIDs        []string `json:"evidence_ids"`
	WarningSignCount   int      `json:"warning_sign_count"`
	ReviewTriggerCount int      `json:"review_trigger_count"`
	DurationWeeks      int      `json:"duration_weeks"`
	Summary            string   `json:"summary"`
	Goal               string   `json:"goal"`
	InterventionTitles []string `json:"intervention_titles"`
}

type TreatmentReplayReport struct {
	Mode                  string                    `json:"mode"`
	SourceRevisionID      uuid.UUID                 `json:"source_revision_id"`
	SourceConfigurationID string                    `json:"source_configuration_id"`
	TargetConfigurationID string                    `json:"target_configuration_id"`
	InputFingerprint      string                    `json:"input_fingerprint"`
	ArtifactIntegrity     TreatmentReplayLayer      `json:"artifact_integrity"`
	GenerationDecision    TreatmentDecision         `json:"generation_decision"`
	Baseline              TreatmentReplaySnapshot   `json:"baseline"`
	Replay                TreatmentReplaySnapshot   `json:"replay"`
	Comparison            TreatmentReplayComparison `json:"comparison"`
	Output                json.RawMessage           `json:"output"`
}

type treatmentReplayAI interface {
	RecommendTreatment(ctx context.Context, req TreatmentRecommendationRequest) (json.RawMessage, error)
}

type TreatmentReplayService struct {
	treatments *TreatmentService
	ai         treatmentReplayAI
}

func NewTreatmentReplayService(treatments *TreatmentService, ai treatmentReplayAI) *TreatmentReplayService {
	return &TreatmentReplayService{treatments: treatments, ai: ai}
}

func EncodeTreatmentReplayInput(
	bodyStateRevision int64,
	bodyState json.RawMessage,
	diagnosisAnalysis json.RawMessage,
	candidateAssessments json.RawMessage,
	profile json.RawMessage,
	userConstraints json.RawMessage,
	evidence json.RawMessage,
	generationFacts TreatmentDecisionFacts,
) (json.RawMessage, error) {
	if bodyStateRevision <= 0 || len(bodyState) == 0 || !json.Valid(bodyState) {
		return nil, errors.New("valid Treatment replay BodyState and revision are required")
	}
	for name, raw := range map[string]json.RawMessage{
		"diagnosis_analysis":    diagnosisAnalysis,
		"candidate_assessments": candidateAssessments,
		"profile":               profile,
		"user_constraints":      userConstraints,
		"evidence":              evidence,
	} {
		if len(raw) == 0 || !json.Valid(raw) {
			return nil, fmt.Errorf("valid Treatment replay %s is required", name)
		}
	}
	return json.Marshal(TreatmentReplayInput{
		BodyStateRevision:    bodyStateRevision,
		BodyState:            bodyState,
		DiagnosisAnalysis:    diagnosisAnalysis,
		CandidateAssessments: candidateAssessments,
		Profile:              profile,
		UserConstraints:      userConstraints,
		Evidence:             evidence,
		GenerationFacts:      generationFacts,
	})
}

func (s *TreatmentReplayService) HistoricalReplay(
	ctx context.Context,
	userID uuid.UUID,
	revisionID uuid.UUID,
) (*TreatmentReplayReport, error) {
	revision, input, baseline, err := s.loadReplayCase(ctx, userID, revisionID)
	if err != nil {
		return nil, err
	}
	decision := EvaluateTreatmentDecision(
		treatmentDecisionPolicyRevision(revision),
		TreatmentDecisionGeneration,
		input.GenerationFacts,
	)
	baselineRaw, _ := json.Marshal(baseline)
	return buildTreatmentReplayReport(
		"historical", revision, input, revision.AgentConfigurationID,
		decision, baseline, baseline, baselineRaw,
	), nil
}

func (s *TreatmentReplayService) CounterfactualReplay(
	ctx context.Context,
	userID uuid.UUID,
	revisionID uuid.UUID,
	targetConfigurationID string,
) (*TreatmentReplayReport, error) {
	revision, input, baseline, err := s.loadReplayCase(ctx, userID, revisionID)
	if err != nil {
		return nil, err
	}
	targetConfigurationID = strings.TrimSpace(targetConfigurationID)
	registration, ok := knownTreatmentConfigurations[targetConfigurationID]
	if !ok {
		return nil, fmt.Errorf("unknown Treatment Agent configuration id %q", targetConfigurationID)
	}
	decision := EvaluateTreatmentDecision(
		registration.DecisionPolicyRevision,
		TreatmentDecisionGeneration,
		input.GenerationFacts,
	)
	if decision.Outcome != TreatmentAllowProposal {
		blocked := map[string]any{
			"status":     "blocked",
			"governance": map[string]any{"kind": "treatment", "verdict": "rejected", "reasons": decision.Reasons},
			"agent_configuration": map[string]any{
				"id": targetConfigurationID, "role": "treatment",
				"decision_policy_revision": registration.DecisionPolicyRevision,
			},
			"execution_provenance": map[string]any{"status": "bypassed", "runtime": "go", "reason": "counterfactual_pre_agent_decision_gate"},
		}
		raw, _ := json.Marshal(blocked)
		return buildTreatmentReplayReport(
			"counterfactual", revision, input, targetConfigurationID,
			decision, baseline, blocked, raw,
		), nil
	}
	if s.ai == nil {
		return nil, errors.New("Treatment replay AI client is not configured")
	}
	result, err := s.ai.RecommendTreatment(ctx, TreatmentRecommendationRequest{
		UserID:               userID.String(),
		ConfigurationID:      targetConfigurationID,
		BodyStateRevision:    input.BodyStateRevision,
		BodyState:            input.BodyState,
		DiagnosisAnalysis:    input.DiagnosisAnalysis,
		CandidateAssessments: input.CandidateAssessments,
		Profile:              input.Profile,
		UserConstraints:      input.UserConstraints,
		Evidence:             input.Evidence,
	})
	if err != nil {
		return nil, fmt.Errorf("counterfactual Treatment replay: %w", err)
	}
	payload, err := normalizeTreatmentAgentPayload(result)
	if err != nil {
		return nil, err
	}
	if err := validateTreatmentAgentIdentity(payload, targetConfigurationID); err != nil {
		return nil, err
	}
	var replayed map[string]any
	if err := json.Unmarshal(result, &replayed); err != nil {
		return nil, fmt.Errorf("decode counterfactual Treatment replay: %w", err)
	}
	return buildTreatmentReplayReport(
		"counterfactual", revision, input, targetConfigurationID,
		decision, baseline, replayed, result,
	), nil
}

func (s *TreatmentReplayService) ExportRegressionCase(
	ctx context.Context,
	userID uuid.UUID,
	revisionID uuid.UUID,
) (map[string]any, error) {
	revision, input, _, err := s.loadReplayCase(ctx, userID, revisionID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"schema_target":      TreatmentRegressionExportSchema,
		"source_revision_id": revision.ID,
		"case": map[string]any{
			"name": "historical-" + strings.ReplaceAll(revision.ID.String()[:13], "-", ""),
			"inputs": map[string]any{
				"user_id":               "historical-regression",
				"body_state_revision":   input.BodyStateRevision,
				"body_state":            sanitizeTreatmentReplayJSON(input.BodyState, false),
				"diagnosis_analysis":    sanitizeTreatmentReplayJSON(input.DiagnosisAnalysis, false),
				"candidate_assessments": sanitizeTreatmentReplayJSON(input.CandidateAssessments, false),
				"profile":               sanitizeTreatmentReplayJSON(input.Profile, true),
				"user_constraints":      sanitizeTreatmentReplayJSON(input.UserConstraints, false),
				"evidence":              sanitizeTreatmentReplayJSON(input.Evidence, false),
			},
			"metadata": map[string]any{
				"split":                      "regression",
				"slices":                     []string{"historical-replay"},
				"required_assessment_states": treatmentReplayAssessmentStates(input.CandidateAssessments),
				"required_context_tokens":    treatmentReplayEvidenceIDs(input.Evidence),
			},
		},
	}, nil
}

func (s *TreatmentReplayService) loadReplayCase(
	ctx context.Context,
	userID uuid.UUID,
	revisionID uuid.UUID,
) (*model.TreatmentRevision, TreatmentReplayInput, map[string]any, error) {
	if s == nil || s.treatments == nil {
		return nil, TreatmentReplayInput{}, nil, errors.New("Treatment replay service is not configured")
	}
	revision, err := s.treatments.GetRevision(ctx, userID, revisionID)
	if err != nil {
		return nil, TreatmentReplayInput{}, nil, err
	}
	if revision == nil {
		return nil, TreatmentReplayInput{}, nil, ErrTreatmentReplayNotFound
	}
	input, err := decodeTreatmentReplayInput(json.RawMessage(revision.ReplayInput))
	if err != nil {
		return nil, TreatmentReplayInput{}, nil, err
	}
	baseline, err := treatmentReplayBaseline(revision)
	if err != nil {
		return nil, TreatmentReplayInput{}, nil, err
	}
	return revision, input, baseline, nil
}

func decodeTreatmentReplayInput(raw json.RawMessage) (TreatmentReplayInput, error) {
	var input TreatmentReplayInput
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return input, ErrTreatmentReplayUnavailable
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return input, fmt.Errorf("decode Treatment replay input: %w", err)
	}
	if input.BodyStateRevision <= 0 || len(input.BodyState) == 0 || !json.Valid(input.BodyState) {
		return input, ErrTreatmentReplayUnavailable
	}
	return input, nil
}

func treatmentReplayBaseline(revision *model.TreatmentRevision) (map[string]any, error) {
	var plan map[string]any
	if len(revision.Plan) == 0 || json.Unmarshal(revision.Plan, &plan) != nil {
		return nil, errors.New("stored Treatment plan is not replayable JSON")
	}
	baseline := cloneTreatmentPayload(plan)
	baseline["status"] = "proposed"
	baseline["evidence_ids"] = decodeReplayStringArray(json.RawMessage(revision.EvidenceIDs))
	baseline["governance"] = decodeReplayObject(json.RawMessage(revision.Governance))
	baseline["agent_configuration"] = decodeReplayObject(json.RawMessage(revision.AgentConfiguration))
	baseline["execution_provenance"] = decodeReplayObject(json.RawMessage(revision.ExecutionProvenance))
	if len(revision.EvidenceAcquisitionTrace) > 0 && string(revision.EvidenceAcquisitionTrace) != "{}" {
		baseline["evidence_acquisition"] = decodeReplayObject(json.RawMessage(revision.EvidenceAcquisitionTrace))
	}
	return baseline, nil
}

func treatmentDecisionPolicyRevision(revision *model.TreatmentRevision) string {
	var configuration struct {
		DecisionPolicyRevision string `json:"decision_policy_revision"`
	}
	_ = json.Unmarshal(revision.AgentConfiguration, &configuration)
	if strings.TrimSpace(configuration.DecisionPolicyRevision) != "" {
		return configuration.DecisionPolicyRevision
	}
	if registration, ok := knownTreatmentConfigurations[revision.AgentConfigurationID]; ok {
		return registration.DecisionPolicyRevision
	}
	return ""
}

func buildTreatmentReplayReport(
	mode string,
	revision *model.TreatmentRevision,
	input TreatmentReplayInput,
	targetConfigurationID string,
	decision TreatmentDecision,
	baseline map[string]any,
	replayed map[string]any,
	replayRaw json.RawMessage,
) *TreatmentReplayReport {
	return &TreatmentReplayReport{
		Mode:                  mode,
		SourceRevisionID:      revision.ID,
		SourceConfigurationID: revision.AgentConfigurationID,
		TargetConfigurationID: targetConfigurationID,
		InputFingerprint:      treatmentReplayInputFingerprint(input),
		ArtifactIntegrity:     treatmentReplayArtifactIntegrity(revision, input, decision),
		GenerationDecision:    decision,
		Baseline:              treatmentReplaySnapshot(baseline),
		Replay:                treatmentReplaySnapshot(replayed),
		Comparison:            compareTreatmentReplayOutputs(baseline, replayed),
		Output:                replayRaw,
	}
}

func treatmentReplayArtifactIntegrity(
	revision *model.TreatmentRevision,
	input TreatmentReplayInput,
	decision TreatmentDecision,
) TreatmentReplayLayer {
	checks := []TreatmentReplayCheck{
		{Name: "body_state_revision", Match: revision.SourceBodyStateRevision == input.BodyStateRevision, Baseline: fmt.Sprint(revision.SourceBodyStateRevision), Candidate: fmt.Sprint(input.BodyStateRevision)},
		{Name: "generation_authority", Match: decision.Outcome == TreatmentAllowProposal, Baseline: string(TreatmentAllowProposal), Candidate: string(decision.Outcome)},
	}
	var storedDecision struct {
		PolicyRevision string                   `json:"policy_revision"`
		Phase          string                   `json:"phase"`
		Outcome        TreatmentDecisionOutcome `json:"outcome"`
	}
	_ = json.Unmarshal(revision.GenerationDecisionTrace, &storedDecision)
	checks = append(checks,
		TreatmentReplayCheck{Name: "stored_generation_policy", Match: storedDecision.PolicyRevision == decision.PolicyRevision, Baseline: storedDecision.PolicyRevision, Candidate: decision.PolicyRevision},
		TreatmentReplayCheck{Name: "stored_generation_phase", Match: storedDecision.Phase == TreatmentDecisionGeneration, Baseline: storedDecision.Phase, Candidate: TreatmentDecisionGeneration},
		TreatmentReplayCheck{Name: "stored_generation_outcome", Match: storedDecision.Outcome == decision.Outcome, Baseline: string(storedDecision.Outcome), Candidate: string(decision.Outcome)},
	)
	var sourceConfig struct {
		ID                     string `json:"id"`
		Role                   string `json:"role"`
		DecisionPolicyRevision string `json:"decision_policy_revision"`
	}
	_ = json.Unmarshal(revision.AgentConfiguration, &sourceConfig)
	checks = append(checks,
		TreatmentReplayCheck{Name: "source_configuration_id", Match: sourceConfig.ID == revision.AgentConfigurationID, Baseline: revision.AgentConfigurationID, Candidate: sourceConfig.ID},
		TreatmentReplayCheck{Name: "source_configuration_role", Match: sourceConfig.Role == "treatment", Baseline: "treatment", Candidate: sourceConfig.Role},
		TreatmentReplayCheck{Name: "source_decision_policy", Match: sourceConfig.DecisionPolicyRevision == decision.PolicyRevision, Baseline: sourceConfig.DecisionPolicyRevision, Candidate: decision.PolicyRevision},
	)
	var diagnosis struct {
		ID         string `json:"id"`
		AnalysisID string `json:"analysis_id"`
	}
	_ = json.Unmarshal(input.DiagnosisAnalysis, &diagnosis)
	frozenID := diagnosis.AnalysisID
	if frozenID == "" {
		frozenID = diagnosis.ID
	}
	checks = append(checks, TreatmentReplayCheck{Name: "diagnosis_analysis_id", Match: frozenID == revision.SourceDiagnosisAnalysisID.String(), Baseline: revision.SourceDiagnosisAnalysisID.String(), Candidate: frozenID})
	return newTreatmentReplayLayer(checks)
}

func treatmentReplaySnapshot(payload map[string]any) TreatmentReplaySnapshot {
	interventions := replayMapSlice(payload["interventions"])
	kinds := make([]string, 0, len(interventions))
	titles := make([]string, 0, len(interventions))
	for _, item := range interventions {
		kinds = append(kinds, replayString(item["kind"]))
		titles = append(titles, replayString(item["title"]))
	}
	sort.Strings(kinds)
	evidenceIDs := replayStringSlice(payload["evidence_ids"])
	sort.Strings(evidenceIDs)
	governance := replayMap(payload["governance"])
	return TreatmentReplaySnapshot{
		Status:             replayString(payload["status"]),
		GovernanceVerdict:  replayString(governance["verdict"]),
		InterventionCount:  len(interventions),
		InterventionKinds:  kinds,
		EvidenceIDs:        evidenceIDs,
		WarningSignCount:   len(replayStringSlice(payload["warning_signs"])),
		ReviewTriggerCount: len(replayStringSlice(payload["review_triggers"])),
		DurationWeeks:      replayInt(payload["duration_weeks"]),
		Summary:            replayString(payload["summary"]),
		Goal:               replayString(payload["goal"]),
		InterventionTitles: titles,
	}
}

func compareTreatmentReplayOutputs(baseline, replayed map[string]any) TreatmentReplayComparison {
	b := treatmentReplaySnapshot(baseline)
	r := treatmentReplaySnapshot(replayed)
	hard := newTreatmentReplayLayer([]TreatmentReplayCheck{
		{Name: "status", Match: b.Status == r.Status, Baseline: b.Status, Candidate: r.Status},
		{Name: "governance_verdict", Match: b.GovernanceVerdict == r.GovernanceVerdict, Baseline: b.GovernanceVerdict, Candidate: r.GovernanceVerdict},
		{Name: "proposal_only", Match: !treatmentReplayHasForbiddenFields(replayed), Candidate: fmt.Sprint(treatmentReplayHasForbiddenFields(replayed))},
	})
	semantic := newTreatmentReplayLayer([]TreatmentReplayCheck{
		{Name: "intervention_count", Match: b.InterventionCount == r.InterventionCount, Baseline: fmt.Sprint(b.InterventionCount), Candidate: fmt.Sprint(r.InterventionCount)},
		{Name: "intervention_kinds", Match: replayEqualStrings(b.InterventionKinds, r.InterventionKinds), Baseline: strings.Join(b.InterventionKinds, ","), Candidate: strings.Join(r.InterventionKinds, ",")},
		{Name: "evidence_ids", Match: replayEqualStrings(b.EvidenceIDs, r.EvidenceIDs), Baseline: strings.Join(b.EvidenceIDs, ","), Candidate: strings.Join(r.EvidenceIDs, ",")},
		{Name: "duration_weeks", Match: b.DurationWeeks == r.DurationWeeks, Baseline: fmt.Sprint(b.DurationWeeks), Candidate: fmt.Sprint(r.DurationWeeks)},
		{Name: "warning_sign_count", Match: b.WarningSignCount == r.WarningSignCount, Baseline: fmt.Sprint(b.WarningSignCount), Candidate: fmt.Sprint(r.WarningSignCount)},
		{Name: "review_trigger_count", Match: b.ReviewTriggerCount == r.ReviewTriggerCount, Baseline: fmt.Sprint(b.ReviewTriggerCount), Candidate: fmt.Sprint(r.ReviewTriggerCount)},
	})
	presentation := newTreatmentReplayLayer([]TreatmentReplayCheck{
		{Name: "summary", Match: b.Summary == r.Summary, Baseline: b.Summary, Candidate: r.Summary},
		{Name: "goal", Match: b.Goal == r.Goal, Baseline: b.Goal, Candidate: r.Goal},
		{Name: "intervention_titles", Match: replayEqualStrings(b.InterventionTitles, r.InterventionTitles), Baseline: strings.Join(b.InterventionTitles, ","), Candidate: strings.Join(r.InterventionTitles, ",")},
	})
	return TreatmentReplayComparison{Hard: hard, Semantic: semantic, Presentation: presentation}
}

func newTreatmentReplayLayer(checks []TreatmentReplayCheck) TreatmentReplayLayer {
	match := true
	for _, check := range checks {
		match = match && check.Match
	}
	return TreatmentReplayLayer{Match: match, Checks: checks}
}

func treatmentReplayInputFingerprint(input TreatmentReplayInput) string {
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func treatmentReplayHasForbiddenFields(payload map[string]any) bool {
	for _, key := range []string{"treatment_id", "treatment_revision_id", "revision_id", "accepted_at", "current_treatment", "training_plan"} {
		if _, exists := payload[key]; exists {
			return true
		}
	}
	return false
}

func sanitizeTreatmentReplayJSON(raw json.RawMessage, profile bool) any {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return map[string]any{}
	}
	return sanitizeTreatmentReplayValue(value, profile)
}

func sanitizeTreatmentReplayValue(value any, profile bool) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			lower := strings.ToLower(key)
			if lower == "user_id" {
				out[key] = "historical-regression"
				continue
			}
			if lower == "email" || lower == "phone" || lower == "avatar" || lower == "avatar_url" {
				continue
			}
			if profile && (lower == "id" || lower == "name" || lower == "full_name" || lower == "display_name") {
				continue
			}
			out[key] = sanitizeTreatmentReplayValue(item, profile)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeTreatmentReplayValue(item, profile))
		}
		return out
	default:
		return value
	}
}

func treatmentReplayEvidenceIDs(raw json.RawMessage) []string {
	var items []map[string]any
	_ = json.Unmarshal(raw, &items)
	seen := map[string]struct{}{}
	for _, item := range items {
		if evidenceID := replayString(item["evidence_id"]); evidenceID != "" {
			seen[evidenceID] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for evidenceID := range seen {
		ids = append(ids, evidenceID)
	}
	sort.Strings(ids)
	return ids
}

func treatmentReplayAssessmentStates(raw json.RawMessage) []string {
	var items []map[string]any
	_ = json.Unmarshal(raw, &items)
	seen := map[string]struct{}{}
	for _, item := range items {
		if state := replayString(item["state"]); state != "" {
			seen[state] = struct{}{}
		}
	}
	states := make([]string, 0, len(seen))
	for state := range seen {
		states = append(states, state)
	}
	sort.Strings(states)
	return states
}

func cloneTreatmentPayload(payload map[string]any) map[string]any {
	raw, _ := json.Marshal(payload)
	var cloned map[string]any
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
func decodeReplayObject(raw json.RawMessage) map[string]any {
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	if value == nil {
		value = map[string]any{}
	}
	return value
}
func decodeReplayStringArray(raw json.RawMessage) []string {
	var value []string
	_ = json.Unmarshal(raw, &value)
	return value
}
func replayMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}
func replayMapSlice(value any) []map[string]any {
	raw, _ := json.Marshal(value)
	var items []map[string]any
	_ = json.Unmarshal(raw, &items)
	return items
}
func replayString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
func replayStringSlice(value any) []string {
	raw, _ := json.Marshal(value)
	var items []string
	_ = json.Unmarshal(raw, &items)
	return items
}
func replayInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}
func replayEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
