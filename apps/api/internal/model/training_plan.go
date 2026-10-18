package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TrainingPlan represents a training plan.
type TrainingPlan struct {
	ID                        uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Title                     string          `gorm:"not null;default:''" json:"-"`
	CurrentAcceptedRevisionID *uuid.UUID      `gorm:"type:uuid" json:"-"`
	UserID                    uuid.UUID       `gorm:"type:uuid;index;not null" json:"user_id"`
	ConsultationID            *uuid.UUID      `gorm:"type:uuid" json:"consultation_id,omitempty"`
	Status                    string          `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	StatusRevision            int64           `gorm:"not null;default:0" json:"-"`
	Goal                      string          `gorm:"type:text;not null" json:"goal"`
	DurationWeeks             int             `gorm:"not null;default:4" json:"duration_weeks"`
	CurrentWeek               int             `gorm:"not null;default:1" json:"current_week"`
	Phases                    json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"phases"`
	CreatedAt                 time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

func (TrainingPlan) TableName() string { return "training_plans" }
