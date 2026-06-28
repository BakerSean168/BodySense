package dto

type SendMessageRequest struct {
	ConversationID  *string     `json:"conversationId"`
	ClientDraftID   string      `json:"clientDraftId,omitempty" binding:"max=128"`
	ClientMessageID string      `json:"clientMessageId" binding:"required,max=128"`
	RequestID       string      `json:"requestId" binding:"required,max=128"`
	Message         MessageDTO  `json:"message" binding:"required"`
	Context         *ContextDTO `json:"context,omitempty"`
}

type MessageDTO struct {
	Role  string    `json:"role"`
	Parts []PartDTO `json:"parts" binding:"required,min=1,dive"`
}

type PartDTO struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ContextDTO struct {
	Entry     string `json:"entry,omitempty"`
	ProfileID string `json:"profileId,omitempty"`
}

type PinRequest struct {
	Pinned bool `json:"pinned"`
}

type RenameRequest struct {
	Title string `json:"title" binding:"required,max=200"`
}
