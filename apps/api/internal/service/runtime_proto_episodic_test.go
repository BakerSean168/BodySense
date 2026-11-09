package service

import (
	"encoding/json"
	"testing"
	"time"
)

// S4 serving regression: the recall result must survive the Proto projection.
// The earlier unit test only marshaled the in-memory Go struct, so a missing
// Proto field silently produced an empty runtime payload on staging.
func TestStartTurnCommandCarriesEpisodicNotes(t *testing.T) {
	note := ConsultationEpisodicNote{
		NoteID:           testRuntimeRunID,
		FromSeq:          11,
		ToSeq:            12,
		SourceMessageIDs: []string{testRuntimeConversationID},
		CreatedAt:        time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC),
		Text:             "用户提到周末去了早市买菜。",
		Authority:        ConsultationEpisodicNoteAuthority,
	}
	req := validStartConsultationTurnRequest()
	req.BusinessContext.EpisodicNotes = []ConsultationEpisodicNote{note}

	body, err := marshalStartTurnCommand(testRuntimeThreadID, req)
	if err != nil {
		t.Fatal(err)
	}
	var command struct {
		BusinessContext struct {
			EpisodicNotes []struct {
				NoteID           string   `json:"note_id"`
				FromSeq          int      `json:"from_seq"`
				ToSeq            int      `json:"to_seq"`
				SourceMessageIDs []string `json:"source_message_ids"`
				CreatedAt        string   `json:"created_at"`
				Text             string   `json:"text"`
				Authority        string   `json:"authority"`
			} `json:"episodic_notes"`
		} `json:"business_context"`
	}
	if err := json.Unmarshal(body, &command); err != nil {
		t.Fatal(err)
	}
	notes := command.BusinessContext.EpisodicNotes
	if len(notes) != 1 {
		t.Fatalf("episodic_notes = %d, want 1\n%s", len(notes), body)
	}
	got := notes[0]
	if got.NoteID != note.NoteID || got.FromSeq != note.FromSeq || got.ToSeq != note.ToSeq ||
		got.Text != note.Text || got.Authority != note.Authority ||
		len(got.SourceMessageIDs) != 1 || got.SourceMessageIDs[0] != note.SourceMessageIDs[0] ||
		got.CreatedAt == "" {
		t.Fatalf("episodic note lost fields on the wire: %+v", got)
	}
}

func TestStartTurnCommandDropsMalformedEpisodicNoteWithoutFailingTheTurn(t *testing.T) {
	req := validStartConsultationTurnRequest()
	req.BusinessContext.EpisodicNotes = []ConsultationEpisodicNote{{
		NoteID:           "not-a-uuid",
		FromSeq:          1,
		ToSeq:            2,
		SourceMessageIDs: []string{testRuntimeConversationID},
		CreatedAt:        time.Now().UTC(),
		Text:             "synthetic",
		Authority:        ConsultationEpisodicNoteAuthority,
	}}

	body, err := marshalStartTurnCommand(testRuntimeThreadID, req)
	if err != nil {
		t.Fatalf("malformed non-clinical note blocked the turn: %v", err)
	}
	var command struct {
		BusinessContext map[string]json.RawMessage `json:"business_context"`
	}
	if err := json.Unmarshal(body, &command); err != nil {
		t.Fatal(err)
	}
	if raw, present := command.BusinessContext["episodic_notes"]; present {
		t.Fatalf("malformed note crossed the boundary: %s", raw)
	}
}
