package model

import (
	"time"

	"github.com/google/uuid"
)

// DiagnosisSettlement is the immutable whole-run outcome. A no-change or
// safety-blocked outcome has no DiagnosisRevision and must not look adopted.
type DiagnosisSettlement struct {
	UserID              uuid.UUID `gorm:"primaryKey"`
	ID                  uuid.UUID `gorm:"primaryKey"`
	RunID               uuid.UUID
	ExpectedRevision    int64
	Revision            int64
	DiagnosisRevisionID *uuid.UUID
	Outcome             string
	ConditionIDs        []uuid.UUID `gorm:"serializer:json;type:jsonb"`
	DecisionIDs         []uuid.UUID `gorm:"serializer:json;type:jsonb"`
	BodyStateRevision   int64
	IdempotencyKey      string
	RequestFingerprint  string
	ToolCallID          string
	ReceiptID           uuid.UUID
	SettledAt           time.Time
}

func (DiagnosisSettlement) TableName() string { return "diagnosis_settlements" }

// ConditionRelationRecord changes only through an explicit accepted relation
// proposal. Sharing symptoms or being in one run never creates a relation.
type ConditionRelationRecord struct {
	UserID            uuid.UUID              `gorm:"primaryKey"`
	FromConditionID   uuid.UUID              `gorm:"primaryKey"`
	ToConditionID     uuid.UUID              `gorm:"primaryKey"`
	Kind              ConditionRelationKind  `gorm:"primaryKey"`
	BasisEvidenceRefs []DiagnosisEvidenceRef `gorm:"serializer:json;type:jsonb"`
	Rationale         string
	LatestDecisionID  uuid.UUID
	CommittedRevision int64
	RecordedAt        time.Time
}

func (ConditionRelationRecord) TableName() string { return "condition_relations" }

type ConditionDispositionChange struct {
	UserID             uuid.UUID `gorm:"primaryKey"`
	ID                 uuid.UUID `gorm:"primaryKey"`
	ConditionID        uuid.UUID
	FromDisposition    ConditionDisposition
	ToDisposition      ConditionDisposition
	Reason             string
	RelatedConditionID *uuid.UUID
	EvidenceRefs       []DiagnosisEvidenceRef `gorm:"serializer:json;type:jsonb"`
	Details            string
	DecisionID         uuid.UUID
	CommittedRevision  int64
	ChangedAt          time.Time
}

func (ConditionDispositionChange) TableName() string { return "condition_disposition_changes" }
