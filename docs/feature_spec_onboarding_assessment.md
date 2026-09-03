# 功能设计文档：用户信息构建与健康评估 (Feature Line 1)

## 1. 功能概述

本功能线是用户进入 BodySense 的起点，但 **Onboarding 只是采集入口，不是数据所有者**。一次表单会把不同语义的信息分别交给稳定 Profile、BodyState Fact、BodyState Observation 与上传/评估管线，而不是把所有答案压进 `user_profiles`。

当前 North Star（ADR 0007）：

- `UserProfile`：只保存性别、出生日期等稳定身份背景；年龄按出生日期派生。
- `BodyState Observation`：身高、体重等身体测量；后续更新保留历史。
- `BodyState Lifestyle Fact`：日常活动、睡眠、运动、饮食节律、酒精/烟草/咖啡因、恢复压力。
- `BodyState history Fact`：既往伤病/手术摘要。
- 上传资料：继续进入现有 Upload / OCR / posture-analysis 流程。
- 当前症状、目标和主观困扰：进入长期 Consultation + BodyState，而不是静态档案。

采集遵循“少填表、讲真实情况”的原则：不问职业名称；不强迫不规律作息变成固定入睡/起床时间；生活方式优先自然语言，只在运动频率等适合比较的地方做轻量量化。

---

## 2. 业务流程与状态机

### 2.1 步骤流转流程 (User Wizard Flow)

```mermaid
graph TD
    Start([用户进入应用]) --> Identity[稳定身份: 性别/出生日期]
    Identity --> Metrics[身体测量: 身高/体重]
    Metrics --> Activity[日常活动]
    Activity --> Sleep[睡眠与作息]
    Sleep --> Exercise[运动类型与频率]
    Exercise --> MoreLifestyle[饮食/相关摄入/恢复压力]
    MoreLifestyle --> History[既往伤病与手术史]
    History --> Upload[材料上传-可选]
    Upload --> Persist[分别写入 Profile + BodyState]
    Persist --> Assessment[生成 observation-only 初始评估]
    Assessment --> End([进入长期健康工作台])
```

1. **稳定身份**：性别、出生日期写入 `UserProfile`；出生日期是年龄的唯一来源。
2. **身体测量**：身高/体重写入 `anthropometry.height/weight` Observation；BMI 只作为当前投影派生。
3. **日常活动**：自然语言描述久坐、久站、走动、搬抬、重复动作或轮班，写入 `lifestyle.activity`。
4. **睡眠与作息**：描述规律性、轮班、通常睡眠时长、夜醒和缺觉，写入 `lifestyle.sleep`。
5. **运动**：类型文本 + 轻量频率选择，形成 `lifestyle.exercise` summary/details。
6. **其他生活方式**：饮食节律、酒精/烟草/咖啡因、恢复与压力分别进入 nutrition/substances/recovery；全部可选。
7. **既往状况**：重要伤病/手术摘要写入 `history.injury_summary`，不进入 Profile。
8. **材料上传（可选）**：体态照片与体检报告沿用现有上传流程。
9. **初始评估**：Assessment 明确接收 stable Profile 与 current BodyState 两个独立输入，形成待审核 Observation。

后续“生活方式”编辑器与 Consultation 都复用同一 BodyState taxonomy，但写入权限不同：编辑器是直接用户确认，可立即形成 confirmed current；Consultation 的模型归一化先保存为 `ai_extracted / unverified / excluded_from_reasoning` 候选，并在“生活方式”页等待用户确认。确认候选后，若旧事实曾经为真，则使用 temporal transition / supersedes 形成历史；若旧事实本身错误，则使用 correction。

### 2.2 状态机设计 (User Onboarding State Machine)

