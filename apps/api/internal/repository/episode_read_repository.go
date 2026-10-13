package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrEpisodeNotFound = errors.New("episode not available for this owner")

// ListEpisodes reads all current Episode identities and status-evidence links
// from one repeatable-read snapshot. It never recomputes medical truth from
// plain symptoms or absence of new records. The public transport is a later
// generated-contract slice; this repository never accepts AI write commands.
func (r *BodyStateRepository) ListEpisodes(ctx context.Context, owner uuid.UUID, beforeRevision int64) (*model.EpisodePage, error) {
	if owner == uuid.Nil || beforeRevision < 0 {
		return nil, fmt.Errorf("%w: owner and non-negative cursor required", model.ErrInvalidEpisodeEvidence)
	}
	var result *model.EpisodePage
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = listEpisodeSnapshot(tx, owner, beforeRevision, nil)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *BodyStateRepository) GetEpisode(ctx context.Context, owner, id uuid.UUID) (*model.EpisodeRecord, error) {
	if owner == uuid.Nil || id == uuid.Nil {
		return nil, fmt.Errorf("%w: owner and episode ID required", model.ErrInvalidEpisodeEvidence)
	}
	var result *model.EpisodeRecord
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		page, err := listEpisodeSnapshot(tx, owner, 0, &id)
		if err != nil {
			return err
		}
		if len(page.Episodes) != 1 {
			return ErrEpisodeNotFound
		}
		result = &page.Episodes[0]
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// episodeSourceReadRow deliberately does not include the statement or minimum
// excerpt. API consumers use the existing owner-scoped Facts view to display
// those. A blocked/erased/superseded source is excluded by the SQL joins.
type episodeSourceReadRow struct {
	EpisodeID         uuid.UUID      `gorm:"column:episode_id"`
	FactID            uuid.UUID      `gorm:"column:fact_id"`
	Role              string         `gorm:"column:role"`
	AssociationMethod string         `gorm:"column:association_method"`
	IsPrimary         bool           `gorm:"column:is_primary"`
	CreatedRevision   int64          `gorm:"column:created_revision"`
	Claim             *string        `gorm:"column:claim"`
	ClaimAsOf         datatypes.JSON `gorm:"column:claim_as_of"`
	ClaimRevision     *int64         `gorm:"column:claim_revision"`
}

func listEpisodeSnapshot(db *gorm.DB, owner uuid.UUID, beforeRevision int64, onlyID *uuid.UUID) (*model.EpisodePage, error) {
	page := &model.EpisodePage{UserID: owner, Episodes: []model.EpisodeRecord{}}
	var state model.BodyState
	err := db.Where("user_id = ?", owner).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return page, nil
	}
	if err != nil {
		return nil, err
	}
	page.CurrentRevision = state.CurrentRevision
	// A blocked/erased source cannot authorize continued display of even the
	// Episode's stored symptom label. Keep historical identities in the audit DB
	// for controlled privacy erasure, but do not expose unsupported identities.
	// A withdrawn/superseded assertion with an AVAILABLE source remains a
	// user-auditable invalidated identity, never an ongoing clinical claim.
	const availableSource = `EXISTS (
		SELECT 1 FROM public.episode_fact_links AS l
		JOIN public.body_facts AS f ON f.user_id = l.user_id AND f.id = l.fact_id
			AND f.kind = 'symptom' AND f.source_evidence_id = l.basis_evidence_id
		JOIN public.statement_evidence AS ev ON ev.user_id = f.user_id
			AND ev.id = f.source_evidence_id AND ev.availability = 'available'
		WHERE l.user_id = ep.user_id AND l.episode_id = ep.id
			AND l.created_revision <= ? AND f.created_revision <= ?
	)`
	base := db.Table("public.symptom_episodes AS ep").
		Where("ep.user_id = ? AND ep.created_revision <= ?", owner, state.CurrentRevision).
		Where(availableSource, state.CurrentRevision, state.CurrentRevision)
	if onlyID == nil {
		if beforeRevision > 0 {
			base = base.Where("ep.created_revision < ?", beforeRevision)
		}
		var revisions []int64
		if err := base.Session(&gorm.Session{}).Distinct("ep.created_revision").Order("ep.created_revision DESC").Limit(21).Pluck("ep.created_revision", &revisions).Error; err != nil {
			return nil, err
		}
		if len(revisions) == 0 {
			return page, nil
		}
		if len(revisions) > 20 {
			page.HasMore = true
			revisions = revisions[:20]
			cursor := revisions[len(revisions)-1]
			page.NextBeforeRevision = &cursor
		}
		base = base.Where("ep.created_revision IN ?", revisions)
	} else {
		base = base.Where("ep.id = ?", *onlyID)
	}
	var identities []model.SymptomEpisode
	if err := base.Order("ep.created_revision DESC, ep.id ASC").Find(&identities).Error; err != nil {
		return nil, err
	}
	if len(identities) == 0 {
		return page, nil
	}

	ids := make([]uuid.UUID, 0, len(identities))
	for _, identity := range identities {
		ids = append(ids, identity.ID)
	}
	var sources []episodeSourceReadRow
	// Every returned Fact must still be an asserted symptom whose minimum
	// original evidence is available, owned by this same user and created
	// no later than the pinned revision. A state claim is optional: a link
	// by itself must never be treated as an ongoing/ended assertion.
	err = db.Table("public.episode_fact_links AS l").
		Select("l.episode_id, l.fact_id, l.role, l.association_method, l.is_primary, l.created_revision, state.claim, state.as_of AS claim_as_of, state.created_revision AS claim_revision").
		Joins("JOIN public.body_facts AS f ON f.user_id = l.user_id AND f.id = l.fact_id AND f.kind = 'symptom' AND f.source_evidence_id = l.basis_evidence_id").
		Joins("JOIN public.statement_evidence AS ev ON ev.user_id = f.user_id AND ev.id = f.source_evidence_id").
		Joins("LEFT JOIN public.episode_state_evidence AS state ON state.user_id = l.user_id AND state.episode_id = l.episode_id AND state.fact_id = l.fact_id AND state.source_evidence_id = l.basis_evidence_id AND state.created_revision <= ?", page.CurrentRevision).
		Where("l.user_id = ? AND l.episode_id IN ? AND l.created_revision <= ? AND f.created_revision <= ? AND f.assertion_status = 'asserted' AND ev.availability = 'available'",
			owner, ids, page.CurrentRevision, page.CurrentRevision).
		Order("l.episode_id ASC, l.created_revision ASC, l.fact_id ASC").
		Scan(&sources).Error
	if err != nil {
		return nil, err
	}

	linked := make(map[uuid.UUID][]model.EpisodeFactReference, len(identities))
	evidence := make(map[uuid.UUID][]model.EpisodeStateBasis, len(identities))
	for _, source := range sources {
		linked[source.EpisodeID] = append(linked[source.EpisodeID], model.EpisodeFactReference{
			FactID: source.FactID, Role: source.Role, AssociationMethod: source.AssociationMethod,
			IsPrimary: source.IsPrimary, LinkedRevision: source.CreatedRevision,
		})
		if source.Claim == nil {
			continue
		}
		if source.ClaimRevision == nil || len(source.ClaimAsOf) == 0 {
			return nil, fmt.Errorf("%w: claim missing temporal provenance", model.ErrInvalidEpisodeEvidence)
		}
		var temporal model.FactTemporal
		if err := json.Unmarshal(source.ClaimAsOf, &temporal); err != nil {
			return nil, fmt.Errorf("%w: unparseable claim time: %v", model.ErrInvalidEpisodeEvidence, err)
		}
		evidence[source.EpisodeID] = append(evidence[source.EpisodeID], model.EpisodeStateBasis{
			FactID: source.FactID, Claim: *source.Claim, AssertionStatus: "asserted",
			SourceAvailability: "available", AsOf: temporal, CreatedRevision: *source.ClaimRevision,
		})
	}
	for _, identity := range identities {
		// A former asserted symptom may be corrected/withdrawn or its source
		// may be blocked/erased. The stored identity stays auditable, but a
		// currently unsupported Episode cannot remain displayed as ongoing.
		status := identity.IdentityStatus
		if status == "current" && len(linked[identity.ID]) == 0 {
			status = "invalidated"
		}
		projection, err := model.ProjectExplicitEpisodeState(status, page.CurrentRevision, evidence[identity.ID])
		if err != nil {
			return nil, err
		}
		refs := linked[identity.ID]
		if refs == nil {
			refs = []model.EpisodeFactReference{}
		}
		page.Episodes = append(page.Episodes, model.EpisodeRecord{
			ID: identity.ID, UserID: identity.UserID, SymptomType: identity.SymptomType,
			BodyRegionIDs: identity.BodyRegionIDs, Laterality: identity.Laterality,
			IdentityStatus: status, CreatedRevision: identity.CreatedRevision,
			CreatedAt: identity.CreatedAt, Links: refs, Projection: projection,
		})
	}
	return page, nil
}
