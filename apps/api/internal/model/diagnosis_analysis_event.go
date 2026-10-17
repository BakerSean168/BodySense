package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var ErrDiagnosisDecisionConflict = errors.New("diagnosis decision conflicts with the presented proposal or revision")
var ErrDiagnosisAnalysisClosed = errors.New("diagnosis analysis is closed")

// ConditionAnalysisEvent is immutable ordered audit, not a mutable diagnosis.
// Decisions acknowledge a selection; only a later DiagnosisRevision adopts it.
type ConditionAnalysisEvent struct {
	UserID            uuid.UUID `gorm:"primaryKey"`
	ID                uuid.UUID `gorm:"primaryKey"`
	RunID             uuid.UUID
	Sequence          int64
	Kind              string
	ProposalID        *uuid.UUID
	DecisionID        *uuid.UUID
	BodyStateRevision int64
	Details           datatypes.JSON
	RecordedAt        time.Time
}

func (ConditionAnalysisEvent) TableName() string { return "condition_analysis_events" }
