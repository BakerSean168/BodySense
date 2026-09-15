package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeAssessmentApplication struct {
	generateReport *model.AssessmentReport
	generateErr    error
	getReport      *model.AssessmentReport
	getErr         error
	listReports    []model.AssessmentReport
	listTotal      int64
	listErr        error
	listCalls      int
	limit          int
	offset         int
}

func (f *fakeAssessmentApplication) GenerateAssessment(context.Context, uuid.UUID) (*model.AssessmentReport, error) {
	return f.generateReport, f.generateErr
}
func (f *fakeAssessmentApplication) GetReport(context.Context, uuid.UUID, uuid.UUID) (*model.AssessmentReport, error) {
	return f.getReport, f.getErr
}
func (f *fakeAssessmentApplication) ListReports(_ context.Context, _ uuid.UUID, limit, offset int) ([]model.AssessmentReport, int64, error) {
	f.listCalls++
	f.limit = limit
	f.offset = offset
	return f.listReports, f.listTotal, f.listErr
}

type fakeAssessmentReplayApplication struct {
	historical *service.AssessmentReplayReport
	counter    *service.AssessmentReplayReport
	export     map[string]any
	err        error
	mode       string
	configID   string
}

func (f *fakeAssessmentReplayApplication) HistoricalReplay(context.Context, uuid.UUID, uuid.UUID) (*service.AssessmentReplayReport, error) {
	f.mode = "historical"
	return f.historical, f.err
}
func (f *fakeAssessmentReplayApplication) CounterfactualReplay(_ context.Context, _ uuid.UUID, _ uuid.UUID, configurationID string) (*service.AssessmentReplayReport, error) {
	f.mode = "counterfactual"
	f.configID = configurationID
	return f.counter, f.err
}
func (f *fakeAssessmentReplayApplication) ExportRegressionCase(context.Context, uuid.UUID, uuid.UUID) (map[string]any, error) {
	return f.export, f.err
}

func newAssessmentOpenAPIRouter(t *testing.T, assessment assessmentApplication, replay assessmentReplayApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	g := r.Group("")
	g.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.NewString())
		c.Set("email", "user@example.com")
		c.Next()
	})
	g.Use(RequestValidator(spec))
	server := NewPublicServer(&fakeBodyStateFactService{}).WithAssessment(assessment, replay)
	openapiv1.RegisterHandlers(g, StrictHandler(server))
	return r
}

func validAssessmentV2Report() *model.AssessmentReport {
	coverage := json.RawMessage(`{
		"status":"partial",
		"available_sources":["body_state"],
		"domains":{
			"posture":{"status":"missing","evidence_refs":[]},
			"exercise":{"status":"available","evidence_refs":["body_state:fact:0"]},
			"lifestyle":{"status":"missing","evidence_refs":[]},
			"anthropometry":{"status":"missing","evidence_refs":[]},
			"health_report":{"status":"missing","evidence_refs":[]},
			"injury_symptoms":{"status":"missing","evidence_refs":[]}
		}
	}`)
	observationID := uuid.New()
	observations := json.RawMessage(fmt.Sprintf(`[{"observation_id":%q,"review_state":"unverified","kind":"exercise_pattern","body_region":"","label":"运动记录","description":"每周训练。","method":"assessment_evidence","evidence_refs":["body_state:fact:0"]}]`, observationID.String()))
	now := time.Now().UTC().Truncate(time.Second)
	revision := int64(4)
	return &model.AssessmentReport{
		ID:                      uuid.New(),
		UserID:                  uuid.New(),
		Status:                  "completed",
		ContractRevision:        "assessment-output-v2",
		EvidenceCoverage:        coverage,
		EvidenceGaps:            json.RawMessage(`[]`),
		Observations:            observations,
		Summary:                 "当前资料支持 1 项待审核观察。",
		InformationGaps:         json.RawMessage(`[]`),
		SafetyNotes:             json.RawMessage(`["不构成医疗诊断。"]`),
		BodyStateRevision:       &revision,
		AgentConfigurationID:    "assessment-v5",
		AgentConfiguration:      datatypes.JSON(`{"id":"assessment-v5","role":"assessment"}`),
		ExecutionProvenance:     datatypes.JSON(`{"status":"executed"}`),
		GenerationDecisionTrace: datatypes.JSON(`{"status":"generated"}`),
		CreatedAt:               now,
	}
}

