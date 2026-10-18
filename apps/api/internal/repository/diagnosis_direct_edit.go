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

func (r *DiagnosisRepository) FindDirectEditByKey(ctx context.Context, owner uuid.UUID, key string) (*model.ConditionDirectEdit, error) {
	var row model.ConditionDirectEdit
	err := r.ownerDB(ctx, owner).Where("idempotency_key=?", key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// RevisionKeyTaken reports whether a settlement already used this key for its
// DiagnosisRevision; revision keys share one owner namespace.
func (r *DiagnosisRepository) RevisionKeyTaken(ctx context.Context, owner uuid.UUID, key string) (bool, error) {
	var count int64
	err := r.ownerDB(ctx, owner).Model(&model.DiagnosisRevision{}).Where("idempotency_key=?", key).Count(&count).Error
	return count > 0, err
}

func (r *DiagnosisRepository) InsertDirectEdit(ctx context.Context, row *model.ConditionDirectEdit) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("diagnosis direct edit requires a transaction")
	}
	return database.FromContext(ctx, r.db).Create(row).Error
}

// WriteDirectEditCondition changes only the user-editable projection columns,
// fenced by the revision the edit was planned against.
func (r *DiagnosisRepository) WriteDirectEditCondition(ctx context.Context, row *model.ConditionEntry, previousRevision int64) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("diagnosis direct edit requires a transaction")
	}
	result := database.FromContext(ctx, r.db).Model(&model.ConditionEntry{}).
		Where("user_id=? AND id=? AND committed_revision=?", row.UserID, row.ID, previousRevision).
		Select("qualifiers", "disposition", "committed_revision", "updated_at").Updates(row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return model.ErrDiagnosisDecisionConflict
	}
	return nil
}
