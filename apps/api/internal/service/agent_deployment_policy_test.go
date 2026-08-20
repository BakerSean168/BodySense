package service

import (
	"strconv"
	"testing"
)

func clearDiagnosisRolloutEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DIAGNOSIS_CHAMPION_CONFIGURATION_ID", "")
	t.Setenv("DIAGNOSIS_CHALLENGER_CONFIGURATION_ID", "")
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", "")
	t.Setenv("DIAGNOSIS_CANARY_BPS", "")
	t.Setenv("DIAGNOSIS_ROLLOUT_SALT", "")
	t.Setenv("DIAGNOSIS_PROMOTION_RECORD", "")
}

func TestAgentDeploymentPolicyDefaultsToChampionWithQualifiedV3Challenger(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}
	selection := policy.SelectDiagnosisRoute("user-1")
	if selection.Stage != DiagnosisRolloutChampion || selection.ServedConfigurationID != defaultDiagnosisConfigurationID {
		t.Fatalf("unexpected default route: %#v", selection)
	}
	if selection.ChallengerConfigurationID != diagnosisDecisionAuthorityConfigID || selection.ShadowConfigurationID != "" {
		t.Fatalf("unexpected default challenger: %#v", selection)
	}
}

func TestAgentDeploymentPolicyShadowServesChampionAndPairsChallenger(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutShadow)
	t.Setenv("DIAGNOSIS_PROMOTION_RECORD", DiagnosisPromotionRecordV1)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	selection := policy.SelectDiagnosisRoute("stable-user")
	if selection.ServedConfigurationID != defaultDiagnosisConfigurationID || selection.ShadowConfigurationID != diagnosisDecisionAuthorityConfigID {
		t.Fatalf("unexpected shadow route: %#v", selection)
	}
	if selection.PromotionRecord != DiagnosisPromotionRecordV1 {
		t.Fatalf("shadow route must carry approved promotion identity: %#v", selection)
	}
}

func TestAgentDeploymentPolicyCanaryAssignmentIsStableAndPaired(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutCanary)
	t.Setenv("DIAGNOSIS_PROMOTION_RECORD", DiagnosisPromotionRecordV1)
	t.Setenv("DIAGNOSIS_CANARY_BPS", "2500")
	t.Setenv("DIAGNOSIS_ROLLOUT_SALT", "rollout-test")
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	first := policy.SelectDiagnosisRoute("user-stable")
	for i := 0; i < 10; i++ {
		if got := policy.SelectDiagnosisRoute("user-stable"); got.SubjectBucket != first.SubjectBucket || got.ServedConfigurationID != first.ServedConfigurationID {
			t.Fatalf("canary assignment changed: first=%#v got=%#v", first, got)
		}
	}
	if first.ServedConfigurationID == first.ShadowConfigurationID {
		t.Fatalf("canary comparison must pair opposite configurations: %#v", first)
	}
}

func TestAgentDeploymentPolicyCanaryDistributionTracksBasisPoints(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutCanary)
	t.Setenv("DIAGNOSIS_PROMOTION_RECORD", DiagnosisPromotionRecordV1)
	t.Setenv("DIAGNOSIS_CANARY_BPS", "1000")
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	challenger := 0
	for i := 0; i < 10000; i++ {
		if policy.SelectDiagnosisRoute("distribution-user-"+strconv.Itoa(i)).ServedConfigurationID == diagnosisDecisionAuthorityConfigID {
			challenger++
		}
	}
	if challenger < 850 || challenger > 1150 {
		t.Fatalf("10%% canary distribution unexpectedly skewed: %d/10000", challenger)
	}
}

func TestAgentDeploymentPolicyPromotedAndRollbackAreExplicit(t *testing.T) {
	for _, tc := range []struct{ stage, want string }{
		{DiagnosisRolloutPromoted, diagnosisDecisionAuthorityConfigID},
		{DiagnosisRolloutRollback, defaultDiagnosisConfigurationID},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			clearDiagnosisRolloutEnv(t)
			t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", tc.stage)
			if tc.stage == DiagnosisRolloutPromoted {
				t.Setenv("DIAGNOSIS_PROMOTION_RECORD", DiagnosisPromotionRecordV1)
			}
			policy, err := NewAgentDeploymentPolicy()
			if err != nil {
				t.Fatal(err)
			}
			selection := policy.SelectDiagnosisRoute("user")
			if selection.ServedConfigurationID != tc.want || selection.ShadowConfigurationID != "" {
				t.Fatalf("unexpected %s route: %#v", tc.stage, selection)
			}
		})
	}
}

func TestAgentDeploymentPolicyRejectsInvalidRollout(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutCanary)
	t.Setenv("DIAGNOSIS_PROMOTION_RECORD", DiagnosisPromotionRecordV1)
	t.Setenv("DIAGNOSIS_CANARY_BPS", "10000")
	if _, err := NewAgentDeploymentPolicy(); err == nil {
		t.Fatal("canary cannot silently become 100% promotion")
	}
}

func TestAgentDeploymentPolicyRefusesRolloutWithoutApprovedPromotionRecord(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutShadow)
	if _, err := NewAgentDeploymentPolicy(); err == nil {
		t.Fatal("shadow must require the CI-qualified promotion record")
	}
}

func TestDiagnosisDecisionPolicyRevisionForConfigurationRejectsUnknownCounterfactual(t *testing.T) {
	if _, err := DiagnosisDecisionPolicyRevisionForConfiguration("diag-config-not-registered"); err == nil {
		t.Fatal("counterfactual replay must not execute an unknown configuration")
	}
	got, err := DiagnosisDecisionPolicyRevisionForConfiguration(diagnosisDecisionAuthorityConfigID)
	if err != nil || got != DiagnosisDecisionPolicyV1 {
		t.Fatalf("unexpected registered decision policy: %q err=%v", got, err)
	}
}

func TestAgentDeploymentPolicyOwnsTreatmentConfigurationPointer(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("TREATMENT_AGENT_CONFIGURATION_ID", "")
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if policy.TreatmentConfigurationID() != defaultTreatmentConfigurationID {
		t.Fatalf("unexpected Treatment configuration: %q", policy.TreatmentConfigurationID())
	}
}

func TestAgentDeploymentPolicyRejectsUnknownTreatmentConfiguration(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("TREATMENT_AGENT_CONFIGURATION_ID", "treat-config-unknown")
	if _, err := NewAgentDeploymentPolicy(); err == nil {
		t.Fatal("unknown Treatment configuration must fail closed")
	}
}
