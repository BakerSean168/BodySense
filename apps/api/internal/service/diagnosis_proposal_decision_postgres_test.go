package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type diagnosisDecisionFixture struct {
	owner     uuid.UUID
	run       model.ConditionAnalysisRun
	proposals []model.DiagnosisProposal
	fact      model.BodyFact
	screen    model.SafetyScreen
	request   model.StatementEvidence
}

// Only synthetic, explicit user-originated inputs. The application under test
// records decisions; a receipt is deliberately not a Diagnosis adoption.
func seedDiagnosisDecisionFixture(t *testing.T, db *gorm.DB) diagnosisDecisionFixture {
	t.Helper()
	ctx := context.Background()
	f := diagnosisDecisionFixture{owner: seedOnboardingContextUser(t, db)}
	if _, err := repository.NewDiagnosisRepository(db).EnsureAggregate(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	commit, err := repository.NewBodyStateRepository(db).RecordDirectFacts(ctx, f.owner, model.RecordBodyFactsCommand{
		IdempotencyKey: uuid.NewString(), Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "Synthetic user reports knee discomfort", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var factIDs []uuid.UUID
	if err = json.Unmarshal(commit.FactIDs, &factIDs); err != nil {
		t.Fatal(err)
	}
	if err = db.Where("user_id=? AND id=?", f.owner, factIDs[0]).Take(&f.fact).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	conversation := model.Conversation{ID: uuid.New(), UserID: f.owner}
	if err = db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	chat := model.Run{ID: uuid.New(), UserID: f.owner, ConversationID: conversation.ID, TurnID: uuid.New(), RequestID: uuid.NewString(), Model: "synthetic", Status: model.RunStatusCompleted}
	if err = db.Create(&chat).Error; err != nil {
		t.Fatal(err)
	}
	message := model.Message{ID: uuid.New(), ConversationID: conversation.ID, TurnID: chat.TurnID, Role: "user", Seq: 1, Parts: datatypes.JSON(`[{"type":"text","text":"Please formally analyze the recorded symptoms"}]`)}
	if err = db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	f.request = model.StatementEvidence{ID: uuid.New(), UserID: f.owner, SourceKind: "user_message", SourceVersion: 1, SourceConversationID: &conversation.ID, SourceRunID: &chat.ID, SourceMessageID: &message.ID, Excerpt: "Please formally analyze the recorded symptoms", Availability: "available", RecordedAt: now}
	if err = db.Create(&f.request).Error; err != nil {
		t.Fatal(err)
	}
	f.screen = model.SafetyScreen{ID: uuid.New(), UserID: f.owner, CaptureID: "0123456789abcdef01234567", BodyFactID: f.fact.ID, SourceConversationID: conversation.ID, SourceRunID: chat.ID, SourceMessageID: message.ID, SourceInteractionID: uuid.New(), SourceToolCallID: "intake-0123456789abcdef01234567", BodyStateRevision: 1, RecordedAt: now}
	if err = db.Create(&f.screen).Error; err != nil {
		t.Fatal(err)
	}
	refs := []model.DiagnosisEvidenceRef{{EvidenceType: model.DiagnosisBodyFact, EvidenceID: f.fact.ID, Version: "1"}}
	f.run = model.ConditionAnalysisRun{UserID: f.owner, ID: uuid.New(), RequestMessageID: message.ID, RequestEvidenceID: f.request.ID, RequestSourceVersion: 1, Trigger: model.ConditionNewAnalysis, Scope: model.ConditionAnalysisScope{ConditionIDs: []uuid.UUID{}, EvidenceRefs: refs, Description: "Synthetic formal request"}, BasedOnBodyStateRevision: 1, InputEvidenceRefs: refs, AgentHypotheses: []model.ConditionAgentHypothesis{}, ReplayInput: datatypes.JSON(`{"frozen":true}`), ExecutionProvenance: datatypes.JSON(`{"provider":"synthetic"}`), IdempotencyKey: uuid.NewString(), RequestFingerprint: strings.Repeat("a", 64), StartedAt: now}
	if err = db.Create(&f.run).Error; err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		p := model.DiagnosisProposal{UserID: f.owner, ID: uuid.New(), RunID: f.run.ID, ChangeKind: model.DiagnosisCreate, Content: datatypes.JSON(`{"proposedEntry":{"medicalName":"Synthetic named syndrome"}}`), ContentFingerprint: strings.Repeat(string(rune('b'+i)), 64), EvidenceRefs: refs, CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: now}
		if err = db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		f.proposals = append(f.proposals, p)
	}
	return f
}

func diagnosisDecisionCommand(f diagnosisDecisionFixture, index int) DiagnosisProposalDecisionCommand {
	p := f.proposals[index]
	return DiagnosisProposalDecisionCommand{RunID: f.run.ID, ProposalID: p.ID, PresentedFingerprint: p.ContentFingerprint, Decision: model.DiagnosisAccept, ToolCallID: "explicit-user-confirmation", IdempotencyKey: uuid.NewString()}
}
func diagnosisDecisionCount(t *testing.T, db *gorm.DB, owner uuid.UUID, table string, want int64) {
	t.Helper()
	var got int64
	if err := db.Table(table).Where("user_id=?", owner).Count(&got).Error; err != nil || got != want {
		t.Fatalf("%s: got=%d want=%d err=%v", table, got, want, err)
	}
}
func assertDiagnosisNotAdopted(t *testing.T, db *gorm.DB, owner uuid.UUID) {
	t.Helper()
	for _, table := range []string{"condition_entries", "diagnosis_revisions"} {
		diagnosisDecisionCount(t, db, owner, table, 0)
	}
	var head model.Diagnosis
	if err := db.Where("user_id=?", owner).Take(&head).Error; err != nil || head.CurrentRevision != 0 {
		t.Fatalf("decision changed Diagnosis: %+v %v", head, err)
	}
}

func TestDiagnosisProposalDecisionPartialAndReplayPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalDecisionService(db)
	command := diagnosisDecisionCommand(f, 0)
	first, err := svc.Record(context.Background(), f.owner, command)
	if err != nil || first == nil || first.Decision != model.DiagnosisAccept || first.ReceiptID == uuid.Nil {
		t.Fatalf("decision: %+v %v", first, err)
	}
	assertDiagnosisNotAdopted(t, db, f.owner)
	second := diagnosisDecisionCommand(f, 1)
	second.Decision = model.DiagnosisReject
	if _, err = svc.Record(context.Background(), f.owner, second); err != nil {
		t.Fatal(err)
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 2)
	diagnosisDecisionCount(t, db, f.owner, "condition_analysis_events", 2)
	// A lost response retry is a read of its immutable receipt, even when a
	// source later becomes unavailable. It never repeats an adoption/mutation.
	if err = db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.request.ID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	replayed, err := svc.Record(context.Background(), f.owner, command)
	if err != nil || replayed.ID != first.ID || replayed.ReceiptID != first.ReceiptID {
		t.Fatalf("replay: %+v %v", replayed, err)
	}
	changed := command
	changed.Decision = model.DiagnosisReject
	if _, err = svc.Record(context.Background(), f.owner, changed); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("changed idempotency body: %v", err)
	}
	diagnosisDecisionCount(t, db, f.owner, "condition_analysis_events", 2)
	assertDiagnosisNotAdopted(t, db, f.owner)
}

