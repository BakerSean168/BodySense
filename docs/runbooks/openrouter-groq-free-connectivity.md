# OpenRouter / Groq：免费渠道连通性与接入边界

Status: **Historical GCP research only; WSL/Oracle2 acceptance pending**
Snapshot: 2026-10-23 (GCP-dev-01 retired on the same date)
Current dev hosts: `oracle2` (dev machine and shared LiteLLM) or `wsl-local`; staging reuses the
production LiteLLM gateway.

> **环境边界（2026-10-26 更新）**：以下 GCP API/模型/配额结果全部是 2026-10-23 的历史研究证据。GCP-dev-01 已退役，不能再用于执行、部署、认证或验收；其本地凭据、进程及未推送文件也不能假定存在于 WSL。当前 WSL 尚未完成 OpenRouter/Groq 免费凭据、专用 BodySense Virtual Key 与实际合成推理验收。

Related: [Business LLM Integration Case Studies](../architecture/business-llm-integration-case-studies.md), [Model Gateway Routing](../architecture/model-gateway-routing.md).

> 本记录不是模型质量排名，也不是运行时配置变更。除少量合成 PONG 请求外，没有执行任何 BodySense 用户健康数据请求，没有启用新的 Fallback。免费模型名、限流与可用性都会变化，实际接入前必须重新读取官方目录。

## 历史接入现状（2026-10-23 GCP Dev，非 WSL 当前状态）

| 内容 | 历史 GCP 记录（不代表 WSL/Oracle2 的当前状态） |
| --- | --- |
| 凭据文件 | `/home/dev/projects/bodysense/.env.dev.local`，权限 `0600`；`OPENROUTER_API_KEY` / `GROQ_API_KEY` 都存在且非占位符；不记录明文 |
| 当时运行的 GCP LiteLLM | `127.0.0.1:20112`，默认 `docker/litellm/config.yaml`；当前业务流量**未切换** |
| OpenRouter 已有映射 | 默认网关的部分通用备用模型和 Posture 视觉模型使用 OpenRouter，但映射到老的 / 非 `:free` ID；**不应直接视为免费渠道** |
| Groq 映射 | 当时 GCP `docker/docker-compose.yml` 没有给 `litellm-gateway` 注入 `GROQ_API_KEY`，默认 LiteLLM 配置也没有 Groq 模型 |
| Oracle2 共享 LiteLLM | PostgreSQL 动态模型表中尚未发现 OpenRouter / Groq 别名；不应假定已由 Oracle2 统一托管这两家 |
| 实施范围 | 仅记录连接与适配方式，**不修改运行配置、数据库、生产/staging 或 Oracle2 共享渠道**；Groq Key 已验证但默认网关仍未配置 Groq 路由 |

## 历史 GCP 合成 API 验证（2026-10-23；不计入 WSL 验收）

| API / 模型 | HTTP | 结论 |
| --- | --- | --- |
| OpenRouter `GET /api/v1/key`，GCP Key | 200 | API Key 认证成功；`is_free_tier=true`；发起探针前免费日配额 `used=0, limit=50, remaining=50` |
| OpenRouter `GET /api/v1/models` | 200 | 在抓取时筛选 `:free` 且输入/输出标价均为零，得到 **17 个**可选 ID |
| OpenRouter `google/gemma-4-31b-it:free` | 429 | 返回“Provider returned error”；模型目录中存在不保证当前上游可承接 |
| OpenRouter `nvidia/nemotron-3.5-super-vl:free` | 200 | **合成纯文本**请求成功，耗时约 0.95 秒、总计 75 tokens；未测试图片、JSON、工具或业务正确性 |
| Groq `GET /openai/v1/models`，使用 Python urllib | 403 | Cloudflare `error code: 1010`，属于该客户端特征被上游屏蔽；**不能由此判断 Key 无效** |
| Groq 同一 URL，无 Key，GCP / Oracle2 urllib | 403 / 403 | 同样为 Cloudflare 层拒绝；证据表明是客户端层兼容性问题，并非 Groq JSON 鉴权错误 |
| Groq `GET /openai/v1/models`，现有 OpenAI Python SDK | **200** | **GCP Key 已通过真实认证**，返回 11 个模型；无需更换 Key |
| Groq `openai/gpt-oss-20b`，现有 OpenAI Python SDK | **200** | 合成文本调用成功，约 0.24 秒，121 tokens；未测工具、JSON、视觉或实际业务 |

这里的 HTTP 200 只证明某次合成调用成功；HTTP 429 不等于模型永久不可用。Groq 在 `urllib` 的 403 已精确定位为 Cloudflare `1010`（客户端特征），且官方兼容的 OpenAI SDK 现已调用成功。**不要通过不受信任的代理、伪造凭据或绕过供应商访问控制来解决错误**。

## 当前只用于后续接入调研的候选

### OpenRouter

官方 API：`https://openrouter.ai/api/v1`；免费变体通常带 `:free`。适合继续检查的 ID（按当前目录快照，不是质量排名）：

- `nvidia/nemotron-3.5-super-vl:free` — 纯文本连通性已实测；其视觉能力未进行 BodySense 验证。
- `nvidia/nemotron-3-super-120b-a12b:free` — 目录存在，未调用。
- `xingchen-agi/xing4.0-29b-a4b:free` — 目录宣称有工具及结构化相关能力，未调用。
- `dots-studio/dots-3-note-preview:free` — 目录存在，未调用。
- `google/gemma-4-31b-it:free` — 目录存在，但本次返回 429。

