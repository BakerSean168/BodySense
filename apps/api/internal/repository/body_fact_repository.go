package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

// RecordDirectFacts owns a single semantic transaction on the existing BodyState
// aggregate. This is deliberately not an Upsert: changing a statement requires a
// new command and correction authorization, never reuse of an idempotency key.
func (r *BodyStateRepository) RecordDirectFacts(ctx context.Context, userID uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	return r.recordFacts(ctx, userID, command, nil)
}

// RecordMessageFacts only accepts literal excerpts from a user-authored message
// in the provided Go-owned run. The run/message/owner/turn join is checked in
// the same transaction as the fact commit, not trusted from an Agent payload.
func (r *BodyStateRepository) RecordMessageFacts(ctx context.Context, userID uuid.UUID, source model.MessageFactSource, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	if source.RunID == uuid.Nil || source.MessageID == uuid.Nil {
		return nil, fmt.Errorf("%w: run and user message identities are required", model.ErrInvalidBodyFact)
	}
	return r.recordFacts(ctx, userID, command, &source)
}

// RecordRunFacts resolves the original user message from the Go-owned turn.
// The model supplies no message ID; stable run+command fingerprint allows exact
// receipt replay even after the original message was legally pruned.
func (r *BodyStateRepository) RecordRunFacts(ctx context.Context, userID, runID uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	if runID == uuid.Nil {
		return nil, fmt.Errorf("%w: run identity required", model.ErrInvalidBodyFact)
	}
	return r.recordFacts(ctx, userID, command, &model.MessageFactSource{RunID: runID})
}

func (r *BodyStateRepository) recordFacts(ctx context.Context, userID uuid.UUID, command model.RecordBodyFactsCommand, source *model.MessageFactSource) (*model.BodyFactCommit, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyFact)
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	fingerprint, err := bodyFactCommandFingerprint(command)
	if source != nil {
		fingerprint, err = bodyFactMessageFingerprint(command, *source)
	}
	if err != nil {
		return nil, err
	}
	var result model.BodyFactCommit
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		// Lock first, but inspect an existing receipt before rejecting the old
		// expected revision: a successful command's original retry is not stale.
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, nil)
		if err != nil {
			return err
		}
		var prior model.BodyFactCommit
		err = tx.Where("user_id = ? AND idempotency_key = ?", userID, command.IdempotencyKey).First(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return model.ErrBodyFactIdempotencyConflict
			}
			result = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if state.CurrentRevision != command.ExpectedRevision {
			return fmt.Errorf("%w: expected %d, current %d", ErrBodyStateRevisionConflict, command.ExpectedRevision, state.CurrentRevision)
		}

		// User messages deliberately have no run_id FK: the Run is identified
		// by the shared turn, which is the real consultation envelope shape.
		// Checking the structured Parts avoids trusting a client-supplied
		// source label or stale/incorrect content_text projection.
		sourceKind := "direct_edit"
		var sourceConversationID, sourceMessageID uuid.UUID
		if source != nil {
			// Shared row locks serialize source edits/removal and Run cancellation
			// with this commit. A completed/cancelled Run cannot accept NEW facts.
			var messages []model.Message
			query := tx.Table("public.messages AS m").Select("m.*").
				Joins("JOIN public.runs AS r ON r.turn_id = m.turn_id AND r.conversation_id = m.conversation_id").
				Joins("JOIN public.conversations AS c ON c.id = m.conversation_id").
				Where("r.id = ? AND r.user_id = ? AND c.user_id = ? AND c.deleted_at IS NULL AND r.status = ? AND m.role = ? AND m.status = ?", source.RunID, userID, userID, "running", "user", "completed")
			if source.MessageID != uuid.Nil {
				query = query.Where("m.id = ?", source.MessageID)
			}
			if err := query.Clauses(clause.Locking{Strength: "SHARE"}).Limit(2).Find(&messages).Error; err != nil {
				return err
			}
			if len(messages) != 1 {
				return fmt.Errorf("%w: exactly one live user-authored source is required for this run", model.ErrInvalidBodyFact)
			}
			userMessage := messages[0]
			original := messageTextFromParts(userMessage.Parts)
			if original == "" {
				return fmt.Errorf("%w: source message contains no user-authored text", model.ErrInvalidBodyFact)
			}
			for _, fact := range command.Facts {
				if !strings.Contains(original, fact.Statement) {
					return fmt.Errorf("%w: statement is not a verbatim excerpt from the user message", model.ErrInvalidBodyFact)
				}
			}
			sourceKind = "user_message"
			sourceConversationID = userMessage.ConversationID
			sourceMessageID = userMessage.ID
		}

		now := time.Now().UTC()
		commitID, eventID := uuid.New(), uuid.New()
		facts := make([]model.BodyFact, 0, len(command.Facts))
		evidence := make([]model.StatementEvidence, 0, len(command.Facts))
		factIDs := make([]uuid.UUID, 0, len(command.Facts))
		evidenceIDs := make([]uuid.UUID, 0, len(command.Facts))
		for _, input := range command.Facts {
			factID, evidenceID := uuid.New(), uuid.New()
			regions, err := json.Marshal(input.BodyRegionIDs)
			if err != nil {
				return err
			}
			temporal, err := json.Marshal(input.Temporal)
			if err != nil {
				return err
			}
			facts = append(facts, model.BodyFact{
				ID: factID, UserID: userID, Subject: "self", Kind: input.Kind, Statement: input.Statement,
				BodyRegionIDs: datatypes.JSON(regions), Temporal: datatypes.JSON(temporal), Details: datatypes.JSON(input.Details),
				SourceEvidenceID: evidenceID, AssertionStatus: "asserted", CreatedRevision: next, RecordedAt: now,
			})
			sourceRow := model.StatementEvidence{
				ID: evidenceID, UserID: userID, SourceKind: sourceKind, SourceVersion: 1,
				Excerpt: input.Statement, Availability: "available", RecordedAt: now,
			}
			if source != nil {
				sourceRow.SourceConversationID = &sourceConversationID
				sourceRow.SourceRunID = &source.RunID
				sourceRow.SourceMessageID = &sourceMessageID
			}
			evidence = append(evidence, sourceRow)
			factIDs = append(factIDs, factID)
			evidenceIDs = append(evidenceIDs, evidenceID)
		}
		// Revision and outbox carry identifiers only. Raw health text remains in
		// its owning rows and can be erased without retaining hidden audit copies.
		metadata := map[string]any{"commit_id": commitID, "fact_ids": factIDs, "source_evidence_ids": evidenceIDs}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "facts.recorded", sourceKind, metadata)
		if err != nil {
			return err
		}
		if err := tx.Create(&evidence).Error; err != nil {
			return err
		}
		if err := tx.Create(&facts).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		event := model.BodyStateOutbox{ID: eventID, UserID: userID, Revision: next, EventType: "body_state.facts_recorded", Payload: datatypes.JSON(payload), CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		encodedIDs, err := json.Marshal(factIDs)
		if err != nil {
			return err
		}
		result = model.BodyFactCommit{
			ID: commitID, UserID: userID, IdempotencyKey: command.IdempotencyKey, Fingerprint: fingerprint,
			Revision: next, RevisionID: revision.ID, EventID: eventID, FactIDs: datatypes.JSON(encodedIDs), CommittedAt: now,
		}
		return tx.Create(&result).Error
	})
	// The caller never receives a success-shaped receipt when commit/rollback
	// failed, even if all inserts had run before a final transaction error.
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ListFacts pages complete semantic batches, not individual rows. A page contains
// at most 20 revisions (400 facts), preserving all rows in a single committed
// batch. It is explicitly the new fact ledger, not the unconverted legacy snapshot.
func (r *BodyStateRepository) ListFacts(ctx context.Context, userID uuid.UUID, beforeRevision int64) (*model.BodyFactPage, error) {
	if userID == uuid.Nil || beforeRevision < 0 {
		return nil, fmt.Errorf("%w: invalid owner or cursor", model.ErrInvalidBodyFact)
	}
	// Corrections can change an existing row's assertion state without changing
	// its created revision. All page queries therefore share one DB snapshot.
	var page *model.BodyFactPage
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var err error
		page, err = listBodyFactsSnapshot(tx, userID, beforeRevision)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return page, nil
}

