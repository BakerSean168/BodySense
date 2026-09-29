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
		ChampionConfigurationID:   diagnosisV1ConfigurationID,
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
	route := rolloutTestRoute(diagnosisV1ConfigurationID, diagnosisDecisionAuthorityConfigID)
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
	route := rolloutTestRoute(diagnosisDecisionAuthorityConfigID, diagnosisV1ConfigurationID)
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
	route := rolloutTestRoute(diagnosisV1ConfigurationID, diagnosisDecisionAuthorityConfigID)
	if err := svc.RecordComparison(context.Background(), route, uuid.New(), nil, errors.New("shadow timeout")); err != nil {
		t.Fatal(err)
	}
	summary, err := svc.Summary(context.Background(), diagnosisV1ConfigurationID, diagnosisDecisionAuthorityConfigID, DiagnosisRolloutShadow, 0, 100)
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

func structuredAuthorityRoute(served, shadow string) DiagnosisRouteSelection {
	return DiagnosisRouteSelection{
		Stage:                        DiagnosisRolloutShadow,
		SubjectBucket:                260,
		CanaryBPS:                    defaultDiagnosisCanaryBPS,
		ServedConfigurationID:        served,
		ShadowConfigurationID:        shadow,
		ChampionConfigurationID:      diagnosisDecisionAuthorityConfigID,
		ChallengerConfigurationID:    diagnosisSafetyContextConfigID,
		PromotionRecord:              "diagnosis_promotion_v8",
		RolloutPolicyRevision:        DiagnosisRolloutPolicyV2StructuredAuthority,
		ServedDecisionPolicyRevision: knownDiagnosisConfigurations[served].DecisionPolicyRevision,
		ShadowDecisionPolicyRevision: knownDiagnosisConfigurations[shadow].DecisionPolicyRevision,
	}
}

func structuredAuthorityMigrationReport() *DiagnosisReplayReport {
	return &DiagnosisReplayReport{
		ArtifactIntegrity: DiagnosisReplayLayer{Match: true},
		Baseline: DiagnosisReplaySnapshot{
			Status: "safety_blocked", DecisionOutcome: string(DiagnosisBlock), CandidateCount: 0,
		},
		Replay: DiagnosisReplaySnapshot{
			Status: "completed", DecisionOutcome: string(DiagnosisAllowNormal), CandidateCount: 1,
		},
		Comparison: DiagnosisReplayComparison{
			Hard: DiagnosisReplayLayer{Match: false, Checks: []DiagnosisReplayCheck{
				{Name: "status", Match: false, Baseline: "safety_blocked", Candidate: "completed"},
				{Name: "decision_outcome", Match: false, Baseline: "block", Candidate: "allow-normal"},
				{Name: "forbidden_side_effects", Match: true, Baseline: "false", Candidate: "false"},
			}},
			Semantic:     DiagnosisReplayLayer{Match: false},
			Presentation: DiagnosisReplayLayer{Match: false},
		},
		AuthorityEvidence: DiagnosisReplayAuthorityEvidence{
			SafetyEnvelopePresent: true,
			CoverageComplete:      true,
			ActiveBlockerCount:    0,
			RequiresReview:        false,
			Baseline: DiagnosisReplayAuthorityEndpoint{
				DecisionPolicyRevision:    DiagnosisDecisionPolicyV1,
				GovernanceVerdict:         "rejected",
				LegacyProseGovernanceOnly: true,
			},
			Replay: DiagnosisReplayAuthorityEndpoint{
				DecisionPolicyRevision: DiagnosisDecisionPolicyV2,
				GovernanceVerdict:      "accepted",
				SafetyFindingCount:     0,
			},
		},
	}
}

