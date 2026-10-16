package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// bodyStateRepository is deliberately consumer-owned: the application service
// declares only the persistence capabilities required by BodyState workflows.
type bodyStateRepository interface {
	bodyContextLedger
	agentBodyContextLedger
	bodyFactLookup
	GetBodyStateHead(ctx context.Context, userID uuid.UUID) (*model.BodyStateHead, error)
	ListRecentRevisions(context.Context, uuid.UUID, int) ([]model.BodyStateRevision, error)
	ListRevisionsAfter(context.Context, uuid.UUID, int64, int) ([]model.BodyStateRevision, error)
}

// BodyStateService is the application boundary around the user-scoped aggregate.
// Producers (chat, workbench, posture analysis) call this service; they never own
// BodyState persistence themselves.
type BodyStateService struct {
	repo                  bodyStateRepository
	bodyRegionIDValidator BodyRegionIDValidator
	readSnapshots         agentReadSnapshotRunner
	equipment             agentEquipmentSource
}

func NewBodyStateService(repo bodyStateRepository) *BodyStateService {
	return &BodyStateService{repo: repo}
}

// WithBodyRegionIDValidator wires the canonical ontology authority without
// moving ontology ownership into the durable lane. Anatomically localized writes
// fail closed unless they resolve to a canonical region identity.
func (s *BodyStateService) WithBodyRegionIDValidator(validator BodyRegionIDValidator) *BodyStateService {
	s.bodyRegionIDValidator = validator
	return s
}

// CurrentBodyContext returns the user's reported metrics, injury history and
// lifestyle from the V2 ledger.
func (s *BodyStateService) CurrentBodyContext(ctx context.Context, userID uuid.UUID) (*BodyContext, error) {
	return loadBodyContext(ctx, s.repo, userID)
}

// UpdateBodyContext appends the user's edited context statements to the V2
// ledger and returns the resulting BodyState revision.
func (s *BodyStateService) UpdateBodyContext(ctx context.Context, userID uuid.UUID, expectedRevision *int64, update BodyContextUpdate) (int64, error) {
	return applyBodyContextUpdate(ctx, s.repo, userID, expectedRevision, update, time.Now())
}

func (s *BodyStateService) ListRevisionsAfter(ctx context.Context, userID uuid.UUID, afterRevision int64, limit int) ([]model.BodyStateRevision, error) {
	return s.repo.ListRevisionsAfter(ctx, userID, afterRevision, limit)
}

// RecordOutcomeSymptom is the only BodyState effect of an intervention
// Outcome. A symptom_change/new_discomfort Outcome whose value carries the
// user's own description becomes one V2 symptom BodyFact (direct_edit source,
// statement = the user's exact words). Every other Outcome (adherence,
// training feeling, measurements, system-authored text) stays only in the
// outcomes table and returns nil: no legacy fact/observation is written.
//
// The fact is deliberately left unscreened: Safety coverage for it is
// incomplete (fail-closed) until a later safety_screens row covers its
// revision. Replays are idempotent through the outcome-derived command key.
func (s *BodyStateService) RecordOutcomeSymptom(ctx context.Context, userID uuid.UUID, outcome model.Outcome) (*model.BodyStateRevision, error) {
	statement, ok := outcomeUserSymptomStatement(outcome)
	if !ok {
		return nil, nil
	}
	if userID == uuid.Nil || outcome.ID == uuid.Nil {
		return nil, fmt.Errorf("%w: outcome owner and identity required", model.ErrInvalidBodyFact)
	}
	regions := []string{}
	if id, resolved := ResolveCanonicalBodyRegionID(strings.TrimSpace(outcome.BodyRegion)); resolved {
		if s.bodyRegionIDValidator == nil || s.bodyRegionIDValidator.IsValidBodyRegionID(id) {
			regions = append(regions, id)
		}
	}
	expected := int64(0)
	state, err := s.repo.GetBodyStateHead(ctx, userID)
	if err != nil {
		return nil, err
	}
	if state != nil {
		expected = state.CurrentRevision
	}
	commit, err := s.repo.RecordDirectFacts(ctx, userID, model.RecordBodyFactsCommand{
		IdempotencyKey:   "outcome:" + outcome.ID.String(),
		ExpectedRevision: expected,
		Facts: []model.BodyFactInput{{
			Kind: "symptom", Statement: statement, BodyRegionIDs: regions,
			Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{}`),
		}},
	})
	if err != nil {
		return nil, err
	}
	if commit == nil || commit.Revision <= 0 {
		return nil, errors.New("outcome symptom fact commit has no revision")
	}
	return &model.BodyStateRevision{ID: commit.RevisionID, UserID: userID, Revision: commit.Revision}, nil
}

// outcomeUserSymptomStatement returns the user's own symptom wording carried
// by an Outcome. Notes, defaults and other free text are never promoted.
func outcomeUserSymptomStatement(outcome model.Outcome) (string, bool) {
	if outcome.Kind != "symptom_change" && outcome.Kind != "new_discomfort" {
		return "", false
	}
	var value map[string]any
	if len(outcome.Value) == 0 || json.Unmarshal(outcome.Value, &value) != nil {
		return "", false
	}
	for _, key := range []string{"description", "symptom"} {
		if text, isString := value[key].(string); isString && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text), true
		}
	}
	return "", false
}

func bodyStateHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}
