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

// ReadSuggestionEpisodeFact resolves the symptom fact an episode suggestion
// organizes: the owner's one asserted, explicitly present, self symptom
// fact whose available user_message evidence is exactly this excerpt of the
// cited message, and that no Episode organizes yet. The fact is Go's, never
// a runtime-named id; anything else is indistinguishable from none.
func (r *BodyStateRepository) ReadSuggestionEpisodeFact(ctx context.Context, owner uuid.UUID, source model.SuggestionSourceMessage, excerpt string) (uuid.UUID, error) {
	if owner == uuid.Nil || source.ID == uuid.Nil || excerpt == "" {
		return uuid.Nil, model.ErrBodyStateSuggestionRejected
	}
	return suggestionEpisodeFact(database.FromContext(ctx, r.db).WithContext(ctx), owner, source.ID, excerpt, uuid.Nil, false)
}

func suggestionEpisodeFact(tx *gorm.DB, owner, messageID uuid.UUID, excerpt string, factID uuid.UUID, lock bool) (uuid.UUID, error) {
	query := tx.Table("public.body_facts AS f").Select("f.id").
		Joins("JOIN public.statement_evidence AS ev ON ev.user_id = f.user_id AND ev.id = f.source_evidence_id").
		Where("f.user_id = ? AND f.kind = 'symptom' AND f.subject = 'self' AND f.assertion_status = 'asserted' AND f.details->>'presence' = 'present'", owner).
		Where("ev.source_kind = 'user_message' AND ev.availability = 'available' AND ev.source_message_id = ? AND ev.excerpt = ?", messageID, excerpt).
		Where("NOT EXISTS (SELECT 1 FROM public.episode_fact_links AS l WHERE l.user_id = f.user_id AND l.fact_id = f.id AND l.is_primary)")
	if factID != uuid.Nil {
		query = query.Where("f.id = ?", factID)
	}
	if lock {
		query = query.Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "f"}})
	}
	var ids []uuid.UUID
	if err := query.Limit(2).Pluck("f.id", &ids).Error; err != nil {
		return uuid.Nil, err
	}
	if len(ids) != 1 {
		return uuid.Nil, model.ErrBodyStateSuggestionRejected
	}
	return ids[0], nil
}

