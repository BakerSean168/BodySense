package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type resumeFixture struct {
	db          *gorm.DB
	owner       uuid.UUID
	envelope    *service.RunEnvelope
	interaction *model.AgentInteraction
	resumes     *service.InteractionResumeService
	events      *service.RuntimeEventService
}

func newResumeFixture(t *testing.T) resumeFixture {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL with native migrations")
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
	owner := uuid.New()
	if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES (?,?,?)", owner, "resume-"+owner.String()+"@example.test", "synthetic-hash").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Exec("DELETE FROM users WHERE id = ?", owner).Error; err != nil {
			t.Error(err)
		}
	})
	repo := repository.NewConsultationRepository(db, "resume-test-owner")
	conversationRepo := repository.NewConversationRepository(db)
	consultations := service.NewConsultationService(repo, conversationRepo)
	envelope, err := consultations.CreateRunEnvelope(context.Background(), owner, nil, uuid.NewString(), datatypes.JSON(`[{"type":"text","text":"A synthetic ambiguous source"}]`), datatypes.JSON(`{}`), "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	run := envelope.Run
	now := time.Now().UTC()
	expiry := now.Add(time.Hour)
	metadata, _ := json.Marshal(map[string]string{"runtime_interrupt_id": strings.ReplaceAll(uuid.NewString(), "-", "")})
	interaction := &model.AgentInteraction{ID: uuid.New(), RunID: run.ID, ConversationID: run.ConversationID, ToolCallID: "ask-test", ToolName: "ask_user", Question: datatypes.JSON(`{"question":"What are you experiencing yourself?","answer_type":"text","allow_custom_input":true}`), Status: model.AgentInteractionPending, Metadata: datatypes.JSON(metadata), ExpiresAt: &expiry}
	if err := db.Create(interaction).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(run).Updates(map[string]any{"status": model.RunStatusWaitingUser, "agent_configuration_id": service.BodyFactConsultationConfigurationID, "lease_owner": "", "lease_expires_at": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Message{}).Where("id = ?", envelope.AssistantMessage.ID).Update("status", "aborted").Error; err != nil {
		t.Fatal(err)
	}
	ids, _ := json.Marshal(map[string]string{"conversation_id": run.ConversationID.String(), "run_id": run.ID.String(), "turn_id": run.TurnID.String()})
	if err := db.Create(&model.RuntimeEvent{ConversationID: run.ConversationID, RunID: run.ID, TurnID: &run.TurnID, Seq: 10, Channel: "stream", Type: "stream.done", Source: "go", Replayable: true, IDs: datatypes.JSON(ids), Payload: datatypes.JSON(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	events := service.NewRuntimeEventService(repository.NewRuntimeEventRepository(db))
	resumes := service.NewInteractionResumeService(repo, database.NewTransactionManager(db), events)
	return resumeFixture{db: db, owner: owner, envelope: envelope, interaction: interaction, resumes: resumes, events: events}
}

func (f resumeFixture) command(text string) model.ClaimInteractionResume {
	answer, _ := json.Marshal(map[string]string{"text": text})
	return model.ClaimInteractionResume{UserID: f.owner, ConversationID: f.interaction.ConversationID, InteractionID: f.interaction.ID, RequestID: uuid.NewString(), Answer: datatypes.JSON(answer)}
}
func resumeCount(t *testing.T, f resumeFixture, table string, expected int64) {
	t.Helper()
	var count int64
	query := f.db.Table(table)
	if table == "messages" || table == "runtime_events" {
		query = query.Where("conversation_id = ?", f.interaction.ConversationID)
	} else {
		query = query.Where("user_id = ?", f.owner)
	}
	if err := query.Count(&count).Error; err != nil || count != expected {
		t.Fatalf("%s count=%d wanted=%d err=%v", table, count, expected, err)
	}
}
func TestInteractionResumeSameRunAndAtomicMilestonesPostgres(t *testing.T) {
	f := newResumeFixture(t)
	command := f.command("I have no shoulder pain")
	first, err := f.resumes.Claim(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	e := first.Envelope
	if e.Existed || e.Run.ID != f.envelope.Run.ID || e.Run.TurnID != f.envelope.TurnID || e.Run.Status != model.RunStatusRunning || e.Run.LeaseOwner != "resume-test-owner" || e.Run.LeaseExpiresAt == nil {
		t.Fatal("resume did not continue the original leased Run")
	}
	if e.UserMessage.Seq != 3 || e.AssistantMessage.Seq != 4 || len(first.Events) != 3 || first.Events[0].Seq != 11 || first.Events[2].Seq != 13 {
		t.Fatalf("message/event sequence lost: %#v", first)
	}
	if first.Events[0].Type != "state.interaction.answered" || first.Events[1].Type != "run.resumed" {
		t.Fatal("wrong milestone order")
	}
	resumeCount(t, f, "runs", 1)
	resumeCount(t, f, "messages", 4)
	resumeCount(t, f, "interaction_resumes", 1)
	resumeCount(t, f, "runtime_events", 4)
	second, err := f.resumes.Claim(context.Background(), command)
	if err != nil || !second.Envelope.Existed || len(second.Events) != 0 || second.Envelope.Receipt.UserMessageID != e.Receipt.UserMessageID {
		t.Fatalf("retry started again: %v", err)
	}
	command.RequestID = uuid.NewString()
	third, err := f.resumes.Claim(context.Background(), command)
	if err != nil || !third.Envelope.Existed {
		t.Fatalf("same answer on another device was not idempotent: %v", err)
	}
	resumeCount(t, f, "messages", 4)
	resumeCount(t, f, "runtime_events", 4)
}

func TestInteractionResumeConcurrentDuplicateExecutesOncePostgres(t *testing.T) {
	f := newResumeFixture(t)
	command := f.command("I sleep seven hours")
	const workers = 12
	var won atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := f.resumes.Claim(context.Background(), command)
			if err != nil {
				errs <- err
				return
			}
			if !result.Envelope.Existed {
				won.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if won.Load() != 1 {
		t.Fatalf("executions=%d", won.Load())
	}
	resumeCount(t, f, "runs", 1)
	resumeCount(t, f, "messages", 4)
	resumeCount(t, f, "interaction_resumes", 1)
	resumeCount(t, f, "runtime_events", 4)
}

func TestInteractionResumeCompetingAnswersCannotBothWinPostgres(t *testing.T) {
	f := newResumeFixture(t)
	results := make(chan error, 2)
	for _, text := range []string{"I have shoulder pain", "I have no shoulder pain"} {
		command := f.command(text)
		go func() { _, err := f.resumes.Claim(context.Background(), command); results <- err }()
	}
	a, b := <-results, <-results
	if (a == nil) == (b == nil) || (a != nil && !errors.Is(a, model.ErrInteractionResumeConflict)) || (b != nil && !errors.Is(b, model.ErrInteractionResumeConflict)) {
		t.Fatalf("a=%v b=%v", a, b)
	}
	resumeCount(t, f, "messages", 4)
	resumeCount(t, f, "runtime_events", 4)
}

func TestInteractionResumeDenialsLeavePendingAndMessagesUnchangedPostgres(t *testing.T) {
	for _, name := range []string{"foreign", "expired", "cancelled", "wrong_summary", "missing_native", "ordinary_request_collision"} {
		t.Run(name, func(t *testing.T) {
			f := newResumeFixture(t)
			cmd := f.command("I have no shoulder pain")
			switch name {
			case "foreign":
				cmd.UserID = uuid.New()
			case "expired":
				if err := f.db.Model(f.interaction).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				if err := f.db.Model(f.envelope.Run).Update("status", model.RunStatusCancelled).Error; err != nil {
					t.Fatal(err)
				}
			case "wrong_summary":
				cmd.Answer = datatypes.JSON(`{"text":"I have shoulder pain","selected":["no"]}`)
			case "missing_native":
				if err := f.db.Model(f.interaction).Update("metadata", datatypes.JSON(`{}`)).Error; err != nil {
					t.Fatal(err)
				}
			case "ordinary_request_collision":
				cmd.RequestID = f.envelope.Run.RequestID
			}
			result, err := f.resumes.Claim(context.Background(), cmd)
			if err == nil || result != nil {
				t.Fatal("invalid resume accepted")
			}
			var i model.AgentInteraction
			if err := f.db.First(&i, "id = ?", f.interaction.ID).Error; err != nil {
				t.Fatal(err)
			}
			if i.Status != model.AgentInteractionPending || i.AnsweredAt != nil {
				t.Fatal("denied answer closed the interaction")
			}
			resumeCount(t, f, "messages", 2)
			resumeCount(t, f, "interaction_resumes", 0)
			resumeCount(t, f, "runtime_events", 1)
		})
	}
}

func TestInteractionResumeLastMilestoneFailureRollsBackAllWritesPostgres(t *testing.T) {
	f := newResumeFixture(t)
	name := "resume-fail-" + uuid.NewString()
	if err := f.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if event, ok := tx.Statement.Dest.(*model.RuntimeEvent); ok && event.Type == "message.created" {
			tx.AddError(errors.New("synthetic last event failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.db.Callback().Create().Remove(name) })
	cmd := f.command("I feel well")
	result, err := f.resumes.Claim(context.Background(), cmd)
	if err == nil || result != nil {
		t.Fatal("failed transaction returned receipt")
	}
	resumeCount(t, f, "messages", 2)
	resumeCount(t, f, "interaction_resumes", 0)
	resumeCount(t, f, "runtime_events", 1)
	var i model.AgentInteraction
	var r model.Run
	if err := f.db.First(&i, "id = ?", f.interaction.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.First(&r, "id = ?", f.envelope.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if i.Status != model.AgentInteractionPending || r.Status != model.RunStatusWaitingUser {
		t.Fatal("answer/lease escaped rollback")
	}
	_ = f.db.Callback().Create().Remove(name)
	if _, err := f.resumes.Claim(context.Background(), cmd); err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}
}

func TestInteractionResumeAnswerEvidenceSurvivesSourcePruningPostgres(t *testing.T) {
	f := newResumeFixture(t)
	accepted, err := f.resumes.Claim(context.Background(), f.command("I have no shoulder pain"))
	if err != nil {
		t.Fatal(err)
	}
	e := accepted.Envelope
	source := model.AnswerFactSource{RunID: e.Run.ID, InteractionID: e.Interaction.ID, MessageID: e.UserMessage.ID}
	command := model.RecordBodyFactsCommand{IdempotencyKey: "accepted-answer", ExpectedRevision: 0, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "I have no shoulder pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"absent"}`)}}}
	repo := repository.NewBodyStateRepository(f.db)
	// An answer cannot pass through the unbound ordinary-message path.
	if receipt, err := repo.RecordMessageFacts(context.Background(), f.owner, model.MessageFactSource{RunID: e.Run.ID, MessageID: e.UserMessage.ID}, command); err == nil || receipt != nil {
		t.Fatal("structured answer accepted without resume source")
	}
	wrong := source
	wrong.MessageID = f.envelope.UserMessage.ID
	if receipt, err := repo.RecordAnswerFacts(context.Background(), f.owner, wrong, command); err == nil || receipt != nil {
		t.Fatal("initial message substituted for accepted answer")
	}
	first, err := repo.RecordAnswerFacts(context.Background(), f.owner, source, command)
	if err != nil {
		t.Fatal(err)
	}
	var evidence model.StatementEvidence
	if err := f.db.First(&evidence, "user_id = ?", f.owner).Error; err != nil {
		t.Fatal(err)
	}
	if evidence.SourceKind != "structured_answer" || evidence.SourceInteractionID == nil || *evidence.SourceInteractionID != source.InteractionID || evidence.SourceToolCallID == nil || *evidence.SourceToolCallID != "ask-test" || evidence.QuestionExcerpt == nil || evidence.Excerpt != "I have no shoulder pain" {
		t.Fatalf("source context missing: %#v", evidence)
	}
	if err := f.db.Delete(&model.Message{}, "id = ?", source.MessageID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Delete(&model.AgentInteraction{}, "id = ?", source.InteractionID).Error; err != nil {
		t.Fatal(err)
	}
	second, err := repo.RecordAnswerFacts(context.Background(), f.owner, source, command)
	if err != nil || second.ID != first.ID {
		t.Fatalf("committed fact lost idempotency after pruning: %v", err)
	}
	resumeCount(t, f, "statement_evidence", 1)
	resumeCount(t, f, "body_facts", 1)
	resumeCount(t, f, "body_state_revisions", 1)
	command.IdempotencyKey = "cannot-reuse-pruned-source"
	command.ExpectedRevision = 1
	if receipt, err := repo.RecordAnswerFacts(context.Background(), f.owner, source, command); err == nil || receipt != nil {
		t.Fatal("new write accepted from erased raw source")
	}
}

func TestInteractionResumeSourceRejectsTamperedQuestionAndAnswerPostgres(t *testing.T) {
	for _, target := range []string{"question", "answer", "message", "other_user"} {
		t.Run(target, func(t *testing.T) {
			f := newResumeFixture(t)
			accepted, err := f.resumes.Claim(context.Background(), f.command("I have no shoulder pain"))
			if err != nil {
				t.Fatal(err)
			}
			e := accepted.Envelope
			owner := f.owner
			source := model.AnswerFactSource{RunID: e.Run.ID, InteractionID: e.Interaction.ID, MessageID: e.UserMessage.ID}
			switch target {
			case "question":
				err = f.db.Model(f.interaction).Update("question", datatypes.JSON(`{"question":"Changed prompt","answer_type":"text"}`)).Error
			case "answer":
				err = f.db.Model(f.interaction).Update("answer", datatypes.JSON(`{"text":"I have shoulder pain"}`)).Error
			case "message":
				err = f.db.Model(&model.Message{}).Where("id = ?", source.MessageID).Update("parts", datatypes.JSON(`[{"type":"text","text":"I have shoulder pain"}]`)).Error
			case "other_user":
				owner = uuid.New()
			}
			if err != nil {
				t.Fatal(err)
			}
			command := model.RecordBodyFactsCommand{IdempotencyKey: "tamper", ExpectedRevision: 0, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "I have no shoulder pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"absent"}`)}}}
			if receipt, err := repository.NewBodyStateRepository(f.db).RecordAnswerFacts(context.Background(), owner, source, command); err == nil || receipt != nil {
				t.Fatal("tampered source accepted")
			}
			resumeCount(t, f, "body_facts", 0)
			resumeCount(t, f, "body_state_revisions", 0)
		})
	}
}

func TestInteractionResumeDowngradeRefusesAcceptedHistoryPostgres(t *testing.T) {
	f := newResumeFixture(t)
	if _, err := f.resumes.Claim(context.Background(), f.command("I feel well")); err != nil {
		t.Fatal(err)
	}
	resumeCount(t, f, "interaction_resumes", 1)
}

func TestInteractionResumeRequestCannotCreateAnOrdinaryTurnPostgres(t *testing.T) {
	f := newResumeFixture(t)
	command := f.command("I feel well")
	if _, err := f.resumes.Claim(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(f.envelope.Run).Update("status", model.RunStatusCompleted).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.Conversation{}).Where("id = ?", f.interaction.ConversationID).Updates(map[string]any{"active_run_id": nil, "active_stream_id": ""}).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewConsultationRepository(f.db)
	_, _, _, _, _, _, err := repo.CreateRunEnvelope(context.Background(), f.owner, &f.interaction.ConversationID, command.RequestID, datatypes.JSON(`[{"type":"text","text":"different command"}]`), datatypes.JSON(`{}`), "synthetic")
	if !errors.Is(err, model.ErrInteractionResumeConflict) {
		t.Fatalf("rebound resume request accepted: %v", err)
	}
	resumeCount(t, f, "runs", 1)
	resumeCount(t, f, "messages", 4)
}
