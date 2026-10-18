package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestIndependentTrainingPlansPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	draft := model.TrainingPlanDraft{Title: "A", Goal: "User goal", Content: model.TrainingPlanContent{
		Sessions: []model.PlanSessionTemplate{{TemplateID: uuid.NewString(), Title: "Session", Stages: []model.PlanStage{{Phase: "main", Exercises: []model.PlanExercise{{PlanExerciseID: uuid.NewString(), ExerciseID: "squat", NameSnapshot: "Squat", Sets: 2, Target: model.PlanTarget{Kind: "repetitions", Min: 5}}}}}}},
		Schedule: model.PlanSchedule{Mode: "frequency_only"},
	}}
	proposal, err := svc.Propose(ctx, owner, PlanProposalInput{Draft: draft, InitiatedBy: "agent", SourceID: "test-generation", IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.Accept(ctx, owner, PlanDecision{ProposalID: proposal.ID, DecisionID: uuid.New(), IdempotencyKey: "accept-A", PresentedFingerprint: proposal.ContentFingerprint})
	if err != nil {
		t.Fatal(err)
	}
	draft.Title = "B"
	draft.Content.Sessions[0].TemplateID = uuid.NewString()
	draft.Content.Sessions[0].Stages[0].Exercises[0].PlanExerciseID = uuid.NewString()
	proposal, err = svc.Propose(ctx, owner, PlanProposalInput{Draft: draft, InitiatedBy: "agent", SourceID: "test-generation", IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Accept(ctx, owner, PlanDecision{ProposalID: proposal.ID, DecisionID: uuid.New(), IdempotencyKey: "accept-B", PresentedFingerprint: proposal.ContentFingerprint})
	if err != nil {
		t.Fatal(err)
	}
	if a.PlanID == b.PlanID {
		t.Fatal("independent plans collapsed")
	}
	plans, err := svc.List(ctx, owner)
	if err != nil || len(plans) != 2 || plans[0].Status != "active" || plans[1].Status != "active" {
		t.Fatalf("multi-active: %+v %v", plans, err)
	}
}

func independentDraft(title string) model.TrainingPlanDraft {
	return model.TrainingPlanDraft{Title: title, Goal: "User goal", Content: model.TrainingPlanContent{
		Sessions: []model.PlanSessionTemplate{{TemplateID: uuid.NewString(), Title: "Session", Stages: []model.PlanStage{{Phase: "main", Exercises: []model.PlanExercise{{PlanExerciseID: uuid.NewString(), ExerciseID: "squat", NameSnapshot: "Squat", Sets: 2, Target: model.PlanTarget{Kind: "repetitions", Min: 5}}}}}}}, Schedule: model.PlanSchedule{Mode: "frequency_only"},
	}}
}
func independentProposal(t *testing.T, svc *IndependentTrainingPlanService, owner uuid.UUID, draft model.TrainingPlanDraft, plan, revision uuid.UUID) *model.PlanProposal {
	t.Helper()
	p, err := svc.Propose(context.Background(), owner, PlanProposalInput{Draft: draft, PlanID: plan, ExpectedRevisionID: revision, InitiatedBy: "agent", SourceID: "synthetic-generation", IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func independentDecision(p *model.PlanProposal) PlanDecision {
	d := PlanDecision{ProposalID: p.ID, DecisionID: uuid.New(), IdempotencyKey: uuid.NewString(), PresentedFingerprint: p.ContentFingerprint}
	if p.PlanID != nil {
		d.TargetPlanID = *p.PlanID
	}
	if p.BasedOnRevisionID != nil {
		d.ExpectedRevisionID = *p.BasedOnRevisionID
	}
	return d
}
func independentPlanSnapshot(t *testing.T, db *gorm.DB, owner, plan uuid.UUID) string {
	t.Helper()
	var snapshot string
	err := db.Raw(`SELECT jsonb_build_object(
 'plan',(SELECT to_jsonb(p) FROM training_plans p WHERE user_id=? AND id=?),
 'revisions',(SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY id),'[]') FROM training_plan_revisions r WHERE user_id=? AND plan_id=?),
 'sessions',(SELECT coalesce(jsonb_agg(to_jsonb(l) ORDER BY id),'[]') FROM training_sessions l WHERE user_id=? AND plan_id=?)
 )::text`, owner, plan, owner, plan, owner, plan).Scan(&snapshot).Error
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestIndependentTrainingDecisionAtomicityPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	foreign := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	aDraft, bDraft := independentDraft("A"), independentDraft("B")
	ap := independentProposal(t, svc, owner, aDraft, uuid.Nil, uuid.Nil)
	ad := independentDecision(ap)
	a, err := svc.Accept(ctx, owner, ad)
	if err != nil {
		t.Fatal(err)
	}
	bp := independentProposal(t, svc, owner, bDraft, uuid.Nil, uuid.Nil)
	b, err := svc.Accept(ctx, owner, independentDecision(bp))
	if err != nil {
		t.Fatal(err)
	}
	beforeB := independentPlanSnapshot(t, db, owner, b.PlanID)
	// Exact retries return the same immutable revision and receipt, even after
	// another independent plan has been activated.
	replay, err := svc.Accept(ctx, owner, ad)
	if err != nil || replay.RevisionID != a.RevisionID || replay.Receipt.ID != a.Receipt.ID {
		t.Fatalf("retry: %+v %v", replay, err)
	}
	changed := ad
	changed.PresentedFingerprint = "changed"
	if _, err = svc.Accept(ctx, owner, changed); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("key payload conflict: %v", err)
	}
	secondKey := ad
	secondKey.IdempotencyKey = uuid.NewString()
	if _, err = svc.Accept(ctx, owner, secondKey); err == nil {
		t.Fatal("accepted twice under a new key")
	}
	if _, err = svc.Accept(ctx, foreign, ad); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("foreign proposal must be 404: %v", err)
	}
	if _, err = svc.Get(ctx, foreign, a.PlanID); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("foreign plan must be 404: %v", err)
	}
	if _, err = svc.Revision(ctx, foreign, a.PlanID, a.RevisionID); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("foreign revision must be 404: %v", err)
	}
	// Neither rejection nor failed acceptance may affect the other plan.
	aDraft.Goal = "Proposed A goal"
	rejected := independentProposal(t, svc, owner, aDraft, a.PlanID, a.RevisionID)
	rd := independentDecision(rejected)
	beforeA := independentPlanSnapshot(t, db, owner, a.PlanID)
	rr, err := svc.Reject(ctx, owner, rd)
	if err != nil {
		t.Fatal(err)
	}
	replayReject, err := svc.Reject(ctx, owner, rd)
	if err != nil || replayReject.ID != rr.ID {
		t.Fatalf("reject retry: %v", err)
	}
	if _, err = svc.Accept(ctx, owner, independentDecision(rejected)); err == nil {
		t.Fatal("accepted rejected proposal")
	}
	// Receipt uniqueness fails AFTER revision and plan CAS writes. The complete
	// transaction must roll back, leaving the proposal pending and both plans intact.
	failed := independentProposal(t, svc, owner, aDraft, a.PlanID, a.RevisionID)
	fd := independentDecision(failed)
	fd.DecisionID = ad.DecisionID
	if _, err = svc.Accept(ctx, owner, fd); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("duplicate user decision must conflict atomically: %v", err)
	}
	pending, err := svc.repo.Proposal(ctx, owner, failed.ID)
	if err != nil || pending.State != "pending" {
		t.Fatalf("partial proposal: %+v %v", pending, err)
	}
	if got := independentPlanSnapshot(t, db, owner, a.PlanID); got != beforeA {
		t.Fatal("rollback or rejection changed A")
	}
	if got := independentPlanSnapshot(t, db, owner, b.PlanID); got != beforeB {
		t.Fatal("A decision changed B status/content/revision/logs")
	}
	// Immutable accepted snapshots cannot be edited in place, even through SQL.
	if err = db.Exec("UPDATE training_plan_revisions SET snapshot='{}' WHERE user_id=? AND id=?", owner, a.RevisionID).Error; err == nil {
		t.Fatal("immutable revision updated")
	}
	// Updates preserve paused/completed state and create exactly one child revision.
	if err = db.Exec("UPDATE training_plans SET status='paused' WHERE user_id=? AND id=?", owner, a.PlanID).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := svc.Accept(ctx, owner, independentDecision(failed))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := svc.Get(ctx, owner, a.PlanID)
	if err != nil || plan.Status != "paused" || *plan.CurrentAcceptedRevisionID != updated.RevisionID {
		t.Fatalf("update changed status: %+v %v", plan, err)
	}
	if got := independentPlanSnapshot(t, db, owner, b.PlanID); got != beforeB {
		t.Fatal("A revision changed B")
	}
}

func TestIndependentTrainingConcurrencyAndCASPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	draft := independentDraft("A")
	p := independentProposal(t, svc, owner, draft, uuid.Nil, uuid.Nil)
	d := independentDecision(p)
	start := make(chan struct{})
	results := make(chan *PlanAcceptance, 8)
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { <-start; r, err := svc.Accept(ctx, owner, d); results <- r; failures <- err }()
	}
	close(start)
	var first *PlanAcceptance
	for i := 0; i < 8; i++ {
		r, err := <-results, <-failures
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = r
		}
		if r.RevisionID != first.RevisionID || r.Receipt.ID != first.Receipt.ID {
			t.Fatal("concurrent duplicate produced extra revision")
		}
	}
	draft.Goal = "new goal"
	p1 := independentProposal(t, svc, owner, draft, first.PlanID, first.RevisionID)
	p2 := independentProposal(t, svc, owner, draft, first.PlanID, first.RevisionID)
	decisions := []PlanDecision{independentDecision(p1), independentDecision(p2)}
	outcomes := make(chan error, 2)
	start = make(chan struct{})
	for _, decision := range decisions {
		go func(d PlanDecision) { <-start; _, err := svc.Accept(ctx, owner, d); outcomes <- err }(decision)
	}
	close(start)
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-outcomes
		if err == nil {
			successes++
		} else {
			var admission *PlanAdmissionError
			if !errors.As(err, &admission) || admission.Verdict != trainingplan.PlanRevisionChanged {
				t.Fatal(err)
			}
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("CAS winners=%d conflicts=%d", successes, conflicts)
	}
	var count int64
	if err := db.Model(&model.TrainingPlanRevision{}).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("CAS revisions=%d %v", count, err)
	}
}

