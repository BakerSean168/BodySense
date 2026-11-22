package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"log/slog"
	"time"
)

// AgentBodyContextSchemaV1 names the native V2 BodyState view that Go hands to
// AI agents (consultation turns, Treatment generation). It replaces the legacy
// snapshot projection for those inputs.
const AgentBodyContextSchemaV1 = "agent-body-context-v1"

const (
	agentBodyContextMaxFactPages    = 50
	agentBodyContextMaxFacts        = 60
	agentBodyContextMaxObservations = 20
	agentBodyContextMaxEpisodes     = 20
	agentBodyContextMaxHypotheses   = 20
	agentBodyContextMaxEquipment    = 30
)

// Authorship labels. Every BodyState statement in the view is the user's own
// report; AI-proposed hypotheses never enter it (they live in Diagnosis audit).
const (
	AgentAuthorshipUserReported = "user_reported"
	AgentAuthorshipUserAuthored = "user_authored"
)

// Per-symptom Safety screening state, decided by the Go Safety envelope.
const (
	AgentSymptomScreened    = "screened"
	AgentSymptomUnscreened  = "unscreened"
	AgentSymptomNotRequired = "not_required" // the user reported the symptom absent
)

// Overall Safety status shown to agents, most severe first.
const (
	AgentSafetyRequiresReview      = "requires_review"
	AgentSafetyScreeningIncomplete = "screening_incomplete"
	AgentSafetyScreened            = "screened"
	AgentSafetyNoCurrentSymptoms   = "no_current_symptoms"
	AgentSafetyUnavailable         = "unavailable"
)

// AgentBodyContext is one owner's BodyState read natively from the V2 ledger
// (body_facts, body_observations, symptom episodes, user body_hypotheses,
// safety screens) plus the user's currently usable equipment, pinned to one
// BodyState revision. It is the frozen agent input; it is never authority:
// Safety decisions stay in Go; the envelope is never sent to the Agent.
type AgentBodyContext struct {
	Schema           string                 `json:"schema"`
	UserID           uuid.UUID              `json:"user_id"`
	CurrentRevision  int64                  `json:"current_revision"`
	RecordsAvailable bool                   `json:"records_available"`
	Facts            []AgentBodyFact        `json:"facts"`
	Observations     []AgentBodyObservation `json:"observations"`
	Episodes         []AgentSymptomEpisode  `json:"episodes"`
	UserHypotheses   []AgentUserHypothesis  `json:"user_hypotheses"`
	Safety           AgentSafetySummary     `json:"safety"`
	Equipment        []AgentEquipmentItem   `json:"equipment"`
	// Truncated reports that a bounded section omitted older items.
	Truncated bool `json:"truncated"`
	// SafetyEnvelope is Go policy input, excluded from Agent serialization.
	SafetyEnvelope *SafetyEnvelopeV2 `json:"-"`
}

// AgentBodyFact is one currently asserted user statement.
type AgentBodyFact struct {
	ID               uuid.UUID       `json:"id"`
	SourceEvidenceID uuid.UUID       `json:"source_evidence_id"`
	Kind             string          `json:"kind"`
	Subject          string          `json:"subject"`
	Statement        string          `json:"statement"`
	Authorship       string          `json:"authorship"`
	ConcernKey       string          `json:"concern_key,omitempty"`
	BodyRegionIDs    json.RawMessage `json:"body_region_ids"`
	Temporal         json.RawMessage `json:"temporal"`
	Details          json.RawMessage `json:"details"`
	SafetyScreen     string          `json:"safety_screen,omitempty"`
	CreatedRevision  int64           `json:"created_revision"`
	RecordedAt       time.Time       `json:"recorded_at"`
}

