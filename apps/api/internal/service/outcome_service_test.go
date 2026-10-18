package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeOutcomeRepo struct {
	stored                *model.Outcome
	storedCreated         bool
	updateOutcomeCalls    int
	updateOutcomeRevision int64
}

func (r *fakeOutcomeRepo) CreateOutcome(_ context.Context, outcome *model.Outcome) (*model.Outcome, bool, error) {
	if r.stored != nil {
		return r.stored, r.storedCreated, nil
	}
	r.stored, r.storedCreated = outcome, true
	return outcome, true, nil
}

func (r *fakeOutcomeRepo) UpdateOutcomeBodyStateRevision(_ context.Context, _, _ uuid.UUID, revision int64) error {
	r.updateOutcomeCalls++
	r.updateOutcomeRevision = revision
	if r.stored != nil {
		r.stored.BodyStateRevision = &revision
	}
	return nil
}

func (r *fakeOutcomeRepo) ListOutcomes(context.Context, uuid.UUID, int) ([]model.Outcome, error) {
	return nil, nil
}

type fakeOutcomeBodyState struct {
	calls    int
	revision int64
}

func (f *fakeOutcomeBodyState) RecordOutcomeSymptom(_ context.Context, _ uuid.UUID, outcome model.Outcome) (*model.BodyStateRevision, error) {
	f.calls++
	if outcome.Kind != "symptom_change" && outcome.Kind != "new_discomfort" {
		return nil, nil
	}
	revision := f.revision
	if revision == 0 {
		revision = 12
	}
	return &model.BodyStateRevision{Revision: revision}, nil
}

func TestRecordOutcomeUsesOneUnitOfWorkForOutcomeAndBodyState(t *testing.T) {
	called := false
	repo := &fakeOutcomeRepo{}
	bodyState := &fakeOutcomeBodyState{revision: 21}
	svc := NewOutcomeService(repo, bodyState, testTreatmentUnitOfWork{called: &called})

	stored, created, err := svc.RecordOutcome(context.Background(), uuid.New(), model.Outcome{
		SourceType: "web_checkin", SourceKey: "outcome-1", Kind: "symptom_change",
		Value: datatypes.JSON(`{"description":"better","trend":"improving"}`),
	})
	if err != nil {
		t.Fatalf("RecordOutcome returned error: %v", err)
	}
	if !called || !created {
		t.Fatalf("expected transactional creation, called=%v created=%v", called, created)
	}
	if stored.BodyStateRevision == nil || *stored.BodyStateRevision != 21 {
		t.Fatalf("expected linked BodyState revision 21, got %#v", stored.BodyStateRevision)
	}
	if bodyState.calls != 1 || repo.updateOutcomeCalls != 1 {
		t.Fatalf("expected one projection and one link update: body=%d update=%d", bodyState.calls, repo.updateOutcomeCalls)
	}
}

func TestRecordOutcomeRepairsDuplicateWithoutBodyStateLink(t *testing.T) {
	userID := uuid.New()
	stored := &model.Outcome{
		ID: uuid.New(), UserID: userID,
		SourceType: "training_feedback", SourceKey: "log-1", Kind: "symptom_change",
		Value: datatypes.JSON(`{"description":"better","trend":"improving"}`),
	}
	repo := &fakeOutcomeRepo{stored: stored}
	bodyState := &fakeOutcomeBodyState{revision: 22}
	svc := NewOutcomeService(repo, bodyState, testTreatmentUnitOfWork{})

	result, created, err := svc.RecordOutcome(context.Background(), userID, *stored)
	if err != nil {
		t.Fatalf("RecordOutcome returned error: %v", err)
	}
	if created {
		t.Fatal("duplicate outcome must remain created=false")
	}
	if bodyState.calls != 1 || repo.updateOutcomeRevision != 22 {
		t.Fatalf("duplicate without link must be projected and linked: calls=%d revision=%d", bodyState.calls, repo.updateOutcomeRevision)
	}
	if result.BodyStateRevision == nil || *result.BodyStateRevision != 22 {
		t.Fatalf("expected repaired BodyState link, got %#v", result.BodyStateRevision)
	}
}

func TestRecordOutcomeWithoutUserSymptomLeavesBodyStateUnlinked(t *testing.T) {
	repo := &fakeOutcomeRepo{}
	svc := NewOutcomeService(repo, &fakeOutcomeBodyState{}, testTreatmentUnitOfWork{})
	stored, created, err := svc.RecordOutcome(context.Background(), uuid.New(), model.Outcome{
		SourceType: "training_checkin", SourceKey: "log-2", Kind: "training_adherence",
		Value: datatypes.JSON(`{"checked_in":true}`),
	})
	if err != nil || !created {
		t.Fatalf("adherence outcome must persist: created=%v err=%v", created, err)
	}
	if stored.BodyStateRevision != nil || repo.updateOutcomeCalls != 0 {
		t.Fatalf("non-symptom outcome must not link a BodyState revision: %#v updates=%d", stored.BodyStateRevision, repo.updateOutcomeCalls)
	}
}

func TestRecordOutcomeSkipsAlreadyAppliedDuplicate(t *testing.T) {
	userID := uuid.New()
	revision := int64(12)
	stored := &model.Outcome{
		ID: uuid.New(), UserID: userID,
		SourceType: "training_feedback", SourceKey: "log-1", Kind: "training_adherence",
		Value: datatypes.JSON(`{"checked_in":true}`), BodyStateRevision: &revision,
	}
	repo := &fakeOutcomeRepo{stored: stored}
	bodyState := &fakeOutcomeBodyState{}
	svc := NewOutcomeService(repo, bodyState, testTreatmentUnitOfWork{})

	_, created, err := svc.RecordOutcome(context.Background(), userID, *stored)
	if err != nil || created {
		t.Fatalf("applied duplicate should be an idempotent read: created=%v err=%v", created, err)
	}
	if bodyState.calls != 0 || repo.updateOutcomeCalls != 0 {
		t.Fatalf("already-linked duplicate must not reproject: body=%d update=%d", bodyState.calls, repo.updateOutcomeCalls)
	}
}

func TestRecordOutcomeRequiresSourceIdentity(t *testing.T) {
	svc := NewOutcomeService(&fakeOutcomeRepo{}, &fakeOutcomeBodyState{}, testTreatmentUnitOfWork{})
	if _, _, err := svc.RecordOutcome(context.Background(), uuid.New(), model.Outcome{Kind: "symptom_change"}); err == nil {
		t.Fatal("an Outcome without source identity must be rejected")
	}
}
