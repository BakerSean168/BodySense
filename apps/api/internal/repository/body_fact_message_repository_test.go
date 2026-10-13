package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Mirrors the real consultation envelope: user Message.RunID is nil; both the
// user message and the run share the same stable TurnID/ConversationID.
func seedUserMessageFactEnvelope(t *testing.T, db *gorm.DB, userID uuid.UUID, parts datatypes.JSON, role string) model.MessageFactSource {
	t.Helper()
	conversation := model.Conversation{ID: uuid.New(), UserID: userID, Status: "active", TitleStatus: "pending"}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	run := model.Run{ID: uuid.New(), UserID: userID, ConversationID: conversation.ID, TurnID: uuid.New(), RequestID: uuid.NewString(), Model: "synthetic", Status: model.RunStatusRunning}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	message := model.Message{ID: uuid.New(), ConversationID: conversation.ID, TurnID: run.TurnID, Role: role, Status: "completed", Seq: 1, Parts: parts}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	if message.RunID != nil {
		t.Fatal("a user message should not carry a RunID in the current envelope")
	}
	return model.MessageFactSource{RunID: run.ID, MessageID: message.ID}
}

func messageFactTestCommand() model.RecordBodyFactsCommand {
	return model.RecordBodyFactsCommand{IdempotencyKey: "message-capture-1", ExpectedRevision: 0, Facts: []model.BodyFactInput{
		{Kind: "symptom", Statement: "My shoulder hurts", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present"}`)},
		{Kind: "lifestyle", Statement: "I sleep seven hours", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"habit_type":"sleep"}`)},
	}}
}

func TestBodyFactMessageVerbatimSourceAndDurableReceiptPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	text := "My shoulder hurts, and I sleep seven hours."
	parts, _ := json.Marshal([]map[string]any{{"type": "text", "text": text}})
	source := seedUserMessageFactEnvelope(t, db, owner, datatypes.JSON(parts), "user")
	repo := NewBodyStateRepository(db)
	command := messageFactTestCommand()

	first, err := repo.RecordMessageFacts(context.Background(), owner, source, command)
	if err != nil || first == nil || first.Revision != 1 {
		t.Fatalf("first receipt=%#v err=%v", first, err)
	}
	assertBodyFactCount(t, db, "body_facts", owner, 2)
	assertBodyFactCount(t, db, "body_state_outbox", owner, 1)
	assertBodyFactCount(t, db, "body_state_revisions", owner, 1)
	var evidence []model.StatementEvidence
	if err := db.Where("user_id = ?", owner).Find(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 2 {
		t.Fatalf("evidence rows=%d", len(evidence))
	}
	for _, sourceRow := range evidence {
		if sourceRow.SourceKind != "user_message" || sourceRow.SourceVersion != 1 ||
			sourceRow.SourceMessageID == nil || *sourceRow.SourceMessageID != source.MessageID ||
			sourceRow.SourceRunID == nil || *sourceRow.SourceRunID != source.RunID ||
			sourceRow.SourceConversationID == nil || sourceRow.Excerpt == text {
			t.Fatalf("bad source evidence binding or leaked whole message: %#v", sourceRow)
		}
	}
	var revision model.BodyStateRevision
	var event model.BodyStateOutbox
	if err := db.First(&revision, "user_id = ?", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&event, "user_id = ?", owner).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(revision.Changes), "My shoulder") || strings.Contains(string(event.Payload), "My shoulder") {
		t.Fatal("health statement leaked into generic audit/outbox payload")
	}
	// Chat message pruning must not orphan the previously persisted minimum
	// evidence; an exact idempotent retry replays the original committed result.
	if err := db.Delete(&model.Message{}, "id = ?", source.MessageID).Error; err != nil {
		t.Fatal(err)
	}
	replay, err := repo.RecordMessageFacts(context.Background(), owner, source, command)
	if err != nil || replay == nil || replay.ID != first.ID {
		t.Fatalf("pruned replay=%#v err=%v", replay, err)
	}
	page, err := repo.ListFacts(context.Background(), owner, 0)
	if err != nil || len(page.Facts) != 2 {
		t.Fatalf("projection after pruning: %#v %v", page, err)
	}
	assertBodyFactCount(t, db, "statement_evidence", owner, 2)
	assertBodyFactCount(t, db, "body_state_revisions", owner, 1)
}

func TestBodyFactMessageRejectsForgedSourcesPostgres(t *testing.T) {
	cases := []struct {
		name       string
		sourceRole string
		sourceText string
		mutate     func(*testing.T, *gorm.DB, uuid.UUID, *model.MessageFactSource, *model.RecordBodyFactsCommand)
	}{
		{"assistant_message", "assistant", "My shoulder hurts, and I sleep seven hours.", nil},
		{"not_verbatim", "user", "I have no reported issues.", nil},
		{"no_text", "user", "", nil},
		{"wrong_run", "user", "My shoulder hurts, and I sleep seven hours.", func(t *testing.T, db *gorm.DB, _ uuid.UUID, source *model.MessageFactSource, _ *model.RecordBodyFactsCommand) {
			source.RunID = uuid.New()
		}},
		{"wrong_turn", "user", "My shoulder hurts, and I sleep seven hours.", func(t *testing.T, db *gorm.DB, _ uuid.UUID, source *model.MessageFactSource, _ *model.RecordBodyFactsCommand) {
			if err := db.Model(&model.Message{}).Where("id = ?", source.MessageID).Update("turn_id", uuid.New()).Error; err != nil {
				t.Fatal(err)
			}
		}},
		{"unrelated_claim", "user", "My shoulder hurts, and I sleep seven hours.", func(t *testing.T, db *gorm.DB, _ uuid.UUID, _ *model.MessageFactSource, command *model.RecordBodyFactsCommand) {
			command.Facts[1].Statement = "The agent diagnosed a condition"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openBodyStateContextIntegrationDB(t)
			owner := seedBodyStateContextUser(t, db)
			parts, _ := json.Marshal([]map[string]any{{"type": "text", "text": tc.sourceText}})
			source := seedUserMessageFactEnvelope(t, db, owner, datatypes.JSON(parts), tc.sourceRole)
			command := messageFactTestCommand()
			command.IdempotencyKey = "forged-" + tc.name
			if tc.mutate != nil {
				tc.mutate(t, db, owner, &source, &command)
			}
			receipt, err := NewBodyStateRepository(db).RecordMessageFacts(context.Background(), owner, source, command)
			if !errors.Is(err, model.ErrInvalidBodyFact) || receipt != nil {
				t.Fatalf("forged source accepted: receipt=%#v err=%v", receipt, err)
			}
			for _, table := range []string{"body_facts", "statement_evidence", "body_fact_commits", "body_state_revisions", "body_state_outbox"} {
				assertBodyFactCount(t, db, table, owner, 0)
			}
		})
	}
}

func TestBodyFactMessageRejectsOtherOwnerAndMismatchedReplayPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	parts := datatypes.JSON(`[{"type":"text","text":"My shoulder hurts, and I sleep seven hours."}]`)
	source := seedUserMessageFactEnvelope(t, db, owner, parts, "user")
	repo := NewBodyStateRepository(db)
	command := messageFactTestCommand()
	command.IdempotencyKey = "shared-key"
	if receipt, err := repo.RecordMessageFacts(context.Background(), other, source, command); !errors.Is(err, model.ErrInvalidBodyFact) || receipt != nil {
		t.Fatalf("other owner may not bind source: %#v %v", receipt, err)
	}
	assertBodyFactCount(t, db, "body_state_revisions", other, 0)
	first, err := repo.RecordMessageFacts(context.Background(), owner, source, command)
	if err != nil {
		t.Fatal(err)
	}
	secondSource := seedUserMessageFactEnvelope(t, db, owner, parts, "user")
	if receipt, err := repo.RecordMessageFacts(context.Background(), owner, secondSource, command); !errors.Is(err, model.ErrBodyFactIdempotencyConflict) || receipt != nil {
		t.Fatalf("reused key with another run/message accepted: %#v %v", receipt, err)
	}
	// Reusing a direct-edit idempotency key cannot impersonate a message source.
	if receipt, err := repo.RecordDirectFacts(context.Background(), owner, command); !errors.Is(err, model.ErrBodyFactIdempotencyConflict) || receipt != nil {
		t.Fatalf("direct source reused message receipt: %#v %v", receipt, err)
	}
	page, err := repo.ListFacts(context.Background(), owner, 0)
	if err != nil || len(page.Facts) != 2 || first.Revision != 1 {
		t.Fatalf("projection=%#v err=%v", page, err)
	}
	if _, err := repo.RecordMessageFacts(context.Background(), owner, source, model.RecordBodyFactsCommand{
		IdempotencyKey: fmt.Sprintf("stale-%s", uuid.NewString()), ExpectedRevision: 0, Facts: command.Facts,
	}); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatalf("stale revision must fail: %v", err)
	}
	assertBodyFactCount(t, db, "body_state_revisions", owner, 1)
}
