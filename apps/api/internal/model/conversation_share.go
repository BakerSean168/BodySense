package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ConversationShare represents a public share snapshot of a conversation.
type ConversationShare struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	ConversationID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`
	ShareToken       string         `gorm:"type:varchar(32);uniqueIndex;not null" json:"share_token"`
	SnapshotMessages datatypes.JSON `gorm:"type:jsonb;not null" json:"snapshot_messages"`
	SnapshotTitle    string         `gorm:"type:varchar(200)" json:"snapshot_title,omitempty"`
	CreatedAt        time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the table name for GORM.
func (ConversationShare) TableName() string {
	return "conversation_shares"
}
