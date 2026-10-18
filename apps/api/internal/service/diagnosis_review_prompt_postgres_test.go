package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestDiagnosisReviewPromptDismissalPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f, entries := settleFixture(t, db)
	a := entries[0]
	repo := repository.NewDiagnosisRepository(db)
	reviewed := NewReviewedNativeDiagnosisReader(NewNativeDiagnosisReadService(repo), NewDiagnosisFreshnessService(db)).WithReviewPromptDismissals(repo)
	prompts := NewDiagnosisReviewPromptService(db, reviewed)
	condition := func() NativeDiagnosisCondition {
		t.Helper()
		view, err := reviewed.Read(ctx, f.owner)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range view.ConditionEntries {
			if c.ConditionID == a.ID {
				return c
			}
		}
		t.Fatal("condition missing")
		return NativeDiagnosisCondition{}
	}
	recordFact := func(expected int64, statement string) {
		t.Helper()
		if _, err := repository.NewBodyStateRepository(db).RecordDirectFacts(ctx, f.owner, model.RecordBodyFactsCommand{
			ExpectedRevision: expected, IdempotencyKey: uuid.NewString(),
			Facts: []model.BodyFactInput{{Kind: "symptom", Statement: statement, BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present"}`)}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	cmd := func(evaluated int64) DiagnosisReviewPromptDismissCommand {
		return DiagnosisReviewPromptDismissCommand{ConditionID: a.ID, ExpectedRevision: 1, EvaluatedBodyStateRevision: evaluated, IdempotencyKey: uuid.NewString()}
	}

	// A current Condition has no prompt to dismiss.
	if _, err := prompts.Dismiss(ctx, f.owner, cmd(1)); !errors.Is(err, ErrDiagnosisReviewPromptNotShown) {
		t.Fatalf("current condition dismissed: %v", err)
	}
	recordFact(1, "Synthetic user reports the knee discomfort returned")
	shown := condition()
	if shown.Freshness.State != DiagnosisFreshnessReviewRecommended || shown.ReviewPromptDismissal != nil {
		t.Fatalf("prompt not shown: %+v", shown.Freshness)
	}
	// The user must dismiss exactly what they saw.
	if _, err := prompts.Dismiss(ctx, f.owner, cmd(1)); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("outdated prompt dismissed: %v", err)
	}
	stale := cmd(2)
	stale.ExpectedRevision = 2
	if _, err := prompts.Dismiss(ctx, f.owner, stale); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("wrong diagnosis revision dismissed: %v", err)
	}
	if _, err := prompts.Dismiss(ctx, uuid.New(), cmd(2)); !errors.Is(err, model.ErrDiagnosisNotFound) && err == nil {
		t.Fatalf("foreign owner dismissed: %v", err)
	}

	before := freshnessAuditCounts(t, db, f.owner)
	first := cmd(2)
	receipt, err := prompts.Dismiss(ctx, f.owner, first)
	if err != nil || receipt == nil || receipt.DismissedThroughBodyStateRevision != 2 || receipt.BasedOnBodyStateRevision != shown.Freshness.BasedOnBodyStateRevision ||
		len(receipt.ReasonCodes) == 0 || receipt.DiagnosisRevision != 1 {
		t.Fatalf("dismiss %+v %v", receipt, err)
	}
	replay, err := prompts.Dismiss(ctx, f.owner, first)
	if err != nil || replay.ID != receipt.ID {
		t.Fatalf("replay %+v %v", replay, err)
	}
	reused := first
	reused.EvaluatedBodyStateRevision = 3
	if _, err := prompts.Dismiss(ctx, f.owner, reused); !errors.Is(err, ErrDiagnosisReviewPromptKeyReused) {
		t.Fatalf("key reuse: %v", err)
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_review_prompt_dismissals", 1)
	dismissed := condition()
	if dismissed.ReviewPromptDismissal == nil || dismissed.ReviewPromptDismissal.DismissalID != receipt.ID ||
		dismissed.Freshness.State != DiagnosisFreshnessReviewRecommended {
		t.Fatalf("dismissal not projected or freshness altered: %+v %+v", dismissed.ReviewPromptDismissal, dismissed.Freshness)
	}
	// Dismissing never edits the Diagnosis or its audit.
	for table, n := range before {
		var got int64
		if err := db.Table(table).Where("user_id=?", f.owner).Count(&got).Error; err != nil || got != n {
			t.Fatalf("dismissal wrote %s: %d -> %d (%v)", table, n, got, err)
		}
	}
	if err := db.Exec("UPDATE diagnosis_review_prompt_dismissals SET dismissed_through_body_state_revision=99 WHERE user_id=?", f.owner).Error; err == nil {
		t.Fatal("dismissal is not append-only")
	}

	// New substantive evidence raises the prompt again.
	recordFact(2, "Synthetic user reports the knee now also swells")
	if again := condition(); again.ReviewPromptDismissal != nil || again.Freshness.State != DiagnosisFreshnessReviewRecommended {
		t.Fatalf("new evidence stayed hidden: %+v", again.ReviewPromptDismissal)
	}

	// A stale Condition can never be put aside.
	if _, err := repository.NewBodyStateRepository(db).WithdrawDirectFact(ctx, f.owner, f.fact.ID, model.AmendBodyFactCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := prompts.Dismiss(ctx, f.owner, cmd(4)); !errors.Is(err, ErrDiagnosisReviewPromptNotShown) {
		t.Fatalf("stale condition dismissed: %v", err)
	}
}

func TestDiagnosisReviewPromptCoverage(t *testing.T) {
	rev := func(v int64) *int64 { return &v }
	d := &model.DiagnosisReviewPromptDismissal{BasedOnBodyStateRevision: 1, DismissedThroughBodyStateRevision: 5, ReasonCodes: []string{freshnessChangeWindowExceeded, freshnessRelatedFactRecorded}}
	base := func(reasons ...DiagnosisFreshnessReason) *DiagnosisFreshness {
		return &DiagnosisFreshness{State: DiagnosisFreshnessReviewRecommended, BasedOnBodyStateRevision: 1, EvaluatedBodyStateRevision: 9, Reasons: reasons}
	}
	if !diagnosisReviewPromptDismissed(base(DiagnosisFreshnessReason{Code: freshnessRelatedFactRecorded, BodyStateRevision: rev(5)}, DiagnosisFreshnessReason{Code: freshnessChangeWindowExceeded}), d) {
		t.Fatal("covered prompt shown")
	}
	for name, f := range map[string]*DiagnosisFreshness{
		"newer evidence":       base(DiagnosisFreshnessReason{Code: freshnessSafetyChanged, BodyStateRevision: rev(6)}),
		"unseen unattributed":  base(DiagnosisFreshnessReason{Code: freshnessRelatedEpisodeChanged}),
		"stale cause":          base(DiagnosisFreshnessReason{Code: freshnessCitedEvidenceWithdrawn, BodyStateRevision: rev(2)}),
		"new analysis basis":   {State: DiagnosisFreshnessReviewRecommended, BasedOnBodyStateRevision: 7, Reasons: []DiagnosisFreshnessReason{}},
		"not review_recommend": {State: DiagnosisFreshnessCurrent, BasedOnBodyStateRevision: 1, Reasons: []DiagnosisFreshnessReason{}},
	} {
		if diagnosisReviewPromptDismissed(f, d) {
			t.Fatalf("%s stayed dismissed", name)
		}
	}
}

func TestDiagnosisAnalysisRequestBindingPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisAnalysisRequestService(db)

	// The fixture's request already binds the full message text: replayed.
	got, replayed, err := svc.Bind(ctx, f.owner, f.run.RequestMessageID)
	if err != nil || !replayed || got.ID != f.request.ID {
		t.Fatalf("existing binding: %+v %v %v", got, replayed, err)
	}
	// A new user message is bound verbatim, once.
	var chat model.Run
	if err := db.Where("user_id=? AND id=?", f.owner, *f.request.SourceRunID).Take(&chat).Error; err != nil {
		t.Fatal(err)
	}
	message := model.Message{ID: uuid.New(), ConversationID: chat.ConversationID, TurnID: chat.TurnID, Role: "user", Seq: 7,
		Parts: datatypes.JSON(`[{"type":"text","text":"帮我正式分析一下头痛。"}]`)}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	created, replayed, err := svc.Bind(ctx, f.owner, message.ID)
	if err != nil || replayed || created.Excerpt != "帮我正式分析一下头痛。" || created.SourceKind != "user_message" || created.SourceVersion != 1 ||
		*created.SourceMessageID != message.ID || *created.SourceRunID != chat.ID || created.Availability != "available" {
		t.Fatalf("new binding: %+v %v %v", created, replayed, err)
	}
	again, replayed, err := svc.Bind(ctx, f.owner, message.ID)
	if err != nil || !replayed || again.ID != created.ID {
		t.Fatalf("rebinding created a second statement: %+v %v %v", again, replayed, err)
	}
	// The bound statement is exactly what a formal run's request check accepts.
	if err := repository.NewDiagnosisSourceRepository(db).UserRequest(ctx, f.owner, created.ID, message.ID, 1); err != nil {
		t.Fatalf("bound request refused by the source authority: %v", err)
	}
	// Foreign, unknown, non-user and structured-answer messages are refused.
	if _, _, err := svc.Bind(ctx, seedOnboardingContextUser(t, db), message.ID); !errors.Is(err, model.ErrDiagnosisRequestSourceNotFound) {
		t.Fatalf("foreign owner bound: %v", err)
	}
	if _, _, err := svc.Bind(ctx, f.owner, uuid.New()); !errors.Is(err, model.ErrDiagnosisRequestSourceNotFound) {
		t.Fatalf("unknown message bound: %v", err)
	}
	assistant := model.Message{ID: uuid.New(), ConversationID: chat.ConversationID, TurnID: chat.TurnID, Role: "assistant", Seq: 8,
		Parts: datatypes.JSON(`[{"type":"text","text":"AI wrote this"}]`)}
	answer := model.Message{ID: uuid.New(), ConversationID: chat.ConversationID, TurnID: chat.TurnID, Role: "user", Seq: 9,
		Parts: datatypes.JSON(`[{"type":"text","text":"是"}]`), Metadata: datatypes.JSON(`{"is_interaction_answer":true}`)}
	if err := db.Create(&assistant).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&answer).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Bind(ctx, f.owner, assistant.ID); !errors.Is(err, model.ErrDiagnosisRequestSourceNotFound) {
		t.Fatalf("assistant message bound: %v", err)
	}
	if _, _, err := svc.Bind(ctx, f.owner, answer.ID); !errors.Is(err, model.ErrDiagnosisRequestSourceInvalid) {
		t.Fatalf("structured answer bound: %v", err)
	}
}
