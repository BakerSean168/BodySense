package chat

import (
	"encoding/json"
	"fmt"
)

type assistantPartsBuilder struct {
	parts              []map[string]any
	toolPartIndexByID  map[string]int
	legacyToolCallSeed int
}

func newAssistantPartsBuilder() *assistantPartsBuilder {
	return &assistantPartsBuilder{
		parts:             make([]map[string]any, 0, 8),
		toolPartIndexByID: make(map[string]int),
	}
}

func (b *assistantPartsBuilder) Parts() []map[string]any {
	return b.parts
}

func (b *assistantPartsBuilder) AddTextDelta(delta string) {
	if delta == "" {
		return
	}
	if len(b.parts) > 0 {
		last := b.parts[len(b.parts)-1]
		if last["type"] == "text" {
			if current, ok := last["text"].(string); ok {
				last["text"] = current + delta
				return
			}
		}
	}
	b.parts = append(b.parts, map[string]any{
		"type": "text",
		"text": delta,
	})
}

func (b *assistantPartsBuilder) AddToolCall(toolCallID, toolName string, args json.RawMessage) {
	id := toolCallID
	if id == "" {
		id = b.nextLegacyToolCallID()
	}
	part := map[string]any{
		"type":       "tool-call",
		"toolCallId": id,
		"toolName":   toolName,
		"args":       decodeJSONObject(args),
		"argsText":   string(args),
	}
	b.parts = append(b.parts, part)
	b.toolPartIndexByID[id] = len(b.parts) - 1
}

func (b *assistantPartsBuilder) AddToolResult(toolCallID, toolName string, result json.RawMessage) {
	if toolCallID != "" {
		if index, ok := b.toolPartIndexByID[toolCallID]; ok {
			b.parts[index]["result"] = decodeJSON(result)
			b.parts[index]["isError"] = toolResultIsError(result)
			return
		}
	}

	if index, ok := b.findLatestOpenToolCall(toolName); ok {
		b.parts[index]["result"] = decodeJSON(result)
		b.parts[index]["isError"] = toolResultIsError(result)
		return
	}

	id := toolCallID
	if id == "" {
		id = b.nextLegacyToolCallID()
	}
	b.parts = append(b.parts, map[string]any{
		"type":       "tool-call",
		"toolCallId": id,
		"toolName":   toolName,
		"args":       map[string]any{},
		"argsText":   "",
		"result":     decodeJSON(result),
		"isError":    toolResultIsError(result),
	})
	b.toolPartIndexByID[id] = len(b.parts) - 1
}

func (b *assistantPartsBuilder) AddCitation(payload json.RawMessage) {
	var envelope struct {
		Citation struct {
			Title        string `json:"title"`
			URL          string `json:"url"`
			Summary      string `json:"summary"`
			Snippet      string `json:"snippet"`
			SourceTitle  string `json:"source_title"`
			SourceAuthor string `json:"source_author"`
		} `json:"citation"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return
	}

	providerMetadata := map[string]any{}
	bodysenseMetadata := map[string]any{}
	if envelope.Citation.Summary != "" {
		bodysenseMetadata["summary"] = envelope.Citation.Summary
	}
	if envelope.Citation.Snippet != "" {
		bodysenseMetadata["snippet"] = envelope.Citation.Snippet
	}
	if envelope.Citation.SourceTitle != "" {
		bodysenseMetadata["source_title"] = envelope.Citation.SourceTitle
	}
	if envelope.Citation.SourceAuthor != "" {
		bodysenseMetadata["source_author"] = envelope.Citation.SourceAuthor
	}
	if len(bodysenseMetadata) > 0 {
		providerMetadata["bodysense"] = bodysenseMetadata
	}

	part := map[string]any{
		"type":       "source",
		"sourceType": "url",
		"id":         "citation-" + envelope.Citation.Title,
		"title":      envelope.Citation.Title,
		"url":        envelope.Citation.URL,
	}
	if len(providerMetadata) > 0 {
		part["providerMetadata"] = providerMetadata
	}
	b.parts = append(b.parts, part)
}

func (b *assistantPartsBuilder) AddKnowledgeGap(payload json.RawMessage) {
	b.parts = append(b.parts, map[string]any{
		"type": "data",
		"name": "knowledge_gap",
		"data": decodeJSON(payload),
	})
}

func (b *assistantPartsBuilder) AddRedFlag(payload json.RawMessage) {
	b.parts = append(b.parts, map[string]any{
		"type": "data",
		"name": "red_flag",
		"data": decodeJSON(payload),
	})
}

func (b *assistantPartsBuilder) findLatestOpenToolCall(toolName string) (int, bool) {
	for index := len(b.parts) - 1; index >= 0; index-- {
		part := b.parts[index]
		if part["type"] != "tool-call" {
			continue
		}
		name, _ := part["toolName"].(string)
		if name != toolName {
			continue
		}
		if _, exists := part["result"]; exists {
			continue
		}
		return index, true
	}
	return 0, false
}

func (b *assistantPartsBuilder) nextLegacyToolCallID() string {
	id := fmt.Sprintf("legacy-tool-call-%d", b.legacyToolCallSeed)
	b.legacyToolCallSeed++
	return id
}

func decodeJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}

func decodeJSONObject(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	return value
}
