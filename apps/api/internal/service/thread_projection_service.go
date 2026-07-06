package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type threadProjectionConversationRepo interface {
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error)
}

type threadProjectionConsultationRepo interface {
	GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error)
}

type threadProjectionMessageRepo interface {
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Message, error)
}

type threadProjectionInteractionRepo interface {
	ListPendingByConversation(ctx context.Context, conversationID uuid.UUID) ([]model.AgentInteraction, error)
	ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]model.AgentInteraction, error)
}

type threadProjectionRuntimeEventSource interface {
	ListAllRunEvents(ctx context.Context, conversationID, runID uuid.UUID) ([]model.RuntimeEvent, error)
	ListConversationEvents(ctx context.Context, conversationID uuid.UUID) ([]model.RuntimeEvent, error)
}

type threadProjectionRepo interface {
	UpsertSnapshot(
		ctx context.Context,
		projection *model.ThreadProjection,
		messages []model.ThreadProjectionMessage,
		toolCalls []model.ThreadProjectionToolCall,
	) error
	GetByConversationID(
		ctx context.Context,
		conversationID, userID uuid.UUID,
	) (*model.ThreadProjection, []model.ThreadProjectionMessage, []model.ThreadProjectionToolCall, error)
}

// ThreadProjectionService materializes and serves the durable consultation thread read model.
// It is intentionally a mixed projection: normalized business tables provide
// conversation/session/message state, while the Runtime Event Log supplies
// replayable run events and tool activity. It is not pure event sourcing.
type ThreadProjectionService struct {
	conversationRepo threadProjectionConversationRepo
	consultationRepo threadProjectionConsultationRepo
	messageRepo      threadProjectionMessageRepo
	interactionRepo  threadProjectionInteractionRepo
	runtimeEventRepo threadProjectionRuntimeEventSource
	repo             threadProjectionRepo
}

// NewThreadProjectionService creates a new ThreadProjectionService.
func NewThreadProjectionService(
	conversationRepo threadProjectionConversationRepo,
	consultationRepo threadProjectionConsultationRepo,
	messageRepo threadProjectionMessageRepo,
	interactionRepo threadProjectionInteractionRepo,
	runtimeEventRepo threadProjectionRuntimeEventSource,
	repo threadProjectionRepo,
) *ThreadProjectionService {
	return &ThreadProjectionService{
		conversationRepo: conversationRepo,
		consultationRepo: consultationRepo,
		messageRepo:      messageRepo,
		interactionRepo:  interactionRepo,
		runtimeEventRepo: runtimeEventRepo,
		repo:             repo,
	}
}

