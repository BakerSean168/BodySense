# OpenRouter / Groq：免费渠道连通性与接入边界

Status: **GCP Dev research / synthetic connectivity only**
Snapshot: 2026-10-23
Related: [Business LLM Integration Case Studies](../architecture/business-llm-integration-case-studies.md), [Model Gateway Routing](../architecture/model-gateway-routing.md).

> 本记录不是模型质量排名，也不是运行时配置变更。除少量合成 PONG 请求外，没有执行任何 BodySense 用户健康数据请求，没有启用新的 Fallback。免费模型名、限流与可用性都会变化，实际接入前必须重新读取官方目录。

## 接入现状（GCP Dev）

| 内容 | 当前证据 |
| --- | --- |
| 凭据文件 | `/home/dev/projects/bodysense/.env.dev.local`，权限 `0600`；`OPENROUTER_API_KEY` / `GROQ_API_KEY` 都存在且非占位符；不记录明文 |
| 正在运行的 GCP LiteLLM | `127.0.0.1:20112`，默认 `docker/litellm/config.yaml`；当前业务流量**未切换** |
| OpenRouter 已有映射 | 默认网关的部分通用备用模型和 Posture 视觉模型使用 OpenRouter，但映射到老的 / 非 `:free` ID；**不应直接视为免费渠道** |
| Groq 映射 | GCP `docker/docker-compose.yml` 目前没有给 `litellm-gateway` 注入 `GROQ_API_KEY`，默认 LiteLLM 配置也没有 Groq 模型 |
| Oracle2 共享 LiteLLM | PostgreSQL 动态模型表中尚未发现 OpenRouter / Groq 别名；不应假定已由 Oracle2 统一托管这两家 |
| 实施范围 | 仅记录连接与适配方式，**不修改运行配置、数据库、生产/staging 或 Oracle2 共享渠道**；Groq Key 已验证但默认网关仍未配置 Groq 路由 |

## 已执行的最小 API 验证（仅合成输入）

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

`openrouter/free` 会从免费模型池自动选模型，**不适合做需要固定物理模型与可复现能力的业务路由或测评**。注意免费模型日额度受账号历史充值影响；本次账号在探针前为 50/day，当前剩余量以 `GET /api/v1/key` 为准。

参考：[官方免费模型列表](https://openrouter.ai/collections/free-models)、[Free Router](https://openrouter.ai/docs/guides/routing/routers/free-router)、[Rate Limits](https://openrouter.ai/docs/api-reference/limits)、[Provider 数据政策](https://openrouter.ai/privacy/)。

### Groq

官方 OpenAI 兼容 API：`https://api.groq.com/openai/v1`。使用项目现有的 `apps/ai-service/.venv` 内 OpenAI SDK，`models.list()` 返回 11 个模型、`openai/gpt-oss-20b` 的最小文本请求成功。**Key 已确认有效**。此前裸 `urllib` 返回 `403 error code: 1010`，Cloudflare 官方解释为客户端浏览器特征被拦截；优先使用受支持的正式 SDK，不应直接把此类 403 归咎于 Key。

官方 Free Plan rate-limit 表中可查到 `openai/gpt-oss-120b`、`openai/gpt-oss-20b` 和 `qwen/qwen3.8-27b` 等模型；**Free Plan 可以受限免费调用**，不代表这些模型的标价为每百万 Token 0 美元，也不代表你的项目已获权限。准确的账号权限/配额必须登录控制台确认。

后续如某客户端仍报 `1010`，使用供应商支持的 OpenAI/Groq SDK 确认，不直接模仿非标准客户端；如正式 SDK 也失败，再查看 Groq [组织/项目模型权限](https://console.groq.com/docs/model-permissions)、[API 错误类型](https://console.groq.com/docs/errors)、[当前模型列表](https://console.groq.com/docs/models)、[账号限流](https://console.groq.com/docs/rate-limits)。[Cloudflare 1010](https://developers.cloudflare.com/support/troubleshooting/http-status-codes/cloudflare-1xxx-errors/error-1010/) 的定义为基于客户端特征的访问拒绝。不得通过不受信任的代理绕过访问控制。

## 未来接入时的正确边界（未实施）

1. **明确入口归属**：现有 Oracle2 专用 Key 尚待合法创建时，不混用其 Master Key。若把 OpenRouter/Groq 直接配置在 GCP dev-only LiteLLM，也只能由 LiteLLM 容器持有真实供应商 Key，不能让 Go / Python Agent 和浏览器持有。
2. **固定别名与能力**：独立的物理模型别名；逻辑 `bodysense-consultation`、`bodysense-structured`、`bodysense-diagnosis`、`bodysense-posture` 保持不变；Posture 模型需真视觉验收。
3. **故障与隐私**：区分连接失败、模型不存在、429、401/403、400 Schema 错误；不把任意免费模型自动当替代渠道。免费服务可能由第三方保留或利用输入，应在处理真实健康数据前审核供应商政策、实际路由、用户知情与数据保护控制。
4. **测评明确延期**：先完成 Key/模型列表/合成连接的资格确认；跨模型业务排名、Prompt/Agent 质量、Champion/Challenger 和自动选型另立阶段，不在本次执行。

## 复查命令（只列模型和配额，不泄露 Key）

~~~bash
# 在 GCP 的 BodySense 工作区，禁止 set -x 或 echo API Key。
set -a
source .env.dev.local
set +a

# 只读模型目录（无需 Key）。
curl -fsS 'https://openrouter.ai/api/v1/models'
# 只读当前 Key 限额。
curl -fsS -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  'https://openrouter.ai/api/v1/key'
# Groq 请使用项目已安装的 OpenAI SDK；裸 urllib 曾触发 Cloudflare 1010。
apps/ai-service/.venv/bin/python - <<'MODEL_LIST'
import os
from openai import OpenAI
client = OpenAI(
    api_key=os.environ["GROQ_API_KEY"],
    base_url="https://api.groq.com/openai/v1",
    max_retries=0,
)
print(sorted(model.id for model in client.models.list().data))
MODEL_LIST
~~~

**不要把以上 curl 的完整响应粘贴到公开日志或 Issue**：虽然常规 Key/Models 响应不应返回 Key 明文，日志仍可能包含账户级元数据。只报告状态、模型 ID、免费配额，不打印凭据。
