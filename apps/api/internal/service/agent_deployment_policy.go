package service

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultDiagnosisConfigurationID     = "diag-config-f492eb1c0c6676ae"
	diagnosisEvidenceGapConfigurationID = "diag-config-20fbfc23ca09cbab"
	diagnosisDecisionAuthorityConfigID  = "diag-config-5a4a13627e14b4cf"
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

// AgentDeploymentPolicy is the Go-owned mutable pointer from business role to
// an immutable, repository-known Agent configuration. Python may execute this
// identity, but does not choose which configuration receives production traffic.
type AgentDeploymentPolicy struct {
	diagnosisConfigurationID string
}

func NewAgentDeploymentPolicy() (*AgentDeploymentPolicy, error) {
	id := strings.TrimSpace(os.Getenv("DIAGNOSIS_AGENT_CONFIGURATION_ID"))
	if id == "" {
		id = defaultDiagnosisConfigurationID
	}
	if !strings.HasPrefix(id, "diag-config-") {
		return nil, fmt.Errorf("invalid Diagnosis Agent configuration id %q", id)
	}
	if _, ok := knownDiagnosisConfigurations[id]; !ok {
		return nil, fmt.Errorf("unknown Diagnosis Agent configuration id %q", id)
	}
	return &AgentDeploymentPolicy{diagnosisConfigurationID: id}, nil
}

func (p *AgentDeploymentPolicy) DiagnosisConfigurationID() string {
	return p.diagnosisConfigurationID
}

func (p *AgentDeploymentPolicy) DiagnosisDecisionPolicyRevision() string {
	return knownDiagnosisConfigurations[p.diagnosisConfigurationID].DecisionPolicyRevision
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
