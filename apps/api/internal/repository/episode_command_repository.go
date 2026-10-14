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

// CreateEpisode is an explicit USER request to organize a positive symptom
// statement as a new Episode identity. No status claim, onset date, clinical
// diagnosis or implied recovery is created.
func (r *BodyStateRepository) CreateEpisode(ctx context.Context, owner uuid.UUID, c model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error) {
	return r.commitEpisodeAssociation(ctx, owner, uuid.Nil, c, "create")
}

// LinkEpisodeFact is an exact user-selected primary observation link to an
// already-visible Episode. This does not silently merge/split identities or
// mark symptom onset/resolution; those require distinct authorized commands.
func (r *BodyStateRepository) LinkEpisodeFact(ctx context.Context, owner, episodeID uuid.UUID, c model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error) {
	return r.commitEpisodeAssociation(ctx, owner, episodeID, c, "link")
}

func (r *BodyStateRepository) commitEpisodeAssociation(ctx context.Context, owner, episodeID uuid.UUID, c model.EpisodeAssociationCommand, operation string) (*model.EpisodeCommandReceipt, error) {
	if owner == uuid.Nil || (operation == "link" && episodeID == uuid.Nil) || (operation != "create" && operation != "link") {
		return nil, fmt.Errorf("%w: explicit authenticated owner and exact operation required", model.ErrInvalidEpisodeEvidence)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Operation string                          `json:"operation"`
		Target    uuid.UUID                       `json:"episode_id"`
		Command   model.EpisodeAssociationCommand `json:"command"`
	}{operation, episodeID, c})
	if err != nil {
		return nil, err
	}
	var receipt model.EpisodeCommandReceipt
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		// The previously committed receipt is checked BEFORE revision checks.
		// Even a source withdrawn/pruned after the successful commit cannot
		// turn the same original retry into a duplicate semantic revision.
		var prior model.EpisodeCommandReceipt
		err = tx.Where("user_id = ? AND idempotency_key = ?", owner, c.IdempotencyKey).First(&prior).Error
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
		if state.CurrentRevision != c.ExpectedRevision {
			return fmt.Errorf("%w: expected %d current %d", ErrBodyStateRevisionConflict, c.ExpectedRevision, state.CurrentRevision)
		}

		// Lock BOTH the accepted Fact and its available original evidence.
		// Privacy source blocking and explicit withdrawal cannot race ahead
		// of this command inside the same transaction. No AI-supplied raw
		// statement, source ID or symptoms are trusted.
		var fact model.BodyFact
		err = tx.Table("public.body_facts AS f").Select("f.*").
			Joins("JOIN public.statement_evidence AS ev ON ev.user_id = f.user_id AND ev.id = f.source_evidence_id").
			Where("f.user_id = ? AND f.id = ? AND f.kind = 'symptom' AND f.subject = 'self' AND f.assertion_status = 'asserted' AND ev.availability = 'available'", owner, c.FactID).
			Clauses(clause.Locking{Strength: "SHARE"}).Take(&fact).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrEpisodeFactUnavailable
		}
		if err != nil {
			return err
		}
		var details model.SymptomFactDetails
		if err := json.Unmarshal(fact.Details, &details); err != nil {
			return fmt.Errorf("%w: malformed accepted symptom details", model.ErrInvalidEpisodeEvidence)
		}
		if details.Presence != "present" && details.Presence != "absent" {
			return fmt.Errorf("%w: symptom presence must be explicit", model.ErrInvalidEpisodeEvidence)
		}
		normalizedType := strings.TrimSpace(details.SymptomType)
		if normalizedType == "" || len([]rune(normalizedType)) > 120 {
			return fmt.Errorf("%w: a nonempty user-supported symptom type is required", model.ErrInvalidEpisodeEvidence)
		}
		side := details.Laterality
		if side == "" {
			side = "unknown"
		}
		if !validEpisodeLaterality(side) {
			return fmt.Errorf("%w: symptom side invalid", model.ErrInvalidEpisodeEvidence)
		}
		var regions []string
		if err := json.Unmarshal(fact.BodyRegionIDs, &regions); err != nil || regions == nil {
			return fmt.Errorf("%w: symptom Fact is missing canonical regions", model.ErrInvalidEpisodeEvidence)
		}

		now := time.Now().UTC()
		if operation == "create" {
			if details.Presence != "present" {
				// A generic negative/absent symptom is NOT a new Episode or
				// an end-of-course assertion.
				return model.ErrEpisodeAssociationConflict
			}
			episodeID = uuid.New()
		} else {
			var target model.SymptomEpisode
			err = tx.Where("user_id = ? AND id = ? AND identity_status = ?", owner, episodeID, "current").Take(&target).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEpisodeNotFound
			}
			if err != nil {
				return err
			}
			if !strings.EqualFold(target.SymptomType, normalizedType) ||
				(target.Laterality != "unknown" && side != "unknown" && target.Laterality != side) {
				return model.ErrEpisodeAssociationConflict
			}
			var targetRegions []string
			if err := json.Unmarshal(target.BodyRegionIDs, &targetRegions); err != nil {
				return err
			}
			if len(regions) > 0 && len(targetRegions) > 0 {
				overlap := false
				for _, region := range regions {
					for _, other := range targetRegions {
						if region == other {
							overlap = true
							break
						}
					}
					if overlap {
						break
					}
				}
				if !overlap {
					return model.ErrEpisodeAssociationConflict
				}
			}
			// An unsupported Episode is not an alternate container where
			// deleted/blocked sources can quietly be re-associated.
			var sourceCount int64
			err = tx.Table("public.episode_fact_links AS l").
				Joins("JOIN public.body_facts AS f ON f.user_id=l.user_id AND f.id=l.fact_id AND f.source_evidence_id=l.basis_evidence_id").
				Joins("JOIN public.statement_evidence AS ev ON ev.user_id=f.user_id AND ev.id=f.source_evidence_id").
				Where("l.user_id=? AND l.episode_id=? AND ev.availability='available' AND f.assertion_status='asserted'", owner, episodeID).
				Count(&sourceCount).Error
			if err != nil {
				return err
			}
			if sourceCount == 0 {
				return ErrEpisodeNotFound
			}
			// A later explicit current report after a source-backed ended
			// course is a new Episode identity, never a silent reopening.
			// Reject BEFORE creating an irreversible unique-primary link.
			if details.EpisodeCourseReport == "explicit_current_report" {
				page, err := listEpisodeSnapshot(tx, owner, 0, &episodeID)
				if err != nil {
					return err
				}
				if len(page.Episodes) != 1 || page.Episodes[0].Projection.State == "ended" {
					return model.ErrEpisodeAssociationConflict
				}
			}
		}
		var conflicting model.EpisodeFactLink
		err = tx.Where("user_id = ? AND fact_id = ? AND is_primary = TRUE", owner, fact.ID).Take(&conflicting).Error
		if err == nil {
			return model.ErrEpisodeAssociationConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		commandID, eventID := uuid.New(), uuid.New()
		metadata := map[string]any{"commit_id": commandID, "episode_id": episodeID,
			"fact_id": fact.ID, "source_evidence_id": fact.SourceEvidenceID,
			"role": "observation", "authorization": "direct_user_command", "operation": operation}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "episode."+operation, "direct_edit", metadata)
		if err != nil {
			return err
		}
		if operation == "create" {
			record := model.SymptomEpisode{ID: episodeID, UserID: owner, SymptomType: normalizedType,
				BodyRegionIDs: fact.BodyRegionIDs, Laterality: side, IdentityStatus: "current",
				CreatedRevision: next, CreatedAt: now}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
		link := model.EpisodeFactLink{UserID: owner, EpisodeID: episodeID, FactID: fact.ID,
			FactKind: "symptom", BasisEvidenceID: fact.SourceEvidenceID, Role: "observation",
			AssociationMethod: "explicit", IsPrimary: true, CreatedRevision: next}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		event := model.BodyStateOutbox{ID: eventID, UserID: owner, Revision: next,
			EventType: "body_state.episode_" + operation, Payload: datatypes.JSON(payload), CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		receipt = model.EpisodeCommandReceipt{ID: commandID, UserID: owner, IdempotencyKey: c.IdempotencyKey,
			Fingerprint: fingerprint, Operation: operation, EpisodeID: episodeID, FactID: fact.ID,
			Revision: next, RevisionID: revision.ID, EventID: eventID, CommittedAt: now}
		return tx.Create(&receipt).Error
	})
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func validEpisodeLaterality(side string) bool {
	switch side {
	case "left", "right", "bilateral", "not_applicable", "unknown":
		return true
	default:
		return false
	}
}
