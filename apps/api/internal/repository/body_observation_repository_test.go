package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func observationDirectCommand(revision int64, key, statement string) model.RecordBodyObservationCommand {
	return model.RecordBodyObservationCommand{IdempotencyKey: key, ExpectedRevision: revision,
		Observation: model.RecordBodyObservationInput{
			Kind: "body_weight", Statement: statement, RawValue: "67.5", RawUnit: "kg",
			Method: "home scale", ObservedAt: model.FactTemporal{Precision: "day", Start: "2026-10-13"},
		}}
}

func TestBodyObservationDirectReportSourceRevisionAndExactReplayPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	cmd := observationDirectCommand(0, "direct-weight-20261013", "My measured weight today was 67.5 kilograms")
	saved, err := repo.RecordDirectObservation(ctx, owner, cmd)
	if err != nil || saved == nil || saved.UserID != owner || saved.ID == uuid.Nil ||
		saved.ObservationID == uuid.Nil || saved.SourceEvidenceID == uuid.Nil ||
		saved.Revision != 1 || saved.EventID == uuid.Nil || saved.RevisionID == uuid.Nil {
		t.Fatalf("no atomic observation receipt: %#v %v", saved, err)
	}
	read, err := repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || read.CurrentRevision != 1 || len(read.Observations) != 1 ||
		read.Observations[0].ID != saved.ObservationID ||
		read.Observations[0].Source.ID != saved.SourceEvidenceID ||
		read.Observations[0].Source.Kind != "user_report" ||
		read.Observations[0].Source.Version != 1 ||
		read.Observations[0].Statement != cmd.Observation.Statement ||
		read.Observations[0].RawValue != "67.5" ||
		read.Observations[0].RawUnit != "kg" ||
		read.Observations[0].ObservedAt.Start != "2026-10-13" ||
		read.Observations[0].AssertionStatus != "asserted" {
		t.Fatalf("user-authored value/source/time projection invalid: %#v %v", read, err)
	}
	var row model.BodyObservation
	if err := db.Where("user_id=? AND id=?", owner, saved.ObservationID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	var source model.StatementEvidence
	if err := db.Where("user_id=? AND id=?", owner, saved.SourceEvidenceID).Take(&source).Error; err != nil {
		t.Fatal(err)
	}
	if row.Kind != "body_weight" || row.SourceEvidenceID != source.ID || source.SourceKind != "direct_edit" ||
		source.Excerpt != cmd.Observation.Statement || source.SourceMessageID != nil {
		t.Fatalf("source was fabricated or duplicate statement stored: row=%#v source=%#v", row, source)
	}
	other, err := repo.ListDirectObservations(ctx, foreign, 0)
	if err != nil || len(other.Observations) != 0 {
		t.Fatalf("foreign user read direct health value: %#v %v", other, err)
	}
	replay, err := repo.RecordDirectObservation(ctx, owner, cmd)
	if err != nil || replay.ID != saved.ID || replay.Revision != 1 || replay.ObservationID != saved.ObservationID {
		t.Fatalf("idempotency did not preserve exact receipt: %#v %v", replay, err)
	}
	changed := cmd
	changed.Observation.RawValue = "60"
	if _, err := repo.RecordDirectObservation(ctx, owner, changed); !errors.Is(err, model.ErrBodyObservationIdempotencyConflict) {
		t.Fatalf("changed body value silently reused authorization: %v", err)
	}
	changed.IdempotencyKey = "new-key-stale"
	if _, err := repo.RecordDirectObservation(ctx, owner, changed); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatalf("stale BodyState revision accepted: %v", err)
	}
	var revision model.BodyStateRevision
	if err := db.Where("user_id=? AND revision=1", owner).Take(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(revision.Changes), "67.5") ||
		strings.Contains(string(revision.Changes), cmd.Observation.Statement) ||
		strings.Contains(string(revision.Changes), "kg") {
		t.Fatalf("private original value copied into audit metadata: %s", revision.Changes)
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, source.ID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	after, err := repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || after.CurrentRevision != 1 || len(after.Observations) != 0 {
		t.Fatalf("blocked original health source still visible: %#v %v", after, err)
	}
	again, err := repo.RecordDirectObservation(ctx, owner, cmd)
	if err != nil || again.ID != saved.ID || again.Revision != 1 {
		t.Fatalf("same-key replay after block minted new evidence: %#v %v", again, err)
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, source.ID).
		Updates(map[string]any{"availability": "erased", "excerpt": ""}).Error; err != nil {
		t.Fatal(err)
	}
	removed, err := repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || len(removed.Observations) != 0 {
		t.Fatalf("erased source still displayed: %#v %v", removed, err)
	}
}

