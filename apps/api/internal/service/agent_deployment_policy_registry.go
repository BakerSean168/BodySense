package service

import (
	"fmt"
	"strings"
)

const (
	diagnosisDecisionAuthorityConfigID = "diag-config-5a4a13627e14b4cf"
	diagnosisClaimSurfaceConfigID      = "diag-config-4a517fea19cb6c49"
	diagnosisNegationAwareConfigID     = "diag-config-375187050b203078"
	diagnosisNegationBridgeConfigID    = "diag-config-4377355ba2012ce8"
	diagnosisNegationListConfigID      = "diag-config-4eb948f419994367"
	diagnosisSafetyContextConfigID     = "diag-config-ba10b8e6820c3691"
	diagnosisSafetyBudgetConfigID      = "diag-config-3f64de162dc937ee"
	diagnosisStructuredSafetyConfigID  = "diag-config-62d312942b76a154"
	// v10 is the current serving default after the DGS-SAFE production promotion.
	// v3 remains registered explicitly as the historical rollback identity.
	defaultDiagnosisConfigurationID = diagnosisSafetyBudgetConfigID

	DiagnosisRolloutChampion = "champion"
	DiagnosisRolloutShadow   = "shadow"
	DiagnosisRolloutCanary   = "canary"
	DiagnosisRolloutPromoted = "promoted"

	defaultDiagnosisCanaryBPS   = 500
	defaultDiagnosisRolloutSalt = "diagnosis-rollout-v1"

	AssessmentRolloutChampion = "champion"
	AssessmentRolloutShadow   = "shadow"
	AssessmentRolloutCanary   = "canary"
	AssessmentRolloutPromoted = "promoted"

	defaultAssessmentCanaryBPS   = 500
	defaultAssessmentRolloutSalt = "assessment-rollout-v1"

	treatmentEvidenceGapConfigurationID = "treat-config-f68eec9846664596"
	defaultTreatmentConfigurationID     = treatmentEvidenceGapConfigurationID
	treatmentLogicalModelV1             = "bodysense-structured"

	TreatmentRolloutChampion = "champion"
	TreatmentRolloutShadow   = "shadow"
	TreatmentRolloutCanary   = "canary"
	TreatmentRolloutPromoted = "promoted"

	defaultTreatmentCanaryBPS   = 500
	defaultTreatmentRolloutSalt = "treatment-rollout-v1"

	defaultAssessmentConfigurationID = "assess-config-617534e4b17c512a"
	assessmentLogicalModelV1         = "bodysense-structured"

	defaultConsultationConfigurationID = BodyFactConsultationConfigurationID

	defaultPostureConfigurationID = "posture-config-efa3a84622818772"

	defaultKnowledgeCuratorConfigurationID  = "knowledge-curator-config-59b2868d6fbbd12a"
	defaultKnowledgeSplitterConfigurationID = "knowledge-splitter-config-b14201d581dbf854"
	knowledgeLogicalModelV1                 = "bodysense-structured"
)

type diagnosisConfigurationRegistration struct {
	DecisionPolicyRevision string
}

type diagnosisPromotionRecordRegistration struct {
	ChampionConfigurationID   string
	ChallengerConfigurationID string
	RolloutPolicyRevision     string
}

type treatmentConfigurationRegistration struct {
	DecisionPolicyRevision string
	LogicalModel           string
}

type assessmentConfigurationRegistration struct {
	DecisionPolicyRevision string
	EvidencePolicyRevision string
	LogicalModel           string
	OutputContractRevision string
}

type consultationConfigurationRegistration struct {
	DecisionPolicyRevision string
	LogicalModel           string
}

type postureConfigurationRegistration struct {
	DecisionPolicyRevision string
	LogicalModel           string
	MechanismRevision      string
	Engine                 string
	EngineVersion          string
	ModelURI               string
	ModelSHA256            string
	ThresholdRevision      string
	ThresholdSHA256        string
}

type knowledgeConfigurationRegistration struct {
	DecisionPolicyRevision string
	LogicalModel           string
}

var knownTreatmentConfigurations = map[string]treatmentConfigurationRegistration{
	treatmentEvidenceGapConfigurationID: {
		DecisionPolicyRevision: TreatmentDecisionPolicyV1,
		LogicalModel:           treatmentLogicalModelV1,
	},
}

const AssessmentDecisionPolicyV2 = "assessment-go-generation-v2"

var knownAssessmentConfigurations = map[string]assessmentConfigurationRegistration{
	defaultAssessmentConfigurationID: {
		DecisionPolicyRevision: AssessmentDecisionPolicyV2,
		EvidencePolicyRevision: assessmentEvidencePolicyV4,
		LogicalModel:           assessmentLogicalModelV1,
		OutputContractRevision: assessmentOutputContractV2,
	},
}

const ConsultationDecisionPolicyV2 = "consultation-go-runtime-v2"
const ConsultationDecisionPolicyV3 = "consultation-go-runtime-v3"
const BodyFactConsultationConfigurationID = "consult-config-83b6a1cc70477c82"

var knownConsultationConfigurations = map[string]consultationConfigurationRegistration{
	BodyFactConsultationConfigurationID: {
		DecisionPolicyRevision: ConsultationDecisionPolicyV3,
		LogicalModel:           "bodysense-consultation",
	},
	"consult-config-7feb8ca2d5bfad5a": {
		DecisionPolicyRevision: ConsultationDecisionPolicyV2,
		LogicalModel:           "bodysense-consultation",
	},
}

