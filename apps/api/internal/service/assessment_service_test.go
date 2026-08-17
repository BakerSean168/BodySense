package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeAssessmentRepository struct {
	created   *model.AssessmentReport
	createErr error
}

func (r *fakeAssessmentRepository) Create(_ context.Context, report *model.AssessmentReport) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = report
	return nil
}
func (r *fakeAssessmentRepository) GetByID(context.Context, uuid.UUID, uuid.UUID) (*model.AssessmentReport, error) {
	return r.created, nil
}
func (r *fakeAssessmentRepository) ListByUserID(context.Context, uuid.UUID, int, int) ([]model.AssessmentReport, int64, error) {
	if r.created == nil {
		return nil, 0, nil
	}
	return []model.AssessmentReport{*r.created}, 1, nil
}

type fakeAssessmentProfileSource struct{ profile *model.UserProfile }

func (s fakeAssessmentProfileSource) GetProfile(context.Context, uuid.UUID) (*model.UserProfile, error) {
	return s.profile, nil
}

type fakeAssessmentUploadSource struct{ uploads []model.UserUpload }

func (s fakeAssessmentUploadSource) GetByUserID(context.Context, uuid.UUID) ([]model.UserUpload, error) {
	return s.uploads, nil
}

type fakeAssessmentBodyState struct {
	observations []model.BodyStateObservation
	fail         bool
}

func (s *fakeAssessmentBodyState) AddAssessmentObservation(_ context.Context, userID uuid.UUID, observation model.BodyStateObservation) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	if s.fail {
		return nil, nil, errors.New("projection failed")
	}
	observation.ID = uuid.New()
	observation.UserID = userID
	observation.ReviewState = "unverified"
	observation.ExcludedFromReasoning = true
	revision := int64(len(s.observations) + 1)
	observation.CreatedRevision = revision
	observation.UpdatedRevision = revision
	s.observations = append(s.observations, observation)
	return &observation, &model.BodyStateRevision{Revision: revision}, nil
}

type fakeAssessmentReasoner struct{ raw json.RawMessage }

func (r fakeAssessmentReasoner) GenerateAssessment(context.Context, AssessmentGenerationRequest) (json.RawMessage, error) {
	return r.raw, nil
}

func assessmentOutput() json.RawMessage {
	return json.RawMessage(`{
		"status":"completed",
		"health_grade":"B",
		"dimension_scores":{"posture":72,"exercise":68,"lifestyle":70,"injury_risk":75,"overall":71},
		"observations":[{
			"kind":"posture_alignment","body_region":"肩部","label":"高低肩倾向",
			"description":"右侧肩峰略高","severity":"轻度","confidence":"中",
			"method":"posture_photo_front","condition":{"view":"front"}
		}],
		"summary":"当前资料支持一项待审核观察。",
		"information_gaps":[],"safety_notes":[]
	}`)
}

func TestAssessmentPersistsReportAndUnverifiedBodyStateObservationsAtomically(t *testing.T) {
	age := 30
	userID := uuid.New()
	repo := &fakeAssessmentRepository{}
	bodyState := &fakeAssessmentBodyState{}
	transactionCalled := false
	svc := NewAssessmentService(
		repo,
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID, Age: &age}},
		fakeAssessmentUploadSource{},
		bodyState,
		fakeAssessmentReasoner{raw: assessmentOutput()},
		testTreatmentUnitOfWork{called: &transactionCalled},
	)

	report, err := svc.GenerateAssessment(context.Background(), userID)
	if err != nil {
		t.Fatalf("GenerateAssessment returned error: %v", err)
	}
	if !transactionCalled || repo.created == nil {
		t.Fatal("report and observations must use the coordinated unit of work")
	}
	if len(bodyState.observations) != 1 {
		t.Fatalf("expected one projected observation, got %d", len(bodyState.observations))
	}
	observation := bodyState.observations[0]
	if observation.ReviewState != "unverified" || !observation.ExcludedFromReasoning {
		t.Fatalf("assessment observation must await explicit review: %#v", observation)
	}
	if report.BodyStateRevision == nil || *report.BodyStateRevision != 1 {
		t.Fatalf("report must link its final BodyState revision: %#v", report.BodyStateRevision)
	}
	var reportObservations []map[string]any
	if err := json.Unmarshal(report.Observations, &reportObservations); err != nil {
		t.Fatalf("report observations are invalid JSON: %v", err)
	}
	if reportObservations[0]["observation_id"] == nil || reportObservations[0]["review_state"] != "unverified" {
		t.Fatalf("report must expose review identity/state: %#v", reportObservations)
	}
}

func TestAssessmentRejectsTreatmentLikeLegacyPayload(t *testing.T) {
	userID := uuid.New()
	svc := NewAssessmentService(
		&fakeAssessmentRepository{},
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		&fakeAssessmentBodyState{},
		fakeAssessmentReasoner{raw: json.RawMessage(`{
			"health_grade":"B",
			"dimension_scores":{"posture":70,"exercise":70,"lifestyle":70,"injury_risk":70,"overall":70},
			"observations":[],"summary":{"exercise":"do squats"}
		}`)},
		testTreatmentUnitOfWork{},
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err == nil {
		t.Fatal("legacy issue/advice payload must not cross the assessment contract")
	}
}

func TestAssessmentProjectionFailurePreventsReportPersistence(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAssessmentRepository{}
	svc := NewAssessmentService(
		repo,
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		&fakeAssessmentBodyState{fail: true},
		fakeAssessmentReasoner{raw: assessmentOutput()},
		testTreatmentUnitOfWork{},
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err == nil {
		t.Fatal("BodyState projection failure must fail the assessment write")
	}
	if repo.created != nil {
		t.Fatal("report must not persist after observation projection failure")
	}
}