// RefreshAndGetThread materializes the latest durable thread projection and then reads it back.
func (s *ThreadProjectionService) RefreshAndGetThread(
	ctx context.Context,
	conversationID, userID uuid.UUID,
) (
	*model.ThreadProjection,
	[]model.ThreadProjectionMessage,
	[]model.ThreadProjectionToolCall,
	*uuid.UUID,
	[]model.RuntimeEvent,
	error,
) {
	conversation, err := s.conversationRepo.GetByID(ctx, conversationID, userID)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("get conversation: %w", err)
	}
	if conversation == nil {
		return nil, nil, nil, nil, nil, nil
	}

	session, err := s.consultationRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("get consultation session: %w", err)
	}
	if session == nil {
		return nil, nil, nil, nil, nil, nil
	}

	messages, err := s.messageRepo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("list messages: %w", err)
	}

	pendingInteractions, err := s.interactionRepo.ListPendingByConversation(ctx, conversationID)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("list pending interactions: %w", err)
	}
	allInteractions, err := s.interactionRepo.ListByConversation(ctx, conversationID)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("list interaction history: %w", err)
	}

	conversationEvents, err := s.runtimeEventRepo.ListConversationEvents(ctx, conversationID)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("list conversation runtime events: %w", err)
	}

	projection := buildThreadProjection(conversation, session, messages, pendingInteractions, allInteractions)
	projectedMessages := buildThreadProjectionMessages(messages)
	projectedToolCalls := buildThreadProjectionToolCallsFromEvents(conversationEvents)
	if err := s.repo.UpsertSnapshot(ctx, projection, projectedMessages, projectedToolCalls); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("upsert thread projection: %w", err)
	}

	storedProjection, storedMessages, storedToolCalls, err := s.repo.GetByConversationID(ctx, conversationID, userID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	activeTurnRunID := selectActiveTurnRunID(conversation.ActiveRunID, pendingInteractions)
	activeTurnEvents, err := s.loadActiveTurnEvents(ctx, conversationID, activeTurnRunID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return storedProjection, storedMessages, storedToolCalls, activeTurnRunID, activeTurnEvents, nil
}

func selectActiveTurnRunID(activeRunID *uuid.UUID, pendingInteractions []model.AgentInteraction) *uuid.UUID {
	var latestPending *model.AgentInteraction
	for index := range pendingInteractions {
		interaction := &pendingInteractions[index]
		if interaction.Status != "pending" {
			continue
		}
		if latestPending == nil || interaction.CreatedAt.After(latestPending.CreatedAt) {
			latestPending = interaction
		}
	}
	if latestPending != nil {
		runID := latestPending.RunID
		return &runID
	}
	if activeRunID == nil {
		return nil
	}
	runID := *activeRunID
	return &runID
}

func (s *ThreadProjectionService) loadActiveTurnEvents(
	ctx context.Context,
	conversationID uuid.UUID,
	runID *uuid.UUID,
) ([]model.RuntimeEvent, error) {
	if runID == nil || s.runtimeEventRepo == nil {
		return nil, nil
	}

	events, err := s.runtimeEventRepo.ListAllRunEvents(ctx, conversationID, *runID)
	if err != nil {
		return nil, fmt.Errorf("list active turn events: %w", err)
	}
	return events, nil
}

func buildThreadProjection(
	conversation *model.Conversation,
	session *model.ConsultationSession,
	messages []model.Message,
	pendingInteractions []model.AgentInteraction,
	allInteractions []model.AgentInteraction,
) *model.ThreadProjection {
	return &model.ThreadProjection{
		ConversationID:        conversation.ID,
		UserID:                conversation.UserID,
		Title:                 conversation.Title,
		TitleStatus:           conversation.TitleStatus,
		Status:                conversation.Status,
		Pinned:                conversation.Pinned,
		PinnedAt:              conversation.PinnedAt,
		DefaultModel:          conversation.DefaultModel,
		ActiveRunID:           conversation.ActiveRunID,
		LastMessageAt:         conversation.LastMessageAt,
		Metadata:              normalizeJSON(conversation.Metadata, `{}`),
		Phase:                 session.Phase,
		ExtractedInfo:         normalizeJSON(session.ExtractedInfo, `[]`),
		HealthFeatures:        marshalJSON(buildHealthFeatures(session, messages, allInteractions), `{}`),
		Diagnosis:             normalizeNullableJSON(session.Diagnosis),
		TreatmentPlan:         normalizeNullableJSON(session.TreatmentPlan),
		PendingInteractions:   marshalJSON(pendingInteractions, `[]`),
		InteractionHistory:    marshalJSON(buildInteractionHistory(allInteractions), `[]`),
		ConversationCreatedAt: conversation.CreatedAt,
		ConversationUpdatedAt: conversation.UpdatedAt,
		SessionCreatedAt:      session.CreatedAt,
		SessionUpdatedAt:      session.UpdatedAt,
		EndedAt:               session.EndedAt,
		RefreshedAt:           time.Now().UTC(),
	}
}

type healthFeatureItem struct {
	Label     string                 `json:"label"`
	BodyPart  string                 `json:"body_part,omitempty"`
	Value     string                 `json:"value,omitempty"`
	Details   string                 `json:"details,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Confirmed bool                   `json:"confirmed,omitempty"`
	Metadata  map[string]any         `json:"metadata,omitempty"`
}

type healthFeatures struct {
	PostureFindings     []healthFeatureItem `json:"posture_findings"`
	Discomforts         []healthFeatureItem `json:"discomforts"`
	NegativeFindings    []healthFeatureItem `json:"negative_findings"`
	MovementLimitations []healthFeatureItem `json:"movement_limitations"`
	RedFlags            []healthFeatureItem `json:"red_flags"`
	UserAnswers         []healthFeatureItem `json:"user_answers"`
}

func buildHealthFeatures(
	session *model.ConsultationSession,
	messages []model.Message,
	interactions []model.AgentInteraction,
) healthFeatures {
	features := mergeHealthFeatures(
		parseStoredHealthFeatures(session.HealthFeatures),
		deriveHealthFeatures(session.ExtractedInfo, messages, interactions),
	)
	return normalizeHealthFeatures(features)
}

func parseStoredHealthFeatures(raw datatypes.JSON) healthFeatures {
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return healthFeatures{}
	}
	var parsed healthFeatures
	_ = json.Unmarshal(raw, &parsed)
	return parsed
}

func deriveHealthFeatures(
	extractedInfoRaw datatypes.JSON,
	messages []model.Message,
	interactions []model.AgentInteraction,
) healthFeatures {
	var features healthFeatures
	var extractedInfo []map[string]any
	_ = json.Unmarshal(extractedInfoRaw, &extractedInfo)

	for _, item := range extractedInfo {
		bodyPart := stringify(item["body_part"])
		symptomType := stringify(item["symptom_type"])
		label := symptomType
		if label == "" {
			label = bodyPart
		}
		if label == "" {
			continue
		}
		features.Discomforts = append(features.Discomforts, healthFeatureItem{
			Label:     label,
			BodyPart:  bodyPart,
			Value:     stringify(item["severity"]),
			Details:   joinNonEmpty("，", stringify(item["duration"]), stringify(item["trigger"]), stringify(item["relief"])),
			Source:    "extracted_info",
			Confirmed: boolValue(item["confirmed"]),
		})
	}

	for _, message := range messages {
		if message.Role != "user" {
			continue
		}
		text := strings.TrimSpace(message.ContentText)
		if text == "" {
			text = extractTextFromParts(message.Parts)
		}
		for _, keyword := range []string{"头前移", "圆肩", "骨盆前倾", "驼背", "高低肩"} {
			if strings.Contains(text, keyword) {
				features.PostureFindings = append(features.PostureFindings, healthFeatureItem{
					Label:   keyword,
					Details: text,
					Source:  "user_message",
				})
			}
		}
	}

	for _, interaction := range interactions {
		if interaction.Status == "pending" || len(interaction.Question) == 0 {
			continue
		}
		questionText, questionContext := parseInteractionQuestion(interaction.Question)
		answerText := parseInteractionAnswer(interaction.Answer)
		if answerText == "" {
			continue
		}

		features.UserAnswers = append(features.UserAnswers, healthFeatureItem{
			Label:   questionText,
			Value:   answerText,
			Details: questionContext,
			Source:  "ask_user",
		})

		normalizedAnswer := normalizeAnswer(answerText)
		if normalizedAnswer == "negative" && containsAny(questionText+questionContext, "不适", "疼痛", "酸痛", "麻木") {
			features.NegativeFindings = append(features.NegativeFindings, healthFeatureItem{
				Label:   "未报告相关不适",
				Value:   answerText,
				Details: questionText,
				Source:  "ask_user",
			})
		}
		if normalizedAnswer == "positive" && containsAny(questionText+questionContext, "受限", "活动", "转头", "弯腰") {
			features.MovementLimitations = append(features.MovementLimitations, healthFeatureItem{
				Label:   "活动受限",
				Value:   answerText,
				Details: questionText,
				Source:  "ask_user",
			})
		}
		if normalizedAnswer == "positive" && containsAny(questionText+questionContext, "麻木", "外伤", "发热", "放射痛") {
			features.RedFlags = append(features.RedFlags, healthFeatureItem{
				Label:   "需进一步关注",
				Value:   answerText,
				Details: questionText,
				Source:  "ask_user",
			})
		}
	}

	return features
}

func mergeHealthFeatures(primary, secondary healthFeatures) healthFeatures {
	primary.PostureFindings = mergeFeatureLists(primary.PostureFindings, secondary.PostureFindings)
	primary.Discomforts = mergeFeatureLists(primary.Discomforts, secondary.Discomforts)
	primary.NegativeFindings = mergeFeatureLists(primary.NegativeFindings, secondary.NegativeFindings)
	primary.MovementLimitations = mergeFeatureLists(primary.MovementLimitations, secondary.MovementLimitations)
	primary.RedFlags = mergeFeatureLists(primary.RedFlags, secondary.RedFlags)
	primary.UserAnswers = mergeFeatureLists(primary.UserAnswers, secondary.UserAnswers)
	return primary
}

func normalizeHealthFeatures(features healthFeatures) healthFeatures {
	if features.PostureFindings == nil {
		features.PostureFindings = []healthFeatureItem{}
	}
	if features.Discomforts == nil {
		features.Discomforts = []healthFeatureItem{}
	}
	if features.NegativeFindings == nil {
		features.NegativeFindings = []healthFeatureItem{}
	}
	if features.MovementLimitations == nil {
		features.MovementLimitations = []healthFeatureItem{}
	}
	if features.RedFlags == nil {
		features.RedFlags = []healthFeatureItem{}
	}
	if features.UserAnswers == nil {
		features.UserAnswers = []healthFeatureItem{}
	}
	return features
}

func mergeFeatureLists(primary, secondary []healthFeatureItem) []healthFeatureItem {
	merged := append([]healthFeatureItem{}, primary...)
	seen := make(map[string]struct{}, len(primary))
	for _, item := range primary {
		seen[featureKey(item)] = struct{}{}
	}
	for _, item := range secondary {
		key := featureKey(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, item)
	}
	return merged
}

func featureKey(item healthFeatureItem) string {
	return strings.Join([]string{item.Label, item.BodyPart, item.Value, item.Details, item.Source}, "|")
}

func extractTextFromParts(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part["type"] == "text" {
			texts = append(texts, stringify(part["text"]))
		}
	}
	return strings.TrimSpace(strings.Join(texts, " "))
}

func parseInteractionQuestion(raw datatypes.JSON) (string, string) {
	if len(raw) == 0 {
		return "", ""
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	return stringify(payload["question"]), stringify(payload["context"])
}

func parseInteractionAnswer(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return strings.Trim(string(raw), `"`)
	}
	switch value := parsed.(type) {
	case string:
		return value
	case map[string]any:
		if text := stringify(value["text"]); text != "" {
			return text
		}
		if option := stringify(value["value"]); option != "" {
			return option
		}
	}
	return stringify(parsed)
}

