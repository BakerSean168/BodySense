package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"strings"
)

var (
	ErrAssessmentReplayUnavailable   = errors.New("assessment replay input is unavailable")
	ErrAssessmentReplayNotFound      = errors.New("assessment report not found for replay")
	ErrAssessmentReplayConfiguration = errors.New("assessment replay configuration is invalid")
)

const AssessmentRegressionExportSchema = "assessment_qualification_v2"

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
	ContractRevision       string   `json:"contract_revision"`
	Status                 string   `json:"status"`
	EvidenceCoverageStatus string   `json:"evidence_coverage_status,omitempty"`
	ObservationCount       int      `json:"observation_count"`
	ObservationKinds       []string `json:"observation_kinds"`
	InformationGaps        []string `json:"information_gaps"`
	EvidenceGaps           []string `json:"evidence_gaps"`
	SafetyNoteCount        int      `json:"safety_note_count"`
	Summary                string   `json:"summary"`
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
	assessments assessmentRepository
	ai          *AIClient
}

// NewAssessmentReplayService attaches the Assessment repository (used to load an
// immutable report by id) and the AI client (used only by counterfactual replay,
// never by historical replay).
func NewAssessmentReplayService(assessments assessmentRepository, ai *AIClient) *AssessmentReplayService {
	return &AssessmentReplayService{assessments: assessments, ai: ai}
}

func (s *AssessmentReplayService) getReport(ctx context.Context, id, userID uuid.UUID) (*model.AssessmentReport, error) {
	if s == nil || s.assessments == nil {
		return nil, errors.New("Assessment replay repository is not configured")
	}
	return s.assessments.GetByID(ctx, id, userID)
}

