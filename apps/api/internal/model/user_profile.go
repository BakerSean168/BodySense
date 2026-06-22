package model

import (
	"time"

	"github.com/google/uuid"
)

// UserProfile represents a user's body profile information.
type UserProfile struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID           uuid.UUID  `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Gender           *string    `gorm:"type:varchar(20)" json:"gender,omitempty"`
	Age              *int       `json:"age,omitempty"`
	HeightCm         *float64   `gorm:"type:decimal(5,1)" json:"height_cm,omitempty"`
	WeightKg         *float64   `gorm:"type:decimal(5,1)" json:"weight_kg,omitempty"`
	BMI              *float64   `gorm:"type:decimal(4,1)" json:"bmi,omitempty"`
	Occupation       *string    `gorm:"type:varchar(100)" json:"occupation,omitempty"`
	SleepTime        *string    `gorm:"type:time" json:"sleep_time,omitempty"`
	WakeTime         *string    `gorm:"type:time" json:"wake_time,omitempty"`
	ExerciseType     *string    `gorm:"type:varchar(100)" json:"exercise_type,omitempty"`
	ExerciseFrequency *string   `gorm:"type:varchar(50)" json:"exercise_frequency,omitempty"`
	InjuryHistory    *string    `gorm:"type:text" json:"injury_history,omitempty"`
	SelfDescription  *string    `gorm:"type:text" json:"self_description,omitempty"`
	CreatedAt        time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (UserProfile) TableName() string {
	return "user_profiles"
}
