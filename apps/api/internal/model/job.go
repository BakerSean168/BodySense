package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Job represents a durable background job (e.g., OCR, knowledge ingestion).
type Job struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	RunID          *uuid.UUID     `gorm:"type:uuid" json:"run_id,omitempty"`
	ConversationID *uuid.UUID     `gorm:"type:uuid" json:"conversation_id,omitempty"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	JobType        string         `gorm:"type:varchar(50);not null;index" json:"job_type"`
	Status         string         `gorm:"type:varchar(30);not null;default:'pending';index" json:"status"`
	Input          datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"input"`
	Progress       datatypes.JSON `gorm:"type:jsonb" json:"progress,omitempty"`
	Result         datatypes.JSON `gorm:"type:jsonb" json:"result,omitempty"`
	Error          datatypes.JSON `gorm:"type:jsonb" json:"error,omitempty"`
	CreatedAt      time.Time      `gorm:"not null;default:now()" json:"created_at"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
}

// TableName specifies the table name for GORM.
func (Job) TableName() string {
	return "jobs"
}

// JobEvent represents an append-only event within a job's lifecycle.
type JobEvent struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	JobID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"job_id"`
	EventType string         `gorm:"type:varchar(50);not null" json:"event_type"`
	Payload   datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"payload"`
	CreatedAt time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the table name for GORM.
func (JobEvent) TableName() string {
	return "job_events"
}
