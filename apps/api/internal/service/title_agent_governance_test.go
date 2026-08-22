package service

import (
	"encoding/json"
	"testing"
)

func TestValidateTitleAgentResponseAcceptsExactIdentity(t *testing.T) {
	response := &TitleGenerateResponse{
		Title: "颈肩不适咨询",
		AgentConfiguration: map[string]any{
			"id":                       defaultTitleConfigurationID,
			"role":                     "title",
			"decision_policy_revision": TitleDecisionPolicyV1,
			"logical_model":            titleLogicalModelV1,
		},
		ExecutionProvenance: map[string]any{
			"status":        "executed",
			"logical_model": titleLogicalModelV1,
		},
	}
	title, configuration, provenance, trace, err := validateTitleAgentResponse(response, defaultTitleConfigurationID)
	if err != nil {
		t.Fatal(err)
	}
	if title != response.Title || !json.Valid(configuration) || !json.Valid(provenance) || !json.Valid(trace) {
		t.Fatalf("unexpected validated title payload")
	}
	var decision map[string]any
	if err := json.Unmarshal(trace, &decision); err != nil {
		t.Fatal(err)
	}
	if decision["authority"] != "go" || decision["decision"] != "persist" {
		t.Fatalf("unexpected title decision trace: %#v", decision)
	}
}

func TestValidateTitleAgentResponseRejectsConfigurationMismatch(t *testing.T) {
	response := &TitleGenerateResponse{
		Title: "bad",
		AgentConfiguration: map[string]any{
			"id":                       "title-config-wrong",
			"role":                     "title",
			"decision_policy_revision": TitleDecisionPolicyV1,
			"logical_model":            titleLogicalModelV1,
		},
		ExecutionProvenance: map[string]any{"status": "executed", "logical_model": titleLogicalModelV1},
	}
	if _, _, _, _, err := validateTitleAgentResponse(response, defaultTitleConfigurationID); err == nil {
		t.Fatal("expected title Agent identity mismatch")
	}
}
