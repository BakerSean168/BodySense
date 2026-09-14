package service

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	protovalidate "buf.build/go/protovalidate"
	runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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

// ConsultationRuntimeEvent is an application-facing private runtime fact. It is
// intentionally distinct from the public dto.StreamEvent vocabulary. Generated
// Proto messages are validated and projected into one handwritten typed payload
// variant inside the AI boundary; the application never re-parses the wire shape.
type ConsultationRuntimeEvent struct {
	Seq     int
	IDs     ConsultationRuntimeEventIDs
	Payload ConsultationRuntimeEventPayload
}

func (e ConsultationRuntimeEvent) Kind() ConsultationRuntimeEventKind {
	if e.Payload == nil {
		return ""
	}
	return e.Payload.Kind()
}

func validateCitationPayload(raw json.RawMessage) error {
	var citation struct {
		SourceType          string `json:"source_type"`
		UnitKey             string `json:"unit_key"`
		SourceKey           string `json:"source_key"`
		LifecycleStatus     string `json:"lifecycle_status"`
		PublicationID       string `json:"publication_id"`
		PublicationKey      string `json:"publication_key"`
		PublicationBatchKey string `json:"publication_batch_key"`
		PublishedVersion    int    `json:"published_version"`
		ClaimID             string `json:"claim_id"`
		ClaimReviewID       string `json:"claim_review_id"`
		SourceLocator       struct {
			LocatorType string `json:"locator_type"`
			GitCommit   string `json:"git_commit"`
			Path        string `json:"path"`
			LineStart   int    `json:"line_start"`
			LineEnd     int    `json:"line_end"`
		} `json:"source_locator"`
	}
	if err := json.Unmarshal(raw, &citation); err != nil {
		return fmt.Errorf("citation payload is malformed")
	}
	if citation.SourceType != "thought_forest_note" {
		return nil
	}
	if strings.TrimSpace(citation.UnitKey) == "" || strings.TrimSpace(citation.SourceKey) == "" ||
		citation.LifecycleStatus != "published" || strings.TrimSpace(citation.PublicationID) == "" ||
		citation.PublishedVersion <= 0 || strings.TrimSpace(citation.PublicationKey) == "" ||
		strings.TrimSpace(citation.PublicationBatchKey) == "" || strings.TrimSpace(citation.ClaimID) == "" ||
		strings.TrimSpace(citation.ClaimReviewID) == "" {
		return fmt.Errorf("published Thought Forest citation identity is incomplete")
	}
	locator := citation.SourceLocator
	if locator.LocatorType != "markdown_lines" || strings.TrimSpace(locator.GitCommit) == "" ||
		strings.TrimSpace(locator.Path) == "" || locator.LineStart <= 0 || locator.LineEnd < locator.LineStart {
		return fmt.Errorf("published Thought Forest citation provenance is incomplete")
	}
	return nil
}

func marshalRuntimeOpaque(message proto.Message) (json.RawMessage, error) {
	if message == nil {
		return nil, nil
	}
	value := reflect.ValueOf(message)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return nil, nil
	}
	payload, err := runtimeProtoPayloadJSON.Marshal(message)
	if err != nil {
		return nil, runtimeProtocolError(RuntimeProtocolPayloadMarshalFailed, err)
	}
	return json.RawMessage(payload), nil
}

func validateConsultationRuntimeApplicationPayload(payload ConsultationRuntimeEventPayload) error {
	switch value := payload.(type) {
	case ConsultationRuntimeCitationAddedPayload:
		if len(value.Citation) == 0 {
			return fmt.Errorf("citation payload is malformed")
		}
		return validateCitationPayload(value.Citation)
	case ConsultationRuntimeAttributionAddedPayload:
		wrapper, err := json.Marshal(struct {
			Attribution json.RawMessage `json:"attribution"`
		}{Attribution: value.Attribution})
		if err != nil {
			return err
		}
		if _, err := ParseConsultationAnswerAttributionPayload(wrapper); err != nil {
			return err
		}
	}
	return nil
}

func decodeConsultationRuntimeProtoEvent(line []byte) (ConsultationRuntimeEvent, error) {
	var wire runtimev1.RuntimeEvent
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(line, &wire); err != nil {
		return ConsultationRuntimeEvent{}, runtimeProtocolError(RuntimeProtocolDecodeFailed, err)
	}
	if err := protovalidate.Validate(&wire); err != nil {
		return ConsultationRuntimeEvent{}, runtimeProtocolError(RuntimeProtocolValidationFailed, err)
	}
	if wire.Seq > uint64(math.MaxInt) {
		return ConsultationRuntimeEvent{}, runtimeProtocolError(RuntimeProtocolSequenceInvalid, fmt.Errorf("runtime event sequence exceeds platform int"))
	}

	ids := ConsultationRuntimeEventIDs{}
	if wire.Ids != nil {
		ids.ConversationID = wire.Ids.ConversationId
		ids.RunID = wire.Ids.RunId
		if wire.Ids.ToolCallId != nil {
			ids.ToolCallID = *wire.Ids.ToolCallId
		}
		if wire.Ids.InteractionId != nil {
			ids.InteractionID = *wire.Ids.InteractionId
		}
	}

	payload, err := runtimeEventOneofPayload(&wire)
	if err != nil {
		return ConsultationRuntimeEvent{}, err
	}
	if err := validateConsultationRuntimeApplicationPayload(payload); err != nil {
		return ConsultationRuntimeEvent{}, runtimeProtocolError(RuntimeProtocolApplicationPayloadInvalid, err)
	}
	return ConsultationRuntimeEvent{Seq: int(wire.Seq), IDs: ids, Payload: payload}, nil
}