func TestStructuredAuthorityRolloutAuthorizesProvenLegacyFalsePositiveRemoval(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)
	route := structuredAuthorityRoute(diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID)
	report := structuredAuthorityMigrationReport()

	if err := svc.RecordComparison(context.Background(), route, uuid.New(), report, nil); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected one observation, got %d", len(repo.items))
	}
	item := repo.items[0]
	if item.UnsafeRelaxation || item.ForbiddenSideEffect || item.ConfigurationMismatch {
		t.Fatalf("proven migration must not trip rollback flags: %#v", item)
	}
	var comparison DiagnosisReplayComparison
	if err := json.Unmarshal(item.Comparison, &comparison); err != nil {
		t.Fatal(err)
	}
	if comparison.Hard.Match || comparison.Semantic.Match {
		t.Fatalf("raw mismatch evidence must remain visible: %#v", comparison)
	}
	if !comparison.Authority.GateEquivalent || comparison.Authority.Classification != "authorized_legacy_prose_false_positive_removal" || comparison.Authority.PolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority || comparison.Authority.PromotionRecord != "diagnosis_promotion_v8" {
		t.Fatalf("unexpected authority classification: %#v", comparison.Authority)
	}

	summary, err := svc.SummaryForPolicy(context.Background(), diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID, DiagnosisRolloutShadow, 0, 100, DiagnosisRolloutPolicyV2StructuredAuthority)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Samples != 1 || summary.AuthorityMigrations != 1 || summary.UnsafeRelaxations != 0 || summary.HardMismatches != 0 || summary.SemanticMismatches != 0 {
		t.Fatalf("authorized migration must be gate-equivalent but auditable: %#v", summary)
	}
	if gate := EvaluateDiagnosisRolloutGate(summary); gate.Action != "continue" {
		t.Fatalf("authorized migration unexpectedly blocked gate: %#v", gate)
	}
}

func TestStructuredAuthorityRolloutStillRejectsUnprovenRelaxations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DiagnosisReplayReport)
	}{
		{"missing-envelope", func(r *DiagnosisReplayReport) { r.AuthorityEvidence.SafetyEnvelopePresent = false }},
		{"coverage-incomplete", func(r *DiagnosisReplayReport) { r.AuthorityEvidence.CoverageComplete = false }},
		{"active-blocker", func(r *DiagnosisReplayReport) { r.AuthorityEvidence.ActiveBlockerCount = 1 }},
		{"requires-review", func(r *DiagnosisReplayReport) { r.AuthorityEvidence.RequiresReview = true }},
		{"champion-not-legacy-prose-only", func(r *DiagnosisReplayReport) { r.AuthorityEvidence.Baseline.LegacyProseGovernanceOnly = false }},
		{"champion-policy-not-v1", func(r *DiagnosisReplayReport) {
			r.AuthorityEvidence.Baseline.DecisionPolicyRevision = DiagnosisDecisionPolicyV2
		}},
		{"challenger-policy-not-v2", func(r *DiagnosisReplayReport) {
			r.AuthorityEvidence.Replay.DecisionPolicyRevision = DiagnosisDecisionPolicyV1
		}},
		{"challenger-governance-rejected", func(r *DiagnosisReplayReport) { r.AuthorityEvidence.Replay.GovernanceVerdict = "rejected" }},
		{"challenger-safety-finding", func(r *DiagnosisReplayReport) { r.AuthorityEvidence.Replay.SafetyFindingCount = 1 }},
		{"challenger-forbidden-side-effect", func(r *DiagnosisReplayReport) { r.AuthorityEvidence.Replay.ForbiddenSideEffectsPresent = true }},
		{"artifact-integrity-mismatch", func(r *DiagnosisReplayReport) { r.ArtifactIntegrity.Match = false }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeDiagnosisRolloutRepository{}
			svc := NewDiagnosisRolloutService(repo)
			route := structuredAuthorityRoute(diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID)
			report := structuredAuthorityMigrationReport()
			tc.mutate(report)
			if err := svc.RecordComparison(context.Background(), route, uuid.New(), report, nil); err != nil {
				t.Fatal(err)
			}
			if len(repo.items) != 1 || !repo.items[0].UnsafeRelaxation {
				t.Fatalf("unproven relaxation must remain unsafe: %#v", repo.items)
			}
			var comparison DiagnosisReplayComparison
			if err := json.Unmarshal(repo.items[0].Comparison, &comparison); err != nil {
				t.Fatal(err)
			}
			if comparison.Authority.GateEquivalent || comparison.Authority.Classification != "unsafe_authority_relaxation" || len(comparison.Authority.ReasonCodes) == 0 {
				t.Fatalf("missing fail-closed authority evidence: %#v", comparison.Authority)
			}
		})
	}
}

