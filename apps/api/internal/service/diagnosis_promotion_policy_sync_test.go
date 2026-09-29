package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type promotionPolicyFixture struct {
	Name                      string `json:"name"`
	ChampionConfigurationID   string `json:"champion_configuration_id"`
	ChallengerConfigurationID string `json:"challenger_configuration_id"`
	Rollout                   struct {
		PolicyRevision   string `json:"policy_revision"`
		ShadowMinSamples int    `json:"shadow_min_samples"`
		CanaryStepsBPS   []int  `json:"canary_steps_bps"`
		PromotionBPS     int    `json:"promotion_bps"`
		StopRules        struct {
			UnsafeRelaxations           int     `json:"unsafe_relaxations"`
			ForbiddenSideEffects        int     `json:"forbidden_side_effects"`
			ConfigurationMismatches     int     `json:"configuration_mismatches"`
			ChallengerErrorsBeforePause int     `json:"challenger_errors_before_pause"`
			RateGateMinSamples          int     `json:"rate_gate_min_samples"`
			MaxHardMismatchRate         float64 `json:"max_hard_mismatch_rate"`
			MaxSemanticMismatchRate     float64 `json:"max_semantic_mismatch_rate"`
		} `json:"stop_rules"`
	} `json:"rollout"`
}

func TestClaimSurfacePromotionPolicyBindsV4Successor(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy_v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Name != "diagnosis_promotion_v2" ||
		policy.ChampionConfigurationID != defaultDiagnosisConfigurationID ||
		policy.ChallengerConfigurationID != diagnosisClaimSurfaceConfigID {
		t.Fatalf("claim-surface promotion identity drifted: %#v", policy)
	}
	if len(policy.Rollout.CanaryStepsBPS) != 3 ||
		policy.Rollout.CanaryStepsBPS[0] != defaultDiagnosisCanaryBPS {
		t.Fatalf("Diagnosis default canary step drifted from v2 policy: %#v", policy.Rollout.CanaryStepsBPS)
	}
	for _, step := range policy.Rollout.CanaryStepsBPS {
		if !approvedDiagnosisCanaryStep(step) {
			t.Fatalf("Diagnosis canary step is not admitted by runtime: %#v", policy.Rollout.CanaryStepsBPS)
		}
	}
	for i, want := range []int{500, 2500, 5000} {
		if policy.Rollout.CanaryStepsBPS[i] != want {
			t.Fatalf("Diagnosis canary steps drifted: %#v", policy.Rollout.CanaryStepsBPS)
		}
	}
}

func TestNegationAwarePromotionPolicyBindsV5Successor(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy_v3.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Name != "diagnosis_promotion_v3" ||
		policy.ChampionConfigurationID != defaultDiagnosisConfigurationID ||
		policy.ChallengerConfigurationID != diagnosisNegationAwareConfigID {
		t.Fatalf("negation-aware promotion identity drifted: %#v", policy)
	}
	if len(policy.Rollout.CanaryStepsBPS) != 3 || policy.Rollout.PromotionBPS != 10000 {
		t.Fatalf("Diagnosis v3 rollout policy drifted: %#v", policy.Rollout)
	}
}

func TestNegationBridgePromotionPolicyBindsV6Successor(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy_v4.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Name != "diagnosis_promotion_v4" ||
		policy.ChampionConfigurationID != defaultDiagnosisConfigurationID ||
		policy.ChallengerConfigurationID != diagnosisNegationBridgeConfigID {
		t.Fatalf("negation-bridge promotion identity drifted: %#v", policy)
	}
	if len(policy.Rollout.CanaryStepsBPS) != 3 || policy.Rollout.PromotionBPS != 10000 {
		t.Fatalf("Diagnosis v4 rollout policy drifted: %#v", policy.Rollout)
	}
}

func TestDiagnosisPromotionRegistryMatchesImmutablePolicies(t *testing.T) {
	if got := knownDiagnosisPromotionRecords["diagnosis_promotion_v2"]; got.ChampionConfigurationID != defaultDiagnosisConfigurationID || got.ChallengerConfigurationID != diagnosisClaimSurfaceConfigID {
		t.Fatalf("v2 runtime promotion registry drifted: %#v", got)
	}
	if got := knownDiagnosisPromotionRecords["diagnosis_promotion_v3"]; got.ChampionConfigurationID != defaultDiagnosisConfigurationID || got.ChallengerConfigurationID != diagnosisNegationAwareConfigID {
		t.Fatalf("v3 runtime promotion registry drifted: %#v", got)
	}
	if got := knownDiagnosisPromotionRecords["diagnosis_promotion_v4"]; got.ChampionConfigurationID != defaultDiagnosisConfigurationID || got.ChallengerConfigurationID != diagnosisNegationBridgeConfigID {
		t.Fatalf("v4 runtime promotion registry drifted: %#v", got)
	}
}

func TestStructuredSafetyPromotionPolicyBindsV8Successor(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy_v6.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	registered := knownDiagnosisPromotionRecords[policy.Name]
	if policy.Name != "diagnosis_promotion_v6" || policy.ChampionConfigurationID != defaultDiagnosisConfigurationID || policy.ChallengerConfigurationID != diagnosisStructuredSafetyConfigID || registered.ChampionConfigurationID != policy.ChampionConfigurationID || registered.ChallengerConfigurationID != policy.ChallengerConfigurationID {
		t.Fatalf("structured safety promotion identity drifted: %#v, %#v", policy, registered)
	}
}