func TestDiagnosisProposalDecisionConcurrentPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalDecisionService(db)
	command := diagnosisDecisionCommand(f, 0)
	var wg sync.WaitGroup
	results := make(chan *model.DiagnosisDecision, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { r, e := svc.Record(context.Background(), f.owner, command); results <- r; errs <- e })
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var id uuid.UUID
	for r := range results {
		if r == nil {
			t.Fatal("nil receipt")
		}
		if id == uuid.Nil {
			id = r.ID
		}
		if r.ID != id {
			t.Fatal("duplicate decision")
		}
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 1)
	diagnosisDecisionCount(t, db, f.owner, "condition_analysis_events", 1)
	other := command
	other.IdempotencyKey = uuid.NewString()
	other.Decision = model.DiagnosisReject
	if _, err := svc.Record(context.Background(), f.owner, other); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("second conflicting choice: %v", err)
	}
	assertDiagnosisNotAdopted(t, db, f.owner)
}

func TestDiagnosisProposalDecisionIdentityAndSourceGuardsPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalDecisionService(db)
	ctx := context.Background()
	command := diagnosisDecisionCommand(f, 0)
	foreign := seedOnboardingContextUser(t, db)
	if _, err := svc.Record(ctx, foreign, command); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatalf("foreign proposal: %v", err)
	}
	for _, kind := range []string{"fingerprint", "revision", "run", "target"} {
		bad := command
		switch kind {
		case "fingerprint":
			bad.PresentedFingerprint = strings.Repeat("e", 64)
		case "revision":
			bad.ExpectedRevision = 1
		case "run":
			bad.RunID = uuid.New()
		case "target":
			bad.ProposalID = uuid.New()
		}
		if r, err := svc.Record(ctx, f.owner, bad); err == nil || r != nil {
			t.Fatalf("accepted invalid %s", kind)
		}
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.fact.SourceEvidenceID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	// The same source supplies safety coverage, so its disappearance blocks
	// at the earlier safety read rather than returning a successful decision.
	if _, err := svc.Record(ctx, f.owner, command); !errors.Is(err, ErrDiagnosisSafetyUnavailable) {
		t.Fatalf("blocked evidence must make safety unavailable: %v", err)
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 0)
	diagnosisDecisionCount(t, db, f.owner, "condition_analysis_events", 0)
	assertDiagnosisNotAdopted(t, db, f.owner)
}

