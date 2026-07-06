# Multi-Provider AI Router 方案设计

> 状态：待实施 | 创建时间：2026-06-26

## 1. 背景与动机

### 当前问题

- **单供应商锁定**：`llm_provider.py` 只有 `OpenAICompatibleProvider` 一个实现，模型/Key/URL 全部硬编码在 `.env`，切换模型需改代码。
- **无自动降级**：429 限流后只做 3 次重试，没有 fallback 到其他 provider 的能力。
- **无 per-task 路由**：所有任务（流式对话、JSON 诊断、评估报告）共用同一个模型，无法按能力需求选择最优模型。
- **JSON 解析脆弱**：`diagnosis_service`、`assessment_service`、`reassessment_service` 各自维护一套 3 策略 JSON 解析（直接解析 → markdown 代码块 → 括号匹配），代码重复。
- **死代码**：`LLM_PROVIDER` 环境变量从未被读取，`build_rag_context()` 已不使用。
- **调用方直接耦合**：5 个服务各自调用 `get_llm_provider()` 单例，无法统一添加日志、追踪、重试。

### 目标

构建一个 **配置驱动、自动路由、自动降级** 的多 Provider AI 系统：

- 业务层只调用 `AIService`，不感知底层 provider
- 模型选择由路由层根据配置 + 能力匹配 + 熔断状态自动决定
- 切换/新增 provider 只改 YAML 配置，不改业务代码
- 统一 Request/Response 协议，统一流式事件格式

---

## 2. 架构总览

```
业务代码（consultation_graph / diagnosis_service / assessment_service / ...）
        │
        ▼
    AIService  ←── 统一入口，接收 AiRequest
        │
        ▼
    ModelRouter  ←── 根据 use_case + capabilities + circuit_breaker 选择 provider+model
        │
        ▼
    CircuitBreaker (in-memory dict)
        │
        ▼
    AiProvider 适配器层（OpenAICompatibleProvider / 未来: AnthropicProvider / ...）
        │
        ▼
    MiMo / OpenRouter / Qwen / DeepSeek / OpenAI ...
```

### 核心设计原则

1. **业务不认识任何厂商 SDK，只认识 AIService**
2. **抽象的不是"模型"，而是"任务能力"** —— 按 use_case 路由
3. **配置驱动** —— providers、models、routes 全部定义在 YAML
4. **流式事件统一** —— 前端不感知厂商差异

---

## 3. 关键设计决策

| # | 决策项 | 选择 | 理由 |
|---|--------|------|------|
| 1 | 路由器放在哪 | Python ai-service 内部 | 唯一 LLM 消费者是 ai-service，无需独立网关 |
| 2 | Provider 接口设计 | 统一 AiRequest/AiResponse 对象 | 更干净的抽象，一个入口点 |
| 3 | 配置格式 | YAML 文件 | 易编辑，支持嵌套结构，env var 插值 |
| 4 | 路由策略 | 有序 fallback + 熔断器 | 实用且不过度设计，可演化为 scored selection |
| 5 | 调用方接入方式 | AIService 作为唯一入口 | 业务不直接接触 router/provider |
| 6 | 熔断器状态存储 | 内存 dict | 单进程足够，重启后自动恢复 |
| 7 | 结构化输出 | 原生 response_format + text 解析兜底 | 消除 3 份重复解析代码 |
| 8 | 迁移策略 | Big bang 全量替换 | 一次性干净切换 |
| 9 | Use-case 划分 | 3 条路由（按能力需求） | consultation.reply / llm.json / llm.text |

---

## 4. 类型定义

### 4.1 AiRequest

```python
@dataclass
class AiRequest:
    use_case: str                          # 路由名: "consultation.reply" | "llm.json" | "llm.text"
    messages: list[ChatMessage]            # 复用现有 ChatMessage dataclass
    tools: list[ToolDefinition] | None = None
    stream: bool = False
    response_format: str | None = None     # "json_object" | None
    temperature: float | None = None       # None = 使用 use_case 默认值
    max_tokens: int | None = None          # None = 使用 use_case 默认值
    metadata: dict[str, Any] | None = None # userId, sessionId, traceId 等
```

