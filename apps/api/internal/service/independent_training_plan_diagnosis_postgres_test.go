package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// DX-002 slice 1 intentionally has no adoption command yet. Seed an explicit
// synthetic user decision + committed native history, as its storage tests do.
func seedPlanDiagnosis(t *testing.T, db *gorm.DB, owner uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := repository.NewDiagnosisRepository(db).EnsureAggregate(ctx, owner); err != nil {
		t.Fatal(err)
	}
	body := repository.NewBodyStateRepository(db)
	commit, err := body.RecordDirectFacts(ctx, owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), Facts: []model.BodyFactInput{{Kind: "lifestyle", Statement: "synthetic user history", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	if err = json.Unmarshal(commit.FactIDs, &ids); err != nil {
		t.Fatal(err)
	}
	var fact model.BodyFact
	if err = db.Where("user_id=? AND id=?", owner, ids[0]).Take(&fact).Error; err != nil {
		t.Fatal(err)
	}
	conversation := model.Conversation{ID: uuid.New(), UserID: owner}
	if err = db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	chatRun := model.Run{ID: uuid.New(), UserID: owner, ConversationID: conversation.ID, TurnID: uuid.New(), RequestID: uuid.NewString(), Model: "synthetic", Status: model.RunStatusCompleted}
	if err = db.Create(&chatRun).Error; err != nil {
		t.Fatal(err)
	}
	message := model.Message{ID: uuid.New(), ConversationID: conversation.ID, TurnID: chatRun.TurnID, Role: "user", Seq: 1, Parts: datatypes.JSON(`[{"type":"text","text":"Please analyze my history"}]`)}
	if err = db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	request := model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "user_message", SourceVersion: 1, SourceConversationID: &conversation.ID, SourceRunID: &chatRun.ID, SourceMessageID: &message.ID, Excerpt: "Please analyze my history", Availability: "available", RecordedAt: now}
	if err = db.Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	refs := []model.DiagnosisEvidenceRef{{EvidenceType: model.DiagnosisBodyFact, EvidenceID: fact.ID, Version: "1"}}
	run := model.ConditionAnalysisRun{UserID: owner, ID: uuid.New(), RequestMessageID: message.ID, RequestEvidenceID: request.ID, RequestSourceVersion: 1, Trigger: model.ConditionNewAnalysis, Scope: model.ConditionAnalysisScope{ConditionIDs: []uuid.UUID{}, EvidenceRefs: refs}, BasedOnBodyStateRevision: 1, InputEvidenceRefs: refs, AgentHypotheses: []model.ConditionAgentHypothesis{}, ReplayInput: datatypes.JSON(`{}`), ExecutionProvenance: datatypes.JSON(`{}`), IdempotencyKey: uuid.NewString(), RequestFingerprint: strings.Repeat("a", 64), StartedAt: now}
	if err = db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	proposal := model.DiagnosisProposal{UserID: owner, ID: uuid.New(), RunID: run.ID, ChangeKind: model.DiagnosisCreate, Content: datatypes.JSON(`{"proposedEntry":{"medicalName":"Synthetic condition"}}`), ContentFingerprint: strings.Repeat("b", 64), EvidenceRefs: refs, CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: now}
	if err = db.Create(&proposal).Error; err != nil {
		t.Fatal(err)
	}
	decision := model.DiagnosisDecision{UserID: owner, ID: uuid.New(), ProposalID: proposal.ID, RunID: run.ID, ContentFingerprint: proposal.ContentFingerprint, DecidedEvidenceRefs: refs, Decision: model.DiagnosisAccept, ToolCallID: "synthetic-user-accept", DecidedAt: now, IdempotencyKey: uuid.NewString(), RequestFingerprint: strings.Repeat("c", 64), ReceiptID: uuid.New()}
	if err = db.Create(&decision).Error; err != nil {
		t.Fatal(err)
	}
	revision := model.DiagnosisRevision{UserID: owner, ID: uuid.New(), Revision: 1, RunID: &run.ID, Patch: datatypes.JSON(`[{"op":"create","medicalName":"Synthetic condition"}]`), DecisionIDs: []uuid.UUID{decision.ID}, BodyStateRevision: 1, EvidenceRefs: refs, ReplayInput: run.ReplayInput, ExecutionProvenance: run.ExecutionProvenance, IdempotencyKey: uuid.NewString(), RequestFingerprint: strings.Repeat("d", 64), ReceiptID: uuid.New(), CommittedAt: now}
	entry := model.ConditionEntry{UserID: owner, ID: uuid.New(), MedicalName: "Synthetic condition", SourceClaimRefs: []model.DiagnosisSourceClaimRef{{ClaimID: "user-claim", SourceKind: model.DiagnosisUserSuspicion, SourceID: fact.SourceEvidenceID.String(), SourceVersion: "1", Statement: fact.Statement, RecordedAt: now, ValidityAtAssessment: model.DiagnosisSourceCurrent}}, InformationGaps: []string{}, Disposition: model.ConditionAdopted, LatestDecisionID: decision.ID, LastAnalysisRunID: &run.ID, CommittedRevision: 1, CreatedAt: now, UpdatedAt: now}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		link := model.ConditionEntryEvidence{UserID: owner, ConditionID: entry.ID, EvidenceType: model.DiagnosisBodyFact, EvidenceID: fact.ID, EvidenceVersion: 1, Role: model.DiagnosisSupporting, Rationale: "synthetic source", AnalyzedByRunID: run.ID}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE diagnoses SET current_revision=1 WHERE user_id=?", owner).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	return entry.ID, fact.SourceEvidenceID
}
func correctPlanDiagnosis(t *testing.T, db *gorm.DB, owner, condition uuid.UUID) {
	t.Helper()
	directEdit := uuid.New()
	rev := model.DiagnosisRevision{UserID: owner, ID: uuid.New(), PreviousRevision: 1, Revision: 2, DirectEditID: &directEdit, Patch: datatypes.JSON(`[{"op":"update","medicalName":"Corrected synthetic condition"}]`), DecisionIDs: []uuid.UUID{}, BodyStateRevision: 1, EvidenceRefs: []model.DiagnosisEvidenceRef{}, ReplayInput: datatypes.JSON(`{}`), ExecutionProvenance: datatypes.JSON(`{}`), IdempotencyKey: uuid.NewString(), RequestFingerprint: strings.Repeat("e", 64), ReceiptID: uuid.New(), CommittedAt: time.Now().UTC()}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rev).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE condition_entries SET medical_name='Corrected synthetic condition',committed_revision=2 WHERE user_id=? AND id=?", owner, condition).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE diagnoses SET current_revision=2 WHERE user_id=? AND current_revision=1", owner).Error
	}); err != nil {
		t.Fatal(err)
	}
}
func TestIndependentTrainingNativeDiagnosisRevisionPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	condition, _ := seedPlanDiagnosis(t, db, owner)
	svc := NewIndependentTrainingPlanService(db)
	p := independentProposal(t, svc, owner, independentDraft("A"), uuid.Nil, uuid.Nil)
	correctPlanDiagnosis(t, db, owner, condition)
	accepted, err := svc.Accept(context.Background(), owner, independentDecision(p))
	var admission *PlanAdmissionError
	if !errors.As(err, &admission) || admission.Verdict != "relevant_context_changed" {
		t.Fatalf("native Diagnosis R1→R2 must invalidate proposal, got acceptance=%+v err=%v", accepted, err)
	}
	for _, table := range []string{"training_plans", "training_plan_revisions", "plan_decision_receipts"} {
		var count int64
		if err := db.Table(table).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("stale context wrote %s: %d %v", table, count, err)
		}
	}
}

func TestIndependentTrainingAssociatedDiagnosisPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	other := seedOnboardingContextUser(t, db)
	condition, _ := seedPlanDiagnosis(t, db, owner)
	otherCondition, _ := seedPlanDiagnosis(t, db, other)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	draft := independentDraft("A")
	draft.RelatedConditionIDs = []uuid.UUID{condition}
	p := independentProposal(t, svc, owner, draft, uuid.Nil, uuid.Nil)
	// An unrelated owner's native revision does not enter this fingerprint.
	correctPlanDiagnosis(t, db, other, otherCondition)
	accepted, err := svc.Accept(ctx, owner, independentDecision(p))
	if err != nil {
		t.Fatalf("foreign Diagnosis interfered: %v", err)
	}
	var links int64
	if err = db.Table("training_plan_revision_conditions").Where("user_id=? AND revision_id=? AND condition_id=?", owner, accepted.RevisionID, condition).Count(&links).Error; err != nil || links != 1 {
		t.Fatalf("missing native condition relation: %d %v", links, err)
	}
	// SQL itself must also fence foreign condition identities.
	if err = db.Exec("INSERT INTO training_plan_revision_conditions(user_id,revision_id,condition_id) VALUES (?,?,?)", owner, accepted.RevisionID, otherCondition).Error; err == nil {
		t.Fatal("foreign condition FK accepted")
	}
	stale := independentProposal(t, svc, owner, draft, accepted.PlanID, accepted.RevisionID)
	before := independentPlanSnapshot(t, db, owner, accepted.PlanID)
	correctPlanDiagnosis(t, db, owner, condition)
	_, err = svc.Accept(ctx, owner, independentDecision(stale))
	var admission *PlanAdmissionError
	if !errors.As(err, &admission) || admission.Verdict != "relevant_context_changed" {
		t.Fatalf("associated native correction accepted: %v", err)
	}
	if got := independentPlanSnapshot(t, db, owner, accepted.PlanID); got != before {
		t.Fatal("Diagnosis correction rewrote accepted plan/revision/logs")
	}
}
func TestIndependentTrainingConditionOwnerAndAvailabilityPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	other := seedOnboardingContextUser(t, db)
	condition, source := seedPlanDiagnosis(t, db, owner)
	foreign, _ := seedPlanDiagnosis(t, db, other)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	for _, id := range []uuid.UUID{foreign, uuid.New()} {
		draft := independentDraft("Invalid relation")
		draft.RelatedConditionIDs = []uuid.UUID{id}
		_, err := svc.Propose(ctx, owner, PlanProposalInput{Draft: draft, InitiatedBy: "agent", SourceID: "synthetic", IdempotencyKey: uuid.NewString()})
		if !errors.Is(err, ErrPlanNotFound) {
			t.Fatalf("foreign/missing condition must be masked 404: %v", err)
		}
	}
	draft := independentDraft("A")
	draft.RelatedConditionIDs = []uuid.UUID{condition}
	pending := independentProposal(t, svc, owner, draft, uuid.Nil, uuid.Nil)
	// Source authority can change without a Diagnosis revision. The frozen claim
	// says 'current' but must not override the live source's unavailable state.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, source).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.Accept(ctx, owner, independentDecision(pending))
	if !errors.Is(err, ErrPlanContextUnavailable) {
		t.Fatalf("stale source availability accepted: %v", err)
	}
	if err = db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, source).Update("availability", "available").Error; err != nil {
		t.Fatal(err)
	}
	// Typed evidence qualification is independently checked, even with no claim
	// references to that source. Clinical-only claims need not have assessment.
	if err = db.Exec("UPDATE condition_entries SET source_claim_refs='[]',assessment=?::jsonb WHERE user_id=? AND id=?", `{"level":"suspected","basedOnRunId":"`+conditionRunID(t, db, owner, condition).String()+`"}`, owner, condition).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, source).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	_, err = svc.Accept(ctx, owner, independentDecision(pending))
	if !errors.Is(err, ErrPlanContextUnavailable) {
		t.Fatalf("unavailable typed evidence accepted: %v", err)
	}
	// Missing linked condition is distinct from a legal empty Diagnosis.
	if err = db.Exec("DELETE FROM condition_entries WHERE user_id=? AND id=?", owner, condition).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Accept(ctx, owner, independentDecision(pending)); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("missing associated condition accepted: %v", err)
	}
	for _, table := range []string{"training_plan_revisions", "plan_decision_receipts"} {
		var count int64
		if err = db.Table(table).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("failed acceptance wrote %s: %d %v", table, count, err)
		}
	}
}