// AgentBodyObservation is the newest asserted user-entered observation of a kind.
type AgentBodyObservation struct {
	ID                 uuid.UUID                   `json:"id"`
	Source             model.BodyObservationSource `json:"source"`
	Kind               string                      `json:"kind"`
	Statement          string                      `json:"statement"`
	RawValue           string                      `json:"raw_value,omitempty"`
	RawUnit            string                      `json:"raw_unit,omitempty"`
	Method             string                      `json:"method,omitempty"`
	MeasurementContext string                      `json:"measurement_context,omitempty"`
	ObservedAt         model.FactTemporal          `json:"observed_at"`
	Authorship         string                      `json:"authorship"`
	CreatedRevision    int64                       `json:"created_revision"`
}

// AgentSymptomEpisode is a current episode identity grouping symptom facts.
// Its state is the deterministic projection, never an AI judgement.
type AgentSymptomEpisode struct {
	ID              uuid.UUID       `json:"id"`
	SymptomType     string          `json:"symptom_type"`
	BodyRegionIDs   json.RawMessage `json:"body_region_ids"`
	Laterality      string          `json:"laterality"`
	State           string          `json:"state"`
	StateReason     string          `json:"state_reason"`
	FactIDs         []uuid.UUID     `json:"fact_ids"`
	CreatedRevision int64           `json:"created_revision"`
}

// AgentUserHypothesis is the user's own guess about a cause. It is labelled
// user-authored so agents never treat it as an established fact.
type AgentUserHypothesis struct {
	ID             uuid.UUID   `json:"id"`
	Statement      string      `json:"statement"`
	Authorship     string      `json:"authorship"`
	RelatedFactIDs []uuid.UUID `json:"related_fact_ids"`
	// RelatedObservationIDs are the user's own observations they linked.
	RelatedObservationIDs []uuid.UUID `json:"related_observation_ids"`
	CreatedRevision       int64       `json:"created_revision"`
}

// AgentSafetySummary is the agent-facing digest of the Go Safety envelope.
// Fail-closed: any unscreened current symptom keeps coverage incomplete.
type AgentSafetySummary struct {
	Status            string      `json:"status"`
	PolicyRevision    string      `json:"policy_revision"`
	RequiresReview    bool        `json:"requires_review"`
	CoverageComplete  bool        `json:"coverage_complete"`
	ActiveBlockers    []string    `json:"active_blockers"`
	UnscreenedFactIDs []uuid.UUID `json:"unscreened_fact_ids"`
}

// AgentEquipmentItem is an EquipmentAsset usable now (user-stated resource).
type AgentEquipmentItem struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	AccessKind  string    `json:"access_kind"`
	Notes       *string   `json:"notes,omitempty"`
}

// agentBodyContextLedger is the V2 ledger the view reads beyond the facts,
// observations and screens already in bodyContextLedger.
type agentBodyContextLedger interface {
	ListSafetyRestrictions(context.Context, uuid.UUID) (*model.SafetyRestrictionList, error)
	ListEpisodes(ctx context.Context, owner uuid.UUID, beforeRevision int64) (*model.EpisodePage, error)
	ListActiveBodyHypotheses(ctx context.Context, owner uuid.UUID) (*model.BodyHypothesisList, error)
}

type agentEquipmentSource interface {
	ListUsable(ctx context.Context, owner uuid.UUID, now time.Time) ([]model.EquipmentAsset, error)
}

type agentReadSnapshotRunner interface {
	WithinReadSnapshot(ctx context.Context, fn func(context.Context) error) error
}

var errAgentBodyContextNotPinned = errors.New("agent body context read is not pinned to one BodyState revision")

// WithAgentContextSources wires the read snapshot boundary and the equipment
// inventory the agent view joins. Without a snapshot runner each read still
// has to report the same revision or the view fails.
func (s *BodyStateService) WithAgentContextSources(snapshots agentReadSnapshotRunner, equipment agentEquipmentSource) *BodyStateService {
	s.readSnapshots = snapshots
	s.equipment = equipment
	return s
}

// GetAgentBodyContext builds the owner's native V2 agent view from one
// read-only REPEATABLE READ snapshot.
func (s *BodyStateService) GetAgentBodyContext(ctx context.Context, owner uuid.UUID) (*AgentBodyContext, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyFact)
	}
	var view *AgentBodyContext
	load := func(ctx context.Context) error {
		var err error
		view, err = s.loadAgentBodyContext(ctx, owner, time.Now().UTC())
		return err
	}
	var err error
	if s.readSnapshots != nil {
		err = s.readSnapshots.WithinReadSnapshot(ctx, load)
	} else {
		err = load(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("load agent body context: %w", err)
	}
	return view, nil
}

