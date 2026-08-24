package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// KnowledgeSource is the durable, operator-registered identity of one source.
// Ingestion may enrich technical fields, but source identity/license/provenance
// are established before any generated Knowledge becomes eligible for review.
type KnowledgeSource struct {
	ID                 int64          `gorm:"primaryKey" json:"id"`
	SourceKey          string         `gorm:"type:varchar(200);not null;uniqueIndex" json:"source_key"`
	SourceType         string         `gorm:"type:varchar(50);not null" json:"source_type"`
	Title              string         `gorm:"type:varchar(500);not null" json:"title"`
	Author             string         `gorm:"type:varchar(255);not null" json:"author"`
	ProblemSlug        string         `gorm:"type:varchar(100);not null;index" json:"problem_slug"`
	ProblemDisplayName string         `gorm:"type:varchar(255);not null" json:"problem_display_name"`
	OriginalFilePath   string         `gorm:"type:text;not null" json:"original_file_path"`
	Language           string         `gorm:"type:varchar(20);not null;default:'zh'" json:"language"`
	IngestStatus       string         `gorm:"type:varchar(50);not null;default:'pending'" json:"ingest_status"`
	Metadata           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	LicenseStatus      string         `gorm:"type:varchar(50);not null;default:'unknown'" json:"license_status"`
	ContentHash        *string        `gorm:"type:text" json:"content_hash,omitempty"`
	CanonicalURL       *string        `gorm:"type:text" json:"canonical_url,omitempty"`
	SourceVersion      string         `gorm:"type:varchar(100);not null;default:'v1'" json:"source_version"`
	Provenance         datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"provenance"`
	RegisteredBy       *uuid.UUID     `gorm:"type:uuid;index" json:"registered_by,omitempty"`
	RegisteredAt       *time.Time     `json:"registered_at,omitempty"`
	CreatedAt          time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt          time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

func (KnowledgeSource) TableName() string { return "knowledge_sources" }