// A committed document review remains in diagnosis history after removal, but
// cannot authorize acceptance of a fresh TrainingPlan using an unavailable original.
func TestIndependentTrainingRemovedReportBlocksPlanAcceptancePostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	condition, _ := seedPlanDiagnosis(t, db, owner)
	var entry model.ConditionEntry
	if err := db.Where("user_id=? AND id=?", owner, condition).Take(&entry).Error; err != nil {
		t.Fatal(err)
	}
	assetID, extractionID, reviewID := uuid.New(), uuid.New(), uuid.New()
	hash := strings.Repeat("f", 64)
	if err := db.Exec(`INSERT INTO user_assets(id,user_id,kind,original_name,mime_type,byte_size,content_sha256,storage_backend,storage_key)
 VALUES (?,?,'report','plan-evidence.pdf','application/pdf',1,?,'local',?)`, assetID, owner, hash, "tp002/"+assetID.String()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO document_extraction_runs(id,user_id,asset_id,configuration_id,mechanism_revision,document_sha256,result_sha256,raw_text_sha256)
 VALUES (?,?,?,'synthetic','1',?,?,?)`, extractionID, owner, assetID, hash, hash, hash).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO document_indicator_reviews(id,user_id,asset_id,extraction_run_id,indicator_index,action,idempotency_key,reviewer_user_id)
 VALUES (?,?,?,?,0,'confirm',?,?)`, reviewID, owner, assetID, extractionID, uuid.NewString(), owner).Error; err != nil {
		t.Fatal(err)
	}
	action := "confirm"
	link := model.ConditionEntryEvidence{
		UserID: owner, ConditionID: condition,
		EvidenceType: model.DiagnosisDocumentEvidence, EvidenceID: reviewID, EvidenceVersion: 1,
		Role: model.DiagnosisSupporting, Rationale: "synthetic reviewed report",
		AnalyzedByRunID: *entry.LastAnalysisRunID, DocumentReviewAction: &action,
	}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewIndependentTrainingPlanService(db)
	draft := independentDraft("Reviewed report")
	draft.RelatedConditionIDs = []uuid.UUID{condition}
	pending := independentProposal(t, svc, owner, draft, uuid.Nil, uuid.Nil)

	// The immutable review is not erased, but the private source is unavailable
	// and must be re-checked under an owner-fenced SHARE lock on acceptance.
	if err := db.Exec("UPDATE user_assets SET availability='source_removed',source_removed_at=now() WHERE user_id=? AND id=?", owner, assetID).Error; err != nil {
		t.Fatal(err)
	}
	accepted, err := svc.Accept(context.Background(), owner, independentDecision(pending))
	if !errors.Is(err, ErrPlanContextUnavailable) || accepted != nil {
		t.Fatalf("removed report accepted into plan: receipt=%+v err=%v", accepted, err)
	}
	for _, table := range []string{"training_plan_revisions", "plan_decision_receipts"} {
		var count int64
		if err := db.Table(table).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("failed report-source admission wrote %s: %d %v", table, count, err)
		}
	}
}

