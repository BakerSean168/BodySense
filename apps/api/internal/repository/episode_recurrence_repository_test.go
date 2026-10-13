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
	"gorm.io/gorm"
)

type recurrenceFixture struct {
	repo     *BodyStateRepository
	owner    uuid.UUID
	old      uuid.UUID
	newer    uuid.UUID
	oldStart model.BodyFact
	oldEnd   model.BodyFact
	newFact  model.BodyFact
	revision int64
}

// Two independently user-authorized Episode identities. The old Episode is
// explicitly ended, and the newer source is separately ongoing. No relation
// has been inferred or persisted by the setup.
func prepareRecurrenceFixture(t *testing.T, db *gorm.DB, newDate string) recurrenceFixture {
	t.Helper()
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	start := episodeRecordCourseFact(t, repo, owner, 0, "Old pain was ongoing", "present", "explicit_current_report", "2026-10-01")
	oldStart := episodeLookupFact(t, db, owner, "Old pain was ongoing")
	old, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(oldStart.ID, start.Revision, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, old.EpisodeID, episodeAssociationCommand(oldStart.ID, 2, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	end := episodeRecordCourseFact(t, repo, owner, 3, "The previous pain completely ended", "absent", "explicit_resolution", "2026-10-04")
	oldEnd := episodeLookupFact(t, db, owner, "The previous pain completely ended")
	if _, err := repo.LinkEpisodeFact(ctx, owner, old.EpisodeID, episodeAssociationCommand(oldEnd.ID, end.Revision, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, old.EpisodeID, episodeAssociationCommand(oldEnd.ID, 5, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	newer := episodeRecordCourseFact(t, repo, owner, 6, "I have another occurrence of the pain", "present", "explicit_current_report", newDate)
	newFact := episodeLookupFact(t, db, owner, "I have another occurrence of the pain")
	newest, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(newFact.ID, newer.Revision, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AdmitEpisodeStatus(ctx, owner, newest.EpisodeID, episodeAssociationCommand(newFact.ID, 8, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	oldRead, err := repo.GetEpisode(ctx, owner, old.EpisodeID)
	if err != nil || oldRead.Projection.State != "ended" {
		t.Fatalf("old Episode lost its independent end: %#v %v", oldRead, err)
	}
	newRead, err := repo.GetEpisode(ctx, owner, newest.EpisodeID)
	if err != nil || newRead.Projection.State != "ongoing" {
		t.Fatalf("new Episode not independent: %#v %v", newRead, err)
	}
	return recurrenceFixture{repo: repo, owner: owner, old: old.EpisodeID, newer: newest.EpisodeID, oldStart: oldStart, oldEnd: oldEnd, newFact: newFact, revision: 9}
}
func recurrenceCommand(f recurrenceFixture, key string) model.EpisodeRecurrenceCommand {
	return model.EpisodeRecurrenceCommand{IdempotencyKey: key, ExpectedRevision: f.revision, NewEpisodeID: f.newer, FactID: f.newFact.ID}
}
func TestSymptomEpisodeRecurrenceDirectAuthorizationReadAndExactReplayPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := prepareRecurrenceFixture(t, db, "2026-10-08")
	ctx := context.Background()
	key := "first-explicit-recurrence"
	cmd := recurrenceCommand(f, key)
	saved, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, cmd)
	if err != nil || saved == nil || saved.Revision != 10 || saved.ID == uuid.Nil || saved.RelationID == uuid.Nil ||
		saved.FromEpisodeID != f.old || saved.ToEpisodeID != f.newer || saved.FactID != f.newFact.ID ||
		saved.RevisionID == uuid.Nil || saved.EventID == uuid.Nil {
		t.Fatalf("exact user recurrence not durably committed: %#v %v", saved, err)
	}
	both := []uuid.UUID{f.old, f.newer}
	for _, target := range both {
		page, err := f.repo.ListEpisodeRelations(ctx, f.owner, target, 0)
		if err != nil || page.CurrentRevision != 10 || page.EpisodeID != target ||
			len(page.Relations) != 1 || page.Relations[0].ID != saved.RelationID ||
			page.Relations[0].FromEpisodeID != f.old ||
			page.Relations[0].ToEpisodeID != f.newer ||
			page.Relations[0].AuthorizationID != saved.ID ||
			page.Relations[0].ReviewState != "supported" || page.HasMore {
			t.Fatalf("missing canonical source-backed relation: %#v %v", page, err)
		}
	}
	oldRead, err := f.repo.GetEpisode(ctx, f.owner, f.old)
	if err != nil || oldRead.Projection.State != "ended" || len(oldRead.Projection.BasisFactIDs) != 2 {
		t.Fatalf("relation improperly reopened or copied old Episode: %#v %v", oldRead, err)
	}
	newRead, err := f.repo.GetEpisode(ctx, f.owner, f.newer)
	if err != nil || newRead.Projection.State != "ongoing" || len(newRead.Projection.BasisFactIDs) != 1 {
		t.Fatalf("relation modified new Episode clinical state: %#v %v", newRead, err)
	}
	again, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, cmd)
	if err != nil || again.ID != saved.ID || again.RelationID != saved.RelationID || again.Revision != 10 {
		t.Fatalf("idempotent POST did not return original relation: %#v %v", again, err)
	}
	sameKeyDifferent := cmd
	sameKeyDifferent.FactID = f.oldStart.ID
	if _, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, sameKeyDifferent); !errors.Is(err, model.ErrEpisodeIdempotencyConflict) {
		t.Fatalf("relation key was rebound to a different Fact: %v", err)
	}
	if _, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, recurrenceCommand(f, uuid.NewString())); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatalf("stale old snapshot allowed duplicate: %v", err)
	}
	var count int64
	if err := db.Table("episode_relations").Where("user_id=?", f.owner).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("one user action produced multiple relation rows: %d %v", count, err)
	}
	if err := db.Table("episode_command_receipts").Where("user_id=? AND operation='recurrence'", f.owner).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("relation lacks one exact durable authorization: %d %v", count, err)
	}
	var revision model.BodyStateRevision
	if err := db.Where("user_id=? AND revision=?", f.owner, 10).First(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(revision.Changes), "Old pain was ongoing") ||
		strings.Contains(string(revision.Changes), "The previous pain") || strings.Contains(string(revision.Changes), "another occurrence") {
		t.Fatalf("source text leaked into generic audit: %s", revision.Changes)
	}
}
func TestSymptomEpisodeRecurrenceRejectsImplicitOldCourseAndConflictingChronologyPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := prepareRecurrenceFixture(t, db, "2026-10-02")
	cmd := recurrenceCommand(f, "chronology-invalid")
	if _, err := f.repo.ReportEpisodeRecurrence(context.Background(), f.owner, f.old, cmd); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("new report occurring before the old course ended became a recurrence: %v", err)
	}
	if err := db.Model(&model.BodyFact{}).Where("user_id=? AND id=?", f.owner, f.oldEnd.ID).Update("assertion_status", "withdrawn").Error; err != nil {
		t.Fatal(err)
	}
	cmd.IdempotencyKey = "old-proof-withdrawn"
	if _, err := f.repo.ReportEpisodeRecurrence(context.Background(), f.owner, f.old, cmd); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("withdrawn old explicit ending authorized recurrence: %v", err)
	}
	var count int64
	if err := db.Table("episode_relations").Where("user_id=?", f.owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("denied relation left durable row: %d %v", count, err)
	}
}
func TestSymptomEpisodeRecurrenceOwnerSourceWithdrawalAndReviewPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := prepareRecurrenceFixture(t, db, "2026-10-08")
	ctx := context.Background()
	foreign := seedBodyStateContextUser(t, db)
	if _, err := f.repo.ReportEpisodeRecurrence(ctx, foreign, f.old, recurrenceCommand(f, "foreign-denied")); !errors.Is(err, ErrEpisodeNotFound) {
		t.Fatalf("foreign Episode was exposed by write: %v", err)
	}
	if _, err := f.repo.ListEpisodeRelations(ctx, foreign, f.old, 0); !errors.Is(err, ErrEpisodeNotFound) {
		t.Fatalf("foreign Episode was exposed by read: %v", err)
	}
	if _, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, model.EpisodeRecurrenceCommand{
		IdempotencyKey: "same-identity", ExpectedRevision: f.revision, NewEpisodeID: f.old, FactID: f.oldStart.ID,
	}); !errors.Is(err, model.ErrInvalidEpisodeEvidence) {
		t.Fatalf("self-loop accepted: %v", err)
	}
	if _, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, model.EpisodeRecurrenceCommand{
		IdempotencyKey: "foreign-fact", ExpectedRevision: f.revision, NewEpisodeID: f.newer, FactID: f.oldStart.ID,
	}); !errors.Is(err, model.ErrEpisodeFactUnavailable) {
		t.Fatalf("new basis not its primary source: %v", err)
	}
	report, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, recurrenceCommand(f, "valid-before-correction"))
	if err != nil || report == nil {
		t.Fatalf("could not write valid relationship: %#v %v", report, err)
	}
	// Only old end evidence is corrected: keep the historical user action,
	// but never describe the relation as still supported.
	if err := db.Model(&model.BodyFact{}).Where("user_id=? AND id=?", f.owner, f.oldEnd.ID).
		Update("assertion_status", "withdrawn").Error; err != nil {
		t.Fatal(err)
	}
	page, err := f.repo.ListEpisodeRelations(ctx, f.owner, f.old, 0)
	if err != nil || len(page.Relations) != 1 || page.Relations[0].ReviewState != "needs_review" {
		t.Fatalf("corrected old end failed relation review downgrade: %#v %v", page, err)
	}
	// Block the new basis original; the relation must now be invisible from
	// the old Episode, and the source-only new Episode hidden as not found.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.newFact.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	hidden, err := f.repo.ListEpisodeRelations(ctx, f.owner, f.old, 0)
	if err != nil || len(hidden.Relations) != 0 {
		t.Fatalf("blocked new source leaked relation: %#v %v", hidden, err)
	}
	if _, err := f.repo.ListEpisodeRelations(ctx, f.owner, f.newer, 0); !errors.Is(err, ErrEpisodeNotFound) {
		t.Fatalf("blocked Episode identity remained visible: %v", err)
	}
	// Exact retry after source deletion cannot create a second authorization.
	replay, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, recurrenceCommand(f, "valid-before-correction"))
	if err != nil || replay.ID != report.ID || replay.RelationID != report.RelationID {
		t.Fatalf("removed-source retry created new authorization: %#v %v", replay, err)
	}
}
func TestSymptomEpisodeRecurrenceConcurrentSameKeyAndAtomicRollbackPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := prepareRecurrenceFixture(t, db, "2026-10-08")
	ctx := context.Background()
	cmd := recurrenceCommand(f, "parallel-report")
	const parallel = 6
	var wg sync.WaitGroup
	errs := make(chan error, parallel)
	ids := make(chan uuid.UUID, parallel)
	for range parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec, err := f.repo.ReportEpisodeRecurrence(context.Background(), f.owner, f.old, cmd)
			errs <- err
			if err == nil && rec != nil {
				ids <- rec.RelationID
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		if err != nil {
			t.Errorf("same-key parallel recurrence failed: %v", err)
		}
	}
	var relation uuid.UUID
	for id := range ids {
		if relation != uuid.Nil && relation != id {
			t.Error("same-key produced different relation IDs")
		}
		relation = id
	}
	if relation == uuid.Nil {
		t.Fatal("no receipt for concurrent user report")
	}
	var count int64
	if err := db.Table("episode_relations").Where("user_id=?", f.owner).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("concurrent recurrence did not serialize: %d %v", count, err)
	}
	// Different isolated user: late relation insert fails after already
	// building revision, outbox and authorization receipt.
	h := prepareRecurrenceFixture(t, db, "2026-10-08")
	injected := errors.New("synthetic recurrence relation insert failure")
	const cb = "episode_relation_injected_rollback"
	if err := db.Callback().Create().Before("gorm:create").Register(cb, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "episode_relations" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(cb) })
	saved, err := h.repo.ReportEpisodeRecurrence(ctx, h.owner, h.old, recurrenceCommand(h, "failing-late-relation"))
	if saved != nil || !errors.Is(err, injected) {
		t.Fatalf("late relation failure returned successful commit: %#v %v", saved, err)
	}
	if err := db.Table("episode_relations").Where("user_id=?", h.owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rollback retained relation row: %d %v", count, err)
	}
	if err := db.Table("episode_command_receipts").Where("user_id=? AND operation='recurrence'", h.owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rollback retained accepted command receipt: %d %v", count, err)
	}
	var state model.BodyState
	if err := db.Where("user_id=?", h.owner).First(&state).Error; err != nil || state.CurrentRevision != 9 {
		t.Fatalf("rollback incremented domain revision: %#v %v", state, err)
	}
}
func TestSymptomEpisodeRecurrenceV10GuardedDownAndAccountCascadePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := prepareRecurrenceFixture(t, db, "2026-10-08")
	if _, err := f.repo.ReportEpisodeRecurrence(context.Background(), f.owner, f.old, recurrenceCommand(f, "durable-down-guard")); err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000010_episode_recurrence_links.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec(string(down)).Error; err == nil || !strings.Contains(err.Error(), "cannot downgrade: accepted Episode recurrence relations exist") {
		t.Fatalf("v10 downgraded user-accepted relation without protection: %v", err)
	}
	// Account cascade removes authorized relation and associated proof IDs.
	if err := db.Exec("DELETE FROM users WHERE id=?", f.owner).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("episode_relations").Where("user_id=?", f.owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("account deletion retained recurrence relation: %d %v", count, err)
	}
}

