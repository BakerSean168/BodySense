package model

import (
	"time"

	"github.com/google/uuid"
)

type ConditionDirectEditKind string

const (
	ConditionCorrectMetadata ConditionDirectEditKind = "correct_metadata"
	ConditionWithdraw        ConditionDirectEditKind = "withdraw"
)

// ConditionDirectEditChange records one exact field change. Before/after are
// server-read and server-applied values, never a client-supplied "before".
type ConditionDirectEditChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// ConditionDirectEdit is the immutable receipt of a user's confirmed direct
// withdrawal or display-metadata correction of one ConditionEntry. It is the
// only Diagnosis write that is not a formal-run settlement, and it can never
// change medical identity, assessment, sources, evidence or handling.
type ConditionDirectEdit struct {
	UserID                     uuid.UUID `gorm:"primaryKey"`
	ID                         uuid.UUID `gorm:"primaryKey"`
	ConditionID                uuid.UUID
	EditKind                   ConditionDirectEditKind
	Changes                    []ConditionDirectEditChange `gorm:"serializer:json;type:jsonb"`
	Reason                     string
	RequestMessageID           uuid.UUID
	RequestEvidenceID          uuid.UUID
	RequestSourceVersion       int64
	ConfirmationToolCallID     string
	BasedOnDiagnosisRevision   int64
	ResultingDiagnosisRevision int64
	DiagnosisRevisionID        uuid.UUID
	BodyStateRevision          int64
	IdempotencyKey             string
	RequestFingerprint         string
	ReceiptID                  uuid.UUID
	ConfirmedAt                time.Time
}

func (ConditionDirectEdit) TableName() string { return "condition_direct_edits" }
