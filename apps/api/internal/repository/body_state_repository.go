package repository

import (
	"bytes"
	"context"
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
		if before.ReviewState == reviewState {
			stored = before
			return nil
		}
		if err := tx.WithContext(ctx).Model(&model.BodyStateFact{}).
			Where("id = ? AND user_id = ?", factID, userID).
			Updates(map[string]any{
				"review_state":     reviewState,
				"updated_revision": next,
				"updated_at":       time.Now().UTC(),
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
