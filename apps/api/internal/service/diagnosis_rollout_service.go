package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const DiagnosisRolloutPolicyRevision = "diagnosis-rollout-policy-v1"

type diagnosisRolloutRepository interface {
	Create(ctx context.Context, observation *model.DiagnosisRolloutObservation) error
	ListRecent(ctx context.Context, championID, challengerID, stage string, canaryBPS, limit int) ([]model.DiagnosisRolloutObservation, error)
}

type DiagnosisRolloutSummary struct {
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

type DiagnosisRolloutGate struct {
	PolicyRevision string   `json:"policy_revision"`
	Action         string   `json:"action"`
	Reasons        []string `json:"reasons"`
}

type DiagnosisRolloutProgression struct {
	PolicyRevision string `json:"policy_revision"`
	Action         string `json:"action"`
	NextStage      string `json:"next_stage,omitempty"`
	NextCanaryBPS  int    `json:"next_canary_bps,omitempty"`
	Reason         string `json:"reason"`
}

type DiagnosisRolloutService struct{ repo diagnosisRolloutRepository }

func NewDiagnosisRolloutService(repo diagnosisRolloutRepository) *DiagnosisRolloutService {
	return &DiagnosisRolloutService{repo: repo}
}

func (s *DiagnosisRolloutService) RecordComparison(
	ctx context.Context,
	route DiagnosisRouteSelection,
	sourceAnalysisID uuid.UUID,
	report *DiagnosisReplayReport,
	shadowErr error,
) error {
	if s == nil || s.repo == nil || route.ShadowConfigurationID == "" {
		return nil
	}
	canaryBPS := 0
	if route.Stage == DiagnosisRolloutCanary {
		canaryBPS = route.CanaryBPS
	}
	var sourceAnalysis *uuid.UUID
	if sourceAnalysisID != uuid.Nil {
		value := sourceAnalysisID
		sourceAnalysis = &value
	}
	observation := &model.DiagnosisRolloutObservation{
		ID: uuid.New(), SourceAnalysisID: sourceAnalysis,
		Stage: route.Stage, SubjectBucket: route.SubjectBucket, CanaryBPS: canaryBPS,
		ChampionConfigurationID:   route.ChampionConfigurationID,
		ChallengerConfigurationID: route.ChallengerConfigurationID,
		ServedConfigurationID:     route.ServedConfigurationID,
		ShadowConfigurationID:     route.ShadowConfigurationID,
		Comparison:                datatypes.JSON(`{}`), CreatedAt: time.Now().UTC(),
	}
	if shadowErr != nil {
		observation.ShadowError = shadowErr.Error()
		if strings.Contains(shadowErr.Error(), "configuration") {
			observation.ConfigurationMismatch = true
		}
		return s.repo.Create(ctx, observation)
	}
	if report == nil {
		observation.ShadowError = "missing shadow comparison report"
		return s.repo.Create(ctx, observation)
	}
	encoded, _ := json.Marshal(report.Comparison)
	observation.Comparison = datatypes.JSON(encoded)
	observation.UnsafeRelaxation = diagnosisRolloutUnsafeRelaxation(route, report)
	observation.ForbiddenSideEffect = replayComparisonCandidateTrue(report.Comparison.Hard, "forbidden_side_effects")
	observation.ConfigurationMismatch = !report.ArtifactIntegrity.Match
	return s.repo.Create(ctx, observation)
}

func (s *DiagnosisRolloutService) Summary(
	ctx context.Context,
	championID string,
	challengerID string,
	stage string,
	canaryBPS int,
	limit int,
) (DiagnosisRolloutSummary, error) {
	items, err := s.repo.ListRecent(ctx, championID, challengerID, stage, canaryBPS, limit)
	if err != nil {
		return DiagnosisRolloutSummary{}, err
	}
	return SummarizeDiagnosisRollout(stage, items), nil
}

func SummarizeDiagnosisRollout(stage string, items []model.DiagnosisRolloutObservation) DiagnosisRolloutSummary {
	summary := DiagnosisRolloutSummary{PolicyRevision: DiagnosisRolloutPolicyRevision, Stage: stage, Samples: len(items)}
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
		var comparison DiagnosisReplayComparison
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

// EvaluateDiagnosisRolloutGate implements the predeclared Phase-9 stop rules.
// Unsafe relaxations and forbidden side effects roll back immediately. Runtime
// errors pause progression; rate gates apply only after a minimum sample count.
func EvaluateDiagnosisRolloutGate(summary DiagnosisRolloutSummary) DiagnosisRolloutGate {
	gate := DiagnosisRolloutGate{PolicyRevision: DiagnosisRolloutPolicyRevision, Action: "continue", Reasons: []string{}}
	if summary.UnsafeRelaxations > 0 {
		gate.Action = "rollback"
		gate.Reasons = append(gate.Reasons, "unsafe_authority_relaxation")
	}
	if summary.ForbiddenSideEffects > 0 {
		gate.Action = "rollback"
		gate.Reasons = append(gate.Reasons, "forbidden_diagnosis_side_effect")
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

func diagnosisRolloutUnsafeRelaxation(route DiagnosisRouteSelection, report *DiagnosisReplayReport) bool {
	champion := report.Baseline.DecisionOutcome
	challenger := report.Replay.DecisionOutcome
	if route.ServedConfigurationID == route.ChallengerConfigurationID {
		champion, challenger = challenger, champion
	}
	return diagnosisRestrictiveOutcome(champion) && diagnosisAllowOutcome(challenger)
}

func diagnosisRestrictiveOutcome(outcome string) bool {
	return outcome == string(DiagnosisBlock) || outcome == string(DiagnosisEscalate) || outcome == string(DiagnosisAbstain)
}

func diagnosisAllowOutcome(outcome string) bool {
	return outcome == string(DiagnosisAllowNormal) || outcome == string(DiagnosisAllowDegraded)
}

func replayComparisonCandidateTrue(layer DiagnosisReplayLayer, name string) bool {
	for _, check := range layer.Checks {
		if check.Name == name && check.Candidate == "true" {
			return true
		}
	}
	return false
}

// EvaluateDiagnosisRolloutProgression turns a green observation gate into the
// next predeclared rollout step. It never mutates deployment state itself.
func EvaluateDiagnosisRolloutProgression(
	stage string,
	canaryBPS int,
	summary DiagnosisRolloutSummary,
) DiagnosisRolloutProgression {
	gate := EvaluateDiagnosisRolloutGate(summary)
	result := DiagnosisRolloutProgression{
		PolicyRevision: DiagnosisRolloutPolicyRevision,
		Action:         gate.Action,
		Reason:         "observation_gate_green",
	}
	if gate.Action != "continue" {
		result.Reason = strings.Join(gate.Reasons, ",")
		return result
	}
	const minimumSamples = 20
	switch stage {
	case DiagnosisRolloutShadow:
		if summary.Samples < minimumSamples {
			result.Action = "wait"
			result.Reason = "shadow_min_samples_not_met"
			return result
		}
		result.Action = "advance"
		result.NextStage = DiagnosisRolloutCanary
		result.NextCanaryBPS = 500
		result.Reason = "shadow_gate_passed"
		return result
	case DiagnosisRolloutCanary:
		if summary.Samples < minimumSamples {
			result.Action = "wait"
			result.Reason = "canary_min_samples_not_met"
			return result
		}
		switch canaryBPS {
		case 500:
			result.Action, result.NextStage, result.NextCanaryBPS = "advance", DiagnosisRolloutCanary, 2500
		case 2500:
			result.Action, result.NextStage, result.NextCanaryBPS = "advance", DiagnosisRolloutCanary, 5000
		case 5000:
			result.Action, result.NextStage, result.NextCanaryBPS = "advance", DiagnosisRolloutPromoted, 10000
		default:
			result.Action = "pause"
			result.Reason = "unapproved_canary_step"
			return result
		}
		result.Reason = "canary_gate_passed"
		return result
	case DiagnosisRolloutPromoted:
		result.Action = "hold"
		result.Reason = "challenger_promoted"
		return result
	case DiagnosisRolloutRollback:
		result.Action = "hold"
		result.Reason = "rollback_active"
		return result
	default:
		result.Action = "pause"
		result.Reason = "unsupported_progression_stage"
		return result
	}
}
