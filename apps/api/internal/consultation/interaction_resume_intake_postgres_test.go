package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func bindIntakeQuestion(t *testing.T, f resumeFixture, toolCallID string, question json.RawMessage) {
	t.Helper()
	f.interaction.ToolCallID, f.interaction.Question = toolCallID, datatypes.JSON(question)
	if err := f.db.Model(&model.AgentInteraction{}).Where("id = ?", f.interaction.ID).Updates(map[string]any{"tool_call_id": toolCallID, "question": datatypes.JSON(question)}).Error; err != nil {
		t.Fatal(err)
	}
}

func intakeCommand(f resumeFixture, fields map[string]any) model.ClaimInteractionResume {
	answer, _ := json.Marshal(map[string]any{"fields": fields})
	return model.ClaimInteractionResume{UserID: f.owner, ConversationID: f.interaction.ConversationID, InteractionID: f.interaction.ID, RequestID: uuid.NewString(), Answer: datatypes.JSON(answer)}
}

func intakeBodyState(f resumeFixture) *service.BodyStateService {
	return service.NewBodyStateService(repository.NewBodyStateRepository(f.db)).WithBodyRegionIDValidator(service.NewCanonicalBodyRegionIDValidator())
}

// commitTurnSymptom runs the real v3 record_body_state commit for the
// fixture's own user message, exactly as the runtime stream does before it
// asks the intake, and returns the committed fact references.
func commitTurnSymptom(t *testing.T, f resumeFixture, presence string) (string, []service.CommittedBodyFact) {
	t.Helper()
	run := f.envelope.Run
	if err := f.db.Model(&model.Run{}).Where("id = ?", run.ID).Update("status", model.RunStatusRunning).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.db.Model(&model.Run{}).Where("id = ?", run.ID).Update("status", model.RunStatusWaitingUser).Error; err != nil {
			t.Fatal(err)
		}
	}()
	ledger := repository.NewBodyStateRepository(f.db)
	runtime := &Runtime{bodyFacts: service.NewBodyFactService(ledger, service.NewCanonicalBodyRegionIDValidator())}
	state := streamState{UID: f.owner, ConversationID: run.ConversationID, Run: run, ExpectedConfigurationID: service.BodyFactConsultationConfigurationID}
	toolCall := "bodyfact-intake-v1:" + run.ID.String()
	proposal := json.RawMessage(`{"status":"proposed","expected_revision":0,"facts":[{"kind":"symptom","statement":"A synthetic ambiguous source","body_region_ids":[],"temporal":{"precision":"unknown"},"details":{"presence":"` + presence + `"}}]}`)
	_, committed, err := runtime.commitBodyFactProposal(context.Background(), state, toolCall, proposal)
	if err != nil || len(committed) != 1 {
		t.Fatalf("turn symptom commit: %v %#v", err, committed)
	}
	return toolCall, committed
}

// runtimeIntake is the question the Python runtime emits; Go binds it.
func runtimeIntake(factToolCall string) (string, json.RawMessage) {
	capture := service.SymptomIntakeCaptureID(factToolCall, 0)
	raw, _ := json.Marshal(map[string]any{
		"question": "我已记录你刚才描述的不适。为了安全分层，请确认：", "answer_type": "text", "purpose": "symptom_intake",
		"fields": []any{map[string]any{"key": "safety_signals", "label": "安全信号", "answer_type": "multi_choice", "required": true}},
		"state_binding": map[string]any{
			"revision": service.SymptomIntakeBindingRevision, "capture_id": capture,
			"fact_ref":  map[string]any{"tool_call_id": factToolCall, "index": 0},
			"field_map": map[string]any{"safety_signals": "safety_signals"},
		},
	})
	return "intake-" + capture, raw
}

func boundIntakeFixture(t *testing.T) (resumeFixture, uuid.UUID) {
	t.Helper()
	f := newResumeFixture(t)
	factToolCall, committed := commitTurnSymptom(t, f, "present")
	toolCall, raw := runtimeIntake(factToolCall)
	bound, ok, err := service.BindSymptomIntakeQuestion(raw, toolCall, committed)
	if err != nil || !ok {
		t.Fatalf("intake was not bound: %v", err)
	}
	bindIntakeQuestion(t, f, toolCall, bound)
	return f, committed[0].FactID
}

