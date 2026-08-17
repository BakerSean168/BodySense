package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateSnapshot, error)
	ListRevisionsAfter(ctx context.Context, userID uuid.UUID, afterRevision int64, limit int) ([]model.BodyStateRevision, error)
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
	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	revisions, err := s.bodyState.ListRevisionsAfter(ctx, userID, analysis.BodyStateRevision, 500)
	if err != nil {
		return nil, err
	}
	state, reasons := EvaluateDiagnosisFreshnessPolicy(analysis, revisions)
	return &model.DiagnosisAnalysisFreshness{
		AnalysisID:               analysis.ID,
		UserID:                   userID,
		State:                    state,
		EvaluatedAgainstRevision: snapshot.CurrentRevision,
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
	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	if stored != nil && stored.EvaluatedAgainstRevision == snapshot.CurrentRevision {
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
// be characterized without a database or model call.
func EvaluateDiagnosisFreshnessPolicy(
	analysis *model.DiagnosisAnalysisRecord,
	revisions []model.BodyStateRevision,
) (string, []DiagnosisFreshnessReason) {
	if analysis == nil || len(revisions) == 0 {
		return model.DiagnosisFreshnessFresh, []DiagnosisFreshnessReason{}
	}

	referencedFacts := map[string]struct{}{}
	referencedObservations := map[string]struct{}{}
	concerns := map[string]struct{}{}
	for _, candidate := range analysis.Candidates {
		if key := strings.TrimSpace(candidate.ConcernKey); key != "" && key != "general" {
			concerns[key] = struct{}{}
		}
		for _, id := range decodeStringList(candidate.BasisFactIDs) {
			referencedFacts[id] = struct{}{}
		}
		for _, id := range decodeStringList(candidate.BasisObservationIDs) {
			referencedObservations[id] = struct{}{}
		}
	}

	state := model.DiagnosisFreshnessFresh
	reasons := make([]DiagnosisFreshnessReason, 0)
	for _, revision := range revisions {
		var changes map[string]any
		_ = json.Unmarshal(revision.Changes, &changes)
		changeType := revision.ChangeType

		if strings.HasPrefix(changeType, "safety.") || changeType == "safety.changed" {
			state = model.DiagnosisFreshnessStale
			reasons = append(reasons, freshnessReason(revision, "safety_state_changed", "", "", "安全状态发生变化，旧分析需要重新审核。"))
			continue
		}

		switch changeType {
		case "fact.corrected":
			itemID := valueString(changes["corrected_fact_id"])
			if _, referenced := referencedFacts[itemID]; referenced {
				state = model.DiagnosisFreshnessStale
				reasons = append(reasons, freshnessReason(revision, "referenced_fact_corrected", itemID, mapConcern(changes["replacement"]), "分析引用的事实被纠正。"))
			} else if concernMatches(concerns, mapConcern(changes["replacement"])) {
				state = maxFreshness(state, model.DiagnosisFreshnessPotentiallyStale)
				reasons = append(reasons, freshnessReason(revision, "related_fact_corrected", itemID, mapConcern(changes["replacement"]), "同一关注区域的事实被纠正。"))
			}
		case "fact.temporal_changed", "fact.updated":
			itemID := valueString(changes["fact_id"])
			after := mapValue(changes["after"])
			concernKey := valueString(after["concern_key"])
			kind := valueString(after["kind"])
			lifecycle := valueString(after["lifecycle_state"])
			_, referenced := referencedFacts[itemID]
			if isSafetyKind(kind) {
				state = model.DiagnosisFreshnessStale
				reasons = append(reasons, freshnessReason(revision, "safety_fact_changed", itemID, concernKey, "安全相关事实发生变化。"))
			} else if referenced && (lifecycle == "inactive" || lifecycle == "resolved") {
				state = model.DiagnosisFreshnessStale
				reasons = append(reasons, freshnessReason(revision, "referenced_fact_no_longer_active", itemID, concernKey, "分析引用的事实已不再处于当前有效状态。"))
			} else if referenced || concernMatches(concerns, concernKey) {
				state = maxFreshness(state, model.DiagnosisFreshnessPotentiallyStale)
				reasons = append(reasons, freshnessReason(revision, "related_fact_changed", itemID, concernKey, "分析相关事实发生变化。"))
			}
		case "fact.added":
			fact := mapValue(changes["fact"])
			itemID := valueString(fact["id"])
			concernKey := valueString(fact["concern_key"])
			kind := valueString(fact["kind"])
			if isSafetyKind(kind) {
				state = model.DiagnosisFreshnessStale
				reasons = append(reasons, freshnessReason(revision, "new_safety_fact", itemID, concernKey, "新增了安全相关事实。"))
			} else if concernMatches(concerns, concernKey) {
				state = maxFreshness(state, model.DiagnosisFreshnessPotentiallyStale)
				reasons = append(reasons, freshnessReason(revision, "new_related_fact", itemID, concernKey, "同一关注区域出现了新的事实。"))
			}
		case "observation.updated":
			itemID := valueString(changes["observation_id"])
			after := mapValue(changes["after"])
			concernKey := valueString(after["concern_key"])
			_, referenced := referencedObservations[itemID]
			if referenced || concernMatches(concerns, concernKey) {
				state = maxFreshness(state, model.DiagnosisFreshnessPotentiallyStale)
				reasons = append(reasons, freshnessReason(revision, "related_observation_changed", itemID, concernKey, "分析相关观察结果发生变化。"))
			}
		case "observation.added":
			observation := mapValue(changes["observation"])
			itemID := valueString(observation["id"])
			concernKey := valueString(observation["concern_key"])
			if concernMatches(concerns, concernKey) {
				state = maxFreshness(state, model.DiagnosisFreshnessPotentiallyStale)
				reasons = append(reasons, freshnessReason(revision, "new_related_observation", itemID, concernKey, "同一关注区域出现了新的观察结果。"))
			}
		}
	}
	return state, reasons
}

func freshnessReason(revision model.BodyStateRevision, code, itemID, concernKey, message string) DiagnosisFreshnessReason {
	return DiagnosisFreshnessReason{
		Code: code, Revision: revision.Revision, ChangeType: revision.ChangeType,
		ItemID: itemID, ConcernKey: concernKey, Message: message,
	}
}

func decodeStringList(raw datatypes.JSON) []string {
	var values []string
	_ = json.Unmarshal(raw, &values)
	return values
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func mapConcern(value any) string {
	return valueString(mapValue(value)["concern_key"])
}

func valueString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func concernMatches(concerns map[string]struct{}, concernKey string) bool {
	if concernKey == "" || concernKey == "general" || len(concerns) == 0 {
		return false
	}
	_, ok := concerns[concernKey]
	return ok
}

func isSafetyKind(kind string) bool {
	return kind == "red_flags" || kind == "safety_finding"
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
