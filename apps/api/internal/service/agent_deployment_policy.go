package service

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	diagnosisV1ConfigurationID          = "diag-config-f492eb1c0c6676ae"
	diagnosisEvidenceGapConfigurationID = "diag-config-20fbfc23ca09cbab"
	diagnosisDecisionAuthorityConfigID  = "diag-config-5a4a13627e14b4cf"
	defaultDiagnosisConfigurationID     = diagnosisDecisionAuthorityConfigID
	diagnosisRollbackConfigurationID    = diagnosisV1ConfigurationID

	DiagnosisRolloutChampion = "champion"
	DiagnosisRolloutShadow   = "shadow"
	DiagnosisRolloutCanary   = "canary"
	DiagnosisRolloutPromoted = "promoted"
	DiagnosisRolloutRollback = "rollback"

	defaultDiagnosisCanaryBPS   = 1000
	defaultDiagnosisRolloutSalt = "diagnosis-rollout-v1"
	DiagnosisPromotionRecordV1  = "diagnosis_promotion_v1"

	AssessmentRolloutChampion = "champion"
	AssessmentRolloutShadow   = "shadow"
	AssessmentRolloutCanary   = "canary"
	AssessmentRolloutPromoted = "promoted"
	AssessmentRolloutRollback = "rollback"

	defaultAssessmentCanaryBPS   = 500
	defaultAssessmentRolloutSalt = "assessment-rollout-v1"
	AssessmentPromotionRecordV1  = "assessment_promotion_v1"

	treatmentV1ConfigurationID          = "treat-config-85718f8e90ac9d80"
	treatmentEvidenceGapConfigurationID = "treat-config-f68eec9846664596"
	defaultTreatmentConfigurationID     = treatmentEvidenceGapConfigurationID
	treatmentRollbackConfigurationID    = treatmentV1ConfigurationID
	treatmentLogicalModelV1             = "bodysense-structured"

	TreatmentRolloutChampion = "champion"
	TreatmentRolloutShadow   = "shadow"
	TreatmentRolloutCanary   = "canary"
	TreatmentRolloutPromoted = "promoted"
	TreatmentRolloutRollback = "rollback"

	defaultTreatmentCanaryBPS   = 500
	defaultTreatmentRolloutSalt = "treatment-rollout-v1"
	TreatmentPromotionRecordV1  = "treatment_promotion_v1"

	historicalAssessmentV1ConfigurationID = "assess-config-fbff8155337b388d"
	historicalAssessmentV2ConfigurationID = "assess-config-cae55474253e1601"
	historicalAssessmentV3ConfigurationID = "assess-config-c6cfff22aa362fff"
	historicalAssessmentV4ConfigurationID = "assess-config-e579030c2b8b540c"
	defaultAssessmentConfigurationID      = "assess-config-617534e4b17c512a"
	assessmentLogicalModelV1              = "bodysense-structured"

	consultationV1ConfigurationID      = "consult-config-2bd9b46735dd693c"
	defaultConsultationConfigurationID = "consult-config-7feb8ca2d5bfad5a"

	historicalPostureV1ConfigurationID = "posture-config-3a774008db422a31"
	defaultPostureConfigurationID      = "posture-config-efa3a84622818772"

	defaultTitleConfigurationID = "title-config-bcc5f3a39bc98200"
	titleLogicalModelV1         = "bodysense-text"

	defaultKnowledgeCuratorConfigurationID  = "knowledge-curator-config-59b2868d6fbbd12a"
	defaultKnowledgeSplitterConfigurationID = "knowledge-splitter-config-b14201d581dbf854"
	knowledgeLogicalModelV1                 = "bodysense-structured"
)

type diagnosisConfigurationRegistration struct {
	DecisionPolicyRevision string
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
	ServingAllowed         bool
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
	ServingAllowed         bool
}

type titleConfigurationRegistration struct {
	DecisionPolicyRevision string
	LogicalModel           string
}

type knowledgeConfigurationRegistration struct {
	DecisionPolicyRevision string
	LogicalModel           string
}

var knownTreatmentConfigurations = map[string]treatmentConfigurationRegistration{
	treatmentV1ConfigurationID: {
		DecisionPolicyRevision: TreatmentDecisionPolicyV1,
		LogicalModel:           treatmentLogicalModelV1,
	},
	treatmentEvidenceGapConfigurationID: {
		DecisionPolicyRevision: TreatmentDecisionPolicyV1,
		LogicalModel:           treatmentLogicalModelV1,
	},
}

// Assessment v1/v2 remain registered for immutable historical replay only.
// V2 decision authority is the first serving contract that requires exact
// evidence refs and removes model-authored health grades / pseudo scores.
const (
	AssessmentDecisionPolicyV1 = "assessment-go-generation-v1"
	AssessmentDecisionPolicyV2 = "assessment-go-generation-v2"
)

