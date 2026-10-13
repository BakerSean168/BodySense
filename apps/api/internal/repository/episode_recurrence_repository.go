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

// ReportEpisodeRecurrence records one authenticated user's EXPLICIT choice
// that a distinct later symptom Episode is a recurrence of a source-backed,
// explicitly ended previous Episode. It never infers shared medical etiology,
// silently reopens the former Episode, copies health text, or merges their Facts.
func (r *BodyStateRepository) ReportEpisodeRecurrence(ctx context.Context, owner, oldEpisodeID uuid.UUID, c model.EpisodeRecurrenceCommand) (*model.EpisodeRecurrenceReceipt, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidEpisodeEvidence)
	}
	if err := c.Validate(oldEpisodeID); err != nil {
		return nil, err
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                         `json:"operation"`
		From      uuid.UUID                      `json:"from_episode_id"`
		Command   model.EpisodeRecurrenceCommand `json:"command"`
	}{"recurrence", oldEpisodeID, c})
	if err != nil {
		return nil, err
	}

	var output model.EpisodeRecurrenceReceipt
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		var prior model.EpisodeCommandReceipt
		err = tx.Where("user_id=? AND idempotency_key=?", owner, c.IdempotencyKey).First(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return model.ErrEpisodeIdempotencyConflict
			}
			output, err = loadEpisodeRecurrenceReceipt(tx, owner, prior)
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Validate both exact owner-scoped identity targets before revision
		// checking: a foreign target must never be leaked as a stale 409.
		var oldEpisode, newEpisode model.SymptomEpisode
		for _, item := range []struct {
			id  uuid.UUID
			out *model.SymptomEpisode
		}{{oldEpisodeID, &oldEpisode}, {c.NewEpisodeID, &newEpisode}} {
			err = tx.Where("user_id=? AND id=? AND identity_status='current'", owner, item.id).Take(item.out).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEpisodeNotFound
			}
			if err != nil {
				return err
			}
		}
		if state.CurrentRevision != c.ExpectedRevision {
			return fmt.Errorf("%w: recurrence source snapshot changed", ErrBodyStateRevisionConflict)
		}
		// The new course's real, CURRENT, primary-linked self-report and the
		// user's original direct-edit source are held stable until COMMIT.
		var basis model.BodyFact
		err = tx.Table("public.body_facts AS f").Select("f.*").
			Joins("JOIN public.episode_fact_links AS l ON l.user_id=f.user_id AND l.fact_id=f.id AND l.basis_evidence_id=f.source_evidence_id").
			Joins("JOIN public.statement_evidence AS ev ON ev.user_id=f.user_id AND ev.id=f.source_evidence_id").
			Where("f.user_id=? AND f.id=? AND f.kind='symptom' AND f.subject='self' AND f.assertion_status='asserted' AND l.episode_id=? AND l.is_primary=TRUE AND ev.availability='available' AND ev.source_kind='direct_edit'",
				owner, c.FactID, c.NewEpisodeID).
			Clauses(clause.Locking{Strength: "SHARE"}).Take(&basis).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrEpisodeFactUnavailable
		}
		if err != nil {
			return err
		}
		var details model.SymptomFactDetails
		if err := json.Unmarshal(basis.Details, &details); err != nil {
			return model.ErrInvalidEpisodeEvidence
		}
		if details.Presence != "present" || !strings.EqualFold(strings.TrimSpace(details.SymptomType), newEpisode.SymptomType) ||
			!recurrenceIdentityCompatible(oldEpisode, newEpisode) {
			return model.ErrEpisodeAssociationConflict
		}
		// Explicit end proof must remain asserted and source-available for
		// the whole transaction, never guessed from a negative observation.
		var oldProof []model.BodyFact
		err = tx.Table("public.body_facts AS f").Select("f.*").
			Joins("JOIN public.episode_state_evidence AS e ON e.user_id=f.user_id AND e.fact_id=f.id AND e.source_evidence_id=f.source_evidence_id AND e.episode_id=?", oldEpisodeID).
			Joins("JOIN public.statement_evidence AS ev ON ev.user_id=f.user_id AND ev.id=f.source_evidence_id").
			Where("f.user_id=? AND f.kind='symptom' AND f.assertion_status='asserted' AND ev.availability='available' AND e.claim='explicit_resolution'", owner).
			Clauses(clause.Locking{Strength: "SHARE"}).Find(&oldProof).Error
		if err != nil {
			return err
		}
		if len(oldProof) == 0 {
			return model.ErrEpisodeAssociationConflict
		}
		// Do not invent an onset/recurrence date when only uncertain time is
		// reported. But if two precise *user-authored business times* prove
		// that the alleged new onset came BEFORE the old resolution, reject
		// the contradictory relation instead of rewriting history.
		var newTime model.FactTemporal
		if err := json.Unmarshal(basis.Temporal, &newTime); err != nil || newTime.Validate() != nil {
			return model.ErrInvalidEpisodeEvidence
		}
		for _, closed := range oldProof {
			var closedTime model.FactTemporal
			if err := json.Unmarshal(closed.Temporal, &closedTime); err != nil || closedTime.Validate() != nil {
				return model.ErrInvalidEpisodeEvidence
			}
			order, comparable := model.CompareEpisodeEvidenceTime(closedTime, newTime)
			if comparable && order > 0 {
				return model.ErrEpisodeAssociationConflict
			}
		}
		oldPage, err := listEpisodeSnapshot(tx, owner, 0, &oldEpisodeID)
		if err != nil {
			return err
		}
		if len(oldPage.Episodes) != 1 {
			return ErrEpisodeNotFound
		}
		if oldPage.Episodes[0].Projection.State != "ended" {
			return model.ErrEpisodeAssociationConflict
		}
		// The new Episode must still be visible from an accepted source.
		newPage, err := listEpisodeSnapshot(tx, owner, 0, &c.NewEpisodeID)
		if err != nil {
			return err
		}
		if len(newPage.Episodes) != 1 {
			return ErrEpisodeNotFound
		}

		var duplicate model.EpisodeRelation
		err = tx.Where("user_id=? AND from_episode_id=? AND to_episode_id=? AND kind='recurrence_reported'", owner, oldEpisodeID, c.NewEpisodeID).Take(&duplicate).Error
		if err == nil {
			return model.ErrEpisodeAssociationConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Recurrence is a directed historical DAG. Prevent loops across
		// existing accepted relations, not only the direct inverse edge.
		var cycles int64
		err = tx.Raw(`WITH RECURSIVE reachable(id) AS (
            SELECT to_episode_id FROM public.episode_relations WHERE user_id=? AND from_episode_id=?
            UNION
            SELECT r.to_episode_id FROM public.episode_relations AS r
            JOIN reachable AS p ON p.id=r.from_episode_id WHERE r.user_id=?
        ) SELECT count(*) FROM reachable WHERE id=?`, owner, c.NewEpisodeID, owner, oldEpisodeID).Scan(&cycles).Error
		if err != nil {
			return err
		}
		if cycles != 0 {
			return model.ErrEpisodeAssociationConflict
		}

		now := time.Now().UTC()
		commitID, eventID, relationID := uuid.New(), uuid.New(), uuid.New()
		metadata := map[string]any{"commit_id": commitID, "relation_id": relationID,
			"from_episode_id": oldEpisodeID, "to_episode_id": c.NewEpisodeID,
			"basis_fact_id": basis.ID, "basis_evidence_id": basis.SourceEvidenceID,
			"authorization": "direct_user_command", "operation": "recurrence"}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "episode.recurrence", "direct_edit", metadata)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		event := model.BodyStateOutbox{ID: eventID, UserID: owner, Revision: next,
			EventType: "body_state.episode_recurrence_reported",
			Payload:   datatypes.JSON(payload), CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		receipt := model.EpisodeCommandReceipt{ID: commitID, UserID: owner,
			IdempotencyKey: c.IdempotencyKey, Fingerprint: fingerprint,
			Operation: "recurrence", EpisodeID: c.NewEpisodeID, FactID: basis.ID,
			Revision: next, RevisionID: revision.ID, EventID: eventID, CommittedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		relation := model.EpisodeRelation{ID: relationID, UserID: owner,
			FromEpisodeID: oldEpisodeID, ToEpisodeID: c.NewEpisodeID,
			Kind: "recurrence_reported", BasisFactID: basis.ID,
			BasisEvidenceID: basis.SourceEvidenceID, AuthorizationID: commitID,
			CreatedRevision: next, CreatedAt: now}
		if err := tx.Create(&relation).Error; err != nil {
			return err
		}
		output = makeEpisodeRecurrenceReceipt(receipt, relation)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func loadEpisodeRecurrenceReceipt(tx *gorm.DB, owner uuid.UUID, receipt model.EpisodeCommandReceipt) (model.EpisodeRecurrenceReceipt, error) {
	var relation model.EpisodeRelation
	err := tx.Where("user_id=? AND authorization_id=?", owner, receipt.ID).Take(&relation).Error
	if err != nil {
		return model.EpisodeRecurrenceReceipt{}, fmt.Errorf("%w: recurrence receipt missing relation: %v", model.ErrInvalidEpisodeEvidence, err)
	}
	if receipt.Operation != "recurrence" || receipt.EpisodeID != relation.ToEpisodeID || receipt.FactID != relation.BasisFactID {
		return model.EpisodeRecurrenceReceipt{}, model.ErrInvalidEpisodeEvidence
	}
	return makeEpisodeRecurrenceReceipt(receipt, relation), nil
}
func makeEpisodeRecurrenceReceipt(receipt model.EpisodeCommandReceipt, relation model.EpisodeRelation) model.EpisodeRecurrenceReceipt {
	return model.EpisodeRecurrenceReceipt{ID: receipt.ID, UserID: receipt.UserID,
		RelationID: relation.ID, FromEpisodeID: relation.FromEpisodeID,
		ToEpisodeID: relation.ToEpisodeID, FactID: receipt.FactID,
		Revision: receipt.Revision, RevisionID: receipt.RevisionID,
		EventID: receipt.EventID, CommittedAt: receipt.CommittedAt}
}
func recurrenceIdentityCompatible(oldEpisode, newEpisode model.SymptomEpisode) bool {
	if !strings.EqualFold(oldEpisode.SymptomType, newEpisode.SymptomType) {
		return false
	}
	if oldEpisode.Laterality != "unknown" && newEpisode.Laterality != "unknown" &&
		oldEpisode.Laterality != newEpisode.Laterality {
		return false
	}
	// Body region labels are not medical etiologies, but a user-confirmed
	// recurrence must at minimum have compatible reported anatomical scope.
	var oldRegions, newRegions []string
	if json.Unmarshal(oldEpisode.BodyRegionIDs, &oldRegions) != nil || json.Unmarshal(newEpisode.BodyRegionIDs, &newRegions) != nil {
		return false
	}
	if len(oldRegions) == 0 || len(newRegions) == 0 {
		return true
	}
	for _, o := range oldRegions {
		for _, n := range newRegions {
			if o == n {
				return true
			}
		}
	}
	return false
}
