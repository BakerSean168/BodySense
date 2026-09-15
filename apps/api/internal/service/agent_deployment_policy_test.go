package service

import "testing"

const (
	retiredDiagnosisV1ConfigurationID          = "diag-config-f492eb1c0c6676ae"
	retiredDiagnosisEvidenceGapConfigurationID = "diag-config-20fbfc23ca09cbab"
	retiredTreatmentV1ConfigurationID          = "treat-config-85718f8e90ac9d80"
	retiredConsultationV1ConfigurationID       = "consult-config-2bd9b46735dd693c"
	retiredPostureV1ConfigurationID            = "posture-config-3a774008db422a31"
	retiredAssessmentV1ConfigurationID         = "assess-config-fbff8155337b388d"
	retiredAssessmentV2ConfigurationID         = "assess-config-cae55474253e1601"
	retiredAssessmentV3ConfigurationID         = "assess-config-c6cfff22aa362fff"
	retiredAssessmentV4ConfigurationID         = "assess-config-e579030c2b8b540c"
)

func clearAgentDeploymentEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"DIAGNOSIS_CHAMPION_CONFIGURATION_ID",
		"DIAGNOSIS_CHALLENGER_CONFIGURATION_ID",
		"DIAGNOSIS_ROLLOUT_STAGE",
		"DIAGNOSIS_CANARY_BPS",
		"DIAGNOSIS_ROLLOUT_SALT",
		"DIAGNOSIS_PROMOTION_RECORD",
		"TREATMENT_CHAMPION_CONFIGURATION_ID",
		"TREATMENT_CHALLENGER_CONFIGURATION_ID",
		"TREATMENT_ROLLOUT_STAGE",
		"TREATMENT_CANARY_BPS",
		"TREATMENT_ROLLOUT_SALT",
		"TREATMENT_PROMOTION_RECORD",
		"ASSESSMENT_CHAMPION_CONFIGURATION_ID",
		"ASSESSMENT_CHALLENGER_CONFIGURATION_ID",
		"ASSESSMENT_ROLLOUT_STAGE",
		"ASSESSMENT_CANARY_BPS",
		"ASSESSMENT_ROLLOUT_SALT",
		"ASSESSMENT_PROMOTION_RECORD",
		"CONSULTATION_CHAMPION_CONFIGURATION_ID",
		"POSTURE_CHAMPION_CONFIGURATION_ID",
		"TITLE_CHAMPION_CONFIGURATION_ID",
		"KNOWLEDGE_CURATOR_CONFIGURATION_ID",
		"KNOWLEDGE_SPLITTER_CONFIGURATION_ID",
	} {
		t.Setenv(name, "")
	}
}

func TestAgentDeploymentPolicyDefaultsToCurrentChampionsOnly(t *testing.T) {
	clearAgentDeploymentEnv(t)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}

	diagnosis := policy.SelectDiagnosisRoute("user-1")
	if diagnosis.Stage != DiagnosisRolloutChampion ||
		diagnosis.ServedConfigurationID != defaultDiagnosisConfigurationID ||
		diagnosis.ChallengerConfigurationID != "" || diagnosis.ShadowConfigurationID != "" {
		t.Fatalf("unexpected Diagnosis route: %#v", diagnosis)
	}

	treatment := policy.SelectTreatmentRoute("user-1")
	if treatment.Stage != TreatmentRolloutChampion ||
		treatment.ServedConfigurationID != defaultTreatmentConfigurationID ||
		treatment.ChallengerConfigurationID != "" || treatment.ShadowConfigurationID != "" {
		t.Fatalf("unexpected Treatment route: %#v", treatment)
	}

	assessment := policy.SelectAssessmentRoute("user-1")
	if assessment.Stage != AssessmentRolloutChampion ||
		assessment.ServedConfigurationID != defaultAssessmentConfigurationID ||
		assessment.ShadowConfigurationID != "" {
		t.Fatalf("unexpected Assessment route: %#v", assessment)
	}
	if policy.ConsultationConfigurationID() != defaultConsultationConfigurationID {
		t.Fatalf("unexpected Consultation champion: %q", policy.ConsultationConfigurationID())
	}
	if policy.PostureConfigurationID() != defaultPostureConfigurationID {
		t.Fatalf("unexpected Posture champion: %q", policy.PostureConfigurationID())
	}
}