var knownAssessmentConfigurations = map[string]assessmentConfigurationRegistration{
	historicalAssessmentV1ConfigurationID: {
		DecisionPolicyRevision: AssessmentDecisionPolicyV1,
		EvidencePolicyRevision: "assessment-evidence-reuse-v1",
		LogicalModel:           assessmentLogicalModelV1,
		OutputContractRevision: "assessment-output-v1",
		ServingAllowed:         false,
	},
	historicalAssessmentV2ConfigurationID: {
		DecisionPolicyRevision: AssessmentDecisionPolicyV1,
		EvidencePolicyRevision: "assessment-evidence-reuse-v1",
		LogicalModel:           assessmentLogicalModelV1,
		OutputContractRevision: "assessment-output-v1",
		ServingAllowed:         false,
	},
	historicalAssessmentV3ConfigurationID: {
		DecisionPolicyRevision: AssessmentDecisionPolicyV2,
		EvidencePolicyRevision: assessmentEvidencePolicyV2,
		LogicalModel:           assessmentLogicalModelV1,
		OutputContractRevision: assessmentOutputContractV2,
		ServingAllowed:         false,
	},
	historicalAssessmentV4ConfigurationID: {
		DecisionPolicyRevision: AssessmentDecisionPolicyV2,
		EvidencePolicyRevision: assessmentEvidencePolicyV3,
		LogicalModel:           assessmentLogicalModelV1,
		OutputContractRevision: assessmentOutputContractV2,
		ServingAllowed:         false,
	},
	defaultAssessmentConfigurationID: {
		DecisionPolicyRevision: AssessmentDecisionPolicyV2,
		EvidencePolicyRevision: assessmentEvidencePolicyV4,
		LogicalModel:           assessmentLogicalModelV1,
		OutputContractRevision: assessmentOutputContractV2,
		ServingAllowed:         true,
	},
}

// Consultation decision policies are immutable runtime contracts. V1 remains
// registered so interrupted/replayed historical runs keep their original
// identity; V2 adds the typed state-acquisition preflight and deterministic HITL
// gate before any visible assistant prose.
const (
	ConsultationDecisionPolicyV1 = "consultation-go-runtime-v1"
	ConsultationDecisionPolicyV2 = "consultation-go-runtime-v2"
)

var knownConsultationConfigurations = map[string]consultationConfigurationRegistration{
	consultationV1ConfigurationID: {
		DecisionPolicyRevision: ConsultationDecisionPolicyV1,
		LogicalModel:           "bodysense-consultation",
	},
	defaultConsultationConfigurationID: {
		DecisionPolicyRevision: ConsultationDecisionPolicyV2,
		LogicalModel:           "bodysense-consultation",
	},
}

// PostureDecisionPolicyV1 is the deterministic fail-closed analysis policy
// revision for the Posture role.
const PostureDecisionPolicyV1 = "posture-go-analysis-v1"

var knownPostureConfigurations = map[string]postureConfigurationRegistration{
	historicalPostureV1ConfigurationID: {
		DecisionPolicyRevision: PostureDecisionPolicyV1,
		LogicalModel:           "bodysense-posture",
		ServingAllowed:         false,
	},
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
		ServingAllowed:         true,
	},
}

const TitleDecisionPolicyV1 = "title-go-generation-v1"

var knownTitleConfigurations = map[string]titleConfigurationRegistration{
	defaultTitleConfigurationID: {
		DecisionPolicyRevision: TitleDecisionPolicyV1,
		LogicalModel:           titleLogicalModelV1,
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
	diagnosisV1ConfigurationID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyPreEnvelope,
	},
	diagnosisEvidenceGapConfigurationID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyPreEnvelope,
	},
	diagnosisDecisionAuthorityConfigID: {
		DecisionPolicyRevision: DiagnosisDecisionPolicyV1,
	},
}

type DiagnosisRouteSelection struct {
	Stage                        string `json:"stage"`
	SubjectBucket                int    `json:"subject_bucket"`
	ServedConfigurationID        string `json:"served_configuration_id"`
	ServedDecisionPolicyRevision string `json:"served_decision_policy_revision"`
	ShadowConfigurationID        string `json:"shadow_configuration_id,omitempty"`
	ShadowDecisionPolicyRevision string `json:"shadow_decision_policy_revision,omitempty"`
	ChampionConfigurationID      string `json:"champion_configuration_id"`
	ChallengerConfigurationID    string `json:"challenger_configuration_id,omitempty"`
	RollbackConfigurationID      string `json:"rollback_configuration_id,omitempty"`
	CanaryBPS                    int    `json:"canary_bps"`
	PromotionRecord              string `json:"promotion_record,omitempty"`
}

type TreatmentRouteSelection struct {
	Stage                        string `json:"stage"`
	SubjectBucket                int    `json:"subject_bucket"`
	ServedConfigurationID        string `json:"served_configuration_id"`
	ServedDecisionPolicyRevision string `json:"served_decision_policy_revision"`
	ShadowConfigurationID        string `json:"shadow_configuration_id,omitempty"`
	ShadowDecisionPolicyRevision string `json:"shadow_decision_policy_revision,omitempty"`
	ChampionConfigurationID      string `json:"champion_configuration_id"`
	ChallengerConfigurationID    string `json:"challenger_configuration_id,omitempty"`
	RollbackConfigurationID      string `json:"rollback_configuration_id,omitempty"`
	CanaryBPS                    int    `json:"canary_bps"`
	PromotionRecord              string `json:"promotion_record,omitempty"`
}

