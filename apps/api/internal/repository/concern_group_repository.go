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

// maxConcernGroupEpisodes bounds the episode side of the derived projection.
const maxConcernGroupEpisodes = 500

type concernGroupNodeRow struct {
	ID              uuid.UUID `gorm:"column:id"`
	CreatedRevision int64     `gorm:"column:created_revision"`
	Label           string    `gorm:"column:label"`
	AnchorID        uuid.UUID `gorm:"column:anchor_id"`
	AnchorRevision  int64     `gorm:"column:anchor_revision"`
}

type concernGroupLinkRow struct {
	FromID       uuid.UUID `gorm:"column:from_id"`
	FactID       uuid.UUID `gorm:"column:fact_id"`
	FactRevision int64     `gorm:"column:fact_revision"`
}

// ReadConcernGroupSnapshot reads, from ONE repeatable-read snapshot pinned to
// the owner's current BodyState revision, the inputs of the derived
// ConcernGroup projection: asserted source-available hypotheses (with their
// lineage start for the stable group id), current source-available episodes,
// their links to asserted source-available facts, and hypothesis links to
// the owner's asserted source-available observations. Nothing is written; the projection has no table and no authority.
func (r *BodyStateRepository) ReadConcernGroupSnapshot(ctx context.Context, owner uuid.UUID) (*model.ConcernGroupSnapshot, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyHypothesis)
	}
	var snap *model.ConcernGroupSnapshot
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		snap = &model.ConcernGroupSnapshot{UserID: owner}
		var state model.BodyState
		if err := tx.Where("user_id=?", owner).Take(&state).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		rev := state.CurrentRevision
		snap.Revision = rev

		var hyps []concernGroupNodeRow
		if err := tx.Table("public.body_hypotheses AS h").
			Joins("JOIN public.statement_evidence AS e ON e.user_id=h.user_id AND e.id=h.source_evidence_id").
			Where("h.user_id=? AND h.created_revision<=? AND h.assertion_status='asserted' AND e.availability='available'", owner, rev).
			Joins("JOIN public.body_hypotheses AS root ON root.user_id=h.user_id AND root.id=h.lineage_id").
			Select("h.id AS id, h.created_revision AS created_revision, h.statement AS label, " +
				"h.lineage_id AS anchor_id, root.created_revision AS anchor_revision").
			Order("h.created_revision DESC, h.id ASC").Limit(maxActiveBodyHypotheses).Scan(&hyps).Error; err != nil {
			return err
		}
		const episodeAvailable = `EXISTS (
			SELECT 1 FROM public.episode_fact_links AS l
			JOIN public.body_facts AS f ON f.user_id = l.user_id AND f.id = l.fact_id
				AND f.kind = 'symptom' AND f.source_evidence_id = l.basis_evidence_id
			JOIN public.statement_evidence AS ev ON ev.user_id = f.user_id
				AND ev.id = f.source_evidence_id AND ev.availability = 'available'
			WHERE l.user_id = ep.user_id AND l.episode_id = ep.id
				AND l.created_revision <= ? AND f.created_revision <= ?
		)`
		var eps []concernGroupNodeRow
		if err := tx.Table("public.symptom_episodes AS ep").
			Where("ep.user_id=? AND ep.created_revision<=? AND ep.identity_status='current'", owner, rev).
			Where(episodeAvailable, rev, rev).
			Select("ep.id AS id, ep.created_revision AS created_revision, ep.symptom_type AS label, " +
				"ep.id AS anchor_id, ep.created_revision AS anchor_revision").
			Order("ep.created_revision DESC, ep.id ASC").Limit(maxConcernGroupEpisodes).Scan(&eps).Error; err != nil {
			return err
		}
		facts := map[uuid.UUID]int64{}
		observations := map[uuid.UUID]int64{}
		if len(hyps) > 0 {
			ids := make([]uuid.UUID, len(hyps))
			for i, h := range hyps {
				ids[i] = h.ID
			}
			var links []concernGroupLinkRow
			if err := tx.Table("public.body_hypothesis_fact_links AS l").
				Joins("JOIN public.body_facts AS f ON f.user_id=l.user_id AND f.id=l.fact_id").
				Joins("JOIN public.statement_evidence AS ev ON ev.user_id=f.user_id AND ev.id=f.source_evidence_id").
				Where("l.user_id=? AND l.hypothesis_id IN ? AND f.assertion_status='asserted' AND f.created_revision<=? AND ev.availability='available'", owner, ids, rev).
				Select("l.hypothesis_id AS from_id, l.fact_id AS fact_id, f.created_revision AS fact_revision").
				Scan(&links).Error; err != nil {
				return err
			}
			for _, l := range links {
				snap.HypothesisFactLinks = append(snap.HypothesisFactLinks, model.ConcernGroupLink{FromID: l.FromID, FactID: l.FactID})
				facts[l.FactID] = l.FactRevision
			}
			var observationLinks []concernGroupLinkRow
			if err := tx.Table("public.body_hypothesis_observation_links AS l").
				Joins("JOIN public.body_observations AS o ON o.user_id=l.user_id AND o.id=l.observation_id").
				Joins("JOIN public.statement_evidence AS ev ON ev.user_id=o.user_id AND ev.id=o.source_evidence_id").
				Where("l.user_id=? AND l.hypothesis_id IN ? AND o.assertion_status='asserted' AND o.created_revision<=? AND ev.availability='available'", owner, ids, rev).
				Select("l.hypothesis_id AS from_id, l.observation_id AS fact_id, o.created_revision AS fact_revision").
				Scan(&observationLinks).Error; err != nil {
				return err
			}
			for _, l := range observationLinks {
				snap.HypothesisObservationLinks = append(snap.HypothesisObservationLinks,
					model.ConcernGroupObservationLink{HypothesisID: l.FromID, ObservationID: l.FactID})
				observations[l.FactID] = l.FactRevision
			}
		}
		if len(eps) > 0 {
			ids := make([]uuid.UUID, len(eps))
			for i, e := range eps {
				ids[i] = e.ID
			}
			var links []concernGroupLinkRow
			if err := tx.Table("public.episode_fact_links AS l").
				Joins("JOIN public.body_facts AS f ON f.user_id=l.user_id AND f.id=l.fact_id AND f.source_evidence_id=l.basis_evidence_id").
				Joins("JOIN public.statement_evidence AS ev ON ev.user_id=f.user_id AND ev.id=f.source_evidence_id").
				Where("l.user_id=? AND l.episode_id IN ? AND l.created_revision<=? AND f.assertion_status='asserted' AND f.created_revision<=? AND ev.availability='available'", owner, ids, rev, rev).
				Select("l.episode_id AS from_id, l.fact_id AS fact_id, f.created_revision AS fact_revision").
				Scan(&links).Error; err != nil {
				return err
			}
			for _, l := range links {
				snap.EpisodeFactLinks = append(snap.EpisodeFactLinks, model.ConcernGroupLink{FromID: l.FromID, FactID: l.FactID})
				facts[l.FactID] = l.FactRevision
			}
		}
		for _, h := range hyps {
			snap.Hypotheses = append(snap.Hypotheses, model.ConcernGroupNode(h))
		}
		for _, e := range eps {
			snap.Episodes = append(snap.Episodes, model.ConcernGroupNode(e))
		}
		for id, r := range facts {
			snap.Facts = append(snap.Facts, model.ConcernGroupNode{ID: id, CreatedRevision: r})
		}
		for id, r := range observations {
			snap.Observations = append(snap.Observations, model.ConcernGroupNode{ID: id, CreatedRevision: r})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return snap, nil
}
