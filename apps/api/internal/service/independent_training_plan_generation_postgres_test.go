package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/google/uuid"
)

func TestIndependentTrainingAgentGenerationProposalPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	if _, err := svc.Propose(ctx, owner, PlanProposalInput{Draft: independentDraft("warm"), InitiatedBy: "user", SourceID: "user_api:warm", IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	var bodyRevision int64
	if err := db.Raw(`SELECT current_revision FROM body_states WHERE user_id = ?`, owner).Scan(&bodyRevision).Error; err != nil || bodyRevision < 0 {
		t.Fatalf("body revision %d %v", bodyRevision, err)
	}
	command := planDigest("generate-command-1")
	generation := func(cmd string, revision int64) *PlanGeneration {
		return &PlanGeneration{CommandFingerprint: cmd, FrozenBodyStateRevision: revision,
			Provenance:  json.RawMessage(`{"schemaRevision":"training-plan-generation-provenance-v1"}`),
			ReplayInput: json.RawMessage(`{"body_state_revision":1}`)}
	}
	agentInput := func(key, title string, g *PlanGeneration) PlanProposalInput {
		return PlanProposalInput{Draft: independentDraft(title), InitiatedBy: trainingplan.InitiatorAgent, SourceID: "treatment_agent:test", IdempotencyKey: key, Generation: g}
	}
	countPlans := func() int64 {
		var n int64
		if err := db.Raw(`SELECT count(*) FROM training_plans WHERE user_id = ?`, owner).Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	plansBefore := countPlans()

	key := "gen-" + uuid.NewString()
	first, err := svc.Propose(ctx, owner, agentInput(key, "Agent plan", generation(command, bodyRevision)))
	if err != nil {
		t.Fatal(err)
	}
	if first.InitiatedBy != trainingplan.InitiatorAgent || first.State != trainingplan.ProposalPending || first.Kind != trainingplan.ProposalCreate ||
		!planJSONObject(first.GenerationProvenance) || !planJSONObject(first.ReplayInput) {
		t.Fatalf("agent proposal: %+v", first)
	}
	if countPlans() != plansBefore {
		t.Fatal("generation must not create or modify a plan")
	}
	stored, err := svc.ProposalByIdempotencyKey(ctx, owner, key)
	if err != nil || stored == nil || stored.ID != first.ID || !planJSONObject(stored.ReplayInput) {
		t.Fatalf("stored proposal: %+v %v", stored, err)
	}

	// The same command replays the stored proposal even if a new Agent output differs.
	replayed, err := svc.Propose(ctx, owner, agentInput(key, "Different agent output", generation(command, bodyRevision)))
	if err != nil || replayed.ID != first.ID {
		t.Fatalf("replay: %+v %v", replayed, err)
	}
	if _, err := svc.Propose(ctx, owner, agentInput(key, "Agent plan", generation(planDigest("other-command"), bodyRevision))); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("different command under same key: %v", err)
	}
	userInput := agentInput(key, "Agent plan", nil)
	userInput.InitiatedBy = trainingplan.InitiatorUser
	if _, err := svc.Propose(ctx, owner, userInput); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("user command under a generation key: %v", err)
	}

	// Stale frozen BodyState: nothing is written.
	staleKey := "gen-stale-" + uuid.NewString()
	if _, err := svc.Propose(ctx, owner, agentInput(staleKey, "Stale", generation(planDigest("stale"), bodyRevision+1))); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("stale body revision: %v", err)
	}
	if p, err := svc.ProposalByIdempotencyKey(ctx, owner, staleKey); err != nil || p != nil {
		t.Fatalf("stale generation wrote %+v %v", p, err)
	}

	// Only an agent create may carry a generation audit.
	invalid := []PlanProposalInput{
		func() PlanProposalInput {
			in := agentInput(uuid.NewString(), "x", generation(planDigest("u"), bodyRevision))
			in.InitiatedBy = "user"
			return in
		}(),
		agentInput(uuid.NewString(), "x", generation("not-a-digest", bodyRevision)),
		agentInput(uuid.NewString(), "x", generation(planDigest("z"), -1)),
		agentInput(uuid.NewString(), "x", &PlanGeneration{CommandFingerprint: planDigest("a"), FrozenBodyStateRevision: bodyRevision, Provenance: json.RawMessage(`[]`), ReplayInput: json.RawMessage(`{}`)}),
		agentInput(uuid.NewString(), "x", &PlanGeneration{CommandFingerprint: planDigest("b"), FrozenBodyStateRevision: bodyRevision, Provenance: json.RawMessage(`{}`)}),
	}
	for i, in := range invalid {
		if _, err := svc.Propose(ctx, owner, in); !errors.Is(err, ErrPlanInvalid) {
			t.Fatalf("invalid generation %d: %v", i, err)
		}
	}

	// The database repeats the pairing rule and freezes content and audit.
	for _, statement := range []string{
		`UPDATE plan_proposals SET replay_input = '{"tampered":true}' WHERE id = ?`,
		`UPDATE plan_proposals SET generation_provenance = NULL, replay_input = NULL WHERE id = ?`,
		`UPDATE plan_proposals SET draft = '{"title":"tampered"}' WHERE id = ?`,
		`UPDATE plan_proposals SET content_fingerprint = repeat('0', 64) WHERE id = ?`,
	} {
		if err := db.Exec(statement, first.ID).Error; err == nil {
			t.Fatalf("mutation allowed: %s", statement)
		}
	}
	if err := db.Exec(`UPDATE plan_proposals SET initiated_by = 'user' WHERE id = ?`, first.ID).Error; err == nil {
		t.Fatal("re-attributing an agent proposal to the user allowed")
	}

	// Only the user's explicit decision turns the agent proposal into a plan.
	acceptance, err := svc.Accept(ctx, owner, independentDecision(first))
	if err != nil {
		t.Fatal(err)
	}
	if countPlans() != plansBefore+1 || acceptance.PlanID == uuid.Nil {
		t.Fatalf("accepted agent proposal: %+v", acceptance)
	}
	var audit struct{ Provenance, Replay string }
	if err := db.Raw(`SELECT generation_provenance::text AS provenance, replay_input::text AS replay FROM plan_proposals WHERE id = ?`, first.ID).Scan(&audit).Error; err != nil ||
		audit.Provenance == "" || audit.Replay == "" {
		t.Fatalf("audit after accept: %+v %v", audit, err)
	}
}
