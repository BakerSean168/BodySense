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

func (r *BodyStateRepository) CorrectDirectObservation(
	ctx context.Context, owner, target uuid.UUID, c model.AmendBodyObservationCommand,
) (*model.BodyObservationCommit, error) {
	return r.amendDirectObservation(ctx, owner, target, c, true)
}
func (r *BodyStateRepository) WithdrawDirectObservation(
	ctx context.Context, owner, target uuid.UUID, c model.AmendBodyObservationCommand,
) (*model.BodyObservationCommit, error) {
	return r.amendDirectObservation(ctx, owner, target, c, false)
}

// A correction changes the truthfulness of the OLD reported measurement,
// preserving old ID, original text and occurrence time in its source row.
// It does not represent a new reading taken at a later time. A withdrawal
// invalidates exactly one assertion; it does not mean the underlying health
// state ended or all controlled private copies have been erased.
func (r *BodyStateRepository) amendDirectObservation(
	ctx context.Context, owner, target uuid.UUID, c model.AmendBodyObservationCommand, correction bool,
) (*model.BodyObservationCommit, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner and exact observation required", model.ErrInvalidBodyObservation)
	}
	if err := c.Validate(correction); err != nil {
		return nil, err
	}
	operation, assertion, eventType, changeType := "withdraw", "withdrawn", "body_state.observation_withdrawn", "observation.withdrawn"
	if correction {
		operation, assertion, eventType, changeType = "correct", "superseded", "body_state.observation_corrected", "observation.corrected"
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                            `json:"operation"`
		Target    uuid.UUID                         `json:"target_observation_id"`
		Command   model.AmendBodyObservationCommand `json:"command"`
	}{Operation: operation, Target: target, Command: c})
	if err != nil {
		return nil, err
	}

	var receipt model.BodyObservationCommit
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		// Identity replay precedes stale-revision checks and source availability
		// re-evaluation. A successfully committed action is not reauthorized
		// merely because its older original source was subsequently removed.
		var prior model.BodyObservationCommit
		err = tx.Where("user_id=? AND idempotency_key=?", owner, c.IdempotencyKey).Take(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return model.ErrBodyObservationIdempotencyConflict
			}
			receipt = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var original bodyObservationReadRow
		query := tx.Table("public.body_observations AS o").
			Select("o.*,e.excerpt AS statement,e.source_version AS source_version").
			Joins("JOIN public.statement_evidence AS e ON e.id=o.source_evidence_id AND e.user_id=o.user_id").
			Where("o.user_id=? AND o.id=? AND e.source_kind='direct_edit'", owner, target)
		if correction {
			query = query.Where("e.availability='available'")
		}
		err = query.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&original).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrBodyObservationNotFound
		}
		if err != nil {
			return err
		}
		// Authorization is checked before optimistic revision. Otherwise a
		// foreign target with a stale revision leaks a 409 instead of owner-
		// scoped 404, even though no foreign health record is returned.
		if state.CurrentRevision != c.ExpectedRevision {
			return fmt.Errorf("%w: expected %d, current %d", ErrBodyStateRevisionConflict, c.ExpectedRevision, state.CurrentRevision)
		}
		if original.AssertionStatus != "asserted" {
			return model.ErrBodyObservationNotCurrent
		}

		now := time.Now().UTC().Truncate(time.Microsecond)
		commitID, eventID := uuid.New(), uuid.New()
		observationID, sourceID := target, original.SourceEvidenceID
		var replacement *model.BodyObservation
		var evidence *model.StatementEvidence
		if correction {
			var temporal model.FactTemporal
			if err := json.Unmarshal(original.ObservedAt, &temporal); err != nil {
				return err
			}
			previous := model.RecordBodyObservationInput{
				Kind: original.Kind, Statement: original.Statement, RawValue: original.RawValue,
				RawUnit: original.RawUnit, Method: original.Method,
				MeasurementContext: original.MeasurementContext, ObservedAt: temporal,
			}
			before, err := bodyFactFingerprintValue(previous)
			if err != nil {
				return err
			}
			after, err := bodyFactFingerprintValue(*c.Replacement)
			if err != nil {
				return err
			}
			if before == after {
				return fmt.Errorf("%w: correction contains no change to original report", model.ErrInvalidBodyObservation)
			}
			observedAt, err := json.Marshal(c.Replacement.ObservedAt)
			if err != nil {
				return err
			}
			observationID, sourceID = uuid.New(), uuid.New()
			evidence = &model.StatementEvidence{ID: sourceID, UserID: owner, SourceKind: "direct_edit",
				SourceVersion: 1, Excerpt: c.Replacement.Statement, Availability: "available", RecordedAt: now}
			replacement = &model.BodyObservation{
				ID: observationID, UserID: owner, Kind: c.Replacement.Kind,
				RawValue: c.Replacement.RawValue, RawUnit: c.Replacement.RawUnit,
				Method: c.Replacement.Method, MeasurementContext: c.Replacement.MeasurementContext,
				ObservedAt: datatypes.JSON(observedAt), SourceEvidenceID: sourceID,
				AssertionStatus: "asserted", SupersedesObservationID: &target,
				CreatedRevision: next, RecordedAt: now,
			}
		}

		// Generic audit/outbox never copies health statement, raw value/unit,
		// device claim, measurement method or user location. Only exact
		// identities and this bounded authorization kind are recorded.
		metadata := map[string]any{
			"commit_id": commitID, "target_observation_id": target,
			"observation_id": observationID, "source_evidence_id": sourceID,
			"authorization": "direct_user_command",
		}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, changeType, "direct_edit", metadata)
		if err != nil {
			return err
		}
		update := tx.Model(&model.BodyObservation{}).
			Where("user_id=? AND id=? AND assertion_status='asserted'", owner, target).
			Update("assertion_status", assertion)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return model.ErrBodyObservationNotCurrent
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
		event := model.BodyStateOutbox{ID: eventID, UserID: owner, Revision: next,
			EventType: eventType, Payload: datatypes.JSON(payload), CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		receipt = model.BodyObservationCommit{
			ID: commitID, UserID: owner, IdempotencyKey: c.IdempotencyKey,
			Fingerprint: fingerprint, Operation: operation, TargetObservationID: &target,
			ObservationID: observationID, SourceEvidenceID: sourceID,
			Revision: next, RevisionID: revision.ID, EventID: eventID, CommittedAt: now,
		}
		return tx.Create(&receipt).Error
	})
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}
