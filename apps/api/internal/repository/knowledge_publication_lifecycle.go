package repository

import (
	"context"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

// KnowledgePublicationUnit is the locked publication view of one knowledge unit.
type KnowledgePublicationUnit struct {
	ID                  int64          `gorm:"column:id"`
	UnitKey             string         `gorm:"column:unit_key"`
	Title               string         `gorm:"column:title"`
	BodyMarkdown        string         `gorm:"column:body_markdown"`
	TranscriptExcerpt   string         `gorm:"column:transcript_excerpt"`
	LifecycleStatus     string         `gorm:"column:lifecycle_status"`
	ReviewStatus        string         `gorm:"column:review_status"`
	QualityScore        float64        `gorm:"column:quality_score"`
	ContentHash         *string        `gorm:"column:content_hash"`
	PublicationID       *uuid.UUID     `gorm:"column:publication_id"`
	PublishedVersion    *int           `gorm:"column:published_version"`
	Metadata            datatypes.JSON `gorm:"column:metadata"`
	SourceType          string         `gorm:"column:source_type"`
	SourceLicenseStatus string         `gorm:"column:source_license_status"`
	HasEmbedding        bool           `gorm:"column:has_embedding"`
}

func (r *KnowledgePublicationRepository) LockUnitsByKeys(
	ctx context.Context,
	unitKeys []string,
) ([]KnowledgePublicationUnit, error) {
	var units []KnowledgePublicationUnit
	err := database.FromContext(ctx, r.db).
		Table("knowledge_units AS ku").
		Select(`ku.id, ku.unit_key, ku.title, ku.body_markdown, ku.transcript_excerpt,
			ku.lifecycle_status, ku.review_status, COALESCE(ku.quality_score, 0) AS quality_score,
			ku.content_hash, ku.publication_id, ku.published_version, ku.metadata,
			ks.source_type, ks.license_status AS source_license_status,
			(ku.embedding IS NOT NULL) AS has_embedding`).
		Joins("JOIN knowledge_sources ks ON ks.id = ku.source_id").
		Where("ku.unit_key IN ?", unitKeys).
		Order("ku.unit_key ASC").
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "ku"}}).
		Scan(&units).Error
	return units, err
}

func (r *KnowledgePublicationRepository) LockPublicationByKey(
	ctx context.Context,
	publicationKey string,
) (*model.KnowledgePublication, error) {
	var publication model.KnowledgePublication
	err := database.FromContext(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("publication_key = ?", publicationKey).
		First(&publication).Error
	if err != nil {
		return nil, err
	}
	return &publication, nil
}

func (r *KnowledgePublicationRepository) NextBatchVersion(
	ctx context.Context,
	batchKey string,
) (int, error) {
	var next int
	err := database.FromContext(ctx, r.db).
		Model(&model.KnowledgePublication{}).
		Where("publication_batch_key = ?", batchKey).
		Select("COALESCE(MAX(published_version), 0) + 1").
		Scan(&next).Error
	if next <= 0 {
		next = 1
	}
	return next, err
}

func (r *KnowledgePublicationRepository) UpdateUnitForPublication(
	ctx context.Context,
	unitID int64,
	publicationID uuid.UUID,
	publishedVersion int,
) error {
	return database.FromContext(ctx, r.db).
		Table("knowledge_units").
		Where("id = ?", unitID).
		Updates(map[string]any{
			"lifecycle_status":  "published",
			"publication_id":    publicationID,
			"published_version": publishedVersion,
		}).Error
}

func (r *KnowledgePublicationRepository) RestoreUnitPublicationState(
	ctx context.Context,
	unitID int64,
	lifecycleStatus string,
	reviewStatus string,
	qualityScore float64,
	publicationID *uuid.UUID,
	publishedVersion *int,
) error {
	return database.FromContext(ctx, r.db).
		Table("knowledge_units").
		Where("id = ?", unitID).
		Updates(map[string]any{
			"lifecycle_status":  lifecycleStatus,
			"review_status":     reviewStatus,
			"quality_score":     qualityScore,
			"publication_id":    publicationID,
			"published_version": publishedVersion,
		}).Error
}

func (r *KnowledgePublicationRepository) UpdatePublicationStatus(
	ctx context.Context,
	publicationID uuid.UUID,
	status string,
) error {
	return database.FromContext(ctx, r.db).
		Model(&model.KnowledgePublication{}).
		Where("id = ?", publicationID).
		Update("status", status).Error
}
