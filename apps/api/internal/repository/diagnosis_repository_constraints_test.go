package repository

import (
	"context"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"strings"
	"testing"
)

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