`openrouter/free` 会从免费模型池自动选模型，**不适合做需要固定物理模型与可复现能力的业务路由或测评**。注意免费模型日额度受账号历史充值影响；历史 GCP 账号在探针前为 50/day；WSL 的账号权限、凭据与当前额度均未验证。

参考：[官方免费模型列表](https://openrouter.ai/collections/free-models)、[Free Router](https://openrouter.ai/docs/guides/routing/routers/free-router)、[Rate Limits](https://openrouter.ai/docs/api-reference/limits)、[Provider 数据政策](https://openrouter.ai/privacy/)。

### Groq

官方 OpenAI 兼容 API：`https://api.groq.com/openai/v1`。使用项目现有的 `apps/ai-service/.venv` 内 OpenAI SDK，`models.list()` 返回 11 个模型、`openai/gpt-oss-20b` 的最小文本请求成功。**仅历史 GCP Key 曾验证有效，不能推断 WSL Key 存在或有效**。此前裸 `urllib` 返回 `403 error code: 1010`，Cloudflare 官方解释为客户端浏览器特征被拦截；优先使用受支持的正式 SDK，不应直接把此类 403 归咎于 Key。

官方 Free Plan rate-limit 表中可查到 `openai/gpt-oss-120b`、`openai/gpt-oss-20b` 和 `qwen/qwen3.8-27b` 等模型；**Free Plan 可以受限免费调用**，不代表这些模型的标价为每百万 Token 0 美元，也不代表你的项目已获权限。准确的账号权限/配额必须登录控制台确认。

后续如某客户端仍报 `1010`，使用供应商支持的 OpenAI/Groq SDK 确认，不直接模仿非标准客户端；如正式 SDK 也失败，再查看 Groq [组织/项目模型权限](https://console.groq.com/docs/model-permissions)、[API 错误类型](https://console.groq.com/docs/errors)、[当前模型列表](https://console.groq.com/docs/models)、[账号限流](https://console.groq.com/docs/rate-limits)。[Cloudflare 1010](https://developers.cloudflare.com/support/troubleshooting/http-status-codes/cloudflare-1xxx-errors/error-1010/) 的定义为基于客户端特征的访问拒绝。不得通过不受信任的代理绕过访问控制。

## 未来接入时的正确边界（未实施）

1. **明确入口归属**：通过 Oracle2 LiteLLM/ProviderAdmin 作为供应商与模型权威；专用 BodySense Virtual Key 仍待按既有授权建立和限制。不混用 Oracle2 Master Key、T3 共享客户端 Key 或历史 GCP Key，不将供应商 Key 暴露给 Go、Python Agent 或浏览器。本 PR 不注册模型、不启用新路由、不修改 Oracle2/生产环境。
2. **固定别名与能力**：独立的物理模型别名；逻辑 `bodysense-consultation`、`bodysense-structured`、`bodysense-diagnosis`、`bodysense-posture` 保持不变；Posture 模型需真视觉验收。
3. **故障与隐私**：区分连接失败、模型不存在、429、401/403、400 Schema 错误；不把任意免费模型自动当替代渠道。免费服务可能由第三方保留或利用输入，应在处理真实健康数据前审核供应商政策、实际路由、用户知情与数据保护控制。
4. **测评明确延期**：先完成 Key/模型列表/合成连接的资格确认；跨模型业务排名、Prompt/Agent 质量、Champion/Challenger 和自动选型另立阶段，不在本次执行。

## 当前 WSL 无凭据复核（不执行推理）

以下命令仅从 `wsl-local` 检查公开目录、Oracle2 HTTPS 连通性与匿名鉴权拒绝；**它们不是 OpenRouter/Groq 账户认证或端到端验收**。不要连接已退役的 GCP，不要从旧 GCP 路径加载 `.env.dev.local`，也不要在日志中输出任何密钥或原始账户响应。

~~~bash
cd /home/baker/projects/bodysense
# 公开目录无须 Key；只输出条目数，不把目录或账户数据写入公开日志。
curl -fsS --max-time 15 'https://openrouter.ai/api/v1/models' \
  | python3 -c 'import json,sys; print("public_models=", len(json.load(sys.stdin).get("data", [])))'

curl -sS -o /dev/null -w 'oracle_health_http=%{http_code} tls=%{ssl_verify_result}\n' \
  --max-time 10 'https://oracle.taile92a8e.ts.net:10446/health/liveliness'
curl -sS -o /dev/null -w 'anonymous_models_http=%{http_code} tls=%{ssl_verify_result}\n' \
  --max-time 10 'https://oracle.taile92a8e.ts.net:10446/v1/models'
# 预期：200 / 401，均需 tls=0；这仅证明连通性和匿名鉴权边界。
~~~

**凭据与合成调用待验收：**只有独立确认 WSL 的合法免费额度、ProviderAdmin 供应商身份、BodySense 专用受限 Virtual Key、模型精确 ID/协议及 Posture 费用边界后，才可通过授权的安全密钥助手执行最小固定路由合成请求。Groq 优先使用其支持的 SDK；遇 401/403/429 不绕过访问控制或强制解封。真实业务健康数据、模型评测与优选均不属于本阶段。历史 GCP 的 200 或 429 不等于当前 WSL 结果。
