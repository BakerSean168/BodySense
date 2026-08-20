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
	defaultDiagnosisConfigurationID     = "diag-config-f492eb1c0c6676ae"
	diagnosisEvidenceGapConfigurationID = "diag-config-20fbfc23ca09cbab"
	diagnosisDecisionAuthorityConfigID  = "diag-config-5a4a13627e14b4cf"

	DiagnosisRolloutChampion = "champion"
	DiagnosisRolloutShadow   = "shadow"
	DiagnosisRolloutCanary   = "canary"
	DiagnosisRolloutPromoted = "promoted"
	DiagnosisRolloutRollback = "rollback"

	defaultDiagnosisCanaryBPS   = 1000
	defaultDiagnosisRolloutSalt = "diagnosis-rollout-v1"
	DiagnosisPromotionRecordV1  = "diagnosis_promotion_v1"

	defaultTreatmentConfigurationID     = "treat-config-85718f8e90ac9d80"
	treatmentEvidenceGapConfigurationID = "treat-config-f68eec9846664596"
	treatmentLogicalModelV1             = "bodysense-structured"

	TreatmentRolloutChampion = "champion"
	TreatmentRolloutShadow   = "shadow"
	TreatmentRolloutCanary   = "canary"
	TreatmentRolloutPromoted = "promoted"
	TreatmentRolloutRollback = "rollback"

	defaultTreatmentCanaryBPS   = 500
	defaultTreatmentRolloutSalt = "treatment-rollout-v1"
	TreatmentPromotionRecordV1  = "treatment_promotion_v1"

	defaultAssessmentConfigurationID = "assess-config-fbff8155337b388d"
	assessmentLogicalModelV1         = "bodysense-structured"
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
	LogicalModel           string
}

var knownTreatmentConfigurations = map[string]treatmentConfigurationRegistration{
	defaultTreatmentConfigurationID: {
		DecisionPolicyRevision: TreatmentDecisionPolicyV1,
		LogicalModel:           treatmentLogicalModelV1,
	},
	treatmentEvidenceGapConfigurationID: {
		DecisionPolicyRevision: TreatmentDecisionPolicyV1,
		LogicalModel:           treatmentLogicalModelV1,
	},
}

// AssessmentDecisionPolicyV1 is the deterministic fail-closed generation policy
// revision for the Assessment role (mirrors treatment-go-acceptance-v1 naming).
const AssessmentDecisionPolicyV1 = "assessment-go-generation-v1"

var knownAssessmentConfigurations = map[string]assessmentConfigurationRegistration{
	defaultAssessmentConfigurationID: {
		DecisionPolicyRevision: AssessmentDecisionPolicyV1,
		LogicalModel:           assessmentLogicalModelV1,
	},
}

