package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConsultationRepository handles database operations for consultation sessions.
type ConsultationRepository struct {
	db *gorm.DB
}

// NewConsultationRepository creates a new ConsultationRepository.
func NewConsultationRepository(db *gorm.DB) *ConsultationRepository {
	return &ConsultationRepository{db: db}
}

// Create creates a new consultation session.
func (r *ConsultationRepository) Create(ctx context.Context, session *model.ConsultationSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByConversationID retrieves a consultation session by conversation ID.
func (r *ConsultationRepository) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error) {
	var session model.ConsultationSession
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// ListByConversationIDs retrieves consultation sessions for a set of conversation IDs.
func (r *ConsultationRepository) ListByConversationIDs(ctx context.Context, conversationIDs []uuid.UUID) ([]model.ConsultationSession, error) {
	var sessions []model.ConsultationSession
	if len(conversationIDs) == 0 {
		return sessions, nil
	}
	err := r.db.WithContext(ctx).
		Where("conversation_id IN ?", conversationIDs).
		Find(&sessions).Error
	return sessions, err
}

// UpdateExtractedInfo updates the extracted_info JSONB field of a session.
func (r *ConsultationRepository) UpdateExtractedInfo(ctx context.Context, conversationID uuid.UUID, extractedInfo any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("extracted_info", extractedInfo).Error
}

// UpdatePhase updates the workflow phase of a consultation session.
func (r *ConsultationRepository) UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase string) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("phase", phase).Error
}

// UpdateDiagnosis updates the diagnosis field of a session.
func (r *ConsultationRepository) UpdateDiagnosis(ctx context.Context, conversationID uuid.UUID, diagnosis any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("diagnosis", diagnosis).Error
}

// UpdateTreatmentPlan updates the treatment_plan field of a session.
func (r *ConsultationRepository) UpdateTreatmentPlan(ctx context.Context, conversationID uuid.UUID, treatmentPlan any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("treatment_plan", treatmentPlan).Error
}

// Delete removes a consultation session by conversation ID.
func (r *ConsultationRepository) Delete(ctx context.Context, conversationID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Delete(&model.ConsultationSession{}).Error
}
