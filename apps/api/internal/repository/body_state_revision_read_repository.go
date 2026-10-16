package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// maxBodyFactLookup bounds one id resolution; a revision window is at most
// 500 revisions and each V2 commit carries a bounded fact batch.
const maxBodyFactLookup = 2000

// GetBodyStateHead reads only the aggregate head (revision + Safety state). A
// user without a BodyState row has revision 0 and an empty Safety state.
func (r *BodyStateRepository) GetBodyStateHead(ctx context.Context, owner uuid.UUID) (*model.BodyStateHead, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid owner", model.ErrInvalidBodyFact)
	}
	var state model.BodyState
	err := database.FromContext(ctx, r.db).Select("user_id", "current_revision").
		Where("user_id = ?", owner).Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.BodyStateHead{UserID: owner}, nil
	}
	if err != nil {
		return nil, err
	}
	if state.UserID != owner {
		return nil, fmt.Errorf("body state head owner mismatch")
	}
	return &model.BodyStateHead{UserID: owner, CurrentRevision: state.CurrentRevision}, nil
}

// GetBodyFactsByIDs resolves V2 fact ids named by revisions, whatever their
// assertion status (a corrected or withdrawn target still has a kind and
// regions). Only the owner's rows are returned; foreign ids resolve to nothing.
func (r *BodyStateRepository) GetBodyFactsByIDs(ctx context.Context, owner uuid.UUID, ids []uuid.UUID) ([]model.BodyFact, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid owner", model.ErrInvalidBodyFact)
	}
	if len(ids) == 0 {
		return []model.BodyFact{}, nil
	}
	if len(ids) > maxBodyFactLookup {
		return nil, fmt.Errorf("%w: too many fact ids", model.ErrInvalidBodyFact)
	}
	facts := []model.BodyFact{}
	if err := database.FromContext(ctx, r.db).Where("user_id = ? AND id IN ?", owner, ids).Find(&facts).Error; err != nil {
		return nil, err
	}
	for index := range facts {
		if facts[index].UserID != owner {
			return nil, fmt.Errorf("body fact owner mismatch")
		}
	}
	return facts, nil
}
