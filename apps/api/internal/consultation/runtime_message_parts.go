package consultation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type durableMessagePart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	UploadID string `json:"upload_id,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

func marshalDurableMessageParts(parts []PartInput) ([]byte, error) {
	projected := make([]durableMessagePart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			projected = append(projected, durableMessagePart{Type: "text", Text: part.Text})
		case "image":
			projected = append(projected, durableMessagePart{
				Type:     "image",
				UploadID: part.UploadID,
				MimeType: part.MimeType,
				ImageURL: part.ImageURL,
			})
		default:
			return nil, fmt.Errorf("unsupported message part type %q", part.Type)
		}
	}
	return json.Marshal(projected)
}

func messagePartsToImageUploadIDs(parts []PartInput) []string {
	ids := make([]string, 0)
	seen := map[string]bool{}
	for _, part := range parts {
		if part.Type != "image" {
			continue
		}
		id := strings.TrimSpace(part.UploadID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// resolveConsultationImages turns image parts into data-URLs owned by uid.
// Failures are logged and skipped so a bad attachment does not kill the turn.
func (r *Runtime) resolveConsultationImages(
	ctx context.Context,
	uid uuid.UUID,
	parts []PartInput,
) []service.ConsultationImageRef {
	if r.uploadService == nil {
		return nil
	}
	ids := messagePartsToImageUploadIDs(parts)
	if len(ids) == 0 {
		return nil
	}
	const maxImages = 3
	if len(ids) > maxImages {
		ids = ids[:maxImages]
	}
	out := make([]service.ConsultationImageRef, 0, len(ids))
	for _, rawID := range ids {
		uploadID, err := uuid.Parse(rawID)
		if err != nil {
			log.Printf("consultation image upload_id invalid %q: %v", rawID, err)
			continue
		}
		dataURL, mime, err := r.uploadService.ReadImageDataURL(ctx, uid, uploadID)
		if err != nil {
			log.Printf("consultation image resolve %s: %v", uploadID, err)
			continue
		}
		out = append(out, service.ConsultationImageRef{
			UploadID: uploadID.String(),
			MimeType: mime,
			DataURL:  dataURL,
		})
	}
	return out
}

func messagePartsToText(parts []PartInput) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}

func interactionAnswerParts(answer json.RawMessage, interactionID string) (datatypes.JSON, datatypes.JSON) {
	text := extractAnswerText(answer)
	partsJSON, _ := json.Marshal([]map[string]any{{"type": "text", "text": text}})
	metadata := map[string]any{
		"is_interaction_answer": true,
		"interaction_id":        interactionID,
	}
	// Preserve structured multi-field answers for runtime audit and analytics.
	var raw map[string]any
	if err := json.Unmarshal(answer, &raw); err == nil {
		if fields, ok := raw["fields"].(map[string]any); ok && len(fields) > 0 {
			metadata["fields"] = fields
		}
		if selected, ok := raw["selected"]; ok {
			metadata["selected"] = selected
		}
	}
	metadataJSON, _ := json.Marshal(metadata)
	return datatypes.JSON(partsJSON), datatypes.JSON(metadataJSON)
}

func extractAnswerText(answer json.RawMessage) string {
	var rawAnswer map[string]any
	if err := json.Unmarshal(answer, &rawAnswer); err != nil {
		return string(answer)
	}
	if text, ok := rawAnswer["text"].(string); ok {
		return text
	}
	// Multi-field form answers: { fields: { key: value, ... } }
	if fields, ok := rawAnswer["fields"].(map[string]any); ok && len(fields) > 0 {
		parts := make([]string, 0, len(fields))
		for key, value := range fields {
			parts = append(parts, fmt.Sprintf("%s: %v", key, value))
		}
		// Stable order is not critical for LLM context readability.
		sort.Strings(parts)
		return strings.Join(parts, "；")
	}
	if selected, ok := rawAnswer["selected"].([]any); ok {
		parts := make([]string, 0, len(selected))
		for _, item := range selected {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ", ")
	}
	if value, ok := rawAnswer["value"]; ok {
		return fmt.Sprintf("%v", value)
	}
	// Fallback: serialize the entire answer as text
	if b, err := json.Marshal(rawAnswer); err == nil {
		return string(b)
	}
	return ""
}
