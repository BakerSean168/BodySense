package consultation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type consultationConfigurationEnvelope struct {
	ID                     string `json:"id"`
	Role                   string `json:"role"`
	DecisionPolicyRevision string `json:"decision_policy_revision"`
	LogicalModel           string `json:"logical_model"`
}

type consultationExecutionProvenanceEnvelope struct {
	LogicalModel string `json:"logical_model"`
}

func validateConsultationExecutionIdentity(
	event service.ConsultationRuntimeEvent,
	expectedConfigurationID string,
) (ConsultationExecutionIdentity, error) {
	payload, ok := event.Payload.(service.ConsultationRuntimeAgentConfigurationPayload)
	if !ok {
		return ConsultationExecutionIdentity{}, fmt.Errorf("unexpected handshake event kind %q", event.Kind())
	}
	expectedConfigurationID = strings.TrimSpace(expectedConfigurationID)
	if expectedConfigurationID == "" {
		return ConsultationExecutionIdentity{}, errors.New("missing expected Consultation configuration id")
	}
	expectedPolicy, err := service.ConsultationDecisionPolicyRevisionForConfiguration(expectedConfigurationID)
	if err != nil {
		return ConsultationExecutionIdentity{}, err
	}
	expectedLogicalModel, err := service.ConsultationLogicalModelForConfiguration(expectedConfigurationID)
	if err != nil {
		return ConsultationExecutionIdentity{}, err
	}
	if len(payload.AgentConfiguration) == 0 || len(payload.ExecutionProvenance) == 0 {
		return ConsultationExecutionIdentity{}, errors.New("handshake is missing configuration or execution provenance")
	}

	var configuration consultationConfigurationEnvelope
	if err := json.Unmarshal(payload.AgentConfiguration, &configuration); err != nil {
		return ConsultationExecutionIdentity{}, fmt.Errorf("decode Agent configuration: %w", err)
	}
	var provenance consultationExecutionProvenanceEnvelope
	if err := json.Unmarshal(payload.ExecutionProvenance, &provenance); err != nil {
		return ConsultationExecutionIdentity{}, fmt.Errorf("decode execution provenance: %w", err)
	}

	if configuration.ID != expectedConfigurationID {
		return ConsultationExecutionIdentity{}, fmt.Errorf("configuration id %q != expected %q", configuration.ID, expectedConfigurationID)
	}
	if configuration.Role != "consultation" {
		return ConsultationExecutionIdentity{}, fmt.Errorf("role %q != consultation", configuration.Role)
	}
	if configuration.DecisionPolicyRevision != expectedPolicy {
		return ConsultationExecutionIdentity{}, fmt.Errorf("decision policy %q != expected %q", configuration.DecisionPolicyRevision, expectedPolicy)
	}
	if configuration.LogicalModel != expectedLogicalModel {
		return ConsultationExecutionIdentity{}, fmt.Errorf("logical model %q != expected %q", configuration.LogicalModel, expectedLogicalModel)
	}
	if provenance.LogicalModel != expectedLogicalModel {
		return ConsultationExecutionIdentity{}, fmt.Errorf("execution logical model %q != expected %q", provenance.LogicalModel, expectedLogicalModel)
	}

	return ConsultationExecutionIdentity{
		ConfigurationID:     configuration.ID,
		AgentConfiguration:  append(json.RawMessage(nil), payload.AgentConfiguration...),
		ExecutionProvenance: append(json.RawMessage(nil), payload.ExecutionProvenance...),
	}, nil
}

