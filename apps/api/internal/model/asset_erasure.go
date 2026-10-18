package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// AssetErasureInventoryRevision names the controlled-copy inventory a request
// was planned against. A later inventory adds targets under a new revision.
const AssetErasureInventoryRevision = "asset-erasure-inventory-v1"

// AssetErasureStatus is the request lifecycle. Erased is terminal and is
// only reachable once every inventoried target verified (enforced in SQL).
type AssetErasureStatus string

const (
	AssetErasurePending     AssetErasureStatus = "pending"
	AssetErasureRunning     AssetErasureStatus = "running"
	AssetErasureOutstanding AssetErasureStatus = "outstanding"
	AssetErasureErased      AssetErasureStatus = "erased"
)

// AssetErasureTargetName is one controlled-copy family of an asset.
type AssetErasureTargetName string

const (
	// AssetErasureSourceObjects is every tracked object-storage replica of the original.
	AssetErasureSourceObjects AssetErasureTargetName = "source_objects"
	// AssetErasureDerivedJobs is the OCR/posture jobs of the asset with their events
	// and AI output reviews (job results carry the full model output).
	AssetErasureDerivedJobs AssetErasureTargetName = "derived_jobs"
	// AssetErasurePostureAnalyses is the posture analysis runs of a photo asset.
	AssetErasurePostureAnalyses AssetErasureTargetName = "posture_analyses"
	// AssetErasureDocumentExtractions is the extraction runs and indicator reviews of a report.
	AssetErasureDocumentExtractions AssetErasureTargetName = "document_extractions"
	// AssetErasureAssessmentReports is assessment reports whose frozen input names the asset.
	AssetErasureAssessmentReports AssetErasureTargetName = "assessment_reports"
	// AssetErasureDiagnosisRecords is Diagnosis audit/projection rows naming the asset or its reviews.
	AssetErasureDiagnosisRecords AssetErasureTargetName = "diagnosis_records"
	// AssetErasureRuntimeCheckpoints is consultation runtime state (LangGraph
	// checkpoints) of conversations the asset was attached to as an image.
	AssetErasureRuntimeCheckpoints AssetErasureTargetName = "runtime_checkpoints"
	// AssetErasureManifest is the asset row itself, tombstoned last.
	AssetErasureManifest AssetErasureTargetName = "asset_manifest"
)

// AssetErasureTargets is the inventory in execution order. Diagnosis
// references are checked before document extractions so evidence a Diagnosis
// still cites is never deleted under it. The manifest is always last: it is
// finalized only after every other target verified.
var AssetErasureTargets = []AssetErasureTargetName{
	AssetErasureSourceObjects,
	AssetErasureDerivedJobs,
	AssetErasurePostureAnalyses,
	AssetErasureDiagnosisRecords,
	AssetErasureDocumentExtractions,
	AssetErasureAssessmentReports,
	AssetErasureRuntimeCheckpoints,
	AssetErasureManifest,
}

// AssetErasureTargetState is the per-target verification state.
type AssetErasureTargetState string

const (
	AssetErasureTargetPending AssetErasureTargetState = "pending"
	// AssetErasureTargetVerified means no copy of this family remains.
	AssetErasureTargetVerified AssetErasureTargetState = "verified"
	// AssetErasureTargetFailed is a transient failure the worker retries.
	AssetErasureTargetFailed AssetErasureTargetState = "failed"
	// AssetErasureTargetBlocked needs a decision or review first; never auto-mutated.
	AssetErasureTargetBlocked AssetErasureTargetState = "blocked"
	// AssetErasureTargetUnsupported has no erasure handler yet (a later slice).
	AssetErasureTargetUnsupported AssetErasureTargetState = "unsupported"
)

// Failure codes are safe, closed identifiers; no health content is stored.
const (
	AssetErasureCodeStorageUnavailable = "storage_unavailable"
	AssetErasureCodeObjectDeleteFailed = "object_delete_failed"
	AssetErasureCodeObjectStillPresent = "object_still_present"
	AssetErasureCodeDerivedWorkActive  = "derived_work_active"
	AssetErasureCodeDiagnosisPinned    = "diagnosis_evidence_pinned"
	AssetErasureCodeDiagnosisReview    = "diagnosis_review_required"
	AssetErasureCodeAssessmentFrozen   = "assessment_replay_frozen"
	AssetErasureCodeRuntimeHookPending = "runtime_erasure_hook_pending"
	AssetErasureCodeTargetsOutstanding = "targets_outstanding"
	AssetErasureCodeCopyReappeared     = "copy_reappeared"
	AssetErasureCodeExecutionError     = "execution_error"
)

var (
	ErrAssetErasureNotFound        = errors.New("asset erasure not found")
	ErrAssetErasureIdempotencyUsed = errors.New("asset erasure idempotency key was used for another asset")
)

// AssetErasureRequest is the durable erasure command and receipt header.
type AssetErasureRequest struct {
	ID                uuid.UUID          `gorm:"type:uuid;primaryKey"`
	UserID            uuid.UUID          `gorm:"type:uuid;not null"`
	AssetID           uuid.UUID          `gorm:"type:uuid;not null"`
	IdempotencyKey    string             `gorm:"not null"`
	InventoryRevision string             `gorm:"not null"`
	Status            AssetErasureStatus `gorm:"not null"`
	AttemptCount      int                `gorm:"not null"`
	LeaseOwner        *string
	LeaseExpiresAt    *time.Time
	RequestedAt       time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
	ErasedAt          *time.Time
}

func (AssetErasureRequest) TableName() string { return "asset_erasure_requests" }

// AssetErasureTarget is one inventoried target's verification record.
type AssetErasureTarget struct {
	UserID           uuid.UUID               `gorm:"type:uuid;primaryKey"`
	RequestID        uuid.UUID               `gorm:"type:uuid;primaryKey"`
	Target           AssetErasureTargetName  `gorm:"primaryKey"`
	State            AssetErasureTargetState `gorm:"not null"`
	ErasedCount      int                     `gorm:"not null"`
	OutstandingCount int                     `gorm:"not null"`
	FailureCode      *string
	AttemptCount     int `gorm:"not null"`
	VerifiedAt       *time.Time
	UpdatedAt        time.Time `gorm:"not null"`
}

func (AssetErasureTarget) TableName() string { return "asset_erasure_targets" }

// AssetErasureReceipt is the request with its targets in inventory order.
type AssetErasureReceipt struct {
	Request AssetErasureRequest
	Targets []AssetErasureTarget
}

// Outstanding lists the targets that have not verified yet.
func (r AssetErasureReceipt) Outstanding() []AssetErasureTarget {
	out := make([]AssetErasureTarget, 0, len(r.Targets))
	for _, target := range r.Targets {
		if target.State != AssetErasureTargetVerified {
			out = append(out, target)
		}
	}
	return out
}

// AssetErasureOutcome is what one execution pass learned about a target.
type AssetErasureOutcome struct {
	State       AssetErasureTargetState
	Erased      int
	Outstanding int
	FailureCode string
}
