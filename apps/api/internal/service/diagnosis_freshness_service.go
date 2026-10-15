package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type diagnosisFreshnessStore interface {
	Upsert(ctx context.Context, freshness *model.DiagnosisAnalysisFreshness) error
	Get(ctx context.Context, analysisID, userID uuid.UUID) (*model.DiagnosisAnalysisFreshness, error)
}

type diagnosisFreshnessRevisionSource interface {
	GetBodyStateHead(ctx context.Context, userID uuid.UUID) (*model.BodyStateHead, error)
	ListRevisionsAfter(ctx context.Context, userID uuid.UUID, afterRevision int64, limit int) ([]model.BodyStateRevision, error)
	ResolveRevisionFacts(ctx context.Context, userID uuid.UUID, revisions []model.BodyStateRevision) (BodyStateFactIndex, error)
}

// DiagnosisFreshnessService classifies an immutable analysis against later
// semantic BodyState changes. Revision-number inequality alone is never enough.
type DiagnosisFreshnessService struct {
	store     diagnosisFreshnessStore
	bodyState diagnosisFreshnessRevisionSource
}

func NewDiagnosisFreshnessService(
	store diagnosisFreshnessStore,
	bodyState diagnosisFreshnessRevisionSource,
) *DiagnosisFreshnessService {
	return &DiagnosisFreshnessService{store: store, bodyState: bodyState}
}

type DiagnosisFreshnessReason struct {
	Code       string `json:"code"`
	Revision   int64  `json:"revision"`
	ChangeType string `json:"change_type"`
	ItemID     string `json:"item_id,omitempty"`
	ConcernKey string `json:"concern_key,omitempty"`
	Message    string `json:"message"`
}

func (s *DiagnosisFreshnessService) Preview(
	ctx context.Context,
	userID uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
) (*model.DiagnosisAnalysisFreshness, error) {
	if analysis == nil || analysis.UserID != userID {
		return nil, fmt.Errorf("diagnosis analysis not found")
	}
	head, err := s.bodyState.GetBodyStateHead(ctx, userID)
	if err != nil {
		return nil, err
	}
	revisions, err := s.bodyState.ListRevisionsAfter(ctx, userID, analysis.BodyStateRevision, 500)
	if err != nil {
		return nil, err
	}
	facts, err := s.bodyState.ResolveRevisionFacts(ctx, userID, revisions)
	if err != nil {
		return nil, err
	}
	state, reasons := EvaluateDiagnosisFreshnessPolicy(analysis, revisions, facts)
	return &model.DiagnosisAnalysisFreshness{
		AnalysisID:               analysis.ID,
		UserID:                   userID,
		State:                    state,
		EvaluatedAgainstRevision: head.CurrentRevision,
		Reasons:                  datatypes.JSON(mustJSON(reasons, `[]`)),
		CheckedAt:                time.Now().UTC(),
	}, nil
}

