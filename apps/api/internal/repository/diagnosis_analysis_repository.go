package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DiagnosisAnalysisRepository persists an immutable analysis and all candidate
// identities atomically. Candidate rows belong to exactly one analysis snapshot.
type DiagnosisAnalysisRepository struct {
	db *gorm.DB
}

func NewDiagnosisAnalysisRepository(db *gorm.DB) *DiagnosisAnalysisRepository {
	return &DiagnosisAnalysisRepository{db: db}
}

func (r *DiagnosisAnalysisRepository) Create(
	ctx context.Context,
	analysis *model.DiagnosisAnalysisRecord,
	candidates []model.DiagnosisCandidateRecord,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(analysis).Error; err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}
		return tx.Create(&candidates).Error
	})
}

func (r *DiagnosisAnalysisRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.DiagnosisAnalysisRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var analyses []model.DiagnosisAnalysisRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&analyses).Error; err != nil {
		return nil, err
	}
	for i := range analyses {
		if err := r.db.WithContext(ctx).
			Where("analysis_id = ?", analyses[i].ID).
			Order("ordinal ASC").
			Find(&analyses[i].Candidates).Error; err != nil {
			return nil, err
		}
	}
	return analyses, nil
}

func (r *DiagnosisAnalysisRepository) UpsertAssessment(ctx context.Context, assessment *model.DiagnosisCandidateAssessment) error {
	return r.db.WithContext(ctx).
		Where("candidate_id = ? AND user_id = ?", assessment.CandidateID, assessment.UserID).
		Assign(map[string]any{"analysis_id": assessment.AnalysisID, "state": assessment.State, "assessed_at": assessment.AssessedAt}).
		FirstOrCreate(assessment).Error
}

func (r *DiagnosisAnalysisRepository) ListAssessments(ctx context.Context, analysisID, userID uuid.UUID) ([]model.DiagnosisCandidateAssessment, error) {
	var items []model.DiagnosisCandidateAssessment
	err := r.db.WithContext(ctx).
		Where("analysis_id = ? AND user_id = ?", analysisID, userID).
		Order("assessed_at ASC").
		Find(&items).Error
	return items, err
}

func (r *DiagnosisAnalysisRepository) GetLatestByUser(ctx context.Context, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	var analysis model.DiagnosisAnalysisRecord
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&analysis).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).
		Where("analysis_id = ?", analysis.ID).
		Order("ordinal ASC").
		Find(&analysis.Candidates).Error; err != nil {
		return nil, err
	}
	return &analysis, nil
}

func (r *DiagnosisAnalysisRepository) GetByID(ctx context.Context, analysisID, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	var analysis model.DiagnosisAnalysisRecord
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", analysisID, userID).
		First(&analysis).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).
		Where("analysis_id = ?", analysis.ID).
		Order("ordinal ASC").
		Find(&analysis.Candidates).Error; err != nil {
		return nil, err
	}
	return &analysis, nil
}
