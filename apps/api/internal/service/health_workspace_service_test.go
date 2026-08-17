package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestWorkspaceCapabilitiesSafetyBlocksOrdinaryReasoningAndExecution(t *testing.T) {
	caps := deriveWorkspaceCapabilities(
		&BodyStateSnapshot{
			Facts:       []model.BodyStateFact{{Kind: "red_flags"}},
			SafetyState: json.RawMessage(`{"has_red_flags":true,"status":"requires_review"}`),
		},
		&model.DiagnosisAnalysisRecord{
			Status:     "completed",
			Candidates: []model.DiagnosisCandidateRecord{{Name: "candidate"}},
		},
		&model.DiagnosisAnalysisFreshness{State: model.DiagnosisFreshnessFresh},
		[]model.DiagnosisCandidateAssessment{{State: "confirmed"}},
		&model.Treatment{
			Status:  model.TreatmentStatusActive,
			Current: &model.TreatmentRevision{AcceptanceState: model.TreatmentAcceptanceAccepted},
		},
		&model.TrainingPlan{Status: "active"},
		nil,
	)
	if !caps.RequiresSafetyReview {
		t.Fatal("expected safety review capability")
	}
	if caps.CanRequestDiagnosis || caps.CanGenerateTreatment || caps.CanExecuteTreatment {
		t.Fatalf("safety state must block ordinary reasoning/execution: %#v", caps)
	}
	if !caps.CanContinueConsultation || !caps.CanEditBodyState {
		t.Fatal("safety review must not disable correction or consultation")
	}
}

func TestWorkspaceCapabilitiesExposeProposalAcceptanceWithoutMakingItCurrent(t *testing.T) {
	analysisID := uuid.New()
	candidateID := uuid.New()
	caps := deriveWorkspaceCapabilities(
		&BodyStateSnapshot{CurrentRevision: 7, Facts: []model.BodyStateFact{{Kind: "discomfort"}}},
		&model.DiagnosisAnalysisRecord{
			ID: analysisID, Status: "completed",
			Candidates: []model.DiagnosisCandidateRecord{{ID: candidateID, Name: "candidate"}},
		},
		&model.DiagnosisAnalysisFreshness{State: model.DiagnosisFreshnessFresh},
		[]model.DiagnosisCandidateAssessment{{CandidateID: candidateID, State: "confirmed"}},
		nil,
		nil,
		[]model.TreatmentRevision{{
			AcceptanceState:           model.TreatmentAcceptanceProposed,
			SourceDiagnosisAnalysisID: analysisID,
			SourceBodyStateRevision:   7,
		}},
	)
	if !caps.CanAcceptTreatment || caps.CanExecuteTreatment {
		t.Fatalf("proposal must be reviewable but not executable: %#v", caps)
	}
}

func TestWorkspaceCapabilitiesRequireCandidateAssessmentForTreatment(t *testing.T) {
	analysisID := uuid.New()
	candidateID := uuid.New()
	caps := deriveWorkspaceCapabilities(
		&BodyStateSnapshot{CurrentRevision: 7, Facts: []model.BodyStateFact{{Kind: "discomfort"}}},
		&model.DiagnosisAnalysisRecord{
			ID: analysisID, Status: "completed",
			Candidates: []model.DiagnosisCandidateRecord{{ID: candidateID, Name: "candidate"}},
		},
		&model.DiagnosisAnalysisFreshness{State: model.DiagnosisFreshnessFresh},
		nil,
		nil,
		nil,
		[]model.TreatmentRevision{{
			AcceptanceState:           model.TreatmentAcceptanceProposed,
			SourceDiagnosisAnalysisID: analysisID,
			SourceBodyStateRevision:   7,
		}},
	)
	if caps.CanGenerateTreatment || caps.CanAcceptTreatment {
		t.Fatalf("treatment must remain gated until a candidate is confirmed or unsure: %#v", caps)
	}
}

func TestWorkspaceCapabilitiesRejectPotentiallyStaleDiagnosisForTreatment(t *testing.T) {
	candidateID := uuid.New()
	caps := deriveWorkspaceCapabilities(
		&BodyStateSnapshot{CurrentRevision: 8, Facts: []model.BodyStateFact{{Kind: "discomfort"}}},
		&model.DiagnosisAnalysisRecord{
			ID: uuid.New(), Status: "completed",
			Candidates: []model.DiagnosisCandidateRecord{{ID: candidateID, Name: "candidate"}},
		},
		&model.DiagnosisAnalysisFreshness{State: model.DiagnosisFreshnessPotentiallyStale},
		[]model.DiagnosisCandidateAssessment{{CandidateID: candidateID, State: "confirmed"}},
		nil,
		nil,
		nil,
	)
	if caps.CanGenerateTreatment || !caps.RequiresDiagnosisReview {
		t.Fatalf("potentially stale analysis must be reviewed before treatment generation: %#v", caps)
	}
}

func TestWorkspaceCapabilitiesRequireActiveTrainingProjectionForExecution(t *testing.T) {
	analysisID := uuid.New()
	candidateID := uuid.New()
	baseArgs := func(plan *model.TrainingPlan) dto.HealthWorkspaceCapabilities {
		return deriveWorkspaceCapabilities(
			&BodyStateSnapshot{CurrentRevision: 7, Facts: []model.BodyStateFact{{Kind: "discomfort"}}},
			&model.DiagnosisAnalysisRecord{
				ID: analysisID, Status: "completed",
				Candidates: []model.DiagnosisCandidateRecord{{ID: candidateID, Name: "candidate"}},
			},
			&model.DiagnosisAnalysisFreshness{State: model.DiagnosisFreshnessFresh},
			[]model.DiagnosisCandidateAssessment{{CandidateID: candidateID, State: "confirmed"}},
			&model.Treatment{Status: model.TreatmentStatusActive, Current: &model.TreatmentRevision{AcceptanceState: model.TreatmentAcceptanceAccepted}},
			plan,
			nil,
		)
	}
	if caps := baseArgs(nil); caps.CanExecuteTreatment {
		t.Fatalf("accepted treatment without projection must not be advertised executable: %#v", caps)
	}
	if caps := baseArgs(&model.TrainingPlan{Status: "active"}); !caps.CanExecuteTreatment {
		t.Fatalf("active projection should make treatment executable: %#v", caps)
	}
}

func TestWorkspaceTrendsPreserveAssociationOnlyOutcome(t *testing.T) {
	now := time.Now().UTC()
	trends := deriveWorkspaceTrends(
		&BodyStateSnapshot{},
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
