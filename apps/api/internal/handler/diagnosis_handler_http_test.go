package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// diagnosisHandlerRepo 是 ConsultationService 所依赖的 consultation repository 的内存测试替身。
// diagnosisHandlerRepo is an in-memory test double for the consultation repository used by ConsultationService.
//
// DMR-002 的目标是测试 HTTP / application boundary，而不是测试 PostgreSQL。
// DMR-002 tests the HTTP/application boundary, not PostgreSQL itself.
// Handler 仍然使用真实的 ConsultationService，因此 ownership check、phase rule 等真实业务代码仍会执行；
// The handler still uses the real ConsultationService, so ownership checks and phase rules still run.
// 只有最底层持久化被这个 Fake Repository 替代，并用计数器记录副作用。
// Only persistence is replaced by this fake repository, which records side effects with counters.
//
// 阅读时可以把调用链理解成：
// Read the call chain as:
//
//	HTTP request
//	    -> real DiagnosisHandler
//	    -> real ConsultationService
//	    -> fake repository (this type)
//
// Fake Repository 不是被测对象，它只是一个“观察点”，用来回答：是否保存 diagnosis？是否推进 phase？
// The fake repository is not the subject under test; it is an observation point for persistence side effects.
type diagnosisHandlerRepo struct {
	session *model.ConsultationSession

	diagnosisUpdateCalls int
	phaseUpdateCalls     int
	persistedDiagnosis   json.RawMessage
	persistedPhase       string
}

func (r *diagnosisHandlerRepo) Create(ctx context.Context, session *model.ConsultationSession) error {
	r.session = session
	return nil
}

func (r *diagnosisHandlerRepo) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error) {
	if r.session == nil || r.session.ConversationID != conversationID {
		return nil, nil
	}
	return r.session, nil
}

func (r *diagnosisHandlerRepo) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
	return r.session, nil
}

func (r *diagnosisHandlerRepo) ListByConversationIDs(ctx context.Context, conversationIDs []uuid.UUID) ([]model.ConsultationSession, error) {
	return nil, nil
}

func (r *diagnosisHandlerRepo) Delete(ctx context.Context, conversationID uuid.UUID) error {
	return nil
}

func (r *diagnosisHandlerRepo) UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase string) error {
	r.phaseUpdateCalls++
	r.persistedPhase = phase

	// 同步更新内存 session，让后续断言看到“如果真实 repository 写入成功后”应有的 durable state。
	// Keep the in-memory session synchronized so later assertions reflect the state a real repository update would make durable.
	if r.session != nil && r.session.ConversationID == conversationID {
		r.session.Phase = phase
	}
	return nil
}

func (r *diagnosisHandlerRepo) UpdateDiagnosis(ctx context.Context, conversationID uuid.UUID, diagnosis any) error {
	r.diagnosisUpdateCalls++

	// ConsultationService 在调用 repository 前会先 json.Marshal，因此这里捕获最终 bytes。
	// ConsultationService marshals the domain value before calling the repository, so capture the final bytes here.
	// 这样测试检查的是“真实情况下会写进 JSONB diagnosis column 的内容”。
	// This lets the test inspect exactly what would be written into the JSONB diagnosis column.
	switch typed := diagnosis.(type) {
	case []byte:
		r.persistedDiagnosis = append(json.RawMessage(nil), typed...)
	case json.RawMessage:
		r.persistedDiagnosis = append(json.RawMessage(nil), typed...)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return err
		}
		r.persistedDiagnosis = encoded
	}
	return nil
}

func (r *diagnosisHandlerRepo) CreateRunEnvelope(
	ctx context.Context,
	userID uuid.UUID,
	conversationID *uuid.UUID,
	requestID string,
	userParts datatypes.JSON,
	userMetadata datatypes.JSON,
	modelName string,
) (*model.ConsultationSession, *model.Run, *model.Message, *model.Message, uuid.UUID, bool, error) {
	return nil, nil, nil, nil, uuid.Nil, false, nil
}

