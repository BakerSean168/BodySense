package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestDiagnosisAggregatePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()
	// Concurrent initialization is idempotent and never creates a revision.
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { _, err := repo.EnsureAggregate(ctx, owner); errs <- err })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertBodyFactCount(t, db, "diagnoses", owner, 1)
	assertBodyFactCount(t, db, "diagnosis_revisions", owner, 0)
	head, err := repo.GetHead(ctx, owner)
	if err != nil || head.CurrentRevision != 0 {
		t.Fatalf("head=%#v err=%v", head, err)
	}
	requireDiagnosisSQLState(t, db.Exec("INSERT INTO diagnoses(user_id) VALUES (?)", owner).Error, "23505")
	requireDiagnosisSQLState(t, db.Exec("UPDATE diagnoses SET current_revision=1 WHERE user_id=?", owner).Error, "23503")
	requireDiagnosisSQLState(t, db.Exec("UPDATE diagnoses SET current_revision=-1 WHERE user_id=?", owner).Error, "23514")
	// Caller rollback must include repository initialization.
	fresh := seedBodyStateContextUser(t, db)
	abort := errors.New("caller abort")
	err = database.NewTransactionManager(db).WithinTransaction(ctx, func(txctx context.Context) error {
		if _, err := repo.EnsureAggregate(txctx, fresh); err != nil {
			return err
		}
		if _, err := repo.GetHead(txctx, fresh); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	if _, err := repo.GetHead(ctx, fresh); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatal(err)
	}
	if _, err := repo.EnsureAggregate(ctx, uuid.Nil); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatal(err)
	}
}

type diagnosisFixture struct {
	owner       uuid.UUID
	run         model.ConditionAnalysisRun
	proposal    model.DiagnosisProposal
	decision    model.DiagnosisDecision
	revision    model.DiagnosisRevision
	entry       model.ConditionEntry
	fact        model.BodyFact
	observation model.BodyObservation
	episode     uuid.UUID
}

