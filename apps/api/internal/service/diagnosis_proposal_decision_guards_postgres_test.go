package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestDiagnosisProposalDecisionEventFailureRollsBackPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	f := seedDiagnosisDecisionFixture(t, db)
	sentinel := errors.New("injected audit-event write failure")
	callback := "test:diagnosis-event-failure:" + uuid.NewString()
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if event, ok := tx.Statement.Dest.(*model.ConditionAnalysisEvent); ok && event.UserID == f.owner {
			tx.AddError(sentinel)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	r, err := NewDiagnosisProposalDecisionService(db).Record(context.Background(), f.owner, diagnosisDecisionCommand(f, 0))
	if !errors.Is(err, sentinel) || r != nil {
		t.Fatalf("event failure did not abort receipt: %+v %v", r, err)
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 0)
	diagnosisDecisionCount(t, db, f.owner, "condition_analysis_events", 0)
	assertDiagnosisNotAdopted(t, db, f.owner)
}

func TestDiagnosisProposalDecisionAuditGuardsPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalDecisionService(db)
	ctx := context.Background()
	if _, err := svc.Record(ctx, f.owner, diagnosisDecisionCommand(f, 0)); err != nil {
		t.Fatal(err)
	}
	var event model.ConditionAnalysisEvent
	if err := db.Where("user_id=? AND run_id=?", f.owner, f.run.ID).Take(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Sequence != 1 || event.Kind != "decision_recorded" {
		t.Fatalf("event not exact: %+v", event)
	}
	if err := db.Model(&model.ConditionAnalysisEvent{}).Where("user_id=? AND id=?", f.owner, event.ID).Update("kind", "run_started").Error; err == nil {
		t.Fatal("immutable event changed")
	}
	bad := model.ConditionAnalysisEvent{UserID: f.owner, ID: uuid.New(), RunID: f.run.ID, Sequence: 3, Kind: "evidence_added", BodyStateRevision: 1, Details: datatypes.JSON(`{}`), RecordedAt: time.Now().UTC()}
	if err := db.Create(&bad).Error; err == nil {
		t.Fatal("noncontiguous event accepted")
	}
	foreign := seedOnboardingContextUser(t, db)
	bad.UserID = foreign
	bad.ID = uuid.New()
	bad.Sequence = 1
	if err := db.Create(&bad).Error; err == nil {
		t.Fatal("foreign run event accepted")
	}
	terminal := model.ConditionAnalysisEvent{UserID: f.owner, ID: uuid.New(), RunID: f.run.ID, Sequence: 2, Kind: "closed_without_change", BodyStateRevision: 1, Details: datatypes.JSON(`{}`), RecordedAt: time.Now().UTC()}
	if err := db.Create(&terminal).Error; err != nil {
		t.Fatal(err)
	}
	bad.UserID = f.owner
	bad.ID = uuid.New()
	bad.Sequence = 3
	if err := db.Create(&bad).Error; err == nil {
		t.Fatal("closed run accepted more audit events")
	}
	if _, err := svc.Record(ctx, f.owner, diagnosisDecisionCommand(f, 1)); !errors.Is(err, ErrDiagnosisAnalysisClosed) {
		t.Fatalf("closed run accepted choice: %v", err)
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 1)
	diagnosisDecisionCount(t, db, f.owner, "condition_analysis_events", 2)
	assertDiagnosisNotAdopted(t, db, f.owner)
}

func TestDiagnosisProposalDecisionSupersessionAndForeignEvidencePostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("superseded content cannot take a fresh choice", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		replacement := f.proposals[0]
		replacement.ID = uuid.New()
		replacement.SupersedesProposalID = &f.proposals[0].ID
		if err := db.Create(&replacement).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := NewDiagnosisProposalDecisionService(db).Record(ctx, f.owner, diagnosisDecisionCommand(f, 0)); !errors.Is(err, ErrDiagnosisDecisionConflict) {
			t.Fatalf("superseded choice: %v", err)
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 0)
	})
	t.Run("foreign evidence is owner masked even within owned proposal", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		other := seedDiagnosisDecisionFixture(t, db)
		p := f.proposals[0]
		p.ID = uuid.New()
		p.EvidenceRefs = []model.DiagnosisEvidenceRef{{EvidenceType: model.DiagnosisBodyFact, EvidenceID: other.fact.ID, Version: "1"}}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		command := diagnosisDecisionCommand(f, 0)
		command.ProposalID = p.ID
		if _, err := NewDiagnosisProposalDecisionService(db).Record(ctx, f.owner, command); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatalf("foreign evidence not masked: %v", err)
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 0)
	})
	t.Run("assistant message cannot authorize a formal run", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		if err := db.Model(&model.Message{}).Where("id=? AND conversation_id=?", f.run.RequestMessageID, *f.request.SourceConversationID).Update("role", "assistant").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := NewDiagnosisProposalDecisionService(db).Record(ctx, f.owner, diagnosisDecisionCommand(f, 0)); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatalf("non-user formal request accepted: %v", err)
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 0)
	})
	t.Run("training restriction is not a diagnosis stop", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		if err := db.Model(&model.SafetyScreen{}).Where("user_id=? AND id=?", f.owner, f.screen.ID).Update("numbness", true).Error; err != nil {
			t.Fatal(err)
		}
		r := model.SafetyRestriction{ID: uuid.New(), UserID: f.owner, SafetyScreenID: f.screen.ID, Concept: "numbness", PolicyRevision: model.SafetyScreenRestrictionPolicyV1, RiskLevel: "restricted", ScopeKind: "all_training", LiftPolicy: "professional_clearance", Reason: "Synthetic current reviewed policy", CreatedRevision: 1, ValidFrom: time.Now().UTC().Add(-time.Minute)}
		if err := db.Create(&r).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := NewDiagnosisProposalDecisionService(db).Record(ctx, f.owner, diagnosisDecisionCommand(f, 0)); err != nil {
			t.Fatalf("ordinary restriction prevented diagnosis choice: %v", err)
		}
		assertDiagnosisNotAdopted(t, db, f.owner)
	})
}
