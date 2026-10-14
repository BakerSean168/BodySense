package model

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Episode identity is a grouping of user-authored evidence, never a diagnosis.
// State is deliberately not persisted in symptom_episodes: it is replayed from
// explicit status evidence and the current validity of each underlying source.
type SymptomEpisode struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	SymptomType     string         `gorm:"not null" json:"symptom_type"`
	BodyRegionIDs   datatypes.JSON `gorm:"type:jsonb;not null" json:"body_region_ids"`
	Laterality      string         `gorm:"not null" json:"laterality"`
	IdentityStatus  string         `gorm:"not null" json:"identity_status"`
	CreatedRevision int64          `gorm:"not null" json:"created_revision"`
	CreatedAt       time.Time      `gorm:"not null" json:"created_at"`
}

func (SymptomEpisode) TableName() string { return "symptom_episodes" }

type EpisodeFactLink struct {
	UserID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	EpisodeID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	FactID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	FactKind          string
	BasisEvidenceID   uuid.UUID `gorm:"type:uuid;not null"`
	Role              string
	AssociationMethod string
	IsPrimary         bool
	AuthorizationID   *uuid.UUID `gorm:"type:uuid"`
	CreatedRevision   int64
}

func (EpisodeFactLink) TableName() string { return "episode_fact_links" }

type EpisodeStateEvidence struct {
	UserID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	EpisodeID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	FactID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	SourceEvidenceID uuid.UUID `gorm:"type:uuid;not null"`
	Claim            string
	AsOf             datatypes.JSON `gorm:"type:jsonb;not null"`
	CreatedRevision  int64
}

func (EpisodeStateEvidence) TableName() string { return "episode_state_evidence" }

// EpisodeStateBasis is a validated, resolved join between a status claim,
// the assertion validity and its minimum source availability. It cannot be
// constructed from a generic absent/present fact without an explicit claim.
type EpisodeStateBasis struct {
	FactID             uuid.UUID
	Claim              string // explicit_current_report | explicit_resolution
	AssertionStatus    string // asserted | superseded | withdrawn
	SourceAvailability string // available | blocked | erased
	AsOf               FactTemporal
	CreatedRevision    int64
}

type EpisodeStateProjection struct {
	State           string      `json:"state"` // unknown | ongoing | ended
	StateAsOf       string      `json:"state_as_of,omitempty"`
	Reason          string      `json:"reason"`
	BasisFactIDs    []uuid.UUID `json:"basis_fact_ids"`
	BasedOnRevision int64       `json:"based_on_revision"`
}

var ErrInvalidEpisodeEvidence = errors.New("invalid episode evidence")

