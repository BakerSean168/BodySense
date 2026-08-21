package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const AssessmentRolloutPolicyRevision = "assessment-rollout-policy-v1"

type assessmentRolloutRepository interface {
	Create(ctx context.Context, observation *model.AssessmentRolloutObservation) error
	ListRecent(ctx context.Context, championID, challengerID, stage string, canaryBPS, limit int) ([]model.AssessmentRolloutObservation, error)
}

type assessmentRolloutReplay interface {
	CounterfactualReplay(ctx context.Context, userID, reportID uuid.UUID, targetConfigurationID string) (*AssessmentReplayReport, error)
}

type AssessmentRolloutSummary struct {
	PolicyRevision          string  `json:"policy_revision"`
	Stage                   string  `json:"stage"`
	Samples                 int     `json:"samples"`
	ForbiddenSideEffects    int     `json:"forbidden_side_effects"`
	ConfigurationMismatches int     `json:"configuration_mismatches"`
	ShadowErrors            int     `json:"shadow_errors"`
	HardMismatches          int     `json:"hard_mismatches"`
	SemanticMismatches      int     `json:"semantic_mismatches"`
	HardMismatchRate        float64 `json:"hard_mismatch_rate"`
	SemanticMismatchRate    float64 `json:"semantic_mismatch_rate"`
}

type AssessmentRolloutProgression struct {
	PolicyRevision string `json:"policy_revision"`
	Action         string `json:"action"`
	NextStage      string `json:"next_stage,omitempty"`
	Reason         string `json:"reason"`
}

type AssessmentRolloutService struct {
	repo   assessmentRolloutRepository
	replay assessmentRolloutReplay
}

func NewAssessmentRolloutService(repo assessmentRolloutRepository, replay assessmentRolloutReplay) *AssessmentRolloutService {
	return &AssessmentRolloutService{repo: repo, replay: replay}
}

// ObserveReport pairs one already-persisted served report with the opposite
// immutable Assessment configuration. The shadow path is read-only; errors
// become rollout evidence and never change the served report result.
func (s *AssessmentRolloutService) ObserveReport(
	ctx context.Context,
	userID uuid.UUID,
	route AssessmentRouteSelection,
	reportID uuid.UUID,
) error {
	if s == nil || s.repo == nil || route.ShadowConfigurationID == "" {
		return nil
	}
	if s.replay == nil {
		return s.RecordComparison(
			ctx, route, reportID, nil,
			errors.New("Assessment shadow replay service is not configured"),
		)
	}
	report, err := s.replay.CounterfactualReplay(ctx, userID, reportID, route.ShadowConfigurationID)
	if err != nil {
		return s.RecordComparison(ctx, route, reportID, nil, err)
	}
	return s.RecordComparison(ctx, route, reportID, report, nil)
}

func (s *AssessmentRolloutService) RecordComparison(
	ctx context.Context,
	route AssessmentRouteSelection,
	reportID uuid.UUID,
	report *AssessmentReplayReport,
	shadowErr error,
) error {
	if s == nil || s.repo == nil {
		return nil
	}
	observation := &model.AssessmentRolloutObservation{
		ID:                        uuid.New(),
		SourceReportID:            reportID,
		Stage:                     route.Stage,
		SubjectBucket:             route.SubjectBucket,
		CanaryBPS:                 route.CanaryBPS,
		ChampionConfigurationID:   route.ChampionConfigurationID,
		ChallengerConfigurationID: route.ChallengerConfigurationID,
		ServedConfigurationID:     route.ServedConfigurationID,
		ShadowConfigurationID:     route.ShadowConfigurationID,
		PromotionRecord:           route.PromotionRecord,
		CreatedAt:                 time.Now().UTC(),
	}
	if shadowErr != nil {
		observation.ShadowError = shadowErr.Error()
	} else if report != nil {
		observation.Comparison = datatypes.JSON(assessmentMustJSON(report.Comparison))
		observation.ForbiddenSideEffect = reportOutputHasForbiddenField(report.Output)
		// Identity mismatch is about the Agent configuration, not output drift:
		// the replay envelope + report must agree, and the shadow must have been
		// produced by the challenger. Output drift is measured separately via
		// Comparison.Hard/Semantic mismatch rates.
		observation.ConfigurationMismatch =
			!report.ArtifactIntegrity.Match ||
				report.SourceConfigurationID != route.ServedConfigurationID ||
				report.TargetConfigurationID != route.ShadowConfigurationID
	}
	return s.repo.Create(ctx, observation)
}