type AssessmentRouteSelection struct {
	Stage                        string `json:"stage"`
	SubjectBucket                int    `json:"subject_bucket"`
	ServedConfigurationID        string `json:"served_configuration_id"`
	ServedDecisionPolicyRevision string `json:"served_decision_policy_revision"`
	ShadowConfigurationID        string `json:"shadow_configuration_id,omitempty"`
	ShadowDecisionPolicyRevision string `json:"shadow_decision_policy_revision,omitempty"`
	ChampionConfigurationID      string `json:"champion_configuration_id"`
	ChallengerConfigurationID    string `json:"challenger_configuration_id"`
	CanaryBPS                    int    `json:"canary_bps"`
	PromotionRecord              string `json:"promotion_record,omitempty"`
}

// AgentDeploymentPolicy is the Go-owned mutable pointer from business role to
// immutable repository-known Agent configurations. Rollout changes selection;
// it never mutates an Agent configuration.
type AgentDeploymentPolicy struct {
	diagnosisChampionConfigurationID   string
	diagnosisChallengerConfigurationID string
	diagnosisRollbackConfigurationID   string
	diagnosisStage                     string
	diagnosisCanaryBPS                 int
	diagnosisRolloutSalt               string
	diagnosisPromotionRecord           string

	treatmentChampionConfigurationID   string
	treatmentChallengerConfigurationID string
	treatmentRollbackConfigurationID   string
	treatmentStage                     string
	treatmentCanaryBPS                 int
	treatmentRolloutSalt               string
	treatmentPromotionRecord           string

	assessmentChampionConfigurationID   string
	assessmentChallengerConfigurationID string
	assessmentStage                     string
	assessmentCanaryBPS                 int
	assessmentRolloutSalt               string
	assessmentPromotionRecord           string

	consultationChampionConfigurationID string

	postureChampionConfigurationID   string
	titleChampionConfigurationID     string
	knowledgeCuratorConfigurationID  string
	knowledgeSplitterConfigurationID string
}