func runtimeEventOneofPayload(event *runtimev1.RuntimeEvent) (ConsultationRuntimeEventPayload, error) {
	switch value := event.Event.(type) {
	case *runtimev1.RuntimeEvent_AgentConfiguration:
		configuration, err := marshalRuntimeOpaque(value.AgentConfiguration.AgentConfiguration)
		if err != nil {
			return nil, err
		}
		provenance, err := marshalRuntimeOpaque(value.AgentConfiguration.ExecutionProvenance)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeAgentConfigurationPayload{
			AgentConfiguration:  configuration,
			ExecutionProvenance: provenance,
		}, nil
	case *runtimev1.RuntimeEvent_TextDelta:
		return ConsultationRuntimeTextDeltaPayload{Delta: value.TextDelta.GetDelta()}, nil
	case *runtimev1.RuntimeEvent_ToolCall:
		args, err := marshalRuntimeOpaque(value.ToolCall.Args)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeToolCallPayload{Tool: value.ToolCall.Tool, Args: args}, nil
	case *runtimev1.RuntimeEvent_ToolResult:
		result, err := marshalRuntimeOpaque(value.ToolResult.Result)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeToolResultPayload{Tool: value.ToolResult.Tool, Result: result}, nil
	case *runtimev1.RuntimeEvent_ExtractedInfo:
		info, err := marshalRuntimeOpaque(value.ExtractedInfo.Info)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeExtractedInfoPayload{Info: info}, nil
	case *runtimev1.RuntimeEvent_LifestyleContext:
		contextPayload, err := marshalRuntimeOpaque(value.LifestyleContext.Context)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeLifestyleContextPayload{Context: contextPayload}, nil
	case *runtimev1.RuntimeEvent_InteractionRequired:
		question, err := marshalRuntimeOpaque(value.InteractionRequired.Question)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeInteractionPayload{
			InteractionID: value.InteractionRequired.InteractionId,
			Question:      question,
		}, nil
	case *runtimev1.RuntimeEvent_PhaseChanged:
		return ConsultationRuntimePhaseChangedPayload{
			From:   value.PhaseChanged.GetFrom(),
			To:     value.PhaseChanged.To,
			Reason: value.PhaseChanged.Reason,
		}, nil
	case *runtimev1.RuntimeEvent_CitationAdded:
		citation, err := marshalRuntimeOpaque(value.CitationAdded.Citation)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeCitationAddedPayload{Citation: citation}, nil
	case *runtimev1.RuntimeEvent_AnswerAttributionAdded:
		attribution, err := marshalRuntimeOpaque(value.AnswerAttributionAdded.Attribution)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeAttributionAddedPayload{Attribution: attribution}, nil
	case *runtimev1.RuntimeEvent_KnowledgeGap:
		return ConsultationRuntimeKnowledgeGapPayload{
			Query:   value.KnowledgeGap.Query,
			Message: value.KnowledgeGap.Message,
		}, nil
	case *runtimev1.RuntimeEvent_RedFlagDetected:
		flags := make([]json.RawMessage, 0, len(value.RedFlagDetected.Flags))
		for _, flag := range value.RedFlagDetected.Flags {
			raw, err := marshalRuntimeOpaque(flag)
			if err != nil {
				return nil, err
			}
			flags = append(flags, raw)
		}
		return ConsultationRuntimeRedFlagDetectedPayload{
			HasRedFlags: value.RedFlagDetected.GetHasRedFlags(),
			Flags:       flags,
		}, nil
	case *runtimev1.RuntimeEvent_OutputReviewed:
		return runtimeSafetyOutputPayload(ConsultationRuntimeOutputReviewed, value.OutputReviewed)
	case *runtimev1.RuntimeEvent_OutputRejected:
		return runtimeSafetyOutputPayload(ConsultationRuntimeOutputRejected, value.OutputRejected)
	case *runtimev1.RuntimeEvent_UsageReported:
		usage, err := marshalRuntimeOpaque(value.UsageReported.Usage)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeUsageReportedPayload{Usage: usage}, nil
	case *runtimev1.RuntimeEvent_StreamDone:
		usage, err := marshalRuntimeOpaque(value.StreamDone.Usage)
		if err != nil {
			return nil, err
		}
		governance, err := marshalRuntimeOpaque(value.StreamDone.Governance)
		if err != nil {
			return nil, err
		}
		return ConsultationRuntimeDonePayload{
			ResponseID: value.StreamDone.GetResponseId(),
			Usage:      usage,
			Governance: governance,
		}, nil
	case *runtimev1.RuntimeEvent_StreamError:
		return ConsultationRuntimeErrorPayload{Message: value.StreamError.Message}, nil
	default:
		return nil, runtimeProtocolError(RuntimeProtocolUnsupportedEvent, fmt.Errorf("runtime Proto event has no supported oneof variant"))
	}
}

func runtimeSafetyOutputPayload(
	kind ConsultationRuntimeEventKind,
	value *runtimev1.SafetyOutputEvent,
) (ConsultationRuntimeEventPayload, error) {
	issues, err := marshalRuntimeOpaque(value.Issues)
	if err != nil {
		return nil, err
	}
	reasons := append([]string{}, value.Reasons...)
	return ConsultationRuntimeSafetyOutputPayload{
		EventKind:      kind,
		OutputKind:     value.Kind,
		Verdict:        value.Verdict,
		Reasons:        reasons,
		Issues:         issues,
		SafetyFallback: value.GetSafetyFallback(),
	}, nil
}

func consultationProtocolError(message string) ConsultationRuntimeEvent {
	return ConsultationRuntimeEvent{Seq: 1, Payload: ConsultationRuntimeErrorPayload{Message: message}}
}
