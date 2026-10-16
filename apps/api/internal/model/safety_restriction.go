package model

import (
	"time"

	"github.com/google/uuid"
)

const SafetyScreenRestrictionPolicyV1 = "safety-screen-restriction-v1"

// SafetyRestriction is an independently addressable Go decision from one
// positive, complete, user-authored screen concept. Current policy is explicitly
// restricted/all_training/professional_clearance; it does not classify urgency.
type SafetyRestriction struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	SafetyScreenID  uuid.UUID `gorm:"type:uuid;not null" json:"safety_screen_id"`
	Concept         string    `gorm:"not null" json:"concept"`
	PolicyRevision  string    `gorm:"not null" json:"policy_revision"`
	RiskLevel       string    `gorm:"not null" json:"risk_level"`
	ScopeKind       string    `gorm:"not null" json:"scope_kind"`
	LiftPolicy      string    `gorm:"not null" json:"lift_policy"`
	Reason          string    `gorm:"not null" json:"reason"`
	CreatedRevision int64     `gorm:"not null" json:"created_revision"`
	ValidFrom       time.Time `gorm:"not null" json:"valid_from"`
}

func (SafetyRestriction) TableName() string { return "safety_restrictions" }