func NewAgentDeploymentPolicy() (*AgentDeploymentPolicy, error) {
	diagnosisChampion := strings.TrimSpace(os.Getenv("DIAGNOSIS_CHAMPION_CONFIGURATION_ID"))
	if diagnosisChampion == "" {
		diagnosisChampion = defaultDiagnosisConfigurationID
	}
	if err := validateDiagnosisConfigurationID(diagnosisChampion); err != nil {
		return nil, err
	}
	diagnosisRollback := strings.TrimSpace(os.Getenv("DIAGNOSIS_ROLLBACK_CONFIGURATION_ID"))
	if diagnosisRollback == "" {
		diagnosisRollback = diagnosisRollbackConfigurationID
	}
	if err := validateDiagnosisConfigurationID(diagnosisRollback); err != nil {
		return nil, err
	}
	diagnosisChallenger := strings.TrimSpace(os.Getenv("DIAGNOSIS_CHALLENGER_CONFIGURATION_ID"))
	if diagnosisChallenger != "" {
		if err := validateDiagnosisConfigurationID(diagnosisChallenger); err != nil {
			return nil, err
		}
		if diagnosisChampion == diagnosisChallenger {
			return nil, fmt.Errorf("Diagnosis champion and challenger must be different immutable configurations")
		}
	}

	diagnosisStage := strings.TrimSpace(strings.ToLower(os.Getenv("DIAGNOSIS_ROLLOUT_STAGE")))
	if diagnosisStage == "" {
		diagnosisStage = DiagnosisRolloutChampion
	}
	if !validRolloutStage(diagnosisStage) {
		return nil, fmt.Errorf("invalid Diagnosis rollout stage %q", diagnosisStage)
	}
	diagnosisCanaryBPS := defaultDiagnosisCanaryBPS
	if raw := strings.TrimSpace(os.Getenv("DIAGNOSIS_CANARY_BPS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid DIAGNOSIS_CANARY_BPS %q", raw)
		}
		diagnosisCanaryBPS = value
	}
	if diagnosisCanaryBPS < 0 || diagnosisCanaryBPS > 10000 {
		return nil, fmt.Errorf("DIAGNOSIS_CANARY_BPS must be between 0 and 10000")
	}
	if diagnosisStage == DiagnosisRolloutCanary && (diagnosisCanaryBPS <= 0 || diagnosisCanaryBPS >= 10000) {
		return nil, fmt.Errorf("canary stage requires DIAGNOSIS_CANARY_BPS between 1 and 9999")
	}
	diagnosisRolloutSalt := strings.TrimSpace(os.Getenv("DIAGNOSIS_ROLLOUT_SALT"))
	if diagnosisRolloutSalt == "" {
		diagnosisRolloutSalt = defaultDiagnosisRolloutSalt
	}
	diagnosisPromotionRecord := strings.TrimSpace(os.Getenv("DIAGNOSIS_PROMOTION_RECORD"))
	if diagnosisStage == DiagnosisRolloutShadow || diagnosisStage == DiagnosisRolloutCanary || diagnosisStage == DiagnosisRolloutPromoted {
		if diagnosisChallenger == "" {
			return nil, fmt.Errorf("Diagnosis rollout stage %q requires a distinct active challenger configuration", diagnosisStage)
		}
		// The only currently approved rollout record is the historical v1 -> v3
		// promotion used to establish the 2026-09-01 baseline. A future v4
		// challenger must introduce a new immutable promotion record.
		if diagnosisPromotionRecord != DiagnosisPromotionRecordV1 ||
			diagnosisChampion != diagnosisV1ConfigurationID ||
			diagnosisChallenger != diagnosisDecisionAuthorityConfigID {
			return nil, fmt.Errorf(
				"Diagnosis rollout stage %q has no approved promotion record for champion %q -> challenger %q",
				diagnosisStage, diagnosisChampion, diagnosisChallenger,
			)
		}
	}

	treatmentChampion := strings.TrimSpace(os.Getenv("TREATMENT_CHAMPION_CONFIGURATION_ID"))
	if treatmentChampion == "" {
		treatmentChampion = defaultTreatmentConfigurationID
	}
	if err := validateTreatmentConfigurationID(treatmentChampion); err != nil {
		return nil, err
	}
	treatmentRollback := strings.TrimSpace(os.Getenv("TREATMENT_ROLLBACK_CONFIGURATION_ID"))
	if treatmentRollback == "" {
		treatmentRollback = treatmentRollbackConfigurationID
	}
	if err := validateTreatmentConfigurationID(treatmentRollback); err != nil {
		return nil, err
	}
	treatmentChallenger := strings.TrimSpace(os.Getenv("TREATMENT_CHALLENGER_CONFIGURATION_ID"))
	if treatmentChallenger != "" {
		if err := validateTreatmentConfigurationID(treatmentChallenger); err != nil {
			return nil, err
		}
		if treatmentChampion == treatmentChallenger {
			return nil, fmt.Errorf("Treatment champion and challenger must be different immutable configurations")
		}
	}

	treatmentStage := strings.TrimSpace(strings.ToLower(os.Getenv("TREATMENT_ROLLOUT_STAGE")))
	if treatmentStage == "" {
		treatmentStage = TreatmentRolloutChampion
	}
	if !validRolloutStage(treatmentStage) {
		return nil, fmt.Errorf("invalid Treatment rollout stage %q", treatmentStage)
	}
	treatmentCanaryBPS := defaultTreatmentCanaryBPS
	if raw := strings.TrimSpace(os.Getenv("TREATMENT_CANARY_BPS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid TREATMENT_CANARY_BPS %q", raw)
		}
		treatmentCanaryBPS = value
	}
	if treatmentCanaryBPS < 0 || treatmentCanaryBPS > 10000 {
		return nil, fmt.Errorf("TREATMENT_CANARY_BPS must be between 0 and 10000")
	}
	if treatmentStage == TreatmentRolloutCanary && !approvedTreatmentCanaryStep(treatmentCanaryBPS) {
		return nil, fmt.Errorf("Treatment canary stage requires TREATMENT_CANARY_BPS to be one of 500, 2500, 5000")
	}
	treatmentRolloutSalt := strings.TrimSpace(os.Getenv("TREATMENT_ROLLOUT_SALT"))
	if treatmentRolloutSalt == "" {
		treatmentRolloutSalt = defaultTreatmentRolloutSalt
	}
	treatmentPromotionRecord := strings.TrimSpace(os.Getenv("TREATMENT_PROMOTION_RECORD"))
	if treatmentStage == TreatmentRolloutShadow || treatmentStage == TreatmentRolloutCanary || treatmentStage == TreatmentRolloutPromoted {
		if treatmentChallenger == "" {
			return nil, fmt.Errorf("Treatment rollout stage %q requires a distinct active challenger configuration", treatmentStage)
		}
		if treatmentPromotionRecord != TreatmentPromotionRecordV1 ||
			treatmentChampion != treatmentV1ConfigurationID ||
			treatmentChallenger != treatmentEvidenceGapConfigurationID {
			return nil, fmt.Errorf(
				"Treatment rollout stage %q has no approved promotion record for champion %q -> challenger %q",
				treatmentStage, treatmentChampion, treatmentChallenger,
			)
		}
	}

	assessmentChampion := strings.TrimSpace(os.Getenv("ASSESSMENT_CHAMPION_CONFIGURATION_ID"))
	if assessmentChampion == "" {
		assessmentChampion = defaultAssessmentConfigurationID
	}
	if err := validateAssessmentServingConfigurationID(assessmentChampion); err != nil {
		return nil, err
	}
	assessmentChallenger := strings.TrimSpace(os.Getenv("ASSESSMENT_CHALLENGER_CONFIGURATION_ID"))
	if assessmentChallenger == "" {
		// No challenger yet: Assessment has a single Champion configuration.
		assessmentChallenger = assessmentChampion
	}
	if err := validateAssessmentKnownConfigurationID(assessmentChallenger); err != nil {
		return nil, err
	}

	assessmentStage := strings.TrimSpace(strings.ToLower(os.Getenv("ASSESSMENT_ROLLOUT_STAGE")))
	if assessmentStage == "" {
		assessmentStage = AssessmentRolloutChampion
	}
	if !validRolloutStage(assessmentStage) {
		return nil, fmt.Errorf("invalid Assessment rollout stage %q", assessmentStage)
	}
	// A challenger is only required once a non-Champion stage is requested.
	if assessmentStage != AssessmentRolloutChampion && assessmentStage != AssessmentRolloutRollback {
		if assessmentChampion == assessmentChallenger {
			return nil, fmt.Errorf("Assessment rollout stage %q requires a distinct challenger configuration", assessmentStage)
		}
	}
	// A historical contract may be used as a read-only shadow/replay comparator,
	// but canary/promoted stages would serve the challenger and therefore require
	// a configuration explicitly marked safe for durable reports.
	if assessmentStage == AssessmentRolloutCanary || assessmentStage == AssessmentRolloutPromoted {
		if err := validateAssessmentServingConfigurationID(assessmentChallenger); err != nil {
			return nil, fmt.Errorf("Assessment rollout stage %q cannot serve challenger: %w", assessmentStage, err)
		}
	}
	assessmentCanaryBPS := defaultAssessmentCanaryBPS
	if raw := strings.TrimSpace(os.Getenv("ASSESSMENT_CANARY_BPS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSESSMENT_CANARY_BPS %q", raw)
		}
		assessmentCanaryBPS = value
	}
	if assessmentCanaryBPS < 0 || assessmentCanaryBPS > 10000 {
		return nil, fmt.Errorf("ASSESSMENT_CANARY_BPS must be between 0 and 10000")
	}
	if assessmentStage == AssessmentRolloutCanary && (assessmentCanaryBPS <= 0 || assessmentCanaryBPS >= 10000) {
		return nil, fmt.Errorf("canary stage requires ASSESSMENT_CANARY_BPS between 1 and 9999")
	}
	assessmentRolloutSalt := strings.TrimSpace(os.Getenv("ASSESSMENT_ROLLOUT_SALT"))
	if assessmentRolloutSalt == "" {
		assessmentRolloutSalt = defaultAssessmentRolloutSalt
	}
	assessmentPromotionRecord := strings.TrimSpace(os.Getenv("ASSESSMENT_PROMOTION_RECORD"))
	if assessmentStage == AssessmentRolloutShadow || assessmentStage == AssessmentRolloutCanary || assessmentStage == AssessmentRolloutPromoted {
		if assessmentPromotionRecord == "" {
			return nil, fmt.Errorf(
				"Assessment rollout stage %q requires an approved promotion record",
				assessmentStage,
			)
		}
	}

	consultationChampion := strings.TrimSpace(os.Getenv("CONSULTATION_CHAMPION_CONFIGURATION_ID"))
	if consultationChampion == "" {
		consultationChampion = defaultConsultationConfigurationID
	}
	if err := validateConsultationConfigurationID(consultationChampion); err != nil {
		return nil, err
	}

	postureChampion := strings.TrimSpace(os.Getenv("POSTURE_CHAMPION_CONFIGURATION_ID"))
	if postureChampion == "" {
		postureChampion = defaultPostureConfigurationID
	}
	if err := validatePostureConfigurationID(postureChampion); err != nil {
		return nil, err
	}

	titleChampion := strings.TrimSpace(os.Getenv("TITLE_CHAMPION_CONFIGURATION_ID"))
	if titleChampion == "" {
		titleChampion = defaultTitleConfigurationID
	}
	if err := validateTitleConfigurationID(titleChampion); err != nil {
		return nil, err
	}

	knowledgeCurator := strings.TrimSpace(os.Getenv("KNOWLEDGE_CURATOR_CONFIGURATION_ID"))
	if knowledgeCurator == "" {
		knowledgeCurator = defaultKnowledgeCuratorConfigurationID
	}
	if err := validateKnowledgeCuratorConfigurationID(knowledgeCurator); err != nil {
		return nil, err
	}
	knowledgeSplitter := strings.TrimSpace(os.Getenv("KNOWLEDGE_SPLITTER_CONFIGURATION_ID"))
	if knowledgeSplitter == "" {
		knowledgeSplitter = defaultKnowledgeSplitterConfigurationID
	}
	if err := validateKnowledgeSplitterConfigurationID(knowledgeSplitter); err != nil {
		return nil, err
	}

	return &AgentDeploymentPolicy{
		diagnosisChampionConfigurationID:    diagnosisChampion,
		diagnosisChallengerConfigurationID:  diagnosisChallenger,
		diagnosisRollbackConfigurationID:    diagnosisRollback,
		diagnosisStage:                      diagnosisStage,
		diagnosisCanaryBPS:                  diagnosisCanaryBPS,
		diagnosisRolloutSalt:                diagnosisRolloutSalt,
		diagnosisPromotionRecord:            diagnosisPromotionRecord,
		treatmentChampionConfigurationID:    treatmentChampion,
		treatmentChallengerConfigurationID:  treatmentChallenger,
		treatmentRollbackConfigurationID:    treatmentRollback,
		treatmentStage:                      treatmentStage,
		treatmentCanaryBPS:                  treatmentCanaryBPS,
		treatmentRolloutSalt:                treatmentRolloutSalt,
		treatmentPromotionRecord:            treatmentPromotionRecord,
		assessmentChampionConfigurationID:   assessmentChampion,
		assessmentChallengerConfigurationID: assessmentChallenger,
		assessmentStage:                     assessmentStage,
		assessmentCanaryBPS:                 assessmentCanaryBPS,
		assessmentRolloutSalt:               assessmentRolloutSalt,
		assessmentPromotionRecord:           assessmentPromotionRecord,
		consultationChampionConfigurationID: consultationChampion,
		postureChampionConfigurationID:      postureChampion,
		titleChampionConfigurationID:        titleChampion,
		knowledgeCuratorConfigurationID:     knowledgeCurator,
		knowledgeSplitterConfigurationID:    knowledgeSplitter,
	}, nil
}

