package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	DiagnosisFreshnessFresh            = "fresh"
	DiagnosisFreshnessPotentiallyStale = "potentially_stale"
	DiagnosisFreshnessStale            = "stale"
)

// DiagnosisAnalysisFreshness is mutable read-state kept outside the immutable
// DiagnosisAnalysis artifact. It records the policy result against a later BodyState revision.
type DiagnosisAnalysisFreshness struct {
	AnalysisID               uuid.UUID      `gorm:"type:uuid;primaryKey" json:"analysis_id"`
	UserID                   uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	State                    string         `gorm:"type:varchar(30);not null;default:'fresh'" json:"state"`
	EvaluatedAgainstRevision int64          `gorm:"not null" json:"evaluated_against_revision"`
	Reasons                  datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"reasons"`
	CheckedAt                time.Time      `gorm:"not null;default:now()" json:"checked_at"`
}

func (DiagnosisAnalysisFreshness) TableName() string { return "diagnosis_analysis_freshness" }
