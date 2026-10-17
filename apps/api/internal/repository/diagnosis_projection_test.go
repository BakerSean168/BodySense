package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// The repository reads *only* accepted Diagnosis, never an unfinished Agent
// proposal, a user decision pending settlement, or another owner's records.
func TestNativeDiagnosisSnapshotPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()
	untouched := seedBodyStateContextUser(t, db)
	empty, err := repo.ReadSnapshot(ctx, untouched)
	if err != nil || empty == nil || empty.Head != nil || len(empty.Conditions) != 0 || len(empty.Evidence) != 0 {
		t.Fatalf("missing diagnosis must be a zero view: %#v %v", empty, err)
	}
	assertBodyFactCount(t, db, "diagnoses", untouched, 0) // GET never writes a row

	f := seedDiagnosisFixture(t, db)
	snapshot, err := repo.ReadSnapshot(ctx, f.owner)
	if err != nil || snapshot == nil || snapshot.Head == nil || snapshot.Head.CurrentRevision != 1 || len(snapshot.Conditions) != 1 {
		t.Fatalf("committed diagnosis snapshot: %#v %v", snapshot, err)
	}
	if snapshot.Conditions[0].ID != f.entry.ID || snapshot.Conditions[0].Assessment == nil {
		t.Fatalf("committed condition not visible: %#v", snapshot.Conditions)
	}

	// Links are joined by owner+condition_id. Their typed source version and
	// attribution must never be overwritten by revision or replay metadata.
	link := model.ConditionEntryEvidence{UserID: f.owner, ConditionID: f.entry.ID,
		EvidenceType: model.DiagnosisBodyFact, EvidenceID: f.fact.ID, EvidenceVersion: f.fact.CreatedRevision,
		Role: model.DiagnosisSupporting, Rationale: "Synthetic explicit source", AnalyzedByRunID: f.run.ID}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err = repo.ReadSnapshot(ctx, f.owner)
	if err != nil || len(snapshot.Evidence[f.entry.ID]) != 1 || snapshot.Evidence[f.entry.ID][0].EvidenceVersion != f.fact.CreatedRevision {
		t.Fatalf("typed evidence links: %#v %v", snapshot, err)
	}

	other := seedDiagnosisFixture(t, db)
	own, err := repo.ReadSnapshot(ctx, other.owner)
	if err != nil || len(own.Conditions) != 1 || own.Conditions[0].ID == f.entry.ID || len(own.Evidence[f.entry.ID]) != 0 {
		t.Fatalf("cross-tenant diagnosis leak: %#v %v", own, err)
	}
	blank, err := repo.ReadSnapshot(ctx, untouched)
	if err != nil || blank.Head != nil || len(blank.Conditions) != 0 {
		t.Fatalf("owner without diagnosis inherited foreign records: %#v %v", blank, err)
	}
	// The committed view is immutable history. A later source withdrawal may
	// require review, but cannot quietly remove or rewrite an adopted entry.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.fact.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	historical, err := repo.ReadSnapshot(ctx, f.owner)
	if err != nil || historical.Head == nil || historical.Head.CurrentRevision != 1 || len(historical.Conditions) != 1 {
		t.Fatalf("source withdrawal rewrote committed diagnosis: %+v %v", historical, err)
	}
	if _, err := repo.ReadSnapshot(ctx, uuid.Nil); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatalf("nil owner must be rejected: %v", err)
	}
}

func TestNativeDiagnosisOnlyCommittedRecordsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewDiagnosisRepository(db)
	if _, err := repo.EnsureAggregate(context.Background(), owner); err != nil {
		t.Fatal(err)
	}
	// The initialized zero revision is still an empty read, even when the
	// application has no current ConditionEntry.
	snap, err := repo.ReadSnapshot(context.Background(), owner)
	if err != nil || snap.Head == nil || snap.Head.CurrentRevision != 0 || len(snap.Conditions) != 0 {
		t.Fatalf("zero revision: %#v %v", snap, err)
	}
}
