package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentToolCall represents an audited tool call within a run.
type AgentToolCall struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	RunID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"run_id"`
	ConversationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`
	MessageID      *uuid.UUID     `gorm:"type:uuid" json:"message_id,omitempty"`
	ToolCallID     string         `gorm:"type:text;not null;uniqueIndex:idx_tool_calls_run_tc" json:"tool_call_id"`
	ToolName       string         `gorm:"type:text;not null" json:"tool_name"`
	Arguments      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"arguments"`
	Status         string         `gorm:"type:varchar(30);not null;default:'running'" json:"status"`
	Result         datatypes.JSON `gorm:"type:jsonb" json:"result,omitempty"`
	Error          datatypes.JSON `gorm:"type:jsonb" json:"error,omitempty"`
	CreatedAt      time.Time      `gorm:"not null;default:now()" json:"created_at"`
	StartedAt      time.Time      `gorm:"not null;default:now()" json:"started_at"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
}

// TableName specifies the table name for GORM.
func (AgentToolCall) TableName() string {
	return "agent_tool_calls"
}
