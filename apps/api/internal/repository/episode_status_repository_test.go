package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func episodeRecordCourseFact(t *testing.T, repo *BodyStateRepository, owner uuid.UUID, expected int64, statement, presence, course, at string) model.BodyFactCommit {
	t.Helper()
	details, err := json.Marshal(model.SymptomFactDetails{SymptomType: "shoulder pain", Presence: presence, Laterality: "right", EpisodeCourseReport: course})
	if err != nil {
		t.Fatal(err)
	}
	c := model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: expected, Facts: []model.BodyFactInput{{
		Kind: "symptom", Statement: statement, BodyRegionIDs: []string{},
		Temporal: model.FactTemporal{Precision: "day", Start: at}, Details: json.RawMessage(details),
	}}}
	receipt, err := repo.RecordDirectFacts(context.Background(), owner, c)
	if err != nil {
		t.Fatalf("record source: %v", err)
	}
	return *receipt
}

func TestSymptomEpisodeStatusSourceAdmissionAndRevisionReplayPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()

	startText := "I still have shoulder pain on Oct 12"
	start := episodeRecordCourseFact(t, repo, owner, 0, startText, "present", "explicit_current_report", "2026-10-12")
	sourceStart := episodeLookupFact(t, db, owner, startText)
	created, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(sourceStart.ID, start.Revision, "status-create"))
	if err != nil || created.Revision != 2 {
		t.Fatalf("create: %#v %v", created, err)
	}
	ongoingCommand := episodeAssociationCommand(sourceStart.ID, 2, "accept-ongoing")
	ongoing, err := repo.AdmitEpisodeStatus(ctx, owner, created.EpisodeID, ongoingCommand)
	if err != nil || ongoing.Operation != "status" || ongoing.Revision != 3 ||
		ongoing.EpisodeID != created.EpisodeID || ongoing.RevisionID == uuid.Nil || ongoing.EventID == uuid.Nil {
		t.Fatalf("explicit current report has no durable receipt: %#v %v", ongoing, err)
	}
	page, err := repo.GetEpisode(ctx, owner, created.EpisodeID)
	if err != nil || page.Projection.State != "ongoing" ||
		page.Projection.Reason != "explicit_current_report" || page.Projection.StateAsOf != "2026-10-12" ||
		len(page.Projection.BasisFactIDs) != 1 || page.Projection.BasedOnRevision != 3 {
		t.Fatalf("read failed to project accepted user current report: %#v %v", page, err)
	}
	replay, err := repo.AdmitEpisodeStatus(ctx, owner, created.EpisodeID, ongoingCommand)
	if err != nil || replay.ID != ongoing.ID || replay.Revision != 3 {
		t.Fatalf("status idempotency broke: %#v %v", replay, err)
	}
	wrong := ongoingCommand
	wrong.FactID = uuid.New()
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, created.EpisodeID, wrong); !errors.Is(err, model.ErrEpisodeIdempotencyConflict) {
		t.Fatalf("fingerprint did not block command key reassignment: %v", err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, other, created.EpisodeID, episodeAssociationCommand(sourceStart.ID, 0, "foreign-status")); !errors.Is(err, ErrEpisodeNotFound) {
		t.Fatalf("other user had access to Episode state: %v", err)
	}

	endedText := "The same shoulder pain completely ended by Oct 14"
	endCommit := episodeRecordCourseFact(t, repo, owner, 3, endedText, "absent", "explicit_resolution", "2026-10-14")
	endFact := episodeLookupFact(t, db, owner, endedText)
	// Not yet associated, so an actual explicit Fact can never mutate course by itself.
	noLink := episodeAssociationCommand(endFact.ID, endCommit.Revision, "status-not-linked")
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, created.EpisodeID, noLink); !errors.Is(err, model.ErrEpisodeFactUnavailable) {
		t.Fatalf("status command bypassed primary source link: %v", err)
	}
	beforeLink, err := repo.GetEpisode(ctx, owner, created.EpisodeID)
	if err != nil || beforeLink.Projection.State != "ongoing" {
		t.Fatalf("unlinked Fact silently changed course: %#v %v", beforeLink, err)
	}
	linked, err := repo.LinkEpisodeFact(ctx, owner, created.EpisodeID, episodeAssociationCommand(endFact.ID, 4, "status-link-end"))
	if err != nil || linked.Revision != 5 {
		t.Fatalf("could not link explicit resolution source: %#v %v", linked, err)
	}
	beforeStatus, err := repo.GetEpisode(ctx, owner, created.EpisodeID)
	if err != nil || beforeStatus.Projection.State != "ongoing" {
		t.Fatalf("ordinary link silently asserted resolution: %#v %v", beforeStatus, err)
	}
	accepted, err := repo.AdmitEpisodeStatus(ctx, owner, created.EpisodeID, episodeAssociationCommand(endFact.ID, 5, "status-accept-end"))
	if err != nil || accepted.Revision != 6 || accepted.Operation != "status" {
		t.Fatalf("resolution evidence not durable: %#v %v", accepted, err)
	}
	ended, err := repo.GetEpisode(ctx, owner, created.EpisodeID)
	if err != nil || ended.Projection.State != "ended" || ended.Projection.Reason != "explicit_resolution" ||
		ended.Projection.StateAsOf != "2026-10-14" || len(ended.Projection.BasisFactIDs) != 2 {
		t.Fatalf("explicit linked source failed to end reported course: %#v %v", ended, err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, created.EpisodeID, episodeAssociationCommand(endFact.ID, 6, "another-key-same-basis")); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("already accepted status was duplicated: %v", err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, created.EpisodeID, episodeAssociationCommand(sourceStart.ID, 5, "stale-status")); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	// After the end fact is explicitly withdrawn, source-bound projection
	// must revoke "ended" without pretending that withdrawal was recovery.
	if err := db.Model(&model.BodyFact{}).Where("user_id=? AND id=?", owner, endFact.ID).Update("assertion_status", "withdrawn").Error; err != nil {
		t.Fatal(err)
	}
	revised, err := repo.GetEpisode(ctx, owner, created.EpisodeID)
	if err != nil || revised.Projection.State != "ongoing" || len(revised.Projection.BasisFactIDs) != 1 ||
		strings.Contains(revised.Projection.StateAsOf, "2026-10-14") {
		t.Fatalf("withdrawn claim still defines course: %#v %v", revised, err)
	}
	var count int64
	if err := db.Table("episode_state_evidence").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("status basis not stored in one canonical table: %d %v", count, err)
	}
	if err := db.Table("episode_command_receipts").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 4 {
		t.Fatalf("statuses lack replayable durable receipt: %d %v", count, err)
	}
	// No original health statement should appear in generic audit metadata.
	var revisions []model.BodyStateRevision
	if err := db.Where("user_id=?", owner).Find(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	for _, rev := range revisions {
		audit := string(rev.Changes)
		if strings.Contains(audit, startText) || strings.Contains(audit, endedText) {
			t.Fatalf("private source copied into audit revision: %s", audit)
		}
	}
}

