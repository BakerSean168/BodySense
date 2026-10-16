package repository

import (
	"context"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// ListRevisionsAfter returns semantic changes newer than the pinned revision in
// ascending order. It is used by explicit freshness/review policies rather than
// treating every revision-number difference as invalidation.
func (r *BodyStateRepository) ListRevisionsAfter(
	ctx context.Context,
	userID uuid.UUID,
	afterRevision int64,
	limit int,
) ([]model.BodyStateRevision, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var revisions []model.BodyStateRevision
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND revision > ?", userID, afterRevision).
		Order("revision ASC").
		Limit(limit).
		Find(&revisions).Error
	return revisions, err
}
