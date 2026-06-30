# AI Run / Job Runtime 架构设计

**文档版本**：v1.0  
**更新日期**：2026-06-29  
**状态**：设计稿  
**适用范围**：OCR、评估报告生成、训练计划生成、重评估、标题生成、知识库入库、Agent resume

---

## 1. 背景

BodySense 已经有多类耗时任务：

- 上传体检报告后触发 OCR。
- 生成健康评估报告。
- 生成训练计划。
- 提交训练反馈后触发重评估。
- 咨询会话中异步生成标题。
- 视频知识入库管道执行 ASR、切分、精修、clip 导出和入库。
- 工具调用工程化后会新增 `ask_user` resume、confirmation、长工具执行。

当前这些任务分散在不同 Module 中：

- Go `UploadService.processOCR` 使用 goroutine 后台执行。
- `AssessmentService.GenerateAssessment` 同步调用 Python AI。
- `TrainingService.GeneratePlan` 和 `AnalyzeFeedback` 同步调用 Python AI。
- Python `VideoIngestionPipeline` 作为本地编排管道存在，但没有统一 job 状态。
- Conversation run 已存在，但主要服务聊天流，不覆盖所有 AI 任务。

这会带来几个问题：

- 任务状态不可统一查询。
- 失败、重试、超时、取消策略不一致。
- 前端很难展示统一进度。
- 服务重启后 goroutine 中的 OCR 任务不可恢复。
- 不同任务的 AI 请求、耗时、错误和产物难以审计。

本设计目标是把“AI 相关长任务”从各业务 Module 里的临时调用升级为统一的 **AI Run / Job Runtime**。

---

## 2. 设计目标

### 2.1 目标

1. **统一 Job Interface**  
   OCR、评估、训练计划、知识入库等任务都通过统一 Job Runtime 创建、执行、查询和恢复。

2. **区分 Run 和 Job**
   - `run`：一次对话或 AI 执行记录，偏 LLM 调用和流式会话。
   - `job`：一个可持久化、可重试、可恢复的后台任务，可能包含多个 run。

3. **Go 作为 Job 真值**
   Go 管理用户权限、job 状态、幂等、重试、产物落库和对外查询。

4. **Python 执行 AI 子任务**
   Python 负责 OCR、评估、训练计划、RAG 入库等具体计算，不保存正式 job 状态。

5. **支持前端进度展示**
   前端能通过轮询、SSE 或后续 WebSocket 查询任务状态和进度。

6. **支持失败恢复**
   服务重启后，`pending` / `running` 的任务可以被扫描、恢复、标记失败或重新执行。

### 2.2 非目标

1. 第一版不引入复杂分布式任务系统。
2. 第一版不要求多 worker 横向调度。
3. 第一版不实现 DAG 编排。
4. 第一版不把所有同步接口立刻改成异步。

---

## 3. 核心概念

### 3.1 Job

Job 是一个业务可见的后台任务。

示例：

```txt
ocr.extract_report
assessment.generate
training.generate_plan
training.reassess
knowledge.ingest_video
conversation.generate_title
agent.resume
```

### 3.2 Job Step

Job Step 是 Job 内部的执行阶段。

示例：

```txt
ocr.extract_report:
  - validate_file
  - call_python_ocr
  - parse_indicators
  - persist_result

knowledge.ingest_video:
  - extract_audio
  - transcribe
  - split_units
  - ai_refine
  - export_clips
  - ingest_vectors
```

### 3.3 Job Artifact

Job Artifact 是任务产物。

示例：

```txt
OCR JSON
assessment report id
training plan id
generated_pack.json path
curated_pack.json path
embedding batch stats
```

---

## 4. 总体架构

```txt
React UI
  - 创建任务
  - 展示 job status / progress / result
  - 重试或取消
        |
        v
Go API
  - JobRuntime
  - JobRepository
  - JobWorker
  - Auth / ownership
  - Idempotency
  - Artifact persistence
        |
        v
Python AI Service
  - OCR
  - Assessment
  - Training generation
  - Reassessment
  - Knowledge ingestion
        |
        v
PostgreSQL / File Storage / pgvector
```

---

## 5. Module 设计

### 5.1 JobRuntime