func (r *Runtime) handleAIEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	event service.ConsultationRuntimeEvent,
	state streamState,
	result *streamResult,
	phase *string,
) bool {
	if err := validateBodyFactRuntimeEvent(state.ExpectedConfigurationID, event); err != nil {
		r.failActiveStream(ctx, sw, state, "旧版事实写入事件未通过当前契约核验，未保存该记录。")
		return true
	}
	publicEvent, projectionErr := projectConsultationRuntimeEvent(event)
	if projectionErr != nil && event.Kind() != service.ConsultationRuntimeAgentConfiguration {
		r.failActiveStream(ctx, sw, state, "invalid internal runtime event projection")
		return true
	}

	switch payload := event.Payload.(type) {
	case service.ConsultationRuntimeTextDeltaPayload:
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, "text.delta")
		result.AssistantParts = append(result.AssistantParts, map[string]any{"type": "text", "text": payload.Delta})

	case service.ConsultationRuntimeToolCallPayload:
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, "tool.call")
		result.AssistantParts = append(result.AssistantParts, map[string]any{
			"type": "tool-call", "toolName": payload.Tool, "args": payload.Args, "toolCallId": event.IDs.ToolCallID,
		})
		msgID := state.AssistantMsg.ID
		r.agentToolService.RecordToolCall(ctx, state.Run.ID, state.ConversationID, &msgID, event.IDs.ToolCallID, payload.Tool, datatypes.JSON(payload.Args))

	case service.ConsultationRuntimeToolResultPayload:
		var captureErr error
		if payload.Tool == "record_body_state" {
			committed, facts, err := r.commitBodyFactProposal(ctx, state, event.IDs.ToolCallID, payload.Result)
			captureErr = err
			if err != nil {
				slog.Default().Warn("body_fact_proposal_rejected", "error", err)
				payload.Result = json.RawMessage(`{"status":"rejected","error":"身体记录尚未确认保存，请刷新记录后重试。"}`)
			} else {
				payload.Result = committed
				result.CommittedFacts = append(result.CommittedFacts, facts...)
			}
			// Only a receipt or explicit rejection crosses the browser boundary.
			publicEvent.Payload, _ = json.Marshal(payload)
		}
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, "tool.result")
		result.AssistantParts = append(result.AssistantParts, map[string]any{
			"type": "tool-result", "toolName": payload.Tool, "result": payload.Result, "toolCallId": event.IDs.ToolCallID,
		})
		r.agentToolService.RecordToolResult(ctx, state.Run.ID, event.IDs.ToolCallID, datatypes.JSON(payload.Result), toolResultIsError(payload.Result))
		if captureErr != nil {
			r.failActiveStream(ctx, sw, state, "身体记录尚未确认保存，请刷新记录后重试。")
			return true
		}

	case service.ConsultationRuntimeExtractedInfoPayload, service.ConsultationRuntimeLifestyleContextPayload:
		// Model-extracted candidates are ephemeral browser state only. They are
		// never persisted: health facts come solely from user-authored or
		// user-confirmed input (record_body_state, structured intake, direct edit).
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, publicEvent.Type)

	case service.ConsultationRuntimeCitationAddedPayload:
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, publicEvent.Type)
		result.AssistantParts = append(result.AssistantParts, citationPart(publicEvent.Payload))

	case service.ConsultationRuntimeAttributionAddedPayload:
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, publicEvent.Type)
		result.AssistantParts = append(result.AssistantParts, dataPart("answer_attribution", publicEvent.Payload))

	case service.ConsultationRuntimeKnowledgeGapPayload:
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, publicEvent.Type)
		result.AssistantParts = append(result.AssistantParts, dataPart("knowledge_gap", publicEvent.Payload))

	case service.ConsultationRuntimeRedFlagDetectedPayload:
		// Model warnings remain conversation output. Only Go-validated user
		// screens can create restrictions; AI text cannot write Safety authority.
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, publicEvent.Type)
		result.AssistantParts = append(result.AssistantParts, dataPart("red_flag", publicEvent.Payload))

	case service.ConsultationRuntimeSafetyOutputPayload:
		r.handleSafetyOutputEvent(ctx, sw, publicEvent, payload, state, result)

	case service.ConsultationRuntimeInteractionPayload:
		return r.handleInteractionRequired(ctx, sw, publicEvent, payload, state, result)

	case service.ConsultationRuntimePhaseChangedPayload:
		nextPhase, ok := model.ParseConsultationPhase(payload.To)
		if !ok {
			r.failActiveStream(ctx, sw, state, "runtime emitted unsupported consultation phase")
			return true
		}
		if payload.From == "" {
			payload.From = *phase
		} else if _, ok := model.ParseConsultationPhase(payload.From); !ok {
			r.failActiveStream(ctx, sw, state, "runtime emitted unsupported previous consultation phase")
			return true
		}
		payload.To = string(nextPhase)
		if patched, err := json.Marshal(payload); err == nil {
			publicEvent.Payload = patched
		}
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, "phase_change")
		*phase = string(nextPhase)
		if err := r.consultationService.UpdatePhase(ctx, state.ConversationID, state.UID, nextPhase); err != nil {
			slog.Default().Error("update_phase_failed", "conversation_id", state.ConversationID, "error", err)
		}

	case service.ConsultationRuntimeUsageReportedPayload:
		result.Usage = payload.Usage
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, "usage.reported")

	case service.ConsultationRuntimeAgentConfigurationPayload:
		identity, err := validateConsultationExecutionIdentity(event, state.ExpectedConfigurationID)
		if err != nil {
			slog.Default().Warn("consultation_runtime_identity_rejected", "run_id", state.Run.ID, "error", err)
			r.failActiveStream(ctx, sw, state, "runtime Agent configuration identity mismatch")
			return true
		}
		if r.runService == nil {
			r.failActiveStream(ctx, sw, state, "runtime Agent configuration persistence unavailable")
			return true
		}
		if err := r.runService.UpdateAgentConfiguration(ctx, state.Run.ID, identity.ConfigurationID, datatypes.JSON(identity.AgentConfiguration), datatypes.JSON(identity.ExecutionProvenance)); err != nil {
			slog.Default().Error("persist_consultation_runtime_identity_failed", "run_id", state.Run.ID, "error", err)
			r.failActiveStream(ctx, sw, state, "failed to persist runtime Agent configuration identity")
			return true
		}
		result.ExecutionIdentity = identity

	case service.ConsultationRuntimeDonePayload:
		if payload.ResponseID != "" {
			result.ProviderResponseID = payload.ResponseID
		}
		if len(payload.Usage) > 0 {
			result.Usage = payload.Usage
		}
		if len(payload.Governance) > 0 {
			result.GovernanceResult = datatypes.JSON(payload.Governance)
		}

	case service.ConsultationRuntimeErrorPayload:
		r.sendEvent(ctx, sw, publicEvent, state.AssistantMsgID, "stream.error")
		r.failActiveStream(ctx, sw, state, payload.Message)
		return true

	default:
		r.failActiveStream(ctx, sw, state, "unsupported trusted runtime event")
		return true
	}

	return false
}

