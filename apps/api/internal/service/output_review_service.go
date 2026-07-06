package service

import (
	"context"
	"log"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// OutputReviewRepo defines the repository interface for review persistence.
type OutputReviewRepo interface {
	Create(ctx context.Context, review *model.AIOutputReview) error
}

// OutputReviewService handles AI output governance persistence.
type OutputReviewService struct {
	repo OutputReviewRepo
}

// NewOutputReviewService creates a new OutputReviewService.
func NewOutputReviewService(repo OutputReviewRepo) *OutputReviewService {
	return &OutputReviewService{repo: repo}
}

// RecordReview persists a governance review result.
// Non-blocking: logs errors but does not fail the caller.
func (s *OutputReviewService) RecordReview(
	ctx context.Context,
	outputType, status string,
	userID, runID, jobID, conversationID *uuid.UUID,
	issues, validatedOutput, rawOutput datatypes.JSON,
) {
	review := &model.AIOutputReview{
		UserID:          userID,
		RunID:           runID,
		JobID:           jobID,
		ConversationID:  conversationID,
		OutputType:      outputType,
		Status:          status,
		Issues:          issues,
		ValidatedOutput: validatedOutput,
		RawOutput:       rawOutput,
	}

	if err := s.repo.Create(ctx, review); err != nil {
		log.Printf("failed to record output review (type=%s, status=%s): %v", outputType, status, err)
	}
}