func TestStructuredAuthorityPolicyKeepsHistoricalV1CohortSeparate(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	legacyComparison, _ := json.Marshal(DiagnosisReplayComparison{
		Hard: DiagnosisReplayLayer{Match: false}, Semantic: DiagnosisReplayLayer{Match: false}, Presentation: DiagnosisReplayLayer{Match: false},
	})
	repo.items = append(repo.items, model.DiagnosisRolloutObservation{
		ChampionConfigurationID:   diagnosisDecisionAuthorityConfigID,
		ChallengerConfigurationID: diagnosisSafetyContextConfigID,
		Stage:                     DiagnosisRolloutShadow,
		CanaryBPS:                 0,
		UnsafeRelaxation:          true,
		Comparison:                datatypes.JSON(legacyComparison),
	})
	svc := NewDiagnosisRolloutService(repo)
	route := structuredAuthorityRoute(diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID)
	if err := svc.RecordComparison(context.Background(), route, uuid.New(), structuredAuthorityMigrationReport(), nil); err != nil {
		t.Fatal(err)
	}

	v1, err := svc.SummaryForPolicy(context.Background(), diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID, DiagnosisRolloutShadow, 0, 100, DiagnosisRolloutPolicyV1)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := svc.SummaryForCohort(context.Background(), diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID, DiagnosisRolloutShadow, 0, 100, DiagnosisRolloutPolicyV2StructuredAuthority, "diagnosis_promotion_v8")
	if err != nil {
		t.Fatal(err)
	}
	if v1.Samples != 1 || v1.UnsafeRelaxations != 1 {
		t.Fatalf("historical v1 cohort changed: %#v", v1)
	}
	if v2.Samples != 1 || v2.UnsafeRelaxations != 0 || v2.AuthorityMigrations != 1 {
		t.Fatalf("v2 cohort was contaminated by historical observation: %#v", v2)
	}
}

func TestStructuredAuthorityClassificationNormalizesCanaryDirection(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)
	route := structuredAuthorityRoute(diagnosisSafetyContextConfigID, diagnosisDecisionAuthorityConfigID)
	route.Stage = DiagnosisRolloutCanary
	route.CanaryBPS = 500
	report := structuredAuthorityMigrationReport()
	// Counterfactual direction is now baseline=served challenger, replay=shadow champion.
	report.Baseline, report.Replay = report.Replay, report.Baseline
	report.AuthorityEvidence.Baseline, report.AuthorityEvidence.Replay = report.AuthorityEvidence.Replay, report.AuthorityEvidence.Baseline
	if err := svc.RecordComparison(context.Background(), route, uuid.New(), report, nil); err != nil {
		t.Fatal(err)
	}
	if repo.items[0].UnsafeRelaxation || repo.items[0].ForbiddenSideEffect {
		t.Fatalf("canary direction was not normalized: %#v", repo.items[0])
	}
	var comparison DiagnosisReplayComparison
	_ = json.Unmarshal(repo.items[0].Comparison, &comparison)
	if !comparison.Authority.GateEquivalent {
		t.Fatalf("canary authority migration should remain gate-equivalent: %#v", comparison.Authority)
	}
}

func TestLegacyRolloutPolicyStillTreatsBlockToAllowAsUnsafe(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)
	route := structuredAuthorityRoute(diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID)
	route.RolloutPolicyRevision = DiagnosisRolloutPolicyV1
	route.PromotionRecord = "diagnosis_promotion_v7"
	if err := svc.RecordComparison(context.Background(), route, uuid.New(), structuredAuthorityMigrationReport(), nil); err != nil {
		t.Fatal(err)
	}
	if !repo.items[0].UnsafeRelaxation {
		t.Fatal("v1 semantics must remain outcome-only and fail closed")
	}
}