// DiagnosisConfigurationID preserves the pre-rollout compatibility accessor. It
// is the champion pointer, not a per-user route decision.
func (p *AgentDeploymentPolicy) DiagnosisConfigurationID() string {
	return p.diagnosisChampionConfigurationID
}

func (p *AgentDeploymentPolicy) DiagnosisDecisionPolicyRevision() string {
	return knownDiagnosisConfigurations[p.diagnosisChampionConfigurationID].DecisionPolicyRevision
}

func (p *AgentDeploymentPolicy) DiagnosisRolloutStage() string { return p.diagnosisStage }

// TreatmentConfigurationID returns the current repository Champion.
// The retired TREATMENT_AGENT_CONFIGURATION_ID alias is intentionally ignored;
// rollback now has its own explicit configuration pointer.
func (p *AgentDeploymentPolicy) TreatmentConfigurationID() string {
	return p.treatmentChampionConfigurationID
}

func (p *AgentDeploymentPolicy) TreatmentRolloutStage() string { return p.treatmentStage }

func (p *AgentDeploymentPolicy) SelectDiagnosisRoute(subjectID string) DiagnosisRouteSelection {
	bucket := stableRolloutBucket(p.diagnosisRolloutSalt, subjectID)
	served := p.diagnosisChampionConfigurationID
	shadow := ""

	switch p.diagnosisStage {
	case DiagnosisRolloutShadow:
		shadow = p.diagnosisChallengerConfigurationID
	case DiagnosisRolloutCanary:
		if bucket < p.diagnosisCanaryBPS {
			served = p.diagnosisChallengerConfigurationID
			shadow = p.diagnosisChampionConfigurationID
		} else {
			shadow = p.diagnosisChallengerConfigurationID
		}
	case DiagnosisRolloutPromoted:
		served = p.diagnosisChallengerConfigurationID
	case DiagnosisRolloutRollback:
		served = p.diagnosisRollbackConfigurationID
	}

	selection := DiagnosisRouteSelection{
		Stage: stageOrChampion(p.diagnosisStage), SubjectBucket: bucket,
		ServedConfigurationID:        served,
		ServedDecisionPolicyRevision: knownDiagnosisConfigurations[served].DecisionPolicyRevision,
		ShadowConfigurationID:        shadow,
		ChampionConfigurationID:      p.diagnosisChampionConfigurationID,
		ChallengerConfigurationID:    p.diagnosisChallengerConfigurationID,
		RollbackConfigurationID:      p.diagnosisRollbackConfigurationID,
		CanaryBPS:                    p.diagnosisCanaryBPS,
		PromotionRecord:              p.diagnosisPromotionRecord,
	}
	if shadow != "" {
		selection.ShadowDecisionPolicyRevision = knownDiagnosisConfigurations[shadow].DecisionPolicyRevision
	}
	return selection
}

