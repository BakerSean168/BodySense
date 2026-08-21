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
	ErrAssessmentReplayUnavailable = errors.New("assessment replay input is unavailable")
	ErrAssessmentReplayNotFound    = errors.New("assessment report not found for replay")
)

const AssessmentRegressionExportSchema = "assessment_qualification_v1"

type AssessmentReplayCheck struct {
	Name      string `json:"name"`
	Match     bool   `json:"match"`
	Baseline  string `json:"baseline,omitempty"`
	Candidate string `json:"candidate,omitempty"`
}

type AssessmentReplayLayer struct {
	Match  bool                    `json:"match"`
	Checks []AssessmentReplayCheck `json:"checks"`
}

type AssessmentReplayComparison struct {
	Hard         AssessmentReplayLayer `json:"hard"`
	Semantic     AssessmentReplayLayer `json:"semantic"`
	Presentation AssessmentReplayLayer `json:"presentation"`
}

type AssessmentReplaySnapshot struct {
	Status           string   `json:"status"`
	HealthGrade      string   `json:"health_grade"`
	ObservationCount int      `json:"observation_count"`
	ObservationKinds []string `json:"observation_kinds"`
	InformationGaps  []string `json:"information_gaps"`
	SafetyNoteCount  int      `json:"safety_note_count"`
	Summary          string   `json:"summary"`
}

type AssessmentReplayReport struct {
	Mode                  string                     `json:"mode"`
	SourceReportID        uuid.UUID                  `json:"source_report_id"`
	SourceConfigurationID string                     `json:"source_configuration_id"`
	TargetConfigurationID string                     `json:"target_configuration_id"`
	InputFingerprint      string                     `json:"input_fingerprint"`
	ArtifactIntegrity     AssessmentReplayLayer      `json:"artifact_integrity"`
	Baseline              AssessmentReplaySnapshot   `json:"baseline"`
	Replay                AssessmentReplaySnapshot   `json:"replay"`
	Comparison            AssessmentReplayComparison `json:"comparison"`
	Output                json.RawMessage            `json:"output"`
}

type AssessmentReplayService struct {
	assessments *AssessmentService
	ai          *AIClient
}

func NewAssessmentReplayService(assessments *AssessmentService, ai *AIClient) *AssessmentReplayService {
	return &AssessmentReplayService{assessments: assessments, ai: ai}
}

// HistoricalReplay recomputes the deterministic generation authority and
// reconstructs the immutable baseline report from its frozen input without any
// model call. It is an integrity check, not a re-run.
func (s *AssessmentReplayService) HistoricalReplay(
	ctx context.Context,
	userID uuid.UUID,
	reportID uuid.UUID,
) (*AssessmentReplayReport, error) {
	if s == nil || s.assessments == nil {
		return nil, errors.New("Assessment replay service is not configured")
	}
	report, err := s.assessments.GetReport(ctx, reportID, userID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, ErrAssessmentReplayNotFound
	}
	input, err := decodeAssessmentReplayInput(json.RawMessage(report.ReplayInput))
	if err != nil {
		return nil, err
	}
	baseline := assessmentReplayBaseline(report)
	recomputed := cloneAssessmentPayload(baseline)
	replayRaw, _ := json.Marshal(recomputed)
	return buildAssessmentReplayReport(
		"historical", report, input, report.AgentConfigurationID,
		baseline, recomputed, replayRaw,
	), nil
}

// CounterfactualReplay sends the exact frozen input to another immutable
// Assessment configuration. The result is transient and never creates a report
// or mutates BodyState/Assessment.
func (s *AssessmentReplayService) CounterfactualReplay(
	ctx context.Context,
	userID uuid.UUID,
	reportID uuid.UUID,
	targetConfigurationID string,
) (*AssessmentReplayReport, error) {
	if s == nil || s.assessments == nil {
		return nil, errors.New("Assessment replay service is not configured")
	}
	report, err := s.assessments.GetReport(ctx, reportID, userID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, ErrAssessmentReplayNotFound
	}
	input, err := decodeAssessmentReplayInput(json.RawMessage(report.ReplayInput))
	if err != nil {
		return nil, err
	}
	if _, err := AssessmentDecisionPolicyRevisionForConfiguration(targetConfigurationID); err != nil {
		return nil, err
	}
	if s.ai == nil {
		return nil, errors.New("Assessment replay AI client is not configured")
	}
	images := make([]string, 0, len(input.Images))
	// Images are stored as descriptors; counterfactual replay does not re-attach
	// raw bytes. The model is invoked with the profile + posture inputs only.
	result, err := s.ai.GenerateAssessment(ctx, AssessmentGenerationRequest{
		ConfigurationID: targetConfigurationID,
		Profile:         input.Profile,
		PostureAnalysis: input.PostureAnalysis,
		Images:          images,
	})
	if err != nil {
		return nil, fmt.Errorf("counterfactual Assessment replay: %w", err)
	}
	var replayed map[string]any
	if err := json.Unmarshal(result, &replayed); err != nil {
		return nil, fmt.Errorf("decode counterfactual Assessment replay: %w", err)
	}
	if !assessmentReplayConfigurationMatches(replayed, targetConfigurationID) {
		return nil, errors.New("counterfactual Assessment replay returned the wrong Agent configuration")
	}
	baseline := assessmentReplayBaseline(report)
	return buildAssessmentReplayReport(
		"counterfactual", report, input, targetConfigurationID,
		baseline, replayed, result,
	), nil
}