### 4.2 AiResponse

```python
@dataclass
class AiResponse:
    text: str                              # LLM 输出文本
    model: str                             # 实际使用的模型 ID
    provider: str                          # 实际使用的 provider ID
    usage: TokenUsage | None = None        # token 用量
    finish_reason: str | None = None       # "stop" | "tool_calls" | "length"
    tool_calls: list[ToolCall] | None = None
    raw: Any = None                        # 原始 API 响应（调试用）
```

### 4.3 AiStreamEvent

```python
@dataclass
class AiStreamEvent:
    type: str                              # "text_delta" | "tool_call_delta" | "tool_call_done" | "usage" | "done" | "error"
    text: str | None = None                # type="text_delta" 时的文本片段
    tool_call_id: str | None = None
    tool_name: str | None = None
    tool_arguments_delta: str | None = None
    tool_arguments: dict | None = None     # type="tool_call_done" 时的完整参数
    usage: TokenUsage | None = None
    finish_reason: str | None = None
    error: str | None = None
```

### 4.4 TokenUsage

```python
@dataclass
class TokenUsage:
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0
```

> **注意**：现有 `ChatMessage`、`ToolDefinition`、`ToolCall` dataclass 保持不变，直接复用。

---

## 5. Provider 适配器层

### 5.1 AiProvider Protocol

```python
class AiProvider(Protocol):
    @property
    def id(self) -> str: ...

    @property
    def capabilities(self) -> set[str]: ...
        # {"stream", "tools", "json_mode", "vision", "long_context", "reasoning"}

    @property
    def model_id(self) -> str: ...

    async def generate(self, req: AiRequest) -> AiResponse: ...
    async def generate_stream(self, req: AiRequest) -> AsyncIterator[AiStreamEvent]: ...
    async def health_check(self) -> bool: ...
```

### 5.2 OpenAICompatibleProvider

所有 OpenAI 兼容 API（MiMo、OpenRouter、Qwen、DeepSeek）共用一个实现：

```python
class OpenAICompatibleProvider:
    def __init__(self, provider_id: str, model_id: str, base_url: str, api_key: str, capabilities: set[str]):
        self._client = AsyncOpenAI(base_url=base_url, api_key=api_key)
        ...

    async def generate(self, req: AiRequest) -> AiResponse:
        # 转换 messages/tools → OpenAI 格式
        # 设置 response_format（如果 req.response_format == "json_object"）
        # 调用 self._client.chat.completions.create()
        # 转换响应 → AiResponse
        ...

    async def generate_stream(self, req: AiRequest) -> AsyncIterator[AiStreamEvent]:
        # 调用 self._client.chat.completions.create(stream=True)
        # 逐 chunk 转换为 AiStreamEvent
        # text_delta: 累积 content delta
        # tool_call_delta: 累积 tool call fragments
        # tool_call_done: finish_reason="tool_calls" 时输出完整 ToolCall
        # done: finish_reason="stop" 时输出
        ...
```

**关键**：`OpenAICompatibleProvider` 不读取任何环境变量。所有配置（base_url、api_key、model_id、capabilities）由 `AIService` 根据 YAML 配置注入。

---

## 6. ModelRouter

### 6.1 路由配置（YAML）

```yaml
# apps/ai-service/config/models.yaml

providers:
  mimo:
    type: openai-compatible
    base_url: ${MIMO_BASE_URL}
    api_key: ${MIMO_API_KEY}
    models:
      - id: mimo-v2.5-pro
        capabilities: [stream, tools, json_mode, long_context, reasoning]

  openrouter:
    type: openai-compatible
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}
    models:
      - id: deepseek/deepseek-chat
        capabilities: [stream, tools, json_mode]
      - id: openai/gpt-oss-120b
        capabilities: [stream, tools, json_mode, reasoning]

routes:
  consultation.reply:
    defaults:
      temperature: 0.7
      max_tokens: 2048
    candidates:
      - provider: mimo
        model: mimo-v2.5-pro
      - provider: openrouter
        model: deepseek/deepseek-chat

  llm.json:
    defaults:
      temperature: 0.3
      max_tokens: 2048
      response_format: json_object
    candidates:
      - provider: mimo
        model: mimo-v2.5-pro
      - provider: openrouter
        model: deepseek/deepseek-chat

  llm.text:
    defaults:
      temperature: 0.3
      max_tokens: 2048
    candidates:
      - provider: mimo
        model: mimo-v2.5-pro
      - provider: openrouter
        model: deepseek/deepseek-chat
```

