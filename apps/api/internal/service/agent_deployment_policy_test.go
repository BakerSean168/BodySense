package service

import (
	"strconv"
	"testing"
)

func clearDiagnosisRolloutEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DIAGNOSIS_CHAMPION_CONFIGURATION_ID", "")
	t.Setenv("DIAGNOSIS_CHALLENGER_CONFIGURATION_ID", "")
	t.Setenv("DIAGNOSIS_ROLLBACK_CONFIGURATION_ID", "")
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", "")
	t.Setenv("DIAGNOSIS_CANARY_BPS", "")
	t.Setenv("DIAGNOSIS_ROLLOUT_SALT", "")
	t.Setenv("DIAGNOSIS_PROMOTION_RECORD", "")
}

func configureHistoricalDiagnosisPromotion(t *testing.T) {
	t.Helper()
	t.Setenv("DIAGNOSIS_CHAMPION_CONFIGURATION_ID", diagnosisV1ConfigurationID)
	t.Setenv("DIAGNOSIS_CHALLENGER_CONFIGURATION_ID", diagnosisDecisionAuthorityConfigID)
	t.Setenv("DIAGNOSIS_PROMOTION_RECORD", DiagnosisPromotionRecordV1)
}

func TestAgentDeploymentPolicyDefaultsToLatestDiagnosisChampionWithoutActiveChallenger(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}
	selection := policy.SelectDiagnosisRoute("user-1")
	if selection.Stage != DiagnosisRolloutChampion ||
		selection.ServedConfigurationID != defaultDiagnosisConfigurationID ||
		selection.ChampionConfigurationID != diagnosisDecisionAuthorityConfigID {
		t.Fatalf("unexpected default route: %#v", selection)
	}
	if selection.ChallengerConfigurationID != "" || selection.ShadowConfigurationID != "" {
		t.Fatalf("new baseline must not invent an active challenger: %#v", selection)
	}
	if selection.RollbackConfigurationID != diagnosisV1ConfigurationID {
		t.Fatalf("unexpected Diagnosis rollback target: %#v", selection)
	}
}

func TestHistoricalDiagnosisPromotionPairRemainsExplicitlyAdmitted(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	configureHistoricalDiagnosisPromotion(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutShadow)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	selection := policy.SelectDiagnosisRoute("stable-user")
	if selection.ServedConfigurationID != diagnosisV1ConfigurationID ||
		selection.ShadowConfigurationID != diagnosisDecisionAuthorityConfigID {
		t.Fatalf("unexpected historical shadow route: %#v", selection)
	}
	if selection.PromotionRecord != DiagnosisPromotionRecordV1 {
		t.Fatalf("historical shadow route must carry approved promotion identity: %#v", selection)
	}
}

func TestHistoricalDiagnosisCanaryAssignmentIsStableAndPaired(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	configureHistoricalDiagnosisPromotion(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutCanary)
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

func TestHistoricalDiagnosisCanaryDistributionTracksBasisPoints(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	configureHistoricalDiagnosisPromotion(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutCanary)
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

func TestDiagnosisHistoricalPromotedAndCurrentRollbackAreExplicit(t *testing.T) {
	t.Run("historical-promoted", func(t *testing.T) {
		clearDiagnosisRolloutEnv(t)
		configureHistoricalDiagnosisPromotion(t)
		t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutPromoted)
		policy, err := NewAgentDeploymentPolicy()
		if err != nil {
			t.Fatal(err)
		}
		selection := policy.SelectDiagnosisRoute("user")
		if selection.ServedConfigurationID != diagnosisDecisionAuthorityConfigID || selection.ShadowConfigurationID != "" {
			t.Fatalf("unexpected historical promoted route: %#v", selection)
		}
	})

	t.Run("current-rollback", func(t *testing.T) {
		clearDiagnosisRolloutEnv(t)
		t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutRollback)
		policy, err := NewAgentDeploymentPolicy()
		if err != nil {
			t.Fatal(err)
		}
		selection := policy.SelectDiagnosisRoute("user")
		if selection.ServedConfigurationID != diagnosisV1ConfigurationID ||
			selection.RollbackConfigurationID != diagnosisV1ConfigurationID {
			t.Fatalf("unexpected current rollback route: %#v", selection)
		}
	})
}

func TestAgentDeploymentPolicyRejectsInvalidDiagnosisRollout(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	configureHistoricalDiagnosisPromotion(t)
	t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutCanary)
	t.Setenv("DIAGNOSIS_CANARY_BPS", "10000")
	if _, err := NewAgentDeploymentPolicy(); err == nil {
		t.Fatal("canary cannot silently become 100% promotion")
	}
}