func TestDiagnosisProposalDecisionSafetyTerminalPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("missing coverage cannot record", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		if err := db.Where("user_id=?", f.owner).Delete(&model.SafetyScreen{}).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := NewDiagnosisProposalDecisionService(db).Record(ctx, f.owner, diagnosisDecisionCommand(f, 0)); !errors.Is(err, ErrDiagnosisSafetyUnavailable) {
			t.Fatalf("uncovered decision: %v", err)
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 0)
	})
	t.Run("urgent preserves choices but closes run without adoption", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		svc := NewDiagnosisProposalDecisionService(db)
		if _, err := svc.Record(ctx, f.owner, diagnosisDecisionCommand(f, 0)); err != nil {
			t.Fatal(err)
		}
		// The current approved screen policy never classifies urgent. Inject a
		// typed future-policy result only at the internal test seam; retain the
		// real PG18 transaction/event constraints and do not weaken its schema.
		svc.projectSafety = func(snapshot *BodyStateReadModel) (SafetyEnvelopeV2, error) {
			envelope, err := ProjectSafetyEnvelopeV2(snapshot)
			envelope.ActiveBlockers = []SafetyBlockerV1{{RiskLevel: "urgent"}}
			envelope.RequiresReview = true
			return envelope, err
		}
		command := diagnosisDecisionCommand(f, 1)
		if _, err := svc.Record(ctx, f.owner, command); !errors.Is(err, ErrDiagnosisSafetyBlocked) {
			t.Fatalf("urgent decision: %v", err)
		}
		if _, err := svc.Record(ctx, f.owner, command); !errors.Is(err, ErrDiagnosisAnalysisClosed) {
			t.Fatalf("closed run reopened: %v", err)
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 1)
		diagnosisDecisionCount(t, db, f.owner, "condition_analysis_events", 2)
		assertDiagnosisNotAdopted(t, db, f.owner)
	})
}
