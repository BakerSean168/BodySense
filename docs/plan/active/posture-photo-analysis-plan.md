# 体态照片 AI 分析 —— 详细实施方案

> 文档状态：Phase 1 已实施（PR #39），Phase 2 / 3-B1 / 3-B2 待实施
> 创建日期：2026-07-10
> 关联：`docs/project-review-2026-07-10.md` §4-7、PRD 3.1.2 / 3.3.3
> 范围：①引导页体态照片的 AI 分析　②诊断台的图片输入能力

---

## 1. 现状盘点（先看清楚已有什么）

### 1.1 已具备（好消息，地基已打好）

| 能力 | 位置 | 说明 |
|---|---|---|
| 三视角照片上传入口 | `apps/web/.../onboarding/steps/UploadStep.tsx:57` | `photo_front/side/back` 三个槽，`accept="image/*"`，已能上传 |
| 上传存储与去重 | `apps/api/internal/service/upload_service.go:65` | 存本地磁盘 `uploads/{userID}/{uuid}`，含 MIME 校验、内容嗅探、10MB 限制 |
| **异步 AI 处理管道（模板）** | `upload_service.go:262` `StartOCRWorker` + `JobRuntime` | 上传→建记录→入队 job→worker 轮询→调 AI→回写结果，**可恢复、幂等**。这是照片分析要复用的骨架 |
| AI 服务处理端点（模板） | `apps/ai-service/src/api/routes/ocr.py:18` | `POST /api/ocr/extract` 接收文件→结构化返回 |
| **多模态底层已就绪** | `ai/types.py:32` + `providers/openai_compatible.py:203` | `ChatMessage.content` 已支持 `list[dict]`（OpenAI vision 内容块格式），provider 直接透传 → **传图给 VLM 的管道已通** |
| 结果存储字段 | `model/user_upload.go:19` | `OCRResult jsonb` + `OCRStatus`（可复用/或新增专用字段） |
| 连接池依赖已装 | `pyproject.toml:10` `psycopg[binary,pool]` | 无需新增依赖 |

### 1.2 尚缺（要补的东西）

1. **照片上传后无任何分析** —— `upload_service.go:158` 只对 `fileType == "report"` 入队 OCR job；三视角照片只存不分析。而 `UploadStep.tsx:137` 的文案却已写着"AI 会通过多模态视觉进行精准骨骼重心分析"（**当前是空头承诺**）。
2. **无视觉模型路由** —— 生产模型 `openai/gpt-oss-120b`（`.env.production:55`）是纯文本模型，不能看图。需在 `models.yaml` 增加一个 vision 路由。
3. **诊断台端到端纯文本** —— 输入框只有 `<textarea>`（`AssistantChatPanel.tsx:163`），消息 parts 只含 `text`，Go `messagePartsToText`（`runtime.go:1289`）只取 text，AI 输入 `ConsultationUserInput{Text}` 纯文本，助手渲染还显式 `Image: () => null`。**诊断台目前完全不支持传图。**
4. **无姿态估计库** —— `pyproject.toml` 无 mediapipe/opencv/YOLO；若走关键点方案需新增依赖。

---

## 2. 技术路线选型（核心决策）

体态分析的本质是"从站姿照片判断头前移、圆肩、骨盆前倾、高低肩、脊柱侧弯倾向等"。有三条路：

| 方案 | 做法 | 优点 | 缺点 | 依赖 |
|---|---|---|---|---|
| **A. 纯 VLM（多模态大模型）** | 三视角图直接喂给视觉模型，让它输出结构化体态评估 | 实现最快，复用现有多模态管道，零新依赖，能给"人话"解释 | VLM 对**精确几何角度**不可靠、可能幻觉出"具体度数"；不可复现 | 无（配一个 vision 模型即可） |
| **B. 姿态估计 + 几何计算** | MediaPipe/YOLO-Pose 提关键点→算颅椎角、肩倾角、骨盆倾角等 | **量化、可复现、可解释、可信**，是真正的"骨骼重心分析" | 需重依赖；单张照片深度缺失，侧面/正面各测各的；无"人话" | 新增 mediapipe 或 ultralytics + opencv |
| **C. 混合（推荐）** | B 出量化指标 + A 出定性视觉发现 → LLM 融合成结构化报告，并用 RAG 知识库佐证 | 兼顾**量化可信**与**可读解释**，作品集叙事最强 | 工作量最大 | B 的依赖 |