// diagnosisHandlerConversationRepo 专门替代 conversation ownership repository。
// diagnosisHandlerConversationRepo is the ownership-side test double for the conversation repository.
// ConsultationService 在受保护的读取/写入前，会先通过它确认 conversation 是否属于当前用户。
// ConsultationService uses it to verify conversation ownership before protected reads and writes.
type diagnosisHandlerConversationRepo struct {
	conversation *model.Conversation
}

func (r *diagnosisHandlerConversationRepo) Create(ctx context.Context, conversation *model.Conversation) error {
	r.conversation = conversation
	return nil
}

func (r *diagnosisHandlerConversationRepo) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	if r.conversation == nil || r.conversation.ID != id || r.conversation.UserID != userID {
		return nil, nil
	}
	return r.conversation, nil
}

func (r *diagnosisHandlerConversationRepo) SoftDelete(ctx context.Context, id, userID uuid.UUID) error {
	return nil
}

func (r *diagnosisHandlerConversationRepo) GetLastEmptyConversation(ctx context.Context, userID uuid.UUID) (*model.Conversation, error) {
	return nil, nil
}

// diagnosisAIStub 用 httptest.Server 替代真正的 Python AI Service。
// diagnosisAIStub replaces the external Python AI service with httptest.Server.
// 这里故意不 fake AIClient：DiagnosisHandler 当前依赖具体的 *service.AIClient，
// We intentionally keep the real AIClient because DiagnosisHandler depends on concrete *service.AIClient.
// 因此测试服务器能让 AIClient 的 JSON marshal、HTTP request、response decode 这些真实 adapter 行为继续被覆盖。
// The HTTP stub keeps AIClient's real JSON/HTTP adapter behavior inside the test boundary.
type diagnosisAIStub struct {
	server       *httptest.Server
	analyzeCalls int
	responseBody string
}

