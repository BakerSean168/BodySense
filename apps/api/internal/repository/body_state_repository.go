package repository

import (
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

func bodyStateLockNextRevision(ctx context.Context, tx *gorm.DB, userID uuid.UUID, expected *int64) (*model.BodyState, int64, error) {
	seed := model.BodyState{UserID: userID}
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

func bodyStateDefaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
