package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// EpisodicNote is a non-authoritative, provenance-bound conversational note.
// S1 only declares the shape. There is deliberately no repository writer,
// background job, HTTP route, model retrieval or clinical decision consumer.
// The DB permits shadow rows only until erasure and user-delete controls ship.
type EpisodicNote struct {
	ID                     uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	UserID                 uuid.UUID      `gorm:"type:uuid;not null"`
	ConversationID         uuid.UUID      `gorm:"type:uuid;not null"`
	FromSeq                int            `gorm:"not null"`
	ToSeq                  int            `gorm:"not null"`
	SourceMessageIDs       pq.StringArray `gorm:"type:uuid[];not null"`
	SourceAssetIDs         pq.StringArray `gorm:"type:uuid[];not null;default:{}"`
	GeneratorVersion       string         `gorm:"size:128;not null"`
	RetentionPolicyVersion string         `gorm:"size:128;not null"`
	ContentText            string         `gorm:"type:text;not null"`
	Status                 string         `gorm:"size:24;not null;default:shadow"`
	CreatedAt              time.Time      `gorm:"not null"`
	UpdatedAt              time.Time      `gorm:"not null"`
}

func (EpisodicNote) TableName() string { return "episodic_notes" }