func (p *AgentDeploymentPolicy) SelectTreatmentRoute(subjectID string) TreatmentRouteSelection {
	bucket := stableRolloutBucket(p.treatmentRolloutSalt, subjectID)
	served := p.treatmentChampionConfigurationID
	shadow := ""

	switch p.treatmentStage {
	case TreatmentRolloutShadow:
		shadow = p.treatmentChallengerConfigurationID
	case TreatmentRolloutCanary:
		if bucket < p.treatmentCanaryBPS {
			served = p.treatmentChallengerConfigurationID
			shadow = p.treatmentChampionConfigurationID
		} else {
			shadow = p.treatmentChallengerConfigurationID
		}
	case TreatmentRolloutPromoted:
		served = p.treatmentChallengerConfigurationID
	case TreatmentRolloutRollback:
		served = p.treatmentRollbackConfigurationID
	}

	selection := TreatmentRouteSelection{
		Stage: stageOrChampion(p.treatmentStage), SubjectBucket: bucket,
		ServedConfigurationID:        served,
		ServedDecisionPolicyRevision: knownTreatmentConfigurations[served].DecisionPolicyRevision,
		ShadowConfigurationID:        shadow,
		ChampionConfigurationID:      p.treatmentChampionConfigurationID,
		ChallengerConfigurationID:    p.treatmentChallengerConfigurationID,
		RollbackConfigurationID:      p.treatmentRollbackConfigurationID,
		CanaryBPS:                    p.treatmentCanaryBPS,
		PromotionRecord:              p.treatmentPromotionRecord,
	}
	if shadow != "" {
		selection.ShadowDecisionPolicyRevision = knownTreatmentConfigurations[shadow].DecisionPolicyRevision
	}
	return selection
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

func validateDiagnosisConfigurationID(id string) error {
	if !strings.HasPrefix(id, "diag-config-") {
		return fmt.Errorf("invalid Diagnosis Agent configuration id %q", id)
	}
	if _, ok := knownDiagnosisConfigurations[id]; !ok {
		return fmt.Errorf("unknown Diagnosis Agent configuration id %q", id)
	}
	return nil
}

func validateTreatmentConfigurationID(id string) error {
	if !strings.HasPrefix(id, "treat-config-") {
		return fmt.Errorf("invalid Treatment Agent configuration id %q", id)
	}
	if _, ok := knownTreatmentConfigurations[id]; !ok {
		return fmt.Errorf("unknown Treatment Agent configuration id %q", id)
	}
	return nil
}

// AssessmentConfigurationID returns the champion Assessment configuration
// pointer (the stable pre-rollout compatibility accessor).
func (p *AgentDeploymentPolicy) AssessmentConfigurationID() string {
	return p.assessmentChampionConfigurationID
}

func (p *AgentDeploymentPolicy) AssessmentDecisionPolicyRevision() string {
	return knownAssessmentConfigurations[p.assessmentChampionConfigurationID].DecisionPolicyRevision
}

func (p *AgentDeploymentPolicy) AssessmentRolloutStage() string { return p.assessmentStage }

func (p *AgentDeploymentPolicy) SelectAssessmentRoute(subjectID string) AssessmentRouteSelection {
	bucket := stableRolloutBucket(p.assessmentRolloutSalt, subjectID)
	served := p.assessmentChampionConfigurationID
	shadow := ""

	switch p.assessmentStage {
	case DiagnosisRolloutShadow:
		shadow = p.assessmentChallengerConfigurationID
	case DiagnosisRolloutCanary:
		if bucket < p.assessmentCanaryBPS {
			served = p.assessmentChallengerConfigurationID
			shadow = p.assessmentChampionConfigurationID
		} else {
			shadow = p.assessmentChallengerConfigurationID
		}
	case DiagnosisRolloutPromoted:
		served = p.assessmentChallengerConfigurationID
	case DiagnosisRolloutRollback:
		served = p.assessmentChampionConfigurationID
	}

	selection := AssessmentRouteSelection{
		Stage: stageOrChampion(p.assessmentStage), SubjectBucket: bucket,
		ServedConfigurationID:        served,
		ServedDecisionPolicyRevision: knownAssessmentConfigurations[served].DecisionPolicyRevision,
		ShadowConfigurationID:        shadow,
		ChampionConfigurationID:      p.assessmentChampionConfigurationID,
		ChallengerConfigurationID:    p.assessmentChallengerConfigurationID,
		CanaryBPS:                    p.assessmentCanaryBPS,
		PromotionRecord:              p.assessmentPromotionRecord,
	}
	if shadow != "" {
		selection.ShadowDecisionPolicyRevision = knownAssessmentConfigurations[shadow].DecisionPolicyRevision
	}
	return selection
}

func AssessmentDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownAssessmentConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Assessment Agent configuration id %q", configurationID)
	}
	return registration.DecisionPolicyRevision, nil
}