func (s *DiagnosisFreshnessService) Evaluate(
	ctx context.Context,
	userID uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
) (*model.DiagnosisAnalysisFreshness, error) {
	item, err := s.Preview(ctx, userID, analysis)
	if err != nil {
		return nil, err
	}
	if err := s.store.Upsert(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *DiagnosisFreshnessService) GetOrEvaluate(
	ctx context.Context,
	userID uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
) (*model.DiagnosisAnalysisFreshness, error) {
	if analysis == nil {
		return nil, nil
	}
	stored, err := s.store.Get(ctx, analysis.ID, userID)
	if err != nil {
		return nil, err
	}
	head, err := s.bodyState.GetBodyStateHead(ctx, userID)
	if err != nil {
		return nil, err
	}
	if stored != nil && stored.EvaluatedAgainstRevision == head.CurrentRevision {
		return stored, nil
	}
	return s.Evaluate(ctx, userID, analysis)
}

func (s *DiagnosisFreshnessService) PreviewMany(
	ctx context.Context,
	userID uuid.UUID,
	analyses []model.DiagnosisAnalysisRecord,
) (map[uuid.UUID]model.DiagnosisAnalysisFreshness, error) {
	result := make(map[uuid.UUID]model.DiagnosisAnalysisFreshness, len(analyses))
	for index := range analyses {
		item, err := s.Preview(ctx, userID, &analyses[index])
		if err != nil {
			return nil, err
		}
		if item != nil {
			result[item.AnalysisID] = *item
		}
	}
	return result, nil
}

// EvaluateDiagnosisFreshnessPolicy is pure and deterministic so product rules can
// be characterized without a database or model call. It reads V2 revisions
// (ids only) and resolves fact ids through facts. An analysis is stale when
// Safety changed or a fact/observation it cites was corrected or withdrawn;
// it is potentially stale when the user recorded, corrected or withdrew a
// statement in a concern it covers, or regrouped such a statement into an
// episode. Unrelated revisions never invalidate it.
func EvaluateDiagnosisFreshnessPolicy(
	analysis *model.DiagnosisAnalysisRecord,
	revisions []model.BodyStateRevision,
	facts BodyStateFactIndex,
) (string, []DiagnosisFreshnessReason) {
	if analysis == nil || len(revisions) == 0 {
		return model.DiagnosisFreshnessFresh, []DiagnosisFreshnessReason{}
	}
	basis := newDiagnosisBasis(analysis)
	state := model.DiagnosisFreshnessFresh
	reasons := make([]DiagnosisFreshnessReason, 0)
	stale := func(change bodyStateChange, code, itemID, concernKey, message string) {
		state = model.DiagnosisFreshnessStale
		reasons = append(reasons, freshnessReason(change, code, itemID, concernKey, message))
	}
	potentially := func(change bodyStateChange, code, itemID, concernKey, message string) {
		state = maxFreshness(state, model.DiagnosisFreshnessPotentiallyStale)
		reasons = append(reasons, freshnessReason(change, code, itemID, concernKey, message))
	}
	for _, revision := range revisions {
		change := decodeBodyStateChange(revision)
		switch change.Kind {
		case bodyStateChangeSafety:
			stale(change, "safety_state_changed", "", "", "安全状态发生变化，旧分析需要重新审核。")
		case bodyStateChangeFactsRecorded:
			if itemID, concernKey, ok := basis.firstRelatedFact(change.FactIDs, facts); ok {
				potentially(change, "new_related_fact", itemID, concernKey, "同一关注区域出现了新的事实。")
			}
		case bodyStateChangeFactCorrected:
			concernKey := facts.concern(change.TargetID)
			if len(change.FactIDs) > 0 && facts.concern(change.FactIDs[0]) != "" {
				concernKey = facts.concern(change.FactIDs[0])
			}
			if basis.citesFact(change.TargetID) {
				stale(change, "referenced_fact_corrected", change.TargetID, concernKey, "分析引用的事实被纠正。")
			} else if _, replacementConcern, ok := basis.firstRelatedFact(change.FactIDs, facts); ok {
				potentially(change, "related_fact_corrected", change.TargetID, replacementConcern, "同一关注区域的事实被纠正。")
			} else if basis.relatedConcern(facts.concern(change.TargetID)) {
				potentially(change, "related_fact_corrected", change.TargetID, facts.concern(change.TargetID), "同一关注区域的事实被纠正。")
			}
		case bodyStateChangeFactWithdrawn:
			concernKey := facts.concern(change.TargetID)
			if basis.citesFact(change.TargetID) {
				stale(change, "referenced_fact_withdrawn", change.TargetID, concernKey, "分析引用的事实已被撤回。")
			} else if basis.relatedConcern(concernKey) {
				potentially(change, "related_fact_withdrawn", change.TargetID, concernKey, "同一关注区域的事实被撤回。")
			}
		case bodyStateChangeObservationCorrected:
			if basis.citesObservation(change.TargetID) {
				stale(change, "referenced_observation_corrected", change.TargetID, "", "分析引用的观察结果被纠正。")
			}
		case bodyStateChangeObservationWithdrawn:
			if basis.citesObservation(change.TargetID) {
				stale(change, "referenced_observation_withdrawn", change.TargetID, "", "分析引用的观察结果已被撤回。")
			}
		case bodyStateChangeEpisode:
			if itemID, concernKey, ok := basis.firstRelatedFact(change.FactIDs, facts); ok {
				potentially(change, "related_episode_changed", itemID, concernKey, "分析相关症状的病程归组发生变化。")
			}
		}
	}
	return state, reasons
}

func freshnessReason(change bodyStateChange, code, itemID, concernKey, message string) DiagnosisFreshnessReason {
	return DiagnosisFreshnessReason{
		Code: code, Revision: change.Revision, ChangeType: change.ChangeType,
		ItemID: itemID, ConcernKey: concernKey, Message: message,
	}
}

func maxFreshness(current, candidate string) string {
	rank := map[string]int{
		model.DiagnosisFreshnessFresh:            0,
		model.DiagnosisFreshnessPotentiallyStale: 1,
		model.DiagnosisFreshnessStale:            2,
	}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func mustJSON(value any, fallback string) []byte {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 {
		return []byte(fallback)
	}
	return encoded
}
