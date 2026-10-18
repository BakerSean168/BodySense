package service

import (
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func workspaceDiagnosis(disposition model.ConditionDisposition) *NativeDiagnosisView {
	owner := uuid.New()
	return &NativeDiagnosisView{
		UserID: owner, CurrentRevision: 2,
		ConditionEntries: []NativeDiagnosisCondition{testAdoptedCondition(owner, 2, disposition)},
	}
}

func TestWorkspaceCapabilitiesUnscreenedSymptomBlocksTrainingButAllowsDiagnosis(t *testing.T) {
	owner := uuid.New()
	caps := deriveWorkspaceCapabilities(
		&BodyStateReadModel{
			UserID: owner, CurrentRevision: 2,
			Facts: []model.BodyFact{{ID: uuid.New(), UserID: owner, Kind: "symptom", AssertionStatus: "asserted", CreatedRevision: 1}},
		},
		workspaceDiagnosis(model.ConditionAdopted),
		nil,
		&model.TrainingPlan{Status: "active"},
	)
	if !caps.RequiresSafetyReview {
		t.Fatal("expected safety review capability")
	}
	if !caps.CanRequestDiagnosis || caps.CanGenerateTreatment || caps.CanExecuteTreatment {
		t.Fatalf("unscreened symptom must allow diagnosis collection and block training: %#v", caps)
	}
	if !caps.CanContinueConsultation || !caps.CanEditBodyState {
		t.Fatal("safety review must not disable correction or consultation")
	}
}

func TestWorkspaceCapabilitiesRequireAnAdoptedConditionForTreatment(t *testing.T) {
	for name, diagnosis := range map[string]*NativeDiagnosisView{
		"no diagnosis":  nil,
		"empty head":    {UserID: uuid.New()},
		"only inactive": workspaceDiagnosis(model.ConditionInactive),
	} {
		caps := deriveWorkspaceCapabilities(screenedWorkspaceBodyState(7), diagnosis, nil, nil)
		if caps.CanGenerateTreatment {
			t.Fatalf("%s: treatment must stay gated until a Condition is adopted: %#v", name, caps)
		}
	}
	caps := deriveWorkspaceCapabilities(screenedWorkspaceBodyState(7), workspaceDiagnosis(model.ConditionAdopted), nil, nil)
	if !caps.CanGenerateTreatment || !caps.CanReviewDiagnosis || caps.RequiresDiagnosisReview {
		t.Fatalf("an adopted Condition with complete Safety should allow generation: %#v", caps)
	}
}

func TestWorkspaceCapabilitiesFlagAnOpenRunForDiagnosisReview(t *testing.T) {
	open := &NativeDiagnosisRunSummary{RunID: uuid.New(), State: "open"}
	caps := deriveWorkspaceCapabilities(screenedWorkspaceBodyState(8), &NativeDiagnosisView{UserID: uuid.New()}, open, nil)
	if !caps.RequiresDiagnosisReview || !caps.CanReviewDiagnosis {
		t.Fatalf("an open native run owes the user a Diagnosis review: %#v", caps)
	}
	committed := &NativeDiagnosisRunSummary{RunID: uuid.New(), State: "committed"}
	caps = deriveWorkspaceCapabilities(screenedWorkspaceBodyState(8), workspaceDiagnosis(model.ConditionAdopted), committed, nil)
	if caps.RequiresDiagnosisReview {
		t.Fatalf("a settled run owes no review: %#v", caps)
	}
	if caps := deriveWorkspaceCapabilities(screenedWorkspaceBodyState(8), nil, nil, nil); caps.CanReviewDiagnosis {
		t.Fatalf("nothing to review without a head or run: %#v", caps)
	}
}

func TestWorkspaceCapabilitiesRequireActiveTrainingPlanForExecution(t *testing.T) {
	baseArgs := func(plan *model.TrainingPlan) HealthWorkspaceCapabilities {
		return deriveWorkspaceCapabilities(screenedWorkspaceBodyState(7), workspaceDiagnosis(model.ConditionAdopted), nil, plan)
	}
	if caps := baseArgs(nil); caps.CanExecuteTreatment || caps.CanRecordOutcome {
		t.Fatalf("no active plan must not be advertised executable: %#v", caps)
	}
	if caps := baseArgs(&model.TrainingPlan{Status: "paused"}); caps.CanExecuteTreatment {
		t.Fatalf("a paused plan must not be advertised executable: %#v", caps)
	}
	if caps := baseArgs(&model.TrainingPlan{Status: "active"}); !caps.CanExecuteTreatment || !caps.CanRecordOutcome {
		t.Fatalf("an active plan with complete Safety should be executable: %#v", caps)
	}
}

func TestWorkspaceActionsOpenTrainingTargetsCanonicalTreatmentView(t *testing.T) {
	conversationID := uuid.New()
	workspace := &HealthWorkspace{
		ConversationID: &conversationID,
		TrainingPlan:   &model.TrainingPlan{ID: uuid.New(), Status: "active"},
		Capabilities:   HealthWorkspaceCapabilities{CanExecuteTreatment: true},
	}

	actions := deriveWorkspaceActions(workspace)
	for _, action := range actions {
		if action.Kind != "open_training" {
			continue
		}
		want := "/consultation/" + conversationID.String() + "?view=treatment"
		if got, _ := action.Target["route"].(string); got != want {
			t.Fatalf("open_training route=%q want=%q", got, want)
		}
		return
	}
	t.Fatal("open_training action missing")
}

func TestWorkspaceTrendsPreserveAssociationOnlyOutcome(t *testing.T) {
	now := time.Now().UTC()
	trends := deriveWorkspaceTrends(
		[]model.Outcome{{
			ID: uuid.New(), ConcernKey: "region:neck", BodyRegion: "颈肩",
			Kind: "symptom_change", SourceType: "training_feedback",
			Value:          datatypes.JSON(`{"trend":"improving","score":3}`),
			CausalityLevel: "association_only", OccurredAt: now,
		}},
	)
	if len(trends) != 1 || trends[0].CurrentTrend != "improving" {
		t.Fatalf("unexpected trends: %#v", trends)
	}
	if trends[0].Points[0].CausalityLevel != "association_only" {
		t.Fatal("trend projection must preserve causality caveat")
	}
}

func screenedWorkspaceBodyState(revision int64) *BodyStateReadModel {
	owner := uuid.New()
	factID := uuid.New()
	return &BodyStateReadModel{UserID: owner, CurrentRevision: revision, Facts: []model.BodyFact{{ID: factID, UserID: owner, Kind: "symptom", AssertionStatus: "asserted", CreatedRevision: 1}}, SafetyScreens: []model.SafetyScreen{{ID: uuid.New(), UserID: owner, BodyFactID: factID, BodyStateRevision: revision}}}
}

func TestWorkspaceCapabilitiesIncompleteSafetyBlocksTraining(t *testing.T) {
	snapshot := screenedWorkspaceBodyState(7)
	snapshot.SafetyScreens = nil
	caps := deriveWorkspaceCapabilities(snapshot, workspaceDiagnosis(model.ConditionAdopted), nil, &model.TrainingPlan{Status: "active"})
	if caps.CanExecuteTreatment || caps.CanGenerateTreatment {
		t.Fatalf("unscreened native symptoms must block training: %+v", caps)
	}
	if !caps.CanContinueConsultation || !caps.CanRequestDiagnosis {
		t.Fatalf("incomplete screening must still allow information collection: %+v", caps)
	}
}