| 当前状态                | 触发事件          | 目标状态                | 动作/说明                                                                          |
| :---------------------- | :---------------- | :---------------------- | :--------------------------------------------------------------------------------- |
| `UNINITIALIZED`         | 进入页面          | `STEP_IDENTITY`         | 初始化分步表单                                                                     |
| `STEP_IDENTITY`         | 完成身份          | `STEP_METRICS`          | 校验出生日期                                                                       |
| `STEP_METRICS`          | 完成测量          | `STEP_LIFESTYLE`        | 校验身高/体重范围                                                                  |
| `STEP_LIFESTYLE`        | 完成/跳过生活方式 | `STEP_HISTORY`          | 自由文本可为空                                                                     |
| `STEP_HISTORY`          | 完成/跳过伤病史   | `STEP_UPLOAD`           | -                                                                                  |
| `STEP_UPLOAD`           | 点击开始评估      | `PERSISTING_CONTEXT`    | 先按领域边界写 stable Profile 与 BodyState                                         |
| `PERSISTING_CONTEXT`    | 写入成功          | `GENERATING_ASSESSMENT` | Assessment 从 frozen health input 构建 evidence catalog；图片本身不进入 Assessment |
| `PERSISTING_CONTEXT`    | 写入失败          | `STEP_UPLOAD`           | 显式错误；不得只保存一份胖 Profile 作为降级真值                                    |
| `GENERATING_ASSESSMENT` | 报告生成成功      | `REPORT_COMPLETED`      | 通过 evidence contract 的 Observation 写入 BodyState；进入工作台                   |
| `GENERATING_ASSESSMENT` | 生成失败          | `STEP_UPLOAD`           | 保留已提交的用户健康上下文，允许重试 Assessment                                    |

---

## 3. AI 节点设计

### 3.1 节点 1：体检报告 OCR / 指标提取机制

OCR 是**非 LLM mechanism**，不是一个医学数据清洗 Agent。当前实现链路：

```text
UploadStorage bytes
  -> durable JobRuntime OCR job
  -> Python /api/ocr/extract
  -> Tesseract OCR
     - image: pytesseract
     - PDF: PyMuPDF 逐页渲染后 OCR
  -> deterministic HealthIndicator regex extractor
  -> OCRResult(raw_text, indicators, confidence)
     -> per-indicator evidence_admissibility
  -> user_uploads.ocr_result
```

每个 `HealthIndicator` 当前包含：

```text
name
value
unit?
reference_range?
confidence = high | medium | low | unknown
evidence_admissibility = admissible | needs_review | rejected
evidence_admissibility.policy_revision = ocr-indicator-admissibility-v1
```

当前实现**没有**再调用 LLM 根据 OCR 文本自由挑选“骨骼/肌肉相关指标”，也没有让模型生成 `normal/high/low` 医学解释。这样可以避免把 OCR 噪声经过第二个生成模型进一步放大。

当前 `ocr-indicator-admissibility-v1` 明确把“识别完成”和“可用于健康推理”分开：只有 OCR confidence 与 indicator confidence 都为 `high` 的完整指标可自动标记为 `admissible`；`medium / low / unknown` 保留为 `needs_review`，不会进入当前 Assessment evidence catalog。`admissible` 只表示可作为来源证据，不表示医学正常/异常或用户确认。完整决策见 ADR 0011。

> [!warning] Remaining provenance gap
> OCR engine/parser/PDF rendering/indicator extractor revision 仍未形成完整 immutable mechanism identity。该剩余问题单独记录在 `docs/plan/active/2026-09-01-documentation-code-alignment-audit.md`，不得与本次已经完成的 indicator admissibility 混淆。

### 3.2 节点 2：Evidence-grounded Assessment

**核心意图**：生成 reviewable observation candidates，而不是把 Onboarding 资料直接变成诊断或治疗建议。

**输入边界必须分开**：

```text
profile
  stable identity only
  -> gender / birth_date / derived age_years

body_state
  current health truth
  -> lifestyle facts
  -> injury/history facts
  -> confirmed anthropometry observations
  -> other current facts/observations

report_indicators
  -> 仅当前 admissibility policy 允许进入 Assessment 的外部报告结构化指标

posture_analysis
  -> Posture Agent 已完成并治理过的体态观察证据
```