func TestRetiredAgentConfigurationsCannotReenterRuntimeServing(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		id   string
	}{
		{"diagnosis-v1", "DIAGNOSIS_CHAMPION_CONFIGURATION_ID", retiredDiagnosisV1ConfigurationID},
		{"diagnosis-v2", "DIAGNOSIS_CHAMPION_CONFIGURATION_ID", retiredDiagnosisEvidenceGapConfigurationID},
		{"treatment-v1", "TREATMENT_CHAMPION_CONFIGURATION_ID", retiredTreatmentV1ConfigurationID},
		{"consultation-v1", "CONSULTATION_CHAMPION_CONFIGURATION_ID", retiredConsultationV1ConfigurationID},
		{"posture-v1", "POSTURE_CHAMPION_CONFIGURATION_ID", retiredPostureV1ConfigurationID},
		{"assessment-v1", "ASSESSMENT_CHAMPION_CONFIGURATION_ID", retiredAssessmentV1ConfigurationID},
		{"assessment-v2", "ASSESSMENT_CHAMPION_CONFIGURATION_ID", retiredAssessmentV2ConfigurationID},
		{"assessment-v3", "ASSESSMENT_CHAMPION_CONFIGURATION_ID", retiredAssessmentV3ConfigurationID},
		{"assessment-v4", "ASSESSMENT_CHAMPION_CONFIGURATION_ID", retiredAssessmentV4ConfigurationID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearAgentDeploymentEnv(t)
			t.Setenv(tc.env, tc.id)
			if _, err := NewAgentDeploymentPolicy(); err == nil {
				t.Fatalf("retired configuration %q must be rejected", tc.id)
			}
		})
	}
}

func TestRetiredConfigurationResolversFailClosed(t *testing.T) {
	if _, err := DiagnosisDecisionPolicyRevisionForConfiguration(retiredDiagnosisV1ConfigurationID); err == nil {
		t.Fatal("retired Diagnosis configuration must not resolve")
	}
	if _, err := DiagnosisDecisionPolicyRevisionForConfiguration(retiredDiagnosisEvidenceGapConfigurationID); err == nil {
		t.Fatal("retired Diagnosis evidence-gap configuration must not resolve")
	}
	if _, err := TreatmentDecisionPolicyRevisionForConfiguration(retiredTreatmentV1ConfigurationID); err == nil {
		t.Fatal("retired Treatment configuration must not resolve")
	}
	if _, err := ConsultationDecisionPolicyRevisionForConfiguration(retiredConsultationV1ConfigurationID); err == nil {
		t.Fatal("retired Consultation configuration must not resolve")
	}
	if _, err := PostureDecisionPolicyRevisionForConfiguration(retiredPostureV1ConfigurationID); err == nil {
		t.Fatal("retired Posture configuration must not resolve")
	}
	for _, id := range []string{
		retiredAssessmentV1ConfigurationID,
		retiredAssessmentV2ConfigurationID,
		retiredAssessmentV3ConfigurationID,
		retiredAssessmentV4ConfigurationID,
	} {
		if _, err := AssessmentDecisionPolicyRevisionForConfiguration(id); err == nil {
			t.Fatalf("retired Assessment configuration %q must not resolve", id)
		}
	}
}

