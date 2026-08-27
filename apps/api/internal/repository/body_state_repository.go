package repository

import (
	"bytes"
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

// ErrBodyStateRevisionConflict is returned when a caller tries to mutate a
// BodyState from an older revision. This is the optimistic-concurrency guard
// that prevents two devices/runs from silently overwriting each other.
var ErrBodyStateRevisionConflict = errors.New("body state revision conflict")

// BodyStateRepository is the transactional persistence boundary for the
// longitudinal BodyState aggregate. The current projection is stored in normal
// tables; body_state_revisions records semantic history without requiring event
// replay for ordinary reads.
type BodyStateRepository struct {
	db *gorm.DB
}

func NewBodyStateRepository(db *gorm.DB) *BodyStateRepository {
	return &BodyStateRepository{db: db}
}

// GetCurrent returns the current reasoning projection only. Corrected/resolved/
// excluded items remain durable history but are intentionally filtered out here.
func (r *BodyStateRepository) GetCurrent(ctx context.Context, userID uuid.UUID) (*model.BodyState, error) {
	var state model.BodyState
	err := database.FromContext(ctx, r.db).Where("user_id = ?", userID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.BodyState{UserID: userID, SafetyState: datatypes.JSON(`{}`)}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND lifecycle_state = ? AND excluded_from_reasoning = FALSE", userID, "active").
		Order("updated_at ASC").
		Find(&state.Facts).Error; err != nil {
		return nil, err
	}
	if err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND lifecycle_state = ? AND review_state = ? AND excluded_from_reasoning = FALSE", userID, "active", "confirmed").
		Order("updated_at ASC").
		Find(&state.Observations).Error; err != nil {
		return nil, err
	}
	if err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND lifecycle_state IN ?", userID, []string{"active", "strengthened", "weakened"}).
		Order("updated_at ASC").
		Find(&state.Hypotheses).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *BodyStateRepository) ListReviewableObservations(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]model.BodyStateObservation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var observations []model.BodyStateObservation
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND lifecycle_state = ? AND review_state = ?", userID, "active", "unverified").
		Order("created_at ASC").
		Limit(limit).
		Find(&observations).Error
	return observations, err
}

func (r *BodyStateRepository) ListReviewableFacts(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]model.BodyStateFact, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var facts []model.BodyStateFact
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND lifecycle_state = ? AND review_state = ?", userID, "active", "unverified").
		Order("created_at ASC").
		Limit(limit).
		Find(&facts).Error
	return facts, err
}

func (r *BodyStateRepository) ListRecentRevisions(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateRevision, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var revisions []model.BodyStateRevision
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("revision DESC").
		Limit(limit).
		Find(&revisions).Error
	return revisions, err
}

// UpsertFact uses source_key as an idempotency key. Replaying the same runtime
// event therefore does not create a second fact or a meaningless revision.
func (r *BodyStateRepository) UpsertFact(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	fact model.BodyStateFact,
	source string,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	var stored model.BodyStateFact
	var committed *model.BodyStateRevision

	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}

		if fact.SourceKey != "" {
			var existing model.BodyStateFact
			err := tx.WithContext(ctx).
				Where("user_id = ? AND source_key = ?", userID, fact.SourceKey).
				First(&existing).Error
			if err == nil {
				bodyStateApplyFactDefaults(&fact)
				if bodyStateSameFact(existing, fact) {
					stored = existing
					return nil
				}
				before := existing
				if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
					Where("id = ? AND user_id = ?", existing.ID, userID).
					Updates(map[string]any{
						"concern_key": fact.ConcernKey, "kind": fact.Kind, "body_region": fact.BodyRegion,
						"value": fact.Value, "details": fact.Details, "origin": fact.Origin,
						"review_state": fact.ReviewState, "lifecycle_state": fact.LifecycleState,
						"trend": fact.Trend, "provenance": fact.Provenance,
						"observed_at": fact.ObservedAt, "valid_from": fact.ValidFrom, "valid_until": fact.ValidUntil,
						"excluded_from_reasoning": fact.ExcludedFromReasoning,
						"updated_revision":        next, "updated_at": time.Now().UTC(),
					}).Error; err != nil {
					return err
				}
				if err := tx.WithContext(ctx).Where("id = ?", existing.ID).First(&stored).Error; err != nil {
					return err
				}
				committed, err = bodyStatePersistRevision(ctx, tx, state, next, "fact.updated", source, map[string]any{
					"fact_id": existing.ID, "before": before, "after": stored,
				})
				return err
			}
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		bodyStateApplyFactDefaults(&fact)
		fact.ID = uuid.New()
		fact.UserID = userID
		fact.CreatedRevision = next
		fact.UpdatedRevision = next
		if err := tx.WithContext(ctx).Create(&fact).Error; err != nil {
			return err
		}
		stored = fact
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "fact.added", source, map[string]any{"fact": fact})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

