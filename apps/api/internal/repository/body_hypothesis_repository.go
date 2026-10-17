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
	"gorm.io/gorm/clause"
)

// maxActiveBodyHypotheses bounds the active projection; user-authored guesses
// are few, so the list is unpaginated until a product need says otherwise.
const maxActiveBodyHypotheses = 100

// RecordBodyHypothesis stores the user's own stated guess about a cause or
// possible condition. Single BodyState revision + direct_edit source + the
// hypothesis + owner-scoped fact links + id-only outbox + replay receipt all
// commit or roll back together. AI hypotheses never enter this path.
func (r *BodyStateRepository) RecordBodyHypothesis(ctx context.Context, owner uuid.UUID, command model.RecordBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyHypothesis)
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                            `json:"operation"`
		Source    string                            `json:"source_kind"`
		Command   model.RecordBodyHypothesisCommand `json:"command"`
	}{Operation: "record", Source: "direct_edit:user", Command: command})
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
		err = tx.Where("user_id=? AND idempotency_key=?", owner, command.IdempotencyKey).Take(&prior).Error
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
		// Ownership of every linked fact/observation is checked before the
		// optimistic revision so a foreign id yields not-found, never a
		// cross-tenant revision oracle.
		if err := checkBodyHypothesisLinks(tx, owner, command.RelatedFactIDs, command.RelatedObservationIDs); err != nil {
			return err
		}
		if state.CurrentRevision != command.ExpectedRevision {
			return fmt.Errorf("%w: hypothesis revision %d != current %d", ErrBodyStateRevisionConflict, command.ExpectedRevision, state.CurrentRevision)
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		evidence := model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "direct_edit",
			SourceVersion: 1, Excerpt: command.Statement, Availability: "available", RecordedAt: now}
		commit, err := persistBodyHypothesisRecord(ctx, tx, state, next, owner, command.IdempotencyKey, fingerprint, command.Statement,
			command.RelatedFactIDs, command.RelatedObservationIDs, evidence, "direct_edit", nil, now)
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

// WithdrawBodyHypothesis retracts exactly one asserted user guess. The row,
// its original source and fact links are preserved as history; the linked
// facts/observations are untouched. Withdrawal is not erasure.
func (r *BodyStateRepository) WithdrawBodyHypothesis(ctx context.Context, owner, target uuid.UUID, command model.WithdrawBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner and exact hypothesis required", model.ErrInvalidBodyHypothesis)
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                              `json:"operation"`
		Target    uuid.UUID                           `json:"target_hypothesis_id"`
		Command   model.WithdrawBodyHypothesisCommand `json:"command"`
	}{Operation: "withdraw", Target: target, Command: command})
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
		err = tx.Where("user_id=? AND idempotency_key=?", owner, command.IdempotencyKey).Take(&prior).Error
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
		var original model.BodyHypothesis
		err = tx.Where("user_id=? AND id=?", owner, target).
			Clauses(clause.Locking{Strength: "UPDATE"}).Take(&original).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrBodyHypothesisNotFound
		}
		if err != nil {
			return err
		}
		if state.CurrentRevision != command.ExpectedRevision {
			return fmt.Errorf("%w: expected %d, current %d", ErrBodyStateRevisionConflict, command.ExpectedRevision, state.CurrentRevision)
		}
		if original.AssertionStatus != "asserted" {
			return model.ErrBodyHypothesisNotCurrent
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		commitID, eventID := uuid.New(), uuid.New()
		metadata := map[string]any{"commit_id": commitID, "hypothesis_id": target, "target_hypothesis_id": target,
			"lineage_id": original.LineageID, "source_evidence_id": original.SourceEvidenceID,
			"authorization": "direct_user_command"}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "hypothesis.withdrawn", "direct_edit", metadata)
		if err != nil {
			return err
		}
		var reason *string
		if command.Reason != "" {
			reason = &command.Reason
		}
		update := tx.Model(&model.BodyHypothesis{}).
			Where("user_id=? AND id=? AND assertion_status='asserted'", owner, target).
			Updates(map[string]any{"assertion_status": "withdrawn", "withdrawn_at": now,
				"withdrawn_revision": next, "withdrawal_reason": reason})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return model.ErrBodyHypothesisNotCurrent
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		event := model.BodyStateOutbox{ID: eventID, UserID: owner, Revision: next,
			EventType: "body_state.hypothesis_withdrawn", Payload: datatypes.JSON(payload), CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		output = model.BodyHypothesisCommit{ID: commitID, UserID: owner, IdempotencyKey: command.IdempotencyKey,
			Fingerprint: fingerprint, Operation: "withdraw", HypothesisID: target, TargetHypothesisID: &target,
			SourceEvidenceID: original.SourceEvidenceID, Revision: next, RevisionID: revision.ID,
			EventID: eventID, CommittedAt: now}
		return tx.Create(&output).Error
	})
	if err != nil {
		return nil, err
	}
	return &output, nil
}