func TestSymptomEpisodeStatusDeniesGenericNegativeAndUnavailableSourcePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	first := episodeRecordCourseFact(t, repo, owner, 0, "Shoulder pain is present", "present", "", "2026-10-10")
	positive := episodeLookupFact(t, db, owner, "Shoulder pain is present")
	ep, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(positive.ID, first.Revision, "generic-create"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, ep.EpisodeID, episodeAssociationCommand(positive.ID, 2, "present-not-current")); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("bare presence was treated as current evidence: %v", err)
	}
	absent := episodeRecordCourseFact(t, repo, owner, 2, "I have no shoulder pain today", "absent", "", "2026-10-12")
	negative := episodeLookupFact(t, db, owner, "I have no shoulder pain today")
	linked, err := repo.LinkEpisodeFact(ctx, owner, ep.EpisodeID, episodeAssociationCommand(negative.ID, absent.Revision, "generic-link"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, ep.EpisodeID, episodeAssociationCommand(negative.ID, linked.Revision, "generic-negative")); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("ordinary negative report ended course: %v", err)
	}
	page, err := repo.GetEpisode(ctx, owner, ep.EpisodeID)
	if err != nil || page.Projection.State != "unknown" {
		t.Fatalf("generic absence misclassified: %#v %v", page, err)
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, negative.SourceEvidenceID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, ep.EpisodeID, episodeAssociationCommand(negative.ID, linked.Revision, "source-blocked")); !errors.Is(err, model.ErrEpisodeFactUnavailable) {
		t.Fatalf("blocked original source authorized a status: %v", err)
	}
	var count int64
	if err := db.Table("episode_state_evidence").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("denied evidence left a source-of-truth row: %d %v", count, err)
	}
}

