package service

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"sort"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	diagnosisFreshnessPageSize  = 500
	diagnosisFreshnessMaxPages  = 4
	diagnosisFreshnessFactChunk = 1000
)

type diagnosisFreshnessBodySource interface {
	GetBodyStateHead(context.Context, uuid.UUID) (*model.BodyStateHead, error)
	ListRevisionsAfter(context.Context, uuid.UUID, int64, int) ([]model.BodyStateRevision, error)
	GetBodyFactsByIDs(context.Context, uuid.UUID, []uuid.UUID) ([]model.BodyFact, error)
}

// diagnosisFreshnessSources is the one source authority commits and plan
// admission already use; freshness asks it the same questions without locks
// held across a transaction.
type diagnosisFreshnessSources interface {
	Evidence(context.Context, uuid.UUID, model.DiagnosisEvidenceRef, int64) error
	Claim(context.Context, uuid.UUID, model.DiagnosisSourceClaimRef) error
	Analysis(context.Context, uuid.UUID, uuid.UUID) error
	UserRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int64) error
}

type diagnosisFreshnessRuns interface {
	AnalysisRunBodyStateRevisions(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]int64, error)
}

type diagnosisFreshnessDiagnosis interface {
	Read(context.Context, uuid.UUID) (*NativeDiagnosisView, error)
}

// DiagnosisFreshnessService evaluates the derived freshness of committed
// Conditions and open formal runs. It is read-only by construction.
type DiagnosisFreshnessService struct {
	body      diagnosisFreshnessBodySource
	sources   diagnosisFreshnessSources
	runs      diagnosisFreshnessRuns
	diagnosis diagnosisFreshnessDiagnosis
}

func NewDiagnosisFreshnessService(db *gorm.DB) *DiagnosisFreshnessService {
	diagnosis := repository.NewDiagnosisRepository(db)
	return &DiagnosisFreshnessService{
		body: repository.NewBodyStateRepository(db), sources: repository.NewDiagnosisSourceRepository(db),
		runs: diagnosis, diagnosis: NewNativeDiagnosisReadService(diagnosis),
	}
}

// EvaluateConditions returns one freshness per Condition of view. A Condition
// is based on the BodyState revision its last formal analysis run started
// from: that is everything the Agent and the user saw before adoption.
func (s *DiagnosisFreshnessService) EvaluateConditions(ctx context.Context, owner uuid.UUID, view *NativeDiagnosisView) (map[uuid.UUID]DiagnosisFreshness, error) {
	if view == nil || view.UserID != owner || owner == uuid.Nil {
		return nil, errors.New("diagnosis freshness owner mismatch")
	}
	result := make(map[uuid.UUID]DiagnosisFreshness, len(view.ConditionEntries))
	if len(view.ConditionEntries) == 0 {
		return result, nil
	}
	runIDs := make([]uuid.UUID, 0, len(view.ConditionEntries))
	known := map[uuid.UUID]bool{}
	for _, condition := range view.ConditionEntries {
		if run := conditionAnalysisRun(condition); run != nil && !known[*run] {
			known[*run] = true
			runIDs = append(runIDs, *run)
		}
	}
	bases, err := s.runs.AnalysisRunBodyStateRevisions(ctx, owner, runIDs)
	if err != nil {
		return nil, fmt.Errorf("diagnosis freshness bases: %w", err)
	}
	type subject struct {
		condition NativeDiagnosisCondition
		basis     diagnosisFreshnessBasis
	}
	subjects := make([]subject, 0, len(view.ConditionEntries))
	oldest := int64(-1)
	for _, condition := range view.ConditionEntries {
		basis := diagnosisFreshnessBasis{Evidence: conditionEvidenceRefs(condition)}
		// A Condition without a resolvable analysis run is compared with the
		// whole BodyState history: more review, never less.
		if run := conditionAnalysisRun(condition); run != nil {
			basis.BodyStateRevision = bases[*run]
		}
		if oldest < 0 || basis.BodyStateRevision < oldest {
			oldest = basis.BodyStateRevision
		}
		subjects = append(subjects, subject{condition, basis})
	}
	cited := make([]model.DiagnosisEvidenceRef, 0)
	for _, item := range subjects {
		cited = append(cited, item.basis.Evidence...)
	}
	changes, err := s.changes(ctx, owner, oldest, cited)
	if err != nil {
		return nil, err
	}
	for _, item := range subjects {
		condition := item.condition
		item.basis.Concerns = diagnosisConditionConcerns(item.basis.Evidence, condition.Qualifiers, changes.Facts)
		builder := newDiagnosisFreshnessBuilder(item.basis.BodyStateRevision, changes.Head, view.CurrentRevision)
		evaluateDiagnosisFreshness(builder, item.basis, changes)
		for _, claim := range condition.SourceClaimRefs {
			if unavailable, err := diagnosisSourceRefused(s.sources.Claim(ctx, owner, claim)); err != nil {
				return nil, err
			} else if unavailable {
				builder.add(DiagnosisFreshnessReason{Code: freshnessSourceClaimUnavailable, ClaimID: claim.ClaimID})
			}
		}
		if condition.Assessment != nil {
			run := condition.Assessment.BasedOnRunID
			if unavailable, err := diagnosisSourceRefused(s.sources.Analysis(ctx, owner, run)); err != nil {
				return nil, err
			} else if unavailable {
				builder.add(DiagnosisFreshnessReason{Code: freshnessAssessmentUnavailable, RunID: &run})
			}
		}
		result[condition.ConditionID] = builder.result()
	}
	return result, nil
}

