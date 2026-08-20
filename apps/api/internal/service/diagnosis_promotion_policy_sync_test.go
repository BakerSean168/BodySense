package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type promotionPolicyFixture struct {
	Name    string `json:"name"`
	Rollout struct {
		ShadowMinSamples int   `json:"shadow_min_samples"`
		CanaryStepsBPS   []int `json:"canary_steps_bps"`
		PromotionBPS     int   `json:"promotion_bps"`
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