// Synthetic explicit user request and authorized history, inserted as test data.
// This does not implement or exercise the future decision application service.
func seedDiagnosisFixture(t *testing.T, db *gorm.DB) diagnosisFixture {
	t.Helper()
	ctx := context.Background()
	f := diagnosisFixture{owner: seedBodyStateContextUser(t, db), episode: uuid.New()}
	repo := NewDiagnosisRepository(db)
	if _, err := repo.EnsureAggregate(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	body := NewBodyStateRepository(db)
	if _, err := body.RecordDirectFacts(ctx, f.owner, bodyFactTestCommand("dx-facts", 0)); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id=? AND kind='symptom'", f.owner).Take(&f.fact).Error; err != nil {
		t.Fatal(err)
	}
	obs, err := body.RecordDirectObservation(ctx, f.owner, observationDirectCommand(1, "dx-observation", "My measured weight is 67.5 kg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id=? AND id=?", f.owner, obs.ObservationID).Take(&f.observation).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := db.Exec(`INSERT INTO symptom_episodes(id,user_id,symptom_type,body_region_ids,laterality,created_revision,created_at)
 VALUES (?,?,'shoulder discomfort','[]','unknown',2,?)`, f.episode, f.owner, now).Error; err != nil {
		t.Fatal(err)
	}
	source := seedUserMessageFactEnvelope(t, db, f.owner, datatypes.JSON(`[{"type":"text","text":"Please formally analyze my shoulder evidence"}]`), "user")
	var chat model.Run
	if err := db.Take(&chat, "id=?", source.RunID).Error; err != nil {
		t.Fatal(err)
	}
	evidence := model.StatementEvidence{ID: uuid.New(), UserID: f.owner, SourceKind: "user_message", SourceVersion: 1,
		SourceConversationID: &chat.ConversationID, SourceRunID: &source.RunID, SourceMessageID: &source.MessageID,
		Excerpt: "Please formally analyze my shoulder evidence", Availability: "available", RecordedAt: now}
	if err := db.Create(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	refs := []model.DiagnosisEvidenceRef{{EvidenceType: model.DiagnosisBodyFact, EvidenceID: f.fact.ID, Version: "1"}}
	f.run = model.ConditionAnalysisRun{UserID: f.owner, ID: uuid.New(), RequestMessageID: source.MessageID,
		RequestEvidenceID: evidence.ID, RequestSourceVersion: 1, Trigger: "new_analysis",
		Scope:                    model.ConditionAnalysisScope{ConditionIDs: []uuid.UUID{}, EvidenceRefs: refs, Description: "shoulder"},
		BasedOnBodyStateRevision: 2, InputEvidenceRefs: refs, AgentHypotheses: []model.ConditionAgentHypothesis{},
		ReplayInput: datatypes.JSON(`{"snapshot":"frozen"}`), ExecutionProvenance: datatypes.JSON(`{"provider":"synthetic"}`),
		IdempotencyKey: "dx-run", RequestFingerprint: strings.Repeat("a", 64), StartedAt: now}
	if err := db.Create(&f.run).Error; err != nil {
		t.Fatal(err)
	}
	f.proposal = model.DiagnosisProposal{UserID: f.owner, ID: uuid.New(), RunID: f.run.ID, ChangeKind: model.DiagnosisCreate,
		Content: datatypes.JSON(`{"proposedEntry":{"medicalName":"Synthetic named syndrome"}}`), ContentFingerprint: strings.Repeat("b", 64),
		EvidenceRefs: refs, CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: now}
	if err := db.Create(&f.proposal).Error; err != nil {
		t.Fatal(err)
	}
	// An AI proposal is not an adopted condition or aggregate mutation.
	assertBodyFactCount(t, db, "condition_entries", f.owner, 0)
	f.decision = model.DiagnosisDecision{UserID: f.owner, ID: uuid.New(), ProposalID: f.proposal.ID, RunID: f.run.ID,
		ContentFingerprint: f.proposal.ContentFingerprint, DecidedEvidenceRefs: refs, Decision: model.DiagnosisAccept,
		ToolCallID: "exact-tool", DecidedAt: now, IdempotencyKey: "dx-decision", RequestFingerprint: strings.Repeat("c", 64), ReceiptID: uuid.New()}
	if err := db.Create(&f.decision).Error; err != nil {
		t.Fatal(err)
	}
	assertBodyFactCount(t, db, "condition_entries", f.owner, 0)
	assertBodyFactCount(t, db, "diagnosis_revisions", f.owner, 0)
	head, err := repo.GetHead(ctx, f.owner)
	if err != nil || head.CurrentRevision != 0 {
		t.Fatalf("decision changed head: %#v %v", head, err)
	}
	f.revision = model.DiagnosisRevision{UserID: f.owner, ID: uuid.New(), Revision: 1, RunID: &f.run.ID,
		Patch: datatypes.JSON(`[{"op":"create","medicalName":"Synthetic named syndrome"}]`), DecisionIDs: []uuid.UUID{f.decision.ID},
		BodyStateRevision: 2, EvidenceRefs: refs, ReplayInput: f.run.ReplayInput, ExecutionProvenance: f.run.ExecutionProvenance,
		IdempotencyKey: "dx-commit", RequestFingerprint: strings.Repeat("d", 64), ReceiptID: uuid.New(), CommittedAt: now}
	f.entry = model.ConditionEntry{UserID: f.owner, ID: uuid.New(), MedicalName: "Synthetic named syndrome",
		Assessment: &model.ConditionAssessment{Level: model.DiagnosisSuspected, Rationale: "Synthetic analysis", InformationGaps: []string{},
			DifferentialNotes: []string{}, BasedOnRunID: f.run.ID},
		SourceClaimRefs: []model.DiagnosisSourceClaimRef{{ClaimID: "synthetic-claim", SourceKind: "user_suspicion", SourceID: evidence.ID.String(),
			SourceVersion: "1", Statement: "Could my shoulder have a named syndrome?", RecordedAt: now, ValidityAtAssessment: "current"}},
		CauseAnalysis: model.ConditionCauseAnalysis{Summary: "Unknown", Rationale: "Insufficient causal evidence"}, InformationGaps: []string{},
		HandlingPlan: model.ConditionHandlingPlan{Summary: "Seek assessment", Actions: []string{}, CautionNotes: []string{}},
		Disposition:  model.ConditionAdopted, LatestDecisionID: f.decision.ID, LastAnalysisRunID: &f.run.ID, CommittedRevision: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&f.revision).Error; err != nil {
			return err
		}
		if err := tx.Create(&f.entry).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE diagnoses SET current_revision=1 WHERE user_id=?", f.owner).Error
	}); err != nil {
		t.Fatal(err)
	}
	return f
}

