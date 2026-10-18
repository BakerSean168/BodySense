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
	// checkpoints, writes and blobs) of the owner's conversations: the inline
	// image of a turn that attached the asset and any derived copy naming it.
	// The ai-service erases and verifies it (POST /runtime/assets/{id}/erasure).
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
	// ErasureTargetRetainedByPolicy is terminal and honest: the copies stay by
	// a documented policy (frozen replay inputs) and its retention reason says
	// which. Only the statement inventory's decision_records target uses it.
	ErasureTargetRetainedByPolicy AssetErasureTargetState = "retained_by_policy"
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
	// ErasureCodePlanReview: an active or paused Training plan's current
	// revision rests on the source; the plan needs review first.
	ErasureCodePlanReview = "training_plan_review_required"
	// ErasureCodeBodyRecordActive: a BodyState record derived from the
	// statement is still asserted; withdraw it first.
	ErasureCodeBodyRecordActive = "body_record_active"
	// AssetErasureCodeRuntimeThreadBusy: a conversation is mid-turn or waiting
	// on an Ask User answer; its runtime state is left untouched and retried.
	AssetErasureCodeRuntimeThreadBusy = "runtime_thread_busy"
	// AssetErasureCodeRuntimeUnavailable: no runtime receipt was obtained.
	AssetErasureCodeRuntimeUnavailable = "runtime_unavailable"
	// AssetErasureCodeRuntimeUnverified: the runtime receipt did not prove
	// every conversation clean (conflict, survivor, or a receipt mismatch).
	AssetErasureCodeRuntimeUnverified  = "runtime_erasure_unverified"
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

// AssetErasureReceipt is the request with its targets in inventory order and
// the live review signal of what depends on the asset.
type AssetErasureReceipt struct {
	Request AssetErasureRequest
	Targets []AssetErasureTarget
	Reviews []ErasureReview
}

// ErasureReviewSubject names what needs the user's review because a source
// being erased still supports it.
type ErasureReviewSubject string

const (
	// ErasureReviewDiagnosisCondition is an adopted condition entry citing the source.
	ErasureReviewDiagnosisCondition ErasureReviewSubject = "diagnosis_condition"
	// ErasureReviewTrainingPlan is an active or paused plan whose current
	// revision rests on such a condition or on a record derived from the source.
	ErasureReviewTrainingPlan ErasureReviewSubject = "training_plan"
	// ErasureReviewBodyFact, ErasureReviewBodyObservation and
	// ErasureReviewBodyHypothesis are BodyState records derived from an erased
	// statement that are still asserted: withdrawing them releases the erasure.
	ErasureReviewBodyFact        ErasureReviewSubject = "body_fact"
	ErasureReviewBodyObservation ErasureReviewSubject = "body_observation"
	ErasureReviewBodyHypothesis  ErasureReviewSubject = "body_hypothesis"
)

// ErasureReview is the review signal of an erasure. It is computed live and
// never changes the Diagnosis or plan: it clears once the user revises the
// Diagnosis or withdraws the condition (or revises the plan), which is also
// what releases the pinned copies for erasure.
type ErasureReview struct {
	Subject    ErasureReviewSubject
	SubjectID  uuid.UUID
	RevisionID *uuid.UUID
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
// RetentionReason is set only with ErasureTargetRetainedByPolicy.
type AssetErasureOutcome struct {
	State           AssetErasureTargetState
	Erased          int
	Outstanding     int
	FailureCode     string
	RetentionReason string
}

// AssetRuntimeScope is what the runtime target asks the ai-service to erase:
// every consultation thread (= Conversation) of the owner, the subset a
// message attached the asset to, and the content hash that recognises the
// asset's inline image copies.
type AssetRuntimeScope struct {
	ThreadIDs         []uuid.UUID
	AttachedThreadIDs []uuid.UUID
	ContentSHA256     string
}

// RuntimeErasureReceipt is the ai-service answer for one erased source (an
// asset or a statement), read back from checkpoint storage.
type RuntimeErasureReceipt struct {
	Status    string                 `json:"status"`
	SubjectID string                 `json:"subject_id"`
	Threads   []RuntimeThreadReceipt `json:"threads"`
}

// RuntimeThreadReceipt is one thread's result: absent (never held the
// source), erased (rewritten, compacted and verified) or outstanding.
type RuntimeThreadReceipt struct {
	ThreadID           string  `json:"thread_id"`
	Status             string  `json:"status"`
	CheckpointID       *string `json:"checkpoint_id"`
	RedactedRefs       int     `json:"redacted_refs"`
	DeletedCheckpoints int     `json:"deleted_checkpoints"`
	FailureCode        *string `json:"failure_code"`
}

// Outcome turns a runtime receipt into the target outcome. It verifies only
// a receipt for this asset that reports every requested thread absent or
// erased; anything else stays outstanding and is retried.
func (s AssetRuntimeScope) Outcome(assetID uuid.UUID, receipt *RuntimeErasureReceipt) AssetErasureOutcome {
	return RuntimeOutcome(s.ThreadIDs, assetID, receipt)
}

// RuntimeOutcome verifies a runtime receipt for one subject: it must name
// the subject and report every requested thread absent or erased.
func RuntimeOutcome(threadIDs []uuid.UUID, subjectID uuid.UUID, receipt *RuntimeErasureReceipt) AssetErasureOutcome {
	unverified := AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: max(len(threadIDs), 1), FailureCode: AssetErasureCodeRuntimeUnverified}
	if receipt == nil || receipt.SubjectID != subjectID.String() {
		return unverified
	}
	byThread := make(map[string]RuntimeThreadReceipt, len(receipt.Threads))
	for _, thread := range receipt.Threads {
		byThread[thread.ThreadID] = thread
	}
	erased, outstanding, busy := 0, 0, 0
	for _, id := range threadIDs {
		thread, ok := byThread[id.String()]
		switch {
		case !ok:
			outstanding++
		case thread.Status == "absent":
		case thread.Status == "erased":
			erased += thread.RedactedRefs + thread.DeletedCheckpoints
		default:
			outstanding++
			if thread.FailureCode != nil && *thread.FailureCode == "thread_busy" {
				busy++
			}
		}
	}
	switch {
	case outstanding == 0 && receipt.Status == "erased":
		return AssetErasureOutcome{State: AssetErasureTargetVerified, Erased: erased}
	case outstanding == 0:
		return unverified
	case busy == outstanding:
		return AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: outstanding, FailureCode: AssetErasureCodeRuntimeThreadBusy}
	default:
		return AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: outstanding, FailureCode: AssetErasureCodeRuntimeUnverified}
	}
}