func TestStructuredAuthorityShadowErrorStaysInV2PromotionCohort(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)
	route := structuredAuthorityRoute(diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID)

	if err := svc.RecordComparison(context.Background(), route, uuid.New(), nil, errors.New("provider timeout")); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected one observation, got %d", len(repo.items))
	}
	var comparison DiagnosisReplayComparison
	if err := json.Unmarshal(repo.items[0].Comparison, &comparison); err != nil {
		t.Fatal(err)
	}
	if comparison.Authority.PolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority ||
		comparison.Authority.PromotionRecord != "diagnosis_promotion_v8" ||
		comparison.Authority.Classification != "shadow_execution_error" {
		t.Fatalf("shadow failure lost cohort identity: %#v", comparison.Authority)
	}

	v2, err := svc.SummaryForCohort(
		context.Background(), diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID,
		DiagnosisRolloutShadow, 0, 100, DiagnosisRolloutPolicyV2StructuredAuthority, "diagnosis_promotion_v8",
	)
	if err != nil {
		t.Fatal(err)
	}
	if v2.Samples != 1 || v2.ShadowErrors != 1 || v2.PolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority {
		t.Fatalf("v2 shadow failure disappeared from cohort: %#v", v2)
	}
	gate := EvaluateDiagnosisRolloutGate(v2)
	if gate.Action != "pause" || gate.PolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority {
		t.Fatalf("v2 shadow failure must pause its own policy cohort: %#v", gate)
	}
	progression := EvaluateDiagnosisRolloutProgression(DiagnosisRolloutShadow, 0, v2)
	if progression.Action != "pause" || progression.PolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority {
		t.Fatalf("v2 progression lost policy identity: %#v", progression)
	}

	v1, err := svc.SummaryForPolicy(
		context.Background(), diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID,
		DiagnosisRolloutShadow, 0, 100, DiagnosisRolloutPolicyV1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Samples != 0 {
		t.Fatalf("v2 shadow failure contaminated historical v1 cohort: %#v", v1)
	}
}

func TestStructuredAuthorityMissingReportStaysInV2PromotionCohort(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)
	route := structuredAuthorityRoute(diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID)

	if err := svc.RecordComparison(context.Background(), route, uuid.New(), nil, nil); err != nil {
		t.Fatal(err)
	}
	v2, err := svc.SummaryForCohort(
		context.Background(), diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID,
		DiagnosisRolloutShadow, 0, 100, DiagnosisRolloutPolicyV2StructuredAuthority, "diagnosis_promotion_v8",
	)
	if err != nil {
		t.Fatal(err)
	}
	if v2.Samples != 1 || v2.ShadowErrors != 1 || EvaluateDiagnosisRolloutGate(v2).Action != "pause" {
		t.Fatalf("missing report must remain a v2 pause signal: %#v", v2)
	}
}

func TestStructuredAuthorityPromotionCohortsRemainSeparate(t *testing.T) {
	repo := &fakeDiagnosisRolloutRepository{}
	svc := NewDiagnosisRolloutService(repo)

	v8Route := structuredAuthorityRoute(diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID)
	if err := svc.RecordComparison(context.Background(), v8Route, uuid.New(), structuredAuthorityMigrationReport(), nil); err != nil {
		t.Fatal(err)
	}
	futureRoute := v8Route
	futureRoute.PromotionRecord = "diagnosis_promotion_future"
	if err := svc.RecordComparison(context.Background(), futureRoute, uuid.New(), structuredAuthorityMigrationReport(), nil); err != nil {
		t.Fatal(err)
	}

	v8, err := svc.SummaryForCohort(
		context.Background(), diagnosisDecisionAuthorityConfigID, diagnosisSafetyContextConfigID,
		DiagnosisRolloutShadow, 0, 100, DiagnosisRolloutPolicyV2StructuredAuthority, "diagnosis_promotion_v8",
	)
	if err != nil {
		t.Fatal(err)
	}
	if v8.Samples != 1 || v8.AuthorityMigrations != 1 {
		t.Fatalf("promotion cohort was contaminated: %#v", v8)
	}
}
