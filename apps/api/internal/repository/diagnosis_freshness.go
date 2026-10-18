package repository

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

const maxDiagnosisFreshnessRuns = 1000

// AnalysisRunBodyStateRevisions returns the frozen BodyState revision each of
// the owner's formal runs started from. Foreign or unknown ids are absent.
func (r *DiagnosisRepository) AnalysisRunBodyStateRevisions(ctx context.Context, owner uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	result := make(map[uuid.UUID]int64, len(ids))
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if len(ids) == 0 {
		return result, nil
	}
	if len(ids) > maxDiagnosisFreshnessRuns {
		return nil, fmt.Errorf("too many formal runs for one freshness read")
	}
	var rows []struct {
		ID                       uuid.UUID
		BasedOnBodyStateRevision int64
	}
	if err := database.FromContext(ctx, r.db).Model(&model.ConditionAnalysisRun{}).
		Select("id", "based_on_body_state_revision").
		Where("user_id = ? AND id IN ?", owner, ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.BasedOnBodyStateRevision
	}
	return result, nil
}
