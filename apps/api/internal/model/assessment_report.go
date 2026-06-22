package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AssessmentReport represents a health assessment report.
type AssessmentReport struct {
	ID                 uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID             uuid.UUID       `gorm:"type:uuid;index;not null" json:"user_id"`
	HealthGrade        string          `gorm:"type:varchar(5);not null" json:"health_grade"`
	DimensionScores    json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"dimension_scores"`
	IdentifiedIssues   json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"identified_issues"`
	ImprovementSummary json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"improvement_summary"`
	CreatedAt          time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the table name for GORM.
func (AssessmentReport) TableName() string {
	return "assessment_reports"
}