func requireDiagnosisSQLState(t *testing.T, err error, code string) {
	t.Helper()
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != code {
		t.Fatalf("want SQLSTATE %s, got %v", code, err)
	}
}

func TestDiagnosisOwnerMaskedReadsAndFrozenReplayPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := seedDiagnosisFixture(t, db)
	foreign := seedBodyStateContextUser(t, db)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()
	entry, err := repo.GetCondition(ctx, f.owner, f.entry.ID)
	if err != nil || entry.Assessment == nil || entry.Assessment.Level != model.DiagnosisSuspected || entry.Disposition != model.ConditionAdopted {
		t.Fatalf("entry=%#v err=%v", entry, err)
	}
	if rows, err := repo.ListConditions(ctx, foreign); err != nil || len(rows) != 0 {
		t.Fatalf("foreign list=%#v %v", rows, err)
	}
	for _, id := range []uuid.UUID{f.entry.ID, uuid.New()} {
		if _, err := repo.GetCondition(ctx, foreign, id); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatalf("owner-masked 404 seam: %v", err)
		}
		if _, err := repo.ListEvidence(ctx, foreign, id); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
	}
	for _, owner := range []uuid.UUID{foreign, uuid.Nil} {
		if _, err := repo.GetAnalysisRun(ctx, owner, f.run.ID); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
		if _, err := repo.GetProposal(ctx, owner, f.proposal.ID); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
		if _, err := repo.GetDecisionReceipt(ctx, owner, "dx-decision"); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
		if _, err := repo.GetRevision(ctx, owner, 1); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
	}
	proposal, err := repo.GetProposal(ctx, f.owner, f.proposal.ID)
	if err != nil || proposal.ContentFingerprint != f.proposal.ContentFingerprint || proposal.EvidenceRefs[0].Version != "1" {
		t.Fatalf("proposal=%#v %v", proposal, err)
	}
	for range 2 {
		receipt, err := repo.GetDecisionReceipt(ctx, f.owner, "dx-decision")
		if err != nil || receipt.ID != f.decision.ID || receipt.ReceiptID != f.decision.ReceiptID || receipt.ToolCallID != "exact-tool" {
			t.Fatalf("receipt=%#v %v", receipt, err)
		}
	}
	// Source changes must not rewrite an already frozen replay base.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.fact.SourceEvidenceID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&model.Message{}, "id=?", f.run.RequestMessageID).Error; err != nil {
		t.Fatal(err)
	}
	run, err := repo.GetAnalysisRun(ctx, f.owner, f.run.ID)
	if err != nil || !jsonEqual(run.ReplayInput, f.run.ReplayInput) || !jsonEqual(run.ExecutionProvenance, f.run.ExecutionProvenance) {
		t.Fatalf("replay changed: %#v %v", run, err)
	}
	rev, err := repo.GetRevision(ctx, f.owner, 1)
	if err != nil || rev.ReceiptID != f.revision.ReceiptID || !jsonEqual(rev.ReplayInput, f.run.ReplayInput) {
		t.Fatalf("revision=%#v %v", rev, err)
	}
	for _, table := range []string{"condition_analysis_runs", "diagnosis_proposals", "diagnosis_decisions", "diagnosis_revisions"} {
		requireDiagnosisSQLState(t, db.Exec("UPDATE "+table+" SET id=id WHERE user_id=?", f.owner).Error, "23514")
	}
	counts, err := NewPrivacyErasureRepository(db).CountUserData(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"diagnoses", "condition_analysis_runs", "condition_entries", "diagnosis_proposals", "diagnosis_decisions", "diagnosis_revisions"} {
		found := false
		for _, count := range counts {
			if count.Name == table && count.Count == 1 {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing privacy inventory: %s", table)
		}
	}
	if err := db.Exec("DELETE FROM users WHERE id=?", f.owner).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"diagnoses", "condition_analysis_runs", "condition_entries", "condition_entry_evidence", "diagnosis_proposals", "diagnosis_decisions", "diagnosis_revisions"} {
		assertBodyFactCount(t, db, table, f.owner, 0)
	}
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	ax, _ := json.Marshal(x)
	by, _ := json.Marshal(y)
	return string(ax) == string(by)
}