func (r *Runtime) handleInteractionRequired(
	ctx context.Context,
	sw *stream.StreamWriter,
	event dto.StreamEvent,
	payload service.ConsultationRuntimeInteractionPayload,
	state streamState,
	result *streamResult,
) bool {
	policy, _ := service.ConsultationDecisionPolicyRevisionForConfiguration(state.ExpectedConfigurationID)
	sameRunResume := policy == service.ConsultationDecisionPolicyV3
	// Go owns the symptom intake binding: only a reference to a symptom fact
	// committed in this stream (v3 only) becomes a bound question; every
	// other binding is stripped so its answer can record nothing.
	var committedFacts []service.CommittedBodyFact
	if sameRunResume {
		committedFacts = result.CommittedFacts
	}
	question, bound, err := service.BindSymptomIntakeQuestion(payload.Question, event.IDs.ToolCallID, committedFacts)
	if err != nil {
		slog.Default().Warn("interaction_question_rejected", "run_id", state.Run.ID, "error", err)
		r.failActiveStream(ctx, sw, state, "invalid user interaction")
		return true
	}
	if !bound && bytes.Contains(payload.Question, []byte(`"state_binding"`)) {
		slog.Default().Warn("stripped_unbound_symptom_intake_binding", "run_id", state.Run.ID, "tool_call_id", event.IDs.ToolCallID)
	}
	// A BodyState suggestion arrives only in the typed runtime field and is
	// bound only against the owner's own cited user message and current
	// revision (v3 only); otherwise it becomes a fixed notice whose answer
	// writes nothing. It runs after intake binding so the intake binder never
	// sees (and strips) a Go-written suggestion binding.
	var suggestionSources service.BodyStateSuggestionSourceReader
	if sameRunResume {
		suggestionSources = r.suggestionSources
	}
	question, err = service.AttachRuntimeBodyStateSuggestion(question, payload.Suggestion)
	if err == nil {
		var suggestionBound bool
		question, suggestionBound, err = service.BindBodyStateSuggestionQuestion(ctx, suggestionSources, state.UID, state.ConversationID, question)
		if err == nil && !suggestionBound && len(payload.Suggestion) > 0 {
			slog.Default().Warn("verify_body_state_suggestion_failed", "run_id", state.Run.ID, "tool_call_id", event.IDs.ToolCallID)
		}
	}
	if err != nil {
		slog.Default().Error("bind_body_state_suggestion_failed", "run_id", state.Run.ID, "error", err)
		r.failActiveStream(ctx, sw, state, "invalid user interaction")
		return true
	}
	payload.Question = question
	var nativeIDs []string
	if sameRunResume {
		nativeIDs = []string{payload.InteractionID}
	}
	_, preparedEvents, err := r.interactionService.CreatePendingInteractionWithEvents(
		ctx,
		state.Run.ID,
		state.ConversationID,
		event.IDs.ToolCallID,
		datatypes.JSON(payload.Question),
		func(interaction *model.AgentInteraction) ([]dto.StreamEvent, error) {
			interactionID := interaction.ID.String()
			requiredEvent := event
			requiredEvent.IDs.InteractionID = interactionID
			publicPayload := struct {
				InteractionID string          `json:"interaction_id"`
				Question      json.RawMessage `json:"question"`
				CreatedAt     string          `json:"created_at"`
			}{InteractionID: interactionID, Question: json.RawMessage(interaction.Question), CreatedAt: interaction.CreatedAt.UTC().Format(time.RFC3339Nano)}
			patched, err := json.Marshal(publicPayload)
			if err != nil {
				return nil, err
			}
			requiredEvent.Payload = patched
			requiredEvent = sw.EnrichEvent(requiredEvent, state.AssistantMsgID)
			interruptedEvent, err := sw.NewEvent(
				"run",
				"run.interrupted",
				dto.StreamEventIDs{
					ConversationID: state.ConversationID.String(),
					RunID:          state.Run.ID.String(),
					TurnID:         state.TurnID.String(),
					InteractionID:  interactionID,
				},
				map[string]any{"status": "waiting_user", "interaction_id": interactionID},
				"",
			)
			if err != nil {
				return nil, err
			}
			prepared := []dto.StreamEvent{requiredEvent, interruptedEvent}
			if sameRunResume {
				done, err := sw.NewEvent("stream", "stream.done", state.BaseIDs, map[string]any{}, "")
				if err != nil {
					return nil, err
				}
				prepared = append(prepared, done)
			}
			return prepared, nil
		},
		nativeIDs...,
	)
	if err != nil {
		slog.Default().Error("create_pending_interaction_failed", "conversation_id", state.ConversationID, "error", err)
		r.sendNewEvent(ctx, sw, "stream", "stream.error", state.BaseIDs, map[string]any{"message": "failed to persist user interaction"}, "", "stream.error")
		r.clearActiveRun(ctx, state.ConversationID, state.UID)
		return true
	}

	finalPartsJSON, _ := json.Marshal(result.AssistantParts)
	if err := r.messageService.UpdateMessageCompletedWithStatus(ctx, state.AssistantMsg.ID, state.ConversationID, datatypes.JSON(finalPartsJSON), "aborted"); err != nil {
		slog.Default().Error("finalize_interrupted_assistant_message_failed", "message_id", state.AssistantMsg.ID, "error", err)
	}

	for _, preparedEvent := range preparedEvents {
		if err := sw.WriteEvent(ctx, preparedEvent); err != nil {
			slog.Default().Warn("sse_write_error", "event_type", preparedEvent.Type, "run_id", state.Run.ID, "conversation_id", state.ConversationID, "seq", preparedEvent.Seq, "error", err)
		}
	}

	if !sameRunResume {
		r.sendNewEvent(ctx, sw, "stream", "stream.done", state.BaseIDs, map[string]any{}, "", "stream.done")
		r.clearActiveRun(ctx, state.ConversationID, state.UID)
	}
	r.refreshThreadProjection(ctx, state.ConversationID, state.UID)
	return true
}

