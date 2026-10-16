package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeAgentEquipment struct {
	assets []model.EquipmentAsset
	err    error
}

func (f fakeAgentEquipment) ListUsable(context.Context, uuid.UUID, time.Time) ([]model.EquipmentAsset, error) {
	return f.assets, f.err
}

type countingSnapshotRunner struct{ calls int }

func (r *countingSnapshotRunner) WithinReadSnapshot(ctx context.Context, fn func(context.Context) error) error {
	r.calls++
	return fn(ctx)
}

func agentTestFact(owner uuid.UUID, kind, statement string, revision int64, details string, regions string) model.BodyFact {
	return model.BodyFact{
		ID: uuid.New(), UserID: owner, Subject: "self", Kind: kind, Statement: statement,
		BodyRegionIDs: datatypes.JSON(regions), Temporal: datatypes.JSON(`{"precision":"unknown"}`),
		Details: datatypes.JSON(details), AssertionStatus: "asserted", CreatedRevision: revision,
		RecordedAt: time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC),
	}
}

func agentTestService(owner uuid.UUID, revision int64, facts []model.BodyFact) (*BodyStateService, *fakeBodyStateRepository) {
	repo := &fakeBodyStateRepository{current: &model.BodyState{UserID: owner, CurrentRevision: revision}}
	repo.revision = revision
	repo.facts = facts // oldest first; ListFacts returns newest first
	return NewBodyStateService(repo), repo
}

func TestAgentBodyContextKeepsUnscreenedSymptomVisible(t *testing.T) {
	owner := uuid.New()
	knee := agentTestFact(owner, "symptom", "右膝下楼梯痛", 3, `{"presence":"present"}`, `["knee_right"]`)
	svc, _ := agentTestService(owner, 3, []model.BodyFact{knee})
	runner := &countingSnapshotRunner{}
	svc.WithAgentContextSources(runner, nil)

	view, err := svc.GetAgentBodyContext(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("view must be read inside one read snapshot, calls=%d", runner.calls)
	}
	if view.Schema != AgentBodyContextSchemaV1 || view.CurrentRevision != 3 || !view.RecordsAvailable || len(view.Facts) != 1 {
		t.Fatalf("view: %#v", view)
	}
	fact := view.Facts[0]
	if fact.ID != knee.ID || fact.SourceEvidenceID != knee.SourceEvidenceID || fact.Authorship != AgentAuthorshipUserReported || fact.SafetyScreen != AgentSymptomUnscreened || fact.ConcernKey != "region:knee_right" {
		t.Fatalf("symptom fact: %#v", fact)
	}
	if view.Safety.Status != AgentSafetyScreeningIncomplete || view.Safety.CoverageComplete || view.Safety.RequiresReview ||
		len(view.Safety.UnscreenedFactIDs) != 1 || view.Safety.UnscreenedFactIDs[0] != knee.ID {
		t.Fatalf("unscreened symptom must keep Safety incomplete: %#v", view.Safety)
	}
}

func TestAgentBodyContextSafetyScreenStates(t *testing.T) {
	owner := uuid.New()
	knee := agentTestFact(owner, "symptom", "膝痛", 2, `{"presence":"present"}`, `[]`)
	absent := agentTestFact(owner, "symptom", "没有麻木", 2, `{"presence":"absent"}`, `[]`)
	for _, test := range []struct {
		name     string
		positive bool
		status   string
		blockers int
	}{
		{"screened clear", false, AgentSafetyScreened, 0},
		{"positive screen", true, AgentSafetyRequiresReview, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc, repo := agentTestService(owner, 4, []model.BodyFact{knee, absent})
			repo.memoryBodyContextLedger.screens = []model.SafetyScreen{{
				ID: uuid.New(), UserID: owner, BodyFactID: knee.ID, Numbness: test.positive, BodyStateRevision: 3, RecordedAt: time.Now().UTC(),
			}}
			if test.positive {
				screen := repo.memoryBodyContextLedger.screens[0]
				repo.restrictions = []model.SafetyRestrictionRecord{{SafetyRestriction: model.SafetyRestriction{ID: uuid.New(), UserID: owner, SafetyScreenID: screen.ID, Concept: "numbness", PolicyRevision: model.SafetyScreenRestrictionPolicyV1, RiskLevel: "restricted", ScopeKind: "all_training", LiftPolicy: "professional_clearance", CreatedRevision: 3, ValidFrom: screen.RecordedAt}, Active: true}}
			}
			view, err := svc.GetAgentBodyContext(context.Background(), owner)
			if err != nil {
				t.Fatal(err)
			}
			states := map[uuid.UUID]string{}
			for _, fact := range view.Facts {
				states[fact.ID] = fact.SafetyScreen
			}
			if states[knee.ID] != AgentSymptomScreened || states[absent.ID] != AgentSymptomNotRequired {
				t.Fatalf("per-symptom screen states: %#v", states)
			}
			if view.Safety.Status != test.status || len(view.Safety.ActiveBlockers) != test.blockers || view.Safety.RequiresReview != test.positive ||
				!view.Safety.CoverageComplete || len(view.Safety.UnscreenedFactIDs) != 0 {
				t.Fatalf("safety: %#v", view.Safety)
			}
		})
	}
}

