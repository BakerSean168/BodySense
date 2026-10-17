package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestNativeDiagnosisRunSnapshotAndOwnerIsolationPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := seedDiagnosisFixture(t, db)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()
	snap, err := repo.ReadRunSnapshot(ctx, f.owner, f.run.ID)
	if err != nil || snap == nil || snap.Run.ID != f.run.ID || snap.CurrentDiagnosisRevision != 1 ||
		len(snap.Proposals) != 1 || snap.Proposals[0].ContentFingerprint != f.proposal.ContentFingerprint ||
		len(snap.Decisions) != 1 || snap.Decisions[0].ReceiptID != f.decision.ReceiptID || snap.Settlement != nil {
		t.Fatalf("raw run snapshot mismatch: %#v err=%v", snap, err)
	}
	if snap.Run.UserID != f.owner || snap.Proposals[0].UserID != f.owner || snap.Decisions[0].UserID != f.owner {
		t.Fatal("read crossed owner boundary")
	}
	foreign := seedBodyStateContextUser(t, db)
	for _, id := range []uuid.UUID{f.run.ID, uuid.New()} {
		if _, err := repo.ReadRunSnapshot(ctx, foreign, id); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatalf("foreign/missing run must mask 404: %v", err)
		}
	}
	none, err := repo.ListRunPage(ctx, foreign, 20, nil)
	if err != nil || len(none.Runs) != 0 || none.HasMore {
		t.Fatalf("foreign index leak: %#v %v", none, err)
	}
	if _, err := repo.ReadRunSnapshot(ctx, uuid.Nil, f.run.ID); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatal(err)
	}
}

func TestNativeDiagnosisRunKeysetPaginationPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := seedDiagnosisFixture(t, db)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()
	another := f.run
	another.ID = uuid.New()
	another.IdempotencyKey = uuid.NewString()
	another.RequestFingerprint = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	another.StartedAt = f.run.StartedAt.Add(time.Second)
	if err := db.Create(&another).Error; err != nil {
		t.Fatal(err)
	}
	page, err := repo.ListRunPage(ctx, f.owner, 1, nil)
	if err != nil || len(page.Runs) != 1 || page.Runs[0].ID != another.ID || !page.HasMore ||
		page.NextBeforeRunID == nil || *page.NextBeforeRunID != another.ID {
		t.Fatalf("first page: %#v %v", page, err)
	}
	older, err := repo.ListRunPage(ctx, f.owner, 1, page.NextBeforeRunID)
	if err != nil || len(older.Runs) != 1 || older.Runs[0].ID != f.run.ID || older.HasMore || older.NextBeforeRunID != nil {
		t.Fatalf("older page: %#v %v", older, err)
	}
	foreign := seedBodyStateContextUser(t, db)
	if _, err := repo.ListRunPage(ctx, foreign, 1, page.NextBeforeRunID); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatalf("foreign cursor masked 404: %v", err)
	}
	if _, err := repo.ListRunPage(ctx, f.owner, 0, nil); err == nil {
		t.Fatal("accepted invalid zero limit")
	}
	if _, err := repo.ListRunPage(ctx, f.owner, 51, nil); err == nil {
		t.Fatal("accepted excessive limit")
	}
}