### 6.2 路由选择逻辑

```python
class ModelRouter:
    def __init__(self, config: RouterConfig):
        self._config = config
        self._providers: dict[str, AiProvider] = {}  # "provider_id/model_id" → provider instance
        self._circuit_breaker: dict[str, datetime] = {}  # "provider_id/model_id" → disabled_until
        self._circuit_breaker_duration = timedelta(seconds=60)

    async def select(self, use_case: str, required_capabilities: set[str] | None = None) -> tuple[AiProvider, str]:
        """
        返回 (provider, model_id)。
        按 candidates 顺序尝试，跳过熔断中的 provider。
        全部不可用时抛出 NoAvailableProviderError。
        """
        route = self._config.routes[use_case]
        now = datetime.now()

        for candidate in route.candidates:
            key = f"{candidate.provider}/{candidate.model}"

            # 1. 检查熔断
            if key in self._circuit_breaker and self._circuit_breaker[key] > now:
                continue

            # 2. 检查能力
            provider = self._get_provider(candidate.provider, candidate.model)
            if required_capabilities and not required_capabilities.issubset(provider.capabilities):
                continue

            return provider, candidate.model

        raise NoAvailableProviderError(f"All candidates for {use_case} are unavailable")

    def trip_breaker(self, provider_id: str, model_id: str):
        """触发熔断，60 秒内不再选择该 provider/model"""
        key = f"{provider_id}/{model_id}"
        self._circuit_breaker[key] = datetime.now() + self._circuit_breaker_duration
        logger.warning(f"Circuit breaker tripped for {key}, disabled until {self._circuit_breaker[key]}")
```

### 6.3 熔断器策略

- **触发条件**：HTTP 429 (Rate Limit)
- **冷却时间**：60 秒（可配置）
- **恢复**：冷却期过后自动恢复，无需手动干预
- **不熔断的错误**：400（参数错误）、401（认证失败）—— 这些是配置问题，重试无意义
- **重试后熔断**：如果 fallback 链全部失败，每个失败的 provider 都触发熔断

---

## 7. AIService 统一入口

