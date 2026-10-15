package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// memoryBodyContextLedger is an append-only in-memory stand-in for the V2
// BodyFact/BodyObservation ledger with the same revision guard semantics.
type memoryBodyContextLedger struct {
	revision     int64
	facts        []model.BodyFact
	observations []model.BodyObservationRecord
	screens      []model.SafetyScreen
	commands     int
}

func (m *memoryBodyContextLedger) ListSafetyScreens(context.Context, uuid.UUID) ([]model.SafetyScreen, error) {
	return append([]model.SafetyScreen{}, m.screens...), nil
}

func (m *memoryBodyContextLedger) guard(expected int64) error {
	if expected != m.revision {
		return fmt.Errorf("%w: expected %d current %d", repository.ErrBodyStateRevisionConflict, expected, m.revision)
	}
	m.revision++
	m.commands++
	return nil
}

func (m *memoryBodyContextLedger) ListFacts(_ context.Context, owner uuid.UUID, _ int64) (*model.BodyFactPage, error) {
	page := &model.BodyFactPage{UserID: owner, CurrentRevision: m.revision}
	for index := len(m.facts) - 1; index >= 0; index-- {
		page.Facts = append(page.Facts, m.facts[index])
	}
	return page, nil
}

func (m *memoryBodyContextLedger) RecordDirectFacts(_ context.Context, owner uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	if err := command.Validate(); err != nil {
		return nil, err
	}
	if err := m.guard(command.ExpectedRevision); err != nil {
		return nil, err
	}
	for _, input := range command.Facts {
		temporal, _ := json.Marshal(input.Temporal)
		regions, _ := json.Marshal(input.BodyRegionIDs)
		m.facts = append(m.facts, model.BodyFact{
			ID: uuid.New(), UserID: owner, Subject: "self", Kind: input.Kind, Statement: input.Statement,
			BodyRegionIDs: datatypes.JSON(regions), Temporal: datatypes.JSON(temporal), Details: datatypes.JSON(input.Details),
			AssertionStatus: "asserted", CreatedRevision: m.revision, RecordedAt: time.Now().UTC(),
		})
	}
	return &model.BodyFactCommit{UserID: owner, Revision: m.revision}, nil
}

func (m *memoryBodyContextLedger) WithdrawDirectFact(_ context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand) (*model.BodyFactCommit, error) {
	for index := range m.facts {
		if m.facts[index].ID == target && m.facts[index].AssertionStatus == "asserted" {
			if err := m.guard(command.ExpectedRevision); err != nil {
				return nil, err
			}
			m.facts[index].AssertionStatus = "withdrawn"
			return &model.BodyFactCommit{UserID: owner, Revision: m.revision}, nil
		}
	}
	return nil, model.ErrBodyFactNotCurrent
}

func (m *memoryBodyContextLedger) ListDirectObservations(_ context.Context, owner uuid.UUID, _ int64) (*model.BodyObservationPage, error) {
	page := &model.BodyObservationPage{UserID: owner, CurrentRevision: m.revision}
	for index := len(m.observations) - 1; index >= 0; index-- {
		page.Observations = append(page.Observations, m.observations[index])
	}
	return page, nil
}

func (m *memoryBodyContextLedger) RecordDirectObservation(_ context.Context, owner uuid.UUID, command model.RecordBodyObservationCommand) (*model.BodyObservationCommit, error) {
	if err := command.Validate(); err != nil {
		return nil, err
	}
	if err := m.guard(command.ExpectedRevision); err != nil {
		return nil, err
	}
	input := command.Observation
	m.observations = append(m.observations, model.BodyObservationRecord{
		ID: uuid.New(), UserID: owner, Kind: input.Kind, Statement: input.Statement, RawValue: input.RawValue,
		RawUnit: input.RawUnit, MeasurementContext: input.MeasurementContext, ObservedAt: input.ObservedAt,
		AssertionStatus: "asserted", CreatedRevision: m.revision, RecordedAt: time.Now().UTC(),
	})
	return &model.BodyObservationCommit{UserID: owner, Revision: m.revision}, nil
}

// memoryBodyContextStore implements bodyContextStore over the memory ledger.
type memoryBodyContextStore struct {
	ledger  memoryBodyContextLedger
	updates []BodyContextUpdate
	err     error
}

func (s *memoryBodyContextStore) CurrentBodyContext(ctx context.Context, userID uuid.UUID) (*BodyContext, error) {
	return loadBodyContext(ctx, &s.ledger, userID)
}

func (s *memoryBodyContextStore) UpdateBodyContext(ctx context.Context, userID uuid.UUID, expected *int64, update BodyContextUpdate) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	s.updates = append(s.updates, update)
	return applyBodyContextUpdate(ctx, &s.ledger, userID, expected, update, time.Now())
}
