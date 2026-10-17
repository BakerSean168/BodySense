package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// citableUserMessage resolves one message a BodyState suggestion may cite:
// the owner's own, completed, user-authored, AI-visible (not behind the
// retention context cursor) chat message that is not an Ask User answer,
// plus the single Run of its turn. A foreign, assistant, pruned or answer
// message is indistinguishable from a missing one.
func citableUserMessage(tx *gorm.DB, owner, conversationID, messageID uuid.UUID, seq *int, lock bool) (*model.SuggestionSourceMessage, error) {
	query := tx.Table("public.messages AS m").Select("m.*").
		Joins("JOIN public.conversations AS c ON c.id = m.conversation_id").
		Where("m.id = ? AND m.conversation_id = ? AND c.user_id = ? AND m.role = ? AND m.status = ? AND m.seq >= c.context_start_seq",
			messageID, conversationID, owner, "user", "completed")
	if seq != nil {
		query = query.Where("m.seq = ?", *seq)
	}
	if lock {
		query = query.Clauses(clause.Locking{Strength: "SHARE"})
	}
	var messages []model.Message
	if err := query.Limit(2).Find(&messages).Error; err != nil {
		return nil, err
	}
	if len(messages) != 1 {
		return nil, model.ErrBodyStateSuggestionRejected
	}
	message := messages[0]
	var metadata struct {
		IsAnswer bool `json:"is_interaction_answer"`
	}
	if len(message.Metadata) > 0 && json.Unmarshal(message.Metadata, &metadata) != nil {
		return nil, model.ErrBodyStateSuggestionRejected
	}
	text := messageTextFromParts(message.Parts)
	if metadata.IsAnswer || text == "" {
		return nil, model.ErrBodyStateSuggestionRejected
	}
	var runIDs []uuid.UUID
	if err := tx.Table("public.runs").Where("turn_id = ? AND conversation_id = ? AND user_id = ?", message.TurnID, message.ConversationID, owner).
		Limit(2).Pluck("id", &runIDs).Error; err != nil {
		return nil, err
	}
	if len(runIDs) != 1 {
		return nil, model.ErrBodyStateSuggestionRejected
	}
	return &model.SuggestionSourceMessage{ID: message.ID, ConversationID: message.ConversationID, RunID: runIDs[0], Seq: message.Seq, Text: text}, nil
}

// ReadSuggestionSource is the bind-time read: the citable message and the
// owner's current BodyState revision the suggestion is bound against.
func (r *BodyStateRepository) ReadSuggestionSource(ctx context.Context, owner, conversationID, messageID uuid.UUID) (*model.SuggestionSourceMessage, int64, error) {
	if owner == uuid.Nil || conversationID == uuid.Nil || messageID == uuid.Nil {
		return nil, 0, model.ErrBodyStateSuggestionRejected
	}
	db := database.FromContext(ctx, r.db).WithContext(ctx)
	source, err := citableUserMessage(db, owner, conversationID, messageID, nil, false)
	if err != nil {
		return nil, 0, err
	}
	var revisions []int64
	if err := db.Model(&model.BodyState{}).Where("user_id = ?", owner).Limit(1).Pluck("current_revision", &revisions).Error; err != nil {
		return nil, 0, err
	}
	var revision int64
	if len(revisions) == 1 {
		revision = revisions[0]
	}
	return source, revision, nil
}

