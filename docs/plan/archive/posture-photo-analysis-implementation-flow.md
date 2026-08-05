# 体态照片 AI 分析 — 实施流程与任务拆分
> ✅ **已完成并归档**（2026-07-29）。含 Phase 1/2/3-B1/3-B2。

> ✅ Phase 1/2/3-B1 已落地（2026-07-29）。本 flow 仅 Phase 3-B2 仍开放。


> 部分实施（更新于 2026-07-13）。基于 [posture-photo-analysis-plan.md](./posture-photo-analysis-plan.md) 生成，聚焦可直接执行的逐文件实施路径。
> 真值来源：当前代码。文中所有 `file:line` 为撰写时锚点，实施前请以最新代码为准。
> **进度**：Phase 1（纯 VLM MVP）已由 PR #39 实施上线；Phase 2 / 3-B1 / 3-B2 尚未实施，故本方案保留在 active。

## 总体路线

```text
Phase 1 VLM MVP（打通端到端，兑现文案承诺）
  -> Phase 2 姿态估计混合（量化可信，核心亮点）
  -> Phase 3-B1 诊断台 Agent 工具（引用已有分析，低成本高价值）
  -> Phase 3-B2 诊断台多模态输入（会话中直接传图，高成本）
```

建议顺序：先让"上传三视角→看到 AI 分析"跑通（P1），再把数值换成几何计算的可信量化（P2），再让问诊 Agent 能引用体态档案（P3-B1），最后才做会话内直接传图（P3-B2）。

## 当前状态

| Phase | 状态 | 说明 |
|-------|------|------|
| Phase 1 VLM MVP | ✅ 已实施 | PR #39：`posture.py` 路由 + `posture_analyzer.py` + migration 000030 + 前端结果展示 |
| Phase 2 姿态估计混合 | ⏳ 待实施 | 需新增 mediapipe/opencv 可选依赖 |
| Phase 3-B1 Agent 工具 | ⏳ 待实施 | 复用 tool-calling 架构，无新范式 |
| Phase 3-B2 多模态输入 | ⏳ 待实施 | 需改动契约层，同步三方契约测试 |

## 前置修复（强烈建议先做）

- **AI 服务连接池**：`apps/ai-service/src/rag/knowledge_library.py` 当前在 `async` 方法里用同步单连接 psycopg，多模态分析负载更重会放大问题。`psycopg[binary,pool]` 依赖已装，改用 `AsyncConnectionPool` 即可。详见审查报告 A-1。此项不阻塞本功能，但建议同期处理。

---

## 数据契约（前后端 + 评估共用，Phase 1 定稿）

### 分析结果 JSON（存 `user_uploads.analysis_result`）

```jsonc
{
  "schema_version": 1,
  "view": "front | side | back",
  "overall_confidence": "high | medium | low",
  "findings": [
    {
      "key": "forward_head",          // 与知识库 problem_slug 对齐
      "label": "头前移倾向",
      "severity": "mild | moderate | marked",
      "confidence": "high | medium | low",
      "evidence": "侧面观耳垂明显位于肩峰前方",
      "metric": {                     // Phase 1 恒为 null；Phase 2 才由几何计算填充
        "name": "craniovertebral_angle",
        "value": 48.5,
        "unit": "deg"
      }
    }
  ],
  "red_flags": [
    { "category": "severe_asymmetry", "message": "双肩高度差异明显，建议就医评估" }
  ],
  "summary_markdown": "……人话总结……",
  "disclaimer": "本分析基于照片视觉判断，仅供参考，不构成医疗诊断……"
}
```

### 各视角可判断项（防止跨视角瞎猜）

