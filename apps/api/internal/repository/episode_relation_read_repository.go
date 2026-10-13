package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListEpisodeRelations returns only relations with a STILL VALID original
// source and both source-visible same-owner identities, inside one
// repeatable-read snapshot. If the old resolution was corrected or withdrawn,
// the explicitly reported relation remains auditable as needs_review,
// NEVER a proven medical recurrence.
func (r *BodyStateRepository) ListEpisodeRelations(ctx context.Context, owner, episodeID uuid.UUID, beforeRevision int64) (*model.EpisodeRelationPage, error) {
	if owner == uuid.Nil || episodeID == uuid.Nil || beforeRevision < 0 {
		return nil, fmt.Errorf("%w: valid owner, Episode and cursor required", model.ErrInvalidEpisodeEvidence)
	}
	var result *model.EpisodeRelationPage
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		visible, err := listEpisodeSnapshot(tx, owner, 0, &episodeID)
		if err != nil {
			return err
		}
		if len(visible.Episodes) != 1 {
			return ErrEpisodeNotFound
		}
		result = &model.EpisodeRelationPage{
			UserID: owner, EpisodeID: episodeID, CurrentRevision: visible.CurrentRevision,
			Relations: []model.EpisodeRelationReference{},
		}
		query := tx.Table("public.episode_relations AS r").Select("r.*").
			Joins("JOIN public.body_facts AS f ON f.user_id=r.user_id AND f.id=r.basis_fact_id AND f.source_evidence_id=r.basis_evidence_id").
			Joins("JOIN public.statement_evidence AS ev ON ev.user_id=r.user_id AND ev.id=r.basis_evidence_id").
			Where("r.user_id=? AND (r.from_episode_id=? OR r.to_episode_id=?) AND r.created_revision<=? AND f.assertion_status='asserted' AND ev.availability='available'",
				owner, episodeID, episodeID, visible.CurrentRevision)
		if beforeRevision > 0 {
			query = query.Where("r.created_revision<?", beforeRevision)
		}
		// Filter identities without ANY still-available original source before
		// LIMIT/pagination, not merely after materializing a page. Otherwise a
		// source-blocked relation can leak its existence through has_more or
		// suppress later visible relations because it consumed a page slot.
		const visibleIdentity = `EXISTS (
			SELECT 1 FROM public.episode_fact_links AS l
			JOIN public.body_facts AS f ON f.user_id=l.user_id AND f.id=l.fact_id AND f.kind='symptom' AND f.source_evidence_id=l.basis_evidence_id
			JOIN public.statement_evidence AS ev ON ev.user_id=f.user_id AND ev.id=f.source_evidence_id AND ev.availability='available'
			WHERE l.user_id=r.user_id AND l.episode_id=r.%s
			AND l.created_revision<=? AND f.created_revision<=?
		)`
		for _, endpoint := range []string{"from_episode_id", "to_episode_id"} {
			// endpoint is one of two hard-coded column identifiers, never user input.
			query = query.Where(fmt.Sprintf(visibleIdentity, endpoint), visible.CurrentRevision, visible.CurrentRevision)
		}
		var rows []model.EpisodeRelation
		if err := query.Order("r.created_revision DESC, r.id ASC").Limit(21).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 20 {
			result.HasMore = true
			rows = rows[:20]
			cursor := rows[len(rows)-1].CreatedRevision
			result.NextBeforeRevision = &cursor
		}
		// One shared snapshot and bounded rows; cache results for relations
		// to the same destination to avoid repeated database projections.
		cache := map[uuid.UUID]*model.EpisodeRecord{episodeID: &visible.Episodes[0]}
		load := func(id uuid.UUID) (*model.EpisodeRecord, error) {
			if hit, ok := cache[id]; ok {
				return hit, nil
			}
			page, err := listEpisodeSnapshot(tx, owner, 0, &id)
			if err != nil {
				return nil, err
			}
			if len(page.Episodes) != 1 {
				cache[id] = nil
				return nil, nil
			}
			copy := page.Episodes[0]
			cache[id] = &copy
			return &copy, nil
		}
		for _, rel := range rows {
			old, err := load(rel.FromEpisodeID)
			if err != nil {
				return err
			}
			current, err := load(rel.ToEpisodeID)
			if err != nil {
				return err
			}
			if old == nil || current == nil {
				continue
			}
			review := "needs_review"
			if old.IdentityStatus == "current" && old.Projection.State == "ended" {
				review = "supported"
			}
			result.Relations = append(result.Relations, model.EpisodeRelationReference{
				ID: rel.ID, FromEpisodeID: rel.FromEpisodeID, ToEpisodeID: rel.ToEpisodeID,
				Kind: rel.Kind, BasisFactID: rel.BasisFactID,
				AuthorizationID: rel.AuthorizationID, CreatedRevision: rel.CreatedRevision,
				ReviewState: review,
			})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrEpisodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
