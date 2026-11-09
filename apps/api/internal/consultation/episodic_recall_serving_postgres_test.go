package consultation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/episodicrecall"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This is the S4 boundary-crossing reproduction: a real consultation turn
// reads a real shadow note from PostgreSQL and the ai-service stand-in captures
// the exact StartConsultationTurn JSON that Go would send. The unit test in
// episodic_recall_context_test.go only proves the in-memory Go struct, which is
// how PR #542 passed while staging served nothing: the Proto projection between
// the Go struct and the HTTP payload dropped episodic_notes.
func TestEpisodicRecallServedNoteReachesConsultationPayloadPostgres(t *testing.T) {
	db := openEpisodicServingTestDB(t)
	owner, conversationID := seedEpisodicServingOwner(t, db)
	note := seedShadowNote(t, db, owner, conversationID, "用户提到周末去了早市买菜。")
	query := "你还记得我上次说的早市买菜吗"

	recaller := episodicrecall.NewRecaller(repository.NewEpisodicRecallRepository(db))
	served := recaller.Recall(context.Background(), owner, query)
	if len(served) != 1 || served[0].ID != note.ID {
		t.Fatalf("real PostgreSQL recall served %d notes (%+v), want the seeded note", len(served), served)
	}

	payload := captureStartTurnPayload(t, db, recaller, owner, conversationID, query)
	var command struct {
		BusinessContext struct {
			EpisodicNotes []service.ConsultationEpisodicNote `json:"episodic_notes"`
		} `json:"business_context"`
	}
	if err := json.Unmarshal(payload, &command); err != nil {
		t.Fatalf("decode captured StartTurnCommand: %v\n%s", err, payload)
	}
	if len(command.BusinessContext.EpisodicNotes) != 1 {
		t.Fatalf("served note did not cross the Go->Python boundary: episodic_notes=%d\npayload=%s",
			len(command.BusinessContext.EpisodicNotes), payload)
	}
	got := command.BusinessContext.EpisodicNotes[0]
	if got.NoteID != note.ID.String() || got.Text != note.ContentText ||
		got.Authority != service.ConsultationEpisodicNoteAuthority {
		t.Fatalf("payload note lost its identity or authority: %+v", got)
	}
	if got.FromSeq != note.FromSeq || got.ToSeq != note.ToSeq ||
		len(got.SourceMessageIDs) != len(note.SourceMessageIDs) {
		t.Fatalf("payload note lost its provenance: %+v", got)
	}
}

// The flag-off payload must stay exactly as before S4: no query and no
// episodic_notes key. The same real turn path and database are used.
func TestEpisodicRecallFlagOffKeepsPayloadWithoutEpisodicNotesPostgres(t *testing.T) {
	db := openEpisodicServingTestDB(t)
	owner, conversationID := seedEpisodicServingOwner(t, db)
	seedShadowNote(t, db, owner, conversationID, "用户提到周末去了早市买菜。")

	payload := captureStartTurnPayload(t, db, nil, owner, conversationID, "你还记得我上次说的早市买菜吗")
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode captured StartTurnCommand: %v\n%s", err, payload)
	}
	var businessContext map[string]json.RawMessage
	if err := json.Unmarshal(decoded["business_context"], &businessContext); err != nil {
		t.Fatalf("decode business_context: %v", err)
	}
	if raw, present := businessContext["episodic_notes"]; present {
		t.Fatalf("flag-off payload carried episodic_notes: %s", raw)
	}
}

func captureStartTurnPayload(
	t *testing.T,
	db *gorm.DB,
	recaller *episodicrecall.Recaller,
	owner uuid.UUID,
	conversationID uuid.UUID,
	query string,
) []byte {
	t.Helper()

	payloads := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		payloads <- body
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_SERVICE_URL", server.URL)

	deployment, err := service.NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("build deployment policy: %v", err)
	}
	runtime := NewRuntime(
		nil,
		nil,
		service.NewProfileService(repository.NewProfileRepository(db)),
		nil,
		nil,
		service.NewAIClient(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		deployment,
	)
	runtime.AttachEpisodicRecall(recaller)

	runID, turnID := uuid.New(), uuid.New()
	baseIDs := dto.StreamEventIDs{
		ConversationID: conversationID.String(),
		RunID:          runID.String(),
		TurnID:         turnID.String(),
	}
	recorder := httptest.NewRecorder()
	streamWriter := runtime.streamRuntime.NewWriter(recorder, baseIDs)
	session := &model.ConsultationSession{Phase: model.ConsultationPhaseCollecting}
	runtime.executeRunFlow(
		context.Background(),
		streamWriter,
		owner,
		conversationID,
		turnID,
		&model.Run{ID: runID},
		&model.Message{ID: uuid.New()},
		baseIDs,
		query,
		uuid.New(),
		nil,
		session,
		nil,
	)

	select {
	case payload := <-payloads:
		return payload
	case <-time.After(5 * time.Second):
		t.Fatal("ai-service stand-in captured no StartConsultationTurn request")
		return nil
	}
}

func openEpisodicServingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL to run episodic recall serving integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func seedEpisodicServingOwner(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	owner := uuid.New()
	if err := db.Exec("INSERT INTO users(id,email) VALUES(?,?)", owner, "episodic-serving-"+owner.String()+"@example.test").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Exec("DELETE FROM users WHERE id = ?", owner).Error; err != nil {
			t.Error(err)
		}
	})
	conversation, _, err := repository.NewConversationRepository(db).GetOrCreateForUser(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	return owner, conversation.ID
}

func seedShadowNote(t *testing.T, db *gorm.DB, owner, conversationID uuid.UUID, text string) model.EpisodicNote {
	t.Helper()
	now := time.Now().UTC()
	note := model.EpisodicNote{
		ID:                     uuid.New(),
		UserID:                 owner,
		ConversationID:         conversationID,
		FromSeq:                1,
		ToSeq:                  2,
		SourceMessageIDs:       pq.StringArray{uuid.NewString(), uuid.NewString()},
		SourceAssetIDs:         pq.StringArray{},
		GeneratorVersion:       "synthetic-serving-v1",
		RetentionPolicyVersion: "synthetic-serving-policy-v1",
		ContentText:            text,
		Status:                 "shadow",
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := db.Create(&note).Error; err != nil {
		t.Fatalf("seed shadow note: %v", err)
	}
	return note
}
