package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Conversation represents a chat conversation.
type Conversation struct {
	ID                     uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	UserID                 uuid.UUID      `gorm:"type:uuid;index;not null" json:"user_id"`
	Title                  string         `gorm:"type:text" json:"title,omitempty"`
	TitleStatus            string         `gorm:"type:varchar(20);not null;default:'pending'" json:"title_status"`
	Status                 string         `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	Pinned                 bool           `gorm:"not null;default:false" json:"pinned"`
	PinnedAt               *time.Time     `json:"pinned_at,omitempty"`
	DefaultModel           string         `gorm:"type:text" json:"default_model,omitempty"`
	SystemPromptVersion    string         `gorm:"type:text" json:"system_prompt_version,omitempty"`
	Provider               string         `gorm:"type:text" json:"provider,omitempty"`
	ProviderConversationID string         `gorm:"type:text" json:"provider_conversation_id,omitempty"`
	ProviderLastResponseID string         `gorm:"type:text" json:"provider_last_response_id,omitempty"`
	ActiveRunID            *uuid.UUID     `gorm:"type:uuid" json:"active_run_id,omitempty"`
	ActiveStreamID         string         `gorm:"type:text" json:"active_stream_id,omitempty"`
	Summary                string         `gorm:"type:text" json:"summary,omitempty"`
	Metadata               datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt              time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt              time.Time      `gorm:"not null;default:now()" json:"updated_at"`
	LastMessageAt          *time.Time     `json:"last_message_at,omitempty"`
	DeletedAt              *time.Time     `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name for GORM.
func (Conversation) TableName() string {
	return "conversations"
}
