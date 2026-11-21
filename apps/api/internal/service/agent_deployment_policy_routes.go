package service

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"
)

type DiagnosisRouteSelection struct {
	Stage                        string `json:"stage"`
	SubjectBucket                int    `json:"subject_bucket"`
	ServedConfigurationID        string `json:"served_configuration_id"`
	ServedDecisionPolicyRevision string `json:"served_decision_policy_revision"`
	ShadowConfigurationID        string `json:"shadow_configuration_id,omitempty"`
	ShadowDecisionPolicyRevision string `json:"shadow_decision_policy_revision,omitempty"`
	ChampionConfigurationID      string `json:"champion_configuration_id"`
	ChallengerConfigurationID    string `json:"challenger_configuration_id,omitempty"`
	CanaryBPS                    int    `json:"canary_bps"`
	PromotionRecord              string `json:"promotion_record,omitempty"`
	RolloutPolicyRevision        string `json:"rollout_policy_revision"`
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

// DiagnosisConfigurationID returns the current repository Champion.
func (p *AgentDeploymentPolicy) DiagnosisConfigurationID() string {
	return p.diagnosisChampionConfigurationID
}

func (p *AgentDeploymentPolicy) DiagnosisDecisionPolicyRevision() string {
	return knownDiagnosisConfigurations[p.diagnosisChampionConfigurationID].DecisionPolicyRevision
}

func (p *AgentDeploymentPolicy) DiagnosisRolloutStage() string { return p.diagnosisStage }

// TreatmentConfigurationID returns the current repository Champion.
func (p *AgentDeploymentPolicy) TreatmentConfigurationID() string {
	return p.treatmentChampionConfigurationID
}

func (p *AgentDeploymentPolicy) TreatmentRolloutStage() string { return p.treatmentStage }

// AssessmentConfigurationID returns the current repository Champion.
func (p *AgentDeploymentPolicy) AssessmentConfigurationID() string {
	return p.assessmentChampionConfigurationID
}

func (p *AgentDeploymentPolicy) AssessmentDecisionPolicyRevision() string {
	return knownAssessmentConfigurations[p.assessmentChampionConfigurationID].DecisionPolicyRevision
}

func (p *AgentDeploymentPolicy) AssessmentRolloutStage() string { return p.assessmentStage }

// ConsultationConfigurationID returns the current repository Champion.
func (p *AgentDeploymentPolicy) ConsultationConfigurationID() string {
	return p.consultationChampionConfigurationID
}

// PostureConfigurationID returns the champion Posture configuration pointer.
func (p *AgentDeploymentPolicy) PostureConfigurationID() string {
	return p.postureChampionConfigurationID
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
	}

	rolloutPolicyRevision := DiagnosisRolloutPolicyV1
	if record, ok := knownDiagnosisPromotionRecords[p.diagnosisPromotionRecord]; ok {
		rolloutPolicyRevision = record.RolloutPolicyRevision
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
		RolloutPolicyRevision:        rolloutPolicyRevision,
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

func (p *AgentDeploymentPolicy) SelectAssessmentRoute(subjectID string) AssessmentRouteSelection {
	bucket := stableRolloutBucket(p.assessmentRolloutSalt, subjectID)
	served := p.assessmentChampionConfigurationID
	shadow := ""

	switch p.assessmentStage {
	case AssessmentRolloutShadow:
		shadow = p.assessmentChallengerConfigurationID
	case AssessmentRolloutCanary:
		if bucket < p.assessmentCanaryBPS {
			served = p.assessmentChallengerConfigurationID
			shadow = p.assessmentChampionConfigurationID
		} else {
			shadow = p.assessmentChallengerConfigurationID
		}
	case AssessmentRolloutPromoted:
		served = p.assessmentChallengerConfigurationID
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
