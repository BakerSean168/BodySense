package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
)

type fakeConsultationRolloutRepo struct {
	created []model.ConsultationRolloutObservation
}

func (f *fakeConsultationRolloutRepo) Create(
	ctx context.Context,
	observation *model.ConsultationRolloutObservation,
) error {
	f.created = append(f.created, *observation)
	return nil
}

func (f *fakeConsultationRolloutRepo) ListByChallenger(
	ctx context.Context,
	challengerConfigurationID string,
	stage string,
	limit int,
) ([]model.ConsultationRolloutObservation, error) {
	return f.created, nil
}

func (f *fakeConsultationRolloutRepo) CountByChallenger(
	ctx context.Context,
	challengerConfigurationID string,
	stage string,
) (int64, error) {
	return int64(len(f.created)), nil
}

func TestConsultationRolloutNoChallengerSkipsObservation(t *testing.T) {
	repo := &fakeConsultationRolloutRepo{}
	svc := NewConsultationRolloutService(repo)

	run := consultationReplayTestRun()
	decision := &ConsultationRunDecision{
		RunID:                      run.ID.String(),
		SourceConfigurationID:      run.AgentConfigurationID,
		PersistedConfigurationID:   run.AgentConfigurationID,
		ConfigurationIdentityMatch: true,
		ReplayInputFrozen:          true,
	}
	if err := svc.ObserveRun(context.Background(), run, decision); err != nil {
		t.Fatalf("ObserveRun: %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatal("champion-only decision must not create a shadow observation")
	}
}

func TestConsultationRolloutObservesDistinctChallenger(t *testing.T) {
	repo := &fakeConsultationRolloutRepo{}
	svc := NewConsultationRolloutService(repo)

	run := consultationReplayTestRun()
	challenger := "consult-config-cae55474253e1601"
	decision := &ConsultationRunDecision{
		RunID:                      run.ID.String(),
		SourceConfigurationID:      challenger,
		PersistedConfigurationID:   run.AgentConfigurationID,
		ConfigurationIdentityMatch: false,
		ReplayInputFrozen:          true,
	}
	if err := svc.ObserveRun(context.Background(), run, decision); err != nil {
		t.Fatalf("ObserveRun: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(repo.created))
	}
	observation := repo.created[0]
	if observation.ChallengerConfigurationID != challenger {
		t.Fatalf("unexpected challenger: %q", observation.ChallengerConfigurationID)
	}
	if observation.ShadowError == "" {
		t.Fatal("identity mismatch must be recorded as shadow error")
	}
}

func TestConsultationRolloutProgressionDenyFirst(t *testing.T) {
	svc := NewConsultationRolloutService(&fakeConsultationRolloutRepo{})

	rollback := svc.Progression(&ConsultationRolloutSummary{
		Stage: "shadow", Samples: 50, ShadowErrors: 1, DecisionMismatches: 0,
	}, false)
	if rollback.Action != "rollback" {
		t.Fatalf("blocking signals must rollback, got %q", rollback.Action)
	}

	hold := svc.Progression(&ConsultationRolloutSummary{
		Stage: "shadow", Samples: 5, ShadowErrors: 0, DecisionMismatches: 0,
	}, false)
	if hold.Action != "hold" {
		t.Fatalf("insufficient samples must hold, got %q", hold.Action)
	}

	promote := svc.Progression(&ConsultationRolloutSummary{
		Stage: "shadow", Samples: ConsultationRolloutMinSamples, ShadowErrors: 0, DecisionMismatches: 0,
	}, false)
	if promote.Action != "promote" || promote.NextStage != "canary" {
		t.Fatalf("clean shadow window should promote to canary, got %+v", promote)
	}
}
