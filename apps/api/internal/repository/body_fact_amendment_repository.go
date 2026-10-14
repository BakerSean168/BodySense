package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *BodyStateRepository) CorrectDirectFact(ctx context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand) (*model.BodyFactCommit, error) {
	return r.amendDirectFact(ctx, owner, target, command, true)
}
func (r *BodyStateRepository) WithdrawDirectFact(ctx context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand) (*model.BodyFactCommit, error) {
	return r.amendDirectFact(ctx, owner, target, command, false)
}

// amendDirectFact is an explicit, authenticated user command. Correction keeps
// the original source and creates a replacement; withdrawal only invalidates
// the assertion. Neither means symptom resolution or health-data erasure.
func (r *BodyStateRepository) amendDirectFact(ctx context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand, correction bool) (*model.BodyFactCommit, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, fmt.Errorf("%w: owner and exact target required", model.ErrInvalidBodyFact)
	}
	if err := command.Validate(correction); err != nil {
		return nil, err
	}
	operation, assertion, eventType := "facts.withdrawn", "withdrawn", "body_state.facts_withdrawn"
	if correction {
		operation, assertion, eventType = "facts.corrected", "superseded", "body_state.facts_corrected"
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                     `json:"operation"`
		Target    uuid.UUID                  `json:"target_id"`
		Command   model.AmendBodyFactCommand `json:"command"`
	}{operation, target, command})
	if err != nil {
		return nil, err
	}
	var receipt model.BodyFactCommit
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		var prior model.BodyFactCommit
		err = tx.Where("user_id = ? AND idempotency_key = ?", owner, command.IdempotencyKey).First(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return model.ErrBodyFactIdempotencyConflict
			}
			receipt = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if state.CurrentRevision != command.ExpectedRevision {
			return fmt.Errorf("%w: expected %d, current %d", ErrBodyStateRevisionConflict, command.ExpectedRevision, state.CurrentRevision)
		}
		var original model.BodyFact
		err = tx.Table("body_facts AS f").Select("f.*").
			Joins("JOIN statement_evidence AS e ON e.id = f.source_evidence_id AND e.user_id = f.user_id").
			Where("f.user_id = ? AND f.id = ? AND e.availability = ?", owner, target, "available").
			Clauses(clause.Locking{Strength: "UPDATE"}).Take(&original).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrBodyFactNotFound
		}
		if err != nil {
			return err
		}
		if original.AssertionStatus != "asserted" {
			return model.ErrBodyFactNotCurrent
		}

		now := time.Now().UTC()
		commitID, eventID := uuid.New(), uuid.New()
		factIDs := []uuid.UUID{target}
		evidenceIDs := []uuid.UUID{original.SourceEvidenceID}
		var replacement *model.BodyFact
		var evidence *model.StatementEvidence
		if correction {
			input := *command.Replacement
			regions, err := json.Marshal(input.BodyRegionIDs)
			if err != nil {
				return err
			}
			temporal, err := json.Marshal(input.Temporal)
			if err != nil {
				return err
			}
			var previous model.BodyFactInput
			previous.Kind, previous.Statement, previous.Details = original.Kind, original.Statement, json.RawMessage(original.Details)
			if err := json.Unmarshal(original.BodyRegionIDs, &previous.BodyRegionIDs); err != nil {
				return err
			}
			if err := json.Unmarshal(original.Temporal, &previous.Temporal); err != nil {
				return err
			}
			before, err := bodyFactFingerprintValue(previous)
			if err != nil {
				return err
			}
			after, err := bodyFactFingerprintValue(input)
			if err != nil {
				return err
			}
			if before == after {
				return fmt.Errorf("%w: correction contains no change", model.ErrInvalidBodyFact)
			}
			evidence = &model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "direct_edit", SourceVersion: 1, Excerpt: input.Statement, Availability: "available", RecordedAt: now}
			replacement = &model.BodyFact{
				ID: uuid.New(), UserID: owner, Subject: "self", Kind: input.Kind, Statement: input.Statement,
				BodyRegionIDs: datatypes.JSON(regions), Temporal: datatypes.JSON(temporal), Details: datatypes.JSON(input.Details),
				SourceEvidenceID: evidence.ID, AssertionStatus: "asserted", CreatedRevision: next, RecordedAt: now, SupersedesFactID: &target,
			}
			factIDs = []uuid.UUID{replacement.ID}
			evidenceIDs = append(evidenceIDs, evidence.ID)
		}
		// Only IDs and authorization kind enter general audit; the user's text
		// stays in the owning fact/source rows for eventual selective erasure.
		metadata := map[string]any{"commit_id": commitID, "target_fact_id": target, "fact_ids": factIDs, "source_evidence_ids": evidenceIDs, "authorization": "direct_user_command", "assertion_status": assertion}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, operation, "direct_edit", metadata)
		if err != nil {
			return err
		}
		update := tx.Model(&model.BodyFact{}).Where("id = ? AND user_id = ? AND assertion_status = ?", target, owner, "asserted").Update("assertion_status", assertion)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return model.ErrBodyFactNotCurrent
		}
		if replacement != nil {
			if err := tx.Create(evidence).Error; err != nil {
				return err
			}
			if err := tx.Create(replacement).Error; err != nil {
				return err
			}
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		event := model.BodyStateOutbox{ID: eventID, UserID: owner, Revision: next, EventType: eventType, Payload: datatypes.JSON(payload), CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		encoded, err := json.Marshal(factIDs)
		if err != nil {
			return err
		}
		receipt = model.BodyFactCommit{ID: commitID, UserID: owner, IdempotencyKey: command.IdempotencyKey, Fingerprint: fingerprint, Revision: next, RevisionID: revision.ID, EventID: eventID, FactIDs: datatypes.JSON(encoded), CommittedAt: now}
		return tx.Create(&receipt).Error
	})
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}