```python
class AIService:
    def __init__(self, config_path: str = "config/models.yaml"):
        self._config = load_yaml_config(config_path)
        self._router = ModelRouter(self._config)
        self._providers: dict[str, AiProvider] = {}
        self._init_providers()

    def _init_providers(self):
        """根据 YAML 配置创建所有 provider 实例"""
        for provider_id, provider_cfg in self._config.providers.items():
            for model_cfg in provider_cfg.models:
                key = f"{provider_id}/{model_cfg.id}"
                if provider_cfg.type == "openai-compatible":
                    self._providers[key] = OpenAICompatibleProvider(
                        provider_id=provider_id,
                        model_id=model_cfg.id,
                        base_url=interpolate_env(provider_cfg.base_url),
                        api_key=interpolate_env(provider_cfg.api_key),
                        capabilities=set(model_cfg.capabilities),
                    )

    async def generate(self, req: AiRequest) -> AiResponse:
        """
        非流式调用。自动路由 + fallback。
        1. 从 route 配置获取 defaults（temperature, max_tokens, response_format）
        2. AiRequest 中显式设置的值覆盖 defaults
        3. 调用 router.select() 获取 provider
        4. 调用 provider.generate()
        5. 429 → trip_breaker → fallback 到下一个 candidate
        """
        route = self._config.routes[req.use_case]
        merged_req = self._apply_defaults(req, route.defaults)

        last_error = None
        for candidate in route.candidates:
            try:
                provider = self._router.get_provider(candidate.provider, candidate.model)
                return await provider.generate(merged_req)
            except RateLimitError:
                self._router.trip_breaker(candidate.provider, candidate.model)
                last_error = RateLimitError(f"{candidate.provider}/{candidate.model} rate limited")
                continue
            except ProviderError as e:
                self._router.trip_breaker(candidate.provider, candidate.model)
                last_error = e
                continue

        raise NoAvailableProviderError(f"All candidates for {req.use_case} failed") from last_error

    async def generate_stream(self, req: AiRequest) -> AsyncIterator[AiStreamEvent]:
        """
        流式调用。逻辑同 generate，但返回 AsyncIterator[AiStreamEvent]。
        注意：流式调用中，429 可能在首个 chunk 之前就抛出，
        此时可以 fallback。但如果已经在流式输出中遇到错误，
        则直接 emit error event，不 fallback（避免中途切换 provider 导致不一致）。
        """
        route = self._config.routes[req.use_case]
        merged_req = self._apply_defaults(req, route.defaults)
        merged_req.stream = True

        for candidate in route.candidates:
            try:
                provider = self._router.get_provider(candidate.provider, candidate.model)
                async for event in provider.generate_stream(merged_req):
                    yield event
                return  # 成功完成
            except RateLimitError:
                self._router.trip_breaker(candidate.provider, candidate.model)
                continue
            except ProviderError:
                self._router.trip_breaker(candidate.provider, candidate.model)
                continue

        raise NoAvailableProviderError(f"All candidates for {req.use_case} failed")

    def _apply_defaults(self, req: AiRequest, defaults: RouteDefaults) -> AiRequest:
        """将 route defaults 合并到 request，显式值优先"""
        return AiRequest(
            use_case=req.use_case,
            messages=req.messages,
            tools=req.tools,
            stream=req.stream,
            response_format=req.response_format or defaults.response_format,
            temperature=req.temperature if req.temperature is not None else defaults.temperature,
            max_tokens=req.max_tokens if req.max_tokens is not None else defaults.max_tokens,
            metadata=req.metadata,
        )
```

---

## 8. 调用方迁移

### 8.1 当前 → 目标对照

| 调用方 | 当前代码 | 目标代码 |
|--------|---------|---------|
| `consultation_graph.py` | `provider = get_llm_provider()` → `provider.chat_stream(messages, tools, 0.7, 2048)` | `ai_service.generate_stream(AiRequest(use_case="consultation.reply", ...))` |
| `diagnosis_service.py` | `provider.chat(messages, None, 0.3, 2048)` → 手动解析 JSON | `ai_service.generate(AiRequest(use_case="llm.json", ...))` → `response.text` 直接是 JSON |
| `assessment_service.py` | 同上 | 同上 |
| `reassessment_service.py` | 同上 | 同上 |
| `video_pipeline.py` | `provider.chat(messages, None, 0.3, 2048)` | `ai_service.generate(AiRequest(use_case="llm.text", ...))` |

### 8.2 JSON 解析统一

迁移后，`diagnosis_service`、`assessment_service`、`reassessment_service` 不再需要各自的 `_parse_json()` 方法。当 `response_format="json_object"` 时，`AiResponse.text` 保证是合法 JSON：

```python
# 之前（3 份重复代码）
raw = await provider.chat(messages, None, 0.3, 2048)
data = self._parse_json(raw.text)  # 3 策略 fallback

# 之后（1 行）
response = await ai_service.generate(AiRequest(use_case="llm.json", messages=messages))
data = json.loads(response.text)  # 保证合法 JSON
```

保留一个 `_parse_json()` 作为兜底，放在 `ai_service.py` 中，仅在 provider 不支持 `response_format` 时使用。

---

## 9. 目录结构

