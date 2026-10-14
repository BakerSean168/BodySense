package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Reusing one command key across different authenticated users must never
// replay another tenant's commit, even if that tenant's payload or revision
// differs. All three direct-user operations share the observation receipt
// table, so exercise their complete lifecycle against real PostgreSQL.
func TestBodyObservationSharedIdempotencyKeysIsolatedAcrossTenantsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()

	type lifecycle struct {
		owner    uuid.UUID
		receipts [3]*model.BodyObservationCommit
		commands struct {
			record   model.RecordBodyObservationCommand
			correct  model.AmendBodyObservationCommand
			withdraw model.AmendBodyObservationCommand
		}
	}
	var users [2]lifecycle
	for i := range users {
		user := &users[i]
		user.owner = seedBodyStateContextUser(t, db)
		user.commands.record = observationDirectCommand(0, "shared-record", fmt.Sprintf("User %d reported 67.5 kg", i))
		initial, err := repo.RecordDirectObservation(ctx, user.owner, user.commands.record)
		if err != nil || initial == nil {
			t.Fatalf("tenant %d record: %#v %v", i, initial, err)
		}
		user.receipts[0] = initial

		replacement := user.commands.record.Observation
		replacement.Statement = fmt.Sprintf("User %d corrected the original scale reading", i)
		replacement.RawValue = fmt.Sprintf("%d.5", 65+i)
		user.commands.correct = model.AmendBodyObservationCommand{
			IdempotencyKey: "shared-correct", ExpectedRevision: 1, Replacement: &replacement,
		}
		corrected, err := repo.CorrectDirectObservation(
			ctx, user.owner, initial.ObservationID, user.commands.correct,
		)
		if err != nil || corrected == nil {
			t.Fatalf("tenant %d correction: %#v %v", i, corrected, err)
		}
		user.receipts[1] = corrected

		user.commands.withdraw = model.AmendBodyObservationCommand{
			IdempotencyKey: "shared-withdraw", ExpectedRevision: 2,
		}
		withdrawn, err := repo.WithdrawDirectObservation(
			ctx, user.owner, corrected.ObservationID, user.commands.withdraw,
		)
		if err != nil || withdrawn == nil {
			t.Fatalf("tenant %d withdrawal: %#v %v", i, withdrawn, err)
		}
		user.receipts[2] = withdrawn
	}

	operations := [3]string{"record", "correct", "withdraw"}
	for i := range users {
		user := &users[i]
		for step, original := range user.receipts {
			if original.UserID != user.owner || original.Operation != operations[step] ||
				original.Revision != int64(step+1) ||
				original.ID == uuid.Nil || original.EventID == uuid.Nil ||
				original.RevisionID == uuid.Nil || original.SourceEvidenceID == uuid.Nil {
				t.Fatalf("tenant %d operation %s invalid receipt: %#v", i, operations[step], original)
			}
			var replay *model.BodyObservationCommit
			var err error
			switch step {
			case 0:
				replay, err = repo.RecordDirectObservation(ctx, user.owner, user.commands.record)
			case 1:
				replay, err = repo.CorrectDirectObservation(
					ctx, user.owner, user.receipts[0].ObservationID, user.commands.correct,
				)
			case 2:
				replay, err = repo.WithdrawDirectObservation(
					ctx, user.owner, user.receipts[1].ObservationID, user.commands.withdraw,
				)
			}
			if err != nil || replay == nil || replay.ID != original.ID ||
				replay.UserID != original.UserID || replay.Operation != original.Operation ||
				replay.Revision != original.Revision || replay.RevisionID != original.RevisionID ||
				replay.EventID != original.EventID || replay.ObservationID != original.ObservationID ||
				replay.SourceEvidenceID != original.SourceEvidenceID ||
				!replay.CommittedAt.Equal(original.CommittedAt) {
				t.Fatalf("tenant %d %s replay not exact: original=%#v replay=%#v err=%v",
					i, operations[step], original, replay, err)
			}
			foreign := users[1-i].receipts[step]
			if foreign.UserID == original.UserID || foreign.ID == original.ID ||
				foreign.EventID == original.EventID || foreign.RevisionID == original.RevisionID ||
				foreign.ObservationID == original.ObservationID ||
				foreign.SourceEvidenceID == original.SourceEvidenceID {
				t.Fatalf("tenant %d %s reused foreign identity: owner=%#v foreign=%#v",
					i, operations[step], original, foreign)
			}
		}
		page, err := repo.ListDirectObservations(ctx, user.owner, 0)
		if err != nil || page == nil || page.CurrentRevision != 3 || len(page.Observations) != 0 {
			t.Fatalf("tenant %d withdrawn report remained current: %#v %v", i, page, err)
		}
		for table, want := range map[string]int64{
			"body_observation_commits": 3,
			"body_state_revisions":     3,
			"body_state_outbox":        3,
			"body_observations":        2,
			"statement_evidence":       2,
		} {
			var got int64
			if err := db.Table(table).Where("user_id=?", user.owner).Count(&got).Error; err != nil || got != want {
				t.Fatalf("tenant %d %s count=%d want=%d err=%v", i, table, got, want, err)
			}
		}
	}
}