func TestInteractionResumeBoundIntakeRecordsScreenWithoutDuplicateFactPostgres(t *testing.T) {
	f, factID := boundIntakeFixture(t)
	bodyState := intakeBodyState(f)
	snapshot, err := bodyState.GetSnapshot(context.Background(), f.owner, 10)
	if err != nil {
		t.Fatal(err)
	}
	if envelope, err := service.ProjectSafetyEnvelopeV2(snapshot); err != nil || envelope.Coverage.Complete {
		t.Fatalf("an unscreened turn symptom must leave coverage incomplete: %+v %v", envelope.Coverage, err)
	}

	resumes := f.resumes.WithStructuredIntake(bodyState)
	command := intakeCommand(f, map[string]any{"safety_signals": []string{"麻木"}})
	claimed, err := resumes.Claim(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	// The bound symptom is not re-inserted: the answer only adds its screen.
	resumeCount(t, f, "body_facts", 1)
	var fact model.BodyFact
	if err := f.db.First(&fact, "id = ? AND user_id = ?", factID, f.owner).Error; err != nil {
		t.Fatal(err)
	}
	var screens []model.SafetyScreen
	if err := f.db.Where("user_id = ?", f.owner).Find(&screens).Error; err != nil || len(screens) != 1 {
		t.Fatalf("expected one safety screen, got %d (%v)", len(screens), err)
	}
	screen := screens[0]
	if screen.BodyFactID != factID || "intake-"+screen.CaptureID != f.interaction.ToolCallID || screen.SourceInteractionID != f.interaction.ID ||
		screen.SourceMessageID != claimed.Envelope.UserMessage.ID || screen.SourceRunID != f.envelope.Run.ID ||
		!screen.Numbness || screen.Trauma || screen.RadiatingPain || screen.Weakness || screen.Dizziness || screen.BodyStateRevision != fact.CreatedRevision+1 {
		t.Fatalf("unexpected safety screen: %#v (fact revision %d)", screen, fact.CreatedRevision)
	}
	var revision model.BodyStateRevision
	if err := f.db.Where("user_id = ? AND revision = ?", f.owner, screen.BodyStateRevision).First(&revision).Error; err != nil || revision.ChangeType != "safety.screened" {
		t.Fatalf("screen must commit its own owner revision: %#v %v", revision, err)
	}

	retry, err := resumes.Claim(context.Background(), command)
	if err != nil || !retry.Envelope.Existed {
		t.Fatalf("retry was not replayed: %v", err)
	}
	resumeCount(t, f, "body_facts", 1)
	resumeCount(t, f, "safety_screens", 1)

	snapshot, err = bodyState.GetSnapshot(context.Background(), f.owner, 10)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := service.ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !envelope.Coverage.Complete || len(envelope.Coverage.CoveredSourceRefs) != 1 || envelope.Coverage.CoveredSourceRefs[0] != "body-state:fact:"+factID.String() {
		t.Fatalf("screened symptom is not covered: %+v", envelope.Coverage)
	}
	if len(envelope.ActiveBlockers) != 1 || envelope.ActiveBlockers[0].Concept != service.SafetyNumbness || envelope.ActiveBlockers[0].SourceRef != "body-state:safety-screen:"+screen.ID.String() {
		t.Fatalf("positive screen answer did not block: %+v", envelope.ActiveBlockers)
	}
	if _, err := bodyState.ResolveSafetyState(context.Background(), f.owner, nil, "cleared_by_review", "synthetic review"); err != nil {
		t.Fatal(err)
	}
	snapshot, err = bodyState.GetSnapshot(context.Background(), f.owner, 10)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err = service.ProjectSafetyEnvelopeV2(snapshot)
	if err != nil || envelope.RequiresReview || !envelope.Coverage.Complete {
		t.Fatalf("explicit Go resolution did not clear the screen blocker: %+v %v", envelope, err)
	}
}

func TestInteractionResumeNoneOfTheAboveCompletesCoveragePostgres(t *testing.T) {
	f, _ := boundIntakeFixture(t)
	bodyState := intakeBodyState(f)
	if _, err := f.resumes.WithStructuredIntake(bodyState).Claim(context.Background(), intakeCommand(f, map[string]any{"safety_signals": []string{"以上均无"}})); err != nil {
		t.Fatal(err)
	}
	snapshot, err := bodyState.GetSnapshot(context.Background(), f.owner, 10)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := service.ProjectSafetyEnvelopeV2(snapshot)
	if err != nil || !envelope.Coverage.Complete || envelope.RequiresReview || len(envelope.ActiveBlockers) != 0 {
		t.Fatalf("a negative screen must complete coverage without blockers: %+v %v", envelope, err)
	}
}

func assertIntakeClaimLeftNothing(t *testing.T, f resumeFixture, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("invalid intake binding must fail the claim")
	}
	resumeCount(t, f, "safety_screens", 0)
	resumeCount(t, f, "interaction_resumes", 0)
	resumeCount(t, f, "messages", 2)
	var interaction model.AgentInteraction
	if err := f.db.First(&interaction, "id = ?", f.interaction.ID).Error; err != nil || interaction.Status != model.AgentInteractionPending {
		t.Fatalf("failed intake left the interaction answered: %v %v", interaction.Status, err)
	}
}

