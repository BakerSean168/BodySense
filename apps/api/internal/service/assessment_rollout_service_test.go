package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeAssessmentRolloutRepo struct {
	created []model.AssessmentRolloutObservation
}

func (r *fakeAssessmentRolloutRepo) Create(_ context.Context, observation *model.AssessmentRolloutObservation) error {
	r.created = append(r.created, *observation)
	return nil
}

func (r *fakeAssessmentRolloutRepo) ListRecent(_ context.Context, _, _ string, _ string, _ int, _ int) ([]model.AssessmentRolloutObservation, error) {
	return r.created, nil
}

func testAssessmentRoute(stage, shadowID string) AssessmentRouteSelection {
	return AssessmentRouteSelection{
		Stage:                        stage,
		SubjectBucket:                42,
		CanaryBPS:                    defaultDiagnosisCanaryBPS,
		ServedConfigurationID:        "assess-config-fbff8155337b388d",
		ServedDecisionPolicyRevision: AssessmentDecisionPolicyV1,
		ShadowConfigurationID:        shadowID,
		ChampionConfigurationID:      "assess-config-fbff8155337b388d",
		ChallengerConfigurationID:    shadowID,
		PromotionRecord:              "",
	}
}

func TestAssessmentRolloutNoChallengerSkipsObservation(t *testing.T) {
	repo := &fakeAssessmentRolloutRepo{}
	svc := NewAssessmentRolloutService(repo, nil)
	route := testAssessmentRoute("champion", "")

	if err := svc.ObserveReport(context.Background(), uuid.New(), route, uuid.New()); err != nil {
		t.Fatalf("ObserveReport must not error without a shadow: %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatalf("no challenger means no rollout observation, got %d", len(repo.created))
	}
}

func TestAssessmentRolloutRecordsShadowErrorEvidence(t *testing.T) {
	repo := &fakeAssessmentRolloutRepo{}
	svc := NewAssessmentRolloutService(repo, nil) // nil replay => shadow error evidence
	route := testAssessmentRoute("shadow", "assess-config-0000000000000000")

	if err := svc.ObserveReport(context.Background(), uuid.New(), route, uuid.New()); err != nil {
		t.Fatalf("ObserveReport with unconfigured replay must not fail: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected one observation, got %d", len(repo.created))
	}
	if repo.created[0].ShadowError == "" {
		t.Fatal("shadow replay misconfiguration must be recorded as evidence")
	}
}

func TestAssessmentRolloutProgressionDenyFirst(t *testing.T) {
	svc := NewAssessmentRolloutService(&fakeAssessmentRolloutRepo{}, nil)

	// Blocking signals => rollback, never promote.
	rollback := svc.Progression(&AssessmentRolloutSummary{
		Samples: 50, ForbiddenSideEffects: 1, ConfigurationMismatches: 0, HardMismatchRate: 0,
	}, false)
	if rollback.Action != "rollback" {
		t.Fatalf("blocking signals must rollback, got %q (%s)", rollback.Action, rollback.Reason)
	}

	// Too few samples => hold even when clean.
	holdSamples := svc.Progression(&AssessmentRolloutSummary{
		Stage: "shadow", Samples: 5, ForbiddenSideEffects: 0, ConfigurationMismatches: 0,
		HardMismatchRate: 0, SemanticMismatchRate: 0,
	}, false)
	if holdSamples.Action != "hold" {
		t.Fatalf("insufficient samples must hold, got %q", holdSamples.Action)
	}

	// Clean shadow window with enough samples => promote to canary.
	promote := svc.Progression(&AssessmentRolloutSummary{
		Stage: "shadow", Samples: AssessmentRolloutMinSamples, ForbiddenSideEffects: 0, ConfigurationMismatches: 0,
		HardMismatchRate: 0, SemanticMismatchRate: 0,
	}, false)
	if promote.Action != "promote" || promote.NextStage != "canary" {
		t.Fatalf("clean shadow window should promote to canary, got %+v", promote)
	}

	// canary requires a promotion record.
	canaryHold := svc.Progression(&AssessmentRolloutSummary{
		Stage: "canary", Samples: AssessmentRolloutMinSamples, ForbiddenSideEffects: 0, ConfigurationMismatches: 0,
		HardMismatchRate: 0, SemanticMismatchRate: 0,
	}, false)
	if canaryHold.Action != "hold" {
		t.Fatalf("canary without promotion record must hold, got %q", canaryHold.Action)
	}
	canaryPromote := svc.Progression(&AssessmentRolloutSummary{
		Stage: "canary", Samples: AssessmentRolloutMinSamples, ForbiddenSideEffects: 0, ConfigurationMismatches: 0,
		HardMismatchRate: 0, SemanticMismatchRate: 0,
	}, true)
	if canaryPromote.Action != "promote" || canaryPromote.NextStage != "promoted" {
		t.Fatalf("clean canary with promotion record should promote to promoted, got %+v", canaryPromote)
	}
}
