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

// rolloutSelection is one role's resolved deployment pointers: the immutable
// champion/challenger configurations plus the rollout stage controls.
type rolloutSelection struct {
	champion        string
	challenger      string
	stage           string
	canaryBPS       int
	rolloutSalt     string
	promotionRecord string
}

func resolveDiagnosisRollout(cfg config.Deployment) (rolloutSelection, error) {
	champion := cfg.DiagnosisChampionConfigurationID
	if champion == "" {
		champion = defaultDiagnosisConfigurationID
	}
	if err := validateDiagnosisConfigurationID(champion); err != nil {
		return rolloutSelection{}, err
	}
	challenger := cfg.DiagnosisChallengerConfigurationID
	if challenger != "" {
		if err := validateDiagnosisConfigurationID(challenger); err != nil {
			return rolloutSelection{}, err
		}
		if champion == challenger {
			return rolloutSelection{}, fmt.Errorf("Diagnosis champion and challenger must be different immutable configurations")
		}
	}

	stage := strings.ToLower(cfg.DiagnosisRolloutStage)
	if stage == "" {
		stage = DiagnosisRolloutChampion
	}
	if !validRolloutStage(stage) {
		return rolloutSelection{}, fmt.Errorf("invalid Diagnosis rollout stage %q", stage)
	}
	canaryBPS := defaultDiagnosisCanaryBPS
	if raw := cfg.DiagnosisCanaryBPS; raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return rolloutSelection{}, fmt.Errorf("invalid DIAGNOSIS_CANARY_BPS %q", raw)
		}
		canaryBPS = value
	}
	if canaryBPS < 0 || canaryBPS > 10000 {
		return rolloutSelection{}, fmt.Errorf("DIAGNOSIS_CANARY_BPS must be between 0 and 10000")
	}
	if stage == DiagnosisRolloutCanary && !approvedDiagnosisCanaryStep(canaryBPS) {
		return rolloutSelection{}, fmt.Errorf("Diagnosis canary stage requires DIAGNOSIS_CANARY_BPS to be one of 500, 2500, 5000")
	}
	rolloutSalt := cfg.DiagnosisRolloutSalt
	if rolloutSalt == "" {
		rolloutSalt = defaultDiagnosisRolloutSalt
	}
	promotionRecord := cfg.DiagnosisPromotionRecord
	if stage == DiagnosisRolloutShadow || stage == DiagnosisRolloutCanary || stage == DiagnosisRolloutPromoted {
		if challenger == "" {
			return rolloutSelection{}, fmt.Errorf("Diagnosis rollout stage %q requires a distinct active challenger configuration", stage)
		}
		if promotionRecord == "" {
			return rolloutSelection{}, fmt.Errorf("Diagnosis rollout stage %q requires an approved promotion record", stage)
		}
		if err := validateDiagnosisPromotionRecord(
			promotionRecord, champion, challenger,
		); err != nil {
			return rolloutSelection{}, err
		}
	}
	return rolloutSelection{
		champion: champion, challenger: challenger, stage: stage,
		canaryBPS: canaryBPS, rolloutSalt: rolloutSalt, promotionRecord: promotionRecord,
	}, nil
}