| view | 允许的 finding.key |
|---|---|
| `side`（侧面） | `forward_head` 头前移、`rounded_shoulders` 圆肩、`kyphosis` 驼背、`pelvic_anterior_tilt` 骨盆前倾、`knee_hyperextension` 膝超伸 |
| `front`（正面） | `shoulder_tilt` 高低肩、`pelvic_lateral_tilt` 骨盆侧倾、`head_tilt` 头侧倾、`knee_valgus_varus` 膝内外翻 |
| `back`（背面） | `shoulder_tilt` 高低肩、`scapular_asymmetry` 肩胛不对称、`spinal_lateral_deviation` 脊柱侧弯倾向、`pelvic_lateral_tilt` 骨盆侧倾 |

> Prompt 与几何计算都必须按 `view` 限定可输出的 key，跨视角的项一律不得输出。

---

## Phase 1：VLM MVP（纯多模态大模型）

**目标**：用户上传任一视角照片后，异步产出结构化体态分析并在前端可见，全程带免责声明与红旗，兑现 `UploadStep.tsx:137` 的文案承诺。

### Task 1.1：数据库迁移 —— 新增分析字段

**涉及文件**
- 新增 `apps/api/migrations/00XX_add_upload_analysis.up.sql` / `.down.sql`（编号接现有最大迁移号）
- `apps/api/internal/model/user_upload.go`

**实施内容**

`up.sql`：
```sql
ALTER TABLE user_uploads
  ADD COLUMN analysis_status varchar(20) NOT NULL DEFAULT 'none',
  ADD COLUMN analysis_result jsonb;

CREATE INDEX IF NOT EXISTS idx_user_uploads_user_type_created
  ON user_uploads (user_id, file_type, created_at DESC);
```
`down.sql`：
```sql
DROP INDEX IF EXISTS idx_user_uploads_user_type_created;
ALTER TABLE user_uploads
  DROP COLUMN IF EXISTS analysis_result,
  DROP COLUMN IF EXISTS analysis_status;
```
`model/user_upload.go`（在 `OCRStatus` 后，`:20` 附近增加）：
```go
AnalysisResult json.RawMessage `gorm:"type:jsonb" json:"analysis_result,omitempty"`
AnalysisStatus string          `gorm:"type:varchar(20);not null;default:'none'" json:"analysis_status"`
```

**注意点**
- 迁移用 golang-migrate（`internal/database/migrate.go` 已是版本化迁移），启动时自动 `Up`。
- `analysis_status` 默认 `none`，区别于 report 的 OCR 语义。

**验收标准**
- `docker compose --profile dev up` 启动后迁移成功，`user_uploads` 有两新列。
- `go build ./...` 通过。

### Task 1.2：Repository —— 分析结果读写

**涉及文件**
- `apps/api/internal/repository/upload_repository.go`

**实施内容**（与现有 `UpdateOCRStatus`/`UpdateOCRResult` 同构）：
```go
func (r *UploadRepository) UpdateAnalysisStatus(ctx context.Context, id, userID uuid.UUID, status string) error {
    return r.db.WithContext(ctx).Model(&model.UserUpload{}).
        Where("id = ? AND user_id = ?", id, userID).
        Update("analysis_status", status).Error
}

func (r *UploadRepository) UpdateAnalysisResult(ctx context.Context, id, userID uuid.UUID, status string, result json.RawMessage) error {
    return r.db.WithContext(ctx).Model(&model.UserUpload{}).
        Where("id = ? AND user_id = ?", id, userID).
        Updates(map[string]any{"analysis_status": status, "analysis_result": result}).Error
}

// 供 Phase 3-B1 使用：取用户三视角最新分析
func (r *UploadRepository) GetLatestPostureAnalyses(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error) {
    var uploads []model.UserUpload
    err := r.db.WithContext(ctx).
        Where("user_id = ? AND file_type IN ? AND analysis_status = ?",
            userID, []string{"photo_front", "photo_side", "photo_back"}, "completed").
        Order("created_at DESC").Find(&uploads).Error
    return uploads, err
}
```

**验收标准**：`go build ./...` 通过；`GetLatestPostureAnalyses` 只返回本人、已完成的三视角。

### Task 1.3：Go 上传服务 —— 照片分流入队 posture job