func TestRuntimeRolloutPolicyMatchesQualifiedPromotionPolicy(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Name != DiagnosisPromotionRecordV1 {
		t.Fatalf("promotion identity drift: %q", policy.Name)
	}
	if policy.Rollout.ShadowMinSamples != 20 || policy.Rollout.PromotionBPS != 10000 {
		t.Fatalf("runtime sample/promotion gates drifted: %#v", policy.Rollout)
	}
	wantSteps := []int{500, 2500, 5000}
	if len(policy.Rollout.CanaryStepsBPS) != len(wantSteps) {
		t.Fatalf("canary step count drift: %#v", policy.Rollout.CanaryStepsBPS)
	}
	for i := range wantSteps {
		if policy.Rollout.CanaryStepsBPS[i] != wantSteps[i] {
			t.Fatalf("canary steps drift: %#v", policy.Rollout.CanaryStepsBPS)
		}
	}
	rules := policy.Rollout.StopRules
	if rules.UnsafeRelaxations != 0 || rules.ForbiddenSideEffects != 0 || rules.ConfigurationMismatches != 0 || rules.ChallengerErrorsBeforePause != 1 || rules.RateGateMinSamples != 20 || rules.MaxHardMismatchRate != 0.10 || rules.MaxSemanticMismatchRate != 0.25 {
		t.Fatalf("runtime stop-rule constants drifted from promotion evidence: %#v", rules)
	}
}

func TestStructuredSafetyPromotionPolicyBindsV9ContextSuccessor(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy_v7.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	if diagnosisSafetyContextConfigID != "diag-config-ba10b8e6820c3691" {
		t.Fatal("v9 identity drift")
	}
	revision, err := DiagnosisDecisionPolicyRevisionForConfiguration(diagnosisSafetyContextConfigID)
	if err != nil || revision != DiagnosisDecisionPolicyV2 {
		t.Fatal("v9 decision policy drift", err)
	}
	registered := knownDiagnosisPromotionRecords[policy.Name]
	if policy.Name != "diagnosis_promotion_v7" || policy.ChampionConfigurationID != defaultDiagnosisConfigurationID || policy.ChallengerConfigurationID != diagnosisSafetyContextConfigID || registered.ChampionConfigurationID != policy.ChampionConfigurationID || registered.ChallengerConfigurationID != policy.ChallengerConfigurationID || registered.RolloutPolicyRevision != DiagnosisRolloutPolicyV1 || policy.Rollout.PolicyRevision != "" {
		t.Fatalf("structured safety promotion identity drifted: %#v, %#v", policy, registered)
	}
}

func TestStructuredSafetyPromotionPolicyV8BindsStructuredAuthorityRollout(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy_v8.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	registered := knownDiagnosisPromotionRecords[policy.Name]
	if policy.Name != "diagnosis_promotion_v8" ||
		policy.ChampionConfigurationID != defaultDiagnosisConfigurationID ||
		policy.ChallengerConfigurationID != diagnosisSafetyContextConfigID ||
		policy.Rollout.PolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority ||
		registered.RolloutPolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority ||
		registered.ChampionConfigurationID != policy.ChampionConfigurationID ||
		registered.ChallengerConfigurationID != policy.ChallengerConfigurationID {
		t.Fatalf("structured-authority rollout identity drifted: %#v, %#v", policy, registered)
	}
}

func TestStructuredSafetyPromotionPolicyV9BindsBudgetSuccessor(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy_v9.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	if diagnosisSafetyBudgetConfigID != "diag-config-3f64de162dc937ee" {
		t.Fatal("v10 identity drift")
	}
	revision, err := DiagnosisDecisionPolicyRevisionForConfiguration(diagnosisSafetyBudgetConfigID)
	if err != nil || revision != DiagnosisDecisionPolicyV2 {
		t.Fatal("v10 decision policy drift", err)
	}
	registered := knownDiagnosisPromotionRecords[policy.Name]
	if policy.Name != "diagnosis_promotion_v9" ||
		policy.ChampionConfigurationID != defaultDiagnosisConfigurationID ||
		policy.ChallengerConfigurationID != diagnosisSafetyBudgetConfigID ||
		policy.Rollout.PolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority ||
		registered.RolloutPolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority ||
		registered.ChampionConfigurationID != policy.ChampionConfigurationID ||
		registered.ChallengerConfigurationID != policy.ChallengerConfigurationID {
		t.Fatalf("budget-successor rollout identity drifted: %#v, %#v", policy, registered)
	}
}

func TestStructuredSafetyPromotionPolicyV10BindsProviderFreshCohort(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "../../../.."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "apps/ai-service/data/evals/diagnosis_promotion_policy_v10.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy promotionPolicyFixture
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	registered := knownDiagnosisPromotionRecords[policy.Name]
	if policy.Name != "diagnosis_promotion_v10" ||
		policy.ChampionConfigurationID != defaultDiagnosisConfigurationID ||
		policy.ChallengerConfigurationID != diagnosisSafetyBudgetConfigID ||
		policy.Rollout.PolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority ||
		registered.RolloutPolicyRevision != DiagnosisRolloutPolicyV2StructuredAuthority ||
		registered.ChampionConfigurationID != policy.ChampionConfigurationID ||
		registered.ChallengerConfigurationID != policy.ChallengerConfigurationID {
		t.Fatalf("provider-fresh rollout identity drifted: %#v, %#v", policy, registered)
	}
}
