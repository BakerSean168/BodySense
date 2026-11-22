package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type streamState struct {
	UID                     uuid.UUID
	ConversationID          uuid.UUID
	TurnID                  uuid.UUID
	Run                     *model.Run
	AssistantMsg            *model.Message
	BaseIDs                 dto.StreamEventIDs
	CurrentPhase            string
	AssistantMsgID          string
	ConversationStr         string
	ExpectedConfigurationID string
	AnswerSource            *model.AnswerFactSource
}

type ConsultationExecutionIdentity struct {
	ConfigurationID     string
	AgentConfiguration  json.RawMessage
	ExecutionProvenance json.RawMessage
}

type streamResult struct {
	AssistantParts     []map[string]any
	Usage              any
	ProviderResponseID string
	GovernanceResult   datatypes.JSON
	ExecutionIdentity  ConsultationExecutionIdentity
	// CommittedFacts are the BodyFacts Go committed in this stream; a symptom
	// intake question may bind only to one of them.
	CommittedFacts []service.CommittedBodyFact
}

func projectConsultationRuntimeEvent(event service.ConsultationRuntimeEvent) (dto.StreamEvent, error) {
	var channel, eventType string
	switch event.Kind() {
	case service.ConsultationRuntimeTextDelta:
		channel, eventType = "message", "message.text.delta"
	case service.ConsultationRuntimeToolCall:
		channel, eventType = "tool", "tool.call"
	case service.ConsultationRuntimeToolResult:
		channel, eventType = "tool", "tool.result"
	case service.ConsultationRuntimeExtractedInfo:
		channel, eventType = "state", "state.extracted_info.upsert"
	case service.ConsultationRuntimeLifestyleContext:
		channel, eventType = "state", "state.lifestyle_context.upsert"
	case service.ConsultationRuntimeInteraction:
		channel, eventType = "state", "state.interaction.required"
	case service.ConsultationRuntimePhaseChanged:
		channel, eventType = "state", "state.phase.changed"
	case service.ConsultationRuntimeCitationAdded:
		channel, eventType = "source", "source.citation.added"
	case service.ConsultationRuntimeAttributionAdded:
		channel, eventType = "source", "source.answer_attribution.added"
	case service.ConsultationRuntimeKnowledgeGap:
		channel, eventType = "source", "source.knowledge_gap"
	case service.ConsultationRuntimeRedFlagDetected:
		channel, eventType = "safety", "safety.red_flag.detected"
	case service.ConsultationRuntimeOutputReviewed:
		channel, eventType = "safety", "safety.output_reviewed"
	case service.ConsultationRuntimeOutputRejected:
		channel, eventType = "safety", "safety.output_rejected"
	case service.ConsultationRuntimeUsageReported:
		channel, eventType = "usage", "usage.reported"
	case service.ConsultationRuntimeDone:
		channel, eventType = "stream", "stream.done"
	case service.ConsultationRuntimeError:
		channel, eventType = "stream", "stream.error"
	case service.ConsultationRuntimeAgentConfiguration:
		return dto.StreamEvent{}, errors.New("runtime Agent configuration is private control-plane state")
	default:
		return dto.StreamEvent{}, fmt.Errorf("unsupported internal runtime event kind %q", event.Kind())
	}

	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return dto.StreamEvent{}, fmt.Errorf("marshal trusted runtime payload: %w", err)
	}
	return dto.StreamEvent{
		Version: 1,
		Seq:     event.Seq,
		Channel: channel,
		Type:    eventType,
		IDs: dto.StreamEventIDs{
			ConversationID: event.IDs.ConversationID,
			RunID:          event.IDs.RunID,
			ToolCallID:     event.IDs.ToolCallID,
		},
		Payload: payload,
	}, nil
}

