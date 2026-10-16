package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// BodyStateReadModel preserves native V2 records. Legacy candidates and AI
// hypotheses are never part of this view.
type BodyStateReadModel struct {
	UserID             uuid.UUID                       `json:"user_id"`
	CurrentRevision    int64                           `json:"current_revision"`
	Facts              []model.BodyFact                `json:"facts"`
	Observations       []model.BodyObservationRecord   `json:"observations"`
	Episodes           []model.EpisodeRecord           `json:"episodes"`
	Hypotheses         []model.BodyHypothesisRecord    `json:"hypotheses"`
	RecentRevisions    []model.BodyStateRevision       `json:"recent_revisions"`
	SafetyRestrictions []model.SafetyRestrictionRecord `json:"safety_restrictions"`
	SafetyScreens      []model.SafetyScreen            `json:"-"`
}

// GetBodyState freezes all constituents in one read-only database snapshot.
// Incomplete reads fail instead of silently omitting symptoms from Safety.
func (s *BodyStateService) GetBodyState(ctx context.Context, owner uuid.UUID, historyLimit int) (*BodyStateReadModel, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("authenticated owner required")
	}
	var view *BodyStateReadModel
	load := func(ctx context.Context) error {
		head, err := s.repo.GetBodyStateHead(ctx, owner)
		if err != nil {
			return err
		}
		if head == nil || head.UserID != owner {
			return fmt.Errorf("body state head owner mismatch")
		}
		view = &BodyStateReadModel{
			UserID: owner, CurrentRevision: head.CurrentRevision,
			Facts: []model.BodyFact{}, Observations: []model.BodyObservationRecord{},
			Episodes: []model.EpisodeRecord{}, Hypotheses: []model.BodyHypothesisRecord{},
			RecentRevisions: []model.BodyStateRevision{},
		}

		pinned := func(userID uuid.UUID, revision int64) error {
			if userID != owner {
				return fmt.Errorf("body state read owner mismatch")
			}
			if revision != head.CurrentRevision {
				return fmt.Errorf("%w: head R%d, read R%d", errAgentBodyContextNotPinned, head.CurrentRevision, revision)
			}
			return nil
		}
		err = readBodyStatePages(func(before int64) (bool, *int64, error) {
			page, err := s.repo.ListFacts(ctx, owner, before)
			if err != nil {
				return false, nil, err
			}
			if page == nil {
				return false, nil, fmt.Errorf("body facts unavailable")
			}
			if err := pinned(page.UserID, page.CurrentRevision); err != nil {
				return false, nil, err
			}
			for _, item := range page.Facts {
				if item.UserID != owner {
					return false, nil, fmt.Errorf("body fact owner mismatch")
				}
				view.Facts = append(view.Facts, item)
			}
			return page.HasMore, page.NextBeforeRevision, nil
		})
		if err != nil {
			return err
		}
		err = readBodyStatePages(func(before int64) (bool, *int64, error) {
			page, err := s.repo.ListDirectObservations(ctx, owner, before)
			if err != nil {
				return false, nil, err
			}
			if page == nil {
				return false, nil, fmt.Errorf("body observations unavailable")
			}
			if err := pinned(page.UserID, page.CurrentRevision); err != nil {
				return false, nil, err
			}
			for _, item := range page.Observations {
				if item.UserID != owner {
					return false, nil, fmt.Errorf("body observation owner mismatch")
				}
				view.Observations = append(view.Observations, item)
			}
			return page.HasMore, page.NextBeforeRevision, nil
		})
		if err != nil {
			return err
		}
		err = readBodyStatePages(func(before int64) (bool, *int64, error) {
			page, err := s.repo.ListEpisodes(ctx, owner, before)
			if err != nil {
				return false, nil, err
			}
			if page == nil {
				return false, nil, fmt.Errorf("body episodes unavailable")
			}
			if err := pinned(page.UserID, page.CurrentRevision); err != nil {
				return false, nil, err
			}
			for _, item := range page.Episodes {
				if item.UserID != owner {
					return false, nil, fmt.Errorf("body episode owner mismatch")
				}
				view.Episodes = append(view.Episodes, item)
			}
			return page.HasMore, page.NextBeforeRevision, nil
		})
		if err != nil {
			return err
		}
		hypotheses, err := s.repo.ListActiveBodyHypotheses(ctx, owner)
		if err != nil {
			return err
		}
		if hypotheses == nil {
			return fmt.Errorf("body hypotheses unavailable")
		}
		if err := pinned(hypotheses.UserID, hypotheses.CurrentRevision); err != nil {
			return err
		}
		for _, item := range hypotheses.Hypotheses {
			if item.UserID != owner {
				return fmt.Errorf("body hypothesis owner mismatch")
			}
			if item.Source.Actor != "user" {
				return fmt.Errorf("non-user BodyHypothesis")
			}
			view.Hypotheses = append(view.Hypotheses, item)
		}
		view.SafetyScreens, err = s.repo.ListSafetyScreens(ctx, owner)
		if err != nil {
			return err
		}
		for _, item := range view.SafetyScreens {
			if item.UserID != owner {
				return fmt.Errorf("safety screen owner mismatch")
			}
			if item.BodyStateRevision > head.CurrentRevision {
				return errAgentBodyContextNotPinned
			}
		}
		restrictions, err := s.repo.ListSafetyRestrictions(ctx, owner)
		if err != nil {
			return err
		}
		if restrictions == nil {
			return fmt.Errorf("native safety unavailable")
		}
		if err := pinned(restrictions.UserID, restrictions.CurrentRevision); err != nil {
			return err
		}
		view.SafetyRestrictions = restrictions.Restrictions
		revisions, err := s.repo.ListRecentRevisions(ctx, owner, historyLimit)
		if err != nil {
			return err
		}
		for _, item := range revisions {
			if item.UserID != owner {
				return fmt.Errorf("body revision owner mismatch")
			}
			if item.Revision > head.CurrentRevision {
				return errAgentBodyContextNotPinned
			}
			view.RecentRevisions = append(view.RecentRevisions, item)
		}
		return nil
	}
	var err error
	if s.readSnapshots != nil {
		err = s.readSnapshots.WithinReadSnapshot(ctx, load)
	} else {
		err = load(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("load native body state: %w", err)
	}
	return view, nil
}

func readBodyStatePages(read func(int64) (bool, *int64, error)) error {
	before := int64(0)
	for page := 0; page < bodyContextMaxPages; page++ {
		more, next, err := read(before)
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
		if next == nil || *next <= 0 || (before > 0 && *next >= before) {
			return fmt.Errorf("invalid native body state cursor")
		}
		before = *next
	}
	return fmt.Errorf("native body state exceeds bounded read")
}