func TestDiagnosisCompositeIdentityConstraintsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	a, b := seedDiagnosisFixture(t, db), seedDiagnosisFixture(t, db)
	// All failures assert the PostgreSQL constraint class, not an unrelated error.
	run := a.run
	run.ID = uuid.New()
	run.IdempotencyKey = uuid.NewString()
	run.RequestEvidenceID = b.run.RequestEvidenceID
	run.RequestMessageID = b.run.RequestMessageID
	requireDiagnosisSQLState(t, db.Create(&run).Error, "23503")
	run = a.run
	run.ID = uuid.New()
	run.IdempotencyKey = uuid.NewString()
	run.RequestSourceVersion = 2
	requireDiagnosisSQLState(t, db.Create(&run).Error, "23503")
	proposal := a.proposal
	proposal.ID = uuid.New()
	proposal.RunID = b.run.ID
	requireDiagnosisSQLState(t, db.Create(&proposal).Error, "23503")
	proposal = a.proposal
	proposal.ID = uuid.New()
	proposal.BaseRevision = 1
	requireDiagnosisSQLState(t, db.Create(&proposal).Error, "23503")
	proposal = a.proposal
	proposal.ID = uuid.New()
	proposal.ChangeKind = model.DiagnosisUpdate
	proposal.TargetConditionID = &b.entry.ID
	requireDiagnosisSQLState(t, db.Create(&proposal).Error, "23503")
	proposal = a.proposal
	proposal.ID = uuid.New()
	proposal.SupersedesProposalID = &b.proposal.ID
	requireDiagnosisSQLState(t, db.Create(&proposal).Error, "23503")
	// Use a fresh proposal so unique(proposal) cannot mask the identity failure.
	fresh := a.proposal
	fresh.ID = uuid.New()
	if err := db.Create(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	decision := a.decision
	decision.ID = uuid.New()
	decision.ReceiptID = uuid.New()
	decision.IdempotencyKey = uuid.NewString()
	decision.ProposalID = fresh.ID
	decision.RunID = b.run.ID
	requireDiagnosisSQLState(t, db.Create(&decision).Error, "23503")
	decision.RunID = a.run.ID
	decision.ContentFingerprint = strings.Repeat("e", 64)
	requireDiagnosisSQLState(t, db.Create(&decision).Error, "23503")
	decision.ContentFingerprint = fresh.ContentFingerprint
	decision.ExpectedRevision = 1
	requireDiagnosisSQLState(t, db.Create(&decision).Error, "23503")
	decision.ExpectedRevision = 0
	decision.Decision = model.DiagnosisReject
	if err := db.Create(&decision).Error; err != nil {
		t.Fatal(err)
	}
	// A recorded rejection cannot authorize an entry; clinical-only entries can
	// omit AI assessment without inventing clinical certainty.
	entry := a.entry
	entry.ID = uuid.New()
	entry.Assessment = nil
	entry.LatestDecisionID = decision.ID
	requireDiagnosisSQLState(t, db.Create(&entry).Error, "23503")
	entry.LatestDecisionID = b.decision.ID
	requireDiagnosisSQLState(t, db.Create(&entry).Error, "23503")
	entry.LatestDecisionID = a.decision.ID
	entry.LastAnalysisRunID = &b.run.ID
	requireDiagnosisSQLState(t, db.Create(&entry).Error, "23503")
	entry.LastAnalysisRunID = &a.run.ID
	entry.CommittedRevision = 2
	requireDiagnosisSQLState(t, db.Create(&entry).Error, "23503")
	entry.CommittedRevision = 1
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}
	if got, err := NewDiagnosisRepository(db).GetCondition(context.Background(), a.owner, entry.ID); err != nil || got.Assessment != nil {
		t.Fatalf("optional assessment=%#v %v", got, err)
	}
	// Same key or proposal cannot create a second decision receipt.
	duplicate := a.decision
	duplicate.ID = uuid.New()
	duplicate.ReceiptID = uuid.New()
	requireDiagnosisSQLState(t, db.Create(&duplicate).Error, "23505")
	duplicate.IdempotencyKey = uuid.NewString()
	requireDiagnosisSQLState(t, db.Create(&duplicate).Error, "23505")
	// Revision links include owner and exact Run base; no empty revisions.
	rev := a.revision
	rev.ID = uuid.New()
	rev.Revision = 2
	rev.PreviousRevision = 1
	rev.RunID = &b.run.ID
	rev.IdempotencyKey = uuid.NewString()
	rev.ReceiptID = uuid.New()
	requireDiagnosisSQLState(t, db.Create(&rev).Error, "23503")
	rev.RunID = nil
	edit := uuid.New()
	rev.DirectEditID = &edit
	rev.Patch = datatypes.JSON(`[]`)
	requireDiagnosisSQLState(t, db.Create(&rev).Error, "23514")
}