func TestAssessmentListRejectsInvalidPaginationBeforeApplication(t *testing.T) {
	assessment := &fakeAssessmentApplication{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment?limit=0", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newAssessmentOpenAPIRouter(t, assessment, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	if assessment.listCalls != 0 {
		t.Fatalf("list calls=%d want=0", assessment.listCalls)
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestAssessmentListUsesOpenAPIDefaultsAndValidatesV2Report(t *testing.T) {
	report := validAssessmentV2Report()
	assessment := &fakeAssessmentApplication{listReports: []model.AssessmentReport{*report}, listTotal: 1}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newAssessmentOpenAPIRouter(t, assessment, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	if assessment.limit != 20 || assessment.offset != 0 {
		t.Fatalf("pagination=(%d,%d) want=(20,0)", assessment.limit, assessment.offset)
	}
	assertOpenAPIResponse(t, req, rec)
}

func TestAssessmentReportFailsClosedWhenStoredV2JSONViolatesPublicSchema(t *testing.T) {
	report := validAssessmentV2Report()
	report.EvidenceCoverage = json.RawMessage(`{"status":"partial","available_sources":["body_state"],"domains":{"exercise":{"status":"available","evidence_refs":[]}}}`)
	assessment := &fakeAssessmentApplication{getReport: report}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/"+report.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newAssessmentOpenAPIRouter(t, assessment, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want=500 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INTERNAL_ERROR")
}

func TestAssessmentReplayRequiresConfigurationForCounterfactual(t *testing.T) {
	reportID := uuid.New()
	replay := &fakeAssessmentReplayApplication{}
	rec := performJSONAt(
		newAssessmentOpenAPIRouter(t, &fakeAssessmentApplication{}, replay),
		http.MethodPost,
		"/api/v1/assessment/"+reportID.String()+"/replay",
		`{"mode":"counterfactual"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestAssessmentReplayMapsUnknownConfigurationToBadRequest(t *testing.T) {
	reportID := uuid.New()
	replay := &fakeAssessmentReplayApplication{err: fmt.Errorf("%w: unknown", service.ErrAssessmentReplayConfiguration)}
	rec := performJSONAt(
		newAssessmentOpenAPIRouter(t, &fakeAssessmentApplication{}, replay),
		http.MethodPost,
		"/api/v1/assessment/"+reportID.String()+"/replay",
		`{"mode":"counterfactual","configuration_id":"does-not-exist"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_CONFIGURATION")
}

func validAssessmentReplayReport(reportID uuid.UUID) *service.AssessmentReplayReport {
	layer := service.AssessmentReplayLayer{
		Match:  true,
		Checks: []service.AssessmentReplayCheck{{Name: "contract_revision", Match: true}},
	}
	snapshot := service.AssessmentReplaySnapshot{
		ContractRevision:       "assessment-output-v2",
		Status:                 "completed",
		EvidenceCoverageStatus: "partial",
		ObservationCount:       1,
		ObservationKinds:       []string{"exercise_pattern"},
		InformationGaps:        []string{},
		EvidenceGaps:           []string{"当前未提供已完成的体态分析。"},
		SafetyNoteCount:        1,
		Summary:                "当前资料支持 1 项待审核观察。",
	}
	return &service.AssessmentReplayReport{
		Mode:                  "historical",
		SourceReportID:        reportID,
		SourceConfigurationID: "assessment-v5",
		TargetConfigurationID: "assessment-v5",
		InputFingerprint:      strings.Repeat("a", 64),
		ArtifactIntegrity:     layer,
		Baseline:              snapshot,
		Replay:                snapshot,
		Comparison: service.AssessmentReplayComparison{
			Hard: layer, Semantic: layer, Presentation: layer,
		},
		Output: json.RawMessage(`{"contract_revision":"assessment-output-v2","status":"completed"}`),
	}
}

func TestAssessmentHistoricalReplayReturnsSchemaValidComparison(t *testing.T) {
	reportID := uuid.New()
	replay := &fakeAssessmentReplayApplication{historical: validAssessmentReplayReport(reportID)}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessment/"+reportID.String()+"/replay", strings.NewReader(`{"mode":"historical"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newAssessmentOpenAPIRouter(t, &fakeAssessmentApplication{}, replay).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	if replay.mode != "historical" {
		t.Fatalf("replay mode=%q want=historical", replay.mode)
	}
	assertOpenAPIResponse(t, req, rec)
}

func TestAssessmentRegressionExportReturnsSchemaValidFrozenCase(t *testing.T) {
	reportID := uuid.New()
	replay := &fakeAssessmentReplayApplication{export: map[string]any{
		"schema_target":    "assessment_qualification_v2",
		"source_report_id": reportID,
		"case": map[string]any{
			"name": "historical-" + strings.ReplaceAll(reportID.String()[:13], "-", ""),
			"inputs": map[string]any{
				"user_id":    "historical-regression",
				"profile":    map[string]any{},
				"body_state": map[string]any{},
			},
			"metadata": map[string]any{
				"scenario_family_id":          "historical-" + reportID.String(),
				"case_category":               "historical-regression",
				"split":                       "regression",
				"slices":                      []string{"historical-replay"},
				"critical":                    false,
				"expected_contract_revision":  "assessment-output-v2",
				"expected_status":             "completed",
				"expected_evidence_gap_count": 0,
				"expected_agent_executed":     true,
				"min_observations":            1,
				"forbidden_output_fields":     []string{"treatment", "training_plan", "prescription", "health_grade", "dimension_scores"},
			},
		},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/"+reportID.String()+"/regression-export", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newAssessmentOpenAPIRouter(t, &fakeAssessmentApplication{}, replay).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	assertOpenAPIResponse(t, req, rec)
}