// EvaluateRun returns the freshness of an open formal run: whether its
// proposals still rest on the Diagnosis revision and the sources it was
// started from. Terminal runs are frozen history and have no freshness.
func (s *DiagnosisFreshnessService) EvaluateRun(ctx context.Context, owner uuid.UUID, run *NativeDiagnosisRunProjection) (*DiagnosisFreshness, error) {
	if run == nil || run.OwnerID != owner || owner == uuid.Nil {
		return nil, errors.New("diagnosis run freshness owner mismatch")
	}
	if run.State != "open" {
		return nil, nil
	}
	view, err := s.diagnosis.Read(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("diagnosis run freshness head: %w", err)
	}
	if view == nil || view.UserID != owner || view.CurrentRevision < run.BasedOnDiagnosisRevision {
		return nil, errors.New("diagnosis run freshness head inconsistent")
	}
	basis := diagnosisFreshnessBasis{BodyStateRevision: run.BasedOnBodyStateRevision, Evidence: runEvidenceRefs(run)}
	changes, err := s.changes(ctx, owner, basis.BodyStateRevision, basis.Evidence)
	if err != nil {
		return nil, err
	}
	basis.Concerns = diagnosisConditionConcerns(basis.Evidence, nil, changes.Facts)
	builder := newDiagnosisFreshnessBuilder(basis.BodyStateRevision, changes.Head, view.CurrentRevision)
	if view.CurrentRevision != run.BasedOnDiagnosisRevision {
		current := view.CurrentRevision
		builder.add(DiagnosisFreshnessReason{Code: freshnessDiagnosisRevisionChanged, DiagnosisRevision: &current})
		changed := make([]NativeDiagnosisCondition, 0)
		for _, condition := range view.ConditionEntries {
			if condition.CommittedRevision > run.BasedOnDiagnosisRevision {
				changed = append(changed, condition)
			}
		}
		sort.Slice(changed, func(i, j int) bool {
			if changed[i].CommittedRevision != changed[j].CommittedRevision {
				return changed[i].CommittedRevision < changed[j].CommittedRevision
			}
			return changed[i].ConditionID.String() < changed[j].ConditionID.String()
		})
		for _, condition := range changed {
			id, revision := condition.ConditionID, condition.CommittedRevision
			builder.add(DiagnosisFreshnessReason{Code: freshnessConditionChanged, ConditionID: &id, DiagnosisRevision: &revision})
		}
	}
	evaluateDiagnosisFreshness(builder, basis, changes)
	if unavailable, err := diagnosisSourceRefused(s.sources.UserRequest(ctx, owner, run.RequestEvidenceID, run.RequestMessageID, run.RequestSourceVersion)); err != nil {
		return nil, err
	} else if unavailable {
		builder.add(DiagnosisFreshnessReason{Code: freshnessRequestUnavailable})
	}
	freshness := builder.result()
	return &freshness, nil
}

