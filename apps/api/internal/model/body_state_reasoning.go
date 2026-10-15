package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// BodyStateEvidence preserves source identity and a minimal retrieved snapshot.
// Evidence supports reasoning; it never becomes a user Fact merely by being stored.
type BodyStateEvidence struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	SourceType    string         `gorm:"type:varchar(40);not null" json:"source_type"`
	SourceKey     string         `gorm:"type:text;not null" json:"source_key"`
	SourceVersion string         `gorm:"type:text;not null;default:''" json:"source_version"`
	Title         string         `gorm:"type:text;not null;default:''" json:"title"`
	Summary       string         `gorm:"type:text;not null;default:''" json:"summary"`
	Excerpt       string         `gorm:"type:text;not null;default:''" json:"excerpt"`
	Metadata      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	RetrievedAt   time.Time      `gorm:"not null;default:now()" json:"retrieved_at"`
	CreatedAt     time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (BodyStateEvidence) TableName() string { return "body_state_evidence" }
