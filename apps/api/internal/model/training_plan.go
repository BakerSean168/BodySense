package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TrainingPlan represents a training plan.
type TrainingPlan struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID              uuid.UUID       `gorm:"type:uuid;index;not null" json:"user_id"`
	ConsultationID      *uuid.UUID      `gorm:"type:uuid" json:"consultation_id,omitempty"`
	TreatmentID         *uuid.UUID      `gorm:"type:uuid;index" json:"treatment_id,omitempty"`
	TreatmentRevisionID *uuid.UUID      `gorm:"type:uuid;index" json:"treatment_revision_id,omitempty"`
	Status              string          `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	Goal                string          `gorm:"type:text;not null" json:"goal"`
	DurationWeeks       int             `gorm:"not null;default:4" json:"duration_weeks"`
	CurrentWeek         int             `gorm:"not null;default:1" json:"current_week"`
	Phases              json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"phases"`
	CreatedAt           time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

func (TrainingPlan) TableName() string { return "training_plans" }

// TrainingLog represents a daily training log entry.
type TrainingLog struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID              uuid.UUID       `gorm:"type:uuid;index;not null" json:"user_id"`
	PlanID              uuid.UUID       `gorm:"type:uuid;index;not null" json:"plan_id"`
	TreatmentRevisionID *uuid.UUID      `gorm:"type:uuid;index" json:"treatment_revision_id,omitempty"`
	InterventionID      *uuid.UUID      `gorm:"type:uuid;index" json:"intervention_id,omitempty"`
	Date                time.Time       `gorm:"type:date;not null;default:current_date" json:"date"`
	Exercises           json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"exercises"`
	Notes               *string         `gorm:"type:text" json:"notes,omitempty"`
	IsCheckedIn         bool            `gorm:"not null;default:false" json:"is_checked_in"`
	OutcomeRecordedAt   *time.Time      `json:"outcome_recorded_at,omitempty"`
	CreatedAt           time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

func (TrainingLog) TableName() string { return "training_logs" }
