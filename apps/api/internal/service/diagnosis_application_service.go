package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DiagnosisApplicationError is a transport-neutral use-case failure. The public
// HTTP adapter owns the final status-code mapping; application code owns stable
// machine-readable failure semantics.
type DiagnosisApplicationError struct {
	Code    string
	Message string
	Cause   error
}

func (e *DiagnosisApplicationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func diagnosisApplicationError(code, message string, cause error) *DiagnosisApplicationError {
	return &DiagnosisApplicationError{Code: code, Message: message, Cause: cause}
}

// DiagnosisApplicationService owns the BodyState -> governed DiagnosisAnalysis
// use case. It intentionally contains no Gin, HTTP DTO, or generated OpenAPI
// types; transports map validated requests into this application boundary.
type DiagnosisApplicationService struct {
	consultations *ConsultationService
	profiles      *ProfileService
	ai            *AIClient
	outputReviews *OutputReviewService
	bodyState     *BodyStateService
	analyses      *DiagnosisAnalysisService
	freshness     *DiagnosisFreshnessService
	deployment    *AgentDeploymentPolicy
	replay        *DiagnosisReplayService
	rollout       *DiagnosisRolloutService
}

func NewDiagnosisApplicationService(
	consultations *ConsultationService,
	profiles *ProfileService,
	ai *AIClient,
	outputReviews *OutputReviewService,
	bodyState *BodyStateService,
	analyses *DiagnosisAnalysisService,
	freshness *DiagnosisFreshnessService,
	deployment *AgentDeploymentPolicy,
	replay *DiagnosisReplayService,
	rollout *DiagnosisRolloutService,
) *DiagnosisApplicationService {
	return &DiagnosisApplicationService{
		consultations: consultations,
		profiles:      profiles,
		ai:            ai,
		outputReviews: outputReviews,
		bodyState:     bodyState,
		analyses:      analyses,
		freshness:     freshness,
		deployment:    deployment,
		replay:        replay,
		rollout:       rollout,
	}
}

// Analyze verifies consultation ownership, freezes the exact BodyState revision,
// applies the Go safety/decision authority, invokes the configured Diagnosis
// Agent when allowed, persists the immutable analysis and returns the public
// projection used by all transports.
func (s *DiagnosisApplicationService) Analyze(
	ctx context.Context,
	userID uuid.UUID,
	conversationID uuid.UUID,
) (map[string]any, *DiagnosisApplicationError) {
	if s == nil || s.consultations == nil {
		return nil, diagnosisApplicationError("DIAGNOSIS_DOMAIN_UNAVAILABLE", "BodyState-backed diagnosis services are not configured", nil)
	}
	session, err := s.consultations.GetConsultation(ctx, conversationID, userID)
	if err != nil {
		return nil, diagnosisApplicationError("INTERNAL_ERROR", "failed to get consultation", err)
	}
	if session == nil {
		return nil, diagnosisApplicationError("NOT_FOUND", "consultation not found", nil)
	}
	if s.bodyState == nil || s.analyses == nil {
		return nil, diagnosisApplicationError("DIAGNOSIS_DOMAIN_UNAVAILABLE", "BodyState-backed diagnosis services are not configured", nil)
	}

	profileJSON := json.RawMessage(`{}`)
	if s.profiles != nil {
		if profile, profileErr := s.profiles.GetProfile(ctx, userID); profileErr == nil && profile != nil {
			if encoded, marshalErr := json.Marshal(profile); marshalErr == nil {
				profileJSON = encoded
			}
		}
	}
	return s.analyzeFromBodyState(ctx, userID, conversationID, profileJSON)
}

func (s *DiagnosisApplicationService) analyzeFromBodyState(
	ctx context.Context,
	userID uuid.UUID,
	conversationID uuid.UUID,
	profileJSON json.RawMessage,
) (map[string]any, *DiagnosisApplicationError) {
	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 50)
	if err != nil {
		return nil, diagnosisApplicationError("INTERNAL_ERROR", "failed to load body state", err)
	}
	if snapshot.CurrentRevision == 0 || (len(snapshot.Facts) == 0 && len(snapshot.Observations) == 0) {
		return nil, diagnosisApplicationError("BODY_STATE_NOT_READY", "body state does not yet contain enough structured information for diagnosis", nil)
	}
	if s.deployment == nil {
		return nil, diagnosisApplicationError("AGENT_DEPLOYMENT_POLICY_UNAVAILABLE", "Diagnosis Agent deployment policy is not configured", nil)
	}

	route := s.deployment.SelectDiagnosisRoute(userID.String())
	configurationID := route.ServedConfigurationID
	policyRevision := route.ServedDecisionPolicyRevision
	bodyStateJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, diagnosisApplicationError("INTERNAL_ERROR", "failed to encode body state", err)
	}
	historyJSON, _ := json.Marshal(snapshot.RecentRevisions)
	replayInput, err := EncodeDiagnosisReplayInput(snapshot.CurrentRevision, bodyStateJSON, historyJSON, profileJSON)
	if err != nil {
		return nil, diagnosisApplicationError("INTERNAL_ERROR", "failed to freeze Diagnosis replay input", err)
	}

	if blocked := s.preAgentSafetyBlock(snapshot, configurationID, policyRevision, route); blocked != nil {
		analysis, persistErr := s.analyses.PersistAIResultWithReplayInput(ctx, userID, snapshot.CurrentRevision, blocked, replayInput)
		if persistErr != nil {
			return nil, diagnosisApplicationError("INTERNAL_ERROR", "failed to persist diagnosis safety state", persistErr)
		}
		s.observeRollout(ctx, userID, analysis, route)
		return s.publicPayload(ctx, userID, analysis), nil
	}

	if s.ai == nil {
		return nil, diagnosisApplicationError("AI_SERVICE_ERROR", "failed to analyze diagnosis", fmt.Errorf("Diagnosis AI client is not configured"))
	}
	result, err := s.ai.AnalyzeDiagnosis(ctx, DiagnosisRequest{
		UserID: userID.String(), ConfigurationID: configurationID,
		BodyStateRevision: snapshot.CurrentRevision, BodyState: bodyStateJSON,
		RelevantHistory: historyJSON, Profile: profileJSON,
	})
	if err != nil {
		log.Printf("AI diagnosis analysis failed for BodyState R%d user %s: %v", snapshot.CurrentRevision, userID, err)
		return nil, diagnosisApplicationError("AI_SERVICE_ERROR", "failed to analyze diagnosis", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, diagnosisApplicationError("INVALID_AI_RESPONSE", "diagnosis response was not valid JSON", err)
	}
	if !diagnosisApplicationConfigurationMatches(parsed, configurationID) {
		return nil, diagnosisApplicationError("INVALID_AGENT_CONFIGURATION", "diagnosis response did not match the selected Agent configuration", nil)
	}
	parsed["rollout_provenance"] = route
	result, err = json.Marshal(parsed)
	if err != nil {
		return nil, diagnosisApplicationError("INTERNAL_ERROR", "failed to encode Diagnosis rollout provenance", err)
	}

	if redFlags, ok := parsed["red_flags"].(map[string]any); ok {
		safetyPayload, marshalErr := json.Marshal(redFlags)
		if marshalErr != nil {
			return nil, diagnosisApplicationError("INVALID_AI_RESPONSE", "diagnosis safety response was not valid", marshalErr)
		}
		if err := s.bodyState.RecordSafetyEvent(ctx, userID, safetyPayload); err != nil {
			log.Printf("failed to promote Diagnosis safety signal into BodyState for user %s: %v", userID, err)
			return nil, diagnosisApplicationError("SAFETY_STATE_PERSISTENCE_FAILED", "failed to persist diagnosis safety state", err)
		}
	}

	s.recordGovernedOutput(ctx, "diagnosis", &userID, &conversationID, nil, parsed, result)
	if policyRevision == DiagnosisDecisionPolicyV1 {
		decision := EvaluateDiagnosisDecision(policyRevision, snapshot.SafetyState, parsed)
		parsed = ApplyDiagnosisDecision(parsed, decision)
		result, err = json.Marshal(parsed)
		if err != nil {
			return nil, diagnosisApplicationError("INTERNAL_ERROR", "failed to encode Diagnosis decision", err)
		}
	} else if governance, ok := parsed["governance"].(map[string]any); ok {
		if verdict, _ := governance["verdict"].(string); verdict == "rejected" {
			s.observeRolloutFrozen(ctx, userID, replayInput, result, route)
			// Preserve the characterized pre-envelope behavior: rejected legacy
			// output is returned but is not frozen as a DiagnosisAnalysis record.
			return parsed, nil
		}
	}

	if normalized, evidenceErr := s.persistEvidence(ctx, userID, parsed); evidenceErr == nil {
		result = normalized
		_ = json.Unmarshal(result, &parsed)
	} else {
		log.Printf("failed to persist Diagnosis evidence for user %s: %v", userID, evidenceErr)
	}

	analysis, err := s.analyses.PersistAIResultWithReplayInput(ctx, userID, snapshot.CurrentRevision, result, replayInput)
	if err != nil {
		log.Printf("failed to persist diagnosis analysis for user %s BodyState R%d: %v", userID, snapshot.CurrentRevision, err)
		return nil, diagnosisApplicationError("INTERNAL_ERROR", "failed to persist diagnosis analysis", err)
	}
	s.observeRollout(ctx, userID, analysis, route)
	if hypothesisErr := s.commitHypotheses(ctx, userID, analysis); hypothesisErr != nil {
		log.Printf("failed to project Diagnosis hypotheses for analysis %s: %v", analysis.ID, hypothesisErr)
	}

	payload := s.publicPayload(ctx, userID, analysis)
	return payload, nil
}

