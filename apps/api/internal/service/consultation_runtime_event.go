package service

import (
	"encoding/json"
	"fmt"
)

type RuntimeProtocolErrorCode string

const (
	RuntimeProtocolDecodeFailed              RuntimeProtocolErrorCode = "RUNTIME_PROTOCOL_DECODE_FAILED"
	RuntimeProtocolValidationFailed          RuntimeProtocolErrorCode = "RUNTIME_PROTOCOL_VALIDATION_FAILED"
	RuntimeProtocolSequenceInvalid           RuntimeProtocolErrorCode = "RUNTIME_PROTOCOL_SEQUENCE_INVALID"
	RuntimeProtocolUnsupportedEvent          RuntimeProtocolErrorCode = "RUNTIME_PROTOCOL_UNSUPPORTED_EVENT"
	RuntimeProtocolPayloadMarshalFailed      RuntimeProtocolErrorCode = "RUNTIME_PROTOCOL_PAYLOAD_MARSHAL_FAILED"
	RuntimeProtocolApplicationPayloadInvalid RuntimeProtocolErrorCode = "RUNTIME_PROTOCOL_APPLICATION_PAYLOAD_INVALID"
)

type RuntimeProtocolError struct {
	Code  RuntimeProtocolErrorCode
	Cause error
}