func validateCurrentAssessmentReplaySource(report *model.AssessmentReport) error {
	if report.ContractRevision != assessmentOutputContractV2 {
		return fmt.Errorf(
			"%w: report %s uses retired contract revision %q",
			ErrAssessmentReplayConfiguration,
			report.ID,
			report.ContractRevision,
		)
	}
	registration, ok := knownAssessmentConfigurations[report.AgentConfigurationID]
	if !ok || registration.OutputContractRevision != assessmentOutputContractV2 {
		return fmt.Errorf(
			"%w: report %s uses retired Agent configuration %q",
			ErrAssessmentReplayConfiguration,
			report.ID,
			report.AgentConfigurationID,
		)
	}
	return nil
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
	report, err := s.getReport(ctx, reportID, userID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, ErrAssessmentReplayNotFound
	}
	if err := validateCurrentAssessmentReplaySource(report); err != nil {
		return nil, err
	}
	input, err := decodeAssessmentReplayInput(json.RawMessage(report.ReplayInput))
	if err != nil {
		return nil, err
	}
	baseline, err := assessmentReplayBaseline(report)
	if err != nil {
		return nil, err
	}
	recomputed := cloneAssessmentPayload(baseline)
	replayRaw, _ := json.Marshal(recomputed)
	return buildAssessmentReplayReport(
		"historical", report, input, report.AgentConfigurationID,
		baseline, recomputed, replayRaw,
	)
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
	report, err := s.getReport(ctx, reportID, userID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, ErrAssessmentReplayNotFound
	}
	if err := validateCurrentAssessmentReplaySource(report); err != nil {
		return nil, err
	}
	input, err := decodeAssessmentReplayInput(json.RawMessage(report.ReplayInput))
	if err != nil {
		return nil, err
	}
	registration, ok := knownAssessmentConfigurations[strings.TrimSpace(targetConfigurationID)]
	if !ok {
		return nil, fmt.Errorf("%w: unknown Assessment Agent configuration id %q", ErrAssessmentReplayConfiguration, targetConfigurationID)
	}
	if s.ai == nil {
		return nil, errors.New("Assessment replay AI client is not configured")
	}
	images := make([]string, 0, len(input.Images))
	// Images are stored as descriptors; counterfactual replay does not re-attach
	// raw bytes. The model is invoked with the profile + posture inputs only.
	result, err := s.ai.GenerateAssessment(ctx, AssessmentGenerationRequest{
		ConfigurationID:        targetConfigurationID,
		Profile:                input.Profile,
		BodyState:              input.BodyState,
		ReportIndicators:       input.ReportIndicators,
		ReviewedReportEvidence: input.ReviewedReportEvidence,
		PostureAnalysis:        input.PostureAnalysis,
		Images:                 images,
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
	payload, err := parseAssessmentAgentPayload(result, registration.EvidencePolicyRevision)
	if err != nil {
		return nil, fmt.Errorf("counterfactual Assessment evidence contract: %w", err)
	}
	request := AssessmentGenerationRequest{
		ConfigurationID: targetConfigurationID, Profile: input.Profile, BodyState: input.BodyState,
		ReportIndicators:       input.ReportIndicators,
		ReviewedReportEvidence: input.ReviewedReportEvidence,
		PostureAnalysis:        input.PostureAnalysis,
	}
	projection, err := validateAssessmentEvidencePayload(payload, request)
	if err != nil {
		return nil, fmt.Errorf("counterfactual Assessment evidence contract: %w", err)
	}
	replayed["status"] = projection.Status
	replayed["observations"] = projection.Observations
	replayed["evidence_coverage"] = projection.Coverage
	replayed["evidence_gaps"] = projection.Gaps
	replayed["summary"] = projection.Summary
	result, _ = json.Marshal(replayed)
	baseline, err := assessmentReplayBaseline(report)
	if err != nil {
		return nil, err
	}
	return buildAssessmentReplayReport(
		"counterfactual", report, input, targetConfigurationID,
		baseline, replayed, result,
	)
}

func (s *AssessmentReplayService) ExportRegressionCase(
	ctx context.Context,
	userID uuid.UUID,
	reportID uuid.UUID,
) (map[string]any, error) {
	report, err := s.getReport(ctx, reportID, userID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, ErrAssessmentReplayNotFound
	}
	if err := validateCurrentAssessmentReplaySource(report); err != nil {
		return nil, err
	}
	input, err := decodeAssessmentReplayInput(json.RawMessage(report.ReplayInput))
	if err != nil {
		return nil, err
	}
	baseline, err := assessmentReplayBaseline(report)
	if err != nil {
		return nil, err
	}
	snapshot := assessmentReplaySnapshot(baseline)
	executed := replayAgentExecuted(json.RawMessage(report.ExecutionProvenance))
	caseName := "historical-" + strings.ReplaceAll(report.ID.String()[:13], "-", "")
	return map[string]any{
		"schema_target":    AssessmentRegressionExportSchema,
		"source_report_id": report.ID,
		"case": map[string]any{
			"name": caseName,
			"inputs": map[string]any{
				"user_id":           "historical-regression",
				"profile":           sanitizeRegressionReplayJSON(input.Profile, true),
				"body_state":        sanitizeRegressionReplayJSON(input.BodyState, false),
				"report_indicators": sanitizeRegressionReplayJSON(input.ReportIndicators, false),
				"posture_analysis":  sanitizeRegressionReplayJSON(input.PostureAnalysis, false),
				"images":            input.Images,
			},
			"metadata": map[string]any{
				"scenario_family_id":                "historical-" + report.ID.String(),
				"case_category":                     "historical-regression",
				"split":                             "regression",
				"slices":                            []string{"historical-replay"},
				"critical":                          snapshot.Status == "insufficient_information",
				"expected_contract_revision":        snapshot.ContractRevision,
				"expected_status":                   snapshot.Status,
				"expected_evidence_coverage_status": snapshot.EvidenceCoverageStatus,
				"expected_evidence_gap_count":       len(snapshot.EvidenceGaps),
				"expected_agent_executed":           executed,
				"min_observations":                  snapshot.ObservationCount,
				"forbidden_output_fields":           assessmentRegressionForbiddenFields(snapshot.ContractRevision),
			},
		},
	}, nil
}