func (s *DiagnosisApplicationService) preAgentSafetyBlock(
	snapshot *BodyStateSnapshot,
	configurationID string,
	policyRevision string,
	route DiagnosisRouteSelection,
) json.RawMessage {
	if policyRevision == DiagnosisDecisionPolicyV1 {
		probe := map[string]any{
			"status":     "completed",
			"candidates": []any{map[string]any{"name": "preflight", "confidence": "n/a"}},
			"governance": map[string]any{"verdict": "accepted"},
		}
		decision := EvaluateDiagnosisDecision(policyRevision, snapshot.SafetyState, probe)
		if decision.Outcome != DiagnosisBlock {
			return nil
		}
		blocked := ApplyDiagnosisDecision(safetyBlockedDiagnosisPayload(snapshot.SafetyState, configurationID, policyRevision, route), decision)
		encoded, _ := json.Marshal(blocked)
		return encoded
	}

	var safetyState struct {
		HasRedFlags bool   `json:"has_red_flags"`
		Status      string `json:"status"`
	}
	_ = json.Unmarshal(snapshot.SafetyState, &safetyState)
	if !safetyState.HasRedFlags || safetyState.Status != "requires_review" {
		return nil
	}
	encoded, _ := json.Marshal(safetyBlockedDiagnosisPayload(snapshot.SafetyState, configurationID, policyRevision, route))
	return encoded
}