### 决策：分阶段落地，先 A 后 C

- **Phase 1（MVP）走 A（纯 VLM）**：最快兑现文案承诺、打通端到端、复用现有一切。
- **Phase 2 升级为 C（加姿态估计）**：把"精确度数"交给关键点几何，VLM 只做定性描述与解释，用 `faithfulness` 思路约束 VLM 不得编造未测量的数值。**这一步才是真正的技术亮点。**
- **Phase 3 接入诊断台**：先做"引用已有分析"的 Agent 工具（低成本），再做真正的多模态输入（高成本）。

> ⚠️ 安全底线（贯穿所有阶段）：体态照片分析**不是医疗诊断**。所有输出必须带免责声明、置信度标注，检出严重不对称/疑似脊柱侧弯等要走**红旗提示**建议就医（复用现有 `red_flag_detector` 与 `RedFlagBanner`）。

---

## 3. 数据模型变更

复用现有 `user_uploads` 表，**新增一个通用分析结果字段**（与 OCR 语义解耦，避免把体态分析塞进 `ocr_result`）：

```sql
-- migrations/00XX_add_upload_analysis.up.sql
ALTER TABLE user_uploads
  ADD COLUMN analysis_status  varchar(20) NOT NULL DEFAULT 'none',  -- none|pending|processing|completed|failed
  ADD COLUMN analysis_result  jsonb;

-- 供"按用户取最新一套三视角分析"用
CREATE INDEX idx_user_uploads_user_type_created
  ON user_uploads (user_id, file_type, created_at DESC);
```

对应 `model/user_upload.go` 加两个字段。`analysis_result` 的 JSON 结构（Phase 1 即定稿，前后端与评估共用）：

```jsonc
{
  "schema_version": 1,
  "view": "front|side|back",          // 单图分析；三视角汇总另存或前端聚合
  "overall_confidence": "high|medium|low",
  "findings": [
    {
      "key": "forward_head",           // 与知识库 problem_slug 对齐
      "label": "头前移倾向",
      "severity": "mild|moderate|marked",
      "confidence": "high|medium|low",
      "evidence": "侧面观耳垂明显位于肩峰前方",
      "metric": { "name": "craniovertebral_angle", "value": 48.5, "unit": "deg" } // Phase 2 才有
    }
  ],
  "red_flags": [ { "category": "severe_asymmetry", "message": "..." } ],
  "summary_markdown": "……（人话总结，末尾必带免责声明）",
  "disclaimer": "本分析基于照片视觉判断，仅供参考，不构成医疗诊断……"
}
```

---

## 4. 后端（Go）改动 —— 复用 OCR 管道

几乎是把 OCR 那套"复制一份 for 照片"，改动集中在 `upload_service.go`：

1. **新增 job 类型**：`const postureJobType = "upload.posture_analyze"`。
2. **上传时按类型分流入队**（`upload_service.go:157` 附近）：
   ```go
   switch fileType {
   case "report":
       s.enqueueOCRJob(ctx, upload.ID, userID, filePath, mimeType)
   case "photo_front", "photo_side", "photo_back":
       s.enqueuePostureJob(ctx, upload.ID, userID, filePath, mimeType, fileType) // fileType→view
   }
   ```