func (s *AssessmentRolloutService) Summarize(
	ctx context.Context,
	championID, challengerID, stage string,
	canaryBPS, limit int,
) (*AssessmentRolloutSummary, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("Assessment rollout service is not configured")
	}
	rows, err := s.repo.ListRecent(ctx, championID, challengerID, stage, canaryBPS, limit)
	if err != nil {
		return nil, err
	}
	summary := &AssessmentRolloutSummary{
		PolicyRevision: AssessmentRolloutPolicyRevision,
		Stage:          stage,
		Samples:        len(rows),
	}
	for _, row := range rows {
		if row.ShadowError != "" {
			summary.ShadowErrors++
		}
		if row.ForbiddenSideEffect {
			summary.ForbiddenSideEffects++
		}
		if row.ConfigurationMismatch {
			summary.ConfigurationMismatches++
		}
		var comparison AssessmentReplayComparison
		if len(row.Comparison) > 0 && json.Unmarshal(row.Comparison, &comparison) == nil {
			if !comparison.Hard.Match {
				summary.HardMismatches++
			}
			if !comparison.Semantic.Match {
				summary.SemanticMismatches++
			}
		}
	}
	if summary.Samples > 0 {
		summary.HardMismatchRate = float64(summary.HardMismatches) / float64(summary.Samples)
		summary.SemanticMismatchRate = float64(summary.SemanticMismatches) / float64(summary.Samples)
	}
	return summary, nil
}

// Assessment rollout gate constants (mirror Treatment thresholds).
const (
	AssessmentRolloutMinSamples            = 20
	AssessmentRolloutHardMismatchLimit     = 0.10 // 10%
	AssessmentRolloutSemanticMismatchLimit = 0.25 // 25%
)

// Progression is deny-first: it recommends the next stage only when the evidence
// window is large enough, clean enough, and a promotion record is present for
// non-Champion goals. It can also recommend rollback when blocking signals appear.
func (s *AssessmentRolloutService) Progression(
	summary *AssessmentRolloutSummary,
	hasPromotionRecord bool,
) *AssessmentRolloutProgression {
	if summary == nil {
		return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "hold", Reason: "no evidence yet"}
	}
	// Deny-first: any blocking signal rolls back or holds, never promotes.
	if summary.ForbiddenSideEffects > 0 || summary.ConfigurationMismatches > 0 {
		return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "rollback", Reason: "blocking signals present (forbidden side effect or configuration mismatch)"}
	}
	if summary.Samples < AssessmentRolloutMinSamples {
		return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "hold", Reason: "insufficient samples"}
	}
	if summary.ShadowErrors > 0 {
		return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "hold", Reason: "shadow errors present"}
	}
	if summary.HardMismatchRate > AssessmentRolloutHardMismatchLimit ||
		summary.SemanticMismatchRate > AssessmentRolloutSemanticMismatchLimit {
		return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "hold", Reason: "mismatch rate above threshold"}
	}
	switch summary.Stage {
	case "shadow":
		return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "promote", NextStage: "canary", Reason: "shadow stable"}
	case "canary":
		if hasPromotionRecord {
			return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "promote", NextStage: "promoted", Reason: "canary clean and promotion record present"}
		}
		return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "hold", Reason: "promotion record missing"}
	}
	return &AssessmentRolloutProgression{PolicyRevision: AssessmentRolloutPolicyRevision, Action: "hold", Reason: "unknown stage"}
}

func reportOutputHasForbiddenField(output json.RawMessage) bool {
	var payload map[string]any
	if json.Unmarshal(output, &payload) != nil {
		return false
	}
	for _, field := range []string{"treatment", "training_plan", "prescription"} {
		if _, ok := payload[field]; ok {
			return true
		}
	}
	return false
}

func assessmentMustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