func safetyBlockedDiagnosisPayload(
	safetyState json.RawMessage,
	configurationID string,
	policyRevision string,
	route DiagnosisRouteSelection,
) map[string]any {
	return map[string]any{
		"status": "safety_blocked", "scope": "full_body",
		"summary":    "当前身体状态包含需要优先处理的安全信号，暂不生成普通可能性候选。",
		"candidates": []any{}, "cross_concern_patterns": []any{}, "information_gaps": []any{},
		"safety_summary": json.RawMessage(safetyState), "citations": []any{},
		"agent_configuration": map[string]any{
			"id": configurationID, "role": "diagnosis", "decision_policy_revision": policyRevision,
		},
		"execution_provenance": map[string]any{
			"status": "bypassed", "runtime": "go", "reason": "go_pre_agent_safety_gate",
		},
		"rollout_provenance": route,
		"governance": map[string]any{
			"kind": "diagnosis", "verdict": "rejected",
			"reasons": []string{"active_body_state_safety_concern"}, "issues": []any{},
		},
	}
}

func (s *DiagnosisApplicationService) publicPayload(
	ctx context.Context,
	userID uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
) map[string]any {
	payload := s.analyses.PublicPayload(analysis)
	if s.freshness != nil {
		if freshness, err := s.freshness.GetOrEvaluate(ctx, userID, analysis); err == nil {
			payload["freshness"] = freshness
		}
	}
	return payload
}