禁止为了 Prompt 方便把 BodyState、OCR 指标或 lifestyle 再嵌回 `profile`。Assessment 需要健康上下文时以 `body_state` 为准。

**当前 serving configuration：Assessment v5 / `assessment-output-v2` / `assessment-evidence-contract-v4`。** Assessment v4 / `assessment-evidence-contract-v3` 保持不可变，仅用于历史 replay。v5 在不改变 machine-admissible 证据权限的前提下，新增独立的 `human-reviewed` 报告证据通道：只有当前有效的 confirmed/corrected review 可进入 catalog，rejected 或已被后续 review supersede 的状态继续 fail closed。模型权限仍收敛为 evidence selection/classification。每个候选只能输出 `kind + 单个 evidence_ref`；模型没有 `label / description / body_region / severity / confidence` 等可持久化自然语言权限，也不再生成健康等级、0-100 分数、总体 summary 或 information gaps。Python 与 Go 都从 frozen evidence 快照确定性渲染 observation 文案，避免“ref 正确但模型仍扩写一个 unsupported claim”。

**证据来源**：应用层从 frozen health input 构建 evidence catalog，仅允许 `posture_analysis / body_state / report`。`profile` 是稳定身份背景，不是健康 observation evidence；raw image 与未建模 `rag_context` 也不能直接进入 serving Assessment contract。模型声明的 ref 不可信，Python governance 与 Go durable boundary 都必须验证 ref 真实存在、唯一且与 observation kind 的 evidence policy 匹配。完整决策见 [[ADR 0009]] `docs/adr/0009-adopt-evidence-grounded-assessment-contract.md`。

---

## 4. 数据结构与上下文

### 4.1 Onboarding 持久化边界

Onboarding 前端可以维护一个临时 form state，但提交时只发送一个 **application command**：

```text
PUT /api/v1/onboarding/context
```

请求按领域语义分组，而不是伪装成一个长期 Profile：

```json
{
  "profile": {
    "gender": "male",
    "birth_date": "1998-05-20"
  },
  "body_metrics": {
    "height_cm": 178.5,
    "weight_kg": 75.0
  },
  "lifestyle": {
    "activity": { "summary": "工作日久坐为主，每次连续坐 2-3 小时" },
    "sleep": { "summary": "白班和夜班交替，平均每天睡 6-7 小时" },
    "exercise": {
      "summary": "健身房抗阻训练；频率：1-2",
      "details": { "type": "健身房抗阻训练", "frequency": "1-2" }
    },
    "nutrition": { "summary": "三餐通常规律" },
    "substances": { "summary": "每天咖啡 2 杯，不吸烟" },
    "recovery": { "summary": "工作日压力偏高，周末恢复较好" }
  },
  "injury_history": "两年前左膝轻微拉伤，偶有酸痛"
}
```

后端 `OnboardingContextService` 使用现有 `TransactionManager` 协调 ProfileRepository 与 BodyStateRepository：

```text
one HTTP command
    |
    +-- stable identity -> user_profiles
    |
    +-- height / weight ------------------+
    +-- six lifestyle sections -----------+--> one BodyStateCurrentContextPatch
    +-- injury-history summary ------------+        -> one BodyStateRevision

all writes participate in one database transaction
```

因此一次 onboarding 基线提交具有两个强约束：

1. **原子性**：Profile 或 BodyState 任一写入失败，整个 onboarding context 提交回滚；不会留下“档案写了一半”的状态。
2. **语义 revision**：身高、体重、六类生活方式和伤病摘要一起构成一个 BodyState revision，而不是按字段生成多条 revision。

后续的 Lifestyle / Body Metrics / Health History 编辑器仍可分别提供聚焦的 application API；它们写入相同 BodyState 真值，不形成第二套数据。

