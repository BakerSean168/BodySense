package service

import "testing"

func TestAgentDeploymentPolicyDefaultsToQualifiedDiagnosisConfiguration(t *testing.T) {
	t.Setenv("DIAGNOSIS_AGENT_CONFIGURATION_ID", "")
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}
	if got := policy.DiagnosisConfigurationID(); got != defaultDiagnosisConfigurationID {
		t.Fatalf("unexpected Diagnosis configuration: %s", got)
	}
	if got := policy.DiagnosisDecisionPolicyRevision(); got != DiagnosisDecisionPolicyPreEnvelope {
		t.Fatalf("unexpected default decision policy: %s", got)
	}
}

func TestAgentDeploymentPolicyAllowsOnlyRepositoryKnownImmutableConfigurations(t *testing.T) {
	t.Setenv("DIAGNOSIS_AGENT_CONFIGURATION_ID", diagnosisDecisionAuthorityConfigID)
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}
	if got := policy.DiagnosisConfigurationID(); got != diagnosisDecisionAuthorityConfigID {
		t.Fatalf("unexpected Diagnosis configuration: %s", got)
	}
	if got := policy.DiagnosisDecisionPolicyRevision(); got != DiagnosisDecisionPolicyV1 {
		t.Fatalf("unexpected v3 decision policy: %s", got)
	}
}

func TestAgentDeploymentPolicyRejectsUnknownConfiguration(t *testing.T) {
	t.Setenv("DIAGNOSIS_AGENT_CONFIGURATION_ID", "diag-config-candidate1234")
	if _, err := NewAgentDeploymentPolicy(); err == nil {
		t.Fatal("unknown immutable configuration must fail closed")
	}
}