**涉及文件**
- `apps/api/internal/service/upload_service.go`

**实施内容**
1. 新增 job 类型常量（`:25` 附近）：
   ```go
   postureJobType = "upload.posture_analyze"
   ```
2. 上传后按类型分流（改 `UploadFile` 的 `:157-162`）：
   ```go
   switch fileType {
   case "report":
       if _, _, err := s.enqueueOCRJob(ctx, upload.ID, userID, filePath, mimeType); err != nil {
           log.Printf("failed to enqueue OCR job for upload %s: %v", upload.ID, err)
       }
   case "photo_front", "photo_side", "photo_back":
       if _, _, err := s.enqueuePostureJob(ctx, upload.ID, userID, filePath, mimeType, fileType); err != nil {
           log.Printf("failed to enqueue posture job for upload %s: %v", upload.ID, err)
       }
   }
   ```
3. 新增 `postureJobInput`（含 `view`）、`enqueuePostureJob`（幂等键 `posture_analyze:{uploadID}`）、`processPostureJob`、`executePostureCall`（与 `executeOCRCall:211` 同构，多带一个 `view` 表单字段，打到 `POST {aiServiceURL}/api/posture/analyze`）。`processPostureJob` 骨架：
   ```go
   func (s *UploadService) processPostureJob(ctx context.Context, job model.Job) error {
       input, err := parsePostureJobInput(job)
       if err != nil { /* TransitionTo failed */ }
       uploadID, _ := uuid.Parse(input.UploadID)

       s.jobRuntime.TransitionTo(ctx, job.ID, "running", nil, nil)
       s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "posture_analyzing", "percent": 10})
       s.uploadRepo.UpdateAnalysisStatus(ctx, uploadID, job.UserID, "processing")

       respBody, err := s.executePostureCall(input.FilePath, input.MimeType, input.View)
       if err != nil {
           s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
           s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "failed",
               json.RawMessage(fmt.Sprintf(`{"error":%q}`, err.Error())))
           return err
       }
       s.jobRuntime.TransitionTo(ctx, job.ID, "completed", json.RawMessage(respBody), nil)
       s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "completed", respBody)
       return nil
   }
   ```
4. Worker 泛化：把 `StartOCRWorker`（`:262`）/`RecoverOCRJobs`（`:283`）扩展为同时 recover 两种 job 类型，或另起 `RecoverPostureJobs` 并在 `main.go` 一并启动。**推荐泛化**：`RecoverUploadJobs` 遍历 `[]string{ocrJobType, postureJobType}`，按 `job.JobType` 分派到 `processOCRJob` / `processPostureJob`。

**注意点**
- 沿用"Go 读本地文件→HTTP multipart 发 AI 服务"，无需共享卷/对象存储。
- 幂等键保证重复上传/重启不重复分析（复用 `CreateJobWithIdempotency`）。
- worker 用 `main.go:72` 已在跑的实例；若泛化，注意 `staleRunningAfter` 超时对多模态较慢调用要放宽（如 10min→ 保持或调大）。

**验收标准**
- 上传 `photo_side` 后，`jobs` 表出现 `upload.posture_analyze` 记录并流转 pending→running→completed。
- `user_uploads.analysis_status` 最终为 `completed`，`analysis_result` 为合法 JSON。

### Task 1.4：AI 服务 —— posture 分析服务与路由

**涉及文件**
- 新增 `apps/ai-service/src/services/posture_analyzer.py`
- 新增 `apps/ai-service/src/api/routes/posture.py`
- 新增 `apps/ai-service/src/prompts/posture.py`
- 新增 `apps/ai-service/src/models/posture.py`（Pydantic 响应模型）
- `apps/ai-service/src/main.py`（注册路由）
- `apps/ai-service/src/config/models.yaml`（新增 `posture.analyze` 路由）

**实施内容**

`models/posture.py`：定义 `PostureFinding` / `PostureAnalysis` / `PostureAnalysisResponse`（对齐上文 JSON 契约）。