func (r *Runtime) streamAIEvents(
	ctx context.Context,
	sw *stream.StreamWriter,
	events <-chan service.ConsultationRuntimeEvent,
	state streamState,
) (streamResult, bool) {
	result := streamResult{}
	eventCount := 0
	phase := state.CurrentPhase
	handshakeAccepted := false

	for {
		select {
		case <-ctx.Done():
			r.handleExecutionContextDone(ctx, sw, state)
			return result, true
		case event, ok := <-events:
			// If explicit cancellation raced with a ready producer event, prefer
			// the business terminal state over accepting one more semantic event.
			// A select may choose either ready case, so re-check the context here.
			if ctx.Err() != nil {
				r.handleExecutionContextDone(ctx, sw, state)
				return result, true
			}
			if !ok {
				if !handshakeAccepted {
					r.failActiveStream(ctx, sw, state, "missing runtime Agent configuration handshake")
					return result, true
				}
				return result, false
			}

			eventCount++
			if eventCount > maxSSEEvents {
				slog.Default().Warn("event_count_exceeded_limit", "conversation_id", state.ConversationID)
				r.failActiveStream(ctx, sw, state, "stream exceeded maximum event count")
				return result, true
			}

			// The immutable execution identity is a control-plane handshake, not
			// ordinary semantic output. Nothing else is trusted until it is the
			// first event and has been validated/persisted by Go.
			if !handshakeAccepted && event.Kind() != service.ConsultationRuntimeAgentConfiguration {
				r.failActiveStream(ctx, sw, state, "runtime Agent configuration handshake must be the first event")
				return result, true
			}
			if handshakeAccepted && event.Kind() == service.ConsultationRuntimeAgentConfiguration {
				r.failActiveStream(ctx, sw, state, "duplicate runtime Agent configuration handshake")
				return result, true
			}

			if r.handleAIEvent(ctx, sw, event, state, &result, &phase) {
				return result, true
			}
			if event.Kind() == service.ConsultationRuntimeAgentConfiguration {
				handshakeAccepted = result.ExecutionIdentity.ConfigurationID != ""
				if !handshakeAccepted {
					r.failActiveStream(ctx, sw, state, "invalid runtime Agent configuration handshake")
					return result, true
				}
			}
		}
	}
}

func storedRuntimeEventToStreamEvent(stored model.RuntimeEvent) (dto.StreamEvent, error) {
	var ids dto.StreamEventIDs
	if len(stored.IDs) > 0 {
		if err := json.Unmarshal(stored.IDs, &ids); err != nil {
			return dto.StreamEvent{}, err
		}
	}
	payload := json.RawMessage(stored.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return dto.StreamEvent{
		Version: 1,
		Seq:     stored.Seq,
		Channel: stored.Channel,
		Type:    stored.Type,
		IDs:     ids,
		Payload: payload,
	}, nil
}

func (r *Runtime) sendNewEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	channel string,
	eventType string,
	ids dto.StreamEventIDs,
	payload any,
	messageID string,
	label string,
) {
	event, err := sw.NewEvent(channel, eventType, ids, payload, messageID)
	if err != nil {
		slog.Default().Error("sse_build_error", "label", label, "error", err)
		return
	}
	r.recordPublicEvent(ctx, event)
	if err := sw.WriteEvent(ctx, event); err != nil {
		slog.Default().Warn("sse_write_error", "event_type", label, "run_id", event.IDs.RunID, "conversation_id", event.IDs.ConversationID, "seq", event.Seq, "error", err)
	}
}

func (r *Runtime) sendEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	event dto.StreamEvent,
	messageID string,
	label string,
) {
	enriched := sw.EnrichEvent(event, messageID)
	r.recordPublicEvent(ctx, enriched)
	if err := sw.WriteEvent(ctx, enriched); err != nil {
		slog.Default().Warn("sse_write_error", "event_type", label, "run_id", enriched.IDs.RunID, "conversation_id", enriched.IDs.ConversationID, "seq", enriched.Seq, "error", err)
	}
}

func (r *Runtime) recordPublicEvent(ctx context.Context, event dto.StreamEvent) {
	if r.runtimeEventService == nil {
		return
	}

	conversationID, err := uuid.Parse(event.IDs.ConversationID)
	if err != nil {
		return
	}

	runID, err := uuid.Parse(event.IDs.RunID)
	if err != nil {
		return
	}

	var turnID *uuid.UUID
	if event.IDs.TurnID != "" {
		if parsedTurnID, parseErr := uuid.Parse(event.IDs.TurnID); parseErr == nil {
			turnID = &parsedTurnID
		}
	}

	if err := r.runtimeEventService.RecordPublicEvent(ctx, conversationID, runID, turnID, event); err != nil {
		slog.Default().Error("persist_runtime_event_failed", "event_type", event.Type, "run_id", runID, "error", err)
	}
}
