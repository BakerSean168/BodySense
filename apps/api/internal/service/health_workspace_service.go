package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type workspaceProfileSource interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error)
}

type workspaceConsultationSource interface {
	GetLatestForUser(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error)
}

type workspaceBodyStateSource interface {
	GetBodyState(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateReadModel, error)
}

type workspaceDiagnosisSource interface {
	GetLatest(ctx context.Context, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	ListAssessments(ctx context.Context, userID, analysisID uuid.UUID) ([]model.DiagnosisCandidateAssessment, error)
	PublicPayload(analysis *model.DiagnosisAnalysisRecord) map[string]any
}

type workspaceFreshnessSource interface {
	Preview(ctx context.Context, userID uuid.UUID, analysis *model.DiagnosisAnalysisRecord) (*model.DiagnosisAnalysisFreshness, error)
}

type workspaceOutcomeSource interface {
	ListOutcomes(ctx context.Context, userID uuid.UUID, limit int) ([]model.Outcome, error)
}

type workspaceTrainingSource interface {
	GetActivePlan(ctx context.Context, userID uuid.UUID) (*model.TrainingPlan, error)
}

// HealthWorkspaceService is the single read model for the continuous product
// loop. It derives capabilities directly from durable domain objects.
type HealthWorkspaceService struct {
	profiles      workspaceProfileSource
	consultations workspaceConsultationSource
	bodyState     workspaceBodyStateSource
	diagnosis     workspaceDiagnosisSource
	freshness     workspaceFreshnessSource
	outcomes      workspaceOutcomeSource
	training      workspaceTrainingSource
}

func NewHealthWorkspaceService(
	profiles workspaceProfileSource,
	consultations workspaceConsultationSource,
	bodyState workspaceBodyStateSource,
	diagnosis workspaceDiagnosisSource,
	freshness workspaceFreshnessSource,
	outcomes workspaceOutcomeSource,
	training workspaceTrainingSource,
) *HealthWorkspaceService {
	return &HealthWorkspaceService{
		profiles: profiles, consultations: consultations, bodyState: bodyState,
		diagnosis: diagnosis, freshness: freshness, outcomes: outcomes, training: training,
	}
}

func (s *HealthWorkspaceService) AttachTrainingSource(training workspaceTrainingSource) {
	s.training = training
}

func (s *HealthWorkspaceService) Get(
	ctx context.Context,
	userID uuid.UUID,
) (*HealthWorkspace, error) {
	workspace := &HealthWorkspace{
		GeneratedAt:    time.Now().UTC(),
		Diagnosis:      map[string]any{},
		RecentOutcomes: []model.Outcome{}, Trends: []HealthWorkspaceTrend{},
		Actions: []HealthWorkspaceAction{},
	}
	if s.profiles != nil {
		profile, err := s.profiles.GetProfile(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load workspace profile: %w", err)
		}
		workspace.ProfileReady = profile != nil
	}
	if s.consultations != nil {
		session, err := s.consultations.GetLatestForUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load workspace consultation: %w", err)
		}
		if session != nil {
			workspace.ConversationID = &session.ConversationID
		}
	}

	snapshot, err := s.bodyState.GetBodyState(ctx, userID, 30)
	if err != nil {
		return nil, fmt.Errorf("load workspace body state: %w", err)
	}

	analysis, err := s.diagnosis.GetLatest(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load workspace diagnosis: %w", err)
	}
	var freshness *model.DiagnosisAnalysisFreshness
	var assessments []model.DiagnosisCandidateAssessment
	if analysis != nil {
		workspace.Diagnosis = s.diagnosis.PublicPayload(analysis)
		assessments, err = s.diagnosis.ListAssessments(ctx, userID, analysis.ID)
		if err != nil {
			return nil, fmt.Errorf("load workspace diagnosis assessments: %w", err)
		}
		workspace.Diagnosis["candidate_assessments"] = assessments
		freshness, err = s.freshness.Preview(ctx, userID, analysis)
		if err != nil {
			return nil, fmt.Errorf("evaluate workspace diagnosis freshness: %w", err)
		}
		workspace.Diagnosis["freshness"] = freshness
	}

	if s.training != nil {
		workspace.TrainingPlan, err = s.training.GetActivePlan(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load workspace training plan: %w", err)
		}
	}
	workspace.RecentOutcomes, err = s.outcomes.ListOutcomes(ctx, userID, 100)
	if err != nil {
		return nil, fmt.Errorf("load workspace outcomes: %w", err)
	}
	workspace.Trends = deriveWorkspaceTrends(workspace.RecentOutcomes)
	workspace.Capabilities = deriveWorkspaceCapabilities(
		snapshot, analysis, freshness, assessments, workspace.TrainingPlan,
	)
	workspace.Actions = deriveWorkspaceActions(workspace)
	return workspace, nil
}