func seedDiagnosisDocumentReview(t *testing.T, db *gorm.DB, owner uuid.UUID, action string) uuid.UUID {
	t.Helper()
	assetID, extraction, review := uuid.New(), uuid.New(), uuid.New()
	hash := strings.Repeat("f", 64)
	if err := db.Exec(`INSERT INTO user_assets(id,user_id,kind,original_name,mime_type,byte_size,content_sha256,storage_backend,storage_key)
 VALUES (?,?,'report','synthetic.pdf','application/pdf',1,?,'local',?)`, assetID, owner, hash, "dx002/"+assetID.String()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO document_extraction_runs(id,user_id,asset_id,configuration_id,mechanism_revision,document_sha256,result_sha256,raw_text_sha256)
 VALUES (?,?,?,'synthetic','1',?,?,?)`, extraction, owner, assetID, hash, hash, hash).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO document_indicator_reviews(id,user_id,asset_id,extraction_run_id,indicator_index,action,idempotency_key,reviewer_user_id)
 VALUES (?,?,?,?,0,?,?,?)`, review, owner, assetID, extraction, action, uuid.NewString(), owner).Error; err != nil {
		t.Fatal(err)
	}
	return review
}

func TestDiagnosisTypedEvidenceVersionsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	a, b := seedDiagnosisFixture(t, db), seedDiagnosisFixture(t, db)
	repo := NewDiagnosisRepository(db)
	confirm, reject := "confirm", "reject"
	admitted := seedDiagnosisDocumentReview(t, db, a.owner, confirm)
	rejected := seedDiagnosisDocumentReview(t, db, a.owner, reject)
	foreignDocument := seedDiagnosisDocumentReview(t, db, b.owner, confirm)
	base := model.ConditionEntryEvidence{UserID: a.owner, ConditionID: a.entry.ID, Role: model.DiagnosisSupporting, Rationale: "Synthetic relevance; not causation", AnalyzedByRunID: a.run.ID}
	valid := []model.ConditionEntryEvidence{}
	for _, ref := range []struct {
		kind    model.DiagnosisEvidenceType
		id      uuid.UUID
		version int64
	}{
		{model.DiagnosisBodyFact, a.fact.ID, 1}, {model.DiagnosisBodyObservation, a.observation.ID, 2},
		{model.DiagnosisSymptomEpisode, a.episode, 2}, {model.DiagnosisDocumentEvidence, admitted, 1},
	} {
		link := base
		link.EvidenceType = ref.kind
		link.EvidenceID = ref.id
		link.EvidenceVersion = ref.version
		if ref.kind == model.DiagnosisDocumentEvidence {
			link.DocumentReviewAction = &confirm
		}
		if err := db.Create(&link).Error; err != nil {
			t.Fatal(err)
		}
		valid = append(valid, link)
	}
	rows, err := repo.ListEvidence(context.Background(), a.owner, a.entry.ID)
	if err != nil || len(rows) != 4 {
		t.Fatalf("typed links=%#v %v", rows, err)
	}
	for i, foreignID := range []uuid.UUID{b.fact.ID, b.observation.ID, b.episode, foreignDocument} {
		link := valid[i]
		link.EvidenceID = foreignID
		requireDiagnosisSQLState(t, db.Create(&link).Error, "23503")
	}
	for _, link := range valid[:3] {
		link.EvidenceVersion = 999999
		requireDiagnosisSQLState(t, db.Create(&link).Error, "23503")
	}
	// A real UUID in the wrong source domain cannot satisfy a polymorphic link.
	wrong := valid[0]
	wrong.EvidenceID = a.observation.ID
	requireDiagnosisSQLState(t, db.Create(&wrong).Error, "23503")
	wrong = valid[0]
	wrong.EvidenceType = "agent_hypothesis"
	requireDiagnosisSQLState(t, db.Create(&wrong).Error, "23514")
	wrong = valid[0]
	wrong.ConditionID = b.entry.ID
	requireDiagnosisSQLState(t, db.Create(&wrong).Error, "23503")
	wrong = valid[0]
	wrong.AnalyzedByRunID = b.run.ID
	wrong.Role = model.DiagnosisCounter
	requireDiagnosisSQLState(t, db.Create(&wrong).Error, "23503")
	wrong = valid[3]
	wrong.EvidenceID = rejected // Cannot disguise a reject as confirm.
	requireDiagnosisSQLState(t, db.Create(&wrong).Error, "23503")
	wrong.DocumentReviewAction = &reject
	requireDiagnosisSQLState(t, db.Create(&wrong).Error, "23514")
	wrong = valid[3]
	wrong.EvidenceVersion = 2
	requireDiagnosisSQLState(t, db.Create(&wrong).Error, "23514")
	wrong = valid[0]
	wrong.EvidenceVersion = 0
	requireDiagnosisSQLState(t, db.Create(&wrong).Error, "23514")
	// Deleting an evidence source independently must not silently destroy adopted
	// provenance. Account-level cascade still removes the complete owned graph.
	requireDiagnosisSQLState(t, db.Exec("DELETE FROM body_facts WHERE user_id=? AND id=?", a.owner, a.fact.ID).Error, "23503")
	if err := db.Exec("DELETE FROM users WHERE id=?", a.owner).Error; err != nil {
		t.Fatal(err)
	}
	assertBodyFactCount(t, db, "condition_entry_evidence", a.owner, 0)
}