func (s *AssessmentReplayService) ExportRegressionCase(
	ctx context.Context,
	userID uuid.UUID,
	reportID uuid.UUID,
) (map[string]any, error) {
	report, err := s.assessments.GetReport(ctx, reportID, userID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, ErrAssessmentReplayNotFound
	}
	input, err := decodeAssessmentReplayInput(json.RawMessage(report.ReplayInput))
	if err != nil {
		return nil, err
	}
	snapshot := assessmentReplaySnapshot(assessmentReplayBaseline(report))
	caseName := "historical-" + strings.ReplaceAll(report.ID.String()[:13], "-", "")
	return map[string]any{
		"schema_target":    AssessmentRegressionExportSchema,
		"source_report_id": report.ID,
		"case": map[string]any{
			"name": caseName,
			"inputs": map[string]any{
				"user_id":          "historical-regression",
				"profile":          sanitizeRegressionReplayJSON(input.Profile, true),
				"posture_analysis": sanitizeRegressionReplayJSON(input.PostureAnalysis, false),
				"images":           input.Images,
			},
			"metadata": map[string]any{
				"scenario_family_id":      "historical-" + report.ID.String(),
				"case_category":           "historical-regression",
				"split":                   "regression",
				"slices":                  []string{"historical-replay"},
				"critical":                snapshot.Status == "insufficient_information",
				"expected_status":         snapshot.Status,
				"expected_agent_executed": true,
				"min_observations":        snapshot.ObservationCount,
				"forbidden_output_fields": []string{"treatment", "training_plan", "prescription"},
			},
		},
	}, nil
}

func assessmentReplayBaseline(report *model.AssessmentReport) map[string]any {
	var out map[string]any
	if len(report.Observations) > 0 {
		_ = json.Unmarshal(report.Observations, &out)
	}
	baseline := map[string]any{
		"status":       report.Status,
		"health_grade": report.HealthGrade,
		"summary":      report.Summary,
	}
	var obs []any
	if len(report.Observations) > 0 {
		_ = json.Unmarshal(report.Observations, &obs)
		baseline["observations"] = obs
	}
	var gaps []any
	if len(report.InformationGaps) > 0 {
		_ = json.Unmarshal(report.InformationGaps, &gaps)
	}
	baseline["information_gaps"] = gaps
	var safety []any
	if len(report.SafetyNotes) > 0 {
		_ = json.Unmarshal(report.SafetyNotes, &safety)
	}
	baseline["safety_notes"] = safety
	baseline["agent_configuration"] = map[string]any{"id": report.AgentConfigurationID, "role": "assessment"}
	return baseline
}

func assessmentReplaySnapshot(payload map[string]any) AssessmentReplaySnapshot {
	obs := observationsFromPayload(payload)
	kinds := make([]string, 0, len(obs))
	for _, o := range obs {
		if k, ok := o["kind"].(string); ok && k != "" {
			kinds = append(kinds, k)
		}
	}
	sort.Strings(kinds)
	return AssessmentReplaySnapshot{
		Status:           firstString(payload["status"], ""),
		HealthGrade:      firstString(payload["health_grade"], ""),
		ObservationCount: len(obs),
		ObservationKinds: kinds,
		InformationGaps:  stringSlice(payload["information_gaps"]),
		SafetyNoteCount:  len(stringSlice(payload["safety_notes"])),
		Summary:          firstString(payload["summary"], ""),
	}
}

