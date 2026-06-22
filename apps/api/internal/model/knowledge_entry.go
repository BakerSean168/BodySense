package model

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

// KnowledgeEntry represents a knowledge base entry with vector embedding.
type KnowledgeEntry struct {
	ID              int64              `gorm:"primaryKey;autoIncrement" json:"id"`
	Category        string             `gorm:"type:varchar(100);not null;index" json:"category"`
	Title           string             `gorm:"type:varchar(500);not null" json:"title"`
	Content         string             `gorm:"type:text;not null" json:"content"`
	Embedding       pgvector.Vector    `gorm:"type:vector(1536)" json:"embedding,omitempty"`
	SourceVideo     string             `gorm:"type:varchar(500)" json:"source_video,omitempty"`
	SourceTimestamp string             `gorm:"type:varchar(50)" json:"source_timestamp,omitempty"`
	CreatedAt       time.Time          `gorm:"not null;default:now();index" json:"created_at"`
	UpdatedAt       time.Time          `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (KnowledgeEntry) TableName() string {
	return "knowledge_entries"
}
