package model

import (
	"time"

	"github.com/google/uuid"
)

// UserProfile is intentionally small. It owns stable identity context only;
// time-varying health state (measurements, lifestyle, injuries, symptoms) lives
// in the longitudinal BodyState aggregate.
type UserProfile struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Gender    *string   `gorm:"type:varchar(20)" json:"gender,omitempty"`
	BirthDate *DateOnly `gorm:"type:date" json:"birth_date,omitempty"`
	AgeYears  *int      `gorm:"-" json:"age_years,omitempty"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (UserProfile) TableName() string { return "user_profiles" }