func resolveTreatmentRollout(cfg config.Deployment) (rolloutSelection, error) {
	champion := cfg.TreatmentChampionConfigurationID
	if champion == "" {
		champion = defaultTreatmentConfigurationID
	}
	if err := validateTreatmentConfigurationID(champion); err != nil {
		return rolloutSelection{}, err
	}
	challenger := cfg.TreatmentChallengerConfigurationID
	if challenger != "" {
		if err := validateTreatmentConfigurationID(challenger); err != nil {
			return rolloutSelection{}, err
		}
		if champion == challenger {
			return rolloutSelection{}, fmt.Errorf("Treatment champion and challenger must be different immutable configurations")
		}
	}

	stage := strings.ToLower(cfg.TreatmentRolloutStage)
	if stage == "" {
		stage = TreatmentRolloutChampion
	}
	if !validRolloutStage(stage) {
		return rolloutSelection{}, fmt.Errorf("invalid Treatment rollout stage %q", stage)
	}
	canaryBPS := defaultTreatmentCanaryBPS
	if raw := cfg.TreatmentCanaryBPS; raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return rolloutSelection{}, fmt.Errorf("invalid TREATMENT_CANARY_BPS %q", raw)
		}
		canaryBPS = value
	}
	if canaryBPS < 0 || canaryBPS > 10000 {
		return rolloutSelection{}, fmt.Errorf("TREATMENT_CANARY_BPS must be between 0 and 10000")
	}
	if stage == TreatmentRolloutCanary && !approvedTreatmentCanaryStep(canaryBPS) {
		return rolloutSelection{}, fmt.Errorf("Treatment canary stage requires TREATMENT_CANARY_BPS to be one of 500, 2500, 5000")
	}
	rolloutSalt := cfg.TreatmentRolloutSalt
	if rolloutSalt == "" {
		rolloutSalt = defaultTreatmentRolloutSalt
	}
	promotionRecord := cfg.TreatmentPromotionRecord
	if stage == TreatmentRolloutShadow || stage == TreatmentRolloutCanary || stage == TreatmentRolloutPromoted {
		if challenger == "" {
			return rolloutSelection{}, fmt.Errorf("Treatment rollout stage %q requires a distinct active challenger configuration", stage)
		}
		if promotionRecord == "" {
			return rolloutSelection{}, fmt.Errorf("Treatment rollout stage %q requires an approved promotion record", stage)
		}
	}
	return rolloutSelection{
		champion: champion, challenger: challenger, stage: stage,
		canaryBPS: canaryBPS, rolloutSalt: rolloutSalt, promotionRecord: promotionRecord,
	}, nil
}

func resolveAssessmentRollout(cfg config.Deployment) (rolloutSelection, error) {
	champion := cfg.AssessmentChampionConfigurationID
	if champion == "" {
		champion = defaultAssessmentConfigurationID
	}
	if err := validateAssessmentServingConfigurationID(champion); err != nil {
		return rolloutSelection{}, err
	}
	challenger := cfg.AssessmentChallengerConfigurationID
	if challenger == "" {
		// No challenger yet: Assessment has a single Champion configuration.
		challenger = champion
	}
	if err := validateAssessmentKnownConfigurationID(challenger); err != nil {
		return rolloutSelection{}, err
	}

	stage := strings.ToLower(cfg.AssessmentRolloutStage)
	if stage == "" {
		stage = AssessmentRolloutChampion
	}
	if !validRolloutStage(stage) {
		return rolloutSelection{}, fmt.Errorf("invalid Assessment rollout stage %q", stage)
	}
	// A challenger is only required once a non-Champion stage is requested.
	if stage != AssessmentRolloutChampion {
		if champion == challenger {
			return rolloutSelection{}, fmt.Errorf("Assessment rollout stage %q requires a distinct challenger configuration", stage)
		}
	}
	if stage != AssessmentRolloutChampion {
		if err := validateAssessmentServingConfigurationID(challenger); err != nil {
			return rolloutSelection{}, fmt.Errorf("Assessment rollout stage %q cannot use challenger: %w", stage, err)
		}
	}
	canaryBPS := defaultAssessmentCanaryBPS
	if raw := cfg.AssessmentCanaryBPS; raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return rolloutSelection{}, fmt.Errorf("invalid ASSESSMENT_CANARY_BPS %q", raw)
		}
		canaryBPS = value
	}
	if canaryBPS < 0 || canaryBPS > 10000 {
		return rolloutSelection{}, fmt.Errorf("ASSESSMENT_CANARY_BPS must be between 0 and 10000")
	}
	if stage == AssessmentRolloutCanary && (canaryBPS <= 0 || canaryBPS >= 10000) {
		return rolloutSelection{}, fmt.Errorf("canary stage requires ASSESSMENT_CANARY_BPS between 1 and 9999")
	}
	rolloutSalt := cfg.AssessmentRolloutSalt
	if rolloutSalt == "" {
		rolloutSalt = defaultAssessmentRolloutSalt
	}
	promotionRecord := cfg.AssessmentPromotionRecord
	if stage == AssessmentRolloutShadow || stage == AssessmentRolloutCanary || stage == AssessmentRolloutPromoted {
		if promotionRecord == "" {
			return rolloutSelection{}, fmt.Errorf(
				"Assessment rollout stage %q requires an approved promotion record",
				stage,
			)
		}
	}
	return rolloutSelection{
		champion: champion, challenger: challenger, stage: stage,
		canaryBPS: canaryBPS, rolloutSalt: rolloutSalt, promotionRecord: promotionRecord,
	}, nil
}

