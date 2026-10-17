package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeNativeRunRepo struct {
	snap                          *repository.DiagnosisRunSnapshot
	page                          *repository.DiagnosisRunPage
	err                           error
	readOwner, readRun, listOwner uuid.UUID
	limit                         int
	before                        *uuid.UUID
}

func (f *fakeNativeRunRepo) ReadRunSnapshot(_ context.Context, o, id uuid.UUID) (*repository.DiagnosisRunSnapshot, error) {
	f.readOwner = o
	f.readRun = id
	return f.snap, f.err
}
func (f *fakeNativeRunRepo) ListRunPage(_ context.Context, o uuid.UUID, limit int, before *uuid.UUID) (*repository.DiagnosisRunPage, error) {
	f.listOwner = o
	f.limit = limit
	f.before = before
	return f.page, f.err
}
func nativeSyntheticRun(owner uuid.UUID) model.ConditionAnalysisRun {
	return model.ConditionAnalysisRun{UserID: owner, ID: uuid.New(), RequestMessageID: uuid.New(), RequestEvidenceID: uuid.New(),
		RequestSourceVersion: 1, Trigger: model.ConditionNewAnalysis, Scope: model.ConditionAnalysisScope{
			ConditionIDs: []uuid.UUID{}, EvidenceRefs: []model.DiagnosisEvidenceRef{}, Description: "synthetic request"},
		InputEvidenceRefs: []model.DiagnosisEvidenceRef{}, AgentHypotheses: []model.ConditionAgentHypothesis{},
		StartedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
}
func nativeSyntheticProposal(owner, run uuid.UUID, hash string) model.DiagnosisProposal {
	return model.DiagnosisProposal{
		UserID: owner, ID: uuid.New(), RunID: run, ChangeKind: model.DiagnosisCreate, BaseRevision: 0,
		Content:            datatypes.JSON(`{"proposedEntry":{"medicalName":"Synthetic named syndrome"}}`),
		ContentFingerprint: strings.Repeat(hash, 64), EvidenceRefs: []model.DiagnosisEvidenceRef{},
		CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: time.Now().UTC(),
	}
}

func TestNativeDiagnosisRunStatesAreAuditedAndNonadopting(t *testing.T) {
	owner := uuid.New()
	run := nativeSyntheticRun(owner)
	svc := NewNativeDiagnosisRunReadService(&fakeNativeRunRepo{snap: &repository.DiagnosisRunSnapshot{Run: run}})
	view, err := svc.Read(context.Background(), owner, run.ID)
	if err != nil || view.State != "open" || view.Outcome != nil || view.Settlement != nil ||
		view.Proposals == nil || view.Events == nil || view.CurrentDiagnosisRevision != 0 {
		t.Fatalf("empty run must be open: %+v %v", view, err)
	}

	old := nativeSyntheticProposal(owner, run.ID, "a")
	newer := nativeSyntheticProposal(owner, run.ID, "b")
	newer.SupersedesProposalID = &old.ID
	decisionID := uuid.New()
	decidedAt := time.Now().UTC()
	decided := model.DiagnosisDecision{UserID: owner, ID: decisionID, ProposalID: old.ID, RunID: run.ID,
		ContentFingerprint: old.ContentFingerprint, Decision: model.DiagnosisAccept,
		ExpectedRevision: 0, ReceiptID: uuid.New(), DecidedAt: decidedAt}
	f := &fakeNativeRunRepo{snap: &repository.DiagnosisRunSnapshot{
		Run: run, Proposals: []model.DiagnosisProposal{old, newer}, Decisions: []model.DiagnosisDecision{decided},
		Events: []model.ConditionAnalysisEvent{
			{UserID: owner, RunID: run.ID, Sequence: 1, Kind: "decision_recorded", ProposalID: &old.ID, DecisionID: &decisionID},
			{UserID: owner, RunID: run.ID, Sequence: 2, Kind: "proposal_superseded", ProposalID: &old.ID, Details: datatypes.JSON(`{"reason":"evidence_changed"}`)},
		},
	}}
	svc = NewNativeDiagnosisRunReadService(f)
	view, err = svc.Read(context.Background(), owner, run.ID)
	if err != nil || view.State != "open" || len(view.Proposals) != 2 || len(view.Events) != 2 {
		t.Fatalf("snapshot projection: %+v %v", view, err)
	}
	if view.Proposals[0].State != "superseded" || view.Proposals[0].Decision == nil ||
		view.Proposals[0].Decision.DecisionID != decisionID ||
		view.Proposals[0].SupersededByProposalID == nil ||
		*view.Proposals[0].SupersededByProposalID != newer.ID ||
		view.Proposals[0].Reason == nil || *view.Proposals[0].Reason != "evidence_changed" ||
		view.Proposals[1].State != "pending" || view.Proposals[1].Decision != nil {
		t.Fatalf("decision cannot supersede newer pending proposal: %+v", view.Proposals)
	}
	var content map[string]any
	if err := json.Unmarshal(view.Proposals[0].Content, &content); err != nil || content["proposedEntry"] == nil {
		t.Fatalf("frozen content changed: %+v %v", view.Proposals[0].Content, err)
	}
	if f.readOwner != owner || f.readRun != run.ID {
		t.Fatalf("wrong owner passed to repository: %+v", f)
	}
	f.snap.Events[1].Sequence = 4
	if _, err := svc.Read(context.Background(), owner, run.ID); err == nil {
		t.Fatal("accepted an audit with skipped sequence")
	}
	f.snap.Events[1].Sequence = 2
	f.snap.Events = f.snap.Events[1:]
	f.snap.Events[0].Sequence = 1
	if _, err := svc.Read(context.Background(), owner, run.ID); err == nil {
		t.Fatal("receipt without decision_recorded audit accepted")
	}
	f.snap.Proposals[0].ContentFingerprint = "invalid"
	if _, err := svc.Read(context.Background(), owner, run.ID); err == nil {
		t.Fatal("accepted a corrupt proposal fingerprint")
	}
}

func TestNativeDiagnosisRunTerminalWithoutAdoptionAndReceipt(t *testing.T) {
	owner := uuid.New()
	run := nativeSyntheticRun(owner)
	pending := nativeSyntheticProposal(owner, run.ID, "a")
	fake := &fakeNativeRunRepo{snap: &repository.DiagnosisRunSnapshot{Run: run, Proposals: []model.DiagnosisProposal{pending},
		Events: []model.ConditionAnalysisEvent{{UserID: owner, RunID: run.ID, Sequence: 1, Kind: "safety_blocked"}}}}
	view, err := NewNativeDiagnosisRunReadService(fake).Read(context.Background(), owner, run.ID)
	if err != nil || view.State != "closed_without_change" || view.Outcome == nil || *view.Outcome != "safety_blocked" ||
		view.CommittedDiagnosisRevision != nil || view.Proposals[0].State != "expired" ||
		view.Proposals[0].Reason == nil || *view.Proposals[0].Reason != "safety_blocked" {
		t.Fatalf("urgent termination allowed phantom adoption: %+v %v", view, err)
	}
	fake.snap.Events[0].Kind = "committed"
	if _, err := NewNativeDiagnosisRunReadService(fake).Read(context.Background(), owner, run.ID); err == nil {
		t.Fatal("committed event without receipt must fail closed")
	}
	id := uuid.New()
	receipt := &model.DiagnosisSettlement{UserID: owner, ID: uuid.New(), RunID: run.ID, ReceiptID: uuid.New(),
		Outcome: "committed", ExpectedRevision: 0, Revision: 1, DiagnosisRevisionID: &id, SettledAt: time.Now().UTC()}
	fake.snap.Settlement = receipt
	fake.snap.CurrentDiagnosisRevision = 1
	fake.snap.Proposals = nil
	view, err = NewNativeDiagnosisRunReadService(fake).Read(context.Background(), owner, run.ID)
	if err != nil || view.State != "committed" || view.CommittedDiagnosisRevision == nil || *view.CommittedDiagnosisRevision != 1 {
		t.Fatalf("valid commitment not projected: %+v %v", view, err)
	}
	receipt.Outcome = "safety_blocked"
	if _, err := NewNativeDiagnosisRunReadService(fake).Read(context.Background(), owner, run.ID); err == nil {
		t.Fatal("unmatched terminal receipt accepted")
	}
}

func TestNativeDiagnosisRunPageStatusAndOwnerBoundary(t *testing.T) {
	owner := uuid.New()
	run := nativeSyntheticRun(owner)
	newer := nativeSyntheticRun(owner)
	newer.StartedAt = run.StartedAt.Add(time.Second)
	f := &fakeNativeRunRepo{page: &repository.DiagnosisRunPage{Runs: []model.ConditionAnalysisRun{newer, run},
		TerminalEvents: []model.ConditionAnalysisEvent{{UserID: owner, RunID: run.ID, Kind: "insufficient_evidence"}},
	}}
	svc := NewNativeDiagnosisRunReadService(f)
	result, err := svc.List(context.Background(), owner, 20, nil)
	if err != nil || len(result.Runs) != 2 || result.Runs[0].State != "open" ||
		result.Runs[1].State != "closed_without_change" || result.Runs[1].Outcome == nil ||
		*result.Runs[1].Outcome != "insufficient_evidence" || f.listOwner != owner || f.limit != 20 {
		t.Fatalf("index projection inaccurate: %+v err=%v", result, err)
	}
	f.page.TerminalEvents[0].UserID = uuid.New()
	if _, err := svc.List(context.Background(), owner, 20, nil); err == nil {
		t.Fatal("foreign terminal event leaked")
	}
	f.err = model.ErrDiagnosisNotFound
	if _, err := svc.List(context.Background(), owner, 20, nil); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatal(err)
	}
}
