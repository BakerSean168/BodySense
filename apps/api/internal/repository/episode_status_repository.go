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

// AdmitEpisodeStatus does not accept an asserted clinical status from HTTP.
// It only admits a preexisting explicit course report already stored in an
// owner-bound symptom Fact and linked to exactly this Episode. The caller
// supplies immutable identity, exact source Fact and optimistic revision.
// Only a user-authored, structured direct-edit Fact can enter this bounded
// path; AI-extracted message claims require a separately verified High-evidence
// gate and cannot impersonate a direct explicit user assertion.
// Generic absent or present is never enough to assert an Episode's course.
func (r *BodyStateRepository) AdmitEpisodeStatus(ctx context.Context, owner, episodeID uuid.UUID, c model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error) {
	if owner == uuid.Nil || episodeID == uuid.Nil {
		return nil, fmt.Errorf("%w: owner and exact Episode required", model.ErrInvalidEpisodeEvidence)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                          `json:"operation"`
		Target    uuid.UUID                       `json:"episode_id"`
		Command   model.EpisodeAssociationCommand `json:"command"`
	}{"status", episodeID, c})
	if err != nil {
		return nil, err
	}
	var receipt model.EpisodeCommandReceipt
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		// Stable replay is decided before checking the present revision and
		// source availability. A later correction/erasure must not create
		// another accepted status when the client retries its old key.
		var prior model.EpisodeCommandReceipt
		err = tx.Where("user_id=? AND idempotency_key=?", owner, c.IdempotencyKey).First(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return model.ErrEpisodeIdempotencyConflict
			}
			receipt = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Exact user-selected target must remain current and source-visible,
		// not an abandoned or superseded Episode identity.
		var episode model.SymptomEpisode
		err = tx.Where("user_id=? AND id=? AND identity_status='current'", owner, episodeID).Take(&episode).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEpisodeNotFound
		}
		if err != nil {
			return err
		}
		// Probe the owner-scoped target BEFORE revision comparison. Otherwise
		// a foreign Episode ID with a stale caller revision incorrectly returns
		// 409 instead of a non-disclosing 404.
		if state.CurrentRevision != c.ExpectedRevision {
			return fmt.Errorf("%w: stale Episode evidence revision", ErrBodyStateRevisionConflict)
		}
		var fact model.BodyFact
		err = tx.Table("public.body_facts AS f").Select("f.*").
			Joins("JOIN public.episode_fact_links AS l ON l.user_id=f.user_id AND l.fact_id=f.id AND l.basis_evidence_id=f.source_evidence_id").
			Joins("JOIN public.statement_evidence AS ev ON ev.user_id=f.user_id AND ev.id=f.source_evidence_id").
			Where("f.user_id=? AND f.id=? AND f.kind='symptom' AND f.subject='self' AND f.assertion_status='asserted' AND l.episode_id=? AND l.is_primary=TRUE AND ev.availability='available' AND ev.source_kind='direct_edit'",
				owner, c.FactID, episodeID).
			Clauses(clause.Locking{Strength: "SHARE"}).Take(&fact).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrEpisodeFactUnavailable
		}
		if err != nil {
			return err
		}
		var details model.SymptomFactDetails
		if err := json.Unmarshal(fact.Details, &details); err != nil {
			return fmt.Errorf("%w: invalid explicit symptom basis", model.ErrInvalidEpisodeEvidence)
		}
		claim := details.EpisodeCourseReport
		if claim == "" || (claim != "explicit_current_report" && claim != "explicit_resolution") ||
			(claim == "explicit_current_report" && details.Presence != "present") ||
			(claim == "explicit_resolution" && details.Presence != "absent") {
			return model.ErrEpisodeAssociationConflict
		}
		if !strings.EqualFold(episode.SymptomType, strings.TrimSpace(details.SymptomType)) {
			return model.ErrEpisodeAssociationConflict
		}
		var temporal model.FactTemporal
		if err := json.Unmarshal(fact.Temporal, &temporal); err != nil || temporal.Validate() != nil {
			return fmt.Errorf("%w: source Fact has no valid authored time", model.ErrInvalidEpisodeEvidence)
		}
		// A new current report after a securely ended course must become a
		// separately authorized recurrence identity, not reopen the old ID.
		if claim == "explicit_current_report" {
			page, err := listEpisodeSnapshot(tx, owner, 0, &episodeID)
			if err != nil {
				return err
			}
			if len(page.Episodes) != 1 {
				return ErrEpisodeNotFound
			}
			if page.Episodes[0].Projection.State == "ended" {
				return model.ErrEpisodeAssociationConflict
			}
		}
		// Existing PK enforces at most one status basis for each Fact/Episode.
		// Any later correction or withdrawal invalidates the source and the
		// projection is derived again by episode_read_repository.go.
		var previous model.EpisodeStateEvidence
		err = tx.Where("user_id=? AND episode_id=? AND fact_id=?", owner, episodeID, fact.ID).Take(&previous).Error
		if err == nil {
			return model.ErrEpisodeAssociationConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		asOf, err := json.Marshal(temporal)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		commandID, eventID := uuid.New(), uuid.New()
		metadata := map[string]any{
			"commit_id": commandID, "episode_id": episodeID,
			"fact_id": fact.ID, "source_evidence_id": fact.SourceEvidenceID,
			"status_basis": claim, "authorization": "direct_user_command",
			"operation": "status",
		}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "episode.status", "direct_edit", metadata)
		if err != nil {
			return err
		}
		status := model.EpisodeStateEvidence{
			UserID: owner, EpisodeID: episodeID, FactID: fact.ID,
			SourceEvidenceID: fact.SourceEvidenceID,
			Claim:            claim, AsOf: datatypes.JSON(asOf), CreatedRevision: next,
		}
		if err := tx.Create(&status).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		event := model.BodyStateOutbox{
			ID: eventID, UserID: owner, Revision: next,
			EventType: "body_state.episode_status",
			Payload:   datatypes.JSON(payload), CreatedAt: now,
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		receipt = model.EpisodeCommandReceipt{
			ID: commandID, UserID: owner, IdempotencyKey: c.IdempotencyKey,
			Fingerprint: fingerprint, Operation: "status",
			EpisodeID: episodeID, FactID: fact.ID,
			Revision: next, RevisionID: revision.ID,
			EventID: eventID, CommittedAt: now,
		}
		return tx.Create(&receipt).Error
	})
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}