func normalizeAnswer(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "无", "没有", "否", "no", "none":
		return "negative"
	case "有", "是", "会", "yes":
		return "positive"
	default:
		return "unknown"
	}
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.RawMessage:
		return strings.TrimSpace(string(typed))
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func joinNonEmpty(separator string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, separator)
}

func buildInteractionHistory(interactions []model.AgentInteraction) []model.AgentInteraction {
	history := make([]model.AgentInteraction, 0, len(interactions))
	for _, interaction := range interactions {
		if interaction.Status == "pending" {
			continue
		}
		history = append(history, interaction)
	}
	return history
}

func buildThreadProjectionMessages(messages []model.Message) []model.ThreadProjectionMessage {
	projected := make([]model.ThreadProjectionMessage, 0, len(messages))
	for _, message := range messages {
		projected = append(projected, model.ThreadProjectionMessage{
			MessageID:          message.ID,
			ConversationID:     message.ConversationID,
			TurnID:             message.TurnID,
			RunID:              message.RunID,
			ParentMessageID:    message.ParentMessageID,
			Seq:                message.Seq,
			Role:               message.Role,
			Status:             message.Status,
			Parts:              normalizeJSON(message.Parts, `[]`),
			ContentText:        message.ContentText,
			Model:              message.Model,
			Provider:           message.Provider,
			ProviderMessageID:  message.ProviderMessageID,
			ProviderResponseID: message.ProviderResponseID,
			InputTokens:        message.InputTokens,
			OutputTokens:       message.OutputTokens,
			TotalTokens:        message.TotalTokens,
			Error:              normalizeNullableJSON(message.Error),
			Metadata:           normalizeJSON(message.Metadata, `{}`),
			CreatedAt:          message.CreatedAt,
			UpdatedAt:          message.UpdatedAt,
		})
	}
	return projected
}