func TestInteractionResumeIntakeBoundToForeignOrMissingFactFailsClosedPostgres(t *testing.T) {
	t.Run("other-owner-fact", func(t *testing.T) {
		f, _ := boundIntakeFixture(t)
		_, foreignFact := boundIntakeFixture(t)
		var q map[string]any
		_ = json.Unmarshal(f.interaction.Question, &q)
		q["state_binding"].(map[string]any)["body_fact_id"] = foreignFact.String()
		forged, _ := json.Marshal(q)
		bindIntakeQuestion(t, f, f.interaction.ToolCallID, forged)
		_, err := f.resumes.WithStructuredIntake(intakeBodyState(f)).Claim(context.Background(), intakeCommand(f, map[string]any{"safety_signals": []string{"以上均无"}}))
		assertIntakeClaimLeftNothing(t, f, err)
	})
	t.Run("absent-symptom", func(t *testing.T) {
		f := newResumeFixture(t)
		factToolCall, committed := commitTurnSymptom(t, f, "absent")
		toolCall, raw := runtimeIntake(factToolCall)
		if _, ok, _ := service.BindSymptomIntakeQuestion(raw, toolCall, committed); ok {
			t.Fatal("an absent symptom must not be bound")
		}
		// Even a hand-forged binding to the absent fact cannot record a screen.
		forged := strings.Replace(string(raw), `"fact_ref":{"index":0,"tool_call_id":"`+factToolCall+`"}`, `"body_fact_id":"`+committed[0].FactID.String()+`"`, 1)
		forged = strings.Replace(forged, `"fields":[{"answer_type":"multi_choice","key":"safety_signals","label":"安全信号","required":true}]`,
			`"fields":[{"answer_type":"multi_choice","exclusive_options":["以上均无"],"key":"safety_signals","label":"安全信号","options":["外伤或创伤","放射痛","麻木","无力","头晕","以上均无"],"required":true}]`, 1)
		if !service.IsBoundSymptomIntake(toolCall, json.RawMessage(forged)) {
			t.Fatalf("test forgery is not shaped like a bound intake: %s", forged)
		}
		bindIntakeQuestion(t, f, toolCall, json.RawMessage(forged))
		_, err := f.resumes.WithStructuredIntake(intakeBodyState(f)).Claim(context.Background(), intakeCommand(f, map[string]any{"safety_signals": []string{"以上均无"}}))
		assertIntakeClaimLeftNothing(t, f, err)
	})
}

func TestInteractionResumeModelAuthoredIntakeAndMissingRecorderPostgres(t *testing.T) {
	t.Run("model-authored", func(t *testing.T) {
		f, _ := boundIntakeFixture(t)
		bindIntakeQuestion(t, f, "model-call-1", json.RawMessage(f.interaction.Question))
		if _, err := f.resumes.WithStructuredIntake(intakeBodyState(f)).Claim(context.Background(), intakeCommand(f, map[string]any{"safety_signals": []string{"以上均无"}})); err != nil {
			t.Fatal(err)
		}
		resumeCount(t, f, "body_facts", 1)
		resumeCount(t, f, "safety_screens", 0)
	})
	t.Run("missing-recorder", func(t *testing.T) {
		f, _ := boundIntakeFixture(t)
		_, err := f.resumes.Claim(context.Background(), intakeCommand(f, map[string]any{"safety_signals": []string{"以上均无"}}))
		if !errors.Is(err, model.ErrInteractionResumeUnavailable) {
			t.Fatalf("bound intake without a recorder must be refused, got %v", err)
		}
		resumeCount(t, f, "interaction_resumes", 0)
	})
}
