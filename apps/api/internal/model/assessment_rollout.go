package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AssessmentRolloutObservation is anonymous operational evidence for one paired
// Assessment shadow/canary comparison. User identity is deliberately not stored.
type AssessmentRolloutObservation struct {
	ID                        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	SourceReportID            uuid.UUID      `gorm:"type:uuid;not null;index" json:"source_report_id"`
	Stage                     string         `gorm:"type:varchar(24);not null;index" json:"stage"`
	SubjectBucket             int            `gorm:"not null" json:"subject_bucket"`
	CanaryBPS                 int            `gorm:"not null;default:0" json:"canary_bps"`
	ChampionConfigurationID   string         `gorm:"type:varchar(80);not null;index" json:"champion_configuration_id"`
	ChallengerConfigurationID string         `gorm:"type:varchar(80);not null;index" json:"challenger_configuration_id"`
	ServedConfigurationID     string         `gorm:"type:varchar(80);not null" json:"served_configuration_id"`
	ShadowConfigurationID     string         `gorm:"type:varchar(80);not null;default:''" json:"shadow_configuration_id"`
	PromotionRecord           string         `gorm:"type:varchar(80);not null;default:''" json:"promotion_record"`
	Comparison                datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"comparison"`
	ForbiddenSideEffect       bool           `gorm:"not null;default:false" json:"forbidden_side_effect"`
	ConfigurationMismatch     bool           `gorm:"not null;default:false" json:"configuration_mismatch"`
	ShadowError               string         `gorm:"type:text;not null;default:''" json:"shadow_error"`
	CreatedAt                 time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (AssessmentRolloutObservation) TableName() string { return "assessment_rollout_observations" }