`prompts/posture.py`：`POSTURE_SYSTEM_PROMPT`，要点：
- 只描述**照片可观察**的现象，用"倾向/疑似"而非确诊。
- 按 `view` 限定可输出的 `finding.key`（见上表）。
- **Phase 1 严禁输出任何角度数值**（`metric` 恒为 null）——防幻觉红线。
- 严格输出契约 JSON（配合 `response_format=json_object`）。
- `summary_markdown` 末尾必须包含免责声明。

`services/posture_analyzer.py`：
```python
import base64, json
from ..ai import AIService, AiRequest
from ..ai.types import ChatMessage
from ..prompts.posture import POSTURE_SYSTEM_PROMPT, VIEW_LABEL
from ..services.red_flag_detector import get_red_flag_detector
from ..services.governance.output_guard import AIOutputGuard

async def analyze_posture(ai: AIService, image_bytes: bytes, mime_type: str, view: str) -> dict:
    b64 = base64.b64encode(image_bytes).decode()
    messages = [
        ChatMessage(role="system", content=POSTURE_SYSTEM_PROMPT),
        ChatMessage(role="user", content=[
            {"type": "text", "text": f"这是用户的{VIEW_LABEL[view]}站姿照片，请按 {view} 视角分析体态。"},
            {"type": "image_url", "image_url": {"url": f"data:{mime_type};base64,{b64}"}},
        ]),
    ]
    resp = await ai.generate(AiRequest(
        use_case="posture.analyze",
        messages=messages,
        response_format="json_object",
    ))
    data = json.loads(resp.text)
    return _govern(data, view)   # 见治理

def _govern(data: dict, view: str) -> dict:
    data["view"] = view
    # 1) Phase 1 强制清空所有 metric，杜绝数值幻觉
    for f in data.get("findings", []):
        f["metric"] = None
    # 2) 强制免责声明
    data.setdefault("disclaimer", DEFAULT_DISCLAIMER)
    # 3) 红旗扫描（复用现有 detector 的文本扫描能力）
    text = data.get("summary_markdown", "") + " ".join(f.get("evidence","") for f in data.get("findings", []))
    rf = get_red_flag_detector().detect([], text)
    if rf.has_red_flags:
        data["red_flags"] = rf.to_dict()["flags"]
    # 4) AIOutputGuard 结构校验（缺字段→降级标注）
    result = AIOutputGuard().validate_structured_output(
        data, required_fields=["view", "findings", "summary_markdown", "disclaimer"])
    if result.status.value != "accepted":
        data["overall_confidence"] = "low"
    return data
```

`api/routes/posture.py`（镜像 `ocr.py:18`）：
```python
router = APIRouter(prefix="/api/posture", tags=["posture"])

@router.post("/analyze", response_model=PostureAnalysisResponse)
async def analyze(view: str = Form(...), file: UploadFile = File(...)):
    if view not in {"front", "side", "back"}:
        raise HTTPException(400, "invalid view")
    allowed = {"image/jpeg", "image/png", "image/webp"}
    if file.content_type not in allowed:
        raise HTTPException(400, f"unsupported type: {file.content_type}")
    data = await file.read()
    if not data:
        raise HTTPException(400, "empty file")
    if len(data) > 10 * 1024 * 1024:
        raise HTTPException(413, "file too large")
    result = await analyze_posture(get_ai_service(), data, file.content_type, view)
    return PostureAnalysisResponse(status="completed", result=result)
```

`main.py`：在 `:60-66` 的 `include_router` 区块加 `app.include_router(posture.router)` 并加 import。

`config/models.yaml`（新增路由；模型按可用 vision 模型填）：
```yaml
routes:
  posture.analyze:
    defaults:
      temperature: 0.2
      max_tokens: 1500
      response_format: json_object
    candidates:
      - { provider: openrouter, model: "qwen/qwen2.5-vl-72b-instruct" }
      - { provider: openrouter, model: "google/gemini-2.0-flash-001" }
```
并确保 `providers.openrouter.models` 里登记这两个模型 id（带 `vision` capability 便于 `required_capabilities` 过滤）。

