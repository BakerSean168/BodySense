package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const TreatmentRolloutPolicyRevision = "treatment-rollout-policy-v1"

type treatmentRolloutRepository interface {
	Create(ctx context.Context, observation *model.TreatmentRolloutObservation) error
	ListRecent(ctx context.Context, championID, challengerID, stage string, canaryBPS, limit int) ([]model.TreatmentRolloutObservation, error)
}

type treatmentRolloutReplay interface {
	CounterfactualReplay(ctx context.Context, userID, revisionID uuid.UUID, targetConfigurationID string) (*TreatmentReplayReport, error)
}

type TreatmentRolloutSummary struct {
	PolicyRevision          string  `json:"policy_revision"`
	Stage                   string  `json:"stage"`
	Samples                 int     `json:"samples"`
	UnsafeRelaxations       int     `json:"unsafe_relaxations"`
	ForbiddenSideEffects    int     `json:"forbidden_side_effects"`
	ConfigurationMismatches int     `json:"configuration_mismatches"`
	ShadowErrors            int     `json:"shadow_errors"`
	HardMismatches          int     `json:"hard_mismatches"`
	SemanticMismatches      int     `json:"semantic_mismatches"`
	HardMismatchRate        float64 `json:"hard_mismatch_rate"`
	SemanticMismatchRate    float64 `json:"semantic_mismatch_rate"`
}

type TreatmentRolloutGate struct {
	PolicyRevision string   `json:"policy_revision"`
	Action         string   `json:"action"`
	Reasons        []string `json:"reasons"`
}

type TreatmentRolloutProgression struct {
	PolicyRevision string `json:"policy_revision"`
	Action         string `json:"action"`
	NextStage      string `json:"next_stage,omitempty"`
	NextCanaryBPS  int    `json:"next_canary_bps,omitempty"`
	Reason         string `json:"reason"`
}

type TreatmentRolloutService struct {
	repo   treatmentRolloutRepository
	replay treatmentRolloutReplay
}

func NewTreatmentRolloutService(repo treatmentRolloutRepository, replay treatmentRolloutReplay) *TreatmentRolloutService {
	return &TreatmentRolloutService{repo: repo, replay: replay}
}

// ObserveProposal pairs one already-persisted served proposal with the opposite
// immutable configuration. The shadow path is read-only; errors become rollout
// evidence and never change the served proposal result.
func (s *TreatmentRolloutService) ObserveProposal(
	ctx context.Context,
	userID uuid.UUID,
	route TreatmentRouteSelection,
	revisionID uuid.UUID,
) error {
	if s == nil || s.repo == nil || route.ShadowConfigurationID == "" {
		return nil
	}
	if s.replay == nil {
		return s.RecordComparison(
			ctx, route, revisionID, nil, errors.New("Treatment shadow replay service is not configured"),
		)
	}
	report, err := s.replay.CounterfactualReplay(ctx, userID, revisionID, route.ShadowConfigurationID)
	return s.RecordComparison(ctx, route, revisionID, report, err)
}

func (s *TreatmentRolloutService) RecordComparison(
	ctx context.Context,
	route TreatmentRouteSelection,
	sourceRevisionID uuid.UUID,
	report *TreatmentReplayReport,
	shadowErr error,
) error {
	if s == nil || s.repo == nil || route.ShadowConfigurationID == "" {
		return nil
	}
	canaryBPS := 0
	if route.Stage == TreatmentRolloutCanary {
		canaryBPS = route.CanaryBPS
	}
	observation := &model.TreatmentRolloutObservation{
		ID: uuid.New(), SourceRevisionID: sourceRevisionID,
		Stage: route.Stage, SubjectBucket: route.SubjectBucket, CanaryBPS: canaryBPS,
		ChampionConfigurationID:   route.ChampionConfigurationID,
		ChallengerConfigurationID: route.ChallengerConfigurationID,
		ServedConfigurationID:     route.ServedConfigurationID,
		ShadowConfigurationID:     route.ShadowConfigurationID,
		PromotionRecord:           route.PromotionRecord,
		Comparison:                datatypes.JSON(`{}`), CreatedAt: time.Now().UTC(),
	}
	if shadowErr != nil {
		observation.ShadowError = shadowErr.Error()
		if strings.Contains(strings.ToLower(shadowErr.Error()), "configuration") {
			observation.ConfigurationMismatch = true
		}
		return s.repo.Create(ctx, observation)
	}
	if report == nil {
		observation.ShadowError = "missing Treatment shadow comparison report"
		return s.repo.Create(ctx, observation)
	}
	encoded, _ := json.Marshal(report.Comparison)
	observation.Comparison = datatypes.JSON(encoded)
	observation.UnsafeRelaxation = treatmentRolloutUnsafeRelaxation(route, report)
	observation.ForbiddenSideEffect = treatmentReplayCheckCandidateTrue(report.Comparison.Hard, "proposal_only")
	observation.ConfigurationMismatch = !report.ArtifactIntegrity.Match ||
		report.SourceConfigurationID != route.ServedConfigurationID ||
		report.TargetConfigurationID != route.ShadowConfigurationID
	return s.repo.Create(ctx, observation)
}