type bodyHypothesisReadRow struct {
	model.BodyHypothesis
	SourceKind    string
	SourceVersion int64
}

// CorrectBodyHypothesis is the user's direct edit of exactly one of their own
// current guesses. The original row, its source and its links stay as history
// with assertion_status 'superseded'; a new hypothesis in the same lineage
// (supersedes_hypothesis_id = target) carries the complete replacement. One
// revision, one direct_edit source, id-only outbox and receipt, one tx. The
// linked facts and observations themselves are never touched.
func (r *BodyStateRepository) CorrectBodyHypothesis(ctx context.Context, owner, target uuid.UUID, command model.CorrectBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner and exact hypothesis required", model.ErrInvalidBodyHypothesis)
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	replacement := *command.Replacement
	replacement.RelatedFactIDs = factIDsOrEmpty(replacement.RelatedFactIDs)
	replacement.RelatedObservationIDs = factIDsOrEmpty(replacement.RelatedObservationIDs)
	command.Replacement = &replacement // canonical (non-null) lists for the fingerprint
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                             `json:"operation"`
		Target    uuid.UUID                          `json:"target_hypothesis_id"`
		Command   model.CorrectBodyHypothesisCommand `json:"command"`
	}{Operation: "correct", Target: target, Command: command})
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
		err = tx.Where("user_id=? AND idempotency_key=?", owner, command.IdempotencyKey).Take(&prior).Error
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
		// Owner-scoped target with a still-available original source; a
		// foreign or erased target is not-found before any revision check.
		var original model.BodyHypothesis
		err = tx.Table("public.body_hypotheses AS h").Select("h.*").
			Joins("JOIN public.statement_evidence AS e ON e.user_id=h.user_id AND e.id=h.source_evidence_id").
			Where("h.user_id=? AND h.id=? AND e.availability='available'", owner, target).
			Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "h"}}).Take(&original).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrBodyHypothesisNotFound
		}
		if err != nil {
			return err
		}
		if err := checkBodyHypothesisLinks(tx, owner, replacement.RelatedFactIDs, replacement.RelatedObservationIDs); err != nil {
			return err
		}
		if state.CurrentRevision != command.ExpectedRevision {
			return fmt.Errorf("%w: expected %d, current %d", ErrBodyStateRevisionConflict, command.ExpectedRevision, state.CurrentRevision)
		}
		if original.AssertionStatus != "asserted" {
			return model.ErrBodyHypothesisNotCurrent
		}
		var storedFacts, storedObservations []uuid.UUID
		if err := tx.Model(&model.BodyHypothesisFactLink{}).Where("user_id=? AND hypothesis_id=?", owner, target).
			Order("ordinal").Pluck("fact_id", &storedFacts).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BodyHypothesisObservationLink{}).Where("user_id=? AND hypothesis_id=?", owner, target).
			Order("ordinal").Pluck("observation_id", &storedObservations).Error; err != nil {
			return err
		}
		if original.Statement == replacement.Statement && sameUUIDs(storedFacts, replacement.RelatedFactIDs) &&
			sameUUIDs(storedObservations, replacement.RelatedObservationIDs) {
			return fmt.Errorf("%w: correction contains no change to the original hypothesis", model.ErrInvalidBodyHypothesis)
		}

		now := time.Now().UTC().Truncate(time.Microsecond)
		commitID, hypothesisID, evidenceID, eventID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		metadata := map[string]any{"commit_id": commitID, "target_hypothesis_id": target,
			"hypothesis_id": hypothesisID, "lineage_id": original.LineageID, "source_evidence_id": evidenceID,
			"related_fact_ids":        factIDsOrEmpty(replacement.RelatedFactIDs),
			"related_observation_ids": factIDsOrEmpty(replacement.RelatedObservationIDs),
			"authorization":           "direct_user_command"}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "hypothesis.corrected", "direct_edit", metadata)
		if err != nil {
			return err
		}
		update := tx.Model(&model.BodyHypothesis{}).
			Where("user_id=? AND id=? AND assertion_status='asserted'", owner, target).
			Update("assertion_status", "superseded")
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return model.ErrBodyHypothesisNotCurrent
		}
		evidence := model.StatementEvidence{ID: evidenceID, UserID: owner, SourceKind: "direct_edit",
			SourceVersion: 1, Excerpt: replacement.Statement, Availability: "available", RecordedAt: now}
		if err := tx.Create(&evidence).Error; err != nil {
			return err
		}
		successor := model.BodyHypothesis{ID: hypothesisID, UserID: owner, Statement: replacement.Statement,
			SourceEvidenceID: evidenceID, AssertionStatus: "asserted", CreatedRevision: next, RecordedAt: now,
			SupersedesHypothesisID: &target, LineageID: original.LineageID}
		if err := tx.Create(&successor).Error; err != nil {
			return err
		}
		if err := createBodyHypothesisLinks(tx, owner, hypothesisID, replacement.RelatedFactIDs, replacement.RelatedObservationIDs); err != nil {
			return err
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		event := model.BodyStateOutbox{ID: eventID, UserID: owner, Revision: next,
			EventType: "body_state.hypothesis_corrected", Payload: datatypes.JSON(payload), CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		output = model.BodyHypothesisCommit{ID: commitID, UserID: owner, IdempotencyKey: command.IdempotencyKey,
			Fingerprint: fingerprint, Operation: "correct", HypothesisID: hypothesisID, TargetHypothesisID: &target,
			SourceEvidenceID: evidenceID, Revision: next, RevisionID: revision.ID, EventID: eventID, CommittedAt: now}
		return tx.Create(&output).Error
	})
	if err != nil {
		return nil, err
	}
	return &output, nil
}

