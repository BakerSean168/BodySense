package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type assessmentProvenance struct {
	AgentConfigurationID    string          `json:"id"`
	AgentConfiguration      json.RawMessage `json:"agent_configuration"`
	ExecutionProvenance     json.RawMessage `json:"execution_provenance"`
	ExecutionStatus         string
	agentConfigurationText  string
	executionProvenanceText string
}

func parseAssessmentProvenance(raw json.RawMessage) (assessmentProvenance, error) {
	var outer struct {
		AgentConfiguration  map[string]any `json:"agent_configuration"`
		ExecutionProvenance map[string]any `json:"execution_provenance"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return assessmentProvenance{}, fmt.Errorf("decode assessment provenance: %w", err)
	}
	configID, _ := outer.AgentConfiguration["id"].(string)
	executionStatus, _ := outer.ExecutionProvenance["status"].(string)
	configJSON, err := json.Marshal(outer.AgentConfiguration)
	if err != nil {
		return assessmentProvenance{}, fmt.Errorf("encode assessment agent configuration: %w", err)
	}
	execJSON, err := json.Marshal(outer.ExecutionProvenance)
	if err != nil {
		return assessmentProvenance{}, fmt.Errorf("encode assessment execution provenance: %w", err)
	}
	return assessmentProvenance{
		AgentConfigurationID:    configID,
		AgentConfiguration:      configJSON,
		ExecutionProvenance:     execJSON,
		ExecutionStatus:         executionStatus,
		agentConfigurationText:  string(configJSON),
		executionProvenanceText: string(execJSON),
	}, nil
}

func validateAssessmentAgentIdentity(
	prov assessmentProvenance,
	expectedConfigurationID string,
	expectedPolicyRevision string,
) error {
	if prov.AgentConfigurationID != expectedConfigurationID {
		return fmt.Errorf(
			"Assessment agent configuration mismatch: got %q want %q",
			prov.AgentConfigurationID,
			expectedConfigurationID,
		)
	}
	if strings.TrimSpace(prov.agentConfigurationText) == "" || strings.TrimSpace(prov.executionProvenanceText) == "" {
		return errors.New("Assessment response missing agent configuration or execution provenance")
	}
	if expectedPolicyRevision != "" {
		registration, ok := knownAssessmentConfigurations[expectedConfigurationID]
		if !ok || registration.DecisionPolicyRevision != expectedPolicyRevision {
			return errors.New("Assessment deployment policy revision not registered for configuration")
		}
	}
	return nil
}

func buildAssessmentGenerationTrace(
	prov assessmentProvenance,
	configurationID string,
	policyRevision string,
	replayFingerprint string,
	contractRevision string,
	evidencePolicyRevision string,
) json.RawMessage {
	modelExecuted := prov.ExecutionStatus == "executed"
	status := "generated"
	phase := "generation"
	if !modelExecuted && prov.ExecutionStatus == "skipped_no_evidence" {
		status = "derived_without_model"
		phase = "deterministic_derivation"
	}
	trace := map[string]any{
		"status":                   status,
		"phase":                    phase,
		"agent_configuration_id":   configurationID,
		"decision_policy_revision": policyRevision,
		"evaluated":                prov.AgentConfigurationID != "",
		"model_executed":           modelExecuted,
		"execution_status":         prov.ExecutionStatus,
		"outcome":                  "accepted",
		"replay_input_fingerprint": replayFingerprint,
		"contract_revision":        contractRevision,
		"evidence_policy_revision": evidencePolicyRevision,
	}
	encoded, err := json.Marshal(trace)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
