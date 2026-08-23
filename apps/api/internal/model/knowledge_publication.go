package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// KnowledgePublication tracks published versions of knowledge units.
type KnowledgePublication struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	KnowledgeUnitID  *int64         `gorm:"column:knowledge_unit_id;index" json:"knowledge_unit_id,omitempty"`
	PublicationKey   string         `gorm:"type:varchar(200);not null;uniqueIndex" json:"publication_key"`
	BatchKey         string         `gorm:"column:publication_batch_key;type:varchar(200);index" json:"publication_batch_key,omitempty"`
	RollbackOf       *uuid.UUID     `gorm:"column:rollback_of;type:uuid;index" json:"rollback_of,omitempty"`
	Title            string         `gorm:"type:varchar(500);not null;default:''" json:"title"`
	Summary          string         `gorm:"type:text;not null;default:''" json:"summary,omitempty"`
	UnitCount        int            `gorm:"not null;default:0" json:"unit_count"`
	PublishedVersion int            `gorm:"not null" json:"published_version"`
	PublishedAt      time.Time      `gorm:"not null;default:now()" json:"published_at"`
	PublishedBy      string         `gorm:"type:text" json:"published_by,omitempty"`
	CreatedBy        string         `gorm:"type:text" json:"created_by,omitempty"`
	Status           string         `gorm:"type:varchar(30);not null;default:'draft';index" json:"status"`
	Metadata         datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt        time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (KnowledgePublication) TableName() string {
	return "knowledge_publications"
}
