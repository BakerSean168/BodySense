package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ConsultationRolloutObservation is anonymous operational evidence for one
// paired Consultation shadow/canary comparison. It deliberately stores no
// user identity: evidence is aggregate-only.
type ConsultationRolloutObservation struct {
	ID                        uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	RunID                     uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_consultation_rollout_observation_pair" json:"run_id"`
	ConversationID            uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`
	Stage                     string         `gorm:"type:varchar(24);not null" json:"stage"`
	ChampionConfigurationID   string         `gorm:"type:varchar(80);not null" json:"champion_configuration_id"`
	ChallengerConfigurationID string         `gorm:"type:varchar(80);not null;uniqueIndex:uq_consultation_rollout_observation_pair" json:"challenger_configuration_id"`
	CanaryBPS                 int            `gorm:"not null;default:0" json:"canary_bps"`
	DecisionIdentityMatch     bool           `gorm:"not null;default:false" json:"decision_identity_match"`
	ReplayInputFrozen         bool           `gorm:"not null;default:false" json:"replay_input_frozen"`
	ShadowError               string         `gorm:"type:text" json:"shadow_error,omitempty"`
	Comparison                datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"comparison"`
	CreatedAt                 time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the table name for GORM.
func (ConsultationRolloutObservation) TableName() string {
	return "consultation_rollout_observations"
}
