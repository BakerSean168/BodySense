package service

import (
	"encoding/json"
	"errors"
)

// DiagnosisReplayInput is the exact, frozen input of one formal Diagnosis
// analysis run, stored with the run so it can be replayed offline.
type DiagnosisReplayInput struct {
	BodyStateRevision int64             `json:"body_state_revision"`
	BodyState         json.RawMessage   `json:"body_state"`
	RelevantHistory   json.RawMessage   `json:"relevant_history"`
	Profile           json.RawMessage   `json:"profile"`
	SafetyEnvelope    *SafetyEnvelopeV2 `json:"safety_envelope,omitempty"`
}

func EncodeDiagnosisReplayInput(
	bodyStateRevision int64,
	bodyState json.RawMessage,
	relevantHistory json.RawMessage,
	profile json.RawMessage,
	safetyEnvelope *SafetyEnvelopeV2,
) (json.RawMessage, error) {
	if bodyStateRevision <= 0 || len(bodyState) == 0 || !json.Valid(bodyState) {
		return nil, errors.New("valid replay BodyState and revision are required")
	}
	if len(relevantHistory) == 0 {
		relevantHistory = json.RawMessage(`[]`)
	}
	if len(profile) == 0 {
		profile = json.RawMessage(`{}`)
	}
	if safetyEnvelope != nil && (safetyEnvelope.BodyStateRevision != bodyStateRevision || safetyEnvelope.SchemaRevision != SafetyEnvelopeSchemaV2 || safetyEnvelope.PolicyRevision != SafetyEnvelopePolicyV1) {
		return nil, errors.New("replay safety envelope does not match pinned BodyState revision or policy")
	}
	return json.Marshal(DiagnosisReplayInput{
		BodyStateRevision: bodyStateRevision,
		BodyState:         bodyState,
		RelevantHistory:   relevantHistory,
		Profile:           profile,
		SafetyEnvelope:    safetyEnvelope,
	})
}

// diagnosisAgentConfigurationMatches checks the Agent's self-reported identity
// against the configuration Go routed the request to.
func diagnosisAgentConfigurationMatches(payload map[string]any, expectedID string) bool {
	configuration, ok := payload["agent_configuration"].(map[string]any)
	if !ok {
		return false
	}
	id, idOK := configuration["id"].(string)
	role, roleOK := configuration["role"].(string)
	return idOK && roleOK && id == expectedID && role == "diagnosis"
}
