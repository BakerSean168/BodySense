package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type interactionResumeRepository interface {
	ClaimInteractionResume(context.Context, model.ClaimInteractionResume) (*model.InteractionResumeEnvelope, error)
}

type InteractionResumeResult struct {
	Envelope *model.InteractionResumeEnvelope
	Events   []dto.StreamEvent
}

// StructuredIntakeRecorder records the safety screen of a Go-bound symptom
// intake answer inside the claim transaction.
type StructuredIntakeRecorder interface {
	RecordStructuredIntakeAnswer(ctx context.Context, userID uuid.UUID, source model.AnswerFactSource, interaction *model.AgentInteraction) error
}

// InteractionResumeService coordinates one Chat transaction. Ordinary answers
// commit no domain facts here; facts follow from source-bound extraction. The
// one exception is the Go-bound structured symptom intake, whose safety screen
// commits atomically with the claim so the Agent is never resumed from an
// unrecorded intake answer.
type InteractionResumeService struct {
	repo         interactionResumeRepository
	transactions interactionTransactionManager
	events       interactionLifecycleEventStore
	intake       StructuredIntakeRecorder
}

// WithStructuredIntake wires the BodyFact recorder for bound symptom intake.
// Without it, a bound intake answer is refused rather than silently dropped.
func (s *InteractionResumeService) WithStructuredIntake(recorder StructuredIntakeRecorder) *InteractionResumeService {
	s.intake = recorder
	return s
}

func NewInteractionResumeService(repo interactionResumeRepository, transactions interactionTransactionManager, events interactionLifecycleEventStore) *InteractionResumeService {
	return &InteractionResumeService{repo: repo, transactions: transactions, events: events}
}

func NativeInteractionInterrupt(interaction *model.AgentInteraction) (string, error) {
	if interaction == nil {
		return "", model.ErrInteractionResumeUnavailable
	}
	var metadata struct {
		Native string `json:"runtime_interrupt_id"`
	}
	if json.Unmarshal(interaction.Metadata, &metadata) != nil || len(metadata.Native) != 32 {
		return "", model.ErrInteractionResumeUnavailable
	}
	id, err := uuid.Parse(metadata.Native)
	if err != nil || id == uuid.Nil {
		return "", model.ErrInteractionResumeUnavailable
	}
	return id.String(), nil
}

func (s *InteractionResumeService) Claim(ctx context.Context, command model.ClaimInteractionResume) (*InteractionResumeResult, error) {
	if s == nil || s.repo == nil || s.transactions == nil || s.events == nil {
		return nil, model.ErrInteractionResumeUnavailable
	}
	if err := s.events.Flush(ctx); err != nil {
		return nil, fmt.Errorf("flush before resume: %w", err)
	}
	result := &InteractionResumeResult{}
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		envelope, err := s.repo.ClaimInteractionResume(txCtx, command)
		if err != nil {
			return err
		}
		if envelope == nil || envelope.Run == nil || envelope.Interaction == nil || envelope.Receipt == nil || envelope.UserMessage == nil || envelope.AssistantMessage == nil {
			return model.ErrInteractionResumeUnavailable
		}
		policy, err := ConsultationDecisionPolicyRevisionForConfiguration(envelope.Run.AgentConfigurationID)
		if err != nil || policy != ConsultationDecisionPolicyV3 {
			return model.ErrInteractionResumeUnavailable
		}
		if _, err := NativeInteractionInterrupt(envelope.Interaction); err != nil {
			return err
		}
		result.Envelope = envelope
		if envelope.Existed {
			return nil
		}
		source := model.AnswerFactSource{RunID: envelope.Run.ID, InteractionID: envelope.Interaction.ID, MessageID: envelope.UserMessage.ID}
		if s.intake != nil {
			if err := s.intake.RecordStructuredIntakeAnswer(txCtx, command.UserID, source, envelope.Interaction); err != nil {
				return fmt.Errorf("record structured intake: %w", err)
			}
		} else if IsBoundSymptomIntake(envelope.Interaction.ToolCallID, json.RawMessage(envelope.Interaction.Question)) {
			return model.ErrInteractionResumeUnavailable
		}
		ids := dto.StreamEventIDs{ConversationID: command.ConversationID.String(), RunID: envelope.Run.ID.String(), TurnID: envelope.Run.TurnID.String(), InteractionID: command.InteractionID.String(), ToolCallID: envelope.Interaction.ToolCallID}
		steps := []struct {
			channel   string
			kind      string
			messageID string
			payload   any
		}{
			{"state", "state.interaction.answered", "", map[string]any{"interaction_id": command.InteractionID.String(), "answer": json.RawMessage(envelope.Interaction.Answer)}},
			{"run", "run.resumed", "", map[string]any{"status": "running", "interaction_id": command.InteractionID.String()}},
			{"message", "message.created", envelope.AssistantMessage.ID.String(), map[string]any{"role": "assistant", "status": "streaming"}},
		}
		for _, step := range steps {
			stepIDs := ids
			stepIDs.MessageID = step.messageID
			event, err := s.events.PersistOutOfBandMilestone(txCtx, command.ConversationID, envelope.Run.ID, &envelope.Run.TurnID, step.channel, step.kind, stepIDs, step.payload)
			if err != nil {
				return fmt.Errorf("persist resume milestone: %w", err)
			}
			result.Events = append(result.Events, event)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