// changes reads the bounded BodyState window after the oldest basis and asks
// the source authority about every cited source at the evaluated head.
func (s *DiagnosisFreshnessService) changes(ctx context.Context, owner uuid.UUID, after int64, cited []model.DiagnosisEvidenceRef) (diagnosisFreshnessChanges, error) {
	changes := diagnosisFreshnessChanges{Revisions: []model.BodyStateRevision{}, Facts: BodyStateFactIndex{}, Unavailable: map[string]bool{}}
	head, err := s.body.GetBodyStateHead(ctx, owner)
	if err != nil {
		return changes, fmt.Errorf("diagnosis freshness body head: %w", err)
	}
	if head == nil || head.UserID != owner || head.CurrentRevision < 0 {
		return changes, errors.New("diagnosis freshness body head inconsistent")
	}
	changes.Head = head.CurrentRevision
	if after < 0 {
		after = 0
	}
	cursor := after
	changes.Complete = cursor >= changes.Head
	for page := 0; page < diagnosisFreshnessMaxPages && !changes.Complete; page++ {
		revisions, err := s.body.ListRevisionsAfter(ctx, owner, cursor, diagnosisFreshnessPageSize)
		if err != nil {
			return changes, fmt.Errorf("diagnosis freshness revisions: %w", err)
		}
		for _, revision := range revisions {
			if revision.UserID != owner || revision.Revision <= cursor {
				return changes, errors.New("diagnosis freshness revisions out of order")
			}
			cursor = revision.Revision
			if revision.Revision <= changes.Head {
				changes.Revisions = append(changes.Revisions, revision)
			}
		}
		if len(revisions) < diagnosisFreshnessPageSize || cursor >= changes.Head {
			changes.Complete = true
		}
	}
	ids := bodyStateRevisionFactIDs(changes.Revisions)
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		seen[id] = true
	}
	for _, ref := range cited {
		if ref.EvidenceType == model.DiagnosisBodyFact && ref.EvidenceID != uuid.Nil && !seen[ref.EvidenceID] {
			seen[ref.EvidenceID] = true
			ids = append(ids, ref.EvidenceID)
		}
	}
	for start := 0; start < len(ids); start += diagnosisFreshnessFactChunk {
		end := min(start+diagnosisFreshnessFactChunk, len(ids))
		facts, err := s.body.GetBodyFactsByIDs(ctx, owner, ids[start:end])
		if err != nil {
			return changes, fmt.Errorf("diagnosis freshness facts: %w", err)
		}
		for _, fact := range facts {
			if fact.UserID != owner {
				return changes, errors.New("diagnosis freshness facts owner mismatch")
			}
			changes.Facts[fact.ID.String()] = BodyStateFactRef{Kind: fact.Kind, ConcernKey: bodyFactConcernKey(fact)}
		}
	}
	checked := map[string]bool{}
	for _, ref := range cited {
		key := evidenceKey(ref)
		if checked[key] {
			continue
		}
		checked[key] = true
		unavailable, err := diagnosisSourceRefused(s.sources.Evidence(ctx, owner, ref, changes.Head))
		if err != nil {
			return changes, err
		}
		if unavailable {
			changes.Unavailable[key] = true
		}
	}
	return changes, nil
}

// diagnosisSourceRefused separates a source the authority refused (absent,
// withdrawn, erased, no longer current) from an infrastructure failure, which
// must surface as an error instead of a review signal.
func diagnosisSourceRefused(err error) (bool, error) {
	if err == nil {
		return false, nil
	}
	var pgErr *pgconn.PgError
	var netErr net.Error
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, driver.ErrBadConn) || errors.As(err, &pgErr) || errors.As(err, &netErr) {
		return false, fmt.Errorf("diagnosis freshness source check: %w", err)
	}
	return true, nil
}

func conditionAnalysisRun(condition NativeDiagnosisCondition) *uuid.UUID {
	if condition.LastAnalysisRunID != nil && *condition.LastAnalysisRunID != uuid.Nil {
		return condition.LastAnalysisRunID
	}
	if condition.Assessment != nil && condition.Assessment.BasedOnRunID != uuid.Nil {
		id := condition.Assessment.BasedOnRunID
		return &id
	}
	return nil
}

func conditionEvidenceRefs(condition NativeDiagnosisCondition) []model.DiagnosisEvidenceRef {
	refs := make([]model.DiagnosisEvidenceRef, 0, len(condition.EvidenceLinks))
	for _, link := range condition.EvidenceLinks {
		refs = append(refs, model.DiagnosisEvidenceRef{EvidenceType: link.EvidenceType, EvidenceID: link.EvidenceID, Version: link.EvidenceVersion})
	}
	return refs
}

// runEvidenceRefs is everything an open run's proposals rest on: its frozen
// input and scope, and the evidence of every proposal still open to a choice.
func runEvidenceRefs(run *NativeDiagnosisRunProjection) []model.DiagnosisEvidenceRef {
	refs := make([]model.DiagnosisEvidenceRef, 0, len(run.InputEvidenceRefs)+len(run.Scope.EvidenceRefs))
	refs = append(refs, run.InputEvidenceRefs...)
	refs = append(refs, run.Scope.EvidenceRefs...)
	for _, proposal := range run.Proposals {
		if proposal.State == "pending" || proposal.State == "decided" {
			refs = append(refs, proposal.EvidenceRefs...)
			refs = append(refs, proposal.CounterEvidenceRefs...)
		}
	}
	return refs
}
