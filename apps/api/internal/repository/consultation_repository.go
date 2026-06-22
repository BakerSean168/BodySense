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

// GetByID retrieves a consultation session by ID and user ID.
func (r *ConsultationRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.ConsultationSession, error) {
	var session model.ConsultationSession
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// ListByUserID retrieves consultation sessions for a user, ordered by creation time desc.
func (r *ConsultationRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.ConsultationSession, int64, error) {
	var sessions []model.ConsultationSession
	var total int64

	query := r.db.WithContext(ctx).Where("user_id = ?", userID)

	// Count total
	if err := query.Model(&model.ConsultationSession{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch with pagination
	err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&sessions).Error

	if err != nil {
		return nil, 0, err
	}
	return sessions, total, nil
}

// UpdateMessages updates the messages JSONB field of a session.
func (r *ConsultationRepository) UpdateMessages(ctx context.Context, id uuid.UUID, messages any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("id = ?", id).
		Update("messages", messages).Error
}

// UpdateExtractedInfo updates the extracted_info JSONB field of a session.
func (r *ConsultationRepository) UpdateExtractedInfo(ctx context.Context, id uuid.UUID, extractedInfo any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("id = ?", id).
		Update("extracted_info", extractedInfo).Error
}

// UpdateStatus updates the status of a session.
func (r *ConsultationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	updates := map[string]any{"status": status}
	if status == "completed" {
		updates["ended_at"] = gorm.Expr("NOW()")
	}
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateDiagnosis updates the diagnosis field of a session.
func (r *ConsultationRepository) UpdateDiagnosis(ctx context.Context, id uuid.UUID, diagnosis any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("id = ?", id).
		Update("diagnosis", diagnosis).Error
}

// UpdateTreatmentPlan updates the treatment_plan field of a session.
func (r *ConsultationRepository) UpdateTreatmentPlan(ctx context.Context, id uuid.UUID, treatmentPlan any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("id = ?", id).
		Update("treatment_plan", treatmentPlan).Error
}
