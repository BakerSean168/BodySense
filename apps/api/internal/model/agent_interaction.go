package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentInteraction represents a pending user interaction (e.g., ask_user).
type AgentInteraction struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	RunID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"run_id"`
	ConversationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`
	ToolCallID     string         `gorm:"type:text;not null;uniqueIndex:idx_interactions_run_tc" json:"tool_call_id"`
	ToolName       string         `gorm:"type:text;not null;default:'ask_user'" json:"tool_name"`
	Question       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"question"`
	Answer         datatypes.JSON `gorm:"type:jsonb" json:"answer,omitempty"`
	Status         string         `gorm:"type:varchar(30);not null;default:'pending';index" json:"status"`
	CreatedAt      time.Time      `gorm:"not null;default:now()" json:"created_at"`
	AnsweredAt     *time.Time     `json:"answered_at,omitempty"`
	ExpiresAt      *time.Time     `gorm:"index" json:"expires_at,omitempty"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
}

// TableName specifies the table name for GORM.
func (AgentInteraction) TableName() string {
	return "agent_interactions"
}
