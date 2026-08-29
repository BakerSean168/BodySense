package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bodysense/api/internal/dto"
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
	GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateSnapshot, error)
	ListReviewableFacts(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateFact, error)
	ListReviewableObservations(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateObservation, error)
}

type workspaceDiagnosisSource interface {
	GetLatest(ctx context.Context, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	ListAssessments(ctx context.Context, userID, analysisID uuid.UUID) ([]model.DiagnosisCandidateAssessment, error)
	PublicPayload(analysis *model.DiagnosisAnalysisRecord) map[string]any
}

type workspaceFreshnessSource interface {
	Preview(ctx context.Context, userID uuid.UUID, analysis *model.DiagnosisAnalysisRecord) (*model.DiagnosisAnalysisFreshness, error)
}

type workspaceTreatmentSource interface {
	PreviewCurrentReview(ctx context.Context, userID uuid.UUID) (*model.Treatment, error)
	ListRevisions(ctx context.Context, userID uuid.UUID, limit int) ([]model.TreatmentRevision, error)
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
	treatment     workspaceTreatmentSource
	training      workspaceTrainingSource
}

func NewHealthWorkspaceService(
	profiles workspaceProfileSource,
	consultations workspaceConsultationSource,
	bodyState workspaceBodyStateSource,
	diagnosis workspaceDiagnosisSource,
	freshness workspaceFreshnessSource,
	treatment workspaceTreatmentSource,
	training workspaceTrainingSource,
) *HealthWorkspaceService {
	return &HealthWorkspaceService{
		profiles: profiles, consultations: consultations, bodyState: bodyState,
		diagnosis: diagnosis, freshness: freshness, treatment: treatment, training: training,
	}
}

func (s *HealthWorkspaceService) AttachTrainingSource(training workspaceTrainingSource) {
	s.training = training
}

func (s *HealthWorkspaceService) Get(
	ctx context.Context,
	userID uuid.UUID,
) (*dto.HealthWorkspace, error) {
	workspace := &dto.HealthWorkspace{
		GeneratedAt: time.Now().UTC(),
		Diagnosis:   map[string]any{}, TreatmentRevisions: []model.TreatmentRevision{},
		RecentOutcomes: []model.Outcome{}, Trends: []dto.HealthWorkspaceTrend{},
		Actions: []dto.HealthWorkspaceAction{},
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

	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 30)
	if err != nil {
		return nil, fmt.Errorf("load workspace body state: %w", err)
	}
	pendingFacts, err := s.bodyState.ListReviewableFacts(ctx, userID, 50)
	if err != nil {
		return nil, fmt.Errorf("load workspace pending facts: %w", err)
	}
	pendingObservations, err := s.bodyState.ListReviewableObservations(ctx, userID, 50)
	if err != nil {
		return nil, fmt.Errorf("load workspace pending observations: %w", err)
	}
	workspace.BodyState = &dto.HealthWorkspaceBodyState{
		CurrentRevision: snapshot.CurrentRevision, SafetyState: snapshot.SafetyState,
		Facts: snapshot.Facts, PendingFacts: pendingFacts, Observations: snapshot.Observations,
		PendingObservations: pendingObservations,
		Hypotheses:          snapshot.Hypotheses, RecentRevisions: snapshot.RecentRevisions,
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

	workspace.Treatment, err = s.treatment.PreviewCurrentReview(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load workspace treatment: %w", err)
	}
	workspace.TreatmentRevisions, err = s.treatment.ListRevisions(ctx, userID, 20)
	if err != nil {
		return nil, fmt.Errorf("load workspace treatment revisions: %w", err)
	}
	if s.training != nil {
		workspace.TrainingPlan, err = s.training.GetActivePlan(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load workspace training plan: %w", err)
		}
	}
	workspace.RecentOutcomes, err = s.treatment.ListOutcomes(ctx, userID, 100)
	if err != nil {
		return nil, fmt.Errorf("load workspace outcomes: %w", err)
	}
	workspace.Trends = deriveWorkspaceTrends(snapshot, workspace.RecentOutcomes)
	workspace.Capabilities = deriveWorkspaceCapabilities(
		snapshot, analysis, freshness, assessments, workspace.Treatment, workspace.TrainingPlan, workspace.TreatmentRevisions,
	)
	workspace.Actions = deriveWorkspaceActions(workspace)
	return workspace, nil
}

func deriveWorkspaceCapabilities(
	snapshot *BodyStateSnapshot,
	analysis *model.DiagnosisAnalysisRecord,
	freshness *model.DiagnosisAnalysisFreshness,
	assessments []model.DiagnosisCandidateAssessment,
	treatment *model.Treatment,
	trainingPlan *model.TrainingPlan,
	revisions []model.TreatmentRevision,
) dto.HealthWorkspaceCapabilities {
	requiresSafety := bodyStateRequiresSafetyReview(snapshot.SafetyState)
	hasReasoningInput := len(snapshot.Facts) > 0 || len(snapshot.Observations) > 0
	hasAnalysis := analysis != nil
	diagnosisReview := freshness != nil && freshness.State != model.DiagnosisFreshnessFresh
	analysisEligible := analysis != nil &&
		(analysis.Status == "completed" || analysis.Status == "partial") &&
		len(analysis.Candidates) > 0 &&
		treatmentCandidateAssessmentsReady(analysis, assessments) &&
		(freshness == nil || freshness.State == model.DiagnosisFreshnessFresh)
	hasAcceptableProposal := false
	for _, revision := range revisions {
		if revision.AcceptanceState == model.TreatmentAcceptanceProposed &&
			analysisEligible && analysis != nil &&
			revision.SourceDiagnosisAnalysisID == analysis.ID &&
			revision.SourceBodyStateRevision <= snapshot.CurrentRevision {
			hasAcceptableProposal = true
			break
		}
	}
	hasCurrent := treatment != nil && treatment.Current != nil &&
		treatment.Current.AcceptanceState == model.TreatmentAcceptanceAccepted
	treatmentReview := hasCurrent && (treatment.Status == model.TreatmentStatusReviewRecommended || treatment.Status == model.TreatmentStatusPaused)
	return dto.HealthWorkspaceCapabilities{
		CanContinueConsultation: true,
		CanEditBodyState:        true,
		CanRequestDiagnosis:     hasReasoningInput && !requiresSafety,
		CanReviewDiagnosis:      hasAnalysis,
		CanGenerateTreatment:    analysisEligible && !requiresSafety,
		CanAcceptTreatment:      hasAcceptableProposal && !requiresSafety,
		CanExecuteTreatment:     hasCurrent && treatment.Status == model.TreatmentStatusActive && trainingPlan != nil && trainingPlan.Status == "active" && !requiresSafety,
		CanRecordOutcome:        hasCurrent,
		CanReviewTreatment:      hasCurrent,
		RequiresSafetyReview:    requiresSafety,
		RequiresDiagnosisReview: diagnosisReview,
		RequiresTreatmentReview: treatmentReview,
	}
}

func deriveWorkspaceActions(workspace *dto.HealthWorkspace) []dto.HealthWorkspaceAction {
	caps := workspace.Capabilities
	actions := make([]dto.HealthWorkspaceAction, 0, 9)
	add := func(kind string, priority int, enabled bool, reason string, target map[string]any) {
		actions = append(actions, dto.HealthWorkspaceAction{
			Kind: kind, Priority: priority, Enabled: enabled, Reason: reason, Target: target,
		})
	}
	if !workspace.ProfileReady {
		add("complete_profile", 110, true, "完善身体档案后再进入长期健康管理。", map[string]any{"route": "/onboarding"})
	}
	if caps.RequiresSafetyReview {
		add("review_safety", 100, true, "存在需要优先审核的安全状态。", map[string]any{"section": "safety"})
	}
	if caps.RequiresTreatmentReview {
		add("review_treatment", 90, true, "当前方案因新状态变化需要审核。", map[string]any{"section": "treatment"})
	}
	if caps.RequiresDiagnosisReview {
		add("review_diagnosis", 85, true, "最近分析与当前 BodyState 可能不再完全一致。", map[string]any{"section": "diagnosis"})
	}
	if caps.CanAcceptTreatment {
		add("review_treatment_proposal", 80, true, "存在尚未接受的方案版本。", map[string]any{"section": "treatment_history"})
	}
	if caps.CanExecuteTreatment && workspace.TrainingPlan != nil {
		add("open_training", 75, true, "当前已接受方案可以继续执行。", map[string]any{"route": "/training/" + workspace.TrainingPlan.ID.String()})
	}
	if caps.CanGenerateTreatment {
		add("generate_treatment", 70, true, "当前分析可用于创建一个需审核的方案 proposal。", map[string]any{"section": "diagnosis"})
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

func deriveWorkspaceTrends(
	snapshot *BodyStateSnapshot,
	outcomes []model.Outcome,
) []dto.HealthWorkspaceTrend {
	byKey := map[string]*dto.HealthWorkspaceTrend{}
	for _, fact := range snapshot.Facts {
		if fact.Trend == "" || fact.Trend == "unknown" {
			continue
		}
		key := strings.Join([]string{fact.ConcernKey, fact.BodyRegion, "fact"}, "|")
		byKey[key] = &dto.HealthWorkspaceTrend{
			Key: key, ConcernKey: fact.ConcernKey, BodyRegion: fact.BodyRegion,
			Kind: "body_state_fact", CurrentTrend: fact.Trend,
			Points: []dto.HealthWorkspaceTrendPoint{},
		}
	}
	for _, outcome := range outcomes {
		key := strings.Join([]string{outcome.ConcernKey, outcome.BodyRegion, outcome.Kind}, "|")
		trend, exists := byKey[key]
		if !exists {
			trend = &dto.HealthWorkspaceTrend{
				Key: key, ConcernKey: outcome.ConcernKey, BodyRegion: outcome.BodyRegion,
				Kind: outcome.Kind, CurrentTrend: outcomeTrend(outcome.Value),
				Points: []dto.HealthWorkspaceTrendPoint{},
			}
			byKey[key] = trend
		}
		trend.Points = append(trend.Points, dto.HealthWorkspaceTrendPoint{
			OccurredAt: outcome.OccurredAt, SourceType: outcome.SourceType,
			Value: json.RawMessage(outcome.Value), Notes: outcome.Notes,
			CausalityLevel: outcome.CausalityLevel,
		})
		if value := outcomeTrend(outcome.Value); value != "" {
			trend.CurrentTrend = value
		}
	}
	trends := make([]dto.HealthWorkspaceTrend, 0, len(byKey))
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
