package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Recurrence is an EXACT USER-REPORTED relation, not a clinical diagnosis,
// symptom similarity guess or reactivation of the original Episode.
type EpisodeRecurrenceCommand struct {
	IdempotencyKey   string    `json:"idempotency_key"`
	ExpectedRevision int64     `json:"expected_revision"`
	NewEpisodeID     uuid.UUID `json:"new_episode_id"`
	FactID           uuid.UUID `json:"fact_id"`
}

func (c EpisodeRecurrenceCommand) Validate(old uuid.UUID) error {
	if old == uuid.Nil || c.NewEpisodeID == uuid.Nil || c.NewEpisodeID == old ||
		c.FactID == uuid.Nil || c.ExpectedRevision < 0 ||
		len(c.IdempotencyKey) < 1 || len(c.IdempotencyKey) > 128 ||
		strings.TrimSpace(c.IdempotencyKey) != c.IdempotencyKey {
		return fmt.Errorf("%w: exact distinct Episode IDs, new Episode source Fact, revision and command key required", ErrInvalidEpisodeEvidence)
	}
	return nil
}

// EpisodeRelation is durable organizational lineage only. The two Episode
// identities, their status evidence and underlying Facts are never duplicated.
// authorization_id binds this relation to one committed direct-user receipt.
type EpisodeRelation struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID          uuid.UUID `gorm:"type:uuid;not null"`
	FromEpisodeID   uuid.UUID `gorm:"type:uuid;not null"`
	ToEpisodeID     uuid.UUID `gorm:"type:uuid;not null"`
	Kind            string
	BasisFactID     uuid.UUID `gorm:"type:uuid;not null"`
	BasisEvidenceID uuid.UUID `gorm:"type:uuid;not null"`
	AuthorizationID uuid.UUID `gorm:"type:uuid;not null"`
	CreatedRevision int64
	CreatedAt       time.Time
}

func (EpisodeRelation) TableName() string { return "episode_relations" }

// The exact receipt proves durable authorization of this one relation,
// including the old and new IDs, not merely an unrelated Episode receipt.
type EpisodeRecurrenceReceipt struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	RelationID    uuid.UUID `json:"relation_id"`
	FromEpisodeID uuid.UUID `json:"from_episode_id"`
	ToEpisodeID   uuid.UUID `json:"to_episode_id"`
	FactID        uuid.UUID `json:"fact_id"`
	Revision      int64     `json:"revision"`
	RevisionID    uuid.UUID `json:"revision_id"`
	EventID       uuid.UUID `json:"event_id"`
	CommittedAt   time.Time `json:"committed_at"`
}

// Read one owner-scoped, whole-revision snapshot. Supported means a currently
// available linked source AND an old Episode still explicitly ended.
// needs_review means that old end evidence was corrected/withdrawn; it NEVER
// asserts a current confirmed recurrence or a shared medical etiology.
type EpisodeRelationReference struct {
	ID              uuid.UUID `json:"id"`
	FromEpisodeID   uuid.UUID `json:"from_episode_id"`
	ToEpisodeID     uuid.UUID `json:"to_episode_id"`
	Kind            string    `json:"kind"`
	BasisFactID     uuid.UUID `json:"basis_fact_id"`
	AuthorizationID uuid.UUID `json:"authorization_id"`
	CreatedRevision int64     `json:"created_revision"`
	ReviewState     string    `json:"review_state"` // supported | needs_review
}

type EpisodeRelationPage struct {
	UserID             uuid.UUID                  `json:"user_id"`
	EpisodeID          uuid.UUID                  `json:"episode_id"`
	CurrentRevision    int64                      `json:"current_revision"`
	Relations          []EpisodeRelationReference `json:"relations"`
	HasMore            bool                       `json:"has_more"`
	NextBeforeRevision *int64                     `json:"next_before_revision,omitempty"`
}