func validateAssessmentKnownConfigurationID(id string) error {
	if !strings.HasPrefix(id, "assess-config-") {
		return fmt.Errorf("invalid Assessment Agent configuration id %q", id)
	}
	if _, ok := knownAssessmentConfigurations[id]; !ok {
		return fmt.Errorf("unknown Assessment Agent configuration id %q", id)
	}
	return nil
}

func validateAssessmentServingConfigurationID(id string) error {
	if err := validateAssessmentKnownConfigurationID(id); err != nil {
		return err
	}
	if !knownAssessmentConfigurations[id].ServingAllowed {
		return fmt.Errorf(
			"Assessment Agent configuration id %q is historical replay-only and cannot serve durable reports",
			id,
		)
	}
	return nil
}

// ConsultationConfigurationID returns the champion Consultation configuration
// pointer (the stable pre-rollout compatibility accessor).
func (p *AgentDeploymentPolicy) ConsultationConfigurationID() string {
	return p.consultationChampionConfigurationID
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

func validateConsultationConfigurationID(id string) error {
	if !strings.HasPrefix(id, "consult-config-") {
		return fmt.Errorf("invalid Consultation Agent configuration id %q", id)
	}
	if _, ok := knownConsultationConfigurations[id]; !ok {
		return fmt.Errorf("unknown Consultation Agent configuration id %q", id)
	}
	return nil
}

// PostureConfigurationID returns the champion Posture configuration pointer.
func (p *AgentDeploymentPolicy) PostureConfigurationID() string {
	return p.postureChampionConfigurationID
}

func PostureDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownPostureConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Posture Agent configuration id %q", configurationID)
	}
	return registration.DecisionPolicyRevision, nil
}

