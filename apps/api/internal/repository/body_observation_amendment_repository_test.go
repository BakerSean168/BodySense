package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
)

func TestBodyObservationCorrectionAndWithdrawalImmutableSourcesPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	initial := observationDirectCommand(0, "measure-first", "I read 67.5 kilograms on my home scale")
	original, err := repo.RecordDirectObservation(ctx, owner, initial)
	if err != nil {
		t.Fatal(err)
	}
	amended := initial.Observation
	amended.Statement = "I misread the scale: my original reading was 65.7 kilograms"
	amended.RawValue = "65.7"
	correction := model.AmendBodyObservationCommand{IdempotencyKey: "correct-first", ExpectedRevision: 1, Replacement: &amended}
	saved, err := repo.CorrectDirectObservation(ctx, owner, original.ObservationID, correction)
	if err != nil || saved == nil || saved.Revision != 2 || saved.ObservationID == original.ObservationID ||
		saved.SourceEvidenceID == original.SourceEvidenceID || saved.Operation != "correct" ||
		saved.TargetObservationID == nil || *saved.TargetObservationID != original.ObservationID {
		t.Fatalf("original correction did not create a source-bound successor: %#v %v", saved, err)
	}
	replay, err := repo.CorrectDirectObservation(ctx, owner, original.ObservationID, correction)
	if err != nil || replay.ID != saved.ID || !replay.CommittedAt.Equal(saved.CommittedAt) {
		t.Fatalf("correction replay changed history or receipt: %#v %v", replay, err)
	}
	var old model.BodyObservation
	if err := db.Where("user_id=? AND id=?", owner, original.ObservationID).Take(&old).Error; err != nil {
		t.Fatal(err)
	}
	if old.AssertionStatus != "superseded" || old.SourceEvidenceID != original.SourceEvidenceID || old.RawValue != "67.5" {
		t.Fatalf("original user's words or source were rewritten: %#v", old)
	}
	var newRecord model.BodyObservation
	if err := db.Where("user_id=? AND id=?", owner, saved.ObservationID).Take(&newRecord).Error; err != nil {
		t.Fatal(err)
	}
	if newRecord.SupersedesObservationID == nil || *newRecord.SupersedesObservationID != old.ID || newRecord.RawValue != "65.7" {
		t.Fatalf("correction genealogy or raw value lost: %#v", newRecord)
	}
	page, err := repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || page.CurrentRevision != 2 || len(page.Observations) != 1 ||
		page.Observations[0].ID != saved.ObservationID ||
		page.Observations[0].Statement != amended.Statement ||
		page.Observations[0].RawValue != "65.7" {
		t.Fatalf("active observation view retained inaccurate old value: %#v %v", page, err)
	}
	var oldSource model.StatementEvidence
	if err := db.Where("user_id=? AND id=?", owner, original.SourceEvidenceID).Take(&oldSource).Error; err != nil {
		t.Fatal(err)
	}
	if oldSource.Excerpt != initial.Observation.Statement || oldSource.Availability != "available" {
		t.Fatalf("historical original is not immutable: %#v", oldSource)
	}
	if _, err := repo.CorrectDirectObservation(ctx, owner, old.ID,
		model.AmendBodyObservationCommand{IdempotencyKey: "correct-again", ExpectedRevision: 2, Replacement: &amended}); !errors.Is(err, model.ErrBodyObservationNotCurrent) {
		t.Fatalf("re-corrected old assertion: %v", err)
	}
	withdrawal := model.AmendBodyObservationCommand{IdempotencyKey: "withdraw-corrected", ExpectedRevision: 2}
	withdrawn, err := repo.WithdrawDirectObservation(ctx, owner, saved.ObservationID, withdrawal)
	if err != nil || withdrawn == nil || withdrawn.Revision != 3 ||
		withdrawn.ObservationID != saved.ObservationID || withdrawn.Operation != "withdraw" {
		t.Fatalf("withdraw failed: %#v %v", withdrawn, err)
	}
	retried, err := repo.WithdrawDirectObservation(ctx, owner, saved.ObservationID, withdrawal)
	if err != nil || retried.ID != withdrawn.ID || !retried.CommittedAt.Equal(withdrawn.CommittedAt) {
		t.Fatalf("withdraw replay minted another event: %#v %v", retried, err)
	}
	page, err = repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || page.CurrentRevision != 3 || len(page.Observations) != 0 {
		t.Fatalf("withdrawn original remains current: %#v %v", page, err)
	}
	if err := db.Where("user_id=? AND id=?", owner, saved.ObservationID).Take(&newRecord).Error; err != nil {
		t.Fatal(err)
	}
	if newRecord.AssertionStatus != "withdrawn" {
		t.Fatal("withdrawing user statement erased or changed other health identity")
	}
	var revisions []model.BodyStateRevision
	if err := db.Where("user_id=?", owner).Order("revision").Find(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 3 {
		t.Fatalf("expected one BodyState revision per command: %d", len(revisions))
	}
	for _, rev := range revisions {
		if strings.Contains(string(rev.Changes), initial.Observation.Statement) ||
			strings.Contains(string(rev.Changes), amended.Statement) ||
			strings.Contains(string(rev.Changes), "65.7") {
			t.Fatalf("private originals copied into generic domain audit: %s", rev.Changes)
		}
	}
	if err := db.Exec("DELETE FROM users WHERE id=?", owner).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"body_observation_commits", "body_observations", "statement_evidence"} {
		var count int64
		if err := db.Table(table).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("account cascade retained %s: %d %v", table, count, err)
		}
	}
}
