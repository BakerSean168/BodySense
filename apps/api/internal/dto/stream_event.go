package dto

import (
	"encoding/json"
)

type StreamEventIDs struct {
	ConversationID string `json:"conversation_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	TurnID         string `json:"turn_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	ToolCallID     string `json:"tool_call_id,omitempty"`
}

type StreamEvent struct {
	Version int             `json:"version"`
	Seq     int             `json:"seq"`
	Channel string          `json:"channel"`
	Type    string          `json:"type"`
	IDs     StreamEventIDs  `json:"ids"`
	Payload json.RawMessage `json:"payload"`
}

func NewStreamEvent(seq int, channel, eventType string, ids StreamEventIDs, payload any) (StreamEvent, error) {
	payloadJSON := json.RawMessage(`{}`)
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return StreamEvent{}, err
		}
		payloadJSON = data
	}
	return StreamEvent{
		Version: 1,
		Seq:     seq,
		Channel: channel,
		Type:    eventType,
		IDs:     ids,
		Payload: payloadJSON,
	}, nil
}

func (e StreamEvent) PayloadAs(target any) error {
	if len(e.Payload) == 0 {
		return json.Unmarshal([]byte(`{}`), target)
	}
	return json.Unmarshal(e.Payload, target)
}