然后触发 Assessment。Assessment 的 replay input 分别保存 `profile`、`body_state`、`report_indicators` 与 posture input，以保证反事实评测不重新混淆领域边界。

#### 数据库持久化结构 (assessment_reports 表)

```json
{
  "id": "8f8b8a8b-4a5d-4f1a-b6d8-74431e7845ba",
  "user_id": "33b8a32a-5b12-4c07-95ba-13d8756c9a62",
  "status": "completed",
  "contract_revision": "assessment-output-v2",
  "evidence_coverage": {
    "status": "partial",
    "available_sources": ["body_state"],
    "domains": {
      "posture": { "status": "missing", "evidence_refs": [] },
      "exercise": {
        "status": "available",
        "evidence_refs": ["body_state:fact:<uuid>"]
      },
      "lifestyle": {
        "status": "available",
        "evidence_refs": ["body_state:fact:<uuid>"]
      },
      "anthropometry": { "status": "missing", "evidence_refs": [] },
      "health_report": { "status": "missing", "evidence_refs": [] },
      "injury_symptoms": { "status": "missing", "evidence_refs": [] }
    }
  },
  "observations": [
    {
      "kind": "exercise_pattern",
      "label": "运动记录",
      "description": "来源记录：健身；频率：1-2。",
      "review_state": "unverified",
      "evidence_refs": ["body_state:fact:<uuid>"]
    }
  ],
  "evidence_gaps": [
    {
      "dimension": "posture",
      "description": "当前未提供已完成的体态分析。",
      "needed_sources": ["posture_analysis"],
      "required": false
    }
  ],
  "summary": "当前资料支持 1 项待审核观察；2/6 个证据领域已有资料，4/6 个领域当前未提供资料。",
  "created_at": "2026-06-29T11:24:22+08:00"
}
```

`health_grade` 与 `dimension_scores` 仅为历史 `assessment-output-v1` 兼容字段；新报告不再写入。没有明确、可执行的评分 rubric 时，不允许用任意 0-100 数字表达健康程度。

---

## 5. 异常与兜底策略

### 5.1 OCR 处理异常

- **情况 1：上传内容不是可识别体检报告，或 OCR 文本/指标为空**。
  - _处理_：OCR job 可以 completed，但 `indicators=[]`；Assessment 不把空报告制造成健康 observation。
  - _前端_：明确提示“未识别到可用体检指标”，其它 BodyState/Posture evidence 仍可独立参与 Assessment。
- **情况 2：OCR job 失败、超时或服务重启恢复**。
  - _处理_：JobRuntime 持久化 job 状态与幂等键 `upload_ocr:<upload_id>`；失败/超时写入 upload OCR 状态，不能伪装成 completed evidence。
  - _恢复_：服务启动后的 recoverable-job 扫描负责处理 pending/stale running OCR job；不依赖不可恢复的 request goroutine。

### 5.2 Posture 图像分析失败或尚未完成

- **情况：Posture Agent 无法读取/分析图片，或用户图片仍处于 pending**。
  - _权威边界_：Assessment 不直接解释 raw image；图片 → 体态 observation 只由 Posture Agent 负责。
  - _降级处理_：若仍有 BodyState/report evidence，可仅生成这些 evidence 能支持的 observation；`posture` coverage 标记为 `missing`。
  - _前端交互_：明确提示“本次没有可用的已治理体态分析证据”，而不是展示伪造的体态分数或结论。

### 5.3 LLM 输出格式或 evidence contract 校验失败

- **情况：模型输出无法通过 typed schema、evidence refs、observation-only 或 Go durable revalidation**。
  - _处理原则_：fail closed。原始输出不得修补后强行进入 BodyState，也不得用固定 B 级、70 分或“亚健康倾向”等静态健康结论兜底。
  - _允许的安全 fallback_：返回不含健康 claim 的错误/空状态，由客户端提示重新生成或补充资料。若应用层能够从 frozen input 确定性计算 evidence coverage/gaps，可展示这些 coverage 信息，但不得伪造 observation。
