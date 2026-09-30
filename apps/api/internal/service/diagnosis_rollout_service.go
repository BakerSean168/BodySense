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

const (
	DiagnosisRolloutPolicyV1                    = "diagnosis-rollout-policy-v1"
	DiagnosisRolloutPolicyV2StructuredAuthority = "diagnosis-rollout-policy-v2-structured-authority"
	DiagnosisRolloutPolicyV3StructuredAuthority = "diagnosis-rollout-policy-v3-structured-authority"
	// DiagnosisRolloutPolicyRevision is retained as the historical/default policy alias.
	DiagnosisRolloutPolicyRevision = DiagnosisRolloutPolicyV1
)

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
	AuthorityMigrations     int     `json:"authority_migrations"`
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
		observation.Comparison = diagnosisRolloutFailureComparison(route, "shadow_execution_error")
		return s.repo.Create(ctx, observation)
	}
	if report == nil {
		observation.ShadowError = "missing shadow comparison report"
		observation.Comparison = diagnosisRolloutFailureComparison(route, "missing_shadow_comparison_report")
		return s.repo.Create(ctx, observation)
	}
	comparison, unsafeRelaxation := diagnosisRolloutClassifyAuthority(route, report)
	encoded, _ := json.Marshal(comparison)
	observation.Comparison = datatypes.JSON(encoded)
	observation.UnsafeRelaxation = unsafeRelaxation
	if diagnosisStructuredAuthorityPolicy(route.RolloutPolicyRevision) {
		_, challenger := diagnosisRolloutAuthorityDirection(route, report)
		observation.ForbiddenSideEffect = challenger.ForbiddenSideEffectsPresent
	} else {
		observation.ForbiddenSideEffect = replayComparisonCandidateTrue(report.Comparison.Hard, "forbidden_side_effects")
	}
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
	return s.SummaryForPolicy(ctx, championID, challengerID, stage, canaryBPS, limit, DiagnosisRolloutPolicyV1)
}

func (s *DiagnosisRolloutService) SummaryForPolicy(
	ctx context.Context,
	championID string,
	challengerID string,
	stage string,
	canaryBPS int,
	limit int,
	policyRevision string,
) (DiagnosisRolloutSummary, error) {
	return s.SummaryForCohort(ctx, championID, challengerID, stage, canaryBPS, limit, policyRevision, "")
}

func (s *DiagnosisRolloutService) SummaryForCohort(
	ctx context.Context,
	championID string,
	challengerID string,
	stage string,
	canaryBPS int,
	limit int,
	policyRevision string,
	promotionRecord string,
) (DiagnosisRolloutSummary, error) {
	if !validDiagnosisRolloutPolicyRevision(policyRevision) {
		return DiagnosisRolloutSummary{}, errors.New("unsupported Diagnosis rollout policy revision")
	}
	items, err := s.repo.ListRecent(ctx, championID, challengerID, stage, canaryBPS, limit)
	if err != nil {
		return DiagnosisRolloutSummary{}, err
	}
	return SummarizeDiagnosisRolloutCohort(stage, policyRevision, promotionRecord, items), nil
}

func SummarizeDiagnosisRollout(stage string, items []model.DiagnosisRolloutObservation) DiagnosisRolloutSummary {
	return SummarizeDiagnosisRolloutForPolicy(stage, DiagnosisRolloutPolicyV1, items)
}

func SummarizeDiagnosisRolloutForPolicy(stage, policyRevision string, items []model.DiagnosisRolloutObservation) DiagnosisRolloutSummary {
	return SummarizeDiagnosisRolloutCohort(stage, policyRevision, "", items)
}

