# 闭环功能拆解 — 从头到尾走通一个特性

> "闭环"指：一个功能从**前端触发 → 后端处理 → 数据落地 → 结果返回 → 前端展示**的完整链路。
> 读懂闭环，才能理解各层代码"为谁服务、为什么这样写"。
> 本文挑 3 条最能代表全栈基础的闭环。

---

## 闭环一：用户注册 / 登录（Go 后端 + React 前端，最经典）

这条闭环把 Go 三层架构和 React 状态管理**完整串起来**，是全项目最值得先吃透的一条。

### 全景时序

```text
[React] LoginForm 提交
   │  fetch POST /api/v1/auth/login  { email, password }
   ▼
[Go] handler.Login
   │  ShouldBindJSON 解析 + 校验
   ▼
[Go] service.Login
   │  ① userRepo.FindByEmail 查用户
   │  ② bcrypt.CompareHashAndPassword 验密码
   │  ③ generateTokens：签 JWT + 存 refresh token 到 Redis + 写会话缓存
   ▼
[Go] 返回 { access_token, refresh_token, expires_in }
   ▼
[React] authStore.login
   │  set({ accessToken, isAuthenticated:true }) + persist 到 localStorage
   │  fetchUser() 拉用户信息
   ▼
[React] navigate('/dashboard') 跳转
```

### 之后每个受保护请求的闭环（鉴权）

```text
[React] fetch /api/v1/me  header: Authorization: Bearer <accessToken>
   ▼
[Go] middleware.AuthMiddleware
   │  ① 取 Bearer token
   │  ② auth.ValidateAccessToken 验签名+过期（显式检查 HS256，防 alg 混淆）
   │  ③ verifyUserExists：先查 Redis 会话缓存（热路径）→ 未命中查 DB → 回写缓存
   │  ④ c.Set("user_id", ...) 塞进 context
   ▼
[Go] handler.Me 从 context 取 user_id → 返回用户信息
   ▼
[React] 401 时 → authStore.refreshAccessToken（带并发去重锁）→ 重试
```

### 关键代码锚点（对照真实文件读）

| 层 | 文件 | 关键点 |
|---|---|---|
| 前端组件 | `features/auth/components/LoginForm.tsx` | `useState` 表单、`handleSubmit`、受控组件 |
| 前端状态 | `stores/authStore.ts` | `login`/`refreshAccessToken`、`persist`、并发去重锁 `refreshPromise` |
| HTTP 层 | `internal/handler/auth_handler.go` | `ShouldBindJSON`、状态码映射、`c.JSON` |
| 业务层 | `internal/service/auth_service.go` | bcrypt、`errors.Is(gorm.ErrRecordNotFound)`、`fmt.Errorf("%w")` |
| 数据层 | `internal/repository/user_repository.go` | GORM `Where().First()`、`Count` |
| 模型 | `internal/model/user.go` | struct tag、`json:"-"` 保护密码 |
| JWT | `internal/auth/jwt.go` | 签名/校验、`crypto/rand` 生成 refresh token |
| 中间件 | `internal/middleware/auth.go` | 两级校验（Redis→DB）、优雅降级 |

### 这条闭环教会你的"基础全景"
- Go：三层架构、依赖注入、错误处理、context、struct tag、JWT、中间件。
- React：useState、受控组件、Zustand、fetch、localStorage 持久化、并发去重。
- 通用：HTTP 方法/状态码、JSON 契约、Bearer token 鉴权、缓存优先 + DB 兜底。

### 安全设计里的"为什么"（面试常问）
- 登录失败统一返回"invalid email or password"，不区分"邮箱不存在"和"密码错" → 防止账号枚举。
- 密码用 bcrypt（cost=12）哈希，`json:"-"` 保证哈希永不出现在响应里。
- access token 短期（2h）+ refresh token 长期（30d，存 Redis 可撤销）→ 兼顾安全与体验。
- 中间件二级校验：Redis 命中走热路径；Redis 挂了降级查 DB，不因缓存故障把所有人登出。

---

## 闭环二：体态照片 AI 分析（Python，多模态 + 安全治理）

这条闭环展示 **Python async + 大模型调用 + 结构化输出 + 安全治理**，是 AI 服务的代表。

### 全景时序

