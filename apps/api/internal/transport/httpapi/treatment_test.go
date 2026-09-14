package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeTreatmentTrainingApplication struct {
	calls          int
	userID         uuid.UUID
	revisionID     uuid.UUID
	consultationID *uuid.UUID
	treatment      *model.Treatment
	plan           *model.TrainingPlan
	err            error
}

func (f *fakeTreatmentTrainingApplication) AcceptTreatmentAndEnsurePlan(
	_ context.Context,
	userID uuid.UUID,
	revisionID uuid.UUID,
	consultationID *uuid.UUID,
) (*model.Treatment, *model.TrainingPlan, error) {
	f.calls++
	f.userID = userID
	f.revisionID = revisionID
	f.consultationID = consultationID
	return f.treatment, f.plan, f.err
}

func newTreatmentRouteTestRouter(t *testing.T, training treatmentTrainingApplication) (*gin.Engine, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	server := NewPublicServer(nil).WithTreatment(nil, training, nil)
	RegisterRoutes(r, StrictHandler(server), RouteSecurity{
		Auth: func(c *gin.Context) {
			c.Set("user_id", userID.String())
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return r, userID
}

func TestTreatmentPublicProjectionRemovesPersistenceUserIdentity(t *testing.T) {
	userID := uuid.New()
	revision := &model.TreatmentRevision{
		ID: uuid.New(), TreatmentID: uuid.New(), Revision: 1,
		AcceptanceState: "proposed", LifecycleState: "active",
		SourceBodyStateRevision: 7, SourceDiagnosisAnalysisID: uuid.New(),
		Goal: "restore shoulder control", DurationWeeks: 4,
		Plan:            datatypes.JSON(`{"summary":"plan","goal":"restore shoulder control","duration_weeks":4,"interventions":[],"daily_habits":[],"expected_timeline":"4 weeks","warning_signs":[],"review_triggers":[],"safety_notes":[]}`),
		UserConstraints: datatypes.JSON(`{}`), EvidenceIDs: datatypes.JSON(`[]`), Governance: datatypes.JSON(`{}`),
		AgentConfigurationID: "treatment-v2", AgentConfiguration: datatypes.JSON(`{}`), ExecutionProvenance: datatypes.JSON(`{}`),
		EvidenceAcquisitionTrace: datatypes.JSON(`{}`), GenerationDecisionTrace: datatypes.JSON(`{}`), AcceptanceDecisionTrace: datatypes.JSON(`{}`),
		RolloutProvenance: datatypes.JSON(`{}`), CreatedAt: time.Now().UTC(),
		Interventions: []model.Intervention{{
			ID: uuid.New(), UserID: userID, TreatmentID: uuid.New(), TreatmentRevisionID: uuid.New(),
			Kind: "exercise", Title: "Scapular control", Description: "controlled movement",
			Prescription: datatypes.JSON(`{"sets":3}`), Position: 0, Status: "active",
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}},
	}
	projected, err := publicTreatmentRevisionMap(revision)
	if err != nil {
		t.Fatalf("project treatment revision: %v", err)
	}
	interventions, ok := projected["interventions"].([]any)
	if !ok || len(interventions) != 1 {
		t.Fatalf("interventions=%#v", projected["interventions"])
	}
	intervention, ok := interventions[0].(map[string]any)
	if !ok {
		t.Fatalf("intervention=%#v", interventions[0])
	}
	if _, leaked := intervention["user_id"]; leaked {
		t.Fatalf("intervention leaked user_id: %#v", intervention)
	}

	treatmentMap, err := publicTreatmentMap(&model.Treatment{ID: uuid.New(), UserID: userID})
	if err != nil {
		t.Fatalf("project treatment: %v", err)
	}
	if _, leaked := treatmentMap["user_id"]; leaked {
		t.Fatalf("treatment leaked user_id: %#v", treatmentMap)
	}
	planMap, err := publicTrainingPlanMap(&model.TrainingPlan{ID: uuid.New(), UserID: userID})
	if err != nil {
		t.Fatalf("project training plan: %v", err)
	}
	if _, leaked := planMap["user_id"]; leaked {
		t.Fatalf("training plan leaked user_id: %#v", planMap)
	}
	outcomeMap, err := publicOutcomeMap(&model.Outcome{ID: uuid.New(), UserID: userID})
	if err != nil {
		t.Fatalf("project outcome: %v", err)
	}
	if _, leaked := outcomeMap["user_id"]; leaked {
		t.Fatalf("outcome leaked user_id: %#v", outcomeMap)
	}
}

func TestAcceptTreatmentRevisionUsesAtomicTrainingBoundary(t *testing.T) {
	now := time.Now().UTC()
	revisionID := uuid.New()
	training := &fakeTreatmentTrainingApplication{
		treatment: &model.Treatment{
			ID: uuid.New(), UserID: uuid.New(), CurrentRevision: 1, Status: "active",
			StatusReasons: datatypes.JSON(`[]`), CreatedAt: now, UpdatedAt: now,
		},
		plan: &model.TrainingPlan{
			ID: uuid.New(), UserID: uuid.New(), Status: "active", Goal: "restore capacity",
			DurationWeeks: 4, CurrentWeek: 1, Phases: json.RawMessage(`[]`), CreatedAt: now,
		},
	}
	r, userID := newTreatmentRouteTestRouter(t, training)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/treatments/revisions/"+revisionID.String()+"/accept", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if training.calls != 1 || training.userID != userID || training.revisionID != revisionID {
		t.Fatalf("atomic acceptance call mismatch: calls=%d user=%s revision=%s", training.calls, training.userID, training.revisionID)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"treatment", "training_plan"} {
		item, ok := body[key].(map[string]any)
		if !ok {
			t.Fatalf("%s=%#v", key, body[key])
		}
		if _, leaked := item["user_id"]; leaked {
			t.Fatalf("%s leaked user_id: %#v", key, item)
		}
	}
}

func TestTreatmentReplayReportMatchesGeneratedSchema(t *testing.T) {
	layer := service.TreatmentReplayLayer{Match: true, Checks: []service.TreatmentReplayCheck{}}
	snapshot := service.TreatmentReplaySnapshot{
		Status: "completed", GovernanceVerdict: "accepted",
		InterventionKinds: []string{}, EvidenceIDs: []string{}, InterventionTitles: []string{},
	}
	decision := service.TreatmentDecision{
		PolicyRevision: service.TreatmentDecisionPolicyV1,
		Phase:          service.TreatmentDecisionGeneration,
		Outcome:        service.TreatmentAllowProposal,
		Reasons:        []string{},
	}
	report := &service.TreatmentReplayReport{
		Mode: "historical", SourceRevisionID: uuid.New(),
		SourceConfigurationID: "treatment-v2", TargetConfigurationID: "treatment-v2",
		InputFingerprint: "sha256:test", ArtifactIntegrity: layer,
		SourceGenerationDecision: decision, GenerationDecision: decision,
		Baseline: snapshot, Replay: snapshot,
		Comparison: service.TreatmentReplayComparison{Hard: layer, Semantic: layer, Presentation: layer},
		Output:     json.RawMessage(`{}`),
	}
	if _, err := strictOpenAPIConvert[openapiv1.TreatmentReplayReport]("TreatmentReplayReport", report); err != nil {
		t.Fatalf("treatment replay report violates generated schema: %v", err)
	}
}