func (s *TreatmentRolloutService) Summary(
	ctx context.Context,
	championID string,
	challengerID string,
	stage string,
	canaryBPS int,
	limit int,
) (TreatmentRolloutSummary, error) {
	items, err := s.repo.ListRecent(ctx, championID, challengerID, stage, canaryBPS, limit)
	if err != nil {
		return TreatmentRolloutSummary{}, err
	}
	return SummarizeTreatmentRollout(stage, items), nil
}

func SummarizeTreatmentRollout(stage string, items []model.TreatmentRolloutObservation) TreatmentRolloutSummary {
	summary := TreatmentRolloutSummary{PolicyRevision: TreatmentRolloutPolicyRevision, Stage: stage, Samples: len(items)}
	for _, item := range items {
		if item.UnsafeRelaxation {
			summary.UnsafeRelaxations++
		}
		if item.ForbiddenSideEffect {
			summary.ForbiddenSideEffects++
		}
		if item.ConfigurationMismatch {
			summary.ConfigurationMismatches++
		}
		if item.ShadowError != "" {
			summary.ShadowErrors++
		}
		var comparison TreatmentReplayComparison
		if json.Unmarshal(item.Comparison, &comparison) == nil {
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
	return summary
}

func EvaluateTreatmentRolloutGate(summary TreatmentRolloutSummary) TreatmentRolloutGate {
	gate := TreatmentRolloutGate{PolicyRevision: TreatmentRolloutPolicyRevision, Action: "continue", Reasons: []string{}}
	if summary.UnsafeRelaxations > 0 {
		gate.Action = "rollback"
		gate.Reasons = append(gate.Reasons, "unsafe_authority_relaxation")
	}
	if summary.ForbiddenSideEffects > 0 {
		gate.Action = "rollback"
		gate.Reasons = append(gate.Reasons, "forbidden_treatment_side_effect_surface")
	}
	if summary.ConfigurationMismatches > 0 {
		gate.Action = "rollback"
		gate.Reasons = append(gate.Reasons, "configuration_identity_mismatch")
	}
	if gate.Action == "rollback" {
		return gate
	}
	if summary.ShadowErrors >= 1 {
		gate.Action = "pause"
		gate.Reasons = append(gate.Reasons, "shadow_execution_error")
		return gate
	}
	if summary.Samples >= 20 && summary.HardMismatchRate > 0.10 {
		gate.Action = "pause"
		gate.Reasons = append(gate.Reasons, "hard_mismatch_rate_exceeded")
	}
	if summary.Samples >= 20 && summary.SemanticMismatchRate > 0.25 {
		gate.Action = "pause"
		gate.Reasons = append(gate.Reasons, "semantic_mismatch_rate_exceeded")
	}
	return gate
}

func EvaluateTreatmentRolloutProgression(
	stage string,
	canaryBPS int,
	summary TreatmentRolloutSummary,
) TreatmentRolloutProgression {
	gate := EvaluateTreatmentRolloutGate(summary)
	result := TreatmentRolloutProgression{
		PolicyRevision: TreatmentRolloutPolicyRevision,
		Action:         gate.Action,
		Reason:         "observation_gate_green",
	}
	if gate.Action != "continue" {
		result.Reason = strings.Join(gate.Reasons, ",")
		return result
	}
	const minimumSamples = 20
	switch stage {
	case TreatmentRolloutShadow:
		if summary.Samples < minimumSamples {
			result.Action = "wait"
			result.Reason = "shadow_min_samples_not_met"
			return result
		}
		result.Action = "advance"
		result.NextStage = TreatmentRolloutCanary
		result.NextCanaryBPS = 500
		result.Reason = "shadow_gate_passed"
		return result
	case TreatmentRolloutCanary:
		if summary.Samples < minimumSamples {
			result.Action = "wait"
			result.Reason = "canary_min_samples_not_met"
			return result
		}
		switch canaryBPS {
		case 500:
			result.Action, result.NextStage, result.NextCanaryBPS = "advance", TreatmentRolloutCanary, 2500
		case 2500:
			result.Action, result.NextStage, result.NextCanaryBPS = "advance", TreatmentRolloutCanary, 5000
		case 5000:
			result.Action, result.NextStage, result.NextCanaryBPS = "advance", TreatmentRolloutPromoted, 10000
		default:
			result.Action = "pause"
			result.Reason = "unapproved_canary_step"
			return result
		}
		result.Reason = "canary_gate_passed"
		return result
	case TreatmentRolloutPromoted:
		result.Action = "hold"
		result.Reason = "challenger_promoted"
		return result
	case TreatmentRolloutRollback:
		result.Action = "hold"
		result.Reason = "rollback_active"
		return result
	default:
		result.Action = "pause"
		result.Reason = "unsupported_progression_stage"
		return result
	}
}

func treatmentRolloutUnsafeRelaxation(route TreatmentRouteSelection, report *TreatmentReplayReport) bool {
	champion := report.SourceGenerationDecision.Outcome
	challenger := report.GenerationDecision.Outcome
	if route.ServedConfigurationID == route.ChallengerConfigurationID {
		champion, challenger = challenger, champion
	}
	return champion == TreatmentBlock && challenger == TreatmentAllowProposal
}

func treatmentReplayCheckCandidateTrue(layer TreatmentReplayLayer, name string) bool {
	for _, check := range layer.Checks {
		if check.Name == name && check.Candidate == "true" {
			return true
		}
	}
	return false
}