func listBodyFactsSnapshot(db *gorm.DB, userID uuid.UUID, beforeRevision int64) (*model.BodyFactPage, error) {
	page := &model.BodyFactPage{UserID: userID, Facts: []model.BodyFact{}}
	var state model.BodyState
	if err := db.Where("user_id = ?", userID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return page, nil
		}
		return nil, err
	}
	page.CurrentRevision = state.CurrentRevision
	// Pin the maximum revision so a concurrent commit cannot make the facts
	// disagree with the aggregate revision returned at the start of this read.
	query := db.Table("body_facts AS f").
		Joins("JOIN statement_evidence AS e ON e.user_id = f.user_id AND e.id = f.source_evidence_id").
		Where("f.user_id = ? AND f.created_revision <= ? AND f.assertion_status = ? AND e.availability = ?", userID, state.CurrentRevision, "asserted", "available")
	if beforeRevision > 0 {
		query = query.Where("f.created_revision < ?", beforeRevision)
	}
	var revisions []int64
	if err := query.Session(&gorm.Session{}).Distinct("f.created_revision").Order("f.created_revision DESC").Limit(21).Pluck("f.created_revision", &revisions).Error; err != nil {
		return nil, err
	}
	if len(revisions) == 0 {
		return page, nil
	}
	if len(revisions) > 20 {
		page.HasMore = true
		revisions = revisions[:20]
		next := revisions[len(revisions)-1]
		page.NextBeforeRevision = &next
	}
	if err := query.Session(&gorm.Session{}).Select("f.*").Where("f.created_revision IN ?", revisions).
		Order("f.created_revision DESC, f.id ASC").Find(&page.Facts).Error; err != nil {
		return nil, err
	}
	return page, nil
}

func bodyFactCommandFingerprint(command model.RecordBodyFactsCommand) (string, error) {
	return bodyFactFingerprintValue(command)
}

func bodyFactMessageFingerprint(command model.RecordBodyFactsCommand, source model.MessageFactSource) (string, error) {
	return bodyFactFingerprintValue(struct {
		Kind      string                       `json:"source_kind"`
		RunID     uuid.UUID                    `json:"run_id"`
		MessageID uuid.UUID                    `json:"message_id"`
		Command   model.RecordBodyFactsCommand `json:"command"`
	}{Kind: "user_message", RunID: source.RunID, MessageID: source.MessageID, Command: command})
}

func bodyFactFingerprintValue(payload any) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
