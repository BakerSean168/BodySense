package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// PrivacyErasureRequest is the durable orchestration/audit record for a full
// user privacy erasure. It deliberately contains no health payload. On success
// SubjectUserID is nulled so the retained audit row is no longer directly
// joinable to the deleted account.
type PrivacyErasureRequest struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	SubjectUserID  *uuid.UUID     `gorm:"type:uuid" json:"-"`
	SubjectDigest  string         `gorm:"type:char(64);not null;uniqueIndex" json:"-"`
	Status         string         `gorm:"type:varchar(24);not null" json:"status"`
	AttemptCount   int            `gorm:"not null" json:"attempt_count"`
	Report         datatypes.JSON `gorm:"type:jsonb;not null" json:"report"`
	LastError      *string        `json:"-"`
	LeaseOwner     *string        `json:"-"`
	LeaseExpiresAt *time.Time     `json:"-"`
	RequestedAt    time.Time      `json:"requested_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
}

func (PrivacyErasureRequest) TableName() string { return "privacy_erasure_requests" }
