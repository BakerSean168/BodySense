package model

import (
	"time"

	"github.com/google/uuid"
)

const SafetyScreenRestrictionPolicyV1 = "safety-screen-restriction-v1"

// SafetyRestriction is an independently addressable Go decision with its own
// source, scope and lift audit. Its effect is derived (validity + lift), never
// a stored status. The only current writer is the structured-screen policy
// (SafetyScreenRestrictionPolicyV1), which records one positive, complete,
// user-authored screen concept as restricted/all_training/professional_clearance;
// it does not classify urgency. The wider shape below (advisory|urgent,
// plan|exercise scope, on_expiry|user_reverification lift) is for later
// reviewed Go policies; AI output can never write, lift or downgrade one.
type SafetyRestriction struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"user_id"`
	SafetyScreenID  uuid.UUID  `gorm:"type:uuid;not null" json:"safety_screen_id"`
	Concept         string     `gorm:"not null" json:"concept"`
	PolicyRevision  string     `gorm:"not null" json:"policy_revision"`
	RiskLevel       string     `gorm:"not null" json:"risk_level"`
	ScopeKind       string     `gorm:"not null" json:"scope_kind"`
	PlanID          *uuid.UUID `gorm:"type:uuid" json:"plan_id"`
	LiftPolicy      string     `gorm:"not null" json:"lift_policy"`
	LiftConditions  []string   `gorm:"serializer:json;type:jsonb;default:'[]'" json:"lift_conditions"`
	Reason          string     `gorm:"not null" json:"reason"`
	CreatedRevision int64      `gorm:"not null" json:"created_revision"`
	ValidFrom       time.Time  `gorm:"not null" json:"valid_from"`
	// ValidUntil ends validity only under on_expiry; otherwise it is a review point.
	ValidUntil *time.Time `json:"valid_until"`
	// ExerciseIDs are exercise-scope concept ids, stored in
	// safety_restriction_exercises and loaded with the restriction.
	ExerciseIDs []string `gorm:"-" json:"exercise_ids"`
}

func (SafetyRestriction) TableName() string { return "safety_restrictions" }

// SafetyRestrictionExercise is one immutable exercise-scope entry.
type SafetyRestrictionExercise struct {
	UserID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	RestrictionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExerciseID    string    `gorm:"primaryKey"`
}

func (SafetyRestrictionExercise) TableName() string { return "safety_restriction_exercises" }
