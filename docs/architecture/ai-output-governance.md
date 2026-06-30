# AI Output Governance 架构设计

**文档版本**：v1.0  
**更新日期**：2026-06-29  
**状态**：设计稿  
**适用范围**：诊断、评估报告、训练计划、重评估、知识库精修、Agent 工具结果

---

## 1. 背景

BodySense 的核心价值来自 AI 输出：

- 问诊回复。
- 症状结构化抽取。
- 红旗风险提示。
- 诊断候选。
- 康复训练建议。
- 健康评估报告。
- 训练计划。
- 训练反馈重评估。
- 知识库视频内容精修。

项目已经有一些质量相关 Module：

- `RedFlagDetector`
- `FaithfulnessChecker`
- Pydantic schema validation
- 诊断和训练计划的结构化模型
- RAG citation / knowledge_gap 事件

但这些质量规则还散落在各处。随着工具调用和 Job Runtime 扩展，如果每个调用点自己做 JSON 解析、校验、降级和落库，会出现：

- 有些 AI JSON 不合法仍被保存。
- 训练动作没有知识来源仍被展示。
- 红旗风险只在咨询阶段拦截，评估或训练阶段漏掉。
- 失败时错误格式不一致。
- 很难复盘某次输出为什么被接受或拒绝。

本设计目标是建立统一的 **AI Output Governance**，让 AI 结果先通过治理 Module，再进入业务状态。

---

## 2. 设计目标

1. **统一输出验收**
   所有关键 AI 输出都经过 schema、规则、安全和引用校验。

2. **保存前治理**
   Go 落库前必须知道输出是 `accepted`、`repaired`、`rejected` 还是 `degraded`。

3. **领域安全优先**
   健康建议中红旗、禁忌、过度承诺和医疗诊断边界必须被检查。

4. **RAG 忠实度可查**
   需要引用知识库的建议必须能追溯来源或显式标记知识缺口。

5. **可审计**
   每次 AI 输出治理结果可以记录输入、输出摘要、规则命中和最终处置。

6. **可测试**
   Golden cases 和规则测试成为主要验证面。

---

## 3. 输出等级

```txt
accepted
  输出合法，允许落库和展示

repaired
  输出有小问题，已自动修复，允许落库但记录修复

degraded
  输出不完整或引用不足，使用降级结果展示

rejected
  输出违反安全或结构规则，不允许落库为正式结果
```

---

## 4. 总体架构

```txt
Python AI Service
  - LLM output
  - Pydantic parsing
  - domain validators
  - red flag check
  - faithfulness check
        |
        v
AIOutputGuard
  - validate
  - repair
  - classify
  - explain
        |
        v
Go API
  - persist governance result
  - persist accepted business state
  - emit StreamEvent
        |
        v
React UI
  - render accepted/degraded result
  - show safety or retry state
```

---

## 5. Module 设计

### 5.1 AIOutputGuard

**位置建议**：`apps/ai-service/src/services/governance/output_guard.py`

Interface：

```python
class AIOutputGuard:
    async def validate(
        self,
        output_type: str,
        raw_output: Any,
        context: GovernanceContext,
    ) -> GovernanceResult:
        ...
```

职责：

- 根据 `output_type` 选择 schema。
- 做 Pydantic validation。
- 执行领域安全规则。
- 执行引用忠实度检查。
- 给出最终处置。

不负责：

- 调用模型生成原始输出。
- 写业务数据库。

### 5.2 GovernancePolicyRegistry

**位置建议**：`apps/ai-service/src/services/governance/policies.py`

每类输出注册一组 policy：

```python
class GovernancePolicy(Protocol):
    name: str

    async def check(
        self,
        output: Any,
        context: GovernanceContext,
    ) -> PolicyResult:
        ...
```

输出类型：

```txt
consultation.reply
consultation.extracted_info
consultation.diagnosis
consultation.treatment_plan
assessment.report
training.plan
training.reassessment
knowledge.curated_unit
tool.result
```

### 5.3 GovernanceResult

```python
class GovernanceResult(BaseModel):
    output_type: str
    status: Literal["accepted", "repaired", "degraded", "rejected"]
    value: dict[str, Any] | str | None = None
    issues: list[GovernanceIssue] = Field(default_factory=list)
    repairs: list[GovernanceRepair] = Field(default_factory=list)
    safety_flags: list[dict[str, Any]] = Field(default_factory=list)
    citations: list[dict[str, Any]] = Field(default_factory=list)
    raw_output_summary: str | None = None
```

### 5.4 Go Governance Persistence

**位置建议**：

```txt
apps/api/internal/model/ai_output_review.go
apps/api/internal/repository/ai_output_review_repository.go
```

Go 保存治理结果，但不重复实现复杂 AI 规则。

---

## 6. Policy 分类

### 6.1 Schema Policy

检查输出结构：

- 必填字段。
- enum。
- 数值范围。
- list 长度。
- JSON 是否可解析。

失败策略：

- 可修复字段缺失：`repaired`。
- 核心字段缺失：`rejected`。

### 6.2 Safety Policy

检查健康安全：

- 是否出现红旗症状。
- 是否给出确定医学诊断。
- 是否建议危险动作。
- 是否无条件要求用户继续训练。
- 是否缺少就医提醒。

失败策略：

- 红旗明确：`rejected` 或改为安全回复。
- 轻微措辞问题：`repaired`。

