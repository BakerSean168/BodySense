package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// BodyState is the user-scoped durable health aggregate introduced by ADR 0004.
//
// It intentionally owns only aggregate identity/current revision plus cross-cutting
// state such as safety. Facts, observations, hypotheses and immutable revisions live
// in their own tables so that current projection reads stay simple without forcing full
// event replay on every request.
type BodyState struct {
	UserID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"user_id"`
	CurrentRevision int64          `gorm:"not null;default:0" json:"current_revision"`
	SafetyState     datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"safety_state"`
	CreatedAt       time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"not null;default:now()" json:"updated_at"`

	// Projection-only fields populated by BodyStateRepository.GetCurrent.
	Facts        []BodyStateFact        `gorm:"-" json:"facts"`
	Observations []BodyStateObservation `gorm:"-" json:"observations"`
	Hypotheses   []BodyStateHypothesis  `gorm:"-" json:"hypotheses"`
}

func (BodyState) TableName() string { return "body_states" }

// BodyStateFact is structured information accepted as part of the user's BodyState.
// AI hypotheses are intentionally NOT stored here.
type BodyStateFact struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	ConcernKey            string         `gorm:"type:varchar(120);not null;default:''" json:"concern_key,omitempty"`
	Kind                  string         `gorm:"type:varchar(80);not null" json:"kind"`
	BodyRegion            string         `gorm:"type:varchar(120);not null;default:''" json:"body_region,omitempty"`
	BodyRegionID          *string        `gorm:"type:varchar(80)" json:"body_region_id"`
	Value                 string         `gorm:"type:text;not null;default:''" json:"value"`
	Details               datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"details"`
	Origin                string         `gorm:"type:varchar(40);not null" json:"origin"`
	ReviewState           string         `gorm:"type:varchar(40);not null;default:'unverified'" json:"review_state"`
	LifecycleState        string         `gorm:"type:varchar(30);not null;default:'active'" json:"lifecycle_state"`
	Trend                 string         `gorm:"type:varchar(30);not null;default:'unknown'" json:"trend"`
	SourceKey             string         `gorm:"type:text;not null;default:''" json:"source_key,omitempty"`
	Provenance            datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"provenance"`
	ObservedAt            *time.Time     `json:"observed_at,omitempty"`
	ValidFrom             *time.Time     `json:"valid_from,omitempty"`
	ValidUntil            *time.Time     `json:"valid_until,omitempty"`
	SupersedesFactID      *uuid.UUID     `gorm:"type:uuid" json:"supersedes_fact_id,omitempty"`
	ExcludedFromReasoning bool           `gorm:"not null;default:false" json:"excluded_from_reasoning"`
	CreatedRevision       int64          `gorm:"not null" json:"created_revision"`
	UpdatedRevision       int64          `gorm:"not null" json:"updated_revision"`
	CreatedAt             time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt             time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

func (BodyStateFact) TableName() string { return "body_state_facts" }

// BodyStateObservation keeps measured/self-test/posture-analysis information
// epistemically separate from user-reported facts.
type BodyStateObservation struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	ConcernKey            string         `gorm:"type:varchar(120);not null;default:''" json:"concern_key,omitempty"`
	Kind                  string         `gorm:"type:varchar(80);not null" json:"kind"`
	BodyRegion            string         `gorm:"type:varchar(120);not null;default:''" json:"body_region,omitempty"`
	BodyRegionID          *string        `gorm:"type:varchar(80)" json:"body_region_id"`
	Method                string         `gorm:"type:varchar(80);not null;default:''" json:"method,omitempty"`
	Value                 datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"value"`
	Condition             datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"condition"`
	SourceKey             string         `gorm:"type:text;not null;default:''" json:"source_key,omitempty"`
	Provenance            datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"provenance"`
	ObservedAt            *time.Time     `json:"observed_at,omitempty"`
	ReviewState           string         `gorm:"type:varchar(40);not null;default:'unverified'" json:"review_state"`
	LifecycleState        string         `gorm:"type:varchar(30);not null;default:'active'" json:"lifecycle_state"`
	ExcludedFromReasoning bool           `gorm:"not null;default:true" json:"excluded_from_reasoning"`
	CreatedRevision       int64          `gorm:"not null" json:"created_revision"`
	UpdatedRevision       int64          `gorm:"not null" json:"updated_revision"`
	CreatedAt             time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt             time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

func (BodyStateObservation) TableName() string { return "body_state_observations" }

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