3. **新增 `processPostureJob`**：与 `processOCRJob`（`:325`）同构 —— 置 running → 更新 `analysis_status=processing` → `executePostureCall`（HTTP multipart 到 AI 服务新端点 `POST /api/posture/analyze`，把 `view` 作为表单字段一起发）→ 写回 `UpdateAnalysisResult`。
4. **worker 复用**：`StartOCRWorker` 改名/泛化为一个能同时 recover 两种 job 类型的 worker，或再起一个 `StartPostureWorker`（更省事，但建议泛化成 `StartUploadWorkers` 统一管理）。
5. **repository**：加 `UpdateAnalysisResult(ctx, uploadID, userID, status, result)`，与 `UpdateOCRResult` 同构。
6. **（Phase 3-B1）新增只读端点**：`GET /api/v1/uploads/posture-analysis` —— 返回该用户三视角的最新分析汇总，供诊断台 Agent 工具与档案页展示。

> 说明：不需要对象存储改造。沿用 OCR 的"Go 读本地文件→HTTP multipart 发给 AI 服务"即可，AI 服务无需直接访问磁盘。

---

## 5. AI 服务（Python）改动 —— 核心分析逻辑

### 5.1 Phase 1（VLM）：新增 `posture` 路由与服务

**新增 `src/services/posture_analyzer.py`**：
```python
async def analyze_posture(image_bytes: bytes, view: str) -> PostureAnalysis:
    b64 = base64.b64encode(image_bytes).decode()
    messages = [
        ChatMessage(role="system", content=POSTURE_SYSTEM_PROMPT),  # 见 5.3
        ChatMessage(role="user", content=[
            {"type": "text", "text": f"这是用户的{VIEW_LABEL[view]}站姿照片，请分析体态。"},
            {"type": "image_url", "image_url": {"url": f"data:image/jpeg;base64,{b64}"}},
        ]),
    ]
    resp = await ai_service.generate(AiRequest(
        use_case="posture.analyze",           # 新 route（vision 模型 + json_object）
        messages=messages,
        response_format="json_object",
    ))
    data = json.loads(resp.text)
    # 治理：红旗扫描 + 强制免责声明 + 数值幻觉过滤（Phase 1 禁止输出 metric）
    return governed_posture_result(data, view)
```

**新增 `src/api/routes/posture.py`**：`POST /api/posture/analyze`，接收 `file: UploadFile` + `view: str`，镜像 `ocr.py` 的校验与错误处理，返回 `PostureAnalysisResponse`。在 `main.py` 注册路由。

**`models.yaml` 新增 vision 路由**（示例）：
```yaml
routes:
  posture.analyze:
    defaults: { temperature: 0.2, response_format: json_object, max_tokens: 1500 }
    candidates:
      - { provider: openrouter, model: "qwen/qwen2.5-vl-72b-instruct" }   # 或 gemini-flash / gpt-4o-mini
      - { provider: openrouter, model: "google/gemini-2.0-flash-001" }    # fallback
```
> 只加配置，不改 provider 代码 —— 多模态透传已在 `_convert_messages` 就绪。熔断/fallback 自动生效。

### 5.2 Phase 2（混合）：加姿态估计做量化底座

- 新增可选依赖组 `pose = ["mediapipe>=0.10", "opencv-python-headless>=4.10", "numpy"]`（或 `ultralytics` YOLO-Pose）。
- 新增 `src/rag/pose_estimator.py`：提 33 个关键点 → 几何函数算：
  - 侧面：**颅椎角 CVA**（耳屏-C7-水平）判头前移；耳垂-肩峰水平偏移；骨盆前倾（ASIS-PSIS 连线角度近似）。
  - 正面/背面：**肩峰高低差**（高低肩）、髂嵴高低差、脊柱中线偏移（侧弯倾向）。
- 分析流程改为：关键点几何算 `metric` → VLM 只做定性描述 → LLM 融合。**数值只能来自几何计算，VLM 不得编造**（用 `faithfulness_checker` 的思路做后置校验：报告里出现的每个 `metric.value` 必须能在几何结果里找到）。
- 输出可叠加"标注图"（在原图上画关键点/铅垂线）作为 clip 存储，前端展示更有说服力。

### 5.3 Prompt 与治理（安全是重点）