func TestSymptomEpisodeStatusConcurrentReplayAndRollbackPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	first := episodeRecordCourseFact(t, repo, owner, 0, "My shoulder still hurts", "present", "explicit_current_report", "2026-10-13")
	fact := episodeLookupFact(t, db, owner, "My shoulder still hurts")
	ep, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(fact.ID, first.Revision, "parallel-status-create"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := episodeAssociationCommand(fact.ID, 2, "parallel-status")
	const callers = 6
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	ids := make(chan uuid.UUID, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := repo.AdmitEpisodeStatus(context.Background(), owner, ep.EpisodeID, cmd)
			if err == nil && r != nil {
				ids <- r.ID
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent same-key replay: %v", err)
		}
	}
	var prior uuid.UUID
	for id := range ids {
		if prior != uuid.Nil && prior != id {
			t.Error("different receipts for same semantic commit")
		}
		prior = id
	}
	if prior == uuid.Nil {
		t.Fatal("no accepted status")
	}
	var count int64
	if err := db.Table("episode_state_evidence").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("concurrent status evidence duplicated: %d %v", count, err)
	}
	var state model.BodyState
	if err := db.Where("user_id=?", owner).First(&state).Error; err != nil || state.CurrentRevision != 3 {
		t.Fatalf("concurrent status command incremented multiple times: %#v %v", state, err)
	}

	// A second user and a failing receipt step must never leave a half-revision.
	other := seedBodyStateContextUser(t, db)
	otherRecord := episodeRecordCourseFact(t, repo, other, 0, "My own shoulder hurts", "present", "explicit_current_report", "2026-10-13")
	otherFact := episodeLookupFact(t, db, other, "My own shoulder hurts")
	otherEP, err := repo.CreateEpisode(ctx, other, episodeAssociationCommand(otherFact.ID, otherRecord.Revision, "rollback-create"))
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected late status receipt rollback")
	const callback = "episode_status_receipt_rollback"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "episode_command_receipts" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	saved, err := repo.AdmitEpisodeStatus(ctx, other, otherEP.EpisodeID, episodeAssociationCommand(otherFact.ID, 2, "rollback-status"))
	if saved != nil || !errors.Is(err, injected) {
		t.Fatalf("late failed write returned receipt: %#v %v", saved, err)
	}
	if err := db.Table("episode_state_evidence").Where("user_id=?", other).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rollback left status basis: %d %v", count, err)
	}
	var otherState model.BodyState
	if err := db.Where("user_id=?", other).First(&otherState).Error; err != nil || otherState.CurrentRevision != 2 {
		t.Fatalf("rollback persisted revision: %#v %v", otherState, err)
	}
}

