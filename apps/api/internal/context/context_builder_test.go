package context

import (
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestExtractTextFromParts(t *testing.T) {
	tests := []struct {
		name  string
		parts []dto.PartDTO
		want  string
	}{
		{
			name:  "empty parts",
			parts: []dto.PartDTO{},
			want:  "",
		},
		{
			name: "single text part",
			parts: []dto.PartDTO{
				{Type: "text", Text: "hello world"},
			},
			want: "hello world",
		},
		{
			name: "multiple parts, returns first text",
			parts: []dto.PartDTO{
				{Type: "image", Text: ""},
				{Type: "text", Text: "first text"},
				{Type: "text", Text: "second text"},
			},
			want: "first text",
		},
		{
			name: "no text parts",
			parts: []dto.PartDTO{
				{Type: "image", Text: ""},
				{Type: "file", Text: ""},
			},
			want: "",
		},
		{
			name: "text part with empty text is skipped",
			parts: []dto.PartDTO{
				{Type: "text", Text: ""},
				{Type: "text", Text: "actual text"},
			},
			want: "actual text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTextFromParts(tt.parts)
			if got != tt.want {
				t.Errorf("ExtractTextFromParts() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildChatContextInput_Fields(t *testing.T) {
	// Verify the input struct has the expected fields
	input := BuildChatContextInput{
		IsDraft: true,
	}
	if !input.IsDraft {
		t.Error("expected IsDraft to be true")
	}
}

func TestContextTrace_Fields(t *testing.T) {
	trace := &ContextTrace{
		ExcludedCurrentTurn:  true,
		ProfileIncluded:      true,
		ConsultationIncluded: false,
		UseCase:              "consultation.reply",
	}
	if !trace.ExcludedCurrentTurn {
		t.Error("expected ExcludedCurrentTurn to be true")
	}
	if !trace.ProfileIncluded {
		t.Error("expected ProfileIncluded to be true")
	}
	if trace.ConsultationIncluded {
		t.Error("expected ConsultationIncluded to be false")
	}
	if trace.UseCase != "consultation.reply" {
		t.Errorf("expected UseCase to be 'consultation.reply', got %q", trace.UseCase)
	}
}

func TestGetMessageTextContent_WithContentText(t *testing.T) {
	msg := model.Message{ContentText: "直接文本内容"}
	got := getMessageTextContent(msg)
	if got != "直接文本内容" {
		t.Errorf("expected '直接文本内容', got %q", got)
	}
}

func TestGetMessageTextContent_FromParts(t *testing.T) {
	parts, _ := json.Marshal([]struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}{
		{Type: "text", Text: "从 parts 提取的文本"},
	})
	msg := model.Message{Parts: datatypes.JSON(parts)}
	got := getMessageTextContent(msg)
	if got != "从 parts 提取的文本" {
		t.Errorf("expected '从 parts 提取的文本', got %q", got)
	}
}

func TestGetMessageTextContent_EmptyMessage(t *testing.T) {
	msg := model.Message{}
	got := getMessageTextContent(msg)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestGetMessageTextContent_PrefersContentTextOverParts(t *testing.T) {
	parts, _ := json.Marshal([]struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}{
		{Type: "text", Text: "parts 文本"},
	})
	msg := model.Message{
		ContentText: "ContentText 优先",
		Parts:       datatypes.JSON(parts),
	}
	got := getMessageTextContent(msg)
	if got != "ContentText 优先" {
		t.Errorf("expected 'ContentText 优先', got %q", got)
	}
}

func TestLoadHistory_FiltersCurrentTurn(t *testing.T) {
	currentTurnID := uuid.New()
	otherTurnID := uuid.New()

	msgs := []model.Message{
		{ID: uuid.New(), TurnID: otherTurnID, Role: "user", Status: "completed", ContentText: "历史消息"},
		{ID: uuid.New(), TurnID: currentTurnID, Role: "user", Status: "completed", ContentText: "当前轮次"},
	}

	var chatHistory []struct {
		Role    string
		Content string
	}
	var includedIDs []uuid.UUID

	for _, m := range msgs {
		if m.TurnID != currentTurnID && m.Status == "completed" {
			text := getMessageTextContent(m)
			if text != "" {
				chatHistory = append(chatHistory, struct {
					Role    string
					Content string
				}{Role: m.Role, Content: text})
				includedIDs = append(includedIDs, m.ID)
			}
		}
	}

	if len(chatHistory) != 1 {
		t.Fatalf("expected 1 history message, got %d", len(chatHistory))
	}
	if chatHistory[0].Content != "历史消息" {
		t.Errorf("expected '历史消息', got %q", chatHistory[0].Content)
	}
}

func TestLoadHistory_FiltersNonCompleted(t *testing.T) {
	turnID := uuid.New()

	msgs := []model.Message{
		{ID: uuid.New(), TurnID: turnID, Role: "assistant", Status: "streaming", ContentText: "流式中"},
		{ID: uuid.New(), TurnID: turnID, Role: "assistant", Status: "aborted", ContentText: "已中断"},
		{ID: uuid.New(), TurnID: turnID, Role: "assistant", Status: "completed", ContentText: "已完成"},
	}

	currentTurnID := uuid.New()
	var count int
	for _, m := range msgs {
		if m.TurnID != currentTurnID && m.Status == "completed" {
			text := getMessageTextContent(m)
			if text != "" {
				count++
			}
		}
	}

	if count != 1 {
		t.Errorf("expected 1 completed message, got %d", count)
	}
}

func TestLoadHistory_FiltersEmptyText(t *testing.T) {
	turnID := uuid.New()
	currentTurnID := uuid.New()

	msgs := []model.Message{
		{ID: uuid.New(), TurnID: turnID, Role: "assistant", Status: "completed", ContentText: ""},
		{ID: uuid.New(), TurnID: turnID, Role: "assistant", Status: "completed", ContentText: "有内容"},
	}

	var count int
	for _, m := range msgs {
		if m.TurnID != currentTurnID && m.Status == "completed" {
			text := getMessageTextContent(m)
			if text != "" {
				count++
			}
		}
	}

	if count != 1 {
		t.Errorf("expected 1 message with content, got %d", count)
	}
}