// ConfirmBodyHypothesisSuggestion commits the user's explicit confirmation
// of a Go-bound hypothesis suggestion as the user's own BodyHypothesis. The
// accepted answer, the cited message span (re-read and digest-checked), and
// the bound BodyState revision are all verified in the commit transaction;
// the statement comes from the user's message, never from the model. One
// confirmation is one receipt: a replay returns the original commit.
func (r *BodyStateRepository) ConfirmBodyHypothesisSuggestion(ctx context.Context, owner uuid.UUID, answer model.AnswerFactSource, command model.ConfirmBodyHypothesisSuggestionCommand) (*model.BodyHypothesisCommit, error) {
	if owner == uuid.Nil || answer.RunID == uuid.Nil || answer.InteractionID == uuid.Nil || answer.MessageID == uuid.Nil {
		return nil, fmt.Errorf("%w: accepted answer identity required", model.ErrBodyStateSuggestionRejected)
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	idempotencyKey := "body-state-suggestion:" + answer.InteractionID.String()
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                                       `json:"operation"`
		Source    string                                       `json:"source_kind"`
		Answer    model.AnswerFactSource                       `json:"answer"`
		Command   model.ConfirmBodyHypothesisSuggestionCommand `json:"command"`
	}{Operation: "record", Source: "user_message:suggestion", Answer: answer, Command: command})
	if err != nil {
		return nil, err
	}
	var output model.BodyHypothesisCommit
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		var prior model.BodyHypothesisCommit
		err = tx.Where("user_id=? AND idempotency_key=?", owner, idempotencyKey).Take(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return model.ErrBodyHypothesisIdempotencyConflict
			}
			output = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// The confirmation must be the live accepted answer of this Run.
		var answers []model.Message
		if err := tx.Table("public.messages AS m").Select("m.*").
			Joins("JOIN public.runs AS r ON r.turn_id = m.turn_id AND r.conversation_id = m.conversation_id").
			Joins("JOIN public.conversations AS c ON c.id = m.conversation_id").
			Where("m.id = ? AND r.id = ? AND r.user_id = ? AND c.user_id = ? AND r.status = ? AND m.role = ? AND m.status = ?", answer.MessageID, answer.RunID, owner, owner, "running", "user", "completed").
			Clauses(clause.Locking{Strength: "SHARE"}).Limit(2).Find(&answers).Error; err != nil {
			return err
		}
		if len(answers) != 1 {
			return fmt.Errorf("%w: exactly one live accepted answer is required", model.ErrBodyStateSuggestionRejected)
		}
		interaction, _, err := verifyBodyFactAnswer(tx, owner, answer, answers[0])
		if err != nil {
			return fmt.Errorf("%w: %v", model.ErrBodyStateSuggestionRejected, err)
		}
		// The command must be exactly the Go binding persisted in the answered
		// question, and the accepted answer must be the explicit confirmation.
		bound, matched, err := model.ParseBoundBodyStateSuggestion(interaction.Question)
		if !matched || err != nil || bound != command {
			return fmt.Errorf("%w: command does not match the bound question", model.ErrBodyStateSuggestionRejected)
		}
		if confirmed, err := model.BodyStateSuggestionConfirmed(interaction.Answer); err != nil || !confirmed {
			return fmt.Errorf("%w: the accepted answer is not a confirmation", model.ErrBodyStateSuggestionRejected)
		}
		// Re-read the cited user span under a share lock: a withdrawn, pruned,
		// edited or foreign source rejects the commit instead of trusting the
		// excerpt bound at suggestion time.
		seq := command.Source.MessageSeq
		cited, err := citableUserMessage(tx, owner, answers[0].ConversationID, command.Source.MessageID, &seq, true)
		if err != nil {
			return err
		}
		statement, ok := model.BodyStateSuggestionExcerpt(cited.Text, command.Source.SpanStart, command.Source.SpanEnd)
		if !ok || model.BodyStateSuggestionDigest(statement) != command.Source.ExcerptSHA256 {
			return fmt.Errorf("%w: cited span no longer matches the user message", model.ErrBodyStateSuggestionRejected)
		}
		if state.CurrentRevision != command.BaseBodyStateRevision {
			return fmt.Errorf("%w: %w: bound %d, current %d", model.ErrBodyStateSuggestionStale, ErrBodyStateRevisionConflict, command.BaseBodyStateRevision, state.CurrentRevision)
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		conversationID, runID, messageID := cited.ConversationID, cited.RunID, cited.ID
		evidence := model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "user_message", SourceVersion: 1,
			SourceConversationID: &conversationID, SourceRunID: &runID, SourceMessageID: &messageID,
			Excerpt: statement, Availability: "available", RecordedAt: now}
		commit, err := persistBodyHypothesisRecord(ctx, tx, state, next, owner, idempotencyKey, fingerprint, statement, nil, nil, evidence, "user_message",
			map[string]any{"authorization": "ask_user_confirmation", "interaction_id": answer.InteractionID}, now)
		if err != nil {
			return err
		}
		output = *commit
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &output, nil
}

// persistBodyHypothesisRecord writes one new user guess and its links:
// revision, statement evidence, hypothesis, id-only outbox and receipt. The
// caller holds the BodyState lock and has verified source and revision.
func persistBodyHypothesisRecord(ctx context.Context, tx *gorm.DB, state *model.BodyState, next int64, owner uuid.UUID, key, fingerprint, statement string,
	facts, observations []uuid.UUID, evidence model.StatementEvidence, revisionSource string, extra map[string]any, now time.Time) (*model.BodyHypothesisCommit, error) {
	if strings.TrimSpace(statement) != statement || statement == "" {
		return nil, model.ErrInvalidBodyHypothesis
	}
	commitID, hypothesisID, eventID := uuid.New(), uuid.New(), uuid.New()
	metadata := map[string]any{"commit_id": commitID, "hypothesis_id": hypothesisID, "lineage_id": hypothesisID,
		"source_evidence_id": evidence.ID, "related_fact_ids": factIDsOrEmpty(facts),
		"related_observation_ids": factIDsOrEmpty(observations)}
	for k, v := range extra {
		metadata[k] = v
	}
	revision, err := bodyStatePersistRevision(ctx, tx, state, next, "hypothesis.recorded", revisionSource, metadata)
	if err != nil {
		return nil, err
	}
	if err := tx.Create(&evidence).Error; err != nil {
		return nil, err
	}
	record := model.BodyHypothesis{ID: hypothesisID, UserID: owner, Statement: statement,
		SourceEvidenceID: evidence.ID, AssertionStatus: "asserted", CreatedRevision: next, RecordedAt: now,
		LineageID: hypothesisID}
	if err := tx.Create(&record).Error; err != nil {
		return nil, err
	}
	if err := createBodyHypothesisLinks(tx, owner, hypothesisID, facts, observations); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	event := model.BodyStateOutbox{ID: eventID, UserID: owner, Revision: next,
		EventType: "body_state.hypothesis_recorded", Payload: datatypes.JSON(payload), CreatedAt: now}
	if err := tx.Create(&event).Error; err != nil {
		return nil, err
	}
	output := model.BodyHypothesisCommit{ID: commitID, UserID: owner, IdempotencyKey: key,
		Fingerprint: fingerprint, Operation: "record", HypothesisID: hypothesisID, SourceEvidenceID: evidence.ID,
		Revision: next, RevisionID: revision.ID, EventID: eventID, CommittedAt: now}
	if err := tx.Create(&output).Error; err != nil {
		return nil, err
	}
	return &output, nil
}
