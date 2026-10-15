package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// AgentBodyContextSchemaV1 names the native V2 BodyState view that Go hands to
// AI agents (consultation turns, Treatment generation). It replaces the legacy
// BodyStateSnapshot projection for those inputs.
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
// Safety decisions stay in Go (SafetyState feeds Go policy and is not sent).
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
	// SafetyState is the Go-owned safety resolution record consumed by Go
	// Treatment policy only. It is never part of the agent payload.
	SafetyState json.RawMessage `json:"-"`
}

// AgentBodyFact is one currently asserted user statement.
type AgentBodyFact struct {
	ID              uuid.UUID       `json:"id"`
	Kind            string          `json:"kind"`
	Subject         string          `json:"subject"`
	Statement       string          `json:"statement"`
	Authorship      string          `json:"authorship"`
	ConcernKey      string          `json:"concern_key,omitempty"`
	BodyRegionIDs   json.RawMessage `json:"body_region_ids"`
	Temporal        json.RawMessage `json:"temporal"`
	Details         json.RawMessage `json:"details"`
	SafetyScreen    string          `json:"safety_screen,omitempty"`
	CreatedRevision int64           `json:"created_revision"`
	RecordedAt      time.Time       `json:"recorded_at"`
}

// AgentBodyObservation is the newest asserted user-entered observation of a kind.
type AgentBodyObservation struct {
	ID                 uuid.UUID          `json:"id"`
	Kind               string             `json:"kind"`
	Statement          string             `json:"statement"`
	RawValue           string             `json:"raw_value,omitempty"`
	RawUnit            string             `json:"raw_unit,omitempty"`
	Method             string             `json:"method,omitempty"`
	MeasurementContext string             `json:"measurement_context,omitempty"`
	ObservedAt         model.FactTemporal `json:"observed_at"`
	Authorship         string             `json:"authorship"`
	CreatedRevision    int64              `json:"created_revision"`
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
	ID              uuid.UUID   `json:"id"`
	Statement       string      `json:"statement"`
	Authorship      string      `json:"authorship"`
	RelatedFactIDs  []uuid.UUID `json:"related_fact_ids"`
	CreatedRevision int64       `json:"created_revision"`
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
		UserHypotheses: []AgentUserHypothesis{}, Equipment: []AgentEquipmentItem{}, SafetyState: head.SafetyState,
	}
	if len(view.SafetyState) == 0 {
		view.SafetyState = json.RawMessage(`{}`)
	}

	// Facts: every asserted statement, newest first. Keyed context (injury
	// summary, lifestyle sections) keeps only its newest statement. Safety
	// coverage reads every asserted symptom, not only those that fit the cap.
	var symptoms []safetyCoverageSubject
	seenKeys := map[string]bool{}
	allFactsRead := false
	before := int64(0)
	for page := 0; page < agentBodyContextMaxFactPages; page++ {
		facts, err := s.repo.ListFacts(ctx, owner, before)
		if err != nil {
			return nil, fmt.Errorf("list body facts: %w", err)
		}
		if err := pinned(facts.UserID, facts.CurrentRevision); err != nil {
			return nil, err
		}
		for index := range facts.Facts {
			fact := facts.Facts[index]
			if fact.UserID != owner || fact.AssertionStatus != "asserted" {
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
				_ = json.Unmarshal(fact.Details, &details)
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
				Authorship: AgentAuthorshipUserReported, ConcernKey: bodyFactConcernKey(fact),
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

	seenObservationKinds := map[string]bool{}
	before = 0
	for page := 0; page < agentBodyContextMaxFactPages && len(view.Observations) < agentBodyContextMaxObservations; page++ {
		observations, err := s.repo.ListDirectObservations(ctx, owner, before)
		if err != nil {
			return nil, fmt.Errorf("list body observations: %w", err)
		}
		if err := pinned(observations.UserID, observations.CurrentRevision); err != nil {
			return nil, err
		}
		for _, observation := range observations.Observations {
			if observation.UserID != owner || observation.AssertionStatus != "asserted" || seenObservationKinds[observation.Kind] {
				continue
			}
			seenObservationKinds[observation.Kind] = true
			if len(view.Observations) >= agentBodyContextMaxObservations {
				view.Truncated = true
				continue
			}
			view.Observations = append(view.Observations, AgentBodyObservation{
				ID: observation.ID, Kind: observation.Kind, Statement: observation.Statement,
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

	episodes, err := s.repo.ListEpisodes(ctx, owner, 0)
	if err != nil {
		return nil, fmt.Errorf("list symptom episodes: %w", err)
	}
	if err := pinned(episodes.UserID, episodes.CurrentRevision); err != nil {
		return nil, err
	}
	for _, episode := range episodes.Episodes {
		if episode.UserID != owner || episode.IdentityStatus != "current" {
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

	hypotheses, err := s.repo.ListActiveBodyHypotheses(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("list user hypotheses: %w", err)
	}
	if err := pinned(hypotheses.UserID, hypotheses.CurrentRevision); err != nil {
		return nil, err
	}
	for _, hypothesis := range hypotheses.Hypotheses {
		if hypothesis.UserID != owner || hypothesis.AssertionStatus != "asserted" || hypothesis.Source.Actor != "user" {
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
		view.UserHypotheses = append(view.UserHypotheses, AgentUserHypothesis{
			ID: hypothesis.ID, Statement: hypothesis.Statement, Authorship: AgentAuthorshipUserAuthored,
			RelatedFactIDs: related, CreatedRevision: hypothesis.CreatedRevision,
		})
	}

	screens, err := s.repo.ListSafetyScreens(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("list safety screens: %w", err)
	}
	applyAgentSafety(view, safetyProjectionInput{
		UserID: owner, CurrentRevision: revision, SafetyState: view.SafetyState, Symptoms: symptoms, Screens: screens,
	}, allFactsRead)

	if s.equipment != nil {
		assets, err := s.equipment.ListUsable(ctx, owner, now)
		if err != nil {
			return nil, fmt.Errorf("list usable equipment: %w", err)
		}
		for _, asset := range assets {
			if asset.UserID != owner {
				continue
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
	return view, nil
}

// applyAgentSafety projects the Go Safety envelope over the pinned inputs and
// stamps each symptom fact. Any projection failure, or symptoms that could not
// all be read, keeps the view fail-closed (unscreened / incomplete).
func applyAgentSafety(view *AgentBodyContext, input safetyProjectionInput, allSymptomsRead bool) {
	summary := AgentSafetySummary{PolicyRevision: SafetyEnvelopePolicyV1, ActiveBlockers: []string{}, UnscreenedFactIDs: []uuid.UUID{}}
	defer func() { view.Safety = summary }()
	if input.CurrentRevision <= 0 && len(input.Symptoms) == 0 && len(input.Screens) == 0 {
		summary.Status = AgentSafetyNoCurrentSymptoms
		return
	}
	envelope, err := projectSafetyEnvelope(input)
	if err != nil {
		summary.Status = AgentSafetyUnavailable
		summary.RequiresReview = true
		for _, symptom := range input.Symptoms {
			summary.UnscreenedFactIDs = append(summary.UnscreenedFactIDs, symptom.ID)
		}
		sortUUIDs(summary.UnscreenedFactIDs)
		return
	}
	covered := map[uuid.UUID]bool{}
	for _, ref := range envelope.Coverage.CoveredSourceRefs {
		if id, parseErr := uuid.Parse(strings.TrimPrefix(ref, "body-state:fact:")); parseErr == nil {
			covered[id] = true
		}
	}
	for _, ref := range envelope.Coverage.IncompleteSourceRefs {
		if id, parseErr := uuid.Parse(strings.TrimPrefix(ref, "body-state:fact:")); parseErr == nil {
			summary.UnscreenedFactIDs = append(summary.UnscreenedFactIDs, id)
		}
	}
	sortUUIDs(summary.UnscreenedFactIDs)
	for index := range view.Facts {
		if view.Facts[index].SafetyScreen == AgentSymptomUnscreened && covered[view.Facts[index].ID] {
			view.Facts[index].SafetyScreen = AgentSymptomScreened
		}
	}
	concepts := map[string]bool{}
	for _, blocker := range envelope.ActiveBlockers {
		concepts[string(blocker.Concept)] = true
	}
	for concept := range concepts {
		summary.ActiveBlockers = append(summary.ActiveBlockers, concept)
	}
	sort.Strings(summary.ActiveBlockers)
	summary.RequiresReview = envelope.RequiresReview
	summary.CoverageComplete = envelope.Coverage.Complete && allSymptomsRead
	switch {
	case summary.RequiresReview:
		summary.Status = AgentSafetyRequiresReview
	case len(summary.UnscreenedFactIDs) > 0 || (!allSymptomsRead && len(input.Symptoms) > 0):
		summary.Status = AgentSafetyScreeningIncomplete
	case len(input.Symptoms) == 0:
		summary.Status = AgentSafetyNoCurrentSymptoms
	default:
		summary.Status = AgentSafetyScreened
	}
}

func agentJSONOr(raw []byte, fallback string) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(append([]byte(nil), raw...))
}

func sortUUIDs(ids []uuid.UUID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
}