func buildThreadProjectionToolCallsFromEvents(events []model.RuntimeEvent) []model.ThreadProjectionToolCall {
	projectedByID := make(map[string]*model.ThreadProjectionToolCall)
	orderedIDs := make([]string, 0)

	for _, event := range events {
		switch event.Type {
		case "tool.call":
			ids, payload, ok := parseToolCallRuntimeEvent(event)
			if !ok || ids.ToolCallID == "" {
				continue
			}

			entry, exists := projectedByID[ids.ToolCallID]
			if !exists {
				entry = &model.ThreadProjectionToolCall{
					ToolCallID:     ids.ToolCallID,
					ConversationID: event.ConversationID,
					RunID:          event.RunID,
					MessageID:      parseOptionalUUID(ids.MessageID),
					ToolName:       payload.Tool,
					Arguments:      normalizeJSON(datatypes.JSON(payload.Args), `{}`),
					Status:         "running",
					CreatedAt:      event.CreatedAt,
					StartedAt:      event.CreatedAt,
					Metadata:       datatypes.JSON(`{}`),
				}
				projectedByID[ids.ToolCallID] = entry
				orderedIDs = append(orderedIDs, ids.ToolCallID)
				continue
			}

			entry.ToolName = payload.Tool
			entry.Arguments = normalizeJSON(datatypes.JSON(payload.Args), `{}`)
			entry.Status = "running"
			if entry.MessageID == nil {
				entry.MessageID = parseOptionalUUID(ids.MessageID)
			}
		case "tool.result":
			ids, payload, ok := parseToolResultRuntimeEvent(event)
			if !ok || ids.ToolCallID == "" {
				continue
			}

			entry, exists := projectedByID[ids.ToolCallID]
			if !exists {
				entry = &model.ThreadProjectionToolCall{
					ToolCallID:     ids.ToolCallID,
					ConversationID: event.ConversationID,
					RunID:          event.RunID,
					MessageID:      parseOptionalUUID(ids.MessageID),
					ToolName:       payload.Tool,
					Arguments:      datatypes.JSON(`{}`),
					CreatedAt:      event.CreatedAt,
					StartedAt:      event.CreatedAt,
					Metadata:       datatypes.JSON(`{}`),
				}
				projectedByID[ids.ToolCallID] = entry
				orderedIDs = append(orderedIDs, ids.ToolCallID)
			}

			entry.FinishedAt = &event.CreatedAt
			if toolResultPayloadIsError(payload.Result) {
				entry.Status = "failed"
				entry.Error = normalizeNullableJSON(datatypes.JSON(payload.Result))
				entry.Result = nil
			} else {
				entry.Status = "succeeded"
				entry.Result = normalizeNullableJSON(datatypes.JSON(payload.Result))
				entry.Error = nil
			}
		}
	}

	projected := make([]model.ThreadProjectionToolCall, 0, len(orderedIDs))
	for _, toolCallID := range orderedIDs {
		if entry := projectedByID[toolCallID]; entry != nil {
			projected = append(projected, *entry)
		}
	}
	return projected
}

