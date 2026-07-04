package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// RuntimeEvent is a durable public runtime event emitted for a run.
type RuntimeEvent struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	ConversationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`
	RunID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"run_id"`
	TurnID         *uuid.UUID     `gorm:"type:uuid" json:"turn_id,omitempty"`
	Seq            int            `gorm:"not null" json:"seq"`
	Channel        string         `gorm:"type:varchar(40);not null" json:"channel"`
	Type           string         `gorm:"type:varchar(120);not null" json:"type"`
	IDs            datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"ids"`
	Payload        datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"payload"`
	Source         string         `gorm:"type:varchar(20);not null;default:'go'" json:"source"`
	Replayable     bool           `gorm:"not null;default:true" json:"replayable"`
	CreatedAt      time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the table name for GORM.
func (RuntimeEvent) TableName() string {
	return "runtime_events"
}