func (s *BodyStateService) loadAgentBodyContext(ctx context.Context, owner uuid.UUID, now time.Time) (*AgentBodyContext, error) {
	head, err := s.repo.GetBodyStateHead(ctx, owner)
	if err != nil {
		return nil, err
	}
	if head == nil || head.UserID != owner {
		return nil, fmt.Errorf("body state head owner mismatch")
	}
	revision := head.CurrentRevision
	pinned := func(pageOwner uuid.UUID, pageRevision int64) error {
		if pageOwner != owner {
			return fmt.Errorf("body state read owner mismatch")
		}
		if pageRevision != revision {
			return fmt.Errorf("%w: head R%d, read R%d", errAgentBodyContextNotPinned, revision, pageRevision)
		}
		return nil
	}
	view := &AgentBodyContext{
		Schema: AgentBodyContextSchemaV1, UserID: owner, CurrentRevision: revision, RecordsAvailable: true,
		Facts: []AgentBodyFact{}, Observations: []AgentBodyObservation{}, Episodes: []AgentSymptomEpisode{},
		UserHypotheses: []AgentUserHypothesis{}, Equipment: []AgentEquipmentItem{},
	}

	symptoms, allFactsRead, err := s.loadAgentBodyFacts(ctx, owner, view, pinned)
	if err != nil {
		return nil, err
	}
	if err := s.loadAgentBodyObservations(ctx, owner, view, pinned); err != nil {
		return nil, err
	}
	if err := s.loadAgentBodyEpisodes(ctx, owner, view, pinned); err != nil {
		return nil, err
	}
	if err := s.loadAgentBodyHypotheses(ctx, owner, view, pinned); err != nil {
		return nil, err
	}
	if err := s.loadAgentSafetyProjection(ctx, owner, view, revision, symptoms, allFactsRead); err != nil {
		return nil, err
	}
	if err := s.loadAgentBodyEquipment(ctx, owner, view, now); err != nil {
		return nil, err
	}
	return view, nil
}

// loadAgentBodyFacts pages every asserted statement, newest first. Keyed
// context (injury summary, lifestyle sections) keeps only its newest
// statement. It also collects the safety coverage subjects: coverage reads
// every asserted symptom, not only those that fit the cap.
func (s *BodyStateService) loadAgentBodyFacts(ctx context.Context, owner uuid.UUID, view *AgentBodyContext, pinned func(uuid.UUID, int64) error) ([]safetyCoverageSubject, bool, error) {
	var symptoms []safetyCoverageSubject
	seenKeys := map[string]bool{}
	allFactsRead := false
	before := int64(0)
	for page := 0; page < agentBodyContextMaxFactPages; page++ {
		facts, err := s.repo.ListFacts(ctx, owner, before)
		if err != nil {
			return nil, false, fmt.Errorf("list body facts: %w", err)
		}
		if err := pinned(facts.UserID, facts.CurrentRevision); err != nil {
			return nil, false, err
		}
		for index := range facts.Facts {
			fact := facts.Facts[index]
			if fact.UserID != owner {
				return nil, false, fmt.Errorf("body fact owner mismatch")
			}
			if fact.AssertionStatus != "asserted" {
				continue
			}
			if key := bodyContextFactKey(fact); key != "" {
				if seenKeys[key] {
					continue
				}
				seenKeys[key] = true
			}
			screen := ""
			if fact.Kind == "symptom" {
				var details model.SymptomFactDetails
				// An undecodable details payload keeps Presence unset, so the
				// fact is treated as unscreened and still enters the safety
				// coverage check — the failure direction is fail-closed, hence
				// tolerate, but leave a log.
				if err := json.Unmarshal(fact.Details, &details); err != nil {
					slog.Default().Warn(
						"symptom fact details are not valid JSON; treating as unscreened",
						"fact_id", fact.ID,
						"error", err,
					)
				}
				if details.Presence == "absent" {
					screen = AgentSymptomNotRequired
				} else {
					screen = AgentSymptomUnscreened
					symptoms = append(symptoms, safetyCoverageSubject{ID: fact.ID, CreatedRevision: fact.CreatedRevision})
				}
			}
			if len(view.Facts) >= agentBodyContextMaxFacts {
				view.Truncated = true
				continue
			}
			view.Facts = append(view.Facts, AgentBodyFact{
				ID: fact.ID, Kind: fact.Kind, Subject: fact.Subject, Statement: fact.Statement,
				SourceEvidenceID: fact.SourceEvidenceID,
				Authorship:       AgentAuthorshipUserReported, ConcernKey: bodyFactConcernKey(fact),
				BodyRegionIDs: agentJSONOr(fact.BodyRegionIDs, `[]`), Temporal: agentJSONOr(fact.Temporal, `{}`),
				Details: agentJSONOr(fact.Details, `{}`), SafetyScreen: screen,
				CreatedRevision: fact.CreatedRevision, RecordedAt: fact.RecordedAt,
			})
		}
		if !facts.HasMore || facts.NextBeforeRevision == nil {
			allFactsRead = true
			break
		}
		before = *facts.NextBeforeRevision
	}
	if !allFactsRead {
		view.Truncated = true
	}
	return symptoms, allFactsRead, nil
}

