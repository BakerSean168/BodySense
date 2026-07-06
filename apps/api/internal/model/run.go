package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Run represents a single LLM inference run within a conversation.
type Run struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	ConversationID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`
	TurnID             uuid.UUID      `gorm:"type:uuid;not null" json:"turn_id"`
	RequestID          string         `gorm:"type:text;not null;uniqueIndex:idx_runs_user_request" json:"request_id"`
	UserID             uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_runs_user_request" json:"user_id"`
	Status             string         `gorm:"type:varchar(20);not null;default:'running'" json:"status"`
	Model              string         `gorm:"type:text;not null" json:"model"`
	Provider           string         `gorm:"type:text" json:"provider,omitempty"`
	ProviderResponseID string         `gorm:"type:text" json:"provider_response_id,omitempty"`
	StartedAt          time.Time      `gorm:"not null;default:now()" json:"started_at"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
	Error              datatypes.JSON `gorm:"type:jsonb" json:"error,omitempty"`
	Usage              datatypes.JSON `gorm:"type:jsonb" json:"usage,omitempty"`
	Metadata           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
}

// TableName specifies the table name for GORM.
func (Run) TableName() string {
	return "runs"
}
