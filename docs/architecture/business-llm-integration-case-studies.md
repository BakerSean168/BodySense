# 真实业务应用的 LLM 接入实践（调研记录）

Status: **research only / no runtime behavior change**
Checked: 2026-10-23
Related: [Model Gateway Routing](./model-gateway-routing.md), [Oracle2 dev-only PR #432（已关闭、未合并，仅作历史记录）](https://github.com/BakerSean168/BodySense/pull/432), [OpenRouter / Groq connectivity](../runbooks/openrouter-groq-free-connectivity.md).

## 调研边界

调研真实业务产品的源码，而非仅比较 Agent 框架。**模型质量比较、选型排名、评测系统迁移与自动路由不在本轮范围内**。三个项目的文件路径可能随上游版本变化，以下链接是调研时可见的源文件。

| 产品及业务 | 真实代码入口 | 模型适配 | 业务选择 |
| --- | --- | --- | --- |
| Chatwoot / Captain 智能客服 | [ResponseBuilderJob](https://github.com/chatwoot/chatwoot/blob/develop/enterprise/app/jobs/captain/conversation/response_builder_job.rb), [AgentRunnerService](https://github.com/chatwoot/chatwoot/blob/develop/enterprise/app/services/captain/assistant/agent_runner_service.rb) | [RubyLLM client facade](https://github.com/chatwoot/chatwoot/blob/develop/lib/captain/base_task_service.rb)，并非强制使用独立 LiteLLM Proxy | [FeatureRouter](https://github.com/chatwoot/chatwoot/blob/develop/lib/llm/feature_router.rb) 按业务功能、账号覆盖、默认配置选模型 |
| FastGPT / 知识库客服、工作流 | [dispatchChatCompletion](https://github.com/labring/FastGPT/blob/main/packages/service/core/workflow/dispatch/ai/chat/dispatchChatCompletion.ts) | [getAIApi](https://github.com/labring/FastGPT/blob/main/packages/service/core/ai/config.ts) 创建 OpenAI SDK 客户端并选择 Base URL 和 Key，[createChatCompletion](https://github.com/labring/FastGPT/blob/main/packages/service/core/ai/llm/request/createChatCompletion.ts) 调用 | 每个工作流节点加载其模型资源；支持 AI Proxy / 自定义 OpenAI 兼容接口 |
| Onyx / 企业知识问答与检索 Agent | [chat model factory](https://github.com/onyx-dot-app/onyx/blob/main/backend/onyx/llm/factory.py), [multi_llm](https://github.com/onyx-dot-app/onyx/blob/main/backend/onyx/llm/multi_llm.py) | 应用进程内的 LiteLLM Python **Library**，可连接外部上游，**不是必须部署 LiteLLM Proxy** | 本轮指定模型配置 → Agent/Persona 默认 → 工作区默认；辅助模型可独立配置 |

## 1. Chatwoot：Agent 不能直接决定持久化业务结果

代码路径：

1. `ResponseBuilderJob.perform` 检查会话是否仍待回复、是否收到更近的客户消息。
2. 通过 `MessageHistoryBuilderService` 装配上下文，交给 `AgentRunnerService`。
3. Runner 使用 Agent 配置的模型、工具与输出 Schema 执行多轮对话，可能搜索 FAQ、自定义 HTTP API 或转人工。
4. **Job 层**根据 `handoff_tool_completed`、新消息和生成错误决定是否发送消息、修改客服会话或转人工。
5. 工具已执行后，不盲目重放整轮 Agent，避免重复业务副作用。会话历史与执行模型可以被记录。

模型选择：[config/llm.yml](https://github.com/chatwoot/chatwoot/blob/develop/config/llm.yml) 为不同功能声明允许模型与默认模型，`FeatureRouter.resolve(feature:, account:)` 支持账号覆盖及安装级覆盖。**模型业务选择不等于模型故障自动回退**。

BodySense 对应原则：Pydantic AI 可以产生解释和提案，但身体档案、Diagnosis/Training 等状态的真实写入仍须经过 Go 的领域约束与用户确认；失败后的重试必须考虑工具副作用。

## 2. FastGPT：每个业务节点有模型 ID，统一通过 SDK/Proxy

调用路径：工作流节点 → `loadWorkflowModelResource(modelId)` → `createLLMResponse` → `createChatCompletion` → `getAIApi()` → `OpenAI(...).chat.completions.create(...)` → 解析结果、用量和节点记录。

- `getAIApi()` 负责选择用户自己的 Base URL/Key 或服务端 AI Proxy；不是为每个 AI 节点手写供应商 HTTP 调用。
- 业务节点可使用不同模型；调用失败时保留上游错误上下文并保护用户凭据。
- [testModelConnection](https://github.com/labring/FastGPT/blob/main/packages/service/core/ai/model/test.ts) 用一条小的流式提示词确认模型**连通性**，不能替代业务质量评测。
- 应用批量 Evaluation 有独立 UI 与 [BullMQ 队列](https://github.com/labring/FastGPT/blob/main/packages/dal/redis/bullmq/services/evaluation.ts)，并允许评测模型与业务应用模型不同。公开仓库可见 UI/队列/Schema，并不意味着本次已完整审查所有评分 Worker。

BodySense 对应原则：保持 `LITELLM_BASE_URL` / `LITELLM_API_KEY` 这一条网关边界；新接入 OpenRouter/Groq 不应在 Assessment、Consultation 等业务文件中直接散落 API Key 和 SDK 构造逻辑。

## 3. Onyx：LiteLLM Library 与独立 Proxy 的区别

- [multi_llm.py](https://github.com/onyx-dot-app/onyx/blob/main/backend/onyx/llm/multi_llm.py) 直接调用 Python `litellm.completion(...)`，处理提供商协议差异、错误、流、模型能力。
- [factory.py](https://github.com/onyx-dot-app/onyx/blob/main/backend/onyx/llm/factory.py) 显式处理模型身份、Persona 配置和请求覆盖；同名模型不应被错误路由到另一个供应商。
- [eval.py](https://github.com/onyx-dot-app/onyx/blob/main/backend/onyx/evals/eval.py) 用真实聊天执行路径评估工具调用、引用、会话和输出，属于**后续评测参考**，本轮不实施。

`litellm.completion()` 是应用进程内部函数调用；Standalone LiteLLM Proxy 则是独立 HTTP 服务。它们可复用同一生态，但部署、鉴权、密钥保护和故障域不同。不能把“使用了 LiteLLM”自动理解为“部署了 LiteLLM Proxy”。

## BodySense 当前保留的架构与决策

~~~text
Go：业务状态与安全规则／配置部署权
  -> Python AI Service：Agent 运行、Manifest logical_model / Prompt / Tool
  -> 本地 Dev Standalone LiteLLM：模型逻辑组与 dev-only 路由（staging 复用生产网关）
  -> [可选] Oracle2 共享 LiteLLM：供应商凭据与专用 Virtual Key
  -> 物理模型渠道：WindHub / Kunyou / 南梁 / OpenRouter / Groq 等
~~~

**本轮决定：不改变以上路径、不修改任何生产/staging/当前 dev 环境正在运行的逻辑模型。** `bodysense-posture` 的视觉能力仍需单独验证。Oracle2 与本地 dev 网关的双层结构需避免重复 Fallback；返回结果必须追踪最终物理模型。`num_retries: 0` 不代表所有 4xx 都不会触发 Fallback；认证及业务契约错误必须区分处理。

按 Chatwoot/FastGPT 的经验，新渠道首先作为网关层的候选适配能力，并按业务能力白名单开放，而不是因为 Key 可用就自动给所有 Agent 分配。

## 延后事项：模型选择与业务评测（尚未实施）

- 复用当前已存在的 Pydantic Evals / Diagnosis / Consultation / Treatment 数据集及运行器，先核对与 BodyState V2 契约的适用性。
- 固定业务输入、Agent Manifest、Prompt、输出 Schema、工具与初始状态；每次测评必须固定唯一真实渠道与物理模型，**关闭自动 Fallback**。
- 先看业务正确性、安全约束、工具副作用和引用，再看成本/延迟；单个 PONG、HTTP 200、合法 JSON **不能**用作业务选型结论。
- 模型升级应由版本化评测、人工审查、回滚策略决定；不让测评进程自动写生产路由。

模型评测和排名由独立后续任务处理；本文件只记录已核实的业务接入模式及当前设计约束。