var knownDiagnosisConfigurations = map[string]diagnosisConfigurationRegistration{
	defaultDiagnosisConfigurationID: {
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
	ChallengerConfigurationID    string `json:"challenger_configuration_id"`
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
	ChallengerConfigurationID    string `json:"challenger_configuration_id"`
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
}

func NewAgentDeploymentPolicy() (*AgentDeploymentPolicy, error) {
	diagnosisChampion := strings.TrimSpace(os.Getenv("DIAGNOSIS_CHAMPION_CONFIGURATION_ID"))
	if diagnosisChampion == "" {
		diagnosisChampion = defaultDiagnosisConfigurationID
	}
	diagnosisChallenger := strings.TrimSpace(os.Getenv("DIAGNOSIS_CHALLENGER_CONFIGURATION_ID"))
	if diagnosisChallenger == "" {
		diagnosisChallenger = diagnosisDecisionAuthorityConfigID
	}
	if err := validateDiagnosisConfigurationID(diagnosisChampion); err != nil {
		return nil, err
	}
	if err := validateDiagnosisConfigurationID(diagnosisChallenger); err != nil {
		return nil, err
	}
	if diagnosisChampion == diagnosisChallenger {
		return nil, fmt.Errorf("Diagnosis champion and challenger must be different immutable configurations")
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
		if diagnosisPromotionRecord != DiagnosisPromotionRecordV1 ||
			diagnosisChampion != defaultDiagnosisConfigurationID ||
			diagnosisChallenger != diagnosisDecisionAuthorityConfigID {
			return nil, fmt.Errorf(
				"Diagnosis rollout stage %q requires approved promotion record %q for the qualified v1 -> v3 pair",
				diagnosisStage,
				DiagnosisPromotionRecordV1,
			)
		}
	}

	treatmentChampion := strings.TrimSpace(os.Getenv("TREATMENT_CHAMPION_CONFIGURATION_ID"))
	legacyTreatmentPointer := strings.TrimSpace(os.Getenv("TREATMENT_AGENT_CONFIGURATION_ID"))
	if treatmentChampion == "" {
		treatmentChampion = legacyTreatmentPointer
	}
	if treatmentChampion == "" {
		treatmentChampion = defaultTreatmentConfigurationID
	}
	if err := validateTreatmentConfigurationID(treatmentChampion); err != nil {
		return nil, err
	}
	explicitTreatmentChallenger := strings.TrimSpace(os.Getenv("TREATMENT_CHALLENGER_CONFIGURATION_ID"))
	treatmentChallenger := explicitTreatmentChallenger
	if treatmentChallenger == "" {
		treatmentChallenger = treatmentEvidenceGapConfigurationID
		if treatmentChallenger == treatmentChampion {
			treatmentChallenger = defaultTreatmentConfigurationID
		}
	}
	if err := validateTreatmentConfigurationID(treatmentChallenger); err != nil {
		return nil, err
	}
	if treatmentChampion == treatmentChallenger {
		return nil, fmt.Errorf("Treatment champion and challenger must be different immutable configurations")
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
		if treatmentPromotionRecord != TreatmentPromotionRecordV1 ||
			treatmentChampion != defaultTreatmentConfigurationID ||
			treatmentChallenger != treatmentEvidenceGapConfigurationID {
			return nil, fmt.Errorf(
				"Treatment rollout stage %q requires approved promotion record %q for the qualified v1 -> v2 pair",
				treatmentStage,
				TreatmentPromotionRecordV1,
			)
		}
	}

	assessmentChampion := strings.TrimSpace(os.Getenv("ASSESSMENT_CHAMPION_CONFIGURATION_ID"))
	if assessmentChampion == "" {
		assessmentChampion = defaultAssessmentConfigurationID
	}
	if err := validateAssessmentConfigurationID(assessmentChampion); err != nil {
		return nil, err
	}
	assessmentChallenger := strings.TrimSpace(os.Getenv("ASSESSMENT_CHALLENGER_CONFIGURATION_ID"))
	if assessmentChallenger == "" {
		// No challenger yet: Assessment has a single Champion configuration.
		assessmentChallenger = assessmentChampion
	}
	if err := validateAssessmentConfigurationID(assessmentChallenger); err != nil {
		return nil, err
	}

	assessmentStage := strings.TrimSpace(strings.ToLower(os.Getenv("ASSESSMENT_ROLLOUT_STAGE")))
	if assessmentStage == "" {
		assessmentStage = DiagnosisRolloutChampion
	}
	if !validRolloutStage(assessmentStage) {
		return nil, fmt.Errorf("invalid Assessment rollout stage %q", assessmentStage)
	}
	// A challenger is only required once a non-Champion stage is requested.
	if assessmentStage != DiagnosisRolloutChampion && assessmentStage != DiagnosisRolloutRollback {
		if assessmentChampion == assessmentChallenger {
			return nil, fmt.Errorf("Assessment rollout stage %q requires a distinct challenger configuration", assessmentStage)
		}
	}
	assessmentCanaryBPS := defaultDiagnosisCanaryBPS
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
	if assessmentStage == DiagnosisRolloutCanary && (assessmentCanaryBPS <= 0 || assessmentCanaryBPS >= 10000) {
		return nil, fmt.Errorf("canary stage requires ASSESSMENT_CANARY_BPS between 1 and 9999")
	}
	assessmentRolloutSalt := strings.TrimSpace(os.Getenv("ASSESSMENT_ROLLOUT_SALT"))
	if assessmentRolloutSalt == "" {
		assessmentRolloutSalt = defaultDiagnosisRolloutSalt
	}
	assessmentPromotionRecord := strings.TrimSpace(os.Getenv("ASSESSMENT_PROMOTION_RECORD"))
	if assessmentStage == DiagnosisRolloutShadow || assessmentStage == DiagnosisRolloutCanary || assessmentStage == DiagnosisRolloutPromoted {
		if assessmentPromotionRecord == "" {
			return nil, fmt.Errorf(
				"Assessment rollout stage %q requires an approved promotion record",
				assessmentStage,
			)
		}
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

// TreatmentConfigurationID preserves the pre-rollout compatibility accessor.
// TREATMENT_AGENT_CONFIGURATION_ID is interpreted as the Champion alias only.
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
		served = p.diagnosisChampionConfigurationID
	}

	selection := DiagnosisRouteSelection{
		Stage: stageOrChampion(p.diagnosisStage), SubjectBucket: bucket,
		ServedConfigurationID:        served,
		ServedDecisionPolicyRevision: knownDiagnosisConfigurations[served].DecisionPolicyRevision,
		ShadowConfigurationID:        shadow,
		ChampionConfigurationID:      p.diagnosisChampionConfigurationID,
		ChallengerConfigurationID:    p.diagnosisChallengerConfigurationID,
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
		served = p.treatmentChampionConfigurationID
	}

	selection := TreatmentRouteSelection{
		Stage: stageOrChampion(p.treatmentStage), SubjectBucket: bucket,
		ServedConfigurationID:        served,
		ServedDecisionPolicyRevision: knownTreatmentConfigurations[served].DecisionPolicyRevision,
		ShadowConfigurationID:        shadow,
		ChampionConfigurationID:      p.treatmentChampionConfigurationID,
		ChallengerConfigurationID:    p.treatmentChallengerConfigurationID,
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

func validateAssessmentConfigurationID(id string) error {
	if !strings.HasPrefix(id, "assess-config-") {
		return fmt.Errorf("invalid Assessment Agent configuration id %q", id)
	}
	if _, ok := knownAssessmentConfigurations[id]; !ok {
		return fmt.Errorf("unknown Assessment Agent configuration id %q", id)
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