func TestDiagnosisEpisodeEvidenceRevisionFloorPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	a := seedDiagnosisFixture(t, db)
	link := model.ConditionEntryEvidence{UserID: a.owner, ConditionID: a.entry.ID,
		EvidenceType: model.DiagnosisSymptomEpisode, EvidenceID: a.episode,
		Role: model.DiagnosisSupporting, Rationale: "Synthetic relevance; not causation", AnalyzedByRunID: a.run.ID}
	// The fixture episode is created at body state revision 2 while revision 1
	// exists for the same owner; both owner FKs alone would admit the link.
	link.EvidenceVersion = 1
	requireDiagnosisSQLState(t, db.Create(&link).Error, "23514")
	// Rewriting a stored episode link down to a pre-creation revision is the same forgery.
	link.EvidenceVersion = 2
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	requireDiagnosisSQLState(t, db.Model(&link).Update("evidence_version", 1).Error, "23514")
}

func TestDiagnosisEntryDimensionsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := seedDiagnosisFixture(t, db)
	for _, value := range []string{
		`{"level":"clinical_diagnosis","basedOnRunId":"` + f.run.ID.String() + `"}`,
		`{"level":null,"basedOnRunId":"` + f.run.ID.String() + `"}`,
		`{"level":"likely","basedOnRunId":null}`,
	} {
		requireDiagnosisSQLState(t, db.Exec("UPDATE condition_entries SET assessment=?::jsonb WHERE user_id=? AND id=?", value, f.owner, f.entry.ID).Error, "23514")
	}
	requireDiagnosisSQLState(t, db.Exec("UPDATE condition_entries SET disposition='resolved' WHERE user_id=? AND id=?", f.owner, f.entry.ID).Error, "23514")
	requireDiagnosisSQLState(t, db.Exec(`UPDATE condition_entries SET concept_ref='{"system":null,"code":"invented"}' WHERE user_id=? AND id=?`, f.owner, f.entry.ID).Error, "23514")
}
