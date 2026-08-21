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
		Stage:                     stage,
		SubjectBucket:             42,
		CanaryBPS:                 defaultDiagnosisCanaryBPS,
		ServedConfigurationID:     "assess-config-fbff8155337b388d",
		ServedDecisionPolicyRevision: AssessmentDecisionPolicyV1,
		ShadowConfigurationID:     shadowID,
		ChampionConfigurationID:   "assess-config-fbff8155337b388d",
		ChallengerConfigurationID: shadowID,
		PromotionRecord:           "",
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

	hold := svc.Progression(&AssessmentRolloutSummary{
		Samples: 5, ShadowErrors: 1, ForbiddenSideEffects: 0, HardMismatchRate: 0,
	}, false)
	if hold.Action != "hold" {
		t.Fatalf("blocking signals must hold, got %q", hold.Action)
	}

	promote := svc.Progression(&AssessmentRolloutSummary{
		Stage: "shadow", Samples: 10, ForbiddenSideEffects: 0, ConfigurationMismatches: 0,
		HardMismatchRate: 0, SemanticMismatchRate: 0,
	}, true)
	if promote.Action != "promote" || promote.NextStage != "canary" {
		t.Fatalf("clean shadow with promotion record should promote, got %+v", promote)
	}
}