// PostureDecisionPolicyV1 is the deterministic fail-closed analysis policy
// revision for the Posture role.
const PostureDecisionPolicyV1 = "posture-go-analysis-v1"

var knownPostureConfigurations = map[string]postureConfigurationRegistration{
	defaultPostureConfigurationID: {
		DecisionPolicyRevision: PostureDecisionPolicyV1,
		LogicalModel:           "bodysense-posture",
		MechanismRevision:      "posture-geometry-v1",
		Engine:                 "mediapipe-tasks",
		EngineVersion:          "1.0.0",
		ModelURI:               "https://storage.googleapis.com/mediapipe-models/pose_landmarker/pose_landmarker_lite/float16/1/pose_landmarker_lite.task",
		ModelSHA256:            "59929e1d1ee95287735ddd833b19cf4ac46d29bc7afddbbf6753c459690d574a",
		ThresholdRevision:      "posture-geometry-thresholds-v1",
		ThresholdSHA256:        "588917b4a071ee1e249d3930b37769c9c9bd7a4fdebd68eb2a00bfdd13fbb140",
	},
}

const (
	KnowledgeCuratorDecisionPolicyV1  = "knowledge-curator-go-v1"
	KnowledgeSplitterDecisionPolicyV1 = "knowledge-splitter-go-v1"
)

var knownKnowledgeCuratorConfigurations = map[string]knowledgeConfigurationRegistration{
	defaultKnowledgeCuratorConfigurationID: {
		DecisionPolicyRevision: KnowledgeCuratorDecisionPolicyV1,
		LogicalModel:           knowledgeLogicalModelV1,
	},
}

var knownKnowledgeSplitterConfigurations = map[string]knowledgeConfigurationRegistration{
	defaultKnowledgeSplitterConfigurationID: {
		DecisionPolicyRevision: KnowledgeSplitterDecisionPolicyV1,
		LogicalModel:           knowledgeLogicalModelV1,
	},
}

var knownDiagnosisConfigurations = map[string]diagnosisConfigurationRegistration{
	diagnosisSafetyBudgetConfigID:  {DecisionPolicyRevision: DiagnosisDecisionPolicyV2},
	diagnosisSafetyContextConfigID: {DecisionPolicyRevision: DiagnosisDecisionPolicyV2},
	diagnosisDecisionAuthorityConfigID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyV1,
	},
	diagnosisClaimSurfaceConfigID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyV1,
	},
	diagnosisNegationAwareConfigID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyV1,
	},
	diagnosisNegationBridgeConfigID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyV1,
	},
	diagnosisNegationListConfigID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyV1,
	},
	diagnosisStructuredSafetyConfigID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyV2,
	},
}

var knownDiagnosisPromotionRecords = map[string]diagnosisPromotionRecordRegistration{
	"diagnosis_promotion_v2": {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisClaimSurfaceConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV1},
	"diagnosis_promotion_v3": {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisNegationAwareConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV1},
	"diagnosis_promotion_v4": {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisNegationBridgeConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV1},
	"diagnosis_promotion_v5": {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisNegationListConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV1},
	"diagnosis_promotion_v6": {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisStructuredSafetyConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV1},
	// v7 was exercised in staging with the historical outcome-only comparator and
	// remains immutable evidence even though its first observation exposed a false-positive migration.
	"diagnosis_promotion_v7":  {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisSafetyContextConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV1},
	"diagnosis_promotion_v8":  {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisSafetyContextConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV2StructuredAuthority},
	"diagnosis_promotion_v9":  {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisSafetyBudgetConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV2StructuredAuthority},
	"diagnosis_promotion_v10": {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisSafetyBudgetConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV2StructuredAuthority},
	"diagnosis_promotion_v11": {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisSafetyBudgetConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV3StructuredAuthority},
	"diagnosis_promotion_v12": {ChampionConfigurationID: diagnosisDecisionAuthorityConfigID, ChallengerConfigurationID: diagnosisSafetyBudgetConfigID, RolloutPolicyRevision: DiagnosisRolloutPolicyV3StructuredAuthority},
}

// DiagnosisDecisionPolicyRevisionForConfiguration resolves one immutable
// repository-known Diagnosis configuration without changing the serving pointer.
func DiagnosisDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownDiagnosisConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Diagnosis Agent configuration id %q", configurationID)
	}
	return registration.DecisionPolicyRevision, nil
}

func TreatmentDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownTreatmentConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Treatment Agent configuration id %q", configurationID)
	}
	return registration.DecisionPolicyRevision, nil
}

func AssessmentDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownAssessmentConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Assessment Agent configuration id %q", configurationID)
	}
	return registration.DecisionPolicyRevision, nil
}

func ConsultationDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownConsultationConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Consultation Agent configuration id %q", configurationID)
	}
	return registration.DecisionPolicyRevision, nil
}

// ConsultationLogicalModelForConfiguration returns the repository-authorized logical
// model for an immutable Consultation Agent configuration. It is used by the
// Go↔Python runtime handshake to fail closed before semantic output is trusted.
func ConsultationLogicalModelForConfiguration(configurationID string) (string, error) {
	registration, ok := knownConsultationConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Consultation Agent configuration id %q", configurationID)
	}
	return registration.LogicalModel, nil
}

func PostureDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownPostureConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Posture Agent configuration id %q", configurationID)
	}
	return registration.DecisionPolicyRevision, nil
}