**注意点**
- **不改 provider 代码**：`openai_compatible.py:200-203` 的 `_convert_messages` 已直接透传 `content`（含内容块），多模态自动可用；熔断/fallback 自动生效。
- 视觉模型比文本贵且慢，`max_tokens` 控制在 1500 左右；异步 job 已避免阻塞用户。
- `get_ai_service()` 复用 `consultation_graph.py:41` 的单例思路，或在 posture 模块内建单例，避免每请求重解析配置。

**验收标准**
- `curl -F view=side -F file=@side.jpg localhost:8100/api/posture/analyze` 返回契约 JSON，`metric` 全为 null，含 `disclaimer`。
- 无 vision key 时熔断/fallback 生效或返回明确错误，不 500 崩溃。
- `uv run ruff check .` 与新增单测通过。

### Task 1.5：AI 服务单测

**涉及文件**
- 新增 `apps/ai-service/tests/unit/test_posture_analyzer.py`

**实施内容**
- Mock `AIService.generate` 返回带 `metric` 的 JSON → 断言 `_govern` 后 `metric` 全为 null、含 `disclaimer`。
- 造含"双肩明显不对称"的 summary → 断言 `red_flags` 非空。
- 缺字段的返回 → 断言 `overall_confidence` 被降级为 low。

**验收标准**：`uv run pytest tests/unit/test_posture_analyzer.py` 通过。

### Task 1.6：前端 —— 展示分析状态与结果

**涉及文件**
- `apps/web/src/features/profile/types/upload.types.ts`
- `apps/web/src/features/profile/components/onboarding/steps/UploadStep.tsx`
- 新增 `apps/web/src/features/profile/components/uploads/PostureAnalysisView.tsx`
- （可选）`apps/web/src/features/profile/pages/ProfilePage.tsx` 展示汇总

**实施内容**
1. `upload.types.ts`：`UserUpload` 增加 `analysis_status?: 'none'|'pending'|'processing'|'completed'|'failed'` 与 `analysis_result?: PostureAnalysis`；补 `PostureAnalysis`/`PostureFinding` 类型。
2. `UploadStep.tsx`：三视角照片槽从"只显示文件名 ✓"（`:81-100`）升级为显示分析态，直接抄报告区已有的状态样式（`:189-221` 的 completed/processing/failed 徽章）。完成后展示 top 2 findings 徽章（label + severity）。
3. `PostureAnalysisView.tsx`：三视角汇总卡片 —— findings 列表（label/severity/confidence/evidence）、红旗提示（复用 `RedFlagBanner` 视觉）、`summary_markdown`、免责声明。Phase 2 追加标注图。
4. **异步轮询**：分析是 job 异步产出，用 TanStack Query 轮询 `GET /uploads`，`refetchInterval` 在存在 `analysis_status ∈ {pending,processing}` 时启用，全部 completed/failed 后停。（当前 `uploadStore` 是 Zustand 手写 fetch，可保留，加一个轮询 effect；或顺势迁到 React Query。）

**注意点**
- 保持与报告 OCR 态一致的交互语言，用户心智统一。
- 免责声明必须显著展示（PRD §7）。

**验收标准**
- 上传侧面照 → 槽内显示"分析中"→ 数秒后变"完成"并展示发现与免责声明。
- 失败态有明确提示与重试入口（重新上传即重新入队）。

### Task 1.7：文案与承诺对齐

**涉及文件**
- `apps/web/src/features/profile/components/onboarding/steps/UploadStep.tsx:137`
- `README.md`（首句"AI 视觉分析"）

**实施内容**：P1 上线后，文案与实际能力一致即可保留；上线前若尚未完成，先把 `:137` 措辞降级为"照片将用于后续 AI 体态分析（开发中）"，避免空头承诺。README 同步。

