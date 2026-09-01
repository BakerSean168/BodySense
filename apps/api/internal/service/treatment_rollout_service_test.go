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

type fakeTreatmentRolloutRepository struct {
	items []model.TreatmentRolloutObservation
}

func (r *fakeTreatmentRolloutRepository) Create(_ context.Context, item *model.TreatmentRolloutObservation) error {
	r.items = append(r.items, *item)
	return nil
}

func (r *fakeTreatmentRolloutRepository) ListRecent(_ context.Context, championID, challengerID, stage string, canaryBPS, _ int) ([]model.TreatmentRolloutObservation, error) {
	var result []model.TreatmentRolloutObservation
	for _, item := range r.items {
		if item.ChampionConfigurationID == championID && item.ChallengerConfigurationID == challengerID && item.Stage == stage && item.CanaryBPS == canaryBPS {
			result = append(result, item)
		}
	}
	return result, nil
}

type fakeTreatmentRolloutReplay struct {
	report *TreatmentReplayReport
	err    error
	calls  int
	target string
}

func (f *fakeTreatmentRolloutReplay) CounterfactualReplay(_ context.Context, _, _ uuid.UUID, target string) (*TreatmentReplayReport, error) {
	f.calls++
	f.target = target
	return f.report, f.err
}

func treatmentRolloutTestRoute(served, shadow string) TreatmentRouteSelection {
	return TreatmentRouteSelection{
		Stage: TreatmentRolloutShadow, SubjectBucket: 1234, CanaryBPS: defaultTreatmentCanaryBPS,
		ServedConfigurationID: served, ShadowConfigurationID: shadow,
		ChampionConfigurationID:   treatmentV1ConfigurationID,
		ChallengerConfigurationID: treatmentEvidenceGapConfigurationID,
		PromotionRecord:           TreatmentPromotionRecordV1,
	}
}

func treatmentRolloutComparisonReport(sourceOutcome, replayOutcome TreatmentDecisionOutcome, hardMatch, semanticMatch bool) *TreatmentReplayReport {
	return &TreatmentReplayReport{
		SourceConfigurationID:    treatmentV1ConfigurationID,
		TargetConfigurationID:    treatmentEvidenceGapConfigurationID,
		ArtifactIntegrity:        TreatmentReplayLayer{Match: true},
		SourceGenerationDecision: TreatmentDecision{Outcome: sourceOutcome},
		GenerationDecision:       TreatmentDecision{Outcome: replayOutcome},
		Comparison: TreatmentReplayComparison{
			Hard: TreatmentReplayLayer{Match: hardMatch, Checks: []TreatmentReplayCheck{
				{Name: "proposal_only", Match: true, Candidate: "false"},
			}},
			Semantic:     TreatmentReplayLayer{Match: semanticMatch},
			Presentation: TreatmentReplayLayer{Match: true},
		},
	}
}

func TestTreatmentRolloutRecordsUnsafeRelaxationAgainstChampionDirection(t *testing.T) {
	repo := &fakeTreatmentRolloutRepository{}
	svc := NewTreatmentRolloutService(repo, nil)
	route := treatmentRolloutTestRoute(treatmentV1ConfigurationID, treatmentEvidenceGapConfigurationID)
	report := treatmentRolloutComparisonReport(TreatmentBlock, TreatmentAllowProposal, false, false)
	if err := svc.RecordComparison(context.Background(), route, uuid.New(), report, nil); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 1 || !repo.items[0].UnsafeRelaxation {
		t.Fatalf("expected unsafe Treatment challenger relaxation: %#v", repo.items)
	}
}

func TestTreatmentRolloutNormalizesCanaryWhenChallengerIsServed(t *testing.T) {
	repo := &fakeTreatmentRolloutRepository{}
	svc := NewTreatmentRolloutService(repo, nil)
	route := treatmentRolloutTestRoute(treatmentEvidenceGapConfigurationID, treatmentV1ConfigurationID)
	route.Stage = TreatmentRolloutCanary
	// Baseline/source is the served Challenger; replay is the shadow Champion.
	// A blocked Challenger against an allowing Champion is conservative, not an unsafe relaxation.
	report := treatmentRolloutComparisonReport(TreatmentBlock, TreatmentAllowProposal, false, false)
	report.SourceConfigurationID = treatmentEvidenceGapConfigurationID
	report.TargetConfigurationID = treatmentV1ConfigurationID
	if err := svc.RecordComparison(context.Background(), route, uuid.New(), report, nil); err != nil {
		t.Fatal(err)
	}
	if repo.items[0].UnsafeRelaxation {
		t.Fatalf("conservative Challenger must not be labeled an unsafe relaxation: %#v", repo.items[0])
	}
}

