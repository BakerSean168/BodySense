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
}

func TestAgentDeploymentPolicyAllowsExplicitImmutableConfiguration(t *testing.T) {
	t.Setenv("DIAGNOSIS_AGENT_CONFIGURATION_ID", "diag-config-candidate1234")
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}
	if got := policy.DiagnosisConfigurationID(); got != "diag-config-candidate1234" {
		t.Fatalf("unexpected Diagnosis configuration: %s", got)
	}
}
