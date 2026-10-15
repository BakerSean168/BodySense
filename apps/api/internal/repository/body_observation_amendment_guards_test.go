package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestBodyObservationAmendmentForeignNoopStaleAndBlockedSourcePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner, foreign := seedBodyStateContextUser(t, db), seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	cmd := observationDirectCommand(0, "first", "My synthetic original value was 67.5 kg")
	original, err := repo.RecordDirectObservation(ctx, owner, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.WithdrawDirectObservation(ctx, foreign, original.ObservationID,
		model.AmendBodyObservationCommand{IdempotencyKey: "foreign", ExpectedRevision: 0}); !errors.Is(err, model.ErrBodyObservationNotFound) {
		t.Fatalf("foreign user amended a private observation: %v", err)
	}
	same := cmd.Observation
	if _, err := repo.CorrectDirectObservation(ctx, owner, original.ObservationID,
		model.AmendBodyObservationCommand{IdempotencyKey: "noop", ExpectedRevision: 1, Replacement: &same}); !errors.Is(err, model.ErrInvalidBodyObservation) {
		t.Fatalf("no-op correction created new source: %v", err)
	}
	newValue := same
	newValue.Statement = "My synthetic corrected original value was 65.7 kg"
	newValue.RawValue = "65.7"
	if _, err := repo.CorrectDirectObservation(ctx, owner, original.ObservationID,
		model.AmendBodyObservationCommand{IdempotencyKey: "stale", ExpectedRevision: 0, Replacement: &newValue}); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatalf("stale report rewrote history: %v", err)
	}
	if err := db.Model(&model.StatementEvidence{}).
		Where("user_id=? AND id=?", owner, original.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CorrectDirectObservation(ctx, owner, original.ObservationID,
		model.AmendBodyObservationCommand{IdempotencyKey: "blocked-correct", ExpectedRevision: 1, Replacement: &newValue}); !errors.Is(err, model.ErrBodyObservationNotFound) {
		t.Fatalf("blocked source was corrected as if still available: %v", err)
	}
	withdraw := model.AmendBodyObservationCommand{IdempotencyKey: "blocked-withdraw", ExpectedRevision: 1}
	saved, err := repo.WithdrawDirectObservation(ctx, owner, original.ObservationID, withdraw)
	if err != nil || saved == nil || saved.Revision != 2 {
		t.Fatalf("user could not retract own blocked-source assertion: %#v %v", saved, err)
	}
	if _, err := repo.WithdrawDirectObservation(ctx, owner, uuid.New(), withdraw); !errors.Is(err, model.ErrBodyObservationIdempotencyConflict) {
		t.Fatalf("same key was rebound to a different target: %v", err)
	}
	if _, err := repo.CorrectDirectObservation(ctx, owner, original.ObservationID,
		model.AmendBodyObservationCommand{IdempotencyKey: "blocked-withdraw", ExpectedRevision: 2, Replacement: &newValue}); !errors.Is(err, model.ErrBodyObservationIdempotencyConflict) {
		t.Fatalf("same key replay switched from withdrawal to correction: %v", err)
	}
	if _, err := repo.WithdrawDirectObservation(ctx, owner, original.ObservationID,
		model.AmendBodyObservationCommand{IdempotencyKey: "withdraw-twice", ExpectedRevision: 2}); !errors.Is(err, model.ErrBodyObservationNotCurrent) {
		t.Fatalf("withdrawn observation altered twice: %v", err)
	}
}