func TestSymptomEpisodeRecurrenceCycleRejectedEvenWhenBusinessTimeUnknownPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := prepareRecurrenceFixture(t, db, "2026-10-08")
	ctx := context.Background()
	if _, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.old, recurrenceCommand(f, "forward-old-to-new")); err != nil {
		t.Fatal(err)
	}
	// The new Episode later closes, so a second direct user request could
	// otherwise invert the recorded relation as new->old (a lineage cycle).
	ended := episodeRecordCourseFact(t, f.repo, f.owner, 10, "The second episode ended", "absent", "explicit_resolution", "2026-10-12")
	secondEnd := episodeLookupFact(t, db, f.owner, "The second episode ended")
	if _, err := f.repo.LinkEpisodeFact(ctx, f.owner, f.newer, episodeAssociationCommand(secondEnd.ID, ended.Revision, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.AdmitEpisodeStatus(ctx, f.owner, f.newer, episodeAssociationCommand(secondEnd.ID, 12, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	// This is an ordinary positive fact with UNKNOWN business time. It can
	// be observed on an ended historical Episode without reopening it; the
	// unknown time must not override the DAG cycle safeguard.
	report := model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 13,
		Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "A retrospective observation, date unknown",
			BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"},
			Details: json.RawMessage(`{"symptom_type":"shoulder pain","presence":"present","laterality":"right"}`)}}}
	recorded, err := f.repo.RecordDirectFacts(ctx, f.owner, report)
	if err != nil || recorded.Revision != 14 {
		t.Fatalf("unknown time observation rejected: %#v %v", recorded, err)
	}
	oldFact := episodeLookupFact(t, db, f.owner, "A retrospective observation, date unknown")
	if _, err := f.repo.LinkEpisodeFact(ctx, f.owner, f.old, episodeAssociationCommand(oldFact.ID, 14, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	backward := model.EpisodeRecurrenceCommand{IdempotencyKey: "inverse-cycle-should-not-write", ExpectedRevision: 15,
		NewEpisodeID: f.old, FactID: oldFact.ID}
	if _, err := f.repo.ReportEpisodeRecurrence(ctx, f.owner, f.newer, backward); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("inverse recurrence formed a cycle: %v", err)
	}
	var count int64
	if err := db.Table("episode_relations").Where("user_id=?", f.owner).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("cycle attempt persisted a second edge: %d %v", count, err)
	}
}
