package service

import (
	"context"
	"encoding/json"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// ConsultationService handles consultation business logic.
type ConsultationService struct {
	consultationRepo *repository.ConsultationRepository
}

// NewConsultationService creates a new ConsultationService.
func NewConsultationService(consultationRepo *repository.ConsultationRepository) *ConsultationService {
	return &ConsultationService{consultationRepo: consultationRepo}
}

// CreateSession creates a new consultation session for a user.
func (s *ConsultationService) CreateSession(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
	// Check if there is an existing in-progress empty session
	existingSession, err := s.consultationRepo.GetLastInProgressEmptySession(ctx, userID)
	if err == nil && existingSession != nil {
		return existingSession, nil
	}

	session := &model.ConsultationSession{
		ID:            uuid.New(),
		UserID:        userID,
		Messages:      json.RawMessage("[]"),
		ExtractedInfo: json.RawMessage("[]"),
		Status:        "in_progress",
	}
	if err := s.consultationRepo.Create(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// GetSession retrieves a consultation session by ID.
func (s *ConsultationService) GetSession(ctx context.Context, id, userID uuid.UUID) (*model.ConsultationSession, error) {
	return s.consultationRepo.GetByID(ctx, id, userID)
}

// ListSessions retrieves consultation sessions for a user with pagination.
func (s *ConsultationService) ListSessions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.ConsultationSession, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.consultationRepo.ListByUserID(ctx, userID, limit, offset)
}

// AppendMessage appends a message to the session's messages array.
func (s *ConsultationService) AppendMessage(ctx context.Context, sessionID uuid.UUID, message map[string]any) error {
	// Fetch current session to get existing messages
	session, err := s.consultationRepo.GetByID(ctx, sessionID, uuid.Nil)
	if err != nil {
		return err
	}
	if session == nil {
		return nil
	}

	// Parse existing messages
	var messages []any
	if len(session.Messages) > 0 {
		if err := json.Unmarshal(session.Messages, &messages); err != nil {
			messages = []any{}
		}
	}
	if messages == nil {
		messages = []any{}
	}

	// Append new message
	messages = append(messages, message)

	// Marshal back to JSON
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	return s.consultationRepo.UpdateMessages(ctx, sessionID, data)
}

// UpdateExtractedInfo updates the extracted info for a session.
func (s *ConsultationService) UpdateExtractedInfo(ctx context.Context, sessionID uuid.UUID, extractedInfo any) error {
	data, err := json.Marshal(extractedInfo)
	if err != nil {
		return err
	}
	return s.consultationRepo.UpdateExtractedInfo(ctx, sessionID, data)
}

// CompleteSession marks a session as completed.
func (s *ConsultationService) CompleteSession(ctx context.Context, sessionID uuid.UUID) error {
	return s.consultationRepo.UpdateStatus(ctx, sessionID, "completed")
}

// UpdateDiagnosis updates the diagnosis for a session.
func (s *ConsultationService) UpdateDiagnosis(ctx context.Context, sessionID uuid.UUID, diagnosis any) error {
	data, err := json.Marshal(diagnosis)
	if err != nil {
		return err
	}
	return s.consultationRepo.UpdateDiagnosis(ctx, sessionID, data)
}

// UpdateTreatmentPlan updates the treatment plan for a session.
func (s *ConsultationService) UpdateTreatmentPlan(ctx context.Context, sessionID uuid.UUID, treatmentPlan any) error {
	data, err := json.Marshal(treatmentPlan)
	if err != nil {
		return err
	}
	return s.consultationRepo.UpdateTreatmentPlan(ctx, sessionID, data)
}