func diagnosisApplicationConfigurationMatches(payload map[string]any, expectedID string) bool {
	configuration, ok := payload["agent_configuration"].(map[string]any)
	if !ok {
		return false
	}
	id, idOK := configuration["id"].(string)
	role, roleOK := configuration["role"].(string)
	return idOK && roleOK && id == expectedID && role == "diagnosis"
}

func (s *DiagnosisApplicationService) observeRolloutFrozen(
	ctx context.Context,
	userID uuid.UUID,
	replayInput json.RawMessage,
	baseline json.RawMessage,
	route DiagnosisRouteSelection,
) {
	if route.ShadowConfigurationID == "" || s.replay == nil || s.rollout == nil {
		return
	}
	report, err := s.replay.CounterfactualFrozen(ctx, userID, replayInput, baseline, route.ServedConfigurationID, route.ShadowConfigurationID)
	if recordErr := s.rollout.RecordComparison(ctx, route, uuid.Nil, report, err); recordErr != nil {
		log.Printf("failed to persist transient Diagnosis rollout observation: %v", recordErr)
	}
	if err != nil {
		log.Printf("Diagnosis %s transient comparison failed: %v", route.Stage, err)
	}
}

func (s *DiagnosisApplicationService) observeRollout(
	ctx context.Context,
	userID uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
	route DiagnosisRouteSelection,
) {
	if route.ShadowConfigurationID == "" || s.replay == nil || s.rollout == nil {
		return
	}
	report, err := s.replay.CounterfactualReplay(ctx, userID, analysis.ID, route.ShadowConfigurationID)
	if recordErr := s.rollout.RecordComparison(ctx, route, analysis.ID, report, err); recordErr != nil {
		log.Printf("failed to persist Diagnosis rollout observation for analysis %s: %v", analysis.ID, recordErr)
	}
	if err != nil {
		log.Printf("Diagnosis %s comparison failed for analysis %s: %v", route.Stage, analysis.ID, err)
	}
}

