package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestNativeDiagnosisRunReadRealDecisionPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(db))
	ctx := context.Background()
	initial, err := svc.Read(ctx, f.owner, f.run.ID)
	if err != nil || initial.State != "open" || len(initial.Proposals) != 2 ||
		initial.Proposals[0].State != "pending" || initial.Proposals[1].State != "pending" ||
		initial.CurrentDiagnosisRevision != 0 {
		t.Fatalf("initial proposal snapshot: %+v %v", initial, err)
	}
	firstIndex := -1
	for i, proposal := range initial.Proposals {
		if proposal.ProposalID == f.proposals[0].ID {
			firstIndex = i
		}
	}
	if firstIndex < 0 || initial.Proposals[firstIndex].ContentFingerprint != f.proposals[0].ContentFingerprint {
		t.Fatalf("exact proposal fingerprint not found in persisted ordering: %+v", initial.Proposals)
	}
	firstFingerprint := initial.Proposals[firstIndex].ContentFingerprint
	if _, err := NewDiagnosisProposalDecisionService(db).Record(ctx, f.owner, diagnosisDecisionCommand(f, 0)); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Read(ctx, f.owner, f.run.ID)
	if err != nil || len(after.Proposals) != 2 || after.Proposals[firstIndex].State != "decided" ||
		after.Proposals[firstIndex].Decision == nil || after.Proposals[firstIndex].Decision.Decision != model.DiagnosisAccept ||
		after.Proposals[firstIndex].ContentFingerprint != firstFingerprint ||
		after.Proposals[1-firstIndex].State != "pending" || after.CommittedDiagnosisRevision != nil ||
		after.Settlement != nil || len(after.Events) != 1 || after.Events[0].Kind != "decision_recorded" {
		t.Fatalf("choice must not be adoption: %+v %v", after, err)
	}
	var committed int64
	if err := db.Model(&model.DiagnosisRevision{}).Where("user_id=?", f.owner).Count(&committed).Error; err != nil || committed != 0 {
		t.Fatalf("read/decision unexpectedly created revision: %d %v", committed, err)
	}
	foreign := seedOnboardingContextUser(t, db)
	if _, err := svc.Read(ctx, foreign, f.run.ID); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatalf("foreign read must owner-mask 404: %v", err)
	}
	index, err := svc.List(ctx, f.owner, 20, nil)
	if err != nil || len(index.Runs) != 1 || index.Runs[0].RunID != f.run.ID || index.Runs[0].State != "open" {
		t.Fatalf("owner's run missing from index: %+v %v", index, err)
	}
	foreignIndex, err := svc.List(ctx, foreign, 20, nil)
	if err != nil || len(foreignIndex.Runs) != 0 {
		t.Fatalf("index crossed user: %+v %v", foreignIndex, err)
	}
}

func TestNativeDiagnosisRunTerminalSafetyHasNoAdoptedProjectionPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	f := seedDiagnosisDecisionFixture(t, db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	terminal := model.ConditionAnalysisEvent{
		UserID: f.owner, ID: uuid.New(), RunID: f.run.ID, Sequence: 1,
		Kind: "safety_blocked", BodyStateRevision: 1,
		Details:    datatypes.JSON(`{"reason":"active_urgent_safety_restriction"}`),
		RecordedAt: now,
	}
	if err := db.Create(&terminal).Error; err != nil {
		t.Fatal(err)
	}
	view, err := NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(db)).Read(context.Background(), f.owner, f.run.ID)
	if err != nil || view.State != "closed_without_change" || view.Outcome == nil || *view.Outcome != "safety_blocked" ||
		view.CommittedDiagnosisRevision != nil || view.Settlement != nil || len(view.Proposals) != 2 {
		t.Fatalf("terminal Safety projected adoption: %+v %v", view, err)
	}
	for _, p := range view.Proposals {
		if p.State != "expired" || p.Reason == nil || *p.Reason != "safety_blocked" {
			t.Fatalf("Safety cannot leave proposal choosable: %+v", p)
		}
	}
}