// CorrectFact means the previous record itself was wrong. It never rewrites the
// historical row in-place into a different claim: the old row becomes inactive/
// corrected and a replacement points back through supersedes_fact_id.
func (r *BodyStateRepository) CorrectFact(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	targetFactID uuid.UUID,
	replacement model.BodyStateFact,
	source string,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	var stored model.BodyStateFact
	var committed *model.BodyStateRevision

	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		var previous model.BodyStateFact
		if err := tx.WithContext(ctx).Where("id = ? AND user_id = ?", targetFactID, userID).First(&previous).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
			Where("id = ? AND user_id = ?", targetFactID, userID).
			Updates(map[string]any{
				"review_state": "corrected", "lifecycle_state": "inactive",
				"updated_revision": next, "updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return err
		}

		bodyStateApplyFactDefaults(&replacement)
		replacement.ID = uuid.New()
		replacement.UserID = userID
		replacement.Origin = bodyStateDefaultString(replacement.Origin, "user_edited")
		replacement.ReviewState = bodyStateDefaultString(replacement.ReviewState, "confirmed")
		replacement.SupersedesFactID = &targetFactID
		replacement.CreatedRevision = next
		replacement.UpdatedRevision = next
		if err := tx.WithContext(ctx).Create(&replacement).Error; err != nil {
			return err
		}
		stored = replacement
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "fact.corrected", source, map[string]any{
			"corrected_fact_id": targetFactID, "previous": previous, "replacement": replacement,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

// TransitionFact records a real change over time: the previous claim remains
// durable history, is closed at effectiveAt, and a new active fact supersedes it.
// Use CorrectFact instead when the previous claim itself was wrong.
func (r *BodyStateRepository) TransitionFact(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	targetFactID uuid.UUID,
	replacement model.BodyStateFact,
	effectiveAt time.Time,
	source string,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	var stored model.BodyStateFact
	var committed *model.BodyStateRevision
	if effectiveAt.IsZero() {
		effectiveAt = time.Now().UTC()
	} else {
		effectiveAt = effectiveAt.UTC()
	}

	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		var previous model.BodyStateFact
		if err := tx.WithContext(ctx).Where("id = ? AND user_id = ?", targetFactID, userID).First(&previous).Error; err != nil {
			return err
		}
		if previous.LifecycleState != "active" {
			return fmt.Errorf("cannot transition non-active fact %s", targetFactID)
		}
		if previous.ValidFrom != nil && effectiveAt.Before(previous.ValidFrom.UTC()) {
			return fmt.Errorf("fact transition time precedes current fact validity")
		}
		if replacement.Kind == "" {
			replacement.Kind = previous.Kind
		}
		if replacement.Kind != previous.Kind {
			return fmt.Errorf("fact transition must preserve kind %q", previous.Kind)
		}

		before := previous
		if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
			Where("id = ? AND user_id = ?", targetFactID, userID).
			Updates(map[string]any{
				"lifecycle_state":  "inactive",
				"valid_until":      effectiveAt,
				"updated_revision": next,
				"updated_at":       time.Now().UTC(),
			}).Error; err != nil {
			return err
		}

		bodyStateApplyFactDefaults(&replacement)
		replacement.ID = uuid.New()
		replacement.UserID = userID
		replacement.SupersedesFactID = &targetFactID
		replacement.ValidFrom = &effectiveAt
		replacement.ValidUntil = nil
		replacement.LifecycleState = "active"
		replacement.CreatedRevision = next
		replacement.UpdatedRevision = next
		if err := tx.WithContext(ctx).Create(&replacement).Error; err != nil {
			return err
		}
		stored = replacement
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "fact.transitioned", source, map[string]any{
			"previous":     before,
			"replacement":  replacement,
			"effective_at": effectiveAt,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

// AcceptCurrentFactCandidate promotes an unverified AI-extracted candidate to
// the confirmed singleton current fact for its kind. If a different confirmed
// value is current, that value is closed at effectiveAt and the candidate
// supersedes it in the same BodyState revision.
func (r *BodyStateRepository) AcceptCurrentFactCandidate(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	candidateID uuid.UUID,
	effectiveAt time.Time,
	source string,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	if effectiveAt.IsZero() {
		effectiveAt = time.Now().UTC()
	} else {
		effectiveAt = effectiveAt.UTC()
	}
	var stored model.BodyStateFact
	var committed *model.BodyStateRevision
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		var candidate model.BodyStateFact
		if err := tx.WithContext(ctx).
			Where("id = ? AND user_id = ?", candidateID, userID).
			First(&candidate).Error; err != nil {
			return err
		}
		if candidate.ReviewState != "unverified" || candidate.LifecycleState != "active" {
			return fmt.Errorf("fact %s is not an active unverified candidate", candidateID)
		}
		current, err := bodyStateFindSingletonFact(ctx, tx, userID, candidate.Kind)
		if err != nil {
			return err
		}
		if current != nil && current.ValidFrom != nil && effectiveAt.Before(current.ValidFrom.UTC()) {
			return fmt.Errorf("candidate acceptance time precedes current fact validity for %q", candidate.Kind)
		}

		if current != nil && bodyStateSameCurrentFactClaim(*current, candidate) {
			if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
				Where("id = ? AND user_id = ?", candidate.ID, userID).
				Updates(map[string]any{
					"review_state":            "confirmed",
					"lifecycle_state":         "inactive",
					"excluded_from_reasoning": true,
					"valid_until":             effectiveAt,
					"updated_revision":        next,
					"updated_at":              time.Now().UTC(),
				}).Error; err != nil {
				return err
			}
			if err := tx.WithContext(ctx).Where("id = ?", candidate.ID).First(&stored).Error; err != nil {
				return err
			}
			committed, err = bodyStatePersistRevision(ctx, tx, state, next, "fact.candidate_accepted_duplicate", source, map[string]any{
				"candidate": stored, "current_fact_id": current.ID,
			})
			return err
		}

		var previous *model.BodyStateFact
		if current != nil {
			before := *current
			previous = &before
			if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
				Where("id = ? AND user_id = ?", current.ID, userID).
				Updates(map[string]any{
					"lifecycle_state":  "inactive",
					"valid_until":      effectiveAt,
					"updated_revision": next,
					"updated_at":       time.Now().UTC(),
				}).Error; err != nil {
				return err
			}
			candidate.SupersedesFactID = &current.ID
		}
		updates := map[string]any{
			"review_state":            "confirmed",
			"lifecycle_state":         "active",
			"excluded_from_reasoning": false,
			"valid_from":              effectiveAt,
			"valid_until":             nil,
			"supersedes_fact_id":      candidate.SupersedesFactID,
			"updated_revision":        next,
			"updated_at":              time.Now().UTC(),
		}
		if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
			Where("id = ? AND user_id = ?", candidate.ID, userID).
			Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", candidate.ID).First(&stored).Error; err != nil {
			return err
		}
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "fact.candidate_accepted", source, map[string]any{
			"candidate": stored, "previous": previous, "effective_at": effectiveAt,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

// UpdateFactReviewState records explicit user review without changing origin,
// value, or temporal truth. Confirming an AI-extracted fact is not a correction.
func (r *BodyStateRepository) UpdateFactReviewState(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	factID uuid.UUID,
	reviewState string,
	source string,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	var stored model.BodyStateFact
	var committed *model.BodyStateRevision
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		var before model.BodyStateFact
		if err := tx.WithContext(ctx).
			Where("id = ? AND user_id = ?", factID, userID).
			First(&before).Error; err != nil {
			return err
		}
		switch reviewState {
		case "unverified", "confirmed", "rejected":
		default:
			return fmt.Errorf("invalid fact review state %q", reviewState)
		}
		excluded := reviewState == "rejected"
		if before.ReviewState == reviewState && before.ExcludedFromReasoning == excluded {
			stored = before
			return nil
		}
		if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
			Where("id = ? AND user_id = ?", factID, userID).
			Updates(map[string]any{
				"review_state":            reviewState,
				"excluded_from_reasoning": excluded,
				"updated_revision":        next,
				"updated_at":              time.Now().UTC(),
			}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", factID).First(&stored).Error; err != nil {
			return err
		}
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "fact.reviewed", source, map[string]any{
			"fact_id": factID,
			"before":  before,
			"after":   stored,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

// UpdateFactTemporal means the old claim was true, but the user's state later
// changed. This is deliberately different from CorrectFact.
func (r *BodyStateRepository) UpdateFactTemporal(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	factID uuid.UUID,
	lifecycleState string,
	trend string,
	validUntil *time.Time,
	source string,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	var stored model.BodyStateFact
	var committed *model.BodyStateRevision

	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		var before model.BodyStateFact
		if err := tx.WithContext(ctx).Where("id = ? AND user_id = ?", factID, userID).First(&before).Error; err != nil {
			return err
		}
		lifecycleState = bodyStateDefaultString(lifecycleState, before.LifecycleState)
		trend = bodyStateDefaultString(trend, before.Trend)
		if before.LifecycleState == lifecycleState && before.Trend == trend && bodyStateSameTime(before.ValidUntil, validUntil) {
			stored = before
			return nil
		}
		if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
			Where("id = ? AND user_id = ?", factID, userID).
			Updates(map[string]any{
				"lifecycle_state": lifecycleState, "trend": trend, "valid_until": validUntil,
				"updated_revision": next, "updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", factID).First(&stored).Error; err != nil {
			return err
		}
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "fact.temporal_changed", source, map[string]any{
			"fact_id": factID, "before": before, "after": stored,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

func (r *BodyStateRepository) UpsertObservation(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	observation model.BodyStateObservation,
	source string,
) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	var stored model.BodyStateObservation
	var committed *model.BodyStateRevision

	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}

		if observation.SourceKey != "" {
			var existing model.BodyStateObservation
			err := tx.WithContext(ctx).
				Where("user_id = ? AND source_key = ?", userID, observation.SourceKey).
				First(&existing).Error
			if err == nil {
				bodyStateApplyObservationDefaults(&observation)
				if bodyStateSameObservation(existing, observation) {
					stored = existing
					return nil
				}
				before := existing
				if err := tx.WithContext(ctx).Model(&model.BodyStateObservation{}).
					Where("id = ? AND user_id = ?", existing.ID, userID).
					Updates(map[string]any{
						"concern_key": observation.ConcernKey, "kind": observation.Kind,
						"body_region": observation.BodyRegion, "method": observation.Method,
						"value": observation.Value, "condition": observation.Condition,
						"provenance": observation.Provenance, "observed_at": observation.ObservedAt,
						"review_state":            observation.ReviewState,
						"lifecycle_state":         observation.LifecycleState,
						"excluded_from_reasoning": observation.ExcludedFromReasoning,
						"updated_revision":        next, "updated_at": time.Now().UTC(),
					}).Error; err != nil {
					return err
				}
				if err := tx.WithContext(ctx).Where("id = ?", existing.ID).First(&stored).Error; err != nil {
					return err
				}
				committed, err = bodyStatePersistRevision(ctx, tx, state, next, "observation.updated", source, map[string]any{
					"observation_id": existing.ID, "before": before, "after": stored,
				})
				return err
			}
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		bodyStateApplyObservationDefaults(&observation)
		observation.ID = uuid.New()
		observation.UserID = userID
		observation.CreatedRevision = next
		observation.UpdatedRevision = next
		if err := tx.WithContext(ctx).Create(&observation).Error; err != nil {
			return err
		}
		stored = observation
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "observation.added", source, map[string]any{"observation": observation})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

// TransitionObservation closes the previous current measurement/observation and
// creates a new one linked through supersedes_observation_id. Point-in-time
// measurements therefore keep their history without turning UserProfile into a
// mutable health record.
func (r *BodyStateRepository) TransitionObservation(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	targetObservationID uuid.UUID,
	replacement model.BodyStateObservation,
	source string,
) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	var stored model.BodyStateObservation
	var committed *model.BodyStateRevision

	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		var previous model.BodyStateObservation
		if err := tx.WithContext(ctx).Where("id = ? AND user_id = ?", targetObservationID, userID).First(&previous).Error; err != nil {
			return err
		}
		if previous.LifecycleState != "active" {
			return fmt.Errorf("cannot transition non-active observation %s", targetObservationID)
		}
		if replacement.Kind == "" {
			replacement.Kind = previous.Kind
		}
		if replacement.Kind != previous.Kind {
			return fmt.Errorf("observation transition must preserve kind %q", previous.Kind)
		}

		before := previous
		if err := tx.WithContext(ctx).Model(&model.BodyStateObservation{}).
			Where("id = ? AND user_id = ?", targetObservationID, userID).
			Updates(map[string]any{
				"lifecycle_state":  "inactive",
				"updated_revision": next,
				"updated_at":       time.Now().UTC(),
			}).Error; err != nil {
			return err
		}

		bodyStateApplyObservationDefaults(&replacement)
		replacement.ID = uuid.New()
		replacement.UserID = userID
		replacement.SupersedesObservationID = &targetObservationID
		replacement.LifecycleState = "active"
		replacement.CreatedRevision = next
		replacement.UpdatedRevision = next
		if err := tx.WithContext(ctx).Create(&replacement).Error; err != nil {
			return err
		}
		stored = replacement
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "observation.transitioned", source, map[string]any{
			"previous":    before,
			"replacement": replacement,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

func (r *BodyStateRepository) UpdateObservationReviewState(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	observationID uuid.UUID,
	reviewState string,
	source string,
) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	if reviewState != "unverified" && reviewState != "confirmed" && reviewState != "rejected" {
		return nil, nil, fmt.Errorf("invalid observation review state %q", reviewState)
	}
	var stored model.BodyStateObservation
	var committed *model.BodyStateRevision
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		var before model.BodyStateObservation
		if err := tx.WithContext(ctx).
			Where("id = ? AND user_id = ?", observationID, userID).
			First(&before).Error; err != nil {
			return err
		}
		excluded := reviewState != "confirmed"
		if before.ReviewState == reviewState && before.ExcludedFromReasoning == excluded {
			stored = before
			return nil
		}
		if err := tx.WithContext(ctx).Model(&model.BodyStateObservation{}).
			Where("id = ? AND user_id = ?", observationID, userID).
			Updates(map[string]any{
				"review_state":            reviewState,
				"excluded_from_reasoning": excluded,
				"updated_revision":        next,
				"updated_at":              time.Now().UTC(),
			}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", observationID).First(&stored).Error; err != nil {
			return err
		}
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "observation.reviewed", source, map[string]any{
			"observation_id": observationID,
			"before":         before,
			"after":          stored,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

// SetSafetyState persists a first-class safety projection. Positive safety
// findings can therefore constrain Diagnosis/Treatment even after the SSE turn is
// over. Clearing a safety state should be an explicit later policy decision.
func (r *BodyStateRepository) SetSafetyState(
	ctx context.Context,
	userID uuid.UUID,
	safetyState datatypes.JSON,
	source string,
) (*model.BodyStateRevision, error) {
	var committed *model.BodyStateRevision
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, nil)
		if err != nil {
			return err
		}
		safetyState = bodyStateJSON(safetyState, `{}`)
		if bodyStateSameJSON(state.SafetyState, safetyState, `{}`) {
			return nil
		}
		if err := tx.WithContext(ctx).Model(&model.BodyState{}).
			Where("user_id = ?", userID).
			Update("safety_state", safetyState).Error; err != nil {
			return err
		}
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "safety.updated", source, map[string]any{
			"before": json.RawMessage(bodyStateJSON(state.SafetyState, `{}`)),
			"after":  json.RawMessage(safetyState),
		})
		return err
	})
	return committed, err
}

// ApplyCurrentContextPatch applies one semantically coherent user context
// mutation under one aggregate lock and produces at most one revision. It is the
// batch primitive behind multi-field Lifestyle/body-metrics saves.
func (r *BodyStateRepository) ApplyCurrentContextPatch(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	patch model.BodyStateCurrentContextPatch,
	source string,
) (*model.BodyStateRevision, error) {
	var committed *model.BodyStateRevision
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		factChanges := make([]map[string]any, 0, len(patch.Facts))
		observationChanges := make([]map[string]any, 0, len(patch.Observations))

		for _, mutation := range patch.Facts {
			kind := strings.TrimSpace(mutation.Kind)
			if kind == "" {
				return errors.New("current fact mutation kind is required")
			}
			current, err := bodyStateFindSingletonFact(ctx, tx, userID, kind)
			if err != nil {
				return err
			}
			effectiveAt := mutation.EffectiveAt.UTC()
			if mutation.EffectiveAt.IsZero() {
				effectiveAt = time.Now().UTC()
			}
			if current != nil && current.ValidFrom != nil && effectiveAt.Before(current.ValidFrom.UTC()) {
				return fmt.Errorf("fact transition time precedes current fact validity for %q", kind)
			}

			if mutation.Replacement == nil || strings.TrimSpace(mutation.Replacement.Value) == "" {
				if current == nil {
					continue
				}
				before := *current
				if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
					Where("id = ? AND user_id = ?", current.ID, userID).
					Updates(map[string]any{
						"lifecycle_state":  "inactive",
						"valid_until":      effectiveAt,
						"updated_revision": next,
						"updated_at":       time.Now().UTC(),
					}).Error; err != nil {
					return err
				}
				factChanges = append(factChanges, map[string]any{
					"kind": kind, "action": "closed", "previous": before, "effective_at": effectiveAt,
				})
				continue
			}

			candidate := *mutation.Replacement
			candidate.Kind = kind
			candidate.Value = strings.TrimSpace(candidate.Value)
			bodyStateApplyFactDefaults(&candidate)
			candidate.LifecycleState = "active"
			candidate.ValidUntil = nil
			if current != nil && bodyStateSameCurrentFactClaim(*current, candidate) {
				continue
			}
			if current != nil {
				before := *current
				if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
					Where("id = ? AND user_id = ?", current.ID, userID).
					Updates(map[string]any{
						"lifecycle_state":  "inactive",
						"valid_until":      effectiveAt,
						"updated_revision": next,
						"updated_at":       time.Now().UTC(),
					}).Error; err != nil {
					return err
				}
				candidate.SupersedesFactID = &current.ID
				factChanges = append(factChanges, map[string]any{
					"kind": kind, "action": "transitioned", "previous": before, "replacement": candidate,
					"effective_at": effectiveAt,
				})
			} else {
				factChanges = append(factChanges, map[string]any{
					"kind": kind, "action": "added", "replacement": candidate, "effective_at": effectiveAt,
				})
			}
			candidate.ID = uuid.New()
			candidate.UserID = userID
			candidate.ValidFrom = &effectiveAt
			candidate.CreatedRevision = next
			candidate.UpdatedRevision = next
			if err := tx.WithContext(ctx).Create(&candidate).Error; err != nil {
				return err
			}
			// Replace the pre-create copy in revision details with durable identity.
			factChanges[len(factChanges)-1]["replacement"] = candidate
		}

		for _, mutation := range patch.Observations {
			kind := strings.TrimSpace(mutation.Kind)
			if kind == "" {
				return errors.New("current observation mutation kind is required")
			}
			current, err := bodyStateFindSingletonObservation(ctx, tx, userID, kind)
			if err != nil {
				return err
			}
			if mutation.Replacement == nil {
				if current == nil {
					continue
				}
				before := *current
				if err := tx.WithContext(ctx).Model(&model.BodyStateObservation{}).
					Where("id = ? AND user_id = ?", current.ID, userID).
					Updates(map[string]any{
						"lifecycle_state":  "inactive",
						"updated_revision": next,
						"updated_at":       time.Now().UTC(),
					}).Error; err != nil {
					return err
				}
				observationChanges = append(observationChanges, map[string]any{
					"kind": kind, "action": "closed", "previous": before,
				})
				continue
			}

			candidate := *mutation.Replacement
			candidate.Kind = kind
			bodyStateApplyObservationDefaults(&candidate)
			candidate.LifecycleState = "active"
			if current != nil && bodyStateSameCurrentObservationClaim(*current, candidate) {
				continue
			}
			if current != nil {
				before := *current
				if err := tx.WithContext(ctx).Model(&model.BodyStateObservation{}).
					Where("id = ? AND user_id = ?", current.ID, userID).
					Updates(map[string]any{
						"lifecycle_state":  "inactive",
						"updated_revision": next,
						"updated_at":       time.Now().UTC(),
					}).Error; err != nil {
					return err
				}
				candidate.SupersedesObservationID = &current.ID
				observationChanges = append(observationChanges, map[string]any{
					"kind": kind, "action": "transitioned", "previous": before, "replacement": candidate,
				})
			} else {
				observationChanges = append(observationChanges, map[string]any{
					"kind": kind, "action": "added", "replacement": candidate,
				})
			}
			candidate.ID = uuid.New()
			candidate.UserID = userID
			candidate.CreatedRevision = next
			candidate.UpdatedRevision = next
			if err := tx.WithContext(ctx).Create(&candidate).Error; err != nil {
				return err
			}
			observationChanges[len(observationChanges)-1]["replacement"] = candidate
		}

		if len(factChanges) == 0 && len(observationChanges) == 0 {
			return nil
		}
		committed, err = bodyStatePersistRevision(ctx, tx, state, next, "current_context.updated", source, map[string]any{
			"facts": factChanges, "observations": observationChanges,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return committed, nil
}

func bodyStateFindSingletonFact(ctx context.Context, tx *gorm.DB, userID uuid.UUID, kind string) (*model.BodyStateFact, error) {
	var items []model.BodyStateFact
	if err := tx.WithContext(ctx).
		Where("user_id = ? AND kind = ? AND lifecycle_state = ? AND review_state = ? AND excluded_from_reasoning = FALSE", userID, kind, "active", "confirmed").
		Limit(2).Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("multiple active facts exist for singleton kind %q", kind)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func bodyStateFindSingletonObservation(ctx context.Context, tx *gorm.DB, userID uuid.UUID, kind string) (*model.BodyStateObservation, error) {
	var items []model.BodyStateObservation
	if err := tx.WithContext(ctx).
		Where("user_id = ? AND kind = ? AND lifecycle_state = ? AND excluded_from_reasoning = FALSE", userID, kind, "active").
		Limit(2).Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("multiple active observations exist for singleton kind %q", kind)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func bodyStateSameCurrentFactClaim(current, candidate model.BodyStateFact) bool {
	return strings.TrimSpace(current.Value) == strings.TrimSpace(candidate.Value) &&
		bodyStateSameJSON(current.Details, candidate.Details, `{}`)
}

func bodyStateSameCurrentObservationClaim(current, candidate model.BodyStateObservation) bool {
	return bodyStateSameJSON(current.Value, candidate.Value, `{}`) &&
		bodyStateSameJSON(current.Condition, candidate.Condition, `{}`)
}

func bodyStateLockNextRevision(ctx context.Context, tx *gorm.DB, userID uuid.UUID, expected *int64) (*model.BodyState, int64, error) {
	seed := model.BodyState{UserID: userID, SafetyState: datatypes.JSON(`{}`)}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return nil, 0, err
	}
	var state model.BodyState
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&state).Error; err != nil {
		return nil, 0, err
	}
	if expected != nil && state.CurrentRevision != *expected {
		return nil, 0, fmt.Errorf("%w: expected %d, current %d", ErrBodyStateRevisionConflict, *expected, state.CurrentRevision)
	}
	return &state, state.CurrentRevision + 1, nil
}

func bodyStatePersistRevision(ctx context.Context, tx *gorm.DB, state *model.BodyState, revision int64, changeType, source string, changes any) (*model.BodyStateRevision, error) {
	encoded, err := json.Marshal(changes)
	if err != nil {
		return nil, err
	}
	record := &model.BodyStateRevision{
		ID: uuid.New(), UserID: state.UserID, Revision: revision,
		ChangeType: changeType, Source: bodyStateDefaultString(source, "unknown"), Changes: datatypes.JSON(encoded),
	}
	if err := tx.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	if err := tx.WithContext(ctx).Model(&model.BodyState{}).
		Where("user_id = ?", state.UserID).
		Updates(map[string]any{"current_revision": revision, "updated_at": time.Now().UTC()}).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func bodyStateApplyFactDefaults(fact *model.BodyStateFact) {
	fact.Details = bodyStateJSON(fact.Details, `{}`)
	fact.Provenance = bodyStateJSON(fact.Provenance, `{}`)
	fact.Origin = bodyStateDefaultString(fact.Origin, "user_reported")
	fact.ReviewState = bodyStateDefaultString(fact.ReviewState, "unverified")
	fact.LifecycleState = bodyStateDefaultString(fact.LifecycleState, "active")
	fact.Trend = bodyStateDefaultString(fact.Trend, "unknown")
}

func bodyStateApplyObservationDefaults(observation *model.BodyStateObservation) {
	observation.Value = bodyStateJSON(observation.Value, `{}`)
	observation.Condition = bodyStateJSON(observation.Condition, `{}`)
	observation.Provenance = bodyStateJSON(observation.Provenance, `{}`)
	observation.ReviewState = bodyStateDefaultString(observation.ReviewState, "unverified")
	observation.LifecycleState = bodyStateDefaultString(observation.LifecycleState, "active")
	observation.ExcludedFromReasoning = observation.ReviewState != "confirmed"
}

func bodyStateSameFact(a, b model.BodyStateFact) bool {
	return a.ConcernKey == b.ConcernKey && a.Kind == b.Kind && a.BodyRegion == b.BodyRegion &&
		a.Value == b.Value && bodyStateSameJSON(a.Details, b.Details, `{}`) &&
		a.Origin == b.Origin && a.ReviewState == b.ReviewState && a.LifecycleState == b.LifecycleState &&
		a.Trend == b.Trend && bodyStateSameJSON(a.Provenance, b.Provenance, `{}`) &&
		bodyStateSameTime(a.ObservedAt, b.ObservedAt) && bodyStateSameTime(a.ValidFrom, b.ValidFrom) &&
		bodyStateSameTime(a.ValidUntil, b.ValidUntil) && a.ExcludedFromReasoning == b.ExcludedFromReasoning
}

func bodyStateSameObservation(a, b model.BodyStateObservation) bool {
	return a.ConcernKey == b.ConcernKey && a.Kind == b.Kind && a.BodyRegion == b.BodyRegion && a.Method == b.Method &&
		bodyStateSameJSON(a.Value, b.Value, `{}`) &&
		bodyStateSameJSON(a.Condition, b.Condition, `{}`) &&
		bodyStateSameJSON(a.Provenance, b.Provenance, `{}`) &&
		bodyStateSameTime(a.ObservedAt, b.ObservedAt) && a.ReviewState == b.ReviewState &&
		a.LifecycleState == b.LifecycleState && a.ExcludedFromReasoning == b.ExcludedFromReasoning
}

func bodyStateJSON(value datatypes.JSON, fallback string) datatypes.JSON {
	if len(value) == 0 {
		return datatypes.JSON(fallback)
	}
	return value
}

func bodyStateSameJSON(a, b datatypes.JSON, fallback string) bool {
	var left any
	var right any
	if err := json.Unmarshal(bodyStateJSON(a, fallback), &left); err != nil {
		return bytes.Equal(bodyStateJSON(a, fallback), bodyStateJSON(b, fallback))
	}
	if err := json.Unmarshal(bodyStateJSON(b, fallback), &right); err != nil {
		return false
	}
	leftCanonical, leftErr := json.Marshal(left)
	rightCanonical, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftCanonical, rightCanonical)
}

func bodyStateDefaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func bodyStateSameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