// ConfirmBodyStateSuggestion commits the user's explicit confirmation of a
// Go-bound suggestion as the user's own record of the bound kind:
//   - hypothesis: a BodyHypothesis whose statement is the cited excerpt;
//   - observation: a user-reported BodyObservation whose statement is the
//     cited excerpt, labelled with the bound organizational kind and the
//     raw value/unit words copied from it (never interpreted);
//   - episode: a new Episode organizing the symptom fact Go resolved from
//     that exact excerpt.
//
// The accepted answer, the cited message span (re-read and digest-checked),
// and the bound BodyState revision are all verified in the commit
// transaction; nothing the model wrote becomes a statement. One confirmation
// is one receipt: a replay returns the original commit.
func (r *BodyStateRepository) ConfirmBodyStateSuggestion(ctx context.Context, owner uuid.UUID, answer model.AnswerFactSource, command model.ConfirmBodyStateSuggestionCommand) (*model.BodyStateSuggestionReceipt, error) {
	if owner == uuid.Nil || answer.RunID == uuid.Nil || answer.InteractionID == uuid.Nil || answer.MessageID == uuid.Nil {
		return nil, fmt.Errorf("%w: accepted answer identity required", model.ErrBodyStateSuggestionRejected)
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	idempotencyKey := "body-state-suggestion:" + answer.InteractionID.String()
	fingerprint, err := suggestionFingerprint(answer, command)
	if err != nil {
		return nil, err
	}
	var output *model.BodyStateSuggestionReceipt
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		prior, err := priorSuggestionReceipt(tx, owner, command.Kind, idempotencyKey, fingerprint)
		if err != nil || prior != nil {
			output = prior
			return err
		}
		cited, statement, err := verifySuggestionConfirmation(tx, owner, answer, command)
		if err != nil {
			return err
		}
		if state.CurrentRevision != command.BaseBodyStateRevision {
			return fmt.Errorf("%w: %w: bound %d, current %d", model.ErrBodyStateSuggestionStale, ErrBodyStateRevisionConflict, command.BaseBodyStateRevision, state.CurrentRevision)
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		authorization := map[string]any{"authorization": "ask_user_confirmation", "interaction_id": answer.InteractionID}
		conversationID, runID, messageID := cited.ConversationID, cited.RunID, cited.ID
		evidence := model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "user_message", SourceVersion: 1,
			SourceConversationID: &conversationID, SourceRunID: &runID, SourceMessageID: &messageID,
			Excerpt: statement, Availability: "available", RecordedAt: now}
		switch command.Kind {
		case model.BodyStateSuggestionKindHypothesis:
			commit, err := persistBodyHypothesisRecord(ctx, tx, state, next, owner, idempotencyKey, fingerprint, statement, nil, nil, evidence, "user_message", authorization, now)
			if err != nil {
				return err
			}
			output = hypothesisSuggestionReceipt(commit)
		case model.BodyStateSuggestionKindObservation:
			input := model.RecordBodyObservationInput{Kind: command.Payload.ObservationKind, Statement: statement,
				RawValue: command.Payload.RawValue, RawUnit: command.Payload.RawUnit, ObservedAt: model.FactTemporal{Precision: "unknown"}}
			if err := (model.RecordBodyObservationCommand{IdempotencyKey: idempotencyKey, ExpectedRevision: state.CurrentRevision, Observation: input}).Validate(); err != nil {
				return fmt.Errorf("%w: %v", model.ErrBodyStateSuggestionRejected, err)
			}
			commit, err := persistBodyObservationRecord(ctx, tx, state, next, owner, idempotencyKey, fingerprint, input, evidence, "user_message", authorization, now)
			if err != nil {
				return err
			}
			output = observationSuggestionReceipt(commit)
		case model.BodyStateSuggestionKindEpisode:
			factID, err := uuid.Parse(command.Payload.FactID)
			if err != nil {
				return model.ErrBodyStateSuggestionRejected
			}
			// The organized fact must still be the asserted, available symptom
			// stated by exactly this cited span and not yet in an Episode.
			if _, err := suggestionEpisodeFact(tx, owner, cited.ID, statement, factID, true); err != nil {
				return err
			}
			commit, err := persistEpisodeAssociation(ctx, tx, state, next, owner, uuid.Nil, factID, "create", idempotencyKey, fingerprint,
				episodeAssociationOptions{RevisionSource: "user_message", Authorization: authorization, FallbackSymptomType: command.Payload.SymptomType}, now)
			if err != nil {
				return err
			}
			output = episodeSuggestionReceipt(commit)
		default:
			return model.ErrBodyStateSuggestionRejected
		}
		// The answered interaction keeps the id-only receipt, so history can
		// show "saved" only for what Go committed in this same transaction.
		receipt, err := json.Marshal(output.Public())
		if err != nil {
			return err
		}
		updated := tx.Exec("UPDATE public.agent_interactions SET metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('body_state_receipt', ?::jsonb) WHERE id = ? AND run_id = ?",
			string(receipt), answer.InteractionID, answer.RunID)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("%w: answered interaction not found for its receipt", model.ErrBodyStateSuggestionRejected)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

// suggestionFingerprint keeps the hypothesis fingerprint byte-identical to
// the first (hypothesis-only) release so an in-flight confirmation replays.
func suggestionFingerprint(answer model.AnswerFactSource, command model.ConfirmBodyStateSuggestionCommand) (string, error) {
	type legacyHypothesisCommand struct {
		Source                model.BodyStateSuggestionSource `json:"source"`
		BaseBodyStateRevision int64                           `json:"base_body_state_revision"`
	}
	var bound any = command
	if command.Kind == model.BodyStateSuggestionKindHypothesis {
		bound = legacyHypothesisCommand{Source: command.Source, BaseBodyStateRevision: command.BaseBodyStateRevision}
	}
	return bodyFactFingerprintValue(struct {
		Operation string                 `json:"operation"`
		Source    string                 `json:"source_kind"`
		Answer    model.AnswerFactSource `json:"answer"`
		Command   any                    `json:"command"`
	}{Operation: "record", Source: "user_message:suggestion", Answer: answer, Command: bound})
}

// priorSuggestionReceipt returns the original receipt of this confirmation
// (same interaction), or nil when there is none; a different command under
// the same key is an idempotency conflict.
func priorSuggestionReceipt(tx *gorm.DB, owner uuid.UUID, kind, key, fingerprint string) (*model.BodyStateSuggestionReceipt, error) {
	find := func(dest any, fp func() string) (bool, error) {
		err := tx.Where("user_id = ? AND idempotency_key = ?", owner, key).Take(dest).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if fp() != fingerprint {
			return false, fmt.Errorf("%w: suggestion confirmation replayed with a different command", model.ErrBodyHypothesisIdempotencyConflict)
		}
		return true, nil
	}
	switch kind {
	case model.BodyStateSuggestionKindHypothesis:
		var prior model.BodyHypothesisCommit
		if found, err := find(&prior, func() string { return prior.Fingerprint }); !found || err != nil {
			return nil, err
		}
		return hypothesisSuggestionReceipt(&prior), nil
	case model.BodyStateSuggestionKindObservation:
		var prior model.BodyObservationCommit
		if found, err := find(&prior, func() string { return prior.Fingerprint }); !found || err != nil {
			return nil, err
		}
		return observationSuggestionReceipt(&prior), nil
	case model.BodyStateSuggestionKindEpisode:
		var prior model.EpisodeCommandReceipt
		if found, err := find(&prior, func() string { return prior.Fingerprint }); !found || err != nil {
			return nil, err
		}
		return episodeSuggestionReceipt(&prior), nil
	}
	return nil, model.ErrBodyStateSuggestionRejected
}

// verifySuggestionConfirmation checks, under share locks, that the answer is
// the live accepted explicit confirmation of exactly this Go binding, and
// re-reads the cited span: a withdrawn, pruned, edited or foreign source
// rejects the commit instead of trusting the excerpt bound at suggestion time.
func verifySuggestionConfirmation(tx *gorm.DB, owner uuid.UUID, answer model.AnswerFactSource, command model.ConfirmBodyStateSuggestionCommand) (*model.SuggestionSourceMessage, string, error) {
	var answers []model.Message
	if err := tx.Table("public.messages AS m").Select("m.*").
		Joins("JOIN public.runs AS r ON r.turn_id = m.turn_id AND r.conversation_id = m.conversation_id").
		Joins("JOIN public.conversations AS c ON c.id = m.conversation_id").
		Where("m.id = ? AND r.id = ? AND r.user_id = ? AND c.user_id = ? AND r.status = ? AND m.role = ? AND m.status = ?", answer.MessageID, answer.RunID, owner, owner, "running", "user", "completed").
		Clauses(clause.Locking{Strength: "SHARE"}).Limit(2).Find(&answers).Error; err != nil {
		return nil, "", err
	}
	if len(answers) != 1 {
		return nil, "", fmt.Errorf("%w: exactly one live accepted answer is required", model.ErrBodyStateSuggestionRejected)
	}
	interaction, _, err := verifyBodyFactAnswer(tx, owner, answer, answers[0])
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", model.ErrBodyStateSuggestionRejected, err)
	}
	bound, matched, err := model.ParseBoundBodyStateSuggestion(interaction.Question)
	if !matched || err != nil || bound != command {
		return nil, "", fmt.Errorf("%w: command does not match the bound question", model.ErrBodyStateSuggestionRejected)
	}
	if confirmed, err := model.BodyStateSuggestionConfirmed(command.Kind, interaction.Answer); err != nil || !confirmed {
		return nil, "", fmt.Errorf("%w: the accepted answer is not a confirmation", model.ErrBodyStateSuggestionRejected)
	}
	seq := command.Source.MessageSeq
	cited, err := citableUserMessage(tx, owner, answers[0].ConversationID, command.Source.MessageID, &seq, true)
	if err != nil {
		return nil, "", err
	}
	statement, ok := model.BodyStateSuggestionExcerpt(cited.Text, command.Source.SpanStart, command.Source.SpanEnd)
	if !ok || model.BodyStateSuggestionDigest(statement) != command.Source.ExcerptSHA256 {
		return nil, "", fmt.Errorf("%w: cited span no longer matches the user message", model.ErrBodyStateSuggestionRejected)
	}
	if err := model.ValidateBodyStateSuggestionPayload(command.Kind, command.Payload, statement, true); err != nil {
		return nil, "", err
	}
	return cited, statement, nil
}

func hypothesisSuggestionReceipt(c *model.BodyHypothesisCommit) *model.BodyStateSuggestionReceipt {
	return &model.BodyStateSuggestionReceipt{Kind: model.BodyStateSuggestionKindHypothesis, UserID: c.UserID, CommitID: c.ID, RecordID: c.HypothesisID,
		SourceEvidenceID: c.SourceEvidenceID, Revision: c.Revision, RevisionID: c.RevisionID, EventID: c.EventID, CommittedAt: c.CommittedAt}
}

func observationSuggestionReceipt(c *model.BodyObservationCommit) *model.BodyStateSuggestionReceipt {
	return &model.BodyStateSuggestionReceipt{Kind: model.BodyStateSuggestionKindObservation, UserID: c.UserID, CommitID: c.ID, RecordID: c.ObservationID,
		SourceEvidenceID: c.SourceEvidenceID, Revision: c.Revision, RevisionID: c.RevisionID, EventID: c.EventID, CommittedAt: c.CommittedAt}
}

func episodeSuggestionReceipt(c *model.EpisodeCommandReceipt) *model.BodyStateSuggestionReceipt {
	return &model.BodyStateSuggestionReceipt{Kind: model.BodyStateSuggestionKindEpisode, UserID: c.UserID, CommitID: c.ID, RecordID: c.EpisodeID,
		FactID: c.FactID, Revision: c.Revision, RevisionID: c.RevisionID, EventID: c.EventID, CommittedAt: c.CommittedAt}
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
