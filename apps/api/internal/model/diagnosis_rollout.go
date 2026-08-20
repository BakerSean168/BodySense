package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DiagnosisRolloutObservation is anonymous operational evidence for one paired
// shadow/canary comparison. User identity is deliberately not stored here.
type DiagnosisRolloutObservation struct {
	ID                        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	SourceAnalysisID          *uuid.UUID     `gorm:"type:uuid;not null;index" json:"source_analysis_id"`
	Stage                     string         `gorm:"type:varchar(24);not null;index" json:"stage"`
	SubjectBucket             int            `gorm:"not null" json:"subject_bucket"`
	CanaryBPS                 int            `gorm:"not null;default:0" json:"canary_bps"`
	ChampionConfigurationID   string         `gorm:"type:varchar(80);not null;index" json:"champion_configuration_id"`
	ChallengerConfigurationID string         `gorm:"type:varchar(80);not null;index" json:"challenger_configuration_id"`
	ServedConfigurationID     string         `gorm:"type:varchar(80);not null" json:"served_configuration_id"`
	ShadowConfigurationID     string         `gorm:"type:varchar(80);not null;default:''" json:"shadow_configuration_id"`
	Comparison                datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"comparison"`
	UnsafeRelaxation          bool           `gorm:"not null;default:false" json:"unsafe_relaxation"`
	ForbiddenSideEffect       bool           `gorm:"not null;default:false" json:"forbidden_side_effect"`
	ConfigurationMismatch     bool           `gorm:"not null;default:false" json:"configuration_mismatch"`
	ShadowError               string         `gorm:"type:text;not null;default:''" json:"shadow_error"`
	CreatedAt                 time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (DiagnosisRolloutObservation) TableName() string { return "diagnosis_rollout_observations" }