func (r *Runtime) handleSafetyOutputEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	event dto.StreamEvent,
	payload service.ConsultationRuntimeSafetyOutputPayload,
	state streamState,
	result *streamResult,
) {
	r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)

	partName := "output_reviewed"
	if event.Type == "safety.output_rejected" {
		partName = "output_rejected"
	}
	result.AssistantParts = append(result.AssistantParts, dataPart(partName, event.Payload))

	if r.outputReviewService == nil {
		return
	}

	outputType := payload.OutputKind
	if outputType == "" {
		outputType = "structured_output"
	}
	verdict := payload.Verdict
	if verdict == "" {
		if event.Type == "safety.output_rejected" {
			verdict = "rejected"
		} else {
			verdict = "unknown"
		}
	}

	issues := datatypes.JSON("[]")
	if len(payload.Issues) > 0 {
		issues = datatypes.JSON(payload.Issues)
	} else if len(payload.Reasons) > 0 {
		if b, err := json.Marshal(payload.Reasons); err == nil {
			issues = datatypes.JSON(b)
		}
	}

	runID := state.Run.ID
	convID := state.ConversationID
	uid := state.UID
	r.outputReviewService.RecordReview(
		ctx,
		outputType,
		verdict,
		&uid,
		&runID,
		nil,
		&convID,
		issues,
		nil,
		datatypes.JSON(event.Payload),
	)
}

