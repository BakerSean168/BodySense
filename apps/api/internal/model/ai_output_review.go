package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AIOutputReview stores governance validation results for AI outputs.
type AIOutputReview struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	UserID          *uuid.UUID     `gorm:"type:uuid;index" json:"user_id,omitempty"`
	RunID           *uuid.UUID     `gorm:"type:uuid" json:"run_id,omitempty"`
	JobID           *uuid.UUID     `gorm:"type:uuid" json:"job_id,omitempty"`
	ConversationID  *uuid.UUID     `gorm:"type:uuid" json:"conversation_id,omitempty"`
	OutputType      string         `gorm:"type:varchar(50);not null" json:"output_type"`
	Status          string         `gorm:"type:varchar(30);not null;default:'accepted';index" json:"status"`
	Issues          datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"issues"`
	ValidatedOutput datatypes.JSON `gorm:"type:jsonb" json:"validated_output,omitempty"`
	RawOutput       datatypes.JSON `gorm:"type:jsonb" json:"raw_output,omitempty"`
	Metadata        datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt       time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the table name for GORM.
func (AIOutputReview) TableName() string {
	return "ai_output_reviews"
}