```text
[前端/Go worker] POST /api/posture/analyze  (multipart: view + file)
   ▼
[Python] routes/posture.analyze
   │  ① 校验 view 合法（front/side/back）
   │  ② 校验 content_type 在白名单
   │  ③ await file.read() 读字节；校验非空 + ≤10MB
   ▼
[Python] services/posture_analyzer.analyze_posture
   │  ① 图片 → base64 → 拼成多模态 messages（system + user[text+image_url]）
   │  ② await ai.generate(...) 调 VLM，要求 json_object 输出
   │  ③ json.loads 解析（失败则降级为空 dict）
   ▼
[Python] govern_posture_result（确定性治理，不调大模型）
   │  ① 按视角白名单过滤 findings（丢弃跨视角乱猜）
   │  ② 抹掉 metric（Phase 1 抗数值幻觉）
   │  ③ 归一化 severity/confidence 非法值
   │  ④ 强制 disclaimer；高召回红旗扫描并合并去重
   │  ⑤ 缺必填字段 → 降级 overall_confidence 而非硬失败
   ▼
[Python] 返回 PostureAnalysisResponse（Pydantic 校验形状）
   ▼
[前端] 展示分析结果 + 免责声明 + 红旗警示
```

### 关键代码锚点

| 层 | 文件 | 关键点 |
|---|---|---|
| 路由 | `api/routes/posture.py` | `@router.post`、`Form/File`、`await file.read()`、`HTTPException`、`raise...from` |
| 业务 | `services/posture_analyzer.py` | 模块单例、base64、多模态 messages、`json.loads` 容错、治理函数 |
| 模型 | `models/posture.py` | `BaseModel`、`Field(...)`、`Literal`、`X | None`、`default_factory` |

### 这条闭环教会你的"基础全景"
- Python：async/await、异常处理与异常链、f-string、dict 操作、推导式、`**` 解包。
- Pydantic：数据契约、必填 vs 可选、可变默认值陷阱。
- AI 工程：多模态消息结构、结构化输出、"大模型不可信 → 确定性治理兜底"的设计思想。

### 设计里的"为什么"
- **治理与生成分离**：`analyze_posture`（调模型，不确定）和 `govern_posture_result`（纯逻辑，可单测）拆开 → 安全规则可独立测试，不受模型波动影响。
- **抗幻觉**：Phase 1 没有真实几何计算，就强制抹掉所有数值 metric，宁可不给也不给假数据。
- **降级而非失败**：结果不完整时降低置信度并保留可用部分，比直接报错体验更好（健康场景尤其重要）。

---

## 闭环三：AI 问诊流式对话（跨三端 + SSE，进阶挑战）

这条最复杂，横跨 Go / Python / React 三端，是项目的"皇冠明珠"。**建议在前两条吃透后再攻**。

### 全景时序（简化）

```text
[React] 发送消息 → POST /api/v1/consultation-runs（SSE 长连接）
   ▼
[Go] consultation runtime：建 run → 转发给 Python Agent 运行时
   ▼
[Python] LangGraph Agent：推理 → 可能调用工具（提取症状/检索知识/ask_user）
   │   逐 token 产出 → 逐事件回传 Go
   ▼
[Go] 每个对外事件持久化为 runtime_event（append-only），再经 SSE 推给前端
   ▼
[React] useSSEProcessor 解析 SSE 流 → 按事件类型分发 → 打字机渲染 + 更新信息面板
```

### 关键代码锚点

| 端 | 文件 | 关键点 |
|---|---|---|
| 前端解析 | `features/consultation/hooks/useSSEProcessor.ts` | ReadableStream、TextDecoder、事件映射分发（**注释最全，先读它**） |
| 前端运行时 | `features/consultation/hooks/useAssistantChatRuntime.ts` | useRef/useCallback、流式状态累加 |
| Go 运行时 | `internal/consultation/runtime.go` | 事件持久化 `recordPublicEvent`、回放 `replayCompletedRun` |
| Python Agent | `services/agent/orchestrator.py` | 工具调用、`ask_user` 中断、fallback |

### 为什么它值得作为"终极练习"
- 它综合了前两条闭环的所有基础，再叠加：SSE 流式协议、事件溯源、HITL（人在回路）Agent、跨语言契约。
- 但也正因如此，**不适合入门**。把它当作学完基础后的"综合应用关卡"。

---

## 学习建议：按这个顺序读闭环

1. **闭环一（注册/登录）** — 先吃透。它把 Go 三层 + React 状态讲全了，且逻辑直白。
2. **闭环二（体态分析）** — 再攻 Python。async + Pydantic + 治理，中等难度。
3. **闭环三（流式问诊）** — 最后挑战。前两条的所有基础在这里综合应用。

> 每读完一条闭环，试着**用自己的话画一遍时序图**（不看本文），能画出来才算真懂。
