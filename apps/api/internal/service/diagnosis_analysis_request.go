package service

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type diagnosisAnalysisRequestBinder interface {
	BindAnalysisRequest(context.Context, uuid.UUID, uuid.UUID, time.Time) (*model.StatementEvidence, bool, error)
}

// DiagnosisAnalysisRequestService binds the user's own chat message, verbatim,
// as the request statement a formal analysis cites (R-DX-06: the user starts
// the analysis explicitly). It starts nothing and adopts nothing.
type DiagnosisAnalysisRequestService struct {
	repo diagnosisAnalysisRequestBinder
	now  func() time.Time
}

func NewDiagnosisAnalysisRequestService(db *gorm.DB) *DiagnosisAnalysisRequestService {
	return &DiagnosisAnalysisRequestService{repo: repository.NewDiagnosisRepository(db), now: func() time.Time { return time.Now().UTC() }}
}

// Bind returns the request evidence and whether it already existed.
func (s *DiagnosisAnalysisRequestService) Bind(ctx context.Context, owner, messageID uuid.UUID) (*model.StatementEvidence, bool, error) {
	if owner == uuid.Nil || messageID == uuid.Nil {
		return nil, false, model.ErrDiagnosisRequestSourceNotFound
	}
	return s.repo.BindAnalysisRequest(ctx, owner, messageID, s.now())
}