**位置建议**：`apps/api/internal/job/runtime.go`

JobRuntime 是外部调用入口。

Interface：

```go
type JobRuntime interface {
    Enqueue(ctx context.Context, req EnqueueJobRequest) (*model.Job, error)
    Get(ctx context.Context, jobID, userID uuid.UUID) (*model.Job, error)
    Cancel(ctx context.Context, jobID, userID uuid.UUID) error
    Retry(ctx context.Context, jobID, userID uuid.UUID) (*model.Job, error)
}
```

职责：

- 校验用户权限。
- 处理幂等 key。
- 创建 job 记录。
- 将 job 交给 worker。
- 查询和取消 job。

不负责：

- 具体 AI 业务逻辑。
- Python endpoint 的私有响应解析。

### 5.2 JobWorker

**位置建议**：`apps/api/internal/job/worker.go`

JobWorker 负责执行可运行的 job。

Interface：

```go
type JobWorker interface {
    Start(ctx context.Context) error
    Execute(ctx context.Context, job *model.Job) error
}
```

第一版可以使用进程内 worker：

```txt
Enqueue -> DB insert -> goroutine Execute
Startup -> scan stale running jobs -> mark failed or retry
```

后续可以替换为 Redis queue / Postgres queue / Temporal。这个替换点就是 JobRuntime 的 Seam。

### 5.3 JobHandler Adapter

每种任务是一个 Adapter。

```go
type JobHandler interface {
    Type() string
    Execute(ctx context.Context, job *model.Job) (*JobExecutionResult, error)
}
```

建议目录：

```txt
apps/api/internal/job/
  runtime.go
  worker.go
  handler.go
  registry.go
  errors.go

apps/api/internal/job/handlers/
  ocr_extract_report.go
  assessment_generate.go
  training_generate_plan.go
  training_reassess.go
  conversation_generate_title.go
  knowledge_ingest_video.go
```

### 5.4 JobRegistry

JobRegistry 保存 job type 到 handler 的映射。

```go
type JobRegistry struct {
    handlers map[string]JobHandler
}
```

规则：

- 未注册 job type 不能执行。
- handler 必须声明默认超时、最大重试次数、是否用户可取消。
- dangerous job 需要用户确认或管理员权限。

---

## 6. 数据库设计

### 6.1 jobs

```sql
CREATE TABLE jobs (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id      UUID REFERENCES conversations(id) ON DELETE SET NULL,
    run_id              UUID REFERENCES runs(id) ON DELETE SET NULL,

    type                VARCHAR(80) NOT NULL,
    status              VARCHAR(30) NOT NULL,
    -- pending / running / waiting_user / completed / failed / cancelled

    input               JSONB NOT NULL DEFAULT '{}',
    progress            JSONB NOT NULL DEFAULT '{}',
    result              JSONB,
    error               JSONB,
    artifacts           JSONB NOT NULL DEFAULT '[]',

    idempotency_key     TEXT,
    attempt             INT NOT NULL DEFAULT 0,
    max_attempts        INT NOT NULL DEFAULT 1,

    started_at          TIMESTAMPTZ,
    finished_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX idx_jobs_user_created
    ON jobs (user_id, created_at DESC);

CREATE INDEX idx_jobs_status
    ON jobs (status, created_at);
```

### 6.2 job_events

```sql
CREATE TABLE job_events (
    id          UUID PRIMARY KEY DEFAULT uuidv7(),
    job_id      UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    seq         INT NOT NULL,
    type        VARCHAR(80) NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (job_id, seq)
);
```

`job_events` 用于审计和前端恢复进度。

---

## 7. 状态机

```txt
pending
  -> running
  -> completed
  -> failed
  -> cancelled

running
  -> waiting_user
  -> running

failed
  -> pending   (retry)
```

状态规则：

- `completed` 不可重试。
- `cancelled` 不自动恢复。
- `running` 超过 heartbeat TTL 后进入 `failed` 或重新排队。
- `waiting_user` 不算失败，不触发自动重试。

---

## 8. API 设计

### 8.1 创建 Job

```http
POST /api/v1/jobs
Content-Type: application/json
```

```json
{
  "type": "assessment.generate",
  "input": {
    "profile_id": "..."
  },
  "idempotency_key": "client-generated-key"
}
```

