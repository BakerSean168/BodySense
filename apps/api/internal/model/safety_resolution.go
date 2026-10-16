package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidSafetyResolution             = errors.New("invalid safety resolution")
	ErrSafetyRestrictionNotFound           = errors.New("safety restriction not found")
	ErrSafetyRestrictionAlreadyResolved    = errors.New("safety restriction already resolved")
	ErrSafetyResolutionSourceNotFound      = errors.New("safety resolution source not found")
	ErrSafetyResolutionIdempotencyConflict = errors.New("safety resolution idempotency conflict")
)

// ProfessionalClearanceStatement is the explicit confirmation supported by the
// first user-reported clearance flow. It must occur verbatim in the user's own
// message; arbitrary positive prose and model interpretations are not clearance.
const ProfessionalClearanceStatement = "专业人员已针对这条限制及全部训练放行"

const SafetyResolutionPolicyV1 = "user-reported-professional-clearance-v1"

type ResolveSafetyRestrictionCommand struct {
	IdempotencyKey   string    `json:"idempotency_key"`
	ExpectedRevision int64     `json:"expected_revision"`
	SourceMessageID  uuid.UUID `json:"source_message_id"`
	Statement        string    `json:"statement"`
}

func (c ResolveSafetyRestrictionCommand) Validate() error {
	if len(c.IdempotencyKey) < 1 || len(c.IdempotencyKey) > 128 || c.IdempotencyKey != strings.TrimSpace(c.IdempotencyKey) || c.ExpectedRevision < 0 || c.SourceMessageID == uuid.Nil || c.Statement != ProfessionalClearanceStatement {
		return fmt.Errorf("%w: exact professional clearance confirmation, message, key and revision required", ErrInvalidSafetyResolution)
	}
	return nil
}

// SafetyResolution is both the immutable per-restriction lift audit and the
// idempotent receipt. Original text lives only in owner-bound StatementEvidence.
// User-reported professional clearance never claims clinical-document admission.
type SafetyResolution struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	RestrictionID    uuid.UUID `gorm:"type:uuid;not null" json:"restriction_id"`
	SourceEvidenceID uuid.UUID `gorm:"type:uuid;not null" json:"source_evidence_id"`
	SourceMessageID  uuid.UUID `gorm:"type:uuid;not null" json:"source_message_id"`
	SourceVersion    int64     `gorm:"not null" json:"source_version"`
	BasisKind        string    `gorm:"not null" json:"basis_kind"`
	SourceLevel      string    `gorm:"not null" json:"source_level"`
	ScopeKind        string    `gorm:"not null" json:"scope_kind"`
	PolicyRevision   string    `gorm:"not null" json:"policy_revision"`
	IdempotencyKey   string    `gorm:"not null" json:"-"`
	Fingerprint      string    `gorm:"not null" json:"-"`
	Revision         int64     `gorm:"not null" json:"revision"`
	RevisionID       uuid.UUID `gorm:"type:uuid;not null" json:"revision_id"`
	EventID          uuid.UUID `gorm:"type:uuid;not null" json:"event_id"`
	Actor            string    `gorm:"not null" json:"actor"`
	LiftedAt         time.Time `gorm:"not null" json:"lifted_at"`
}

func (SafetyResolution) TableName() string { return "safety_resolutions" }

// SafetyRestrictionRecord is a read projection. Source invalidation makes an
// earlier resolution ineffective without erasing its immutable audit.
type SafetyRestrictionRecord struct {
	SafetyRestriction
	Resolution         *SafetyResolution `json:"resolution"`
	ClearanceAvailable bool              `json:"clearance_available"`
	Active             bool              `json:"active"`
}

type SafetyRestrictionList struct {
	UserID          uuid.UUID                 `json:"user_id"`
	CurrentRevision int64                     `json:"current_revision"`
	Restrictions    []SafetyRestrictionRecord `json:"restrictions"`
}
