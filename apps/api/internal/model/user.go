package model

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system.
type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Email        string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// TableName specifies the table name for GORM.
func (User) TableName() string {
	return "users"
}