// checkBodyHypothesisLinks verifies, inside the write tx, that every linked
// fact and observation is the owner's own current (asserted) record.
func checkBodyHypothesisLinks(tx *gorm.DB, owner uuid.UUID, facts, observations []uuid.UUID) error {
	if len(facts) > 0 {
		var found int64
		if err := tx.Model(&model.BodyFact{}).
			Where("user_id=? AND id IN ? AND assertion_status='asserted'", owner, facts).
			Count(&found).Error; err != nil {
			return err
		}
		if found != int64(len(facts)) {
			return model.ErrBodyHypothesisFactNotFound
		}
	}
	if len(observations) > 0 {
		var found int64
		if err := tx.Model(&model.BodyObservation{}).
			Where("user_id=? AND id IN ? AND assertion_status='asserted'", owner, observations).
			Count(&found).Error; err != nil {
			return err
		}
		if found != int64(len(observations)) {
			return model.ErrBodyHypothesisObservationNotFound
		}
	}
	return nil
}

func createBodyHypothesisLinks(tx *gorm.DB, owner, hypothesisID uuid.UUID, facts, observations []uuid.UUID) error {
	for i, factID := range facts {
		link := model.BodyHypothesisFactLink{UserID: owner, HypothesisID: hypothesisID, FactID: factID, Ordinal: int16(i)}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
	}
	for i, observationID := range observations {
		link := model.BodyHypothesisObservationLink{UserID: owner, HypothesisID: hypothesisID, ObservationID: observationID, Ordinal: int16(i)}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
	}
	return nil
}