func TestAgentDeploymentPolicyRefusesDiagnosisRolloutWithoutQualifiedPair(t *testing.T) {
	t.Run("no-active-challenger", func(t *testing.T) {
		clearDiagnosisRolloutEnv(t)
		t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutShadow)
		if _, err := NewAgentDeploymentPolicy(); err == nil {
			t.Fatal("shadow must require an active Challenger")
		}
	})

	t.Run("no-promotion-record", func(t *testing.T) {
		clearDiagnosisRolloutEnv(t)
		t.Setenv("DIAGNOSIS_CHAMPION_CONFIGURATION_ID", diagnosisV1ConfigurationID)
		t.Setenv("DIAGNOSIS_CHALLENGER_CONFIGURATION_ID", diagnosisDecisionAuthorityConfigID)
		t.Setenv("DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutShadow)
		if _, err := NewAgentDeploymentPolicy(); err == nil {
			t.Fatal("historical shadow must still require its CI-qualified promotion record")
		}
	})
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

func clearTreatmentRolloutEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TREATMENT_AGENT_CONFIGURATION_ID", "")
	t.Setenv("TREATMENT_CHAMPION_CONFIGURATION_ID", "")
	t.Setenv("TREATMENT_CHALLENGER_CONFIGURATION_ID", "")
	t.Setenv("TREATMENT_ROLLBACK_CONFIGURATION_ID", "")
	t.Setenv("TREATMENT_ROLLOUT_STAGE", "")
	t.Setenv("TREATMENT_CANARY_BPS", "")
	t.Setenv("TREATMENT_ROLLOUT_SALT", "")
	t.Setenv("TREATMENT_PROMOTION_RECORD", "")
}

func configureHistoricalTreatmentPromotion(t *testing.T) {
	t.Helper()
	t.Setenv("TREATMENT_CHAMPION_CONFIGURATION_ID", treatmentV1ConfigurationID)
	t.Setenv("TREATMENT_CHALLENGER_CONFIGURATION_ID", treatmentEvidenceGapConfigurationID)
	t.Setenv("TREATMENT_PROMOTION_RECORD", TreatmentPromotionRecordV1)
}

func TestAgentDeploymentPolicyDefaultsToLatestTreatmentChampionWithoutActiveChallenger(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	clearTreatmentRolloutEnv(t)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	selection := policy.SelectTreatmentRoute("user-1")
	if selection.Stage != TreatmentRolloutChampion ||
		selection.ServedConfigurationID != defaultTreatmentConfigurationID ||
		selection.ChampionConfigurationID != treatmentEvidenceGapConfigurationID {
		t.Fatalf("unexpected Treatment default route: %#v", selection)
	}
	if selection.ChallengerConfigurationID != "" || selection.ShadowConfigurationID != "" {
		t.Fatalf("new Treatment baseline must not invent an active Challenger: %#v", selection)
	}
	if selection.RollbackConfigurationID != treatmentV1ConfigurationID {
		t.Fatalf("unexpected Treatment rollback target: %#v", selection)
	}
}

func TestRetiredTreatmentAgentAliasCannotDowngradeChampion(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	clearTreatmentRolloutEnv(t)
	t.Setenv("TREATMENT_AGENT_CONFIGURATION_ID", treatmentV1ConfigurationID)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if policy.TreatmentConfigurationID() != defaultTreatmentConfigurationID {
		t.Fatalf("retired alias changed current Champion: %q", policy.TreatmentConfigurationID())
	}
}

func TestAgentDeploymentPolicyRejectsUnknownTreatmentChampion(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	clearTreatmentRolloutEnv(t)
	t.Setenv("TREATMENT_CHAMPION_CONFIGURATION_ID", "treat-config-unknown")
	if _, err := NewAgentDeploymentPolicy(); err == nil {
		t.Fatal("unknown Treatment Champion must fail closed")
	}
}

func TestHistoricalTreatmentPromotionPairRemainsExplicitlyAdmitted(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	clearTreatmentRolloutEnv(t)
	configureHistoricalTreatmentPromotion(t)
	t.Setenv("TREATMENT_ROLLOUT_STAGE", TreatmentRolloutShadow)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	selection := policy.SelectTreatmentRoute("stable-user")
	if selection.ServedConfigurationID != treatmentV1ConfigurationID ||
		selection.ShadowConfigurationID != treatmentEvidenceGapConfigurationID {
		t.Fatalf("unexpected Treatment historical shadow route: %#v", selection)
	}
	if selection.PromotionRecord != TreatmentPromotionRecordV1 {
		t.Fatalf("Treatment historical shadow route lost promotion identity: %#v", selection)
	}
}

