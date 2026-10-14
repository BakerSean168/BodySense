package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// EpisodeFactReference is a link to an available and currently asserted
// source-backed symptom statement. Corrected, withdrawn, blocked or erased
// sources must never leak through this presentation projection.
type EpisodeFactReference struct {
	FactID            uuid.UUID `json:"fact_id"`
	Role              string    `json:"role"`
	AssociationMethod string    `json:"association_method"`
	IsPrimary         bool      `json:"is_primary"`
	LinkedRevision    int64     `json:"linked_revision"`
}

// EpisodeRecord is a read model, not a separately persisted body-health truth.
// Identity is stable; the current projection is recomputed at the same DB
// snapshot as its links and all underlying source-availability rows.
type EpisodeRecord struct {
	ID              uuid.UUID              `json:"id"`
	UserID          uuid.UUID              `json:"user_id"`
	SymptomType     string                 `json:"symptom_type"`
	BodyRegionIDs   datatypes.JSON         `json:"body_region_ids"`
	Laterality      string                 `json:"laterality"`
	IdentityStatus  string                 `json:"identity_status"`
	CreatedRevision int64                  `json:"created_revision"`
	CreatedAt       time.Time              `json:"created_at"`
	Links           []EpisodeFactReference `json:"links"`
	Projection      EpisodeStateProjection `json:"projection"`
}

// EpisodePage uses whole-revision paging so no semantic creation batch is
// silently split. A caller must refetch if the returned current_revision
// changes between separate pages.
type EpisodePage struct {
	UserID             uuid.UUID       `json:"user_id"`
	CurrentRevision    int64           `json:"current_revision"`
	Episodes           []EpisodeRecord `json:"episodes"`
	HasMore            bool            `json:"has_more"`
	NextBeforeRevision *int64          `json:"next_before_revision,omitempty"`
}
