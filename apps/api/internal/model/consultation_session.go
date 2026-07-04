package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ConsultationSession represents a medical consultation tied 1:1 to a Conversation.
type ConsultationSession struct {
	ConversationID uuid.UUID      `gorm:"type:uuid;primaryKey" json:"conversation_id"`
	Phase          string         `gorm:"type:varchar(30);not null;default:'collecting'" json:"phase"`
	ExtractedInfo  datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"extracted_info"`
	Diagnosis      datatypes.JSON `gorm:"type:jsonb" json:"diagnosis,omitempty"`
	TreatmentPlan  datatypes.JSON `gorm:"type:jsonb" json:"treatment_plan,omitempty"`
	PendingInteractions []AgentInteraction `gorm:"-" json:"pending_interactions,omitempty"`
	CreatedAt      time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"not null;default:now()" json:"updated_at"`
	EndedAt        *time.Time     `json:"ended_at,omitempty"`
	Conversation   Conversation   `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
}

// TableName specifies the table name for GORM.
func (ConsultationSession) TableName() string {
	return "consultation_sessions"
}