**验收标准**：UI 文案与实际功能不再存在"过度承诺"。

---

## Phase 2：姿态估计混合（量化可信 —— 核心亮点）

**目标**：把"头前移中度"这类定性判断升级为"颅椎角 48°→头前移中度"的量化结论；数值只来自几何计算，VLM 只做定性描述，杜绝数值幻觉。

### Task 2.1：引入姿态估计依赖

**涉及文件**：`apps/ai-service/pyproject.toml`

**实施内容**：新增可选依赖组：
```toml
pose = [
    "mediapipe>=0.10.14",
    "opencv-python-headless>=4.10.0",
    "numpy>=1.26.0",
]
```
Docker（`apps/ai-service/Dockerfile`）的 `uv sync` 增加 `--extra pose`；CI（`.github/workflows/ci.yml:55`）同步。

**注意点**：mediapipe 体积较大、对 Python 版本敏感；确认 3.13 兼容性，不兼容则退到 `ultralytics` YOLO-Pose 或独立姿态微服务。

### Task 2.2：关键点提取与几何指标

**涉及文件**：新增 `apps/ai-service/src/rag/pose_estimator.py`（或 `services/pose_estimator.py`）

**实施内容**
- `extract_landmarks(image_bytes) -> Landmarks | None`：MediaPipe Pose 提 33 点；置信度不足或未检出人体→返回 None（分析降级为纯 VLM 定性）。
- 几何函数（按 view）：
  - `side`：颅椎角 CVA（耳屏-C7 连线与水平夹角）、耳垂-肩峰水平偏移、骨盆前倾近似。
  - `front`/`back`：肩峰高度差、髂嵴高度差、脊柱中线偏移。
- 输出 `metric{name,value,unit}` 列表 + 依阈值映射的 `severity`（阈值表集中定义，便于调参与写测试）。

**验收标准**：对已知样例照片，几何函数输出稳定、可复现；无人体时优雅降级。

### Task 2.3：融合与数值防幻觉校验

**涉及文件**：`apps/ai-service/src/services/posture_analyzer.py`、`prompts/posture.py`

**实施内容**
1. 流程改为：几何算 `metric` → 把 metric 作为 system/user 提示的一部分喂给 VLM，让其做定性描述与解释（**不让 VLM 自己造数值**）。
2. 后置校验（借鉴 `faithfulness_checker` 思路）：报告里出现的每个 `metric.value` 必须能在几何结果集里找到匹配（同名 + 数值一致）；对不上则剔除该数值或标注 low confidence。
3. 放开 Phase 1 的"禁止数值"限制，但数值只允许来自几何结果集。

**验收标准**：`analysis_result.findings[].metric` 的数值与几何计算完全一致，无 VLM 编造。

### Task 2.4：标注图（可选增强）

**实施内容**：在原图叠加关键点/铅垂线/角度标注，编码存储（复用 `knowledge_clips` 或新 `analysis_annotations`），前端展示。极大提升可信观感。

---

## Phase 3-B1：诊断台 Agent 工具（引用已有分析 —— 推荐先做）

**目标**：不改输入框，让问诊 Agent 能主动读取用户三视角体态分析并结合症状给方案。完美复用现有 tool-calling + HITL + 引用可视化。

### Task 3B1.1：Go 只读端点

**涉及文件**：`apps/api/internal/handler/upload_handler.go`、`cmd/server/main.go`

**实施内容**
- 新增 `GET /api/v1/uploads/posture-analysis` → `uploadService.GetPostureAnalyses(ctx, uid)` → 调 `repo.GetLatestPostureAnalyses`（Task 1.2 已加），返回三视角最新分析汇总（去掉大字段，保留 findings/summary/red_flags）。
- 在 `main.go` protected 组注册（`:194-198` upload 路由附近）。

**验收标准**：登录用户 GET 返回本人三视角分析；无分析时返回空结构而非报错。

### Task 3B1.2：AI 服务 Agent 工具