func SummarizeDiagnosisRolloutCohort(stage, policyRevision, promotionRecord string, items []model.DiagnosisRolloutObservation) DiagnosisRolloutSummary {
	summary := DiagnosisRolloutSummary{PolicyRevision: policyRevision, Stage: stage}
	for _, item := range items {
		var comparison DiagnosisReplayComparison
		comparisonOK := json.Unmarshal(item.Comparison, &comparison) == nil
		itemPolicy := DiagnosisRolloutPolicyV1
		if comparisonOK && comparison.Authority.PolicyRevision != "" {
			itemPolicy = comparison.Authority.PolicyRevision
		}
		if itemPolicy != policyRevision {
			continue
		}
		if promotionRecord != "" && comparison.Authority.PromotionRecord != promotionRecord {
			continue
		}
		summary.Samples++
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
		if comparisonOK {
			if comparison.Authority.GateEquivalent {
				summary.AuthorityMigrations++
				continue
			}
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
	policyRevision := summary.PolicyRevision
	if !validDiagnosisRolloutPolicyRevision(policyRevision) {
		policyRevision = DiagnosisRolloutPolicyV1
	}
	gate := DiagnosisRolloutGate{PolicyRevision: policyRevision, Action: "continue", Reasons: []string{}}
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

func diagnosisRolloutFailureComparison(route DiagnosisRouteSelection, classification string) datatypes.JSON {
	authority := DiagnosisReplayAuthorityComparison{
		PolicyRevision:  diagnosisRolloutPolicyRevision(route),
		PromotionRecord: route.PromotionRecord,
		Classification:  classification,
		GateEquivalent:  false,
	}
	encoded, _ := json.Marshal(DiagnosisReplayComparison{Authority: authority})
	return datatypes.JSON(encoded)
}

func diagnosisRolloutPolicyRevision(route DiagnosisRouteSelection) string {
	if validDiagnosisRolloutPolicyRevision(route.RolloutPolicyRevision) {
		return route.RolloutPolicyRevision
	}
	return DiagnosisRolloutPolicyV1
}

func validDiagnosisRolloutPolicyRevision(revision string) bool {
	switch revision {
	case DiagnosisRolloutPolicyV1, DiagnosisRolloutPolicyV2StructuredAuthority, DiagnosisRolloutPolicyV3StructuredAuthority:
		return true
	default:
		return false
	}
}

func diagnosisStructuredAuthorityPolicy(revision string) bool {
	return revision == DiagnosisRolloutPolicyV2StructuredAuthority || revision == DiagnosisRolloutPolicyV3StructuredAuthority
}

func diagnosisRolloutClassifyAuthority(route DiagnosisRouteSelection, report *DiagnosisReplayReport) (DiagnosisReplayComparison, bool) {
	comparison := report.Comparison
	policyRevision := diagnosisRolloutPolicyRevision(route)
	championEvidence, challengerEvidence := diagnosisRolloutAuthorityDirection(route, report)
	championOutcome, challengerOutcome := diagnosisRolloutOutcomeDirection(route, report)
	authority := DiagnosisReplayAuthorityComparison{
		PolicyRevision:  policyRevision,
		PromotionRecord: route.PromotionRecord,
		Classification:  "non_authority_difference",
		GateEquivalent:  false,
		ReasonCodes:     []string{},
		Evidence:        report.AuthorityEvidence,
	}
	unsafe := diagnosisRestrictiveOutcome(championOutcome) && diagnosisAllowOutcome(challengerOutcome)
	if !unsafe {
		if comparison.Hard.Match {
			authority.Classification = "aligned"
		}
		comparison.Authority = authority
		return comparison, false
	}
	if !diagnosisStructuredAuthorityPolicy(policyRevision) {
		authority.Classification = "unsafe_authority_relaxation"
		authority.ReasonCodes = []string{"legacy_outcome_only_comparator"}
		comparison.Authority = authority
		return comparison, true
	}

	reasons := []string{}
	evidence := report.AuthorityEvidence
	if !report.ArtifactIntegrity.Match {
		reasons = append(reasons, "artifact_integrity_mismatch")
	}
	if !evidence.SafetyEnvelopePresent {
		reasons = append(reasons, "safety_envelope_missing")
	}
	if !evidence.CoverageComplete {
		reasons = append(reasons, "structured_safety_coverage_incomplete")
	}
	if evidence.ActiveBlockerCount != 0 {
		reasons = append(reasons, "active_structured_safety_blocker")
	}
	if evidence.RequiresReview {
		reasons = append(reasons, "structured_safety_requires_review")
	}
	if championOutcome != string(DiagnosisBlock) {
		reasons = append(reasons, "champion_outcome_not_legacy_block")
	}
	if championEvidence.DecisionPolicyRevision != DiagnosisDecisionPolicyV1 {
		reasons = append(reasons, "champion_not_decision_policy_v1")
	}
	if !championEvidence.LegacyProseGovernanceOnly {
		reasons = append(reasons, "champion_block_not_legacy_prose_governance_only")
	}
	if policyRevision == DiagnosisRolloutPolicyV2StructuredAuthority && championEvidence.LegacyProseSafetySource != LegacyProseSafetySourcePostAgent {
		reasons = append(reasons, "legacy_prose_source_not_authorized_by_policy")
	}
	if policyRevision == DiagnosisRolloutPolicyV3StructuredAuthority &&
		championEvidence.LegacyProseSafetySource != LegacyProseSafetySourcePostAgent &&
		championEvidence.LegacyProseSafetySource != LegacyProseSafetySourcePreAgent {
		reasons = append(reasons, "legacy_prose_source_not_authorized_by_policy")
	}
	if len(championEvidence.LegacyProseSafetyCategories) == 0 {
		reasons = append(reasons, "legacy_red_flag_category_missing")
	} else if !diagnosisRolloutCategoriesConfirmedAbsent(
		championEvidence.LegacyProseSafetyCategories, evidence.ConfirmedAbsentConcepts,
	) {
		reasons = append(reasons, "legacy_red_flag_category_not_confirmed_absent")
	}
	if challengerEvidence.DecisionPolicyRevision != DiagnosisDecisionPolicyV2 {
		reasons = append(reasons, "challenger_not_structured_decision_policy")
	}
	if challengerEvidence.GovernanceVerdict != "accepted" {
		reasons = append(reasons, "challenger_governance_not_accepted")
	}
	if challengerEvidence.SafetyFindingCount != 0 {
		reasons = append(reasons, "challenger_safety_findings_present")
	}
	if challengerEvidence.ForbiddenSideEffectsPresent {
		reasons = append(reasons, "challenger_forbidden_side_effect")
	}
	if len(reasons) == 0 {
		authority.Classification = "authorized_legacy_prose_false_positive_removal"
		authority.GateEquivalent = true
		authority.ReasonCodes = []string{
			"complete_structured_safety_envelope",
			"no_active_structured_safety_blocker",
			"legacy_prose_governance_only",
			"legacy_red_flag_categories_confirmed_absent",
			"structured_authority_allow",
		}
		comparison.Authority = authority
		return comparison, false
	}
	authority.Classification = "unsafe_authority_relaxation"
	authority.ReasonCodes = reasons
	comparison.Authority = authority
	return comparison, true
}

func diagnosisRolloutCategoriesConfirmedAbsent(categories, confirmedAbsent []string) bool {
	if len(categories) == 0 || len(confirmedAbsent) == 0 {
		return false
	}
	absent := map[string]struct{}{}
	for _, concept := range confirmedAbsent {
		if value := strings.TrimSpace(concept); value != "" {
			absent[value] = struct{}{}
		}
	}
	for _, category := range categories {
		value := strings.TrimSpace(category)
		if value == "" {
			return false
		}
		if _, ok := absent[value]; !ok {
			return false
		}
	}
	return true
}

func diagnosisRolloutOutcomeDirection(route DiagnosisRouteSelection, report *DiagnosisReplayReport) (string, string) {
	champion := report.Baseline.DecisionOutcome
	challenger := report.Replay.DecisionOutcome
	if route.ServedConfigurationID == route.ChallengerConfigurationID {
		champion, challenger = challenger, champion
	}
	return champion, challenger
}

func diagnosisRolloutAuthorityDirection(route DiagnosisRouteSelection, report *DiagnosisReplayReport) (DiagnosisReplayAuthorityEndpoint, DiagnosisReplayAuthorityEndpoint) {
	champion := report.AuthorityEvidence.Baseline
	challenger := report.AuthorityEvidence.Replay
	if route.ServedConfigurationID == route.ChallengerConfigurationID {
		champion, challenger = challenger, champion
	}
	return champion, challenger
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
		PolicyRevision: gate.PolicyRevision,
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
	default:
		result.Action = "pause"
		result.Reason = "unsupported_progression_stage"
		return result
	}
}