func TestSymptomEpisodeStatusV9DowngradeRefusesAcceptedReceiptsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	episodeRecordCourseFact(t, repo, owner, 0, "Pain ongoing", "present", "explicit_current_report", "2026-10-13")
	fact := episodeLookupFact(t, db, owner, "Pain ongoing")
	ep, err := repo.CreateEpisode(context.Background(), owner, episodeAssociationCommand(fact.ID, 1, "status-downgrade-create"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(context.Background(), owner, ep.EpisodeID, episodeAssociationCommand(fact.ID, 2, "status-downgrade")); err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000009_episode_status_receipts.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec(string(down)).Error; err == nil || !strings.Contains(err.Error(), "cannot downgrade: Episode status receipts exist") {
		t.Fatalf("v9 destructively downgraded retained user authorization: %v", err)
	}
}

func TestSymptomEpisodeStatusRecurrenceRequiresNewIdentityPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	first := episodeRecordCourseFact(t, repo, owner, 0, "Pain was present", "present", "explicit_current_report", "2026-10-01")
	current := episodeLookupFact(t, db, owner, "Pain was present")
	ep, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(current.ID, first.Revision, "recurrence-create"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, ep.EpisodeID, episodeAssociationCommand(current.ID, 2, "recurrence-current")); err != nil {
		t.Fatal(err)
	}
	resolution := episodeRecordCourseFact(t, repo, owner, 3, "Pain completely ended", "absent", "explicit_resolution", "2026-10-04")
	resolved := episodeLookupFact(t, db, owner, "Pain completely ended")
	if _, err := repo.LinkEpisodeFact(ctx, owner, ep.EpisodeID, episodeAssociationCommand(resolved.ID, resolution.Revision, "recurrence-link-end")); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, ep.EpisodeID, episodeAssociationCommand(resolved.ID, 5, "recurrence-end")); err != nil {
		t.Fatal(err)
	}
	after, err := repo.GetEpisode(ctx, owner, ep.EpisodeID)
	if err != nil || after.Projection.State != "ended" {
		t.Fatalf("basis not ended: %#v %v", after, err)
	}
	newReport := episodeRecordCourseFact(t, repo, owner, 6, "The pain has returned as a new occurrence", "present", "explicit_current_report", "2026-10-08")
	returning := episodeLookupFact(t, db, owner, "The pain has returned as a new occurrence")
	if _, err := repo.LinkEpisodeFact(ctx, owner, ep.EpisodeID, episodeAssociationCommand(returning.ID, newReport.Revision, "invalid-reopen")); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("new episode silently reopened ended course: %v", err)
	}
	separate, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(returning.ID, newReport.Revision, "distinct-recurrence"))
	if err != nil || separate.EpisodeID == ep.EpisodeID || separate.Revision != 8 {
		t.Fatalf("legitimate separate recurrence identity rejected: %#v %v", separate, err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, separate.EpisodeID, episodeAssociationCommand(returning.ID, 8, "recurrence-new-current")); err != nil {
		t.Fatal(err)
	}
	older, err := repo.GetEpisode(ctx, owner, ep.EpisodeID)
	if err != nil || older.Projection.State != "ended" {
		t.Fatalf("new recurrence mutated ended identity: %#v %v", older, err)
	}
	latest, err := repo.GetEpisode(ctx, owner, separate.EpisodeID)
	if err != nil || latest.Projection.State != "ongoing" {
		t.Fatalf("new recurrence did not retain own source state: %#v %v", latest, err)
	}
}

func TestSymptomEpisodeStatusRejectsUnverifiedMessageProvenancePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	statement := "My shoulder symptoms remain present"
	parts, err := json.Marshal([]map[string]any{{"type": "text", "text": statement}})
	if err != nil {
		t.Fatal(err)
	}
	source := seedUserMessageFactEnvelope(t, db, owner, datatypes.JSON(parts), "user")
	command := model.RecordBodyFactsCommand{
		IdempotencyKey: "unverified-message-fact", ExpectedRevision: 0,
		Facts: []model.BodyFactInput{{
			Kind: "symptom", Statement: statement, BodyRegionIDs: []string{},
			Temporal: model.FactTemporal{Precision: "day", Start: "2026-10-13"},
			Details:  json.RawMessage(`{"symptom_type":"shoulder pain","presence":"present","episode_course_report":"explicit_current_report"}`),
		}},
	}
	first, err := repo.RecordMessageFacts(ctx, owner, source, command)
	if err != nil || first == nil || first.Revision != 1 {
		t.Fatalf("source-bound message Fact: %#v %v", first, err)
	}
	fact := episodeLookupFact(t, db, owner, statement)
	var original model.StatementEvidence
	if err := db.Where("user_id=? AND id=?", owner, fact.SourceEvidenceID).First(&original).Error; err != nil {
		t.Fatal(err)
	}
	if original.SourceKind != "user_message" || original.SourceMessageID == nil {
		t.Fatalf("fixture was not verified chat-message provenance: %#v", original)
	}
	ep, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(fact.ID, first.Revision, "unreviewed-provenance-create"))
	if err != nil {
		t.Fatal(err)
	}
	// AI/Chat-derived interpreted course labels are NOT a direct, independently
	// confirmed user command and may not enter the narrow admission boundary.
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, ep.EpisodeID, episodeAssociationCommand(fact.ID, 2, "unreviewed-provenance-status")); !errors.Is(err, model.ErrEpisodeFactUnavailable) {
		t.Fatalf("unreviewed message-derived status impersonated direct-user choice: %v", err)
	}
	var count int64
	if err := db.Table("episode_state_evidence").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("denied model interpretation became a status: count=%d err=%v", count, err)
	}
}
