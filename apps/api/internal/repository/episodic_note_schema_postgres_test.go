package repository

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// #495 S1: migration contract only. Source-message UUID arrays are soft
// provenance, not clinical evidence or references that block raw pruning.
// Runtime writes and reads remain absent until the later privacy gates.
func TestEpisodicNoteSchemaIsInertAndOwnerScopedPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	owner := createSingletonTestUser(t, db)
	other := createSingletonTestUser(t, db)
	conversation, _, err := NewConversationRepository(db).GetOrCreateForUser(t.Context(), owner)
	if err != nil {
		t.Fatal(err)
	}

	source := uuid.New()
	insert := `INSERT INTO episodic_notes
      (user_id, conversation_id, from_seq, to_seq, source_message_ids,
       generator_version, retention_policy_version, content_text)
      VALUES (?, ?, ?, ?, ARRAY[?::uuid], 'test-generator-v1', 'test-policy-v1', ?)`

	if err := db.Exec(insert, owner, conversation.ID, 1, 2, source, "synthetic episode").Error; err != nil {
		t.Fatalf("expected valid shadow row: %v", err)
	}

	var status string
	if err := db.Raw("SELECT status FROM episodic_notes WHERE user_id = ?", owner).Scan(&status).Error; err != nil || status != "shadow" {
		t.Fatalf("S1 may write shadow rows only, status=%q err=%v", status, err)
	}

	// The duplicated owner id is checked by a composite FK, not trusted.
	if err := db.Exec(insert, other, conversation.ID, 3, 4, uuid.New(), "wrong owner").Error; err == nil {
		t.Fatal("mismatched conversation owner must be rejected")
	}
	if err := db.Exec(insert, owner, conversation.ID, 5, 4, uuid.New(), "bad range").Error; err == nil {
		t.Fatal("reversed sequence range must be rejected")
	}
	if err := db.Exec(`INSERT INTO episodic_notes
        (user_id, conversation_id, from_seq, to_seq, source_message_ids,
        generator_version, retention_policy_version, content_text, status)
        VALUES (?, ?, 5, 6, ARRAY[?::uuid], 'g', 'p', 'no serving', 'available')`,
		owner, conversation.ID, uuid.New()).Error; err == nil {
		t.Fatal("runtime serving must remain disabled at schema level")
	}
	if err := db.Exec(insert, owner, conversation.ID, 5, 6, uuid.New(), strings.Repeat("x", 4097)).Error; err == nil {
		t.Fatal("overlong notes must be rejected")
	}

	// No chat-row FK: pruning the source may not be blocked or triggered by
	// a shadow note. S2 must verify input ids through Go before writing.
	var chatFKs int64
	if err := db.Raw(`SELECT count(*) FROM pg_constraint
      WHERE contype='f' AND conrelid='public.episodic_notes'::regclass
      AND confrelid='public.messages'::regclass`).Scan(&chatFKs).Error; err != nil || chatFKs != 0 {
		t.Fatalf("episodic note cannot FK-protect raw messages: count=%d err=%v", chatFKs, err)
	}

	var sourceColumnCount int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.columns
      WHERE table_schema='public' AND table_name='episodic_notes'
      AND column_name='source_message_ids' AND udt_name='_uuid'`).Scan(&sourceColumnCount).Error; err != nil || sourceColumnCount != 1 {
		t.Fatalf("source UUID provenance array changed without inventory update: %d %v", sourceColumnCount, err)
	}

	// Account erasure deletes the Conversation and its controlled copies.
	if err := db.Exec("DELETE FROM users WHERE id = ?", owner).Error; err != nil {
		t.Fatal(err)
	}
	var remaining int64
	if err := db.Raw("SELECT count(*) FROM episodic_notes WHERE user_id = ?", owner).Scan(&remaining).Error; err != nil || remaining != 0 {
		t.Fatalf("account erasure leaked episodic note: remaining=%d err=%v", remaining, err)
	}
}
