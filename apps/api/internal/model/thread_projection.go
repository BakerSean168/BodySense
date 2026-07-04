package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ThreadProjection is the durable thread read model consumed by the web workbench.
type ThreadProjection struct {
	ConversationID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"conversation_id"`
	UserID                uuid.UUID      `gorm:"type:uuid;index;not null" json:"user_id"`
	Title                 string         `gorm:"type:text" json:"title,omitempty"`
	TitleStatus           string         `gorm:"type:varchar(20);not null;default:'pending'" json:"title_status"`
	Status                string         `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	Pinned                bool           `gorm:"not null;default:false" json:"pinned"`
	PinnedAt              *time.Time     `json:"pinned_at,omitempty"`
	DefaultModel          string         `gorm:"type:text" json:"default_model,omitempty"`
	ActiveRunID           *uuid.UUID     `gorm:"type:uuid" json:"active_run_id,omitempty"`
	LastMessageAt         *time.Time     `json:"last_message_at,omitempty"`
	Metadata              datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	Phase                 string         `gorm:"type:varchar(30);not null;default:'collecting'" json:"phase"`
	ExtractedInfo         datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"extracted_info"`
	Diagnosis             datatypes.JSON `gorm:"type:jsonb" json:"diagnosis,omitempty"`
	TreatmentPlan         datatypes.JSON `gorm:"type:jsonb" json:"treatment_plan,omitempty"`
	PendingInteractions   datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"pending_interactions"`
	ConversationCreatedAt time.Time      `gorm:"column:conversation_created_at;not null" json:"conversation_created_at"`
	ConversationUpdatedAt time.Time      `gorm:"column:conversation_updated_at;not null" json:"conversation_updated_at"`
	SessionCreatedAt      time.Time      `gorm:"column:session_created_at;not null" json:"created_at"`
	SessionUpdatedAt      time.Time      `gorm:"column:session_updated_at;not null" json:"updated_at"`
	EndedAt               *time.Time     `json:"ended_at,omitempty"`
	RefreshedAt           time.Time      `gorm:"not null;default:now()" json:"refreshed_at"`
}

func (ThreadProjection) TableName() string {
	return "thread_projections"
}

// ThreadProjectionMessage is the durable message-level read model for a thread.
type ThreadProjectionMessage struct {
	MessageID          uuid.UUID      `gorm:"column:message_id;type:uuid;primaryKey" json:"id"`
	ConversationID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`
	TurnID             uuid.UUID      `gorm:"type:uuid;not null" json:"turn_id"`
	RunID              *uuid.UUID     `gorm:"type:uuid" json:"run_id,omitempty"`
	ParentMessageID    *uuid.UUID     `gorm:"type:uuid" json:"parent_message_id,omitempty"`
	Seq                int            `gorm:"not null" json:"seq"`
	Role               string         `gorm:"type:varchar(20);not null" json:"role"`
	Status             string         `gorm:"type:varchar(20);not null" json:"status"`
	Parts              datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"parts"`
	ContentText        string         `gorm:"type:text;not null;default:''" json:"content_text,omitempty"`
	Model              string         `gorm:"type:text" json:"model,omitempty"`
	Provider           string         `gorm:"type:text" json:"provider,omitempty"`
	ProviderMessageID  string         `gorm:"type:text" json:"provider_message_id,omitempty"`
	ProviderResponseID string         `gorm:"type:text" json:"provider_response_id,omitempty"`
	InputTokens        *int           `json:"input_tokens,omitempty"`
	OutputTokens       *int           `json:"output_tokens,omitempty"`
	TotalTokens        *int           `json:"total_tokens,omitempty"`
	Error              datatypes.JSON `gorm:"type:jsonb" json:"error,omitempty"`
	Metadata           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt          time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt          time.Time      `gorm:"not null" json:"updated_at"`
}

func (ThreadProjectionMessage) TableName() string {
	return "thread_projection_messages"
}

// ThreadProjectionToolCall is the durable tool timeline read model for a thread.
type ThreadProjectionToolCall struct {
	ToolCallID     string         `gorm:"column:tool_call_id;type:text;primaryKey" json:"tool_call_id"`
	ConversationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`
	RunID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"run_id"`
	MessageID      *uuid.UUID     `gorm:"type:uuid" json:"message_id,omitempty"`
	ToolName       string         `gorm:"type:text;not null" json:"tool_name"`
	Arguments      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"arguments"`
	Status         string         `gorm:"type:varchar(30);not null" json:"status"`
	Result         datatypes.JSON `gorm:"type:jsonb" json:"result,omitempty"`
	Error          datatypes.JSON `gorm:"type:jsonb" json:"error,omitempty"`
	CreatedAt      time.Time      `gorm:"not null" json:"created_at"`
	StartedAt      time.Time      `gorm:"not null" json:"started_at"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
}

func (ThreadProjectionToolCall) TableName() string {
	return "thread_projection_tool_calls"
}