- 新增 `src/prompts/posture.py`：`POSTURE_SYSTEM_PROMPT` 明确要求——只描述可观察现象、用"倾向/疑似"而非确诊、按 `problem_slug` 归类（与知识库对齐便于后续 RAG 佐证）、严格输出上文 JSON schema、**Phase 1 禁止输出任何角度数值**（防幻觉）、末尾必须带免责声明。
- 复用 `red_flag_detector`：对分析文本/发现扫红旗（明显不对称、疑似结构性侧弯等）→ 建议就医。
- 复用 `AIOutputGuard`：`validate_structured_output` 校验必填字段 + 红旗 + 免责声明存在性；不合规则 `degraded`（前端提示"分析置信度低"）。

---

## 6. 前端（React）改动

### 6.1 引导页 / 档案页：展示分析结果（Phase 1 即可见）

- `upload.types.ts`：`UserUpload` 加 `analysis_status` / `analysis_result` 类型。
- `UploadStep.tsx`：照片槽从"只显示文件名 ✓"升级为显示分析态（等待/分析中/完成/失败），与现有报告 OCR 态一致（`:189-221` 已有现成的态样式可抄）。完成后展示 top findings 徽章。
- 新增 `PostureAnalysisView` 组件：三视角汇总卡片（发现列表 + 严重度 + 置信度 + 免责声明 + Phase 2 的标注图）。放进档案页与"健康评估报告"。
- 轮询/失效：分析是异步的，可用 TanStack Query 轮询 `uploads`（`analysis_status` 变 completed 即停），或复用 SSE/job 事件（若接入 JobRuntime 事件流）。

### 6.2 诊断台接入（回答 Q2：诊断台如何支持图片）

分两档，**强烈建议先做 B1**：

**B1 —— Agent 工具引用已有分析（低成本、高契合，推荐先做）**
不改输入框，而是新增一个 Agent 工具，让 AI 在问诊中主动调用：
- AI 服务侧：注册工具 `get_posture_analysis`（放进 `services/agent/tools/`，与 `search_knowledge`/`extract_symptom_info` 同构），调用 Go 的 `GET /uploads/posture-analysis` 拿用户已有三视角分析。
- 效果：用户说"帮我看看我的体态"，Agent 调工具 → 把量化发现拉进上下文 → 结合症状与 RAG 给方案。**完美复用你现有的 tool-calling + HITL + 引用可视化架构，几乎零新范式。**
- 前端：`StreamingTurnToolCalls` 已能渲染工具调用，只需给该工具加个卡片样式。

**B2 —— 真正的多模态输入（会话中直接传新图，成本高）**
需要打通每一层：
1. `ChatInputArea`（`AssistantChatPanel.tsx:152`）加附件按钮 → 选图 → 走现有 `/api/v1/uploads`（`file_type=photo_*`）拿到 `upload_id`，或走新的临时图床。
2. 消息 parts 增加 image 部分：`{ type: 'image', image_url | upload_id }`（assistant-ui 支持 image part；同时把 `Image: () => null` 改成真正渲染）。
3. `useAssistantChatRuntime.ts:287` 组装 parts 时带上 image。
4. Go：`messagePartsToText`（`runtime.go:1289`）→ 扩展为 `messagePartsToInput`，保留 image 引用；`ConsultationUserInput` 增加 `images []ImageRef` 字段；DTO/契约 `stream-event`、`PartDTO` 同步（记得更新 `packages/contracts` fixtures，保持三方契约测试通过）。
5. AI 服务：`consultation.reply` 路由需切到 vision 模型（或按"本轮是否含图"动态选 use_case）；`build_messages`（`consultation_graph.py:112`）把 image 拼进 user content 块。
6. 治理：图片输入要防 prompt-injection（图中文字）、限制尺寸、EXIF 去隐私。

> 建议：**Phase 3 先交付 B1**（一两天，且叙事上"Agent 会读你的体态档案"很惊艳），B2 作为后续增强。

---

## 7. 分阶段计划与工作量

