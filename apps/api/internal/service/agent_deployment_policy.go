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
)

type diagnosisConfigurationRegistration struct {
	DecisionPolicyRevision string
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

// AgentDeploymentPolicy is the Go-owned mutable pointer from business role to
// immutable repository-known Agent configurations. Rollout changes selection;
// it never mutates an Agent configuration.
type AgentDeploymentPolicy struct {
	championConfigurationID   string
	challengerConfigurationID string
	stage                     string
	canaryBPS                 int
	rolloutSalt               string
	promotionRecord           string
}

func NewAgentDeploymentPolicy() (*AgentDeploymentPolicy, error) {
	champion := strings.TrimSpace(os.Getenv("DIAGNOSIS_CHAMPION_CONFIGURATION_ID"))
	if champion == "" {
		// Temporary Phase-10 compatibility alias.
		champion = strings.TrimSpace(os.Getenv("DIAGNOSIS_AGENT_CONFIGURATION_ID"))
	}
	if champion == "" {
		champion = defaultDiagnosisConfigurationID
	}
	challenger := strings.TrimSpace(os.Getenv("DIAGNOSIS_CHALLENGER_CONFIGURATION_ID"))
	if challenger == "" {
		challenger = diagnosisDecisionAuthorityConfigID
	}
	if err := validateDiagnosisConfigurationID(champion); err != nil {
		return nil, err
	}
	if err := validateDiagnosisConfigurationID(challenger); err != nil {
		return nil, err
	}
	if champion == challenger {
		return nil, fmt.Errorf("Diagnosis champion and challenger must be different immutable configurations")
	}

	stage := strings.TrimSpace(strings.ToLower(os.Getenv("DIAGNOSIS_ROLLOUT_STAGE")))
	if stage == "" {
		stage = DiagnosisRolloutChampion
	}
	switch stage {
	case DiagnosisRolloutChampion, DiagnosisRolloutShadow, DiagnosisRolloutCanary, DiagnosisRolloutPromoted, DiagnosisRolloutRollback:
	default:
		return nil, fmt.Errorf("invalid Diagnosis rollout stage %q", stage)
	}

	canaryBPS := defaultDiagnosisCanaryBPS
	if raw := strings.TrimSpace(os.Getenv("DIAGNOSIS_CANARY_BPS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid DIAGNOSIS_CANARY_BPS %q", raw)
		}
		canaryBPS = value
	}
	if canaryBPS < 0 || canaryBPS > 10000 {
		return nil, fmt.Errorf("DIAGNOSIS_CANARY_BPS must be between 0 and 10000")
	}
	if stage == DiagnosisRolloutCanary && (canaryBPS <= 0 || canaryBPS >= 10000) {
		return nil, fmt.Errorf("canary stage requires DIAGNOSIS_CANARY_BPS between 1 and 9999")
	}
	rolloutSalt := strings.TrimSpace(os.Getenv("DIAGNOSIS_ROLLOUT_SALT"))
	if rolloutSalt == "" {
		rolloutSalt = defaultDiagnosisRolloutSalt
	}
	promotionRecord := strings.TrimSpace(os.Getenv("DIAGNOSIS_PROMOTION_RECORD"))
	if stage == DiagnosisRolloutShadow || stage == DiagnosisRolloutCanary || stage == DiagnosisRolloutPromoted {
		if promotionRecord != DiagnosisPromotionRecordV1 || champion != defaultDiagnosisConfigurationID || challenger != diagnosisDecisionAuthorityConfigID {
			return nil, fmt.Errorf("Diagnosis rollout stage %q requires approved promotion record %q for the qualified v1 -> v3 pair", stage, DiagnosisPromotionRecordV1)
		}
	}

	return &AgentDeploymentPolicy{
		championConfigurationID:   champion,
		challengerConfigurationID: challenger,
		stage:                     stage,
		canaryBPS:                 canaryBPS,
		rolloutSalt:               rolloutSalt,
		promotionRecord:           promotionRecord,
	}, nil
}

// DiagnosisConfigurationID preserves the pre-rollout compatibility accessor. It
// is the champion pointer, not a per-user route decision.
func (p *AgentDeploymentPolicy) DiagnosisConfigurationID() string {
	return p.championConfigurationID
}

func (p *AgentDeploymentPolicy) DiagnosisDecisionPolicyRevision() string {
	return knownDiagnosisConfigurations[p.championConfigurationID].DecisionPolicyRevision
}

func (p *AgentDeploymentPolicy) DiagnosisRolloutStage() string { return p.stage }

func (p *AgentDeploymentPolicy) SelectDiagnosisRoute(subjectID string) DiagnosisRouteSelection {
	bucket := diagnosisStableRolloutBucket(p.rolloutSalt, subjectID)
	served := p.championConfigurationID
	shadow := ""

	switch p.stage {
	case DiagnosisRolloutShadow:
		shadow = p.challengerConfigurationID
	case DiagnosisRolloutCanary:
		if bucket < p.canaryBPS {
			served = p.challengerConfigurationID
			shadow = p.championConfigurationID
		} else {
			shadow = p.challengerConfigurationID
		}
	case DiagnosisRolloutPromoted:
		served = p.challengerConfigurationID
	case DiagnosisRolloutRollback:
		served = p.championConfigurationID
	}

	selection := DiagnosisRouteSelection{
		Stage: stageOrChampion(p.stage), SubjectBucket: bucket,
		ServedConfigurationID:        served,
		ServedDecisionPolicyRevision: knownDiagnosisConfigurations[served].DecisionPolicyRevision,
		ShadowConfigurationID:        shadow,
		ChampionConfigurationID:      p.championConfigurationID,
		ChallengerConfigurationID:    p.challengerConfigurationID,
		CanaryBPS:                    p.canaryBPS,
		PromotionRecord:              p.promotionRecord,
	}
	if shadow != "" {
		selection.ShadowDecisionPolicyRevision = knownDiagnosisConfigurations[shadow].DecisionPolicyRevision
	}
	return selection
}

// DiagnosisDecisionPolicyRevisionForConfiguration resolves one immutable
// repository-known Diagnosis configuration without changing the serving pointer.
// Replay, shadow, and qualification paths use this to evaluate a selected config.
func DiagnosisDecisionPolicyRevisionForConfiguration(configurationID string) (string, error) {
	registration, ok := knownDiagnosisConfigurations[strings.TrimSpace(configurationID)]
	if !ok {
		return "", fmt.Errorf("unknown Diagnosis Agent configuration id %q", configurationID)
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

func diagnosisStableRolloutBucket(salt, subjectID string) int {
	sum := sha256.Sum256([]byte(salt + "\x00" + strings.TrimSpace(subjectID)))
	return int(binary.BigEndian.Uint64(sum[:8]) % 10000)
}

func stageOrChampion(stage string) string {
	if stage == "" {
		return DiagnosisRolloutChampion
	}
	return stage
}
