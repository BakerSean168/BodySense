package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type assessmentRepository interface {
	Create(ctx context.Context, report *model.AssessmentReport) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.AssessmentReport, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.AssessmentReport, int64, error)
}

type assessmentProfileSource interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error)
}

type assessmentUploadSource interface {
	GetUploads(ctx context.Context, userID uuid.UUID) ([]model.UserUploadView, error)
	GetPostureAnalysisSummary(ctx context.Context, userID uuid.UUID) (PostureAnalysisSummary, error)
}

type assessmentBodyStateSource interface {
	GetAgentBodyContext(ctx context.Context, userID uuid.UUID) (*AgentBodyContext, error)
}

type assessmentReasoner interface {
	GenerateAssessment(ctx context.Context, req AssessmentGenerationRequest) (json.RawMessage, error)
}

type AssessmentService struct {
	assessmentRepo assessmentRepository
	profiles       assessmentProfileSource
	uploads        assessmentUploadSource
	reviews        AssessmentReviewSource
	bodyState      assessmentBodyStateSource
	reasoner       assessmentReasoner
	deployment     *AgentDeploymentPolicy
	rollout        *AssessmentRolloutService
}

func NewAssessmentService(
	assessmentRepo assessmentRepository,
	profiles assessmentProfileSource,
	uploads assessmentUploadSource,
	bodyState assessmentBodyStateSource,
	reasoner assessmentReasoner,
) *AssessmentService {
	return &AssessmentService{
		assessmentRepo: assessmentRepo,
		profiles:       profiles,
		uploads:        uploads,
		bodyState:      bodyState,
		reasoner:       reasoner,
	}
}

// WithAssessmentReviews attaches the durable review projection so Assessment
// derives reviewed report evidence from append-only document_indicator_reviews.
func (s *AssessmentService) WithAssessmentReviews(reviews AssessmentReviewSource) *AssessmentService {
	s.reviews = reviews
	return s
}

// WithAssessmentDeployment attaches the Go-owned deployment policy so Assessment
// generation resolves its champion Agent configuration through the North-Star
// control plane and records provenance/decision trace on the immutable report.
func (s *AssessmentService) WithAssessmentDeployment(p *AgentDeploymentPolicy) *AssessmentService {
	s.deployment = p
	return s
}

// WithAssessmentRollout attaches the Go-owned rollout observer so served reports
// trigger anonymous shadow observations when a Challenger is active.
func (s *AssessmentService) WithAssessmentRollout(r *AssessmentRolloutService) *AssessmentService {
	s.rollout = r
	return s
}

const (
	assessmentOutputContractV2 = "assessment-output-v2"
	assessmentEvidencePolicyV4 = "assessment-evidence-contract-v4"
)

// ErrAssessmentOutputRejected means an upstream/generated Assessment failed the
// trusted evidence contract. No Assessment report or BodyState projection may be
// persisted when this error is returned.
var ErrAssessmentOutputRejected = errors.New("assessment output rejected by evidence governance")

func (s *AssessmentService) GetReport(ctx context.Context, id, userID uuid.UUID) (*model.AssessmentReport, error) {
	return s.assessmentRepo.GetByID(ctx, id, userID)
}

func (s *AssessmentService) ListReports(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.AssessmentReport, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.assessmentRepo.ListByUserID(ctx, userID, limit, offset)
}
