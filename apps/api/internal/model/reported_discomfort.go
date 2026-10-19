package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var ErrInvalidReportedDiscomfort = errors.New("invalid reported discomfort areas")

const (
	// DiscomfortAreaNone is the explicit "都没有不适" answer. It is only valid alone.
	DiscomfortAreaNone = "none"
	// ReportedDiscomfortSourceOnboarding is the only writer today.
	ReportedDiscomfortSourceOnboarding = "onboarding"
)

// reportedDiscomfortAreaOrder is the Penpot step 05 order (颈部 … 手臂 / 手腕).
// These are coarse onboarding areas, not BodyRegion ontology ids.
var reportedDiscomfortAreaOrder = []string{
	"neck", "shoulder", "upper_back", "lower_back", "hip", "leg", "arm_wrist",
}

// ReportedDiscomfortAreas is the user's own onboarding answer to "目前哪些部位有
// 不适感？": coarse areas, or an explicit "none". It is a self-report kept apart
// from the BodyState ledger: not a symptom record, a safety screen, an episode
// or a diagnosis, and nothing derives clinical meaning from it.
type ReportedDiscomfortAreas struct {
	UserID     uuid.UUID      `gorm:"type:uuid;primaryKey" json:"-"`
	AreaCodes  datatypes.JSON `gorm:"type:jsonb;not null" json:"area_codes"`
	Source     string         `gorm:"not null" json:"source"`
	ReportedAt time.Time      `gorm:"not null" json:"reported_at"`
}

func (ReportedDiscomfortAreas) TableName() string { return "reported_discomfort_areas" }

// NormalizeReportedDiscomfortAreas validates one answer and returns it in the
// canonical step order. At least one code is required (skipping the step means
// sending nothing); codes must be known and unique; "none" stands alone.
func NormalizeReportedDiscomfortAreas(codes []string) ([]string, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("%w: choose at least one area or none", ErrInvalidReportedDiscomfort)
	}
	seen := make(map[string]bool, len(codes))
	for _, code := range codes {
		known := code == DiscomfortAreaNone
		for _, area := range reportedDiscomfortAreaOrder {
			if code == area {
				known = true
				break
			}
		}
		if !known {
			return nil, fmt.Errorf("%w: unknown area %q", ErrInvalidReportedDiscomfort, code)
		}
		if seen[code] {
			return nil, fmt.Errorf("%w: duplicate area %q", ErrInvalidReportedDiscomfort, code)
		}
		seen[code] = true
	}
	if seen[DiscomfortAreaNone] {
		if len(codes) != 1 {
			return nil, fmt.Errorf("%w: none cannot be combined with an area", ErrInvalidReportedDiscomfort)
		}
		return []string{DiscomfortAreaNone}, nil
	}
	out := make([]string, 0, len(codes))
	for _, area := range reportedDiscomfortAreaOrder {
		if seen[area] {
			out = append(out, area)
		}
	}
	return out, nil
}
