package dto

import (
	"encoding/json"
	"time"
)

type RuntimeEventDTO struct {
	Seq       int             `json:"seq"`
	Channel   string          `json:"channel"`
	Type      string          `json:"type"`
	IDs       json.RawMessage `json:"ids"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

type ListRuntimeEventsResponse struct {
	Events       []RuntimeEventDTO `json:"events"`
	HasMore      bool              `json:"hasMore"`
	NextAfterSeq *int              `json:"nextAfterSeq,omitempty"`
}
