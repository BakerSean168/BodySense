package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
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

// UpsertEvidence persists traceable source material independently from Facts.
// A stable UUID derived from the source identity lets Python and Go reference the
// same evidence before/after the retrieval snapshot is committed.
func (r *BodyStateRepository) UpsertEvidence(
	ctx context.Context,
	userID uuid.UUID,
	evidence model.BodyStateEvidence,
) (*model.BodyStateEvidence, error) {
	evidence.SourceType = strings.TrimSpace(evidence.SourceType)
	evidence.SourceKey = strings.TrimSpace(evidence.SourceKey)
	if evidence.SourceType == "" || evidence.SourceKey == "" {
		return nil, errors.New("evidence source_type and source_key are required")
	}
	identity := strings.Join([]string{
		"bodysense:evidence",
		userID.String(),
		evidence.SourceType,
		evidence.SourceKey,
		evidence.SourceVersion,
	}, ":")
	evidence.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(identity))
	evidence.UserID = userID
	if len(evidence.Metadata) == 0 {
		evidence.Metadata = datatypes.JSON(`{}`)
	}
	if evidence.RetrievedAt.IsZero() {
		evidence.RetrievedAt = time.Now().UTC()
	}
	if evidence.CreatedAt.IsZero() {
		evidence.CreatedAt = evidence.RetrievedAt
	}

	err := database.FromContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "source_type"},
			{Name: "source_key"},
			{Name: "source_version"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "summary", "excerpt", "metadata", "retrieved_at",
		}),
	}).Create(&evidence).Error
	if err != nil {
		return nil, err
	}
	// ON CONFLICT can leave the proposed ID in memory even when an older row was
	// updated. Reload by semantic identity so callers always receive the stored ID.
	var stored model.BodyStateEvidence
	if err := database.FromContext(ctx, r.db).
		Where(
			"user_id = ? AND source_type = ? AND source_key = ? AND source_version = ?",
			userID,
			evidence.SourceType,
			evidence.SourceKey,
			evidence.SourceVersion,
		).
		First(&stored).Error; err != nil {
		return nil, err
	}
	return &stored, nil
}

func (r *BodyStateRepository) ListEvidence(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]model.BodyStateEvidence, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var items []model.BodyStateEvidence
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("retrieved_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *BodyStateRepository) GetEvidenceByIDs(
	ctx context.Context,
	userID uuid.UUID,
	ids []uuid.UUID,
) ([]model.BodyStateEvidence, error) {
	if len(ids) == 0 {
		return []model.BodyStateEvidence{}, nil
	}
	var items []model.BodyStateEvidence
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND id IN ?", userID, ids).
		Find(&items).Error
	return items, err
}