**涉及文件**
- 新增 `apps/ai-service/src/services/agent/tools/get_posture_analysis.py`
- `apps/ai-service/src/services/agent/consultation_tools.py`（注册工具）
- `apps/ai-service/src/services/agent/orchestrator.py`（`_handle_tool_calls` 分派）

**实施内容**
1. 工具定义 `get_posture_analysis`（无参或按 view 过滤），handler 调 Go 端点（需把用户 token/内部鉴权透传；沿用现有 Go→Python 上下文传参方式，把 posture 分析随 `BusinessContext` 预取传入，或让工具回调 Go 内部 API）。
   - **更省事的做法**：Go 在 `StartConsultationTurn` 的 `BusinessContext`（`runtime.go:254`）里预取并带上 `posture_analysis`，工具直接从上下文读，无需 Python 反向调 Go。推荐此法。
2. 与 `search_knowledge`/`extract_symptom_info` 同构：返回结构化结果 + 触发引用/信息卡片事件。
3. `orchestrator.py:213` 的 if/elif 工具分派增加 `get_posture_analysis` 分支，emit 对应 `tool_result`。
4. `prompts/consultation.py` 系统提示补充：当用户询问体态/有可用照片分析时，应调用该工具。

**验收标准**：用户说"帮我看看我的体态问题"，Agent 调 `get_posture_analysis` → 把量化发现拉进上下文 → 结合症状与 RAG 给方案；前端 `StreamingTurnToolCalls` 正常渲染该工具调用。

### Task 3B1.3：前端工具卡片

**涉及文件**：`apps/web/src/features/consultation/components/StreamingTurnToolCalls.tsx`（或 `ToolCallCard.tsx`）

**实施内容**：为 `get_posture_analysis` 加专属卡片样式（展示引用的体态发现）。其余渲染管线已就绪。

**验收标准**：工具调用在对话中以可读卡片呈现，非裸 JSON。

---

## Phase 3-B2：诊断台多模态输入（会话内直接传图 —— 高成本增强）

**目标**：用户在问诊中直接附带新照片，走视觉模型即时分析。需打通每一层并同步契约。

### Task 3B2.1：前端 composer 附件

**涉及文件**：`AssistantChatPanel.tsx`（`ChatInputArea:152`、`handleSend:250`、`Image: () => null:458`）、`useAssistantChatRuntime.ts`（parts 组装 `:281-290`）

**实施内容**：加附件按钮→选图→走现有 `/api/v1/uploads`（`file_type=photo_*`）拿 `upload_id`→消息 parts 增加 `{type:'image', image_url|upload_id}`；把 `Image: () => null` 改成真正渲染缩略图。

### Task 3B2.2：契约与 Go 输入扩展

**涉及文件**：`packages/contracts`（schema + fixtures）、`apps/api/internal/dto/*`、`internal/consultation/runtime.go`（`messagePartsToText:1289`、`ConsultationUserInput:250`）、`apps/api/internal/dto/stream_event*`

**实施内容**
- `PartDTO` 增加 image 类型；`messagePartsToText`→`messagePartsToInput` 保留 image 引用。
- `ConsultationUserInput` 增加 `images []ImageRef`。
- **同步更新 `packages/contracts` 的 schema 与 fixtures**，保证 Go/Python/TS 三方契约测试通过（`test_stream_event.py`、`stream_event_test.go`）。

### Task 3B2.3：AI 服务多模态消息构建与路由

**涉及文件**：`apps/ai-service/src/services/consultation_graph.py`（`build_messages:112`）、`config/models.yaml`

**实施内容**：`build_messages` 把 image 拼进 user content 块；本轮含图时用视觉模型路由（新增 `consultation.reply.vision` use_case，或按 metadata 动态选）。

### Task 3B2.4：多模态治理

**实施内容**：图中文字 prompt-injection 防护、尺寸/格式限制、EXIF 去隐私、base64 不落日志。

---

## 建议任务切片（便于提交与回滚）

