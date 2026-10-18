package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Uses only a throwaway PostgreSQL 18 container whose Unix socket is exposed
// in a temporary local /tmp/bs-tpd1-owner-* directory. Never set this env
// variable to a shared, staging, or production database connection.
func TestTrainingOwnerLockConcurrentNativePG18(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_TP_D1_SYNTHETIC_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires a locally isolated TP-D1 synthetic PostgreSQL 18 Unix socket")
	}
	if !strings.HasPrefix(dsn, "host=/tmp/bs-tpd1-owner-") {
		t.Fatal("refusing a database URL outside the dedicated local synthetic Unix socket")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to synthetic PG18: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(6)
	var version int
	if err := db.Raw("SELECT current_setting('server_version_num')::int").Scan(&version).Error; err != nil || version < 180000 {
		t.Fatalf("PG18 synthetic fixture required: version=%d err=%v", version, err)
	}
	var unixSocket bool
	if err := db.Raw("SELECT inet_server_addr() IS NULL").Scan(&unixSocket).Error; err != nil || !unixSocket {
		t.Fatalf("refusing TCP/shared Postgres: local Unix socket=%v err=%v", unixSocket, err)
	}
	var oldUniqueIndex, revisionIndex bool
	if err := db.Raw("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='public' AND tablename='training_plans' AND indexname='idx_training_plans_user_active')").
		Scan(&oldUniqueIndex).Error; err != nil || oldUniqueIndex {
		t.Fatalf("synthetic forward preview must remove old active index: index=%v err=%v", oldUniqueIndex, err)
	}
	if err := db.Raw("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='public' AND tablename='training_plans' AND indexname='idx_training_plans_treatment_revision')").
		Scan(&revisionIndex).Error; err != nil || revisionIndex {
		t.Fatalf("the TreatmentRevision index must be gone with Treatment: index=%v err=%v", revisionIndex, err)
	}
	owner, other := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, other} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES (?,?,?)",
			id, "synthetic-training-lock-"+id.String()+"@example.invalid", "synthetic-only").Error; err != nil {
			t.Fatalf("create ephemeral owner: %v", err)
		}
	}
	defer func() {
		_ = db.Exec("DELETE FROM training_plans WHERE user_id IN (?,?)", owner, other).Error
		_ = db.Exec("DELETE FROM users WHERE id IN (?,?)", owner, other).Error
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	repo := NewTrainingRepository(db)
	manager := database.NewTransactionManager(db)
	firstAcquired := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstResult := make(chan error, 1)
	secondStarted := make(chan struct{})
	secondResult := make(chan error, 1)
	defer func() {
		select {
		case <-releaseFirst:
		default:
			close(releaseFirst)
		}
	}()
	create := func(txCtx context.Context, user, id uuid.UUID, goal string) error {
		return repo.CreatePlan(txCtx, &model.TrainingPlan{
			ID: id, UserID: user, Status: "active", Goal: goal,
			DurationWeeks: 4, CurrentWeek: 1, Phases: json.RawMessage("[]"),
		})
	}
	firstID, secondID, foreignID, childID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	go func() {
		firstResult <- manager.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := repo.LockOwnerForPlanActivation(txCtx, owner); err != nil {
				return err
			}
			if err := create(txCtx, owner, firstID, "synthetic A"); err != nil {
				return err
			}
			close(firstAcquired)
			select {
			case <-releaseFirst:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	select {
	case <-firstAcquired:
	case e := <-firstResult:
		t.Fatalf("first lock/plan insertion failed: %v", e)
	case <-ctx.Done():
		t.Fatal("timeout acquiring first synthetic owner lock")
	}
	// While owner A holds the activation lock, the FOR KEY SHARE that every
	// same-owner child foreign-key insert takes on users must not wait.
	keyShareResult := make(chan error, 1)
	go func() {
		keyShareResult <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL lock_timeout = '2s'").Error; err != nil {
				return err
			}
			var n int64
			return tx.Raw("SELECT count(*) FROM (SELECT 1 FROM users WHERE id = ? FOR KEY SHARE) s", owner).
				Scan(&n).Error
		})
	}()
	select {
	case err := <-keyShareResult:
		if err != nil {
			t.Fatalf("owner lock blocked a same-owner FOR KEY SHARE: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("owner lock stalled a same-owner foreign-key check")
	}
	childResult := make(chan error, 1)
	go func() {
		childResult <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL lock_timeout = '2s'").Error; err != nil {
				return err
			}
			return tx.Exec("INSERT INTO training_plans(id,user_id,status,goal) VALUES (?,?,?,?)",
				childID, owner, "completed", "synthetic unlocked child insert").Error
		})
	}()
	select {
	case err := <-childResult:
		if err != nil {
			t.Fatalf("owner lock blocked a same-owner child FK insert: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("owner lock stalled a same-owner child FK insert")
	}
	go func() {
		secondResult <- manager.WithinTransaction(ctx, func(txCtx context.Context) error {
			close(secondStarted)
			if err := repo.LockOwnerForPlanActivation(txCtx, owner); err != nil {
				return err
			}
			return create(txCtx, owner, secondID, "synthetic B")
		})
	}()
	select {
	case <-secondStarted:
	case <-ctx.Done():
		t.Fatal("second concurrent activation never started")
	}

	// Another user's row is independent: it must not wait for owner A.
	otherResult := make(chan error, 1)
	go func() {
		otherResult <- manager.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := repo.LockOwnerForPlanActivation(txCtx, other); err != nil {
				return err
			}
			return create(txCtx, other, foreignID, "synthetic independent owner")
		})
	}()
	select {
	case err := <-otherResult:
		if err != nil {
			t.Fatalf("unrelated owner unexpectedly blocked/failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("per-owner locking stalled an unrelated user")
	}
	select {
	case err := <-secondResult:
		t.Fatalf("second same-owner transaction ran before release: %v", err)
	case <-time.After(180 * time.Millisecond):
		// Expected: owner B's accepted-plan transaction is still waiting.
	case <-ctx.Done():
		t.Fatal("second owner-lock wait exceeded synthetic test timeout")
	}
	close(releaseFirst)
	for i, result := range []<-chan error{firstResult, secondResult} {
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("transaction %d did not commit safely: %v", i+1, err)
			}
		case <-ctx.Done():
			t.Fatalf("transaction %d deadlocked after releasing the owner lock", i+1)
		}
	}
	var ownActive, foreignActive int64
	if err := db.Model(&model.TrainingPlan{}).Where("user_id = ? AND status = 'active'", owner).
		Count(&ownActive).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.TrainingPlan{}).Where("user_id = ? AND status = 'active'", other).
		Count(&foreignActive).Error; err != nil {
		t.Fatal(err)
	}
	if ownActive != 2 || foreignActive != 1 {
		t.Fatalf("serialized synthetic plans were altered: owner=%d other=%d", ownActive, foreignActive)
	}
	if selected, err := repo.GetActivePlanByUserID(ctx, owner); selected != nil || !errors.Is(err, ErrMultipleActiveTrainingPlans) {
		t.Fatalf("legacy single-plan view failed open: selected=%#v err=%v", selected, err)
	}
	// A failing transaction cannot commit a third plan or change the two
	// committed plan statuses. The owner lock must be released on rollback.
	rollbackOnly := fmt.Errorf("synthetic plan rollback")
	thirdID := uuid.New()
	err = manager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if e := repo.LockOwnerForPlanActivation(txCtx, owner); e != nil {
			return e
		}
		if e := create(txCtx, owner, thirdID, "must roll back"); e != nil {
			return e
		}
		return rollbackOnly
	})
	if !errors.Is(err, rollbackOnly) {
		t.Fatalf("expected synthetic forced rollback, got %v", err)
	}
	var afterRollback int64
	if err := db.Model(&model.TrainingPlan{}).Where("user_id = ?", owner).
		Count(&afterRollback).Error; err != nil || afterRollback != 3 {
		t.Fatalf("owner transaction leaked uncommitted plan: %d %v", afterRollback, err)
	}
	t.Log("TP_D1_NATIVE_PG18_PER_OWNER_SERIALIZATION=PASS")
	t.Log("TP_D1_NATIVE_PG18_CROSS_OWNER_INDEPENDENCE=PASS")
	t.Log("TP_D1_NATIVE_PG18_TWO_ACTIVE_AND_LEGACY_FAIL_CLOSED=PASS")
	t.Log("TP_D1_NATIVE_PG18_ROLLBACK_INTEGRITY=PASS")
	t.Log("TP_D1_NATIVE_PG18_FK_CHILD_INSERT_NOT_BLOCKED=PASS")
}
