package service

import (
	"fmt"
	"strings"
)

func validateDiagnosisPromotionRecord(recordID, championID, challengerID string) error {
	record, ok := knownDiagnosisPromotionRecords[recordID]
	if !ok {
		return fmt.Errorf("unknown Diagnosis promotion record %q", recordID)
	}
	if !validDiagnosisRolloutPolicyRevision(record.RolloutPolicyRevision) {
		return fmt.Errorf("Diagnosis promotion record %q has unsupported rollout policy %q", recordID, record.RolloutPolicyRevision)
	}
	if record.ChampionConfigurationID != championID || record.ChallengerConfigurationID != challengerID {
		return fmt.Errorf(
			"Diagnosis promotion record %q does not approve Champion %q -> Challenger %q",
			recordID, championID, challengerID,
		)
	}
	return nil
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
	return validateAssessmentKnownConfigurationID(id)
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
	return validatePostureKnownConfigurationID(id)
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
	case "champion", "shadow", "canary", "promoted":
		return true
	default:
		return false
	}
}

func approvedTreatmentCanaryStep(bps int) bool {
	return bps == 500 || bps == 2500 || bps == 5000
}

func approvedDiagnosisCanaryStep(bps int) bool {
	return bps == 500 || bps == 2500 || bps == 5000
}