func newDiagnosisAIStub(t *testing.T, responseBody string) *diagnosisAIStub {
	t.Helper()

	stub := &diagnosisAIStub{responseBody: responseBody}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/diagnosis/analyze":
			stub.analyzeCalls++
			var request struct {
				ConfigurationID string `json:"configuration_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			var response map[string]any
			if err := json.Unmarshal([]byte(stub.responseBody), &response); err == nil {
				if _, exists := response["agent_configuration"]; !exists {
					response["agent_configuration"] = map[string]any{
						"id":   request.ConfigurationID,
						"role": "diagnosis",
					}
				}
				encoded, _ := json.Marshal(response)
				stub.responseBody = string(encoded)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(stub.responseBody))
		case "/api/knowledge/search":
			// 当前多数用例 extracted_info 为空，因此不会真正调用 knowledge search。
			// Most current cases use empty extracted_info, so this endpoint is usually not called.
			// 仍然保留这个 endpoint，是为了未来增加 extracted_info 时不会误连真实 AI service。
			// Keeping it makes the stub safe for future cases without accidentally calling a real service.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(stub.server.Close)

	return stub
}

// newDiagnosisProfileService 用 sqlmock + GORM 构造真实的 ProfileService。
// newDiagnosisProfileService builds the real ProfileService on top of sqlmock + GORM.
// DiagnosisHandler 把 profile 当作可选信息；查不到 profile 时会退回 {}，因此测试只需要返回 0 rows。
// DiagnosisHandler treats profile as optional and falls back to {}, so zero rows are sufficient here.
//
// 之所以需要 sqlmock，是因为 ProfileService 当前依赖具体的 *repository.ProfileRepository，而不是接口。
// sqlmock is needed only because ProfileService currently depends on concrete *repository.ProfileRepository.
// 如果未来改成 interface-based dependency，这里就可以像 consultation repo 一样直接写一个很小的 Fake。
// If that dependency becomes interface-based later, this helper can be replaced by a much smaller fake.
func newDiagnosisProfileService(t *testing.T) *service.ProfileService {
	t.Helper()

	sqlDB, sqlMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	gormDB, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("create gorm database for profile service: %v", err)
	}

	// 只有通过 session/readiness 检查后的路径才会执行 ProfileRepository.GetByUserID 的 SELECT。
	// ProfileRepository.GetByUserID executes a SELECT only after session/readiness checks pass.
	// 返回 0 rows 用来模拟“用户没有 profile，但 profile 本身是可选的”。
	// Returning zero rows models an absent-but-optional profile.
	//
	// 这里故意不在 cleanup 调 ExpectationsWereMet：404（以及未来的 readiness 409）必须在 profile lookup 前返回。
	// We intentionally do not call ExpectationsWereMet in cleanup because 404 and future readiness-409 paths return before profile lookup.
	// 如果强制每个测试都必须发生 SELECT，就变成测试替身在规定生产控制流，而不是观察生产控制流。
	// Requiring the SELECT in every case would make the test double dictate production control flow instead of observing it.
	sqlMock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows([]string{"id", "user_id"}),
	)

	return service.NewProfileService(repository.NewProfileRepository(gormDB))
}

// newDiagnosisHandlerHarness 把一次 Diagnosis 请求所需的真实对象和测试替身统一装配起来。
// newDiagnosisHandlerHarness wires a realistic but fully local Diagnosis request path.
// 它保留真实 Handler / Service / AIClient，只替换 durable repository 和外部 Python HTTP service。
// It keeps the real Handler / Service / AIClient and replaces only persistence and the external Python service.
func newDiagnosisHandlerHarness(
	t *testing.T,
	userID uuid.UUID,
	conversationID uuid.UUID,
	session *model.ConsultationSession,
	aiResponse string,
) (*DiagnosisHandler, *diagnosisHandlerRepo, *diagnosisAIStub) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	repo := &diagnosisHandlerRepo{session: session}
	conversationRepo := &diagnosisHandlerConversationRepo{
		conversation: &model.Conversation{
			ID:     conversationID,
			UserID: userID,
			Status: "active",
		},
	}

	aiStub := newDiagnosisAIStub(t, aiResponse)

	// AIClient 在构造时读取 AI_SERVICE_URL，所以这里把它临时指向 httptest.Server。
	// AIClient reads AI_SERVICE_URL at construction time, so point it to httptest.Server for this test.
	// t.Setenv 会在测试结束后自动恢复环境变量，避免影响相邻测试。
	// t.Setenv restores the environment automatically after the test and prevents cross-test leakage.
	t.Setenv("AI_SERVICE_URL", aiStub.server.URL)
	aiClient := service.NewAIClient()
	agentDeploymentPolicy, err := service.NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}

	h := NewDiagnosisHandler(
		service.NewConsultationService(repo, conversationRepo),
		newDiagnosisProfileService(t),
		aiClient,
		nil, // DMR-002 不测试 output-review persistence / output-review persistence is outside DMR-002's boundary.
		nil,
		nil,
		nil,
		agentDeploymentPolicy,
	)

	return h, repo, aiStub
}

// performAnalyzeDiagnosisRequest 直接构造 gin.Context 并调用 AnalyzeDiagnosis。
// performAnalyzeDiagnosisRequest invokes AnalyzeDiagnosis with a manually created gin.Context.
// 这样可以测试 Handler / application boundary，同时不必启动完整 router 和 middleware stack。
// This keeps the test focused without booting the full router and middleware stack.
func performAnalyzeDiagnosisRequest(
	t *testing.T,
	h *DiagnosisHandler,
	userID uuid.UUID,
	conversationID uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/consultations/"+conversationID.String()+"/diagnosis",
		nil,
	)
	ctx.Params = gin.Params{{Key: "id", Value: conversationID.String()}}
	ctx.Set("user_id", userID.String())

	h.AnalyzeDiagnosis(ctx)
	return recorder
}

func assertDiagnosisAPIErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, expectedCode string) {
	t.Helper()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, recorder.Body.String())
	}
	if body.Error.Code != expectedCode {
		t.Fatalf("expected error code %q, got %q: %s", expectedCode, body.Error.Code, recorder.Body.String())
	}
}

func TestDiagnosisConfigurationMatchesRequiresSelectedIDAndRole(t *testing.T) {
	if diagnosisConfigurationMatches(map[string]any{
		"agent_configuration": map[string]any{"id": "diag-config-wrong", "role": "diagnosis"},
	}, "diag-config-selected") {
		t.Fatal("mismatched Agent configuration must be rejected")
	}
	if diagnosisConfigurationMatches(map[string]any{
		"agent_configuration": map[string]any{"id": "diag-config-selected", "role": "treatment"},
	}, "diag-config-selected") {
		t.Fatal("wrong Agent role must be rejected")
	}
	if !diagnosisConfigurationMatches(map[string]any{
		"agent_configuration": map[string]any{"id": "diag-config-selected", "role": "diagnosis"},
	}, "diag-config-selected") {
		t.Fatal("selected Diagnosis Agent configuration should be accepted")
	}
}

func TestAnalyzeDiagnosisReturns404WhenConsultationSessionDoesNotExist(t *testing.T) {
	// Characterization target / 当前行为特性：
	//
	// - parent Conversation 存在，并且属于当前 authenticated user；
	// - the parent Conversation exists and belongs to the authenticated user;
	// - ConsultationSession 本身不存在；
	// - the ConsultationSession itself is missing;
	// - Handler 应在 profile / AI 之前停止，并返回公开的 404 NOT_FOUND contract。
	// - the handler must stop before profile/AI work and return the public 404 NOT_FOUND contract.
	//
	// 为什么一定保留 Conversation？因为“ownership 不存在”和“session 不存在”是两个不同边界。
	// Why keep the Conversation? Because missing ownership and missing session are different boundaries.
	// 如果 Conversation 也不存在，当前 ConsultationService 会先返回 ownership error，Handler 现在会把它映射成 500。
	// If the Conversation were also missing, the current service returns an ownership error that maps to 500 today.
	userID := uuid.New()
	conversationID := uuid.New()

	h, _, aiStub := newDiagnosisHandlerHarness(
		t,
		userID,
		conversationID,
		nil,
		`{"governance":{"verdict":"accepted"}}`,
	)

	recorder := performAnalyzeDiagnosisRequest(t, h, userID, conversationID)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
	assertDiagnosisAPIErrorCode(t, recorder, "NOT_FOUND")

	if aiStub.analyzeCalls != 0 {
		t.Fatalf("AI service must not be called when consultation session is missing; got %d calls", aiStub.analyzeCalls)
	}
}

func TestAnalyzeDiagnosisRequiresBodyStateDomainServices(t *testing.T) {
	userID := uuid.New()
	conversationID := uuid.New()
	session := &model.ConsultationSession{
		ConversationID: conversationID,
		Phase:          "ready_for_analysis",
		ExtractedInfo:  datatypes.JSON(`[]`),
	}

	h, repo, aiStub := newDiagnosisHandlerHarness(
		t,
		userID,
		conversationID,
		session,
		`{"candidates":[{"name":"legacy"}]}`,
	)

	recorder := performAnalyzeDiagnosisRequest(t, h, userID, conversationID)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without BodyState diagnosis services, got %d: %s", recorder.Code, recorder.Body.String())
	}
	assertDiagnosisAPIErrorCode(t, recorder, "DIAGNOSIS_DOMAIN_UNAVAILABLE")
	if aiStub.analyzeCalls != 0 {
		t.Fatalf("diagnosis must not call AI when its durable domain services are unavailable, got %d calls", aiStub.analyzeCalls)
	}
	if repo.diagnosisUpdateCalls != 0 || repo.phaseUpdateCalls != 0 {
		t.Fatalf("unwired diagnosis domain must have no side effects; diagnosis=%d phase=%d", repo.diagnosisUpdateCalls, repo.phaseUpdateCalls)
	}
}
