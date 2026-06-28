package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Message represents a single message within a conversation.
type Message struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	ConversationID     uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_messages_conversation_seq" json:"conversation_id"`
	TurnID             uuid.UUID      `gorm:"type:uuid;not null" json:"turn_id"`
	ParentMessageID    *uuid.UUID     `gorm:"type:uuid" json:"parent_message_id,omitempty"`
	Role               string         `gorm:"type:varchar(20);not null" json:"role"`
	Status             string         `gorm:"type:varchar(20);not null;default:'completed'" json:"status"`
	Seq                int            `gorm:"not null;uniqueIndex:idx_messages_conversation_seq" json:"seq"`
	Parts              datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"parts"`
	ContentText        string         `gorm:"type:text" json:"content_text,omitempty"`
	Model              string         `gorm:"type:text" json:"model,omitempty"`
	Provider           string         `gorm:"type:text" json:"provider,omitempty"`
	ProviderMessageID  string         `gorm:"type:text" json:"provider_message_id,omitempty"`
	ProviderResponseID string         `gorm:"type:text" json:"provider_response_id,omitempty"`
	InputTokens        *int           `json:"input_tokens,omitempty"`
	OutputTokens       *int           `json:"output_tokens,omitempty"`
	TotalTokens        *int           `json:"total_tokens,omitempty"`
	Error              datatypes.JSON `gorm:"type:jsonb" json:"error,omitempty"`
	Metadata           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt          time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt          time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (Message) TableName() string {
	return "messages"
}