func (s *BodyStateService) loadAgentBodyObservations(ctx context.Context, owner uuid.UUID, view *AgentBodyContext, pinned func(uuid.UUID, int64) error) error {
	seenObservationKinds := map[string]bool{}
	before := int64(0)
	for page := 0; page < agentBodyContextMaxFactPages && len(view.Observations) < agentBodyContextMaxObservations; page++ {
		observations, err := s.repo.ListDirectObservations(ctx, owner, before)
		if err != nil {
			return fmt.Errorf("list body observations: %w", err)
		}
		if err := pinned(observations.UserID, observations.CurrentRevision); err != nil {
			return err
		}
		for _, observation := range observations.Observations {
			if observation.UserID != owner {
				return fmt.Errorf("body observation owner mismatch")
			}
			if observation.AssertionStatus != "asserted" || seenObservationKinds[observation.Kind] {
				continue
			}
			seenObservationKinds[observation.Kind] = true
			if len(view.Observations) >= agentBodyContextMaxObservations {
				view.Truncated = true
				continue
			}
			view.Observations = append(view.Observations, AgentBodyObservation{
				ID: observation.ID, Kind: observation.Kind, Statement: observation.Statement,
				Source:   observation.Source,
				RawValue: observation.RawValue, RawUnit: observation.RawUnit, Method: observation.Method,
				MeasurementContext: observation.MeasurementContext, ObservedAt: observation.ObservedAt,
				Authorship: AgentAuthorshipUserReported, CreatedRevision: observation.CreatedRevision,
			})
		}
		if !observations.HasMore || observations.NextBeforeRevision == nil {
			break
		}
		before = *observations.NextBeforeRevision
	}
	return nil
}

func (s *BodyStateService) loadAgentBodyEpisodes(ctx context.Context, owner uuid.UUID, view *AgentBodyContext, pinned func(uuid.UUID, int64) error) error {
	episodes, err := s.repo.ListEpisodes(ctx, owner, 0)
	if err != nil {
		return fmt.Errorf("list symptom episodes: %w", err)
	}
	if err := pinned(episodes.UserID, episodes.CurrentRevision); err != nil {
		return err
	}
	for _, episode := range episodes.Episodes {
		if episode.UserID != owner {
			return fmt.Errorf("body episode owner mismatch")
		}
		if episode.IdentityStatus != "current" {
			continue
		}
		if len(view.Episodes) >= agentBodyContextMaxEpisodes {
			view.Truncated = true
			break
		}
		factIDs := make([]uuid.UUID, 0, len(episode.Links))
		for _, link := range episode.Links {
			factIDs = append(factIDs, link.FactID)
		}
		view.Episodes = append(view.Episodes, AgentSymptomEpisode{
			ID: episode.ID, SymptomType: episode.SymptomType, BodyRegionIDs: agentJSONOr(episode.BodyRegionIDs, `[]`),
			Laterality: episode.Laterality, State: episode.Projection.State, StateReason: episode.Projection.Reason,
			FactIDs: factIDs, CreatedRevision: episode.CreatedRevision,
		})
	}
	if episodes.HasMore {
		view.Truncated = true
	}
	return nil
}