func TestAgentBodyContextMissingNativeRestrictionFailsClosed(t *testing.T) {
	owner := uuid.New()
	knee := agentTestFact(owner, "symptom", "膝痛", 2, `{"presence":"present"}`, `[]`)
	svc, repo := agentTestService(owner, 3, []model.BodyFact{knee})
	repo.memoryBodyContextLedger.screens = []model.SafetyScreen{{ID: uuid.New(), UserID: owner, BodyFactID: knee.ID, Numbness: true, BodyStateRevision: 3, RecordedAt: time.Now().UTC()}}
	view, err := svc.GetAgentBodyContext(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if view.Safety.Status != AgentSafetyUnavailable || !view.Safety.RequiresReview || view.Safety.CoverageComplete ||
		len(view.Safety.UnscreenedFactIDs) != 1 || view.Facts[0].SafetyScreen != AgentSymptomUnscreened {
		t.Fatalf("an unreadable safety record must fail closed: %#v %#v", view.Safety, view.Facts)
	}
}

func TestAgentBodyContextSectionsAndLabels(t *testing.T) {
	owner := uuid.New()
	oldSleep := agentTestFact(owner, "lifestyle", "以前睡 6 小时", 1, `{"habit_type":"sleep"}`, `[]`)
	newSleep := agentTestFact(owner, "lifestyle", "现在睡 8 小时", 2, `{"habit_type":"sleep"}`, `[]`)
	injury := agentTestFact(owner, "health_history", "去年扭伤脚踝", 2, `{"history_kind":"injury_summary"}`, `[]`)
	svc, repo := agentTestService(owner, 5, []model.BodyFact{oldSleep, newSleep, injury})
	repo.memoryBodyContextLedger.observations = []model.BodyObservationRecord{
		{ID: uuid.New(), UserID: owner, Kind: "weight", Statement: "体重 70 kg", RawValue: "70", RawUnit: "kg", Source: model.BodyObservationSource{ID: uuid.New(), Version: 1}, AssertionStatus: "asserted", CreatedRevision: 3},
	}
	episodeFact := uuid.New()
	repo.memoryBodyContextLedger.episodes = []model.EpisodeRecord{
		{ID: uuid.New(), UserID: owner, SymptomType: "pain", BodyRegionIDs: datatypes.JSON(`["knee_right"]`), IdentityStatus: "current",
			Links: []model.EpisodeFactReference{{FactID: episodeFact}}, Projection: model.EpisodeStateProjection{State: "ongoing", Reason: "explicit"}},
		{ID: uuid.New(), UserID: owner, SymptomType: "pain", IdentityStatus: "invalidated"},
	}
	repo.memoryBodyContextLedger.hypotheses = []model.BodyHypothesisRecord{
		{ID: uuid.New(), UserID: owner, Statement: "我觉得是跑步太多", Source: model.BodyHypothesisSource{Actor: "user"}, AssertionStatus: "asserted", CreatedRevision: 4},
		{ID: uuid.New(), UserID: owner, Statement: "not a user guess", Source: model.BodyHypothesisSource{Actor: "agent"}, AssertionStatus: "asserted", CreatedRevision: 4},
	}
	notes := "中等阻力"
	svc.WithAgentContextSources(nil, fakeAgentEquipment{assets: []model.EquipmentAsset{
		{ID: uuid.New(), UserID: owner, DisplayName: "弹力带", AccessKind: "owned", Notes: &notes},
	}})

	view, err := svc.GetAgentBodyContext(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	statements := map[string]bool{}
	for _, fact := range view.Facts {
		statements[fact.Statement] = true
		if fact.Authorship != AgentAuthorshipUserReported || fact.SafetyScreen != "" {
			t.Fatalf("non-symptom fact: %#v", fact)
		}
	}
	if len(view.Facts) != 2 || !statements["现在睡 8 小时"] || !statements["去年扭伤脚踝"] || statements["以前睡 6 小时"] {
		t.Fatalf("keyed context must keep only the newest statement: %#v", view.Facts)
	}
	if len(view.Observations) != 1 || view.Observations[0].RawValue != "70" || view.Observations[0].Source != repo.memoryBodyContextLedger.observations[0].Source || view.Observations[0].Authorship != AgentAuthorshipUserReported {
		t.Fatalf("observations: %#v", view.Observations)
	}
	if len(view.Episodes) != 1 || view.Episodes[0].State != "ongoing" || len(view.Episodes[0].FactIDs) != 1 || view.Episodes[0].FactIDs[0] != episodeFact {
		t.Fatalf("only current episodes: %#v", view.Episodes)
	}
	if len(view.UserHypotheses) != 1 || view.UserHypotheses[0].Authorship != AgentAuthorshipUserAuthored || view.UserHypotheses[0].Statement != "我觉得是跑步太多" {
		t.Fatalf("only user-authored hypotheses, labelled: %#v", view.UserHypotheses)
	}
	if len(view.Equipment) != 1 || view.Equipment[0].DisplayName != "弹力带" {
		t.Fatalf("owner equipment only: %#v", view.Equipment)
	}
	if view.Safety.Status != AgentSafetyNoCurrentSymptoms {
		t.Fatalf("no symptoms: %#v", view.Safety)
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"safety_state", "foreign bar", "not a user guess", "以前睡 6 小时"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("agent payload leaked %q: %s", forbidden, raw)
		}
	}
}

func TestAgentBodyContextRejectsUnpinnedOrFailedReads(t *testing.T) {
	owner := uuid.New()
	svc, repo := agentTestService(owner, 3, nil)
	repo.revision = 4 // ledger reads disagree with the head revision
	if _, err := svc.GetAgentBodyContext(context.Background(), owner); !errors.Is(err, errAgentBodyContextNotPinned) {
		t.Fatalf("unpinned read must fail: %v", err)
	}
	svc, _ = agentTestService(owner, 3, nil)
	svc.WithAgentContextSources(nil, fakeAgentEquipment{err: errors.New("equipment down")})
	if _, err := svc.GetAgentBodyContext(context.Background(), owner); err == nil {
		t.Fatal("equipment read failure must fail the view")
	}
	if _, err := svc.GetAgentBodyContext(context.Background(), uuid.Nil); err == nil {
		t.Fatal("nil owner must be rejected")
	}
}

func TestAgentBodyContextEmptyOwner(t *testing.T) {
	owner := uuid.New()
	svc := NewBodyStateService(&fakeBodyStateRepository{})
	view, err := svc.GetAgentBodyContext(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if view.CurrentRevision != 0 || !view.RecordsAvailable || view.Safety.Status != AgentSafetyNoCurrentSymptoms || view.Safety.CoverageComplete ||
		view.Facts == nil || view.Equipment == nil || view.UserHypotheses == nil {
		t.Fatalf("empty owner view: %#v", view)
	}
}

func TestAgentBodyContextRejectsForeignRows(t *testing.T) {
	owner, foreign := uuid.New(), uuid.New()
	for _, section := range []string{"facts", "observations", "episodes", "hypotheses", "equipment", "screens"} {
		t.Run(section, func(t *testing.T) {
			svc, repo := agentTestService(owner, 3, nil)
			switch section {
			case "facts":
				repo.facts = []model.BodyFact{agentTestFact(foreign, "symptom", "foreign", 1, `{}`, `[]`)}
			case "observations":
				repo.observations = []model.BodyObservationRecord{{ID: uuid.New(), UserID: foreign, Kind: "weight", AssertionStatus: "asserted"}}
			case "episodes":
				repo.episodes = []model.EpisodeRecord{{ID: uuid.New(), UserID: foreign, IdentityStatus: "current"}}
			case "hypotheses":
				repo.hypotheses = []model.BodyHypothesisRecord{{ID: uuid.New(), UserID: foreign, AssertionStatus: "asserted", Source: model.BodyHypothesisSource{Actor: "user"}}}
			case "equipment":
				svc.WithAgentContextSources(nil, fakeAgentEquipment{assets: []model.EquipmentAsset{{ID: uuid.New(), UserID: foreign}}})
			case "screens":
				repo.memoryBodyContextLedger.screens = []model.SafetyScreen{{ID: uuid.New(), UserID: foreign, BodyStateRevision: 1}}
			}
			if _, err := svc.GetAgentBodyContext(context.Background(), owner); err == nil {
				t.Fatal("owner violation must fail the read")
			}
		})
	}
}