Response：

```json
{
  "id": "...",
  "type": "assessment.generate",
  "status": "pending"
}
```

### 8.2 查询 Job

```http
GET /api/v1/jobs/:id
```

Response：

```json
{
  "id": "...",
  "type": "assessment.generate",
  "status": "running",
  "progress": {
    "current_step": "call_python_assessment",
    "percent": 60
  },
  "result": null,
  "error": null
}
```

### 8.3 取消 Job

```http
POST /api/v1/jobs/:id/cancel
```

### 8.4 重试 Job

```http
POST /api/v1/jobs/:id/retry
```

---

## 9. SSE 事件

建议扩展 StreamEvent：

```txt
job.created
job.progress
job.completed
job.failed
job.cancelled
```

示例：

```json
{
  "channel": "job",
  "type": "job.progress",
  "payload": {
    "job_id": "job_...",
    "job_type": "ocr.extract_report",
    "current_step": "parse_indicators",
    "percent": 80
  }
}
```

---

## 10. 和现有 Module 的关系

### 10.1 UploadService

当前：

```txt
UploadFile -> goroutine processOCR
```

目标：

```txt
UploadFile -> JobRuntime.Enqueue(type=ocr.extract_report)
```

### 10.2 AssessmentService

当前：

```txt
GenerateAssessment -> sync HTTP Python -> persist report
```

目标：

```txt
GenerateAssessment -> enqueue assessment.generate
JobHandler -> call Python -> validate -> persist report
```

### 10.3 TrainingService

当前：

```txt
GeneratePlan / AnalyzeFeedback -> sync HTTP Python
```

目标：

```txt
training.generate_plan job
training.reassess job
```

### 10.4 Knowledge Ingestion

当前：

```txt
Python local pipeline writes files
```

目标：

```txt
Go admin endpoint -> knowledge.ingest_video job -> Python pipeline -> artifacts -> publish
```

---

## 11. 错误处理

统一错误结构：

```json
{
  "code": "PYTHON_AI_TIMEOUT",
  "message": "AI service timed out",
  "retryable": true,
  "details": {}
}
```

错误码建议：

```txt
UNKNOWN_JOB_TYPE
INVALID_JOB_INPUT
JOB_TIMEOUT
PYTHON_AI_UNAVAILABLE
PYTHON_AI_FAILED
ARTIFACT_WRITE_FAILED
PERSIST_RESULT_FAILED
JOB_CANCELLED
JOB_STALE_RUNNING
```

---

## 12. 测试策略

Go 单元测试：

```txt
apps/api/internal/job/runtime_test.go
apps/api/internal/job/worker_test.go
apps/api/internal/job/handlers/ocr_extract_report_test.go
apps/api/internal/job/handlers/assessment_generate_test.go
```

覆盖：

- 创建 job。
- 幂等 key 命中返回已有 job。
- handler 未注册时失败。
- handler 成功时写 result。
- handler 超时时写 error。
- retry 增加 attempt。
- cancel 后不再执行。

---

## 13. 分阶段落地

### Phase 1：Job 表和 Runtime

- 新增 `jobs`、`job_events`。
- 新增 JobRuntime、JobRegistry、JobWorker。
- 保留进程内 worker。

### Phase 2：迁移 OCR

- `UploadService` 不再直接 goroutine 调 OCR。
- 创建 `ocr.extract_report` handler。
- 前端上传列表显示 job 进度。

### Phase 3：迁移评估和训练计划

- `assessment.generate` job。
- `training.generate_plan` job。
- 前端生成页从同步 loading 改为 job 状态。

### Phase 4：迁移重评估和标题生成

- `training.reassess` job。
- `conversation.generate_title` job。

### Phase 5：知识入库任务

- `knowledge.ingest_video` job。
- 支持 artifacts 和发布状态。

---

## 14. 成功标准

落地后应满足：

1. 所有 AI 长任务都有统一状态。
2. OCR 服务失败后不会丢失任务。
3. 前端可展示任务进度和失败原因。
4. 用户重复点击生成不会重复创建报告或训练计划。
5. 服务重启后可恢复或安全失败 `running` job。
6. 后续替换 Redis queue / Temporal 时不需要改业务 handler。