func (s *BodyStateService) loadAgentBodyHypotheses(ctx context.Context, owner uuid.UUID, view *AgentBodyContext, pinned func(uuid.UUID, int64) error) error {
	hypotheses, err := s.repo.ListActiveBodyHypotheses(ctx, owner)
	if err != nil {
		return fmt.Errorf("list user hypotheses: %w", err)
	}
	if err := pinned(hypotheses.UserID, hypotheses.CurrentRevision); err != nil {
		return err
	}
	for _, hypothesis := range hypotheses.Hypotheses {
		if hypothesis.UserID != owner {
			return fmt.Errorf("body hypothesis owner mismatch")
		}
		if hypothesis.AssertionStatus != "asserted" || hypothesis.Source.Actor != "user" {
			continue
		}
		if len(view.UserHypotheses) >= agentBodyContextMaxHypotheses {
			view.Truncated = true
			break
		}
		related := hypothesis.RelatedFactIDs
		if related == nil {
			related = []uuid.UUID{}
		}
		observations := hypothesis.RelatedObservationIDs
		if observations == nil {
			observations = []uuid.UUID{}
		}
		view.UserHypotheses = append(view.UserHypotheses, AgentUserHypothesis{
			ID: hypothesis.ID, Statement: hypothesis.Statement, Authorship: AgentAuthorshipUserAuthored,
			RelatedFactIDs: related, RelatedObservationIDs: observations, CreatedRevision: hypothesis.CreatedRevision,
		})
	}
	return nil
}

func (s *BodyStateService) loadAgentSafetyProjection(ctx context.Context, owner uuid.UUID, view *AgentBodyContext, revision int64, symptoms []safetyCoverageSubject, allFactsRead bool) error {
	screens, err := s.repo.ListSafetyScreens(ctx, owner)
	if err != nil {
		return fmt.Errorf("list safety screens: %w", err)
	}
	for _, screen := range screens {
		if screen.UserID != owner {
			return fmt.Errorf("safety screen owner mismatch")
		}
	}
	restrictions, err := s.repo.ListSafetyRestrictions(ctx, owner)
	if err != nil {
		return fmt.Errorf("read native safety: %w", err)
	}
	if restrictions == nil || restrictions.UserID != owner || restrictions.CurrentRevision != revision {
		return errAgentBodyContextNotPinned
	}
	applyAgentSafety(view, safetyProjectionInput{
		UserID: owner, CurrentRevision: revision, Restrictions: restrictions.Restrictions, Symptoms: symptoms, Screens: screens,
	}, allFactsRead)
	return nil
}

func (s *BodyStateService) loadAgentBodyEquipment(ctx context.Context, owner uuid.UUID, view *AgentBodyContext, now time.Time) error {
	if s.equipment != nil {
		assets, err := s.equipment.ListUsable(ctx, owner, now)
		if err != nil {
			return fmt.Errorf("list usable equipment: %w", err)
		}
		for _, asset := range assets {
			if asset.UserID != owner {
				return fmt.Errorf("equipment owner mismatch")
			}
			if len(view.Equipment) >= agentBodyContextMaxEquipment {
				view.Truncated = true
				break
			}
			view.Equipment = append(view.Equipment, AgentEquipmentItem{
				ID: asset.ID, DisplayName: asset.DisplayName, AccessKind: asset.AccessKind, Notes: asset.Notes,
			})
		}
	}
	return nil
}
