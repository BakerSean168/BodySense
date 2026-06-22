package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ConsultationSession represents a consultation chat session.
type ConsultationSession struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID        uuid.UUID       `gorm:"type:uuid;index;not null" json:"user_id"`
	Messages      json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"messages"`
	ExtractedInfo json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"extracted_info"`
	Diagnosis     json.RawMessage `gorm:"type:jsonb" json:"diagnosis,omitempty"`
	TreatmentPlan json.RawMessage `gorm:"type:jsonb" json:"treatment_plan,omitempty"`
	Status        string          `gorm:"type:varchar(20);not null;default:'in_progress'" json:"status"`
	CreatedAt     time.Time       `gorm:"not null;default:now()" json:"created_at"`
	EndedAt       *time.Time      `json:"ended_at,omitempty"`
}

// TableName specifies the table name for GORM.
func (ConsultationSession) TableName() string {
	return "consultation_sessions"
}