func sameUUIDs(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ListActiveBodyHypotheses is an owner-scoped, revision-pinned projection of
// asserted user guesses whose original source is still available. Linked fact
// ids are filtered to facts that are still asserted; a withdrawn/superseded
// fact silently drops out of the link list but the stored link is kept.
func (r *BodyStateRepository) ListActiveBodyHypotheses(ctx context.Context, owner uuid.UUID) (*model.BodyHypothesisList, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyHypothesis)
	}
	var list *model.BodyHypothesisList
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		list = &model.BodyHypothesisList{UserID: owner, Hypotheses: []model.BodyHypothesisRecord{}}
		var state model.BodyState
		if err := tx.Where("user_id=?", owner).Take(&state).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		list.CurrentRevision = state.CurrentRevision
		var rows []bodyHypothesisReadRow
		if err := tx.Table("public.body_hypotheses AS h").
			Joins("JOIN public.statement_evidence AS e ON e.user_id=h.user_id AND e.id=h.source_evidence_id").
			Where("h.user_id=? AND h.created_revision<=? AND h.assertion_status='asserted' AND e.availability='available'", owner, state.CurrentRevision).
			Select("h.*, e.source_kind AS source_kind, e.source_version AS source_version").
			Order("h.created_revision DESC, h.id ASC").Limit(maxActiveBodyHypotheses).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, len(rows))
		for i, row := range rows {
			ids[i] = row.ID
		}
		var links []model.BodyHypothesisFactLink
		if err := tx.Table("public.body_hypothesis_fact_links AS l").
			Joins("JOIN public.body_facts AS f ON f.user_id=l.user_id AND f.id=l.fact_id").
			Where("l.user_id=? AND l.hypothesis_id IN ? AND f.assertion_status='asserted'", owner, ids).
			Select("l.*").Order("l.hypothesis_id, l.ordinal").Scan(&links).Error; err != nil {
			return err
		}
		byHypothesis := map[uuid.UUID][]uuid.UUID{}
		for _, link := range links {
			byHypothesis[link.HypothesisID] = append(byHypothesis[link.HypothesisID], link.FactID)
		}
		var observationLinks []model.BodyHypothesisObservationLink
		if err := tx.Table("public.body_hypothesis_observation_links AS l").
			Joins("JOIN public.body_observations AS o ON o.user_id=l.user_id AND o.id=l.observation_id").
			Where("l.user_id=? AND l.hypothesis_id IN ? AND o.assertion_status='asserted' AND o.created_revision<=?", owner, ids, state.CurrentRevision).
			Select("l.*").Order("l.hypothesis_id, l.ordinal").Scan(&observationLinks).Error; err != nil {
			return err
		}
		observationsByHypothesis := map[uuid.UUID][]uuid.UUID{}
		for _, link := range observationLinks {
			observationsByHypothesis[link.HypothesisID] = append(observationsByHypothesis[link.HypothesisID], link.ObservationID)
		}
		for _, row := range rows {
			if row.SourceVersion < 1 {
				return fmt.Errorf("%w: invalid hypothesis provenance", model.ErrInvalidBodyHypothesis)
			}
			list.Hypotheses = append(list.Hypotheses, model.BodyHypothesisRecord{
				ID: row.ID, UserID: row.UserID, LineageID: row.LineageID,
				SupersedesHypothesisID: row.SupersedesHypothesisID, Statement: row.Statement,
				Source:                model.BodyHypothesisSource{Actor: "user", Kind: row.SourceKind, ID: row.SourceEvidenceID, Version: row.SourceVersion},
				RelatedFactIDs:        factIDsOrEmpty(byHypothesis[row.ID]),
				RelatedObservationIDs: factIDsOrEmpty(observationsByHypothesis[row.ID]),
				AssertionStatus:       row.AssertionStatus, CreatedRevision: row.CreatedRevision, RecordedAt: row.RecordedAt,
			})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func factIDsOrEmpty(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}