func validatePostureKnownConfigurationID(id string) error {
	if !strings.HasPrefix(id, "posture-config-") {
		return fmt.Errorf("invalid Posture Agent configuration id %q", id)
	}
	if _, ok := knownPostureConfigurations[id]; !ok {
		return fmt.Errorf("unknown Posture Agent configuration id %q", id)
	}
	return nil
}

func validatePostureConfigurationID(id string) error {
	if err := validatePostureKnownConfigurationID(id); err != nil {
		return err
	}
	if !knownPostureConfigurations[id].ServingAllowed {
		return fmt.Errorf(
			"Posture Agent configuration id %q is historical and cannot serve new analyses",
			id,
		)
	}
	return nil
}

func (p *AgentDeploymentPolicy) TitleConfigurationID() string {
	return p.titleChampionConfigurationID
}

func TitleDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownTitleConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Title Agent configuration id %q", configurationID)
	}
	return registration.DecisionPolicyRevision, nil
}

func validateTitleConfigurationID(id string) error {
	if !strings.HasPrefix(id, "title-config-") {
		return fmt.Errorf("invalid Title Agent configuration id %q", id)
	}
	if _, ok := knownTitleConfigurations[id]; !ok {
		return fmt.Errorf("unknown Title Agent configuration id %q", id)
	}
	return nil
}

func (p *AgentDeploymentPolicy) KnowledgeCuratorConfigurationID() string {
	return p.knowledgeCuratorConfigurationID
}

func (p *AgentDeploymentPolicy) KnowledgeSplitterConfigurationID() string {
	return p.knowledgeSplitterConfigurationID
}

func (p *AgentDeploymentPolicy) KnowledgeCuratorDecisionPolicyRevision() string {
	return knownKnowledgeCuratorConfigurations[p.knowledgeCuratorConfigurationID].DecisionPolicyRevision
}

func (p *AgentDeploymentPolicy) KnowledgeCuratorLogicalModel() string {
	return knownKnowledgeCuratorConfigurations[p.knowledgeCuratorConfigurationID].LogicalModel
}

func (p *AgentDeploymentPolicy) KnowledgeSplitterDecisionPolicyRevision() string {
	return knownKnowledgeSplitterConfigurations[p.knowledgeSplitterConfigurationID].DecisionPolicyRevision
}

func (p *AgentDeploymentPolicy) KnowledgeSplitterLogicalModel() string {
	return knownKnowledgeSplitterConfigurations[p.knowledgeSplitterConfigurationID].LogicalModel
}

func validateKnowledgeCuratorConfigurationID(id string) error {
	if !strings.HasPrefix(id, "knowledge-curator-config-") {
		return fmt.Errorf("invalid Knowledge Curator Agent configuration id %q", id)
	}
	if _, ok := knownKnowledgeCuratorConfigurations[id]; !ok {
		return fmt.Errorf("unknown Knowledge Curator Agent configuration id %q", id)
	}
	return nil
}

func validateKnowledgeSplitterConfigurationID(id string) error {
	if !strings.HasPrefix(id, "knowledge-splitter-config-") {
		return fmt.Errorf("invalid Knowledge Splitter Agent configuration id %q", id)
	}
	if _, ok := knownKnowledgeSplitterConfigurations[id]; !ok {
		return fmt.Errorf("unknown Knowledge Splitter Agent configuration id %q", id)
	}
	return nil
}

func validRolloutStage(stage string) bool {
	switch stage {
	case "champion", "shadow", "canary", "promoted", "rollback":
		return true
	default:
		return false
	}
}

func approvedTreatmentCanaryStep(bps int) bool {
	return bps == 500 || bps == 2500 || bps == 5000
}

func stableRolloutBucket(salt, subjectID string) int {
	sum := sha256.Sum256([]byte(salt + "\x00" + strings.TrimSpace(subjectID)))
	return int(binary.BigEndian.Uint64(sum[:8]) % 10000)
}

func diagnosisStableRolloutBucket(salt, subjectID string) int {
	return stableRolloutBucket(salt, subjectID)
}

func stageOrChampion(stage string) string {
	if stage == "" {
		return DiagnosisRolloutChampion
	}
	return stage
}
