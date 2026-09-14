package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ConsultationPhase is the complete workflow vocabulary owned by Consultation.
// Diagnosis/Treatment readiness belongs to their own durable domains rather than
// extending this state machine with cross-domain aliases.
type ConsultationPhase string

const (
	ConsultationPhaseCollecting       ConsultationPhase = "collecting"
	ConsultationPhaseReadyForAnalysis ConsultationPhase = "ready_for_analysis"
)

func ParseConsultationPhase(value string) (ConsultationPhase, bool) {
	switch ConsultationPhase(value) {
	case ConsultationPhaseCollecting, ConsultationPhaseReadyForAnalysis:
		return ConsultationPhase(value), true
	default:
		return "", false
	}
}

// ConsultationSession represents a medical consultation tied 1:1 to a Conversation.
type ConsultationSession struct {
	ConversationID      uuid.UUID          `gorm:"type:uuid;primaryKey" json:"conversation_id"`
	Phase               ConsultationPhase  `gorm:"type:varchar(30);not null;default:'collecting'" json:"phase"`
	ExtractedInfo       datatypes.JSON     `gorm:"type:jsonb;not null;default:'[]'" json:"extracted_info"`
	PendingInteractions []AgentInteraction `gorm:"-" json:"pending_interactions,omitempty"`
	InteractionHistory  []AgentInteraction `gorm:"-" json:"interaction_history,omitempty"`
	CreatedAt           time.Time          `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time          `gorm:"not null;default:now()" json:"updated_at"`
	EndedAt             *time.Time         `json:"ended_at,omitempty"`
	Conversation        Conversation       `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
}

// TableName specifies the table name for GORM.
func (ConsultationSession) TableName() string {
	return "consultation_sessions"
}
