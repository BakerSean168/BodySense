# BodySense · SafetyState 领域契约

SafetyState 由 BodyState 持有；Trainer 只执行安全门禁，不持有风险真值。

## 2.3 SafetyState 与执行限制

~~~ts
type SafetyState = {             // 风险状态的权威来源（R-SAFE-03）
  restrictions: SafetyRestriction[];
};

type SafetyRestriction = {
  restrictionId: string;
  riskLevel: string;             // 风险等级；取值集合待定（Q-18）
  reason: string;                // 面向用户的原因与就医提示
  scope:
    | { kind: "plan"; planId: string }
    | { kind: "exercise"; planId: string; exerciseIds: string[] };
  validFrom: string;
  validUntil?: string;           // on_expiry：到期时间，必填；其他策略：复核时点，为空表示无预定复核时点
  liftPolicy: "on_expiry" | "user_reverification" | "professional_clearance";  // 解除策略，见下文
  liftConditions: string[];      // 如「经医生检查确认可恢复训练」
  liftedAt?: string;
  liftedReason?: string;         // 解除时记录满足了哪个条件及来源
  liftBasis?:                    // 解除依据，必须与 liftPolicy 一致
    | { kind: "expiry" }                                              // 仅 on_expiry
    | { kind: "user_reverification"; checkInteractionId: string }     // 用户经复核问答重新确认症状
    | { kind: "professional_clearance";
        sourceKind: "user_message" | "clinical_document"; sourceId: string };  // 用户记录的专业放行及来源
  sourceIds: string[];           // 触发该限制的事实 / 观察 / 分析
};
~~~

- SafetyState 持有风险状态；Trainer 在开始和继续执行时检查当前有效的限制并把关（R-SAFE-03）。限制不修改 `TrainingPlan.status` 或计划内容（INV-08）。
- 范围为 `exercise` 时只拦截指定动作。
- **解除规则**（R-SAFE-04、INV-20）：
  - `on_expiry`：到达 `validUntil` 自动解除（`liftBasis.kind = expiry`）。只有这一策略允许到期自动解除。
  - `user_reverification`：到达 `validUntil` 只触发复核提示，限制继续生效；用户通过复核问答重新确认症状、结果满足 `liftConditions` 后才解除。
  - `professional_clearance`：到达 `validUntil` 同样只触发复核提示，限制继续生效；用户记录专业人员（如医生）的放行并给出来源（用户消息或经 [证据管线](../../architecture/health-document-evidence-pipeline.md) 采纳的文件证据）后才解除。
  - 需要就医检查的警示类风险（红旗征象 / 建议就医）默认 `professional_clearance`。其余风险等级的取值及其默认策略见 [Q-18](../open-questions.md)。
  - Agent 不能自行解除限制，也不能放宽已设定的 `liftPolicy`；每次解除必须写入与策略一致的 `liftBasis`。

## 2.4 生命周期、作用范围与授权

SafetyRestriction 没有获批准的独立持久化 status 字段，限制是否生效应依据 validFrom、有效期、liftedAt、liftPolicy、scope 和事实来源派生，不能与 TrainingPlan.status 混用。

| 情况 | 已定义的合法行为 | 不得发生的行为 |
| --- | --- | --- |
| 新增限制 | BodyState 记录来源和范围；Trainer 在开始、继续执行时查询匹配限制 | 静默改写训练处方/计划状态 |
| on_expiry | validUntil 必填；到期后可按 expiry 记录自动解除 | 对专业人员放行类风险自动解除 |
| user_reverification | 复核时点到达仍保持限制，直到用户复核符合条件且留痕 | Agent 自行解除 |
| professional_clearance | 用户记录专业人员放行及来源，满足解除条件后才解除 | 仅凭时间经过、无来源直接清除 |
| 严重风险中断 Diagnosis | 对真正需要停止分析的警示，触发安全终结及无修订；保留决定审计 | 将普通训练限制一律当作诊断中断 |
| 执行中的新风险 | 下一次继续执行前必须重新检查限制 | 具体暂停/终止和日志语义仍待 DC-006 |

**P0 待决**：riskLevel 的枚举/分类（DC-001/002）、全训练限制范围（DC-003）、结构化解除依据（DC-004）、新分析启动门槛（DC-005）、训练中途阻断行为（DC-006）；这些都不能从旧代码或 Penpot 推断。

## 2.5 UI 派生状态、事件与验收

UI 必须能够区分：当前可执行、存在匹配限制而暂不宜执行、已到复核时点但限制尚未解除、专业人员放行待核实、已解除/历史以及紧急就医警示。以上是**展示投影**，不新增持久化枚举。

解除/复核事件必须关联 restrictionId、用户、源证据及策略相匹配的 liftBasis；Go 后端应控制权限，Agent 不得降低风险级别或放宽解除策略。验收沿用 TP-06～06f，增加 SAFE-01～03；Q-18 未决内容不伪造为已通过验收。