```
1. feat(api): add analysis columns to user_uploads + repository methods
2. feat(api): enqueue posture-analyze job for photo uploads
3. feat(ai): add posture analyze route, analyzer, prompt, governance
4. feat(ai): register posture.analyze vision route in models.yaml
5. test(ai): posture analyzer unit tests
6. feat(web): show posture analysis status and result in onboarding/profile
7. docs(web): align photo AI analysis copy with actual capability
--- Phase 2 ---
8. build(ai): add pose estimation optional deps
9. feat(ai): pose landmark extraction and geometric metrics
10. feat(ai): fuse metrics with VLM + numeric anti-hallucination guard
--- Phase 3-B1 ---
11. feat(api): read-only posture-analysis endpoint + business context prefetch
12. feat(ai): get_posture_analysis agent tool
13. feat(web): posture analysis tool call card
--- Phase 3-B2 ---
14. feat(web): image attachment in consultation composer
15. feat(contracts): image message part + fixtures
16. feat(api): carry image refs through consultation input
17. feat(ai): multimodal consultation messages + vision route
18. feat(ai): multimodal input governance
```

## 依赖关系

```text
Task 1.1 -> 1.2 -> 1.3 -> 1.4 -> 1.5 -> 1.6 -> 1.7
Phase 1 complete -> Phase 2 (2.1 -> 2.2 -> 2.3 -> 2.4)
Task 1.2 -> 3B1.1 -> 3B1.2 -> 3B1.3           (B1 只需 P1 的数据层，不必等 P2)
Phase 3-B1 complete -> Phase 3-B2 (3B2.1 -> 3B2.2 -> 3B2.3 -> 3B2.4)
```

说明：
1. B1（Agent 工具）只依赖 P1 的分析数据，**可在 P2 之前先交付**，性价比最高。
2. B2（多模态输入）改动契约层，风险最高，放最后。
3. 姿态估计（P2）是可信度与亮点的关键，但不阻塞 B1。

## 最小可交付版本（MVP）

只做以下 3 步即可让功能"看得见、可讲"：
1. Task 1.1–1.3：DB + Go 照片分流入队
2. Task 1.4–1.5：AI VLM 分析端点 + 治理 + 单测
3. Task 1.6：前端展示分析结果与免责声明

完成后：**上传三视角照片 → 数秒后看到 AI 体态发现 + 免责声明**，README/文案承诺兑现。

## 验证清单

- [ ] 迁移在 dev compose 启动时自动应用，字段/索引存在
- [ ] 上传三视角任一照片 → `jobs` 出现 `upload.posture_analyze` 并流转到 completed
- [ ] `analysis_result` 为合法契约 JSON，Phase 1 下 `metric` 全为 null，含 `disclaimer`
- [ ] 含明显不对称的照片 → `red_flags` 非空，前端红旗提示可见
- [ ] 无 vision key/模型不可用 → 熔断/fallback 生效，不 500 崩溃
- [ ] 前端三视角槽显示 待/中/完成/失败 四态，完成后展示发现与免责声明
- [ ] UI/README 文案与实际能力一致（无过度承诺）
- [ ] `go test ./...`、`uv run pytest`、`pnpm nx run web:typecheck` 通过
- [ ] （B2）`packages/contracts` 三方契约测试通过
- [ ] 隐私：分析用 base64 不落日志；上传可删除（`DeleteUpload` 已支持）

## 风险与回滚

- **VLM 数值幻觉**：P1 硬性清空 metric；P2 数值仅来自几何 + 后置校验。
- **单图深度缺失**：按 view 限定可判断项，禁止跨视角推断。
- **医疗合规**：全程免责 + 红旗就医；措辞"倾向/疑似"，不确诊。
- **成本/时延**：异步 job 化（已具备）；三视角可并发。
- **回滚**：功能以新增列 + 新 job 类型 + 新路由实现，对既有链路零侵入；关闭办法 = 上传时不再入队 posture job（改回只 `report` 分流），前端隐藏分析展示。