func (s *DiagnosisApplicationService) persistEvidence(
	ctx context.Context,
	userID uuid.UUID,
	parsed map[string]any,
) (json.RawMessage, error) {
	if s.bodyState == nil {
		return json.Marshal(parsed)
	}
	rawCitations, _ := parsed["citations"].([]any)
	if len(rawCitations) == 0 {
		return json.Marshal(parsed)
	}
	identityMap := map[string]string{}
	for index, rawCitation := range rawCitations {
		citation, ok := rawCitation.(map[string]any)
		if !ok {
			continue
		}
		sourceType := firstDiagnosisApplicationString(citation, "source_type", "type", "source")
		if sourceType == "" {
			sourceType = "knowledge"
		}
		sourceKey := firstDiagnosisApplicationString(citation, "source_key", "id", "source_id", "url", "uri")
		if sourceKey == "" {
			encoded, _ := json.Marshal(citation)
			digest := sha256.Sum256(encoded)
			sourceKey = "citation:" + hex.EncodeToString(digest[:12])
		}
		version := firstDiagnosisApplicationString(citation, "source_version", "version", "updated_at")
		metadata, _ := json.Marshal(citation)
		stored, err := s.bodyState.UpsertEvidence(ctx, userID, model.BodyStateEvidence{
			SourceType: sourceType, SourceKey: sourceKey, SourceVersion: version,
			Title:    firstDiagnosisApplicationString(citation, "title", "name"),
			Summary:  firstDiagnosisApplicationString(citation, "summary", "content"),
			Excerpt:  firstDiagnosisApplicationString(citation, "excerpt", "chunk", "text"),
			Metadata: datatypes.JSON(metadata),
		})
		if err != nil {
			return nil, err
		}
		storedID := stored.ID.String()
		citation["evidence_id"] = storedID
		citation["source_key"] = sourceKey
		rawCitations[index] = citation
		for _, key := range []string{sourceKey, firstDiagnosisApplicationString(citation, "id"), firstDiagnosisApplicationString(citation, "source_id"), firstDiagnosisApplicationString(citation, "url"), storedID} {
			if key != "" {
				identityMap[key] = storedID
			}
		}
	}
	parsed["citations"] = rawCitations
	if candidates, ok := parsed["candidates"].([]any); ok {
		for index, rawCandidate := range candidates {
			candidate, ok := rawCandidate.(map[string]any)
			if !ok {
				continue
			}
			if references, ok := candidate["supporting_evidence_ids"].([]any); ok {
				normalized := make([]string, 0, len(references))
				for _, rawReference := range references {
					reference := strings.TrimSpace(fmt.Sprint(rawReference))
					if mapped := identityMap[reference]; mapped != "" {
						normalized = append(normalized, mapped)
					} else if _, err := uuid.Parse(reference); err == nil {
						normalized = append(normalized, reference)
					}
				}
				candidate["supporting_evidence_ids"] = normalized
			}
			candidates[index] = candidate
		}
		parsed["candidates"] = candidates
	}
	return json.Marshal(parsed)
}

func (s *DiagnosisApplicationService) commitHypotheses(
	ctx context.Context,
	userID uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
) error {
	if s.bodyState == nil || analysis == nil {
		return nil
	}
	for _, candidate := range analysis.Candidates {
		confidence := candidate.Confidence
		_, _, err := s.bodyState.AddDiagnosisHypothesis(ctx, userID, model.BodyStateHypothesis{
			ConcernKey: candidate.ConcernKey, Statement: candidate.Name,
			LifecycleState: "active", Confidence: &confidence,
			SupportingFactIDs:        candidate.BasisFactIDs,
			SupportingObservationIDs: candidate.BasisObservationIDs,
			SupportingEvidenceIDs:    candidate.SupportingEvidenceIDs,
			CounterevidenceIDs:       candidate.CounterevidenceIDs,
			SourceAnalysisID:         &analysis.ID,
			Provenance: datatypes.JSON(mustDiagnosisApplicationJSON(map[string]any{
				"source_type": "diagnosis_analysis", "analysis_id": analysis.ID,
				"candidate_id": candidate.ID, "reasoning_summary": candidate.ReasoningSummary,
			})),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func firstDiagnosisApplicationString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(values[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func mustDiagnosisApplicationJSON(value any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func (s *DiagnosisApplicationService) recordGovernedOutput(
	ctx context.Context,
	outputType string,
	userID, conversationID, jobID *uuid.UUID,
	parsed map[string]any,
	raw []byte,
) {
	if s.outputReviews == nil {
		return
	}
	verdict := "unknown"
	issues := datatypes.JSON("[]")
	var validated datatypes.JSON
	if governance, ok := parsed["governance"].(map[string]any); ok {
		if value, ok := governance["verdict"].(string); ok && value != "" {
			verdict = value
		}
		if rawIssues, ok := governance["issues"]; ok {
			if encoded, err := json.Marshal(rawIssues); err == nil {
				issues = datatypes.JSON(encoded)
			}
		}
	}
	if verdict == "accepted" || verdict == "degraded" {
		safe := make(map[string]any, len(parsed))
		for key, value := range parsed {
			if key == "governance" || key == "safety_fallback" {
				continue
			}
			safe[key] = value
		}
		if encoded, err := json.Marshal(safe); err == nil {
			validated = datatypes.JSON(encoded)
		}
	}
	s.outputReviews.RecordReview(ctx, outputType, verdict, userID, nil, jobID, conversationID, issues, validated, datatypes.JSON(raw))
}
