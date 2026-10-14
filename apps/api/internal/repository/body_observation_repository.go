package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RecordDirectObservation stores a self-reported measurement or nonnumeric
// observation as exactly the user's original statement. It never labels the
// value as device verified, clinically normal/abnormal, or a Diagnosis.
// Single BodyState revision + original evidence + observation + id-only
// outbox + original replay receipt all commit or roll back together.
func (r *BodyStateRepository) RecordDirectObservation(ctx context.Context, owner uuid.UUID, command model.RecordBodyObservationCommand) (*model.BodyObservationCommit, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyObservation)
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Source  string                             `json:"source_kind"`
		Command model.RecordBodyObservationCommand `json:"command"`
	}{Source: "direct_edit:user_report", Command: command})
	if err != nil {
		return nil, err
	}
	var output model.BodyObservationCommit
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		var prior model.BodyObservationCommit
		err = tx.Where("user_id=? AND idempotency_key=?", owner, command.IdempotencyKey).Take(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return model.ErrBodyObservationIdempotencyConflict
			}
			output = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if state.CurrentRevision != command.ExpectedRevision {
			return fmt.Errorf("%w: observation revision %d != current %d", ErrBodyStateRevisionConflict, command.ExpectedRevision, state.CurrentRevision)
		}
		// PostgreSQL timestamptz persists microsecond precision. Truncate
		// before the first HTTP reply so an identical replay returns the
		// exact same committed_at JSON, not a 9-vs-6-digit timestamp.
		now := time.Now().UTC().Truncate(time.Microsecond)
		commitID, observationID, evidenceID, eventID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		observedAt, err := json.Marshal(command.Observation.ObservedAt)
		if err != nil {
			return err
		}
		metadata := map[string]any{"commit_id": commitID, "observation_id": observationID, "source_evidence_id": evidenceID}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "observation.recorded", "direct_edit", metadata)
		if err != nil {
			return err
		}
		evidence := model.StatementEvidence{ID: evidenceID, UserID: owner,
			SourceKind: "direct_edit", SourceVersion: 1, Excerpt: command.Observation.Statement,
			Availability: "available", RecordedAt: now}
		if err := tx.Create(&evidence).Error; err != nil {
			return err
		}
		record := model.BodyObservation{ID: observationID, UserID: owner,
			Kind: command.Observation.Kind, RawValue: command.Observation.RawValue,
			RawUnit: command.Observation.RawUnit, Method: command.Observation.Method,
			MeasurementContext: command.Observation.MeasurementContext,
			ObservedAt:         datatypes.JSON(observedAt), SourceEvidenceID: evidenceID,
			AssertionStatus: "asserted", CreatedRevision: next, RecordedAt: now}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		event := model.BodyStateOutbox{ID: eventID, UserID: owner,
			Revision: next, EventType: "body_state.observation_recorded",
			Payload: datatypes.JSON(payload), CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		receipt := model.BodyObservationCommit{ID: commitID, UserID: owner,
			IdempotencyKey: command.IdempotencyKey, Fingerprint: fingerprint, Operation: "record",
			ObservationID: observationID, SourceEvidenceID: evidenceID,
			Revision: next, RevisionID: revision.ID, EventID: eventID, CommittedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		output = receipt
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &output, nil
}

type bodyObservationReadRow struct {
	model.BodyObservation
	Statement     string
	SourceVersion int64
}

// ListDirectObservations is an exact owner-scoped, revision-pinned source
// projection. A withdrawn/superseded observation or blocked/erased original
// source is never returned as current; zero results is NOT an assertion about
// the person's health. Historical raw copies remain in the original source
// pending an authenticated erasure workflow.
func (r *BodyStateRepository) ListDirectObservations(ctx context.Context, owner uuid.UUID, beforeRevision int64) (*model.BodyObservationPage, error) {
	if owner == uuid.Nil || beforeRevision < 0 {
		return nil, fmt.Errorf("%w: observation owner and non-negative cursor required", model.ErrInvalidBodyObservation)
	}
	var page *model.BodyObservationPage
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		page = &model.BodyObservationPage{UserID: owner, Observations: []model.BodyObservationRecord{}}
		var state model.BodyState
		if err := tx.Where("user_id=?", owner).Take(&state).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		page.CurrentRevision = state.CurrentRevision
		query := tx.Table("public.body_observations AS o").
			Joins("JOIN public.statement_evidence AS e ON e.user_id=o.user_id AND e.id=o.source_evidence_id").
			Where("o.user_id=? AND o.created_revision<=? AND o.assertion_status='asserted' AND e.availability='available' AND e.source_kind='direct_edit'", owner, state.CurrentRevision)
		if beforeRevision > 0 {
			query = query.Where("o.created_revision<?", beforeRevision)
		}
		var rows []bodyObservationReadRow
		if err := query.Select("o.*, e.excerpt AS statement, e.source_version AS source_version").
			Order("o.created_revision DESC, o.id ASC").Limit(21).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 20 {
			page.HasMore = true
			rows = rows[:20]
			cursor := rows[len(rows)-1].CreatedRevision
			page.NextBeforeRevision = &cursor
		}
		for _, item := range rows {
			if item.SourceVersion < 1 {
				return fmt.Errorf("%w: invalid observation provenance", model.ErrInvalidBodyObservation)
			}
			var observed model.FactTemporal
			if err := json.Unmarshal(item.ObservedAt, &observed); err != nil {
				return err
			}
			if err := observed.Validate(); err != nil {
				return err
			}
			page.Observations = append(page.Observations, model.BodyObservationRecord{
				ID: item.ID, UserID: item.UserID, Kind: item.Kind, Statement: item.Statement,
				RawValue: item.RawValue, RawUnit: item.RawUnit, Method: item.Method,
				MeasurementContext: item.MeasurementContext, ObservedAt: observed,
				Source:          model.BodyObservationSource{Kind: "user_report", ID: item.SourceEvidenceID, Version: item.SourceVersion},
				AssertionStatus: item.AssertionStatus, CreatedRevision: item.CreatedRevision, RecordedAt: item.RecordedAt,
			})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return page, nil
}