// ProjectExplicitEpisodeState deterministically projects an accepted source
// set at a specific BodyState revision. It never uses wall-clock inactivity,
// a bare negative fact, or the record's system timestamp as resolution.
func ProjectExplicitEpisodeState(identityStatus string, revision int64, input []EpisodeStateBasis) (EpisodeStateProjection, error) {
	projection := EpisodeStateProjection{
		State: "unknown", Reason: "insufficient_evidence", BasisFactIDs: []uuid.UUID{}, BasedOnRevision: revision,
	}
	if revision < 0 || (identityStatus != "current" && identityStatus != "invalidated" && identityStatus != "superseded") {
		return projection, ErrInvalidEpisodeEvidence
	}
	if identityStatus != "current" {
		return projection, nil
	}

	active := make([]EpisodeStateBasis, 0, len(input))
	seen := make(map[uuid.UUID]bool, len(input))
	for _, evidence := range input {
		if evidence.FactID == uuid.Nil || evidence.CreatedRevision <= 0 ||
			evidence.AssertionStatus == "" || evidence.SourceAvailability == "" ||
			(evidence.Claim != "explicit_current_report" && evidence.Claim != "explicit_resolution") ||
			evidence.AsOf.Validate() != nil || seen[evidence.FactID] {
			return projection, ErrInvalidEpisodeEvidence
		}
		seen[evidence.FactID] = true
		if evidence.CreatedRevision > revision || evidence.AssertionStatus != "asserted" ||
			evidence.SourceAvailability != "available" {
			continue
		}
		active = append(active, evidence)
	}
	if len(active) == 0 {
		return projection, nil
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].CreatedRevision == active[j].CreatedRevision {
			return active[i].FactID.String() < active[j].FactID.String()
		}
		return active[i].CreatedRevision < active[j].CreatedRevision
	})
	for _, b := range active {
		projection.BasisFactIDs = append(projection.BasisFactIDs, b.FactID)
	}
	current := make([]EpisodeStateBasis, 0, len(active))
	resolved := make([]EpisodeStateBasis, 0, len(active))
	for _, b := range active {
		if b.Claim == "explicit_current_report" {
			current = append(current, b)
		} else {
			resolved = append(resolved, b)
		}
	}
	// With only one explicit type of current-state evidence, a status is known
	// as of the report, regardless of whether a normalized occurrence date
	// could be extracted. No onset or resolution date is invented.
	if len(resolved) == 0 {
		projection.State = "ongoing"
		projection.Reason = "explicit_current_report"
		projection.StateAsOf = lastExplicitAsOf(active)
		return projection, nil
	}
	if len(current) == 0 {
		projection.State = "ended"
		projection.Reason = "explicit_resolution"
		projection.StateAsOf = lastExplicitAsOf(active)
		return projection, nil
	}

	// A resolution may supersede earlier current reports if source-backed
	// business times are strictly later. A current report after an explicit
	// resolution requires a distinct recurrence Episode, not reopening this one.
	for _, open := range current {
		for _, closed := range resolved {
			order, comparable := compareEvidenceTimes(open.AsOf, closed.AsOf)
			if !comparable || order >= 0 {
				projection.Reason = "conflicting_evidence"
				return projection, nil
			}
		}
	}
	projection.State = "ended"
	projection.Reason = "explicit_resolution"
	projection.StateAsOf = lastExplicitAsOf(resolved)
	return projection, nil
}

func lastExplicitAsOf(input []EpisodeStateBasis) string {
	var latest *FactTemporal
	for _, evidence := range input {
		candidate := evidence.AsOf
		if _, _, valid := episodeTimeBounds(candidate); !valid {
			return ""
		}
		if latest == nil {
			latest = &candidate
			continue
		}
		if candidate.Precision == latest.Precision && candidate.Start == latest.Start {
			continue
		}
		order, comparable := compareEvidenceTimes(*latest, candidate)
		if !comparable {
			return ""
		}
		if order < 0 {
			latest = &candidate
		}
	}
	if latest == nil {
		return ""
	}
	return latest.Start
}

// CompareEpisodeEvidenceTime reports A relative to B only when trusted,
// sufficiently precise business times establish strict chronology. A
// non-comparable overlap or unknown time is never treated as "later".
func CompareEpisodeEvidenceTime(a, b FactTemporal) (int, bool) {
	return compareEvidenceTimes(a, b)
}

// Return order of A relative to B only when the underlying explicit times
// prove separation. Unknown/approximate/range or overlapping day windows do
// not establish a medical course by guessing chronology.
func compareEvidenceTimes(a, b FactTemporal) (int, bool) {
	aStart, aEnd, aOK := episodeTimeBounds(a)
	bStart, bEnd, bOK := episodeTimeBounds(b)
	if !aOK || !bOK {
		return 0, false
	}
	if aEnd.Before(bStart) || (aEnd.Equal(bStart) && aStart.Before(aEnd)) {
		return -1, true
	}
	if bEnd.Before(aStart) || (bEnd.Equal(aStart) && bStart.Before(bEnd)) {
		return 1, true
	}
	return 0, false
}

func episodeTimeBounds(value FactTemporal) (time.Time, time.Time, bool) {
	if value.Start == "" || value.End != "" {
		return time.Time{}, time.Time{}, false
	}
	switch value.Precision {
	case "day":
		day, err := time.Parse("2006-01-02", value.Start)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return day, day.AddDate(0, 0, 1), true
	case "exact":
		exact, err := time.Parse(time.RFC3339, value.Start)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return exact, exact, true
	default:
		return time.Time{}, time.Time{}, false
	}
}