type toolCallRuntimePayload struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

type toolResultRuntimePayload struct {
	Tool   string          `json:"tool"`
	Result json.RawMessage `json:"result"`
}

func parseToolCallRuntimeEvent(event model.RuntimeEvent) (dto.StreamEventIDs, toolCallRuntimePayload, bool) {
	var ids dto.StreamEventIDs
	if len(event.IDs) == 0 || json.Unmarshal(event.IDs, &ids) != nil {
		return dto.StreamEventIDs{}, toolCallRuntimePayload{}, false
	}
	var payload toolCallRuntimePayload
	if len(event.Payload) == 0 || json.Unmarshal(event.Payload, &payload) != nil {
		return dto.StreamEventIDs{}, toolCallRuntimePayload{}, false
	}
	return ids, payload, true
}

func parseToolResultRuntimeEvent(event model.RuntimeEvent) (dto.StreamEventIDs, toolResultRuntimePayload, bool) {
	var ids dto.StreamEventIDs
	if len(event.IDs) == 0 || json.Unmarshal(event.IDs, &ids) != nil {
		return dto.StreamEventIDs{}, toolResultRuntimePayload{}, false
	}
	var payload toolResultRuntimePayload
	if len(event.Payload) == 0 || json.Unmarshal(event.Payload, &payload) != nil {
		return dto.StreamEventIDs{}, toolResultRuntimePayload{}, false
	}
	return ids, payload, true
}

func parseOptionalUUID(raw string) *uuid.UUID {
	if raw == "" {
		return nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func toolResultPayloadIsError(result json.RawMessage) bool {
	if len(result) == 0 {
		return false
	}
	var parsed struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return false
	}
	return parsed.Status == "error" || parsed.Error != ""
}

func marshalJSON(value any, fallback string) datatypes.JSON {
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 {
		return datatypes.JSON(fallback)
	}
	return datatypes.JSON(data)
}

func normalizeJSON(value datatypes.JSON, fallback string) datatypes.JSON {
	if len(value) == 0 {
		return datatypes.JSON(fallback)
	}
	return value
}

func normalizeNullableJSON(value datatypes.JSON) datatypes.JSON {
	if len(value) == 0 {
		return nil
	}
	return value
}