func conditionRunID(t *testing.T, db *gorm.DB, owner, condition uuid.UUID) uuid.UUID {
	t.Helper()
	var entry model.ConditionEntry
	if err := db.Where("user_id=? AND id=?", owner, condition).Take(&entry).Error; err != nil {
		t.Fatal(err)
	}
	return *entry.LastAnalysisRunID
}
func TestIndependentTrainingEmptyAndUnreadableDiagnosisPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	var count int64
	if err := db.Model(&model.Diagnosis{}).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("fixture not empty: %d %v", count, err)
	}
	p := independentProposal(t, svc, owner, independentDraft("No Diagnosis required"), uuid.Nil, uuid.Nil)
	if _, err := svc.Accept(ctx, owner, independentDecision(p)); err != nil {
		t.Fatalf("legal empty initial Diagnosis blocked: %v", err)
	}
	if err := db.Model(&model.DiagnosisRevision{}).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("plan command created Diagnosis revision: %d %v", count, err)
	}
	linkedOwner := seedOnboardingContextUser(t, db)
	condition, _ := seedPlanDiagnosis(t, db, linkedOwner)
	draft := independentDraft("Linked")
	draft.RelatedConditionIDs = []uuid.UUID{condition}
	linked := independentProposal(t, svc, linkedOwner, draft, uuid.Nil, uuid.Nil)
	abort := errors.New("rollback synthetic unavailable source authority")
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("ALTER TABLE condition_entries RENAME TO tp002_unreadable_conditions").Error; err != nil {
			return err
		}
		_, err := NewIndependentTrainingPlanService(tx).Accept(ctx, linkedOwner, independentDecision(linked))
		if !errors.Is(err, ErrPlanContextUnavailable) {
			t.Errorf("unreadable related Diagnosis did not fail closed: %v", err)
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	if err = db.Model(&model.TrainingPlanRevision{}).Where("user_id=?", linkedOwner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("unreadable context wrote revision: %d %v", count, err)
	}
}
