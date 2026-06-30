package service

import (
	"context"
	"log"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// OutputReviewService handles AI output governance persistence.
type OutputReviewService struct {
	repo *repository.AIOutputReviewRepository
}

// NewOutputReviewService creates a new OutputReviewService.
func NewOutputReviewService(repo *repository.AIOutputReviewRepository) *OutputReviewService {
	return &OutputReviewService{repo: repo}
}

// RecordReview persists a governance review result.
// Non-blocking: logs errors but does not fail the caller.
func (s *OutputReviewService) RecordReview(
	ctx context.Context,
	outputType, status string,
	runID, jobID, conversationID *uuid.UUID,
	issues, validatedOutput, rawOutput datatypes.JSON,
) {
	review := &model.AIOutputReview{
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
