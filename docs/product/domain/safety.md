# BodySense · SafetyState / Trainer 安全门禁领域契约

**状态：2026-10-13 Safety 一揽子业务决策（SAFE-D1～D8）已收敛。** 这是产品级风险状态与授权规则，**不是临床分诊算法、医学阈值或经验证的医疗器械规则**；症状判定和处方禁忌需要后续有证据的安全策略及专业评审。

## 2.3 SafetyState 与执行限制（SAFE-D1～D2）

- `SafetyState` **由 BodyState 持有**，按用户隔离；`SafetyRestriction` 是逐条具有来源、范围和解除审计的独立对象。
- `TrainingPlan` 仍拥有唯一内容与状态，`Trainer` 在每次开始、恢复、前进到受影响动作以及安全限制改变时从 SafetyState 获取**当前有效门禁**。安全事件不生成 TrainingPlanRevision，不将 plan.status 改为 paused/completed。
- `Diagnosis` 只有遇到需要立即停止正式医学分析的 `urgent` 警示才按 INV-23 的既有规则 safety_blocked 终结该 Run；普通训练限制不停止正式分析，也不限制普通健康知识问答。

~~~ts
type SafetyState = {
  restrictions: SafetyRestriction[];
};

type SafetyRestriction = {
  restrictionId: string;
  userId: string;
  riskLevel: "advisory" | "restricted" | "urgent";
  reason: string; // 具体事实与用户可理解的提示；不是自动临床诊断
  scope:
    | { kind: "all_training" }
    | { kind: "plan"; planId: string }
    | { kind: "exercise"; exerciseIds: string[]; planId?: string };
  validFrom: string;
  validUntil?: string; // on_expiry 为真正失效时间，其他策略只作复核时点
  liftPolicy: "on_expiry" | "user_reverification" | "professional_clearance";
  liftConditions: string[];
  liftedAt?: string;
  liftedReason?: string;
  liftBasis?:
    | { kind: "expiry" }
    | { kind: "user_reverification"; checkInteractionId: string }
    | {
        kind: "professional_clearance";
        sourceKind: "user_message" | "clinical_document";
        sourceId: string;
      };
  sourceIds: string[]; // 有来源及版本的事实、观察或正式安全评估
};

type TrainerGate = {
  result: "allow" | "warn" | "block";
  matchedRestrictionIds: string[];
  reason: string[];
  requiresReview: boolean;
}; // 只读投影；不持久化为另一份真值
~~~

- `advisory`：提醒但不强制阻断指定执行范围；`restricted`：阻断匹配的计划/动作；`urgent`：全训练阻断并优先展示必要的紧急就医提示，亦可阻断正在执行的 Diagnosis Run。它们是**产品执行等级**，不是医学诊断分级或临床严重程度判定。
- `all_training` 覆盖任意计划；`plan` 覆盖该计划的所有课模板；`exercise` 按标准动作 concept `exerciseId` 判断，可限定 planId。禁止按显示名称或 UI `planExerciseId` 绕过同一动作风险；自定义动作匹配信息不足时，相关路径须明确保守阻断并要求复核，不得默认标示安全。
- 对重叠且仍有效的限制取**最严格结果**：urgent block > restricted block > advisory warn；展示各自 reason，解除一条不影响其他条。不能因最近添加了较轻限制就覆盖先前风险。
- `SafetyRestriction` **不另设持久 status 枚举**；是否有效从 validFrom、validUntil、liftPolicy、liftedAt、liftBasis 派生。`TrainerGate` 只能从当前限制推导，不作第二权威存储。

## 2.4 分类依据与策略默认值（SAFE-D3～D4）

| 等级 | 门禁与最小用户反馈 | 默认解除策略 |
| --- | --- | --- |
| advisory | 匹配范围提示风险/注意事项，不阻断；必要时请求用户澄清 | `user_reverification`；仅有明确、已审核的短期规则和期限才允许 `on_expiry` |
| restricted | 阻断匹配动作/计划；展示原因、影响范围、复核路径 | `user_reverification`，**若涉及需专业评估的条件则 `professional_clearance`** |
| urgent | 阻断所有训练，突出安全提示与适当的就医建议；需要停止正式分析时按 `safety_blocked` 终结 | **`professional_clearance`**，不得自动到期解除 |

- Agent 可根据明确的新事实提出/上报风险并触发**更保守**的临时门禁；最终写入由 Go 后端校验**结构化风险分类规则、证据来源与权限**，不可由模型自由文本单独决定医学风险等级或擅自判定安全。新鲜警示不能等待 Episode 合并、计划提案授权或新一轮诊断采纳才处理。
- `on_expiry` 不是等级默认行为：只适用于规则已明确为低风险且具备确定有效期的限制；缺乏确定期限时不可擅自设置。`validUntil` 不填或不可靠时必须显式复核，而非把“暂时”猜成某个天数。
- 证据不足时不声明「可安全训练」。涉及当前动作的未澄清高风险信号，保守阻断其适用范围并要求澄清；不能据此自动把任何普通症状升级为全局 urgent。临床风险判定阈值、红旗规则和处方禁忌需独立版本化、专业审核和回归验证。

## 2.5 解除与来源变更（SAFE-D5）

