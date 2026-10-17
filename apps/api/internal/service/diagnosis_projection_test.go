package service

import (
	"context"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

type nativeDiagnosisRepoStub struct {
	snap *repository.DiagnosisReadSnapshot
	err  error
}

func (s nativeDiagnosisRepoStub) ReadSnapshot(_ context.Context, _ uuid.UUID) (*repository.DiagnosisReadSnapshot, error) {
	return s.snap, s.err
}

func TestNativeDiagnosisReadModelOnlyCommitted(t *testing.T) {
	owner := uuid.New()
	empty, err := NewNativeDiagnosisReadService(nativeDiagnosisRepoStub{snap: &repository.DiagnosisReadSnapshot{Evidence: map[uuid.UUID][]model.ConditionEntryEvidence{}}}).Read(context.Background(), owner)
	if err != nil || empty.CurrentRevision != 0 || empty.UserID != owner || empty.ConditionEntries == nil || len(empty.ConditionEntries) != 0 || empty.UpdatedAt != nil {
		t.Fatalf("empty projection: %+v %v", empty, err)
	}
	now := time.Now().UTC()
	id := uuid.New()
	decision := uuid.New()
	analysis := uuid.New()
	evidence := uuid.New()
	entry := model.ConditionEntry{UserID: owner, ID: id, MedicalName: "Synthetic named syndrome", Disposition: model.ConditionAdopted,
		LatestDecisionID: decision, CommittedRevision: 2, Assessment: &model.ConditionAssessment{
			Level: model.DiagnosisSuspected, BasedOnRunID: analysis,
		}, CreatedAt: now, UpdatedAt: now}
	head := &model.Diagnosis{UserID: owner, CurrentRevision: 2, UpdatedAt: now}
	ref := model.ConditionEntryEvidence{UserID: owner, ConditionID: id, EvidenceType: model.DiagnosisBodyFact, EvidenceID: evidence,
		EvidenceVersion: 7, Role: model.DiagnosisSupporting, Rationale: "source-qualified", AnalyzedByRunID: analysis}
	svc := NewNativeDiagnosisReadService(nativeDiagnosisRepoStub{snap: &repository.DiagnosisReadSnapshot{
		Head: head, Conditions: []model.ConditionEntry{entry}, Evidence: map[uuid.UUID][]model.ConditionEntryEvidence{id: {ref}},
	}})
	view, err := svc.Read(context.Background(), owner)
	if err != nil || view.CurrentRevision != 2 || len(view.ConditionEntries) != 1 || view.UpdatedAt == nil {
		t.Fatalf("view: %+v %v", view, err)
	}
	condition := view.ConditionEntries[0]
	if condition.ConditionID != id || condition.LatestDecisionID != decision || condition.Assessment == nil || condition.Assessment.Level != model.DiagnosisSuspected {
		t.Fatalf("condition mismatch: %+v", condition)
	}
	if len(condition.EvidenceLinks) != 1 || condition.EvidenceLinks[0].EvidenceVersion != "7" ||
		condition.EvidenceLinks[0].EvidenceType != model.DiagnosisBodyFact {
		t.Fatalf("evidence: %+v", condition.EvidenceLinks)
	}
	if condition.SourceClaimRefs == nil || condition.InformationGaps == nil || condition.HandlingPlan.Actions == nil ||
		condition.Assessment.InformationGaps == nil {
		t.Fatalf("nil arrays cannot be emitted as null: %+v", condition)
	}
}
