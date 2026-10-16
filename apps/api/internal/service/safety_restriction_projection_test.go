package service

import (
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestNativeSafetyResolutionLeavesOverlappingRestriction(t *testing.T) {
	owner := uuid.New()
	screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, BodyFactID: uuid.New(), BodyStateRevision: 1, RecordedAt: time.Now().UTC(), Numbness: true, Weakness: true}
	first := model.SafetyRestrictionRecord{SafetyRestriction: model.SafetyRestriction{ID: uuid.New(), UserID: owner, SafetyScreenID: screen.ID, Concept: "numbness", PolicyRevision: model.SafetyScreenRestrictionPolicyV1, RiskLevel: "restricted", ScopeKind: "all_training", LiftPolicy: "professional_clearance", CreatedRevision: 1, ValidFrom: screen.RecordedAt}, Active: false, ClearanceAvailable: true, Resolution: &model.SafetyResolution{ID: uuid.New(), UserID: owner, Revision: 2}}
	first.Resolution.RestrictionID = first.ID
	second := model.SafetyRestrictionRecord{SafetyRestriction: model.SafetyRestriction{ID: uuid.New(), UserID: owner, SafetyScreenID: screen.ID, Concept: "weakness", PolicyRevision: model.SafetyScreenRestrictionPolicyV1, RiskLevel: "restricted", ScopeKind: "all_training", LiftPolicy: "professional_clearance", CreatedRevision: 1, ValidFrom: screen.RecordedAt}, Active: true}
	view := &BodyStateReadModel{UserID: owner, CurrentRevision: 2, SafetyScreens: []model.SafetyScreen{screen}, SafetyRestrictions: []model.SafetyRestrictionRecord{first, second}}
	envelope, err := ProjectSafetyEnvelopeV2(view)
	if err != nil || len(envelope.ActiveBlockers) != 1 || envelope.ActiveBlockers[0].RestrictionID != second.ID || envelope.ActiveBlockers[0].RiskLevel != "restricted" {
		t.Fatalf("overlapping risk lost: %+v %v", envelope, err)
	}
	view.SafetyRestrictions = nil
	if _, err := ProjectSafetyEnvelopeV2(view); err == nil {
		t.Fatal("positive screen without restriction must fail closed")
	}
}

func TestTreatmentNativeSafetyRejectsInconsistentCoverage(t *testing.T) {
	facts := readyTreatmentDecisionFacts()
	facts.SafetyEnvelope.Coverage.Complete = true
	got := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionGeneration, facts)
	if got.Outcome != TreatmentBlock {
		t.Fatalf("inconsistent coverage authorized treatment: %+v", got)
	}
}

func TestDiagnosisContinuesWithRestrictedTrainingGate(t *testing.T) {
	envelope := completeSafetyEnvelope()
	envelope.RequiresReview = true
	envelope.ActiveBlockers = []SafetyBlockerV1{{RestrictionID: uuid.New(), RiskLevel: "restricted", Concept: SafetyWeakness, SourceRef: "body-state:safety-restriction:test", SourceKind: SafetyStructuredScreen, Reason: SafetyStructuredCurrentSignal}}
	if got := EvaluateDiagnosisDecisionV2(envelope, 12, completedV2Payload()); got.Outcome != DiagnosisAllowNormal {
		t.Fatalf("ordinary training restriction ended diagnosis: %+v", got)
	}
	envelope.ActiveBlockers[0].RiskLevel = "urgent"
	if got := EvaluateDiagnosisDecisionV2(envelope, 12, completedV2Payload()); got.Outcome != DiagnosisBlock {
		t.Fatalf("urgent restriction allowed diagnosis: %+v", got)
	}
}

func testNativeSafetyEnvelope(revision int64) *SafetyEnvelopeV2 {
	e, err := ProjectSafetyEnvelopeV2(&BodyStateReadModel{CurrentRevision: revision})
	if err != nil {
		panic(err)
	}
	return &e
}
func testBlockedSafetyEnvelope(revision int64) *SafetyEnvelopeV2 {
	e := testNativeSafetyEnvelope(revision)
	e.RequiresReview = true
	e.ActiveBlockers = []SafetyBlockerV1{{RestrictionID: uuid.New(), RiskLevel: "restricted", Concept: SafetyWeakness, SourceRef: "body-state:safety-screen:test", SourceKind: SafetyStructuredScreen, Reason: SafetyStructuredCurrentSignal}}
	return e
}