| 阶段 | 内容 | 交付物 | 估时 |
|---|---|---|---|
| **P1** VLM MVP | 迁移加字段 → Go 照片 job 分流 + `processPostureJob` → AI `posture.py` + `posture_analyzer.py` + prompt + 治理 → vision 路由配置 → 前端展示态与结果卡片 | 上传三视角后能看到 AI 体态发现 + 免责声明，兑现文案承诺 | 3–5 天 |
| **P2** 混合量化 | `pose_estimator.py`（MediaPipe）+ 几何指标 + VLM 融合 + 数值防幻觉校验 + 标注图 | "颅椎角 48°→头前移中度"级别的**量化可信**报告 | 4–6 天 |
| **P3-B1** 诊断台工具 | `get_posture_analysis` Agent 工具 + Go 只读端点 + 工具卡片 | 问诊中 AI 能引用体态分析给方案 | 1–2 天 |
| **P3-B2** 多模态输入 | 全链路 image part + vision 路由 + 契约更新 + 治理 | 会话中直接传图分析 | 3–5 天 |

---

## 8. 风险与注意点

- **VLM 幻觉数值**：Phase 1 严禁输出角度；Phase 2 数值只来自几何、并做后置校验。这是可信度红线。
- **单图深度缺失**：正面照测不了矢状面（头前移），侧面照测不了冠状面（高低肩）——分析要**按视角限定可判断的项**，别让 VLM 跨视角猜。
- **医疗合规**：全程免责声明 + 红旗就医提示；措辞用"倾向/疑似"，绝不"确诊"。
- **隐私**：体态照是敏感 PII。上传即去 EXIF；分析用完的 base64 不落日志；对象存储/本地文件设访问控制；PRD §5.4 要求可删除（现已支持 `DeleteUpload`）。
- **成本与时延**：VLM 调用比文本贵且慢，异步 job 化（已具备）避免阻塞；三视角可并发分析。
- **契约一致性**：B2 改了消息 parts 结构，务必同步 `packages/contracts` 的 schema+fixtures，让 Go/Python/TS 三方契约测试通过（这正是你已有的强项，别破坏它）。
- **A-1 前置**：AI 服务的阻塞 DB 问题（见审查报告）在照片分析这种更重的多模态负载下会更明显，建议先修连接池（`psycopg_pool` 已装）。

---

## 9. 求职亮点叙事（做完能怎么讲）

> "我给体态助手做了**多模态照片分析**：用**姿态估计提取骨骼关键点算出颅椎角/肩倾角等量化指标**，再让**视觉大模型做定性描述**，两者由 LLM 融合成结构化报告，并用**RAG 知识库佐证 + 忠实度校验防止模型编造数值**，最后接入 Agent 问诊 —— AI 能主动读取用户的体态档案给出改善方案。整条链路是**可恢复的异步 job**，还带**红旗就医提示与医疗免责的安全治理**。"

这一段同时踩中：多模态、CV/姿态估计、RAG、Agent 工具调用、AI 安全治理、异步任务系统 —— 是把你现有架构价值最大化的一个功能。

---

## 10. 落地第一步（建议）

若确认，从 **P1** 开始，第一批改动清单：
1. `migrations/00XX_add_upload_analysis.{up,down}.sql` + `model/user_upload.go`
2. `upload_service.go`：`postureJobType`、上传分流、`processPostureJob`、worker 泛化；`upload_repository.go`：`UpdateAnalysisResult`
3. `ai-service`：`api/routes/posture.py`、`services/posture_analyzer.py`、`prompts/posture.py`、`models/posture.py`（Pydantic）、`main.py` 注册、`config/models.yaml` 加 `posture.analyze` 路由
4. `.env*`：加 vision 模型相关变量（`VISION_MODEL` 等）
5. 前端：`upload.types.ts`、`UploadStep.tsx` 展示态、`PostureAnalysisView`
6. 测试：AI 服务加 `test_posture_analyzer`（mock provider）；Go 加 `processPostureJob` 单测；契约/评估按需扩展

需要我把 P1 展开成可直接执行的逐文件实现（含代码），还是先按某个阶段进入编码？