func TestBodyObservationAmendmentReceiptFailureIsFullRollbackPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	cmd := observationDirectCommand(0, "original", "My synthetic measured value was 67.5 kilograms")
	original, err := repo.RecordDirectObservation(ctx, owner, cmd)
	if err != nil {
		t.Fatal(err)
	}
	replacement := cmd.Observation
	replacement.RawValue = "65.7"
	replacement.Statement = "My synthetic measured value was 65.7 kilograms"
	artificial := errors.New("synthetic final amendment receipt failure")
	const callback = "observation_amendment_receipt_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "body_observation_commits" {
			tx.AddError(artificial)
		}
	}); err != nil {
		t.Fatal(err)
	}
	result, err := repo.CorrectDirectObservation(ctx, owner, original.ObservationID,
		model.AmendBodyObservationCommand{IdempotencyKey: "failed", ExpectedRevision: 1, Replacement: &replacement})
	if result != nil || !errors.Is(err, artificial) {
		t.Fatalf("late failure falsely confirmed correction: %#v %v", result, err)
	}
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	var old model.BodyObservation
	if err := db.Where("user_id=? AND id=?", owner, original.ObservationID).Take(&old).Error; err != nil {
		t.Fatal(err)
	}
	if old.AssertionStatus != "asserted" {
		t.Fatalf("failed correction changed old version: %#v", old)
	}
	page, err := repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || page.CurrentRevision != 1 || len(page.Observations) != 1 {
		t.Fatalf("partial revision/evidence persisted after rollback: %#v %v", page, err)
	}
	var observations, sources, receipts int64
	for _, c := range []struct {
		table string
		out   *int64
	}{{"body_observations", &observations},
		{"statement_evidence", &sources}, {"body_observation_commits", &receipts}} {
		if err := db.Table(c.table).Where("user_id=?", owner).Count(c.out).Error; err != nil {
			t.Fatal(err)
		}
		if *c.out != 1 {
			t.Fatalf("%s partial failed amendment rows: %d", c.table, *c.out)
		}
	}
}

func TestBodyObservationWithdrawalReceiptFailureRollsBackAndRetriesPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	initial, err := repo.RecordDirectObservation(ctx, owner, observationDirectCommand(0, "withdraw-rollback-initial", "My synthetic direct report"))
	if err != nil {
		t.Fatal(err)
	}
	command := model.AmendBodyObservationCommand{IdempotencyKey: "withdraw-rollback-attempt", ExpectedRevision: 1}
	injected := errors.New("synthetic final withdrawal receipt failure")
	const callback = "body_observation_withdraw_receipt_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "body_observation_commits" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	receipt, err := repo.WithdrawDirectObservation(ctx, owner, initial.ObservationID, command)
	if receipt != nil || !errors.Is(err, injected) {
		t.Fatalf("failed durable withdrawal returned success: %#v %v", receipt, err)
	}
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	page, err := repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || page.CurrentRevision != 1 || len(page.Observations) != 1 || page.Observations[0].ID != initial.ObservationID {
		t.Fatalf("failed withdrawal left partial state: %#v %v", page, err)
	}
	var receipts int64
	if err := db.Table("body_observation_commits").Where("user_id=?", owner).Count(&receipts).Error; err != nil || receipts != 1 {
		t.Fatalf("failed withdrawal left a durable receipt: %d %v", receipts, err)
	}
	saved, err := repo.WithdrawDirectObservation(ctx, owner, initial.ObservationID, command)
	if err != nil || saved == nil || saved.Revision != 2 || saved.Operation != "withdraw" {
		t.Fatalf("same authorized command did not recover after rollback: %#v %v", saved, err)
	}
	replay, err := repo.WithdrawDirectObservation(ctx, owner, initial.ObservationID, command)
	if err != nil || replay == nil || replay.ID != saved.ID || replay.EventID != saved.EventID {
		t.Fatalf("successful withdrawal replay minted a new event: %#v %v", replay, err)
	}
	page, err = repo.ListDirectObservations(ctx, owner, 0)
	if err != nil || page.CurrentRevision != 2 || len(page.Observations) != 0 {
		t.Fatalf("withdrawal retry failed to change only current assertion: %#v %v", page, err)
	}
}

func TestBodyObservationCompetingExplicitAmendmentsAndGuardedDowngradePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	initial, err := repo.RecordDirectObservation(context.Background(), owner,
		observationDirectCommand(0, "initial", "My synthetic self report"))
	if err != nil {
		t.Fatal(err)
	}
	const workers = 5
	var wg sync.WaitGroup
	successes := make(chan *model.BodyObservationCommit, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			receipt, err := repo.WithdrawDirectObservation(context.Background(), owner, initial.ObservationID,
				model.AmendBodyObservationCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 1})
			if err != nil {
				errs <- err
			} else {
				successes <- receipt
			}
		}()
	}
	wg.Wait()
	close(successes)
	close(errs)
	count := 0
	for range successes {
		count++
	}
	if count != 1 {
		t.Fatalf("competing explicit withdrawals must choose exactly one: %d", count)
	}
	for err := range errs {
		if !errors.Is(err, ErrBodyStateRevisionConflict) {
			t.Fatalf("non-winning competing command must be stale: %v", err)
		}
	}
}