| 策略 | 合法解除触发 | 不合法行为 |
| --- | --- | --- |
| on_expiry | `validUntil` 到达，按 `liftBasis.kind=expiry` 对**该条限制**幂等记录解除 | 缺期限自动解除，或顺带清除其它限制 |
| user_reverification | 用户对指定风险完成明确的复核问答、条件满足，并保存 `checkInteractionId` | 复核时间到达即解除；Agent 自行判断“应该没事了” |
| professional_clearance | 用户提供有来源的专业人员放行陈述或经准入的临床文件，满足「针对哪项活动/限制」等 liftConditions；记录来源等级 | 普通无来源肯定句、OCR 未准入候选当成医生放行；用户转述自动升级为临床文件证明 |

- `validUntil` 对非 `on_expiry` 策略仅是**建议复核时点**；到期仍保持阻断，直到具有匹配解除依据。
- 撤回/更正某条来源事实只触发**此限制的证据复评**，不会自动清空限制。无法证明当前风险解除时继续保持原门禁或临时保守限制；必要时按规则产生新限制并保留旧历史。
- Agent 不能撤销/降级 `riskLevel`、放宽 `liftPolicy` 或将专业放行改为自述复核。任何解除均记录操作者、来源、时间、限制 ID、依据及修订；来源失效或冲突则保留限制并提示用户复核。
- 并发新增与解除使用**按 restrictionId 的条件更新 + SafetyState 修订校验**；只能解除被准确指向、已满足条件的限制。跨设备/重试不产生重复解除，也不覆盖并发新增的更严重风险。

## 2.6 执行中新增安全限制（SAFE-D6）

- 新限制出现后，应让相关 Trainer 立即得知并重新计算门禁；**任何下一步动作的开始/继续由后端再次检查**，不能依赖前端缓存成功。
- 如果 `restricted` 命中正在执行的动作或计划：Session **暂停**为 `state=paused,pauseReason=safety_hold`，保留已完成记录；解除前不可继续对应范围，但其他不受限动作可在用户明确选择后继续（不得自动修改计划/制造跳过）。
- 如果 `urgent` 生效：Trainer 安全终止本次执行 `state=ended,endedReason=safety_stopped`，并阻断所有新训练；保留原真实动作与反馈记录，不能自动把剩余动作记为 skipped/completed。
- 断网、运行时失败或安全检查不可用：**不开放新的受影响执行动作**，可查看既有记录并提示检查不可用/稍后重试；不是自动结束或修改处方。
- SafetyState 的阻断权限独立于用户接受 PlanProposal 的权限；用户拒绝改计划不等于有权覆盖安全限制。

## 2.7 状态、UI 与 Agent 语义（SAFE-D7）

- UI 投影明确区分：可执行、提醒仍可执行、局部受限、整个计划受限、全训练紧急阻断、到复核时点但尚未放行、专业放行信息待核对、历史已解除。
- 展示**范围、原因、解除途径**，不把「计划暂停」作为安全限制提示；不将“训练界面关闭”当成风险解除。
- Chat 可引导用户澄清具体信号、查看限制或上传证据，但任何用户输入/模型回复必须经过 Go 授权与来源核验后才能影响门禁。
- 针对正式诊断：只有 urgent 级安全阻断可结束当前 ConditionAnalysisRun，按既有 INV-23 保留审计且**零 Diagnosis 提交**；用户处理风险后须自己重新发起分析，旧已终结 Run 不可恢复提交。

## 2.8 旧代码差异与实施准入（SAFE-D8）

- 现有 `BodyStateService.ResolveSafetyState` 接受 resolved/cleared_by_review/monitoring 加自由文本；**尚未证明**具备目标 liftPolicy、restrictionId 和 liftBasis 的服务器级校验；旧代码不能当作合规实现。
- 现有 `SetSafetyState` 以整块 JSON 覆盖写入；须迁移为单条限制的授权事件/条件更新，防止解除旧限制时覆盖并发新增的风险。
- 旧 Treatment 暂停/复核状态不可直接映射到 TrainingPlan.status；Trainer 执行门禁应从新的 SafetyState 投影计算。
- 运行前要求结构化安全分类决策、来源和复核验证、可追溯策略版本、单条解除事务、重大/轻微风险重叠测试、断网和并发测试。安全阈值/医疗规则验证不属于此次**业务建模**完成的范围，不能宣称训练医疗安全已通过实际验证。

## 2.9 必须验收的边界案例

1. 相同动作在 A/B 两个计划里出现，禁忌按 exerciseId 命中全部适用条目，不能因计划修订/重命名绕过。
2. 同时存在 advisory、restricted 与 urgent 时逐条保留，门禁采取最严格结果；只解除一条不放行其他有效阻断。
3. 专业放行策略到期但未收到有来源的放行，保持阻断；OCR 候选或无来源说明不当成放行。
4. 训练中收到匹配 restricted → paused(safety_hold)，收到 urgent → ended(safety_stopped)，日志中的已完成组保持原样。
5. BodyState 事实更正/撤回与新增风险并发，不会发生整体 JSON 覆写、默认降级或静默解除。
6. 普通 scoped training restriction 不 safety_blocked 诊断；真正 urgent 安全事件对已产生提案的 Diagnosis Run 执行零提交终结。