func (r *Runtime) recordGovernance(
	ctx context.Context,
	uid uuid.UUID,
	conversationID uuid.UUID,
	runID uuid.UUID,
	governanceResult datatypes.JSON,
) {
	if len(governanceResult) == 0 {
		return
	}

	var govStatus string
	var govIssues datatypes.JSON
	var govValidatedOutput datatypes.JSON
	var govPayload struct {
		// Prefer P2 "verdict"; keep "status" for older stream.done payloads.
		Verdict         string          `json:"verdict"`
		Status          string          `json:"status"`
		Issues          json.RawMessage `json:"issues"`
		ValidatedOutput json.RawMessage `json:"validated_output"`
	}
	if err := json.Unmarshal(governanceResult, &govPayload); err == nil {
		govStatus = govPayload.Verdict
		if govStatus == "" {
			govStatus = govPayload.Status
		}
		govIssues = datatypes.JSON(govPayload.Issues)
		if len(govPayload.ValidatedOutput) > 0 {
			govValidatedOutput = datatypes.JSON(govPayload.ValidatedOutput)
		}
	} else {
		govStatus = "unknown"
		govIssues = datatypes.JSON("[]")
	}
	if govStatus == "" {
		govStatus = "unknown"
	}

	r.outputReviewService.RecordReview(
		ctx,
		"consultation_reply",
		govStatus,
		&uid,
		&runID,
		nil,
		&conversationID,
		govIssues,
		govValidatedOutput,
		governanceResult,
	)
}