func TestBodyObservationConcurrentReplayRollbackAndAccountCascadePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	cmd := observationDirectCommand(0, "parallel-observation", "My weight measured 67.5 kg")
	const attempts = 6
	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	ids := make(chan uuid.UUID, attempts)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			saved, err := repo.RecordDirectObservation(context.Background(), owner, cmd)
			errs <- err
			if err == nil && saved != nil {
				ids <- saved.ObservationID
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent exact replay: %v", err)
		}
	}
	var first uuid.UUID
	for id := range ids {
		if first != uuid.Nil && first != id {
			t.Errorf("duplicated idempotent observation source")
		}
		first = id
	}
	if first == uuid.Nil {
		t.Fatal("no committed observation")
	}
	var count int64
	if err := db.Table("body_observations").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("concurrent insert duplicated records: count=%d err=%v", count, err)
	}
	other := seedBodyStateContextUser(t, db)
	injected := errors.New("synthetic late observation receipt failure")
	const callback = "observation_commit_rollback"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "body_observation_commits" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := repo.RecordDirectObservation(context.Background(), other, observationDirectCommand(0, "late-failure", "My current weight"))
	if err == nil || !errors.Is(err, injected) || saved != nil {
		t.Fatalf("late receipt failure was reported as a durable save: %#v %v", saved, err)
	}
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("body_observations").Where("user_id=?", other).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rollback left health record: %d %v", count, err)
	}
	if err := db.Table("statement_evidence").Where("user_id=?", other).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rollback left a private evidence excerpt: %d %v", count, err)
	}
	// The first attempt seeded body_states INSIDE the failed semantic
	// transaction. Full rollback may leave no aggregate row at all, which is
	// stronger than persisting a synthetic revision-zero aggregate.
	var state model.BodyState
	err = db.Where("user_id=?", other).Take(&state).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal(err)
	}
	if err == nil && state.CurrentRevision != 0 {
		t.Fatalf("failed semantic command incremented domain revision: %#v", state)
	}
	if err := db.Exec("DELETE FROM users WHERE id=?", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("body_observations").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("account removal retained observation: %d %v", count, err)
	}
	if err := db.Table("body_observation_commits").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("account removal retained observation receipt: %d %v", count, err)
	}
}

func TestBodyObservationCursorAndGuardedDowngradePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	for rev := int64(0); rev < 22; rev++ {
		cmd := observationDirectCommand(rev, uuid.NewString(), "I recorded a measurement at this time "+uuid.NewString())
		if _, err := repo.RecordDirectObservation(ctx, owner, cmd); err != nil {
			t.Fatalf("observe revision %d: %v", rev, err)
		}
	}
	newest, err := repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || newest.CurrentRevision != 22 || len(newest.Observations) != 20 ||
		!newest.HasMore || newest.NextBeforeRevision == nil || *newest.NextBeforeRevision != 3 {
		t.Fatalf("observation cursor leaked/discarded batches: %#v %v", newest, err)
	}
	older, err := repo.ListDirectObservations(ctx, owner, *newest.NextBeforeRevision)
	if err != nil || len(older.Observations) != 2 || older.HasMore ||
		older.Observations[0].CreatedRevision != 2 {
		t.Fatalf("older immutable revision pages not consistent: %#v %v", older, err)
	}
	if _, err := repo.ListDirectObservations(ctx, uuid.Nil, 0); !errors.Is(err, model.ErrInvalidBodyObservation) {
		t.Fatalf("anonymous read should fail: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000011_body_observation_direct_reports.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec(string(down)).Error; err == nil || !strings.Contains(err.Error(), "cannot downgrade: accepted BodyObservation records exist") {
		t.Fatalf("downgrade erased user measurements: %v", err)
	}
}

func TestBodyObservationCommitCannotBindUnrelatedOriginalSourcePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	first, err := repo.RecordDirectObservation(context.Background(), owner,
		observationDirectCommand(0, "first-source", "My first self-reported value"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.RecordDirectObservation(context.Background(), owner,
		observationDirectCommand(1, "second-source", "My next self-reported value"))
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceEvidenceID == other.SourceEvidenceID {
		t.Fatal("two direct reports silently shared one original source")
	}
	// A perfectly valid other source belonging to the SAME user cannot be
	// rebound to the first observation's authoritative commit receipt.
	if err := db.Exec("UPDATE public.body_observation_commits SET source_evidence_id=? WHERE user_id=? AND id=?",
		other.SourceEvidenceID, owner, first.ID).Error; err == nil {
		t.Fatal("observation receipt was allowed to claim the wrong user's own source")
	}
	var bound model.BodyObservationCommit
	if err := db.Where("user_id=? AND id=?", owner, first.ID).Take(&bound).Error; err != nil {
		t.Fatal(err)
	}
	if bound.SourceEvidenceID != first.SourceEvidenceID {
		t.Fatalf("bad source binding changed despite FK refusal: %#v", bound)
	}
}
