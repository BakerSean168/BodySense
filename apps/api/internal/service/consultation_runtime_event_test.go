package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDecodeConsultationRuntimeProtoEventProjectsTypedOneof(t *testing.T) {
	line := []byte(`{"version":1,"seq":"7","ids":{"conversation_id":"33333333-3333-4333-8333-333333333333","run_id":"22222222-2222-4222-8222-222222222222","tool_call_id":"tool-1"},"tool_call":{"tool":"lookup","args":{"query":"neck"}}}`)
	event, err := decodeConsultationRuntimeProtoEvent(line)
	if err != nil {
		t.Fatal(err)
	}
	if event.Kind() != ConsultationRuntimeToolCall || event.Seq != 7 || event.IDs.ToolCallID != "tool-1" {
		t.Fatalf("unexpected private event: %#v", event)
	}
	payload, ok := event.Payload.(ConsultationRuntimeToolCallPayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", event.Payload)
	}
	var args map[string]any
	if err := json.Unmarshal(payload.Args, &args); err != nil {
		t.Fatal(err)
	}
	if payload.Tool != "lookup" || args["query"] != "neck" {
		t.Fatalf("unexpected tool payload: %#v", payload)
	}
}

func TestDecodeConsultationRuntimeProtoEventPreservesDefaultValuedFactsWithoutNullMessages(t *testing.T) {
	redFlagLine := []byte(`{"version":1,"seq":"1","ids":{"conversation_id":"33333333-3333-4333-8333-333333333333","run_id":"22222222-2222-4222-8222-222222222222"},"red_flag_detected":{"has_red_flags":false,"flags":[]}}`)
	redFlag, err := decodeConsultationRuntimeProtoEvent(redFlagLine)
	if err != nil {
		t.Fatal(err)
	}
	redPayload, ok := redFlag.Payload.(ConsultationRuntimeRedFlagDetectedPayload)
	if !ok {
		t.Fatalf("unexpected red-flag payload type %T", redFlag.Payload)
	}
	if redPayload.HasRedFlags {
		t.Fatalf("false red-flag fact was lost: %#v", redPayload)
	}
	if redPayload.Flags == nil || len(redPayload.Flags) != 0 {
		t.Fatalf("empty red-flag list was not preserved: %#v", redPayload)
	}

	doneLine := []byte(`{"version":1,"seq":"2","ids":{"conversation_id":"33333333-3333-4333-8333-333333333333","run_id":"22222222-2222-4222-8222-222222222222"},"stream_done":{}}`)
	done, err := decodeConsultationRuntimeProtoEvent(doneLine)
	if err != nil {
		t.Fatal(err)
	}
	donePayload, ok := done.Payload.(ConsultationRuntimeDonePayload)
	if !ok {
		t.Fatalf("unexpected stream.done payload type %T", done.Payload)
	}
	if donePayload.ResponseID != "" || len(donePayload.Usage) != 0 || len(donePayload.Governance) != 0 {
		t.Fatalf("absent stream.done fields must remain absent, got %#v", donePayload)
	}
}

func TestDecodeConsultationRuntimeProtoEventPreservesNestedIntakeProvenance(t *testing.T) {
	line := []byte(`{"version":1,"seq":"1","ids":{"conversation_id":"33333333-3333-4333-8333-333333333333","run_id":"22222222-2222-4222-8222-222222222222"},"agent_configuration":{"agent_configuration":{"id":"consult-config-0123456789abcdef","role":"consultation","manifest_revision":"v2","logical_model":"bodysense-consultation","model_group_revision":"models-v1","prompt_revision":"prompt-v2","tool_policy_revision":"tools-v2","governance_policy_revision":"gov-v1","decision_policy_revision":"decision-v2","intake":{"logical_model":"bodysense-structured","model_group_revision":"intake-model-v1","prompt_revision":"intake-prompt-v1","output_schema_revision":"intake-output-v1","policy_revision":"intake-policy-v1","generation":{"temperature":0.1,"max_tokens":1200}}},"execution_provenance":{"status":"executed","runtime":"langgraph","logical_model":"bodysense-consultation","model_group_revision":"models-v1","usage":{}}}}`)
	event, err := decodeConsultationRuntimeProtoEvent(line)
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := event.Payload.(ConsultationRuntimeAgentConfigurationPayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", event.Payload)
	}
	if !strings.Contains(string(payload.AgentConfiguration), `"output_schema_revision":"intake-output-v1"`) || !strings.Contains(string(payload.AgentConfiguration), `"max_tokens":1200`) {
		t.Fatalf("nested intake provenance was lost: %s", payload.AgentConfiguration)
	}
}

func TestDecodeConsultationRuntimeProtoEventRejectsLegacyGenericWire(t *testing.T) {
	line := []byte(`{"version":1,"seq":1,"channel":"message","type":"message.text.delta","ids":{"conversation_id":"33333333-3333-4333-8333-333333333333","run_id":"22222222-2222-4222-8222-222222222222"},"payload":{"delta":"legacy"}}`)
	_, err := decodeConsultationRuntimeProtoEvent(line)
	if err == nil {
		t.Fatal("legacy channel/type/payload wire must not be accepted after Proto cutover")
	}
	var protocolErr *RuntimeProtocolError
	if !errors.As(err, &protocolErr) {
		t.Fatalf("expected RuntimeProtocolError, got %T: %v", err, err)
	}
	if protocolErr.Code != RuntimeProtocolDecodeFailed {
		t.Fatalf("unexpected protocol error code: %s", protocolErr.Code)
	}
}

func TestDecodeConsultationRuntimeProtoEventClassifiesApplicationPayloadFailure(t *testing.T) {
	line := []byte(`{"version":1,"seq":"1","ids":{"conversation_id":"33333333-3333-4333-8333-333333333333","run_id":"22222222-2222-4222-8222-222222222222"},"citation_added":{"citation":{"source_type":"thought_forest_note","title":"Incomplete citation"}}}`)
	_, err := decodeConsultationRuntimeProtoEvent(line)
	if err == nil {
		t.Fatal("incomplete Thought Forest citation must fail application payload validation")
	}
	var protocolErr *RuntimeProtocolError
	if !errors.As(err, &protocolErr) {
		t.Fatalf("expected RuntimeProtocolError, got %T: %v", err, err)
	}
	if protocolErr.Code != RuntimeProtocolApplicationPayloadInvalid {
		t.Fatalf("unexpected protocol error code: %s", protocolErr.Code)
	}
}