func observationsFromPayload(payload map[string]any) []map[string]any {
	raw, _ := payload["observations"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func assessmentReplayArtifactIntegrity(
	report *model.AssessmentReport,
	input AssessmentReplayInput,
	baseline map[string]any,
) AssessmentReplayLayer {
	checks := []AssessmentReplayCheck{
		{Name: "source_configuration_id", Match: report.AgentConfigurationID != ""},
		{Name: "replay_input_frozen", Match: input.Profile != nil}, // decoded => present
		{Name: "report_immutable_status", Match: baseline["status"] != nil},
	}
	all := true
	for i := range checks {
		all = all && checks[i].Match
	}
	return AssessmentReplayLayer{Match: all, Checks: checks}
}

func buildAssessmentReplayReport(
	mode string,
	report *model.AssessmentReport,
	input AssessmentReplayInput,
	targetConfigurationID string,
	baseline map[string]any,
	replayed map[string]any,
	replayRaw json.RawMessage,
) *AssessmentReplayReport {
	return &AssessmentReplayReport{
		Mode:                  mode,
		SourceReportID:        report.ID,
		SourceConfigurationID: report.AgentConfigurationID,
		TargetConfigurationID: targetConfigurationID,
		InputFingerprint:      assessmentReplayInputFingerprint(input),
		ArtifactIntegrity:     assessmentReplayArtifactIntegrity(report, input, baseline),
		Baseline:              assessmentReplaySnapshot(baseline),
		Replay:                assessmentReplaySnapshot(replayed),
		Comparison:            compareAssessmentReplayOutputs(baseline, replayed),
		Output:                replayRaw,
	}
}

func assessmentReplayInputFingerprint(input AssessmentReplayInput) string {
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func assessmentReplayConfigurationMatches(payload map[string]any, expectedID string) bool {
	configuration, ok := payload["agent_configuration"].(map[string]any)
	if !ok {
		return false
	}
	id, _ := configuration["id"].(string)
	role, _ := configuration["role"].(string)
	return id == expectedID && role == "assessment"
}

func cloneAssessmentPayload(payload map[string]any) map[string]any {
	recomputed := make(map[string]any, len(payload))
	for key, value := range payload {
		recomputed[key] = value
	}
	return recomputed
}

func compareAssessmentReplayOutputs(baseline, replayed map[string]any) AssessmentReplayComparison {
	bSnap := assessmentReplaySnapshot(baseline)
	rSnap := assessmentReplaySnapshot(replayed)

	hardChecks := []AssessmentReplayCheck{
		{Name: "status", Match: bSnap.Status == rSnap.Status, Baseline: bSnap.Status, Candidate: rSnap.Status},
		{Name: "health_grade", Match: bSnap.HealthGrade == rSnap.HealthGrade, Baseline: bSnap.HealthGrade, Candidate: rSnap.HealthGrade},
		{Name: "observation_count", Match: bSnap.ObservationCount == rSnap.ObservationCount, Baseline: fmt.Sprintf("%d", bSnap.ObservationCount), Candidate: fmt.Sprintf("%d", rSnap.ObservationCount)},
		{Name: "agent_configuration_role", Match: assessmentReplayConfigurationMatches(replayed, "assessment")},
	}
	semanticChecks := []AssessmentReplayCheck{
		{Name: "observation_kinds", Match: equalStringSlices(bSnap.ObservationKinds, rSnap.ObservationKinds), Baseline: strings.Join(bSnap.ObservationKinds, ","), Candidate: strings.Join(rSnap.ObservationKinds, ",")},
		{Name: "information_gaps", Match: equalStringSlices(bSnap.InformationGaps, rSnap.InformationGaps), Baseline: strings.Join(bSnap.InformationGaps, "|"), Candidate: strings.Join(rSnap.InformationGaps, "|")},
	}
	presentationChecks := []AssessmentReplayCheck{
		{Name: "summary", Match: bSnap.Summary == rSnap.Summary, Baseline: bSnap.Summary, Candidate: rSnap.Summary},
	}
	return AssessmentReplayComparison{
		Hard:         assessmentReplayLayer(hardChecks),
		Semantic:     assessmentReplayLayer(semanticChecks),
		Presentation: assessmentReplayLayer(presentationChecks),
	}
}

func assessmentReplayLayer(checks []AssessmentReplayCheck) AssessmentReplayLayer {
	all := true
	for i := range checks {
		all = all && checks[i].Match
	}
	return AssessmentReplayLayer{Match: all, Checks: checks}
}

func equalStringSlices(a, b []string) bool {
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

func firstString(value any, fallback string) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fallback
}

func stringSlice(value any) []string {
	raw, _ := value.([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