### 6.3 Faithfulness Policy

检查输出是否被知识库支持：

- 训练动作是否来自 retrieved source。
- 关键建议是否有 citation。
- 是否出现知识库之外的具体动作参数。
- 是否把低置信内容说成确定事实。

失败策略：

- 找不到依据：`degraded`，只保留通用建议。
- 明显幻觉：`rejected`。

### 6.4 Business Rule Policy

检查业务状态：

- 诊断必须与已确认症状相关。
- 训练计划必须匹配用户可用器械和时长。
- 重评估不得覆盖用户手动确认的信息。
- consultation phase 不允许倒退。

失败策略：

- 可调整：`repaired`。
- 与用户状态冲突：`rejected`。

---

## 7. 数据库设计

```sql
CREATE TABLE ai_output_reviews (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
    run_id          UUID REFERENCES runs(id) ON DELETE SET NULL,
    job_id          UUID REFERENCES jobs(id) ON DELETE SET NULL,

    output_type     VARCHAR(80) NOT NULL,
    status          VARCHAR(30) NOT NULL,

    raw_output      JSONB,
    reviewed_output JSONB,
    issues          JSONB NOT NULL DEFAULT '[]',
    repairs         JSONB NOT NULL DEFAULT '[]',
    citations       JSONB NOT NULL DEFAULT '[]',
    safety_flags    JSONB NOT NULL DEFAULT '[]',

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_output_reviews_run
    ON ai_output_reviews (run_id);

CREATE INDEX idx_ai_output_reviews_job
    ON ai_output_reviews (job_id);
```

---

## 8. 输出类型治理要求

| output_type | 必须校验 | 可降级 |
|---|---|---|
| `consultation.reply` | safety、citation when advice | 是 |
| `consultation.extracted_info` | schema、state merge rule | 是 |
| `consultation.diagnosis` | schema、red flag、evidence | 否 |
| `consultation.treatment_plan` | schema、faithfulness、contraindication | 是 |
| `assessment.report` | schema、profile consistency | 是 |
| `training.plan` | schema、exercise safety、faithfulness | 是 |
| `training.reassessment` | schema、plan mutation rule | 是 |
| `knowledge.curated_unit` | source evidence、toxic text、quality | 否 |

---

## 9. StreamEvent 扩展

```txt
governance.reviewed
governance.degraded
governance.rejected
```

示例：

```json
{
  "channel": "debug",
  "type": "governance.reviewed",
  "payload": {
    "output_type": "training.plan",
    "status": "degraded",
    "issues": [
      {
        "code": "MISSING_CITATION",
        "message": "Two exercises were not supported by retrieved knowledge"
      }
    ]
  }
}
```

生产环境可以不向普通用户展示 debug 事件，但应写日志或数据库。

---

## 10. 和现有 Module 的关系

### 10.1 DiagnosisService

当前 `DiagnosisService` 做 Pydantic validation。目标是：

```txt
LLM output -> DiagnosisResponse schema -> AIOutputGuard -> Go persist diagnosis
```

### 10.2 FaithfulnessChecker

`FaithfulnessChecker` 作为 Faithfulness Policy 的 Adapter。

### 10.3 RedFlagDetector

`RedFlagDetector` 作为 Safety Policy 的 Adapter。

### 10.4 Tool Runtime

工具返回结果进入模型前，可以先通过轻量治理：

- tool result 是否过长。
- 是否包含敏感信息。
- 是否符合工具 schema。

---

## 11. 测试策略

### 11.1 Golden Cases

扩展：

```txt
apps/ai-service/tests/golden/
  diagnosis/
  treatment_plan/
  training_plan/
  assessment_report/
```

每个 case 包含：

```txt
input_context.json
raw_output.json
expected_governance.json
```

### 11.2 单元测试

```txt
apps/ai-service/tests/unit/test_output_guard.py
apps/ai-service/tests/unit/test_governance_policies.py
apps/ai-service/tests/unit/test_faithfulness_checker.py
apps/ai-service/tests/unit/test_red_flag_detector.py
```

覆盖：

- schema 不合法时 rejected。
- 轻微字段缺失时 repaired。
- 无引用动作时 degraded。
- 红旗风险时 rejected。
- 用户禁忌动作被拦截。

---

## 12. 分阶段落地

### Phase 1：统一结果模型

- 新增 `GovernanceResult`。
- 将现有 Pydantic validation 包装成 Schema Policy。

### Phase 2：接入诊断和训练计划

- 诊断、治疗方案、训练计划生成后先过 Guard。
- rejected 不落库。

### Phase 3：接入 Faithfulness 和 RedFlag

- `FaithfulnessChecker` 进入 policy registry。
- `RedFlagDetector` 进入 policy registry。

### Phase 4：Go 持久化审计

- 新增 `ai_output_reviews`。
- Job/run 关联 review。

### Phase 5：前端降级展示

- 对 degraded 输出显示保守建议。
- 对 rejected 输出显示重试或安全提示。

---

## 13. 成功标准

落地后应满足：

1. 关键 AI 输出不会绕过 schema 校验直接落库。
2. 训练计划中的动作能追溯知识来源或被降级。
3. 红旗风险不会只在咨询回复中生效。
4. 失败输出有统一错误和审计记录。
5. 可以用 golden cases 回归 AI 输出质量。
6. 后续新增输出类型只需要注册 policy，不需要在各 handler 散写校验。

