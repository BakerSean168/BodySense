package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Consultation rollout gate thresholds (mirror Assessment).
const (
	ConsultationRolloutMinSamples        = 20
	ConsultationRolloutHardMismatchLimit = 0.10
)

// ConsultationRolloutSummary is the aggregate evidence window for one
// challenger configuration at one stage.
type ConsultationRolloutSummary struct {
	Stage                string
	Samples              int64
	ShadowErrors         int64
	DecisionMismatches   int64
	ReplayInputNotFrozen int64
}

// ConsultationRolloutAction is the deny-first progression recommendation.
type ConsultationRolloutAction struct {
	Action    string `json:"action"` // hold | promote | rollback
	Stage     string `json:"stage"`
	NextStage string `json:"next_stage,omitempty"`
	Reason    string `json:"reason"`
}

// ConsultationRolloutService records anonymous shadow/canary observations and
// recommends progression. Evidence is aggregate-only (no user identity).
type ConsultationRolloutService struct {
	repo rolloutObservationRepo
}

type rolloutObservationRepo interface {
	Create(ctx context.Context, observation *model.ConsultationRolloutObservation) error
	ListByChallenger(ctx context.Context, challengerConfigurationID, stage string, limit int) ([]model.ConsultationRolloutObservation, error)
	CountByChallenger(ctx context.Context, challengerConfigurationID, stage string) (int64, error)
}

func NewConsultationRolloutService(repo rolloutObservationRepo) *ConsultationRolloutService {
	return &ConsultationRolloutService{repo: repo}
}

// ObserveRun records anonymous shadow evidence for a completed run. It is
// non-blocking: errors are returned to the caller (runtime logs them) but never
// change the served reply.
func (s *ConsultationRolloutService) ObserveRun(
	ctx context.Context,
	run *model.Run,
	decision *ConsultationRunDecision,
) error {
	if s == nil || s.repo == nil || run == nil {
		return nil
	}
	challenger := decision.SourceConfigurationID
	if decision.SourceConfigurationID == run.AgentConfigurationID {
		// No distinct challenger: nothing to observe.
		return nil
	}
	observation := &model.ConsultationRolloutObservation{
		ID:                        uuid.New(),
		RunID:                     run.ID,
		ConversationID:            run.ConversationID,
		Stage:                     AssessmentRolloutShadow,
		ChampionConfigurationID:   run.AgentConfigurationID,
		ChallengerConfigurationID: challenger,
		DecisionIdentityMatch:     decision.ConfigurationIdentityMatch,
		ReplayInputFrozen:         decision.ReplayInputFrozen,
	}
	if !decision.ConfigurationIdentityMatch || !decision.ReplayInputFrozen {
		observation.ShadowError = "decision authority mismatch or replay input not frozen"
	}
	observation.Comparison = datatypes.JSON(consultationDecisionJSON(decision))
	return s.repo.Create(ctx, observation)
}

// Summarize aggregates evidence for one challenger at one stage.
func (s *ConsultationRolloutService) Summarize(
	ctx context.Context,
	challengerConfigurationID string,
	stage string,
) (*ConsultationRolloutSummary, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("consultation rollout service unavailable")
	}
	observations, err := s.repo.ListByChallenger(ctx, challengerConfigurationID, stage, 200)
	if err != nil {
		return nil, err
	}
	summary := &ConsultationRolloutSummary{Stage: stage}
	for _, observation := range observations {
		summary.Samples++
		if observation.ShadowError != "" {
			summary.ShadowErrors++
		}
		if !observation.DecisionIdentityMatch {
			summary.DecisionMismatches++
		}
		if !observation.ReplayInputFrozen {
			summary.ReplayInputNotFrozen++
		}
	}
	return summary, nil
}

// Progression is deny-first: it recommends promote only when the evidence
// window is large enough and clean; blocking signals trigger rollback.
func (s *ConsultationRolloutService) Progression(
	summary *ConsultationRolloutSummary,
	hasPromotionRecord bool,
) ConsultationRolloutAction {
	if summary == nil {
		return ConsultationRolloutAction{Action: "hold", Stage: "", Reason: "no evidence"}
	}
	if summary.ShadowErrors > 0 || summary.DecisionMismatches > 0 || summary.ReplayInputNotFrozen > 0 {
		return ConsultationRolloutAction{
			Action: "rollback",
			Stage:  summary.Stage,
			Reason: "blocking decision or replay-input signals observed",
		}
	}
	if summary.Samples < ConsultationRolloutMinSamples {
		return ConsultationRolloutAction{
			Action: "hold",
			Stage:  summary.Stage,
			Reason: "insufficient samples for promotion",
		}
	}
	switch summary.Stage {
	case AssessmentRolloutShadow:
		return ConsultationRolloutAction{Action: "promote", Stage: "shadow", NextStage: "canary", Reason: "clean shadow window"}
	case AssessmentRolloutCanary:
		if !hasPromotionRecord {
			return ConsultationRolloutAction{Action: "hold", Stage: "canary", Reason: "canary requires an approved promotion record"}
		}
		return ConsultationRolloutAction{Action: "promote", Stage: "canary", NextStage: "promoted", Reason: "clean canary window"}
	default:
		return ConsultationRolloutAction{Action: "hold", Stage: summary.Stage, Reason: "no progression from this stage"}
	}
}

func consultationDecisionJSON(decision *ConsultationRunDecision) json.RawMessage {
	encoded, err := json.Marshal(decision)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