func TestHistoricalTreatmentCanaryAssignmentIsStableAndOnlyAllowsApprovedSteps(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	clearTreatmentRolloutEnv(t)
	configureHistoricalTreatmentPromotion(t)
	t.Setenv("TREATMENT_ROLLOUT_STAGE", TreatmentRolloutCanary)
	t.Setenv("TREATMENT_CANARY_BPS", "2500")
	t.Setenv("TREATMENT_ROLLOUT_SALT", "treatment-rollout-test")
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	first := policy.SelectTreatmentRoute("user-stable")
	for i := 0; i < 10; i++ {
		got := policy.SelectTreatmentRoute("user-stable")
		if got.SubjectBucket != first.SubjectBucket || got.ServedConfigurationID != first.ServedConfigurationID {
			t.Fatalf("Treatment canary assignment changed: first=%#v got=%#v", first, got)
		}
	}
	if first.ServedConfigurationID == first.ShadowConfigurationID {
		t.Fatalf("Treatment canary must pair opposite configurations: %#v", first)
	}

	clearTreatmentRolloutEnv(t)
	configureHistoricalTreatmentPromotion(t)
	t.Setenv("TREATMENT_ROLLOUT_STAGE", TreatmentRolloutCanary)
	t.Setenv("TREATMENT_CANARY_BPS", "1000")
	if _, err := NewAgentDeploymentPolicy(); err == nil {
		t.Fatal("Treatment canary must reject non-policy basis-point steps")
	}
}

func TestTreatmentHistoricalPromotedAndCurrentRollbackAreExplicit(t *testing.T) {
	t.Run("historical-promoted", func(t *testing.T) {
		clearDiagnosisRolloutEnv(t)
		clearTreatmentRolloutEnv(t)
		configureHistoricalTreatmentPromotion(t)
		t.Setenv("TREATMENT_ROLLOUT_STAGE", TreatmentRolloutPromoted)
		policy, err := NewAgentDeploymentPolicy()
		if err != nil {
			t.Fatal(err)
		}
		selection := policy.SelectTreatmentRoute("user")
		if selection.ServedConfigurationID != treatmentEvidenceGapConfigurationID || selection.ShadowConfigurationID != "" {
			t.Fatalf("unexpected Treatment historical promoted route: %#v", selection)
		}
	})

	t.Run("current-rollback", func(t *testing.T) {
		clearDiagnosisRolloutEnv(t)
		clearTreatmentRolloutEnv(t)
		t.Setenv("TREATMENT_ROLLOUT_STAGE", TreatmentRolloutRollback)
		policy, err := NewAgentDeploymentPolicy()
		if err != nil {
			t.Fatal(err)
		}
		selection := policy.SelectTreatmentRoute("user")
		if selection.ServedConfigurationID != treatmentV1ConfigurationID ||
			selection.RollbackConfigurationID != treatmentV1ConfigurationID {
			t.Fatalf("unexpected Treatment current rollback route: %#v", selection)
		}
	})
}

func TestAgentDeploymentPolicyOwnsUtilityAndKnowledgeAgentPointers(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("TITLE_CHAMPION_CONFIGURATION_ID", "")
	t.Setenv("KNOWLEDGE_CURATOR_CONFIGURATION_ID", "")
	t.Setenv("KNOWLEDGE_SPLITTER_CONFIGURATION_ID", "")
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.TitleConfigurationID(); got != defaultTitleConfigurationID {
		t.Fatalf("unexpected Title configuration: %q", got)
	}
	if got := policy.KnowledgeCuratorConfigurationID(); got != defaultKnowledgeCuratorConfigurationID {
		t.Fatalf("unexpected Knowledge Curator configuration: %q", got)
	}
	if got := policy.KnowledgeSplitterConfigurationID(); got != defaultKnowledgeSplitterConfigurationID {
		t.Fatalf("unexpected Knowledge Splitter configuration: %q", got)
	}
}

func TestAgentDeploymentPolicyRejectsUnknownUtilityAndKnowledgeConfigurations(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		id   string
	}{
		{"title", "TITLE_CHAMPION_CONFIGURATION_ID", "title-config-unknown"},
		{"knowledge-curator", "KNOWLEDGE_CURATOR_CONFIGURATION_ID", "knowledge-curator-config-unknown"},
		{"knowledge-splitter", "KNOWLEDGE_SPLITTER_CONFIGURATION_ID", "knowledge-splitter-config-unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearDiagnosisRolloutEnv(t)
			t.Setenv(tc.env, tc.id)
			if _, err := NewAgentDeploymentPolicy(); err == nil {
				t.Fatalf("unknown %s configuration must fail closed", tc.name)
			}
		})
	}
}

