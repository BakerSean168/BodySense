package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bodysense/api/internal/config"
)

// AgentDeploymentPolicy is the Go-owned mutable pointer from business role to
// immutable repository-known Agent configurations. Rollout changes selection;
// it never mutates an Agent configuration.
type AgentDeploymentPolicy struct {
	diagnosisChampionConfigurationID   string
	diagnosisChallengerConfigurationID string
	diagnosisStage                     string
	diagnosisCanaryBPS                 int
	diagnosisRolloutSalt               string
	diagnosisPromotionRecord           string

	treatmentChampionConfigurationID   string
	treatmentChallengerConfigurationID string
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
	knowledgeCuratorConfigurationID  string
	knowledgeSplitterConfigurationID string
}

// NewAgentDeploymentPolicy resolves the deployment pointers read by the
// composition root (see config.Deployment) into the serving policy. Defaults,
// validation and error text stay here: this is the package that owns what a
// legal rollout is.
func NewAgentDeploymentPolicy(cfg config.Deployment) (*AgentDeploymentPolicy, error) {
	diagnosisChampion := cfg.DiagnosisChampionConfigurationID
	if diagnosisChampion == "" {
		diagnosisChampion = defaultDiagnosisConfigurationID
	}
	if err := validateDiagnosisConfigurationID(diagnosisChampion); err != nil {
		return nil, err
	}
	diagnosisChallenger := cfg.DiagnosisChallengerConfigurationID
	if diagnosisChallenger != "" {
		if err := validateDiagnosisConfigurationID(diagnosisChallenger); err != nil {
			return nil, err
		}
		if diagnosisChampion == diagnosisChallenger {
			return nil, fmt.Errorf("Diagnosis champion and challenger must be different immutable configurations")
		}
	}

	diagnosisStage := strings.ToLower(cfg.DiagnosisRolloutStage)
	if diagnosisStage == "" {
		diagnosisStage = DiagnosisRolloutChampion
	}
	if !validRolloutStage(diagnosisStage) {
		return nil, fmt.Errorf("invalid Diagnosis rollout stage %q", diagnosisStage)
	}
	diagnosisCanaryBPS := defaultDiagnosisCanaryBPS
	if raw := cfg.DiagnosisCanaryBPS; raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid DIAGNOSIS_CANARY_BPS %q", raw)
		}
		diagnosisCanaryBPS = value
	}
	if diagnosisCanaryBPS < 0 || diagnosisCanaryBPS > 10000 {
		return nil, fmt.Errorf("DIAGNOSIS_CANARY_BPS must be between 0 and 10000")
	}
	if diagnosisStage == DiagnosisRolloutCanary && !approvedDiagnosisCanaryStep(diagnosisCanaryBPS) {
		return nil, fmt.Errorf("Diagnosis canary stage requires DIAGNOSIS_CANARY_BPS to be one of 500, 2500, 5000")
	}
	diagnosisRolloutSalt := cfg.DiagnosisRolloutSalt
	if diagnosisRolloutSalt == "" {
		diagnosisRolloutSalt = defaultDiagnosisRolloutSalt
	}
	diagnosisPromotionRecord := cfg.DiagnosisPromotionRecord
	if diagnosisStage == DiagnosisRolloutShadow || diagnosisStage == DiagnosisRolloutCanary || diagnosisStage == DiagnosisRolloutPromoted {
		if diagnosisChallenger == "" {
			return nil, fmt.Errorf("Diagnosis rollout stage %q requires a distinct active challenger configuration", diagnosisStage)
		}
		if diagnosisPromotionRecord == "" {
			return nil, fmt.Errorf("Diagnosis rollout stage %q requires an approved promotion record", diagnosisStage)
		}
		if err := validateDiagnosisPromotionRecord(
			diagnosisPromotionRecord, diagnosisChampion, diagnosisChallenger,
		); err != nil {
			return nil, err
		}
	}

	treatmentChampion := cfg.TreatmentChampionConfigurationID
	if treatmentChampion == "" {
		treatmentChampion = defaultTreatmentConfigurationID
	}
	if err := validateTreatmentConfigurationID(treatmentChampion); err != nil {
		return nil, err
	}
	treatmentChallenger := cfg.TreatmentChallengerConfigurationID
	if treatmentChallenger != "" {
		if err := validateTreatmentConfigurationID(treatmentChallenger); err != nil {
			return nil, err
		}
		if treatmentChampion == treatmentChallenger {
			return nil, fmt.Errorf("Treatment champion and challenger must be different immutable configurations")
		}
	}

	treatmentStage := strings.ToLower(cfg.TreatmentRolloutStage)
	if treatmentStage == "" {
		treatmentStage = TreatmentRolloutChampion
	}
	if !validRolloutStage(treatmentStage) {
		return nil, fmt.Errorf("invalid Treatment rollout stage %q", treatmentStage)
	}
	treatmentCanaryBPS := defaultTreatmentCanaryBPS
	if raw := cfg.TreatmentCanaryBPS; raw != "" {
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
	treatmentRolloutSalt := cfg.TreatmentRolloutSalt
	if treatmentRolloutSalt == "" {
		treatmentRolloutSalt = defaultTreatmentRolloutSalt
	}
	treatmentPromotionRecord := cfg.TreatmentPromotionRecord
	if treatmentStage == TreatmentRolloutShadow || treatmentStage == TreatmentRolloutCanary || treatmentStage == TreatmentRolloutPromoted {
		if treatmentChallenger == "" {
			return nil, fmt.Errorf("Treatment rollout stage %q requires a distinct active challenger configuration", treatmentStage)
		}
		if treatmentPromotionRecord == "" {
			return nil, fmt.Errorf("Treatment rollout stage %q requires an approved promotion record", treatmentStage)
		}
	}

	assessmentChampion := cfg.AssessmentChampionConfigurationID
	if assessmentChampion == "" {
		assessmentChampion = defaultAssessmentConfigurationID
	}
	if err := validateAssessmentServingConfigurationID(assessmentChampion); err != nil {
		return nil, err
	}
	assessmentChallenger := cfg.AssessmentChallengerConfigurationID
	if assessmentChallenger == "" {
		// No challenger yet: Assessment has a single Champion configuration.
		assessmentChallenger = assessmentChampion
	}
	if err := validateAssessmentKnownConfigurationID(assessmentChallenger); err != nil {
		return nil, err
	}

	assessmentStage := strings.ToLower(cfg.AssessmentRolloutStage)
	if assessmentStage == "" {
		assessmentStage = AssessmentRolloutChampion
	}
	if !validRolloutStage(assessmentStage) {
		return nil, fmt.Errorf("invalid Assessment rollout stage %q", assessmentStage)
	}
	// A challenger is only required once a non-Champion stage is requested.
	if assessmentStage != AssessmentRolloutChampion {
		if assessmentChampion == assessmentChallenger {
			return nil, fmt.Errorf("Assessment rollout stage %q requires a distinct challenger configuration", assessmentStage)
		}
	}
	if assessmentStage != AssessmentRolloutChampion {
		if err := validateAssessmentServingConfigurationID(assessmentChallenger); err != nil {
			return nil, fmt.Errorf("Assessment rollout stage %q cannot use challenger: %w", assessmentStage, err)
		}
	}
	assessmentCanaryBPS := defaultAssessmentCanaryBPS
	if raw := cfg.AssessmentCanaryBPS; raw != "" {
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
	assessmentRolloutSalt := cfg.AssessmentRolloutSalt
	if assessmentRolloutSalt == "" {
		assessmentRolloutSalt = defaultAssessmentRolloutSalt
	}
	assessmentPromotionRecord := cfg.AssessmentPromotionRecord
	if assessmentStage == AssessmentRolloutShadow || assessmentStage == AssessmentRolloutCanary || assessmentStage == AssessmentRolloutPromoted {
		if assessmentPromotionRecord == "" {
			return nil, fmt.Errorf(
				"Assessment rollout stage %q requires an approved promotion record",
				assessmentStage,
			)
		}
	}

	consultationChampion := cfg.ConsultationChampionConfigurationID
	if consultationChampion == "" {
		consultationChampion = defaultConsultationConfigurationID
	}
	if err := validateConsultationConfigurationID(consultationChampion); err != nil {
		return nil, err
	}

	postureChampion := cfg.PostureChampionConfigurationID
	if postureChampion == "" {
		postureChampion = defaultPostureConfigurationID
	}
	if err := validatePostureConfigurationID(postureChampion); err != nil {
		return nil, err
	}

	knowledgeCurator := cfg.KnowledgeCuratorConfigurationID
	if knowledgeCurator == "" {
		knowledgeCurator = defaultKnowledgeCuratorConfigurationID
	}
	if err := validateKnowledgeCuratorConfigurationID(knowledgeCurator); err != nil {
		return nil, err
	}
	knowledgeSplitter := cfg.KnowledgeSplitterConfigurationID
	if knowledgeSplitter == "" {
		knowledgeSplitter = defaultKnowledgeSplitterConfigurationID
	}
	if err := validateKnowledgeSplitterConfigurationID(knowledgeSplitter); err != nil {
		return nil, err
	}

	return &AgentDeploymentPolicy{
		diagnosisChampionConfigurationID:    diagnosisChampion,
		diagnosisChallengerConfigurationID:  diagnosisChallenger,
		diagnosisStage:                      diagnosisStage,
		diagnosisCanaryBPS:                  diagnosisCanaryBPS,
		diagnosisRolloutSalt:                diagnosisRolloutSalt,
		diagnosisPromotionRecord:            diagnosisPromotionRecord,
		treatmentChampionConfigurationID:    treatmentChampion,
		treatmentChallengerConfigurationID:  treatmentChallenger,
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
		knowledgeCuratorConfigurationID:     knowledgeCurator,
		knowledgeSplitterConfigurationID:    knowledgeSplitter,
	}, nil
}