// deriveWorkspaceCapabilities gates the loop from durable state. Generation
// only creates a pending PlanProposal; execution needs an active TrainingPlan
// and complete Safety coverage.
func deriveWorkspaceCapabilities(
	snapshot *BodyStateReadModel,
	analysis *model.DiagnosisAnalysisRecord,
	freshness *model.DiagnosisAnalysisFreshness,
	assessments []model.DiagnosisCandidateAssessment,
	trainingPlan *model.TrainingPlan,
) HealthWorkspaceCapabilities {
	envelope, safetyErr := ProjectSafetyEnvelopeV2(snapshot)
	requiresSafety := safetyErr != nil || envelope.RequiresReview || len(envelope.Coverage.IncompleteSourceRefs) > 0
	diagnosisBlocked := safetyErr != nil
	for _, blocker := range envelope.ActiveBlockers {
		if blocker.RiskLevel == "urgent" {
			diagnosisBlocked = true
		}
	}
	trainingSafetyIncomplete := requiresSafety || !envelope.Coverage.Complete
	hasReasoningInput := len(snapshot.Facts) > 0 || len(snapshot.Observations) > 0
	diagnosisReview := freshness != nil && freshness.State != model.DiagnosisFreshnessFresh
	analysisEligible := analysis != nil &&
		(analysis.Status == "completed" || analysis.Status == "partial") &&
		len(analysis.Candidates) > 0 &&
		treatmentCandidateAssessmentsReady(analysis, assessments) &&
		(freshness == nil || freshness.State == model.DiagnosisFreshnessFresh)
	activePlan := trainingPlan != nil && trainingPlan.Status == "active"
	return HealthWorkspaceCapabilities{
		CanContinueConsultation: true,
		CanEditBodyState:        true,
		CanRequestDiagnosis:     hasReasoningInput && !diagnosisBlocked,
		CanReviewDiagnosis:      analysis != nil,
		CanGenerateTreatment:    analysisEligible && !trainingSafetyIncomplete,
		CanExecuteTreatment:     activePlan && !trainingSafetyIncomplete,
		CanRecordOutcome:        activePlan,
		RequiresSafetyReview:    requiresSafety,
		RequiresDiagnosisReview: diagnosisReview,
	}
}

func deriveWorkspaceActions(workspace *HealthWorkspace) []HealthWorkspaceAction {
	caps := workspace.Capabilities
	actions := make([]HealthWorkspaceAction, 0, 9)
	add := func(kind string, priority int, enabled bool, reason string, target map[string]any) {
		actions = append(actions, HealthWorkspaceAction{
			Kind: kind, Priority: priority, Enabled: enabled, Reason: reason, Target: target,
		})
	}
	if !workspace.ProfileReady {
		add("complete_profile", 110, true, "完善身体档案后再进入长期健康管理。", map[string]any{"route": "/onboarding"})
	}
	if caps.RequiresSafetyReview {
		add("review_safety", 100, true, "存在需要优先审核的安全状态。", map[string]any{"section": "safety"})
	}
	if caps.RequiresDiagnosisReview {
		add("review_diagnosis", 85, true, "最近分析与当前 BodyState 可能不再完全一致。", map[string]any{"section": "diagnosis"})
	}
	if caps.CanExecuteTreatment && workspace.TrainingPlan != nil {
		add("open_training", 75, true, "当前训练计划可以继续执行。", map[string]any{"route": workspaceTreatmentRoute(workspace.ConversationID)})
	}
	if caps.CanGenerateTreatment {
		add("generate_treatment", 70, true, "当前分析可用于生成一个需你确认的训练计划 proposal。", map[string]any{"section": "diagnosis"})
	}
	if caps.CanRequestDiagnosis {
		add("request_diagnosis", 60, true, "当前 BodyState 已有可分析信息。", map[string]any{"section": "diagnosis"})
	}
	if caps.CanRecordOutcome {
		add("record_outcome", 50, true, "记录干预后的变化以更新长期趋势。", map[string]any{"section": "outcomes"})
	}
	add("continue_consultation", 40, true, "健康状态持续变化，可随时补充或纠正信息。", map[string]any{"conversation_id": workspace.ConversationID})
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].Priority > actions[j].Priority })
	return actions
}

func workspaceTreatmentRoute(conversationID *uuid.UUID) string {
	route := "/consultation"
	if conversationID != nil {
		route += "/" + conversationID.String()
	}
	return route + "?view=treatment"
}

func deriveWorkspaceTrends(
	outcomes []model.Outcome,
) []HealthWorkspaceTrend {
	byKey := map[string]*HealthWorkspaceTrend{}
	for _, outcome := range outcomes {
		key := strings.Join([]string{outcome.ConcernKey, outcome.BodyRegion, outcome.Kind}, "|")
		trend, exists := byKey[key]
		if !exists {
			trend = &HealthWorkspaceTrend{
				Key: key, ConcernKey: outcome.ConcernKey, BodyRegion: outcome.BodyRegion,
				Kind: outcome.Kind, CurrentTrend: outcomeTrend(outcome.Value),
				Points: []HealthWorkspaceTrendPoint{},
			}
			byKey[key] = trend
		}
		trend.Points = append(trend.Points, HealthWorkspaceTrendPoint{
			OccurredAt: outcome.OccurredAt, SourceType: outcome.SourceType,
			Value: json.RawMessage(outcome.Value), Notes: outcome.Notes,
			CausalityLevel: outcome.CausalityLevel,
		})
		if value := outcomeTrend(outcome.Value); value != "" {
			trend.CurrentTrend = value
		}
	}
	trends := make([]HealthWorkspaceTrend, 0, len(byKey))
	for _, trend := range byKey {
		sort.Slice(trend.Points, func(i, j int) bool {
			return trend.Points[i].OccurredAt.Before(trend.Points[j].OccurredAt)
		})
		trends = append(trends, *trend)
	}
	sort.Slice(trends, func(i, j int) bool { return trends[i].Key < trends[j].Key })
	return trends
}

func outcomeTrend(raw []byte) string {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	trend, _ := value["trend"].(string)
	return strings.TrimSpace(trend)
}