func TestConsultationDefaultsToStateAcquisitionV2AndKeepsV1Replayable(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("CONSULTATION_CHAMPION_CONFIGURATION_ID", "")
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.ConsultationConfigurationID(); got != defaultConsultationConfigurationID {
		t.Fatalf("unexpected Consultation champion: %q", got)
	}
	decision, err := ConsultationDecisionPolicyRevisionForConfiguration(defaultConsultationConfigurationID)
	if err != nil || decision != ConsultationDecisionPolicyV2 {
		t.Fatalf("unexpected V2 decision policy: %q err=%v", decision, err)
	}
	legacyDecision, err := ConsultationDecisionPolicyRevisionForConfiguration(consultationV1ConfigurationID)
	if err != nil || legacyDecision != ConsultationDecisionPolicyV1 {
		t.Fatalf("historical V1 configuration must remain replayable: %q err=%v", legacyDecision, err)
	}
}

func TestConsultationCanPinHistoricalV1WithoutChangingDefault(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	t.Setenv("CONSULTATION_CHAMPION_CONFIGURATION_ID", consultationV1ConfigurationID)
	legacyPolicy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if legacyPolicy.ConsultationConfigurationID() != consultationV1ConfigurationID {
		t.Fatalf("explicit V1 pin not honored: %q", legacyPolicy.ConsultationConfigurationID())
	}

	clearDiagnosisRolloutEnv(t)
	t.Setenv("CONSULTATION_CHAMPION_CONFIGURATION_ID", "")
	defaultPolicy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if defaultPolicy.ConsultationConfigurationID() != defaultConsultationConfigurationID {
		t.Fatalf("default Consultation champion changed: %q", defaultPolicy.ConsultationConfigurationID())
	}
}

func clearAssessmentRolloutEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ASSESSMENT_CHAMPION_CONFIGURATION_ID", "")
	t.Setenv("ASSESSMENT_CHALLENGER_CONFIGURATION_ID", "")
	t.Setenv("ASSESSMENT_ROLLOUT_STAGE", "")
	t.Setenv("ASSESSMENT_CANARY_BPS", "")
	t.Setenv("ASSESSMENT_ROLLOUT_SALT", "")
	t.Setenv("ASSESSMENT_PROMOTION_RECORD", "")
}

func TestAssessmentRolloutDefaultsToEvidenceContractV3(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	clearAssessmentRolloutEnv(t)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	selection := policy.SelectAssessmentRoute("user-1")
	if selection.ServedConfigurationID != defaultAssessmentConfigurationID {
		t.Fatalf("unexpected Assessment serving config: %#v", selection)
	}
	if selection.ShadowConfigurationID != "" || selection.Stage != AssessmentRolloutChampion {
		t.Fatalf("unexpected default Assessment route: %#v", selection)
	}
}

func TestAssessmentShadowMayCompareHistoricalReplayOnlyContract(t *testing.T) {
	clearDiagnosisRolloutEnv(t)
	clearAssessmentRolloutEnv(t)
	t.Setenv("ASSESSMENT_CHALLENGER_CONFIGURATION_ID", historicalAssessmentV3ConfigurationID)
	t.Setenv("ASSESSMENT_ROLLOUT_STAGE", AssessmentRolloutShadow)
	t.Setenv("ASSESSMENT_PROMOTION_RECORD", AssessmentPromotionRecordV1)

	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("historical shadow should remain replayable: %v", err)
	}
	selection := policy.SelectAssessmentRoute("user-shadow")
	if selection.ServedConfigurationID != defaultAssessmentConfigurationID || selection.ShadowConfigurationID != historicalAssessmentV3ConfigurationID {
		t.Fatalf("historical contract must be shadow-only: %#v", selection)
	}
}

func TestAssessmentHistoricalContractCannotBeServed(t *testing.T) {
	for _, historicalID := range []string{
		historicalAssessmentV2ConfigurationID,
		historicalAssessmentV3ConfigurationID,
	} {
		t.Run(historicalID, func(t *testing.T) {
			clearDiagnosisRolloutEnv(t)
			clearAssessmentRolloutEnv(t)
			t.Setenv("ASSESSMENT_CHAMPION_CONFIGURATION_ID", historicalID)
			if _, err := NewAgentDeploymentPolicy(); err == nil {
				t.Fatal("historical Assessment contract must never serve as champion")
			}

			clearAssessmentRolloutEnv(t)
			t.Setenv("ASSESSMENT_CHALLENGER_CONFIGURATION_ID", historicalID)
			t.Setenv("ASSESSMENT_ROLLOUT_STAGE", AssessmentRolloutCanary)
			t.Setenv("ASSESSMENT_CANARY_BPS", "500")
			t.Setenv("ASSESSMENT_PROMOTION_RECORD", AssessmentPromotionRecordV1)
			if _, err := NewAgentDeploymentPolicy(); err == nil {
				t.Fatal("historical Assessment contract must never receive canary traffic")
			}
		})
	}
}
