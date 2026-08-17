package dto

import "encoding/json"

type ShareResponse struct {
	ShareToken string `json:"shareToken"`
	ShareURL   string `json:"shareUrl"`
}

type SharedConversationResponse struct {
	Title    string          `json:"title"`
	Messages json.RawMessage `json:"messages"`
}
