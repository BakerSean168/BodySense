package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// StatementErasureInventoryRevision names the controlled-copy inventory a
// statement erasure was planned against.
const StatementErasureInventoryRevision = "statement-erasure-inventory-v1"

// StatementErasureStatus is the request lifecycle. Erased and
// ErasedWithRetention are terminal and are only reachable once the statement
// itself is erased and every target is verified (enforced in SQL).
type StatementErasureStatus string

const (
	StatementErasurePending     StatementErasureStatus = "pending"
	StatementErasureRunning     StatementErasureStatus = "running"
	StatementErasureOutstanding StatementErasureStatus = "outstanding"
	// StatementErasureErased: every target verified; no copy remains.
	StatementErasureErased StatementErasureStatus = "erased"
	// StatementErasureErasedWithRetention: erased except the frozen replay
	// inputs decision_records keeps by policy. It never claims a full erasure;
	// account erasure removes those inputs too.
	StatementErasureErasedWithRetention StatementErasureStatus = "erased_with_retention"
)

// Terminal reports whether the request will never run again.
func (s StatementErasureStatus) Terminal() bool {
	return s == StatementErasureErased || s == StatementErasureErasedWithRetention
}

// StatementErasureTargetName is one controlled-copy family of a statement.
type StatementErasureTargetName string

const (
	// StatementErasureDiagnosisRecords is the adopted Diagnosis conditions that
	// cite the statement or a record derived from it (they pin the erasure).
	StatementErasureDiagnosisRecords StatementErasureTargetName = "diagnosis_records"
	// StatementErasureTrainingPlans is active or paused plans whose current
	// revision rests on a derived record or a pinning condition.
	StatementErasureTrainingPlans StatementErasureTargetName = "training_plans"
	// StatementErasureBodyRecords is the facts, observations and hypotheses
	// derived from the statement: withdrawn first, then their text is cleared.
	StatementErasureBodyRecords StatementErasureTargetName = "body_records"
	// StatementErasureAssessmentReports is assessment reports naming the
	// statement or a derived record (deleted whole, never rewritten).
	StatementErasureAssessmentReports StatementErasureTargetName = "assessment_reports"
	// StatementErasureDecisionRecords is the immutable Diagnosis and plan
	// decision trail (analysis runs and events, proposals, decisions,
	// revisions, withdrawn conditions, plan proposals and accepted revisions)
	// whose frozen replay inputs name the statement or a derived record.
	StatementErasureDecisionRecords StatementErasureTargetName = "decision_records"
	// StatementErasureRuntimeCheckpoints is consultation runtime state naming
	// the statement or a derived record (POST /runtime/statements/{id}/erasure).
	StatementErasureRuntimeCheckpoints StatementErasureTargetName = "runtime_checkpoints"
	// StatementErasureManifest is the statement row itself, erased last.
	StatementErasureManifest StatementErasureTargetName = "statement_manifest"
)

// StatementErasureTargets is the inventory in execution order. The pins
// (Diagnosis, plans) are checked before the body records whose text they
// protect; the manifest is always last.
var StatementErasureTargets = []StatementErasureTargetName{
	StatementErasureDiagnosisRecords,
	StatementErasureTrainingPlans,
	StatementErasureBodyRecords,
	StatementErasureAssessmentReports,
	StatementErasureDecisionRecords,
	StatementErasureRuntimeCheckpoints,
	StatementErasureManifest,
}

// ErasureRetentionReplayInputFrozen is the retention reason of decision_records:
// frozen replay inputs are never rewritten, so a decision can always be
// replayed exactly as it was made. Account erasure removes them.
const ErasureRetentionReplayInputFrozen = "replay_input_frozen"

var (
	ErrStatementErasureNotFound        = errors.New("statement erasure not found")
	ErrStatementErasureIdempotencyUsed = errors.New("statement erasure idempotency key was used for another statement")
)

// StatementErasureRequest is the durable erasure command and receipt header.
type StatementErasureRequest struct {
	ID                uuid.UUID              `gorm:"type:uuid;primaryKey"`
	UserID            uuid.UUID              `gorm:"type:uuid;not null"`
	StatementID       uuid.UUID              `gorm:"type:uuid;not null"`
	IdempotencyKey    string                 `gorm:"not null"`
	InventoryRevision string                 `gorm:"not null"`
	Status            StatementErasureStatus `gorm:"not null"`
	AttemptCount      int                    `gorm:"not null"`
	LeaseOwner        *string
	LeaseExpiresAt    *time.Time
	RequestedAt       time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
	ErasedAt          *time.Time
}

func (StatementErasureRequest) TableName() string { return "statement_erasure_requests" }

// StatementErasureTarget is one inventoried target's verification record.
type StatementErasureTarget struct {
	UserID           uuid.UUID                  `gorm:"type:uuid;primaryKey"`
	RequestID        uuid.UUID                  `gorm:"type:uuid;primaryKey"`
	Target           StatementErasureTargetName `gorm:"primaryKey"`
	State            AssetErasureTargetState    `gorm:"not null"`
	ErasedCount      int                        `gorm:"not null"`
	OutstandingCount int                        `gorm:"not null"`
	FailureCode      *string
	RetentionReason  *string
	AttemptCount     int `gorm:"not null"`
	VerifiedAt       *time.Time
	UpdatedAt        time.Time `gorm:"not null"`
}

func (StatementErasureTarget) TableName() string { return "statement_erasure_targets" }

// StatementErasureReceipt is the request with its targets in inventory order
// and the live review signal of what still rests on the statement.
type StatementErasureReceipt struct {
	Request StatementErasureRequest
	Targets []StatementErasureTarget
	Reviews []ErasureReview
}

// StatementRuntimeScope is what the runtime target asks the ai-service to
// erase: every consultation thread of the owner, the one the statement was
// said in, and the ids of the BodyState records derived from it.
type StatementRuntimeScope struct {
	ThreadIDs         []uuid.UUID
	AttachedThreadIDs []uuid.UUID
	RecordIDs         []uuid.UUID
}
