package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeDiagnosisRolloutRepository struct {
	items []model.DiagnosisRolloutObservation
}

func (r *fakeDiagnosisRolloutRepository) Create(_ context.Context, item *model.DiagnosisRolloutObservation) error {
	r.items = append(r.items, *item)
	return nil
}

func (r *fakeDiagnosisRolloutRepository) ListRecent(_ context.Context, championID, challengerID, stage string, canaryBPS, _ int) ([]model.DiagnosisRolloutObservation, error) {
	var result []model.DiagnosisRolloutObservation
	for _, item := range r.items {
		if item.ChampionConfigurationID == championID && item.ChallengerConfigurationID == challengerID && item.Stage == stage && item.CanaryBPS == canaryBPS {
			result = append(result, item)
		}
	}
	return result, nil
}

func rolloutTestRoute(served, shadow string) DiagnosisRouteSelection {
	return DiagnosisRouteSelection{
		Stage: DiagnosisRolloutShadow, SubjectBucket: 1234, CanaryBPS: defaultDiagnosisCanaryBPS,
		ServedConfigurationID: served, ShadowConfigurationID: shadow,
		ChampionConfigurationID:   defaultDiagnosisConfigurationID,
		ChallengerConfigurationID: diagnosisDecisionAuthorityConfigID,
	}
}

func rolloutComparisonReport(baselineOutcome, replayOutcome string, hardMatch, semanticMatch bool) *DiagnosisReplayReport {
	return &DiagnosisReplayReport{
		ArtifactIntegrity: DiagnosisReplayLayer{Match: true},
		Baseline:          DiagnosisReplaySnapshot{DecisionOutcome: baselineOutcome},
		Replay:            DiagnosisReplaySnapshot{DecisionOutcome: replayOutcome},
		Comparison: DiagnosisReplayComparison{
			Hard: DiagnosisReplayLayer{Match: hardMatch, Checks: []DiagnosisReplayCheck{
				{Name: "forbidden_side_effects", Match: true, Baseline: "false", Candidate: "false"},
			}},
			Semantic:     DiagnosisReplayLayer{Match: semanticMatch},
			Presentation: DiagnosisReplayLayer{Match: true},
		},
	}
}

func TestDiagnosisRolloutRecordsUnsafeRelaxationAgainstChampionDirection(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)
	route := rolloutTestRoute(defaultDiagnosisConfigurationID, diagnosisDecisionAuthorityConfigID)
	report := rolloutComparisonReport(string(DiagnosisBlock), string(DiagnosisAllowNormal), false, false)

	if err := svc.RecordComparison(context.Background(), route, uuid.New(), report, nil); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 1 || !repo.items[0].UnsafeRelaxation {
		t.Fatalf("expected unsafe challenger relaxation: %#v", repo.items)
	}
}

func TestDiagnosisRolloutNormalizesCanaryWhenChallengerIsServed(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)
	route := rolloutTestRoute(diagnosisDecisionAuthorityConfigID, defaultDiagnosisConfigurationID)
	route.Stage = DiagnosisRolloutCanary
	// Baseline is the served challenger; replay is the shadow champion. Challenger
	// is more conservative, so this must not be labeled an unsafe relaxation.
	report := rolloutComparisonReport(string(DiagnosisAbstain), string(DiagnosisAllowNormal), false, false)
	if err := svc.RecordComparison(context.Background(), route, uuid.New(), report, nil); err != nil {
		t.Fatal(err)
	}
	if repo.items[0].UnsafeRelaxation {
		t.Fatalf("conservative challenger must not be an unsafe relaxation: %#v", repo.items[0])
	}
}

func TestDiagnosisRolloutGatePredeclaresRollbackAndPauseRules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		summary DiagnosisRolloutSummary
		want    string
	}{
		{"green", DiagnosisRolloutSummary{Samples: 20}, "continue"},
		{"unsafe", DiagnosisRolloutSummary{Samples: 1, UnsafeRelaxations: 1}, "rollback"},
		{"forbidden", DiagnosisRolloutSummary{Samples: 1, ForbiddenSideEffects: 1}, "rollback"},
		{"identity", DiagnosisRolloutSummary{Samples: 1, ConfigurationMismatches: 1}, "rollback"},
		{"error", DiagnosisRolloutSummary{Samples: 1, ShadowErrors: 1}, "pause"},
		{"hard-rate", DiagnosisRolloutSummary{Samples: 20, HardMismatchRate: 0.11}, "pause"},
		{"semantic-rate", DiagnosisRolloutSummary{Samples: 20, SemanticMismatchRate: 0.26}, "pause"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateDiagnosisRolloutGate(tc.summary); got.Action != tc.want {
				t.Fatalf("expected %s, got %#v", tc.want, got)
			}
		})
	}
}

func TestDiagnosisRolloutShadowErrorIsDurableOperationalEvidence(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)
	route := rolloutTestRoute(defaultDiagnosisConfigurationID, diagnosisDecisionAuthorityConfigID)
	if err := svc.RecordComparison(context.Background(), route, uuid.New(), nil, errors.New("shadow timeout")); err != nil {
		t.Fatal(err)
	}
	summary, err := svc.Summary(context.Background(), defaultDiagnosisConfigurationID, diagnosisDecisionAuthorityConfigID, DiagnosisRolloutShadow, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Samples != 1 || summary.ShadowErrors != 1 || EvaluateDiagnosisRolloutGate(summary).Action != "pause" {
		t.Fatalf("unexpected shadow error summary: %#v", summary)
	}
}

func TestSummarizeDiagnosisRolloutReadsComparisonLayers(t *testing.T) {
	comparison, _ := json.Marshal(DiagnosisReplayComparison{
		Hard:         DiagnosisReplayLayer{Match: false},
		Semantic:     DiagnosisReplayLayer{Match: false},
		Presentation: DiagnosisReplayLayer{Match: true},
	})
	summary := SummarizeDiagnosisRollout(DiagnosisRolloutShadow, []model.DiagnosisRolloutObservation{
		{Comparison: datatypes.JSON(comparison)}, {Comparison: datatypes.JSON(comparison)},
	})
	if summary.HardMismatches != 2 || summary.SemanticMismatches != 2 || summary.HardMismatchRate != 1 || summary.SemanticMismatchRate != 1 {
		t.Fatalf("unexpected comparison summary: %#v", summary)
	}
}

func TestDiagnosisRolloutProgressionRequiresSamplesAndUsesPredeclaredCanarySteps(t *testing.T) {
	green := DiagnosisRolloutSummary{Samples: 20}
	if got := EvaluateDiagnosisRolloutProgression(DiagnosisRolloutShadow, 0, DiagnosisRolloutSummary{Samples: 19}); got.Action != "wait" {
		t.Fatalf("shadow must wait for minimum evidence: %#v", got)
	}
	for _, tc := range []struct {
		stage     string
		bps       int
		nextStage string
		nextBPS   int
	}{
		{DiagnosisRolloutShadow, 0, DiagnosisRolloutCanary, 500},
		{DiagnosisRolloutCanary, 500, DiagnosisRolloutCanary, 2500},
		{DiagnosisRolloutCanary, 2500, DiagnosisRolloutCanary, 5000},
		{DiagnosisRolloutCanary, 5000, DiagnosisRolloutPromoted, 10000},
	} {
		got := EvaluateDiagnosisRolloutProgression(tc.stage, tc.bps, green)
		if got.Action != "advance" || got.NextStage != tc.nextStage || got.NextCanaryBPS != tc.nextBPS {
			t.Fatalf("unexpected progression for %s/%d: %#v", tc.stage, tc.bps, got)
		}
	}
}
