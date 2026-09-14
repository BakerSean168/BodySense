package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	protovalidate "buf.build/go/protovalidate"
	runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

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

// ConsultationRuntimeEvent is an application-facing private runtime fact. It is
// intentionally distinct from the public dto.StreamEvent vocabulary. Generated
// Proto messages are validated and projected into this type inside AIClient.
type ConsultationRuntimeEvent struct {
	Seq     int
	Kind    ConsultationRuntimeEventKind
	IDs     ConsultationRuntimeEventIDs
	Payload json.RawMessage
}

func (e ConsultationRuntimeEvent) PayloadAs(target any) error {
	payload := e.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return json.Unmarshal(payload, target)
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

func validateConsultationRuntimeApplicationPayload(event ConsultationRuntimeEvent) error {
	switch event.Kind {
	case ConsultationRuntimeCitationAdded:
		var payload struct {
			Citation json.RawMessage `json:"citation"`
		}
		if err := event.PayloadAs(&payload); err != nil || len(payload.Citation) == 0 {
			return fmt.Errorf("citation payload is malformed")
		}
		return validateCitationPayload(payload.Citation)
	case ConsultationRuntimeAttributionAdded:
		if _, err := ParseConsultationAnswerAttributionPayload(event.Payload); err != nil {
			return err
		}
	}
	return nil
}

func decodeConsultationRuntimeProtoEvent(line []byte) (ConsultationRuntimeEvent, error) {
	var wire runtimev1.RuntimeEvent
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(line, &wire); err != nil {
		return ConsultationRuntimeEvent{}, fmt.Errorf("decode runtime Proto event: %w", err)
	}
	if err := protovalidate.Validate(&wire); err != nil {
		return ConsultationRuntimeEvent{}, fmt.Errorf("validate runtime Proto event: %w", err)
	}
	if wire.Seq > uint64(math.MaxInt) {
		return ConsultationRuntimeEvent{}, fmt.Errorf("runtime event sequence exceeds platform int")
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

	kind, payload, err := runtimeEventOneofPayload(&wire)
	if err != nil {
		return ConsultationRuntimeEvent{}, err
	}
	event := ConsultationRuntimeEvent{Seq: int(wire.Seq), Kind: kind, IDs: ids, Payload: payload}
	if err := validateConsultationRuntimeApplicationPayload(event); err != nil {
		return ConsultationRuntimeEvent{}, fmt.Errorf("validate runtime application payload: %w", err)
	}
	return event, nil
}

func runtimeEventOneofPayload(event *runtimev1.RuntimeEvent) (ConsultationRuntimeEventKind, json.RawMessage, error) {
	var kind ConsultationRuntimeEventKind
	var message proto.Message
	switch value := event.Event.(type) {
	case *runtimev1.RuntimeEvent_AgentConfiguration:
		kind, message = ConsultationRuntimeAgentConfiguration, value.AgentConfiguration
	case *runtimev1.RuntimeEvent_TextDelta:
		kind, message = ConsultationRuntimeTextDelta, value.TextDelta
	case *runtimev1.RuntimeEvent_ToolCall:
		kind, message = ConsultationRuntimeToolCall, value.ToolCall
	case *runtimev1.RuntimeEvent_ToolResult:
		kind, message = ConsultationRuntimeToolResult, value.ToolResult
	case *runtimev1.RuntimeEvent_ExtractedInfo:
		kind, message = ConsultationRuntimeExtractedInfo, value.ExtractedInfo
	case *runtimev1.RuntimeEvent_LifestyleContext:
		kind, message = ConsultationRuntimeLifestyleContext, value.LifestyleContext
	case *runtimev1.RuntimeEvent_InteractionRequired:
		kind, message = ConsultationRuntimeInteraction, value.InteractionRequired
	case *runtimev1.RuntimeEvent_PhaseChanged:
		kind, message = ConsultationRuntimePhaseChanged, value.PhaseChanged
	case *runtimev1.RuntimeEvent_CitationAdded:
		kind, message = ConsultationRuntimeCitationAdded, value.CitationAdded
	case *runtimev1.RuntimeEvent_AnswerAttributionAdded:
		kind, message = ConsultationRuntimeAttributionAdded, value.AnswerAttributionAdded
	case *runtimev1.RuntimeEvent_KnowledgeGap:
		kind, message = ConsultationRuntimeKnowledgeGap, value.KnowledgeGap
	case *runtimev1.RuntimeEvent_RedFlagDetected:
		kind, message = ConsultationRuntimeRedFlagDetected, value.RedFlagDetected
	case *runtimev1.RuntimeEvent_OutputReviewed:
		kind, message = ConsultationRuntimeOutputReviewed, value.OutputReviewed
	case *runtimev1.RuntimeEvent_OutputRejected:
		kind, message = ConsultationRuntimeOutputRejected, value.OutputRejected
	case *runtimev1.RuntimeEvent_UsageReported:
		kind, message = ConsultationRuntimeUsageReported, value.UsageReported
	case *runtimev1.RuntimeEvent_StreamDone:
		kind, message = ConsultationRuntimeDone, value.StreamDone
	case *runtimev1.RuntimeEvent_StreamError:
		kind, message = ConsultationRuntimeError, value.StreamError
	default:
		return "", nil, fmt.Errorf("runtime Proto event has no supported oneof variant")
	}
	payload, err := runtimeProtoPayloadJSON.Marshal(message)
	if err != nil {
		return "", nil, fmt.Errorf("marshal runtime event payload: %w", err)
	}
	return kind, json.RawMessage(payload), nil
}

func consultationProtocolError(message string) ConsultationRuntimeEvent {
	payload, _ := json.Marshal(map[string]string{"message": message})
	return ConsultationRuntimeEvent{Seq: 1, Kind: ConsultationRuntimeError, Payload: payload}
}