func TestTreatmentRolloutObserveProposalUsesOppositeConfigAndPersistsShadowError(t *testing.T) {
	repo := &fakeTreatmentRolloutRepository{}
	replay := &fakeTreatmentRolloutReplay{err: errors.New("shadow timeout")}
	svc := NewTreatmentRolloutService(repo, replay)
	route := treatmentRolloutTestRoute(treatmentV1ConfigurationID, treatmentEvidenceGapConfigurationID)
	if err := svc.ObserveProposal(context.Background(), uuid.New(), route, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if replay.calls != 1 || replay.target != treatmentEvidenceGapConfigurationID {
		t.Fatalf("unexpected shadow replay call: %#v", replay)
	}
	if len(repo.items) != 1 || repo.items[0].ShadowError == "" {
		t.Fatalf("shadow error was not durable: %#v", repo.items)
	}
}

func TestTreatmentRolloutGatePredeclaresRollbackAndPauseRules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		summary TreatmentRolloutSummary
		want    string
	}{
		{"green", TreatmentRolloutSummary{Samples: 20}, "continue"},
		{"unsafe", TreatmentRolloutSummary{Samples: 1, UnsafeRelaxations: 1}, "rollback"},
		{"forbidden", TreatmentRolloutSummary{Samples: 1, ForbiddenSideEffects: 1}, "rollback"},
		{"identity", TreatmentRolloutSummary{Samples: 1, ConfigurationMismatches: 1}, "rollback"},
		{"error", TreatmentRolloutSummary{Samples: 1, ShadowErrors: 1}, "pause"},
		{"hard-rate", TreatmentRolloutSummary{Samples: 20, HardMismatchRate: 0.11}, "pause"},
		{"semantic-rate", TreatmentRolloutSummary{Samples: 20, SemanticMismatchRate: 0.26}, "pause"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateTreatmentRolloutGate(tc.summary); got.Action != tc.want {
				t.Fatalf("expected %s, got %#v", tc.want, got)
			}
		})
	}
}

func TestSummarizeTreatmentRolloutReadsComparisonLayers(t *testing.T) {
	comparison, _ := json.Marshal(TreatmentReplayComparison{
		Hard:         TreatmentReplayLayer{Match: false},
		Semantic:     TreatmentReplayLayer{Match: false},
		Presentation: TreatmentReplayLayer{Match: true},
	})
	summary := SummarizeTreatmentRollout(TreatmentRolloutShadow, []model.TreatmentRolloutObservation{
		{Comparison: datatypes.JSON(comparison)}, {Comparison: datatypes.JSON(comparison)},
	})
	if summary.HardMismatches != 2 || summary.SemanticMismatches != 2 || summary.HardMismatchRate != 1 || summary.SemanticMismatchRate != 1 {
		t.Fatalf("unexpected Treatment comparison summary: %#v", summary)
	}
}

func TestTreatmentRolloutProgressionRequiresSamplesAndUsesPredeclaredCanarySteps(t *testing.T) {
	green := TreatmentRolloutSummary{Samples: 20}
	if got := EvaluateTreatmentRolloutProgression(TreatmentRolloutShadow, 0, TreatmentRolloutSummary{Samples: 19}); got.Action != "wait" {
		t.Fatalf("Treatment shadow must wait for minimum evidence: %#v", got)
	}
	for _, tc := range []struct {
		stage     string
		bps       int
		nextStage string
		nextBPS   int
	}{
		{TreatmentRolloutShadow, 0, TreatmentRolloutCanary, 500},
		{TreatmentRolloutCanary, 500, TreatmentRolloutCanary, 2500},
		{TreatmentRolloutCanary, 2500, TreatmentRolloutCanary, 5000},
		{TreatmentRolloutCanary, 5000, TreatmentRolloutPromoted, 10000},
	} {
		got := EvaluateTreatmentRolloutProgression(tc.stage, tc.bps, green)
		if got.Action != "advance" || got.NextStage != tc.nextStage || got.NextCanaryBPS != tc.nextBPS {
			t.Fatalf("unexpected Treatment progression for %s/%d: %#v", tc.stage, tc.bps, got)
		}
	}
}
