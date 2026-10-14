package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"time"
)

// EpisodeAssociationCommand is an explicit authenticated user action on
// one already-committed symptom Fact. It does not accept fabricated source,
// condition, episode title, onset/end or medical state. The endpoint/target
// decides whether to create a new identity or link an existing one.
type EpisodeAssociationCommand struct {
	IdempotencyKey   string    `json:"idempotency_key"`
	ExpectedRevision int64     `json:"expected_revision"`
	FactID           uuid.UUID `json:"fact_id"`
}

var (
	ErrEpisodeIdempotencyConflict = errors.New("episode command idempotency conflict")
	ErrEpisodeFactUnavailable     = errors.New("episode fact is unavailable for this owner")
	ErrEpisodeAssociationConflict = errors.New("episode association conflicts with accepted identity")
)

func (c EpisodeAssociationCommand) Validate() error {
	if c.FactID == uuid.Nil || c.ExpectedRevision < 0 || len(c.IdempotencyKey) < 1 ||
		len(c.IdempotencyKey) > 128 || strings.TrimSpace(c.IdempotencyKey) != c.IdempotencyKey {
		return fmt.Errorf("%w: exact symptom Fact, command identity and nonnegative revision are required", ErrInvalidEpisodeEvidence)
	}
	return nil
}

// EpisodeCommandReceipt has the same integrity role as BodyFactCommit, but
// references exactly one Episode association. It contains IDs only, never a
// second copy of the source statement or symptom clinical history.
type EpisodeCommandReceipt struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	IdempotencyKey string    `gorm:"not null" json:"-"`
	Fingerprint    string    `gorm:"not null" json:"-"`
	Operation      string    `gorm:"not null" json:"operation"`
	EpisodeID      uuid.UUID `gorm:"type:uuid;not null" json:"episode_id"`
	FactID         uuid.UUID `gorm:"type:uuid;not null" json:"fact_id"`
	Revision       int64     `gorm:"not null" json:"revision"`
	RevisionID     uuid.UUID `gorm:"type:uuid;not null" json:"revision_id"`
	EventID        uuid.UUID `gorm:"type:uuid;not null" json:"event_id"`
	CommittedAt    time.Time `gorm:"not null" json:"committed_at"`
}

func (EpisodeCommandReceipt) TableName() string { return "episode_command_receipts" }