```
apps/ai-service/src/
├── ai/                              # 新增：多 Provider 系统
│   ├── __init__.py
│   ├── types.py                     # AiRequest, AiResponse, AiStreamEvent, TokenUsage
│   ├── errors.py                    # RateLimitError, ProviderError, NoAvailableProviderError
│   ├── service.py                   # AIService（唯一入口）
│   ├── router.py                    # ModelRouter + CircuitBreaker
│   ├── config.py                    # YAML 配置加载 + env var 插值
│   └── providers/
│       ├── __init__.py
│       ├── base.py                  # AiProvider Protocol
│       └── openai_compatible.py     # OpenAICompatibleProvider
├── config/                          # 新增：配置目录
│   └── models.yaml                  # provider/model/route 定义
├── services/
│   ├── llm_provider.py              # 删除（被 ai/ 替代）
│   ├── chat_service.py              # 迁移：使用 AIService
│   ├── consultation_graph.py        # 迁移：使用 AIService
│   ├── diagnosis_service.py         # 迁移：使用 AIService + 删除 _parse_json
│   ├── assessment_service.py        # 迁移：使用 AIService + 删除 _parse_json
│   ├── reassessment_service.py      # 迁移：使用 AIService + 删除 _parse_json
│   └── ...
├── rag/
│   ├── embedding.py                 # 暂不迁移（embedding 是独立系统）
│   └── ...
└── prompts/                         # 不变
```

---

## 10. YAML 配置详解

### 10.1 环境变量插值

YAML 中的 `${VAR_NAME}` 在加载时替换为环境变量值：

```yaml
base_url: ${MIMO_BASE_URL}    # → os.environ["MIMO_BASE_URL"]
api_key: ${MIMO_API_KEY}      # → os.environ["MIMO_API_KEY"]
```

实现：正则替换 `\$\{(\w+)\}` → `os.environ[match]`，缺失时抛出 `ConfigError`。

### 10.2 .env.example 更新

```bash
# === AI Provider Keys ===
MIMO_API_KEY=sk-your-mimo-api-key
MIMO_BASE_URL=https://token-plan-cn.xiaomimimo.com/v1
OPENROUTER_API_KEY=sk-your-openrouter-key

# 以下变量废弃（LLM_PROVIDER 从未被使用，LLM_MODEL/LLM_BASE_URL/LLM_API_KEY 迁移到 YAML 配置）
# LLM_PROVIDER=mimo
# LLM_MODEL=mimo-v2.5-pro
# LLM_BASE_URL=...
# LLM_API_KEY=...
```

### 10.3 使用示例

新增 provider 只需在 YAML 中添加：

```yaml
providers:
  qwen:
    type: openai-compatible
    base_url: ${QWEN_BASE_URL}
    api_key: ${QWEN_API_KEY}
    models:
      - id: qwen-plus
        capabilities: [stream, tools, json_mode]

routes:
  llm.json:
    candidates:
      - provider: qwen          # 新增为首选
        model: qwen-plus
      - provider: mimo          # 降级为备选
        model: mimo-v2.5-pro
```

业务代码零修改。

---

## 11. 流式事件规范

### 11.1 AiStreamEvent 类型

| type | 字段 | 说明 |
|------|------|------|
| `text_delta` | `text` | 文本片段，追加到已有文本 |
| `tool_call_delta` | `tool_call_id`, `tool_name?`, `tool_arguments_delta?` | 工具调用增量 |
| `tool_call_done` | `tool_call_id`, `tool_name`, `tool_arguments` | 工具调用完成，参数已完整 |
| `usage` | `usage` | token 用量统计 |
| `done` | `finish_reason` | 流结束 ("stop" / "tool_calls" / "length") |
| `error` | `error` | 错误信息 |

### 11.2 与现有 SSE 事件的映射

现有 `consultation_graph.py` 通过 `StreamWriter` 发出的事件（`text`、`extracted_info`、`citation` 等）是**业务层事件**，不属于 provider 流式事件。Provider 层只负责 LLM 通信，业务层事件的格式和发送逻辑不变。

映射关系：