func TestIndependentTrainingStaleSafetyAndExpiryPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	draft := independentDraft("A")
	p := independentProposal(t, svc, owner, draft, uuid.Nil, uuid.Nil)
	d := independentDecision(p)
	a, err := svc.Accept(ctx, owner, d)
	if err != nil {
		t.Fatal(err)
	}
	if a.Gate.Permits() {
		t.Fatal("missing safety screen allowed execution")
	}
	stale := independentProposal(t, svc, owner, draft, a.PlanID, a.RevisionID)
	before := independentPlanSnapshot(t, db, owner, a.PlanID)
	repo := repository.NewBodyStateRepository(db)
	_, err = repo.RecordDirectFacts(ctx, owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 0, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic knee pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Accept(ctx, owner, independentDecision(stale))
	var admission *PlanAdmissionError
	if !errors.As(err, &admission) || admission.Verdict != trainingplan.ContextChanged {
		t.Fatalf("stale safety admitted: %v", err)
	}
	expired, err := svc.repo.Proposal(ctx, owner, stale.ID)
	if err != nil || expired.State != "expired" || *expired.ExpiredReason != "relevant_context_changed" {
		t.Fatalf("stale proposal: %+v %v", expired, err)
	}
	replay, err := svc.Accept(ctx, owner, d)
	if err != nil || replay.Gate.Permits() {
		t.Fatalf("retry replayed an allow: %+v %v", replay, err)
	}
	if got := independentPlanSnapshot(t, db, owner, a.PlanID); got != before {
		t.Fatal("safety change rewrote prescription")
	}
	timeout := independentProposal(t, svc, owner, independentDraft("B"), uuid.Nil, uuid.Nil)
	rejectTimeout := independentProposal(t, svc, owner, independentDraft("C"), uuid.Nil, uuid.Nil)
	svc.now = func() time.Time { return timeout.ExpiresAt }
	if _, err = svc.Accept(ctx, owner, independentDecision(timeout)); !errors.As(err, &admission) || admission.Verdict != trainingplan.ProposalTimedOut {
		t.Fatalf("7-day boundary: %v", err)
	}
	svc.now = func() time.Time { return rejectTimeout.ExpiresAt }
	if _, err = svc.Reject(ctx, owner, independentDecision(rejectTimeout)); !errors.As(err, &admission) || admission.Verdict != trainingplan.ProposalTimedOut {
		t.Fatalf("expired proposal rejected as pending: %v", err)
	}
}

func TestIndependentTrainingSafetySaveBoundaryPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	foreign := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	repo := repository.NewBodyStateRepository(db)
	ctx := context.Background()
	receipt, err := repo.RecordDirectFacts(ctx, owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic knee pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	var factIDs []uuid.UUID
	if err = json.Unmarshal(receipt.FactIDs, &factIDs); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, CaptureID: "0123456789abcdef01234567", BodyFactID: factIDs[0], SourceConversationID: uuid.New(), SourceRunID: uuid.New(), SourceMessageID: uuid.New(), SourceInteractionID: uuid.New(), SourceToolCallID: "intake-0123456789abcdef01234567", BodyStateRevision: 1, RecordedAt: now}
	if err = db.Create(&screen).Error; err != nil {
		t.Fatal(err)
	}
	draft := independentDraft("A")
	draft.RelatedBodyRefs = []model.PlanBodyRef{{Kind: "body_fact", ID: factIDs[0], SourceVersion: 1}}
	// With authoritative complete negative screening, a current snapshot permits
	// execution. This is a projection, never a durable authorization receipt.
	clean := independentProposal(t, svc, owner, draft, uuid.Nil, uuid.Nil)
	accepted, err := svc.Accept(ctx, owner, independentDecision(clean))
	if err != nil || accepted.Gate.Result != safety.ResultAllow {
		t.Fatalf("screened projection: %+v %v", accepted, err)
	}
	if _, err = svc.Propose(ctx, foreign, PlanProposalInput{Draft: draft, InitiatedBy: "agent", SourceID: "synthetic", IdempotencyKey: uuid.NewString()}); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("foreign body reference: %v", err)
	}
	// Use the existing Go screen restriction mapping, never an invented threshold.
	// Native safety changes own a new BodyState revision; direct fixture insert
	// here models an already-current restriction at the next proposal's capture.
	restriction := model.SafetyRestriction{ID: uuid.New(), UserID: owner, SafetyScreenID: screen.ID, Concept: "numbness", PolicyRevision: model.SafetyScreenRestrictionPolicyV1, RiskLevel: "restricted", ScopeKind: "all_training", LiftPolicy: "professional_clearance", Reason: "synthetic numbness", CreatedRevision: 1, ValidFrom: now}
	screen.Numbness = true
	if err = db.Model(&screen).Update("numbness", true).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&restriction).Error; err != nil {
		t.Fatal(err)
	}
	blocked := independentProposal(t, svc, owner, draft, accepted.PlanID, accepted.RevisionID)
	saved, err := svc.Accept(ctx, owner, independentDecision(blocked))
	if err != nil || saved.Gate.Result != safety.ResultBlock || saved.Gate.Permits() {
		t.Fatalf("save under existing restriction: %+v %v", saved, err)
	}
	plan, err := svc.Get(ctx, owner, saved.PlanID)
	if err != nil || plan.Status != "active" {
		t.Fatalf("restriction changed plan state: %+v %v", plan, err)
	}
	// Owner + plan identities are enforced in current/base/parent revision FKs.
	other := independentProposal(t, svc, owner, independentDraft("B"), uuid.Nil, uuid.Nil)
	b, err := svc.Accept(ctx, owner, independentDecision(other))
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("UPDATE training_plans SET current_accepted_revision_id=? WHERE user_id=? AND id=?", saved.RevisionID, owner, b.PlanID).Error; err == nil {
		t.Fatal("cross-plan accepted revision linked")
	}
	if err = db.Exec("UPDATE training_plans SET current_accepted_revision_id=? WHERE user_id=? AND id=?", saved.RevisionID, foreign, b.PlanID).Error; err != nil {
		t.Fatal(err)
	} // owner-scoped no-op
	// An entry/template cannot be transplanted from A to B.
	copied := independentProposal(t, svc, owner, draft, b.PlanID, b.RevisionID)
	if _, err = svc.Accept(ctx, owner, independentDecision(copied)); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("cross-plan entry identity accepted: %v", err)
	}
}

func TestIndependentTrainingProposalIdempotencyPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	input := PlanProposalInput{Draft: independentDraft("A"), InitiatedBy: "agent", SourceID: "synthetic", IdempotencyKey: uuid.NewString()}
	first, err := svc.Propose(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Propose(ctx, owner, input)
	if err != nil || first.ID != second.ID {
		t.Fatalf("proposal retry: %+v %v", second, err)
	}
	plans, err := svc.List(ctx, owner)
	if err != nil || len(plans) != 0 {
		t.Fatalf("AI proposal created accepted plan: %+v %v", plans, err)
	}
	input.Draft.Title = "Changed"
	if _, err = svc.Propose(ctx, owner, input); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("proposal key payload changed: %v", err)
	}
}
