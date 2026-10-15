package model

import (
	"time"

	"github.com/google/uuid"
)

// SafetyScreen is the user's answer to the Go-validated five-concept safety
// checklist of one structured symptom intake. Rows exist only for a complete
// checklist bound to an accepted interaction answer; no model output, seed
// text or free-form detail can create one.
type SafetyScreen struct {
	ID                   uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID               uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	CaptureID            string    `gorm:"not null" json:"capture_id"`
	SourceConversationID uuid.UUID `gorm:"type:uuid;not null" json:"source_conversation_id"`
	SourceRunID          uuid.UUID `gorm:"type:uuid;not null" json:"source_run_id"`
	SourceMessageID      uuid.UUID `gorm:"type:uuid;not null" json:"source_message_id"`
	SourceInteractionID  uuid.UUID `gorm:"type:uuid;not null" json:"source_interaction_id"`
	SourceToolCallID     string    `gorm:"not null" json:"source_tool_call_id"`
	Trauma               bool      `gorm:"not null" json:"trauma"`
	RadiatingPain        bool      `gorm:"not null" json:"radiating_pain"`
	Numbness             bool      `gorm:"not null" json:"numbness"`
	Weakness             bool      `gorm:"not null" json:"weakness"`
	Dizziness            bool      `gorm:"not null" json:"dizziness"`
	BodyStateRevision    int64     `gorm:"not null" json:"body_state_revision"`
	RecordedAt           time.Time `gorm:"not null" json:"recorded_at"`
}

func (SafetyScreen) TableName() string { return "safety_screens" }

// SafetyScreenAnswers is the complete five-concept selection. It has no
// partial form: an incomplete checklist is simply not a screen.
type SafetyScreenAnswers struct {
	Trauma        bool
	RadiatingPain bool
	Numbness      bool
	Weakness      bool
	Dizziness     bool
}

// StructuredIntakeCommand is the Go-built result of one bound symptom intake
// answer: exactly one symptom statement and, when the checklist was complete,
// its safety screen. Both commit in one BodyState revision or not at all.
type StructuredIntakeCommand struct {
	CaptureID string
	Fact      BodyFactInput
	Screen    *SafetyScreenAnswers
}
