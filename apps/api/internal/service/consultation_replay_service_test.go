package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func consultationReplayTestRun() *model.Run {
	now := time.Now().UTC()
	replay := ConsultationReplayInput{
		ConfigurationID:   consultationV1ConfigurationID,
		ConversationID:    uuid.New().String(),
		UserMessage:       "我最近肩膀疼",
		Profile:           json.RawMessage(`{"gender":"female","birth_date":"1996-08-27","age_years":30}`),
		BodyState:         json.RawMessage(`{"current_revision":3}`),
		BodyStateRevision: 3,
		Phase:             "collecting",
	}
	replayJSON, _ := json.Marshal(replay)
	return &model.Run{
		ID:                   uuid.New(),
		ConversationID:       uuid.New(),
		TurnID:               uuid.New(),
		RequestID:            "req-replay-test",
		UserID:               uuid.New(),
		Status:               "completed",
		Model:                "bodysense-consultation",
		StartedAt:            now,
		AgentConfigurationID: consultationV1ConfigurationID,
		AgentConfiguration:   datatypes.JSON(`{"id":"consult-config-2bd9b46735dd693c","role":"consultation"}`),
		ExecutionProvenance:  datatypes.JSON(`{"status":"executed","runtime":"langgraph"}`),
		ReplayInput:          datatypes.JSON(replayJSON),
	}
}

func TestConsultationHistoricalReplayRecomputesDecisionAuthorityWithoutModel(t *testing.T) {
	repo := &mockRunRepo{runs: map[uuid.UUID]*model.Run{}}
	run := consultationReplayTestRun()
	repo.runs[run.ID] = run
	svc := NewConsultationReplayService(repo)

	decision, err := svc.HistoricalReplay(context.Background(), run.UserID, run.ID)
	if err != nil {
		t.Fatalf("HistoricalReplay: %v", err)
	}
	if decision.SourceConfigurationID != consultationV1ConfigurationID {
		t.Fatalf("unexpected source config: %q", decision.SourceConfigurationID)
	}
	if decision.DecisionPolicyRevision != ConsultationDecisionPolicyV1 {
		t.Fatalf("unexpected policy revision: %q", decision.DecisionPolicyRevision)
	}
	if !decision.ConfigurationIdentityMatch {
		t.Fatal("configuration identity must match for a clean replay")
	}
	if !decision.ReplayInputFrozen {
		t.Fatal("replay input must be frozen")
	}
	if len(decision.InputFingerprint) != 64 {
		t.Fatalf("expected 64-char sha256 fingerprint, got %d", len(decision.InputFingerprint))
	}
}

func TestConsultationReplayFailsClosedWhenRunPredatesProvenance(t *testing.T) {
	repo := &mockRunRepo{runs: map[uuid.UUID]*model.Run{}}
	run := consultationReplayTestRun()
	run.AgentConfigurationID = ""
	repo.runs[run.ID] = run
	svc := NewConsultationReplayService(repo)

	if _, err := svc.HistoricalReplay(context.Background(), run.UserID, run.ID); err != ErrConsultationReplayUnavailable {
		t.Fatalf("expected ErrConsultationReplayUnavailable, got %v", err)
	}
}

func TestConsultationCounterfactualReplayRejectsUnknownConfiguration(t *testing.T) {
	repo := &mockRunRepo{runs: map[uuid.UUID]*model.Run{}}
	run := consultationReplayTestRun()
	repo.runs[run.ID] = run
	svc := NewConsultationReplayService(repo)

	if _, err := svc.CounterfactualReplay(context.Background(), run.UserID, run.ID, "consult-config-does-not-exist"); err == nil {
		t.Fatal("expected unknown configuration rejection")
	}
}
