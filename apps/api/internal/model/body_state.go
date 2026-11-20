package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ErrBodyStateRevisionConflict is returned when a caller tries to mutate a
// BodyState from an older revision. This is the optimistic-concurrency guard
// that prevents two devices/runs from silently overwriting each other.
var ErrBodyStateRevisionConflict = errors.New("body state revision conflict")

// BodyState is the user-scoped durable health aggregate introduced by ADR 0004.
//
// It intentionally owns only aggregate identity/current revision plus cross-cutting
// state. Native facts, observations and immutable revisions live
// in their own tables so that current projection reads stay simple without forcing full
// event replay on every request.
type BodyState struct {
	UserID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	CurrentRevision int64     `gorm:"not null;default:0" json:"current_revision"`
	CreatedAt       time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (BodyState) TableName() string { return "body_states" }

// BodyStateRevision is an immutable semantic change record. Current reads do not
// replay this table; it exists for provenance, temporal reasoning and exact pinning.
type BodyStateRevision struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Revision   int64          `gorm:"not null" json:"revision"`
	ChangeType string         `gorm:"type:varchar(80);not null" json:"change_type"`
	Source     string         `gorm:"type:varchar(60);not null" json:"source"`
	Changes    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"changes"`
	CreatedAt  time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (BodyStateRevision) TableName() string { return "body_state_revisions" }