```
Provider 层 (AiStreamEvent)          业务层 (StreamWriter)
─────────────────────────           ─────────────────────
text_delta                          → 累积后 emit {"type": "text", "text": chunk}
tool_call_done(extract_symptom)     → emit {"type": "extracted_info", ...}
tool_call_done(search_knowledge)    → 执行 RAG 查询 → 返回 tool result
done                                → 循环结束或 emit __done__
```

---

## 12. 实施步骤

### Phase 1：基础框架

- [ ] 创建 `ai/` 目录结构
- [ ] 实现 `types.py`（AiRequest, AiResponse, AiStreamEvent, TokenUsage）
- [ ] 实现 `errors.py`（自定义异常）
- [ ] 实现 `config.py`（YAML 加载 + env 插值）
- [ ] 编写 `config/models.yaml`（3 条路由，MiMo + OpenRouter）
- [ ] 实现 `providers/base.py`（AiProvider Protocol）
- [ ] 实现 `providers/openai_compatible.py`
- [ ] 实现 `router.py`（ModelRouter + CircuitBreaker）
- [ ] 实现 `service.py`（AIService）

### Phase 2：迁移调用方

- [ ] 迁移 `consultation_graph.py` → `ai_service.generate_stream()`
- [ ] 迁移 `diagnosis_service.py` → `ai_service.generate()` + 删除 `_parse_json()`
- [ ] 迁移 `assessment_service.py` → 同上
- [ ] 迁移 `reassessment_service.py` → 同上
- [ ] 迁移 `video_pipeline.py`（splitter/curator）→ `ai_service.generate()`

### Phase 3：清理

- [ ] 删除 `services/llm_provider.py`
- [ ] 删除 `get_llm_provider()` 单例
- [ ] 更新 `.env.example`（废弃旧变量）
- [ ] 更新所有测试

### Phase 4：验证

- [ ] `python -m pytest` 全部通过
- [ ] `pnpm nx run ai-service:lint` 通过
- [ ] 手动测试：consultation 对话流式输出正常
- [ ] 手动测试：diagnosis JSON 输出正常
- [ ] 手动测试：429 触发熔断 → fallback 到下一个 provider
- [ ] 手动测试：全部 provider 不可用时返回明确错误

---

## 13. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| YAML 解析引入新依赖 | 需要 `pyyaml` | PyYAML 是 Python 生态最常用的 YAML 库，风险极低 |
| 流式 fallback 时机 | 已开始输出后遇到 429 无法 fallback | 仅在首个 chunk 之前做 fallback；流式中途错误直接 emit error event |
| 嵌入系统（embedding）未纳入路由 | embedding 有独立的 provider 模式 | embedding 是独立系统（hashing/local/api），使用模式不同，暂不纳入路由；后续可单独加 `embedding.json` 路由 |
| 现有测试大量 mock `get_llm_provider()` | 迁移后 mock 接口变化 | 更新测试：mock `AIService.generate()` 而非 provider 内部方法 |
| `response_format=json_object` 不是所有模型都支持 | 某些免费模型可能不支持 | fallback 链中靠后的模型如果不支持 json_mode，capabilities 声明中不包含 `json_mode`，router 会跳过 |

---

## 14. 未来演化路径

当前方案是第一阶段，以下能力可在后续迭代中加入：

1. **Scored selection**：在 ordered fallback 基础上加打分（priority × quota_remaining × health），适合 provider 数量 > 3 时
2. **Quota tracking**：在内存或 Redis 中记录每个 provider 的 token 用量、429 次数、错误率
3. **Anthropic/Gemini provider**：当需要接入非 OpenAI 兼容 API 时，实现新的 `AiProvider`
4. **Embedding 路由**：将 `rag/embedding.py` 也纳入路由系统，支持 embedding provider fallback
5. **结构化输出 Schema**：使用 `response_format: {type: "json_schema", schema: {...}}` 做更严格的输出验证
6. **Prompt 版本管理**：将 prompts 从硬编码 Python 字符串迁移到 YAML/Jinja2 模板
7. **LiteLLM 集成**：当 provider 数量过多时，将 LiteLLM 作为底层 provider adapter
