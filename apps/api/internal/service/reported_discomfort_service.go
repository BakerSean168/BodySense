package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type reportedDiscomfortStore interface {
	Replace(ctx context.Context, userID uuid.UUID, codes []string, reportedAt time.Time) error
	Get(ctx context.Context, userID uuid.UUID) (*model.ReportedDiscomfortAreas, error)
}

// ReportedDiscomfortSnapshot is the public read of the onboarding answer. Report
// is nil until the user answers step 05.
type ReportedDiscomfortSnapshot struct {
	Report *ReportedDiscomfortReport `json:"report"`
}

type ReportedDiscomfortReport struct {
	AreaCodes  []string  `json:"area_codes"`
	Source     string    `json:"source"`
	ReportedAt time.Time `json:"reported_at"`
}

// ReportedDiscomfortService reads the onboarding discomfort self-report. It is
// deliberately not part of BodyContext: Agent, safety and diagnosis inputs do
// not read it.
type ReportedDiscomfortService struct {
	store reportedDiscomfortStore
}

func NewReportedDiscomfortService(store reportedDiscomfortStore) *ReportedDiscomfortService {
	return &ReportedDiscomfortService{store: store}
}

func (s *ReportedDiscomfortService) Get(ctx context.Context, userID uuid.UUID) (*ReportedDiscomfortSnapshot, error) {
	row, err := s.store.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load reported discomfort areas: %w", err)
	}
	if row == nil {
		return &ReportedDiscomfortSnapshot{}, nil
	}
	var codes []string
	if err := json.Unmarshal(row.AreaCodes, &codes); err != nil {
		return nil, fmt.Errorf("decode reported discomfort areas: %w", err)
	}
	return &ReportedDiscomfortSnapshot{Report: &ReportedDiscomfortReport{
		AreaCodes: codes, Source: row.Source, ReportedAt: row.ReportedAt.UTC(),
	}}, nil
}
