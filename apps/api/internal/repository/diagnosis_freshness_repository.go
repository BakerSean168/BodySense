package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DiagnosisFreshnessRepository stores mutable freshness classification outside
// immutable DiagnosisAnalysis rows.
type DiagnosisFreshnessRepository struct {
	db *gorm.DB
}

func NewDiagnosisFreshnessRepository(db *gorm.DB) *DiagnosisFreshnessRepository {
	return &DiagnosisFreshnessRepository{db: db}
}

func (r *DiagnosisFreshnessRepository) Upsert(
	ctx context.Context,
	freshness *model.DiagnosisAnalysisFreshness,
) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "analysis_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id", "state", "evaluated_against_revision", "reasons", "checked_at",
		}),
	}).Create(freshness).Error
}

func (r *DiagnosisFreshnessRepository) Get(
	ctx context.Context,
	analysisID, userID uuid.UUID,
) (*model.DiagnosisAnalysisFreshness, error) {
	var item model.DiagnosisAnalysisFreshness
	err := r.db.WithContext(ctx).
		Where("analysis_id = ? AND user_id = ?", analysisID, userID).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