func TestRollbackIsNotARuntimeServingStage(t *testing.T) {
	for _, env := range []string{
		"DIAGNOSIS_ROLLOUT_STAGE",
		"TREATMENT_ROLLOUT_STAGE",
		"ASSESSMENT_ROLLOUT_STAGE",
	} {
		t.Run(env, func(t *testing.T) {
			clearAgentDeploymentEnv(t)
			t.Setenv(env, "rollback")
			if _, err := NewAgentDeploymentPolicy(); err == nil {
				t.Fatalf("%s=rollback must be rejected; rollback changes the champion pointer instead", env)
			}
		})
	}
}

func TestNonChampionRolloutRequiresADistinctRepositoryKnownChallenger(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stageEnv   string
		stage      string
		challenger string
	}{
		{"diagnosis", "DIAGNOSIS_ROLLOUT_STAGE", DiagnosisRolloutShadow, "DIAGNOSIS_CHALLENGER_CONFIGURATION_ID"},
		{"treatment", "TREATMENT_ROLLOUT_STAGE", TreatmentRolloutShadow, "TREATMENT_CHALLENGER_CONFIGURATION_ID"},
		{"assessment", "ASSESSMENT_ROLLOUT_STAGE", AssessmentRolloutShadow, "ASSESSMENT_CHALLENGER_CONFIGURATION_ID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearAgentDeploymentEnv(t)
			t.Setenv(tc.stageEnv, tc.stage)
			if _, err := NewAgentDeploymentPolicy(); err == nil {
				t.Fatal("non-Champion rollout must require a distinct active Challenger")
			}
		})
	}
}

func TestCurrentConfigurationResolversRemainCanonical(t *testing.T) {
	if got, err := DiagnosisDecisionPolicyRevisionForConfiguration(defaultDiagnosisConfigurationID); err != nil || got != DiagnosisDecisionPolicyV1 {
		t.Fatalf("Diagnosis policy=%q err=%v", got, err)
	}
	if got, err := TreatmentDecisionPolicyRevisionForConfiguration(defaultTreatmentConfigurationID); err != nil || got != TreatmentDecisionPolicyV1 {
		t.Fatalf("Treatment policy=%q err=%v", got, err)
	}
	if got, err := AssessmentDecisionPolicyRevisionForConfiguration(defaultAssessmentConfigurationID); err != nil || got != AssessmentDecisionPolicyV2 {
		t.Fatalf("Assessment policy=%q err=%v", got, err)
	}
	if got, err := ConsultationDecisionPolicyRevisionForConfiguration(defaultConsultationConfigurationID); err != nil || got != ConsultationDecisionPolicyV2 {
		t.Fatalf("Consultation policy=%q err=%v", got, err)
	}
	if got, err := PostureDecisionPolicyRevisionForConfiguration(defaultPostureConfigurationID); err != nil || got != PostureDecisionPolicyV1 {
		t.Fatalf("Posture policy=%q err=%v", got, err)
	}
}

func TestPostureChampionBindsPinnedGeometry(t *testing.T) {
	registration := knownPostureConfigurations[defaultPostureConfigurationID]
	if registration.MechanismRevision != "posture-geometry-v1" || registration.ModelSHA256 == "" || registration.ThresholdSHA256 == "" {
		t.Fatalf("current Posture geometry identity is incomplete: %#v", registration)
	}
}

func TestAgentDeploymentPolicyOwnsUtilityAndKnowledgeAgentPointers(t *testing.T) {
	clearAgentDeploymentEnv(t)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if policy.TitleConfigurationID() != defaultTitleConfigurationID ||
		policy.KnowledgeCuratorConfigurationID() != defaultKnowledgeCuratorConfigurationID ||
		policy.KnowledgeSplitterConfigurationID() != defaultKnowledgeSplitterConfigurationID {
		t.Fatal("utility/knowledge Agent defaults drifted")
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
			clearAgentDeploymentEnv(t)
			t.Setenv(tc.env, tc.id)
			if _, err := NewAgentDeploymentPolicy(); err == nil {
				t.Fatalf("unknown %s configuration must fail closed", tc.name)
			}
		})
	}
}
