package consultation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bodysense/api/internal/episodicrecall"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type recallCandidateStore struct {
	notes []model.EpisodicNote
	calls int
}

func (s *recallCandidateStore) ListOwnedRecallCandidates(ctx context.Context, owner uuid.UUID, limit int) ([]model.EpisodicNote, error) {
	s.calls++
	return s.notes, nil
}

func episodicTestNote(text string) model.EpisodicNote {
	source := uuid.New()
	return model.EpisodicNote{
		ID:               uuid.New(),
		ConversationID:   uuid.New(),
		FromSeq:          11,
		ToSeq:            12,
		SourceMessageIDs: pq.StringArray{source.String()},
		ContentText:      text,
		Status:           "shadow",
		CreatedAt:        time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC),
	}
}

// The recall result may exist only in the consultation non-clinical channel.
// Diagnosis payloads are assembled by a different path and stay untouched.
func TestBuildBusinessContextInjectsEpisodicNotesOnlyInTheirChannel(t *testing.T) {
	note := episodicTestNote("用户提到周末去了早市买菜。")
	store := &recallCandidateStore{notes: []model.EpisodicNote{note}}
	runtime := &Runtime{episodicRecall: episodicrecall.NewRecaller(store)}

	bc := runtime.buildBusinessContext(
		context.Background(), uuid.New(), uuid.New(), "你还记得我去早市的事吗",
		json.RawMessage(`{}`), &model.ConsultationSession{}, nil,
	)
	if len(bc.EpisodicNotes) != 1 {
		t.Fatalf("injected %d notes, want 1", len(bc.EpisodicNotes))
	}
	injected := bc.EpisodicNotes[0]
	if injected.Text != note.ContentText {
		t.Fatalf("note text was rewritten: %q", injected.Text)
	}
	if injected.NoteID != note.ID.String() || injected.FromSeq != 11 || injected.ToSeq != 12 ||
		len(injected.SourceMessageIDs) != 1 || injected.SourceMessageIDs[0] != string(note.SourceMessageIDs[0]) {
		t.Fatalf("provenance missing from injected note: %+v", injected)
	}
	if injected.Authority != service.ConsultationEpisodicNoteAuthority {
		t.Fatalf("authority label = %q", injected.Authority)
	}
	if len(bc.CurrentDiagnosis) != 0 || len(bc.BodyState) != 0 {
		t.Fatalf("episodic recall leaked into clinical payload fields")
	}

	raw, err := json.Marshal(bc)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["episodic_notes"]; !ok {
		t.Fatalf("episodic_notes field missing from wire payload")
	}
	for _, key := range []string{"current_diagnosis", "body_state"} {
		if value, exists := decoded[key]; exists && string(value) != "null" {
			t.Fatalf("clinical field %s was populated by recall: %s", key, value)
		}
	}
}

func TestBuildBusinessContextFlagOffPerformsNoRecallQuery(t *testing.T) {
	runtime := &Runtime{} // no recaller == EPISODIC_RECALL_ENABLED off

	bc := runtime.buildBusinessContext(
		context.Background(), uuid.New(), uuid.New(), "你还记得我去早市的事吗",
		json.RawMessage(`{}`), &model.ConsultationSession{}, nil,
	)
	if len(bc.EpisodicNotes) != 0 {
		t.Fatalf("flag-off turn injected %d notes", len(bc.EpisodicNotes))
	}
	raw, err := json.Marshal(bc)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["episodic_notes"]; ok {
		t.Fatalf("flag-off payload must not carry episodic_notes")
	}
}