func (e *RuntimeProtocolError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func (e *RuntimeProtocolError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func runtimeProtocolError(code RuntimeProtocolErrorCode, cause error) error {
	return &RuntimeProtocolError{Code: code, Cause: cause}
}

type ConsultationRuntimeEventKind string

const (
	ConsultationRuntimeAgentConfiguration ConsultationRuntimeEventKind = "agent_configuration"
	ConsultationRuntimeTextDelta          ConsultationRuntimeEventKind = "text_delta"
	ConsultationRuntimeToolCall           ConsultationRuntimeEventKind = "tool_call"
	ConsultationRuntimeToolResult         ConsultationRuntimeEventKind = "tool_result"
	ConsultationRuntimeExtractedInfo      ConsultationRuntimeEventKind = "extracted_info"
	ConsultationRuntimeLifestyleContext   ConsultationRuntimeEventKind = "lifestyle_context"
	ConsultationRuntimeInteraction        ConsultationRuntimeEventKind = "interaction_required"
	ConsultationRuntimePhaseChanged       ConsultationRuntimeEventKind = "phase_changed"
	ConsultationRuntimeCitationAdded      ConsultationRuntimeEventKind = "citation_added"
	ConsultationRuntimeAttributionAdded   ConsultationRuntimeEventKind = "answer_attribution_added"
	ConsultationRuntimeKnowledgeGap       ConsultationRuntimeEventKind = "knowledge_gap"
	ConsultationRuntimeRedFlagDetected    ConsultationRuntimeEventKind = "red_flag_detected"
	ConsultationRuntimeOutputReviewed     ConsultationRuntimeEventKind = "output_reviewed"
	ConsultationRuntimeOutputRejected     ConsultationRuntimeEventKind = "output_rejected"
	ConsultationRuntimeUsageReported      ConsultationRuntimeEventKind = "usage_reported"
	ConsultationRuntimeDone               ConsultationRuntimeEventKind = "done"
	ConsultationRuntimeError              ConsultationRuntimeEventKind = "error"
)

type ConsultationRuntimeEventIDs struct {
	ConversationID string
	RunID          string
	ToolCallID     string
	InteractionID  string
}

type ConsultationRuntimeEventPayload interface {
	Kind() ConsultationRuntimeEventKind
	isConsultationRuntimeEventPayload()
}

type consultationRuntimePayloadMarker struct{}

func (consultationRuntimePayloadMarker) isConsultationRuntimeEventPayload() {}

type ConsultationRuntimeAgentConfigurationPayload struct {
	consultationRuntimePayloadMarker
	AgentConfiguration  json.RawMessage `json:"agent_configuration"`
	ExecutionProvenance json.RawMessage `json:"execution_provenance"`
}

func (ConsultationRuntimeAgentConfigurationPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeAgentConfiguration
}

type ConsultationRuntimeTextDeltaPayload struct {
	consultationRuntimePayloadMarker
	Delta string `json:"delta"`
}

func (ConsultationRuntimeTextDeltaPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeTextDelta
}

type ConsultationRuntimeToolCallPayload struct {
	consultationRuntimePayloadMarker
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

func (ConsultationRuntimeToolCallPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeToolCall
}

type ConsultationRuntimeToolResultPayload struct {
	consultationRuntimePayloadMarker
	Tool   string          `json:"tool"`
	Result json.RawMessage `json:"result"`
}

func (ConsultationRuntimeToolResultPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeToolResult
}

type ConsultationRuntimeExtractedInfoPayload struct {
	consultationRuntimePayloadMarker
	Info json.RawMessage `json:"info"`
}

func (ConsultationRuntimeExtractedInfoPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeExtractedInfo
}

type ConsultationRuntimeLifestyleContextPayload struct {
	consultationRuntimePayloadMarker
	Context json.RawMessage `json:"context"`
}

func (ConsultationRuntimeLifestyleContextPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeLifestyleContext
}

type ConsultationRuntimeInteractionPayload struct {
	consultationRuntimePayloadMarker
	InteractionID string          `json:"interaction_id"`
	Question      json.RawMessage `json:"question"`
	// Suggestion is the typed source-cited BodyState suggestion (BS-AGENT-001)
	// as the binder's runtime envelope; empty when the interrupt carries none.
	Suggestion json.RawMessage `json:"-"`
}

func (ConsultationRuntimeInteractionPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeInteraction
}

type ConsultationRuntimePhaseChangedPayload struct {
	consultationRuntimePayloadMarker
	From   string `json:"from,omitempty"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

func (ConsultationRuntimePhaseChangedPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimePhaseChanged
}

type ConsultationRuntimeCitationAddedPayload struct {
	consultationRuntimePayloadMarker
	Citation json.RawMessage `json:"citation"`
}

func (ConsultationRuntimeCitationAddedPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeCitationAdded
}

type ConsultationRuntimeAttributionAddedPayload struct {
	consultationRuntimePayloadMarker
	Attribution json.RawMessage `json:"attribution"`
}

func (ConsultationRuntimeAttributionAddedPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeAttributionAdded
}

type ConsultationRuntimeKnowledgeGapPayload struct {
	consultationRuntimePayloadMarker
	Query   string `json:"query"`
	Message string `json:"message"`
}

func (ConsultationRuntimeKnowledgeGapPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeKnowledgeGap
}

type ConsultationRuntimeRedFlagDetectedPayload struct {
	consultationRuntimePayloadMarker
	HasRedFlags bool              `json:"has_red_flags"`
	Flags       []json.RawMessage `json:"flags"`
}

func (ConsultationRuntimeRedFlagDetectedPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeRedFlagDetected
}

type ConsultationRuntimeSafetyOutputPayload struct {
	consultationRuntimePayloadMarker
	EventKind      ConsultationRuntimeEventKind `json:"-"`
	OutputKind     string                       `json:"kind"`
	Verdict        string                       `json:"verdict"`
	Reasons        []string                     `json:"reasons"`
	Issues         json.RawMessage              `json:"issues,omitempty"`
	SafetyFallback string                       `json:"safety_fallback,omitempty"`
}

func (p ConsultationRuntimeSafetyOutputPayload) Kind() ConsultationRuntimeEventKind {
	return p.EventKind
}

type ConsultationRuntimeUsageReportedPayload struct {
	consultationRuntimePayloadMarker
	Usage json.RawMessage `json:"usage"`
}

func (ConsultationRuntimeUsageReportedPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeUsageReported
}

type ConsultationRuntimeDonePayload struct {
	consultationRuntimePayloadMarker
	ResponseID string          `json:"response_id,omitempty"`
	Usage      json.RawMessage `json:"usage,omitempty"`
	Governance json.RawMessage `json:"governance,omitempty"`
}

func (ConsultationRuntimeDonePayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeDone
}

type ConsultationRuntimeErrorPayload struct {
	consultationRuntimePayloadMarker
	Message string `json:"message"`
}

func (ConsultationRuntimeErrorPayload) Kind() ConsultationRuntimeEventKind {
	return ConsultationRuntimeError
}
