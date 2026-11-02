# OpenRouter / Groq：免费渠道连通性与接入边界

Status: **Historical GCP research only; WSL/Oracle2 acceptance pending**
Snapshot: 2026-10-23 (GCP-dev-01 retired on the same date)
Current main development workspace: Agent Fabric `wsl-local`, `/home/baker/projects/bodysense`;
persistent Agent/API host: `oracle2`. No GCP staging or development host is valid.

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

## 2026-11-02 WSL / Oracle2 网关身份核对（只读，非供应商验收）

- `wsl-local` 为唯一主开发工作区，已有 PR #486 的 direct-dev 默认网关使用只读 `docker/litellm/config.dev-offline.yaml`（`model_list: []`），禁止外部推理。
- Oracle2 的 **BodySense 专用开发实例** 是 `bodysense-dev-infra-litellm-gateway-1`，仅监听 Oracle2 的 `127.0.0.1:20112`，当前挂载旧版 `docker/litellm/config.yaml`（8 个逻辑路由）；此配置**不是** PR #486 的离线配置，Groq 路由未在运行实例中启用。
- `https://oracle.taile92a8e.ts.net:10446/v1` 经 Tailscale Serve 反代到 Oracle2 的 `127.0.0.1:4000`，实际为 **共享 `hermes-litellm`**，不是 BodySense 专用实例。该 URL 的健康检查 200 / 匿名模型请求 401 **不得作为 BodySense 专用网关连通性或验收证据**。
- Oracle2 本机专用实例的 `/health/liveliness` 为 200，匿名 `/v1/models` 为 401；这只验证其本机存活及匿名拒绝，**不证明 WSL 可以远程访问、不证明供应商授权或零费用**。目前尚无已确认的 WSL→专用实例独立 HTTPS 入口。
- 未来网络方案需独立审批，至少满足专用服务身份、tailnet-only HTTPS、BodySense 专属最小权限 Virtual Key、服务端凭据隔离、禁止付费路由及明确回滚；**不可复用共享 Hermes URL/密钥**。未审批前不得创建端口转发、变更 Tailscale Serve、防火墙、生产或共享模型注册表。

## 2026-11-02 官方零费用约束核查（仅政策研究，未授权调用）

此处只核对供应商公开规则，**不进行模型质量评测、选型或运行时路由变更**；官方免费政策不是本项目账户权限或账单担保。

- **OpenRouter**：[Free Models](https://openrouter.ai/collections/free-models) 明确列出的 `:free` 变体为零模型单价，但免费请求有限额（通常 20 RPM、50 RPD，失败尝试也计入日限额）；[Pricing](https://openrouter.ai/pricing) 显示同一普通 API Key 可调用收费模型，Free 计划不提供预算/费用控制。**因此仅有免费账号/API Key 并不是零费用技术隔离**。必须在网关与专用 Virtual Key 双层使用已核实的精确 `:free` ID 白名单，检查文本与视觉/图片等实际请求的所有计价项，不启用 `openrouter/free`、自动付费回退或附加计费服务；不满足任一条件直接拒绝请求。查询目录不等于发起计费调用。
- **Groq**：[Billing FAQ](https://console.groq.com/docs/billing-faqs) 区分 Free 和按量计费的 Developer 计划；[Rate Limits](https://console.groq.com/docs/rate-limits) 以组织为单位计数。必须先在**实际使用的组织/项目**核实 Free 层级、模型授权及剩余额度，只对白名单模型执行受限请求。[Spend Limits](https://console.groq.com/docs/spend-limits) 仅面向付费账户，费用统计延迟约 10–15 分钟，**不能替代严格零费用门禁**。无法证明零计费时保持禁用。
- **审计顺序**：服务身份及仅 tailnet 可达 → 账号/组织级 Free 权限 → 精确路由、协议与多模态费用核验 → BodySense-only 最小权限 Virtual Key → 无真实健康资料的固定模型合成调用 → 业务 E2E 与独立审查。任何模糊价格、Key 权限、数据保留政策或 401/403/429 都中止对应上游；不得静默切换到付费或共享 Hermes 账户。

## 未来接入时的正确边界（未实施）

1. **明确入口归属**：通过 Oracle2 LiteLLM/ProviderAdmin 作为供应商与模型权威；专用 BodySense Virtual Key 仍待按既有授权建立和限制。不混用 Oracle2 Master Key、T3 共享客户端 Key 或历史 GCP Key，不将供应商 Key 暴露给 Go、Python Agent 或浏览器。本 PR 不注册模型、不启用新路由、不修改 Oracle2/生产环境。
2. **固定别名与能力**：独立的物理模型别名；逻辑 `bodysense-consultation`、`bodysense-structured`、`bodysense-diagnosis`、`bodysense-posture` 保持不变；Posture 模型需真视觉验收。
3. **故障与隐私**：区分连接失败、模型不存在、429、401/403、400 Schema 错误；不把任意免费模型自动当替代渠道。免费服务可能由第三方保留或利用输入，应在处理真实健康数据前审核供应商政策、实际路由、用户知情与数据保护控制。
4. **测评明确延期**：先完成 Key/模型列表/合成连接的资格确认；跨模型业务排名、Prompt/Agent 质量、Champion/Challenger 和自动选型另立阶段，不在本次执行。

## 当前无凭据复核（不执行推理）

在 `wsl-local` 只能无密钥读取 OpenRouter 公开目录；**不以共享 Hermes 的 :10446 作为 BodySense 服务入口**。公开目录不是当前账户的权限或费用凭证。不要连接已退役 GCP，也不要加载其历史 `.env.dev.local`。

~~~bash
# 在 wsl-local 执行；不传递凭据或用户健康数据。
cd /home/baker/projects/bodysense
curl -fsS --max-time 15 'https://openrouter.ai/api/v1/models' \
  | python3 -c 'import json,sys; print("public_models=", len(json.load(sys.stdin).get("data", [])))'
~~~

Oracle2 专用网关只能在 **Oracle2 本机**进行下列只读匿名检查，不代表从 WSL 可达；本机 `127.0.0.1` 绝不能误用为 WSL 的回环地址。

~~~bash
# 仅在 Agent Fabric oracle2 目标内执行；不创建隧道、不改动服务或密钥。
curl -sS -o /dev/null -w 'bodysense_local_health_http=%{http_code}\n' \
  --max-time 5 'http://127.0.0.1:20112/health/liveliness'
curl -sS -o /dev/null -w 'bodysense_local_anonymous_models_http=%{http_code}\n' \
  --max-time 5 'http://127.0.0.1:20112/v1/models'
# 预期：200 / 401。仅为本机存活与匿名拒绝；不构成提供商/业务验收。
~~~

**凭据与合成调用待验收：**只有独立确认 WSL 的合法免费额度、ProviderAdmin 供应商身份、BodySense 专用受限 Virtual Key、模型精确 ID/协议及 Posture 费用边界后，才可通过授权的安全密钥助手执行最小固定路由合成请求。Groq 优先使用其支持的 SDK；遇 401/403/429 不绕过访问控制或强制解封。真实业务健康数据、模型评测与优选均不属于本阶段。历史 GCP 的 200 或 429 不等于当前 WSL 结果。