// resolveChampionConfiguration applies the default-then-validate rule shared
// by the single-configuration roles.
func resolveChampionConfiguration(raw, defaultID string, validate func(string) error) (string, error) {
	id := raw
	if id == "" {
		id = defaultID
	}
	if err := validate(id); err != nil {
		return "", err
	}
	return id, nil
}

// NewAgentDeploymentPolicy resolves the deployment pointers read by the
// composition root (see config.Deployment) into the serving policy. Defaults,
// validation and error text stay here: this is the package that owns what a
// legal rollout is.
func NewAgentDeploymentPolicy(cfg config.Deployment) (*AgentDeploymentPolicy, error) {
	diagnosis, err := resolveDiagnosisRollout(cfg)
	if err != nil {
		return nil, err
	}
	treatment, err := resolveTreatmentRollout(cfg)
	if err != nil {
		return nil, err
	}
	assessment, err := resolveAssessmentRollout(cfg)
	if err != nil {
		return nil, err
	}

	consultationChampion, err := resolveChampionConfiguration(
		cfg.ConsultationChampionConfigurationID, defaultConsultationConfigurationID, validateConsultationConfigurationID,
	)
	if err != nil {
		return nil, err
	}
	postureChampion, err := resolveChampionConfiguration(
		cfg.PostureChampionConfigurationID, defaultPostureConfigurationID, validatePostureConfigurationID,
	)
	if err != nil {
		return nil, err
	}
	knowledgeCurator, err := resolveChampionConfiguration(
		cfg.KnowledgeCuratorConfigurationID, defaultKnowledgeCuratorConfigurationID, validateKnowledgeCuratorConfigurationID,
	)
	if err != nil {
		return nil, err
	}
	knowledgeSplitter, err := resolveChampionConfiguration(
		cfg.KnowledgeSplitterConfigurationID, defaultKnowledgeSplitterConfigurationID, validateKnowledgeSplitterConfigurationID,
	)
	if err != nil {
		return nil, err
	}

	return &AgentDeploymentPolicy{
		diagnosisChampionConfigurationID:    diagnosis.champion,
		diagnosisChallengerConfigurationID:  diagnosis.challenger,
		diagnosisStage:                      diagnosis.stage,
		diagnosisCanaryBPS:                  diagnosis.canaryBPS,
		diagnosisRolloutSalt:                diagnosis.rolloutSalt,
		diagnosisPromotionRecord:            diagnosis.promotionRecord,
		treatmentChampionConfigurationID:    treatment.champion,
		treatmentChallengerConfigurationID:  treatment.challenger,
		treatmentStage:                      treatment.stage,
		treatmentCanaryBPS:                  treatment.canaryBPS,
		treatmentRolloutSalt:                treatment.rolloutSalt,
		treatmentPromotionRecord:            treatment.promotionRecord,
		assessmentChampionConfigurationID:   assessment.champion,
		assessmentChallengerConfigurationID: assessment.challenger,
		assessmentStage:                     assessment.stage,
		assessmentCanaryBPS:                 assessment.canaryBPS,
		assessmentRolloutSalt:               assessment.rolloutSalt,
		assessmentPromotionRecord:           assessment.promotionRecord,
		consultationChampionConfigurationID: consultationChampion,
		postureChampionConfigurationID:      postureChampion,
		knowledgeCuratorConfigurationID:     knowledgeCurator,
		knowledgeSplitterConfigurationID:    knowledgeSplitter,
	}, nil
}
