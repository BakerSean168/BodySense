package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const intakeTestCapture = "aabbccddeeff00112233aabb"

const intakeTestQuestion = `{"question":"请补全下面信息：","answer_type":"text","purpose":"symptom_intake",
"fields":[
 {"key":"safety_signals","label":"安全信号","answer_type":"multi_choice","required":true,"options":["外伤或创伤","放射痛","麻木","无力","头晕","以上均无"],"exclusive_options":["以上均无"]},
 {"key":"duration","label":"持续多久","answer_type":"single_choice","required":true,"options":["今天或24小时内","2–7天","1–4周","超过1个月"]}],
"state_binding":{"revision":"symptom-intake-binding-v1","capture_id":"aabbccddeeff00112233aabb",
 "seed_info":{"body_part":"腰部","symptom_type":"疼痛"},
 "field_map":{"safety_signals":"safety_signals","duration":"duration"}}}`

func bindIntakeQuestion(t *testing.T, f resumeFixture, toolCallID, question string) {
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

func TestInteractionResumeStructuredIntakeRecordsSymptomAndSafetyScreenPostgres(t *testing.T) {
	f := newResumeFixture(t)
	bindIntakeQuestion(t, f, "intake-"+intakeTestCapture, intakeTestQuestion)
	bodyState := intakeBodyState(f)
	resumes := f.resumes.WithStructuredIntake(bodyState)
	command := intakeCommand(f, map[string]any{"safety_signals": []string{"麻木"}, "duration": "1–4周"})
	claimed, err := resumes.Claim(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}

	var facts []model.BodyFact
	if err := f.db.Where("user_id = ?", f.owner).Find(&facts).Error; err != nil || len(facts) != 1 {
		t.Fatalf("expected exactly one symptom fact, got %d (%v)", len(facts), err)
	}
	fact := facts[0]
	if fact.Kind != "symptom" || fact.AssertionStatus != "asserted" || fact.Statement != "安全信号: 麻木；持续多久: 1–4周" {
		t.Fatalf("intake did not record the user's answer as a symptom statement: %#v", fact)
	}
	var evidence model.StatementEvidence
	if err := f.db.Where("user_id = ? AND id = ?", f.owner, fact.SourceEvidenceID).First(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if evidence.SourceKind != "structured_answer" || evidence.SourceInteractionID == nil || *evidence.SourceInteractionID != f.interaction.ID ||
		evidence.SourceMessageID == nil || *evidence.SourceMessageID != claimed.Envelope.UserMessage.ID || evidence.Excerpt != fact.Statement ||
		evidence.SourceToolCallID == nil || *evidence.SourceToolCallID != "intake-"+intakeTestCapture {
		t.Fatalf("symptom is not bound to the accepted answer: %#v", evidence)
	}
	var screens []model.SafetyScreen
	if err := f.db.Where("user_id = ?", f.owner).Find(&screens).Error; err != nil || len(screens) != 1 {
		t.Fatalf("expected one safety screen, got %d (%v)", len(screens), err)
	}
	screen := screens[0]
	if screen.CaptureID != intakeTestCapture || screen.SourceInteractionID != f.interaction.ID || screen.SourceMessageID != claimed.Envelope.UserMessage.ID ||
		!screen.Numbness || screen.Trauma || screen.RadiatingPain || screen.Weakness || screen.Dizziness || screen.BodyStateRevision != fact.CreatedRevision {
		t.Fatalf("unexpected safety screen: %#v", screen)
	}
	var state model.BodyState
	if err := f.db.Where("user_id = ?", f.owner).First(&state).Error; err != nil || state.CurrentRevision != fact.CreatedRevision {
		t.Fatalf("intake must commit exactly one owner revision: %#v %v", state, err)
	}

	retry, err := resumes.Claim(context.Background(), command)
	if err != nil || !retry.Envelope.Existed {
		t.Fatalf("retry was not replayed: %v", err)
	}
	resumeCount(t, f, "body_facts", 1)
	resumeCount(t, f, "safety_screens", 1)

	snapshot, err := bodyState.GetSnapshot(context.Background(), f.owner, 10)
	if err != nil {
		t.Fatal(err)
	}
	projected := false
	for _, item := range snapshot.Facts {
		projected = projected || (item.ID == fact.ID && item.Kind == "discomfort" && item.Value == fact.Statement)
	}
	if !projected {
		t.Fatalf("V2 symptom is not visible to snapshot readers: %#v", snapshot.Facts)
	}
	envelope, err := service.ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !envelope.Coverage.Complete || len(envelope.Coverage.CoveredSourceRefs) != 1 || envelope.Coverage.CoveredSourceRefs[0] != "body-state:fact:"+fact.ID.String() {
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

func TestInteractionResumeStructuredIntakeFailureLeavesNothingPostgres(t *testing.T) {
	f := newResumeFixture(t)
	// The runtime binding lacks a symptom identity: nothing may be committed,
	// including the chat answer itself, so the Agent never resumes from it.
	broken := `{"question":"请补全下面信息：","answer_type":"text","purpose":"symptom_intake",
"fields":[{"key":"duration","label":"持续多久","answer_type":"single_choice","required":true,"options":["1–4周"]}],
"state_binding":{"revision":"symptom-intake-binding-v1","capture_id":"aabbccddeeff00112233aabb","seed_info":{"body_part":"腰部"},"field_map":{"duration":"duration"}}}`
	bindIntakeQuestion(t, f, "intake-"+intakeTestCapture, broken)
	if _, err := f.resumes.WithStructuredIntake(intakeBodyState(f)).Claim(context.Background(), intakeCommand(f, map[string]any{"duration": "1–4周"})); err == nil {
		t.Fatal("invalid intake binding must fail the claim")
	}
	resumeCount(t, f, "body_facts", 0)
	resumeCount(t, f, "safety_screens", 0)
	resumeCount(t, f, "interaction_resumes", 0)
	resumeCount(t, f, "messages", 2)
	var interaction model.AgentInteraction
	if err := f.db.First(&interaction, "id = ?", f.interaction.ID).Error; err != nil || interaction.Status != model.AgentInteractionPending {
		t.Fatalf("failed intake left the interaction answered: %v %v", interaction.Status, err)
	}
}

func TestInteractionResumeStructuredIntakeWithoutScreenPostgres(t *testing.T) {
	f := newResumeFixture(t)
	// A checklist without the runtime's exclusive option is not a valid screen.
	question := `{"question":"请补全下面信息：","answer_type":"text","purpose":"symptom_intake",
"fields":[{"key":"safety_signals","label":"安全信号","answer_type":"multi_choice","required":true,"options":["外伤或创伤","放射痛","麻木","无力","头晕","以上均无"]},
 {"key":"duration","label":"持续多久","answer_type":"single_choice","required":true,"options":["1–4周"]}],
"state_binding":{"revision":"symptom-intake-binding-v1","capture_id":"aabbccddeeff00112233aabb","seed_info":{"body_part":"腰部","symptom_type":"疼痛"},"field_map":{"safety_signals":"safety_signals","duration":"duration"}}}`
	bindIntakeQuestion(t, f, "intake-"+intakeTestCapture, question)
	bodyState := intakeBodyState(f)
	if _, err := f.resumes.WithStructuredIntake(bodyState).Claim(context.Background(), intakeCommand(f, map[string]any{"safety_signals": []string{"以上均无"}, "duration": "1–4周"})); err != nil {
		t.Fatal(err)
	}
	resumeCount(t, f, "body_facts", 1)
	resumeCount(t, f, "safety_screens", 0)
	snapshot, err := bodyState.GetSnapshot(context.Background(), f.owner, 10)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := service.ProjectSafetyEnvelopeV2(snapshot)
	if err != nil || envelope.Coverage.Complete || len(envelope.Coverage.IncompleteSourceRefs) != 1 {
		t.Fatalf("an unscreened symptom must leave coverage incomplete: %+v %v", envelope.Coverage, err)
	}
}

func TestInteractionResumeModelAuthoredIntakeAndMissingRecorderPostgres(t *testing.T) {
	t.Run("model-authored", func(t *testing.T) {
		f := newResumeFixture(t)
		bindIntakeQuestion(t, f, "model-call-1", intakeTestQuestion)
		if _, err := f.resumes.WithStructuredIntake(intakeBodyState(f)).Claim(context.Background(), intakeCommand(f, map[string]any{"safety_signals": []string{"以上均无"}, "duration": "1–4周"})); err != nil {
			t.Fatal(err)
		}
		resumeCount(t, f, "body_facts", 0)
		resumeCount(t, f, "safety_screens", 0)
	})
	t.Run("missing-recorder", func(t *testing.T) {
		f := newResumeFixture(t)
		bindIntakeQuestion(t, f, "intake-"+intakeTestCapture, intakeTestQuestion)
		_, err := f.resumes.Claim(context.Background(), intakeCommand(f, map[string]any{"safety_signals": []string{"以上均无"}, "duration": "1–4周"}))
		if !errors.Is(err, model.ErrInteractionResumeUnavailable) {
			t.Fatalf("bound intake without a recorder must be refused, got %v", err)
		}
		resumeCount(t, f, "interaction_resumes", 0)
	})
}
