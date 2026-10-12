# BodySense · TrainingPlan / Trainer 领域契约

**状态：2026-10-13 Training 一揽子业务决策（TP-D1～D12）已收敛。** 本文是目标领域契约，不是当前数据库或 API 的现状；物理 Schema、旧 Treatment 迁移和具体算法属于后续工程设计。与旧版待决定表冲突时，以本版本为准。

## 4. TrainingPlan

### 4.0 权威边界

- **TrainingPlan**：用户可拥有多份独立、完整的已接受训练处方。每份计划分别持有内容、计划状态、修订历史；每个 PlanSessionTemplate 是一次课的处方模板，不是训练事实。
- **Trainer / TrainingSessionLog**：执行某份计划、某次课；真实完成/跳过、计时、反馈只记录为执行事实，绝不回写处方。浏览、演示播放、下一项、断线与关闭窗口都不能制造完成记录。
- **BodyState**：身体症状、体征、训练后真实身体变化和 SafetyState 的权威来源。反馈中明确的身体陈述遵循 BodyState 自动记录/澄清规则；不能把主观反馈直接提升为医学因果或确诊。
- **Diagnosis**：仅提供经采纳的具名医学诊断及相关证据上下文。计划可关联零个或多个 ConditionEntry，也可关联 BodyState 证据；不要求存在 Diagnosis 才能训练。
- **SafetyState**：风险限制唯一权威；Trainer 在开始和每次继续执行前检查。限制**不自动修改** TrainingPlan.status、处方、修订或既有训练事实。
- **Agent**：可提出计划草案与影响建议；其推理/提示不等于业务写入。Go 域服务校验主体、精确目标、用户授权、修订和安全门禁。
- 不新建 BodyConcern；关联 BodyState 时使用稳定的 BodyFact、SymptomEpisode、BodyObservation 的 ID/证据版本。派生 ConcernGroup 仅供展示组织，不持有计划关系真值。

## 4.1 逻辑对象（业务语义已定）

~~~ts
type TrainingPlan = {
  planId: string;
  userId: string;
  title: string;
  goal: string;
  relatedConditionIds: string[]; // Diagnosis: 可为空、多个；不是医学确诊推断
  relatedBodyRefs: {
    kind: "body_fact" | "symptom_episode" | "body_observation";
    id: string;
    sourceVersion?: string;
  }[];
  status: "active" | "paused" | "completed"; // 用户管理状态，不是安全门禁
  currentAcceptedRevisionId: string;
  currentContent: TrainingPlanContent; // 指向最新已接受修订的不可变快照
};

type TrainingPlanContent = {
  sessions: PlanSessionTemplate[]; // 至少一个，允许 A/B 等多课模板
  frequency: { sessionsPerWeek?: number; description?: string };
  schedule:
    | { mode: "frequency_only" } // 默认：目标频率，不承诺具体星期几
    | { mode: "calendar"; timeZone: string; occurrences: PlannedOccurrence[] };
  constraints: string[]; // 处方注意事项；不是安全限制的第二权威
};

type PlanSessionTemplate = {
  templateId: string; // 同一课模板跨非替换修订维持稳定
  title: string;
  stages: {
    phase: "warmup" | "main" | "cooldown";
    exercises: PlanExercise[];
  }[]; // 顺序有意义；热身/放松按实际需要，不能强行添加
};

type PlanExercise = {
  planExerciseId: string; // 此模板里的独立条目身份，不按名字识别
  exerciseId: string; // 标准动作概念 ID 或稳定的自定义动作 ID
  nameSnapshot: string; // 供历史准确展示
  sets: number;
  target:
    | { kind: "repetitions"; min: number; max?: number }
    | { kind: "duration"; seconds: number }
    | { kind: "distance"; meters: number };
  restSeconds?: number;
  side?: "left" | "right" | "both" | "alternating";
  loadOrIntensity?: string; // 需要时记录强度，不凭空指定
  instructions?: string;
  demoAssetId?: string; // 仅演示，不当作真实完成证据
};

type PlannedOccurrence = {
  occurrenceId: string;
  templateId: string;
  localDate: string;
}; // 可选显式日历安排；未实际训练不生成 SessionLog

type TrainingPlanRevision = {
  revisionId: string;
  planId: string;
  parentRevisionId?: string;
  contentSnapshot: TrainingPlanContent;
  changeSet: PlanPatch[]; // 可验证的新增/删除/移动/剂量/频率/排期/引用变更
  sourceKind: "user_direct" | "user_accepted_proposal";
  sourceId: string;
  authorizationId: string;
  createdAt: string;
};

type PlanProposal = {
  proposalId: string;
  userId: string;
  kind: "create" | "update"; // 统一初次生成与已有计划调整
  initiatedBy: "user" | "agent";
  sourceId: string;
  planId?: string; // update 必填，create 不填
  basedOnRevisionId?: string; // update 必填
  proposedPlan?: { // create
    title: string;
    goal: string;
    relatedConditionIds: string[];
    relatedBodyRefs: TrainingPlan["relatedBodyRefs"];
    content: TrainingPlanContent;
  };
  proposedChanges?: PlanPatch[]; // update
  rationale: string;
  evidenceRefs: string[];
  contentFingerprint: string;
  state: "pending" | "accepted" | "rejected" | "expired";
  expiredReason?: "timeout" | "base_revision_changed" | "relevant_context_changed";
  createdAt: string;
  expiresAt: string;
  userDecisionId?: string;
};

type TrainingSessionLog = {
  sessionLogId: string; // 唯一执行实例；不是 (planId,date) 唯一
  userId: string;
  planId: string;
  planRevisionId: string; // 固定执行时接受的处方版本
  templateId: string;
  plannedOccurrenceId?: string;
  startedAt: string; // 时间点
  trainingDate: string; // startedAt 在 timeZoneSnapshot 中的本地日期
  timeZoneSnapshot: string; // IANA 时区；跨午夜不拆分记录
  state: "in_progress" | "paused" | "ended";
  pauseReason?: "user" | "safety_hold" | "recovery";
  endedAt?: string;
  endedReason?: "finished" | "stopped_early" | "safety_stopped";
  actions: SessionAction[]; // 追加式真实操作和更正，不是计划内容
  feedback?: { text?: string; recordedAt: string; bodyStateSourceIds?: string[] };
};

type SessionAction = {
  actionId: string;
  planExerciseId: string;
  setNumber?: number; // scope=set 时必填
  scope: "set" | "exercise";
  kind: "completed" | "skipped" | "corrected";
  actualRepetitions?: number;
  actualDurationSeconds?: number;
  actualDistanceMeters?: number;
  correctsActionId?: string; // 撤销/更正以追加事件呈现
  recordedAt: string;
  sourceId: string; // 用户明确操作或本人陈述
};
~~~

**字段校验**：运动目标必须有有效数值、组数为正；`PlanExercise` 的同名实例必须有不同 `planExerciseId`。同一动作概念可在不同计划、课模板或同一模板出现多次；替换动作产生新的 planExerciseId，调整组次/剂量保留原条目身份。物理数据库字段、单位校验和修订序列化由工程契约收敛，不推翻上述逻辑身份。

## 4.2 计划创建、状态与修订（TP-D1～D5）

| 场景 | 唯一有效行为 | 审计与授权 |
| --- | --- | --- |
| 用户要求「帮我制定一套计划」 | Agent 生成 **Draft=未接受的计划预览**，保存为 `PlanProposal(kind=create)` 或关联的待回复草案；用户明确「使用/保存此计划」后才创建 `TrainingPlan` 首个已接受修订 | 生成、浏览 Draft 不是激活计划；无授权不进入已生效计划列表 |
| 用户明确指令「把 A 计划深蹲换成 B」 | 目标与安全条件明确时，按原指令直接提交**指定计划**的变更；有歧义、医学风险或重大影响先澄清/确认 | `sourceKind=user_direct`、精确目标、sourceId、授权和修订留痕 |
| Agent 主动认为应该改计划 | 只读影响检查 → 精确差异与理由 → `PlanProposal(kind=update)` → 用户接受后提交 | 否决/关闭/未答绝不改变计划；关闭保持 pending |
| 提案接受 | 检查 userId、proposalId、内容指纹、底版修订、相关 BodyState/Diagnosis/Safety 新变化和幂等键；仅准确授权内容一次提交 | 竞争/重大新证据使提案 expired，重新生成并要求新授权；不可静默套用旧决定 |
| 提案超时 | 训练提案创建后 **7×24 小时**未决定则 expired；底版或关联风险发生实质变化可提前失效 | Diagnosis 提案期限独立，不由此修改其生命周期 |
| Plan active → paused → active | 用户显式暂停/恢复计划；保留计划 ID 和内容修订 | 每次状态变化独立记审计，不构成内容修订 |
| Plan active/paused → completed | 用户显式结束计划；不能凭训练次数/自然日期自动完成 | 保留全部执行历史 |
| Plan completed → active | 用户明确重新启用同一计划，沿用 planId；重开不修改历史课次 | 新状态事件；安全限制依旧生效，不暗改处方 |

- 允许**多份计划同时 active**；仅 Trainer 正在执行的实际 Session 需要并发互斥。
- TrainingPlan.status 没有 `review_recommended`；新证据、训练反馈只能产生**去重的只读复核提示**，不能自动暂停/完成/改写计划。SafetyState 可在状态仍 active 时阻断执行。
- `TrainingPlanRevision` 内容快照不可变。用户修改处方产生新修订和精确差异；撤销处方通过**新的修订**完成，不能篡改既有历史。
- 修改历史已经执行的课模板不能追写到旧 SessionLog；未来未执行排期按新修订重算/展示，已经开始的 Session 固定旧 revision。
- `PlanPatch` 最小逻辑操作：add/remove/move template、add/remove/replace/move exercise、update target/rest/notes、update goal/frequency/schedule/constraints/relations；patch 用稳定 ID 定位路径，服务端验证前置版本，跨计划不能引用另一个计划的条目 ID。

## 4.3 排期与训练日（TP-D6）

- 默认 `frequency_only`：例如每周 3 次只是目标频率；未选择具体日期时不创建计划日，不产生「缺练/漏练/补练」事实。
- 用户明确选择日历排期才有 `calendar` 与 PlannedOccurrence；每个安排显式指定课模板、日期和时区。到了日期未执行只能显示**待执行/过期的计划安排**，不能伪造 `skipped`、训练失败或疾病变化；补排需用户授权。
- `trainingDate` 固定为开始时在用户 IANA 时区下的本地日期；跨午夜保持同一执行实例与训练日，时区变更不重写历史。多次同日训练允许，使用不同 `sessionLogId`。
- 执行数据按 sessionLogId 查询，也可按用户本地日期/计划/模板检索；不能将 (planId,date,exerciseId) 当执行记录唯一键。

## 4.4 Trainer 执行、真实完成与反馈（TP-D7～D10）

| 事件 | 允许的状态/动作 | 严禁 |
| --- | --- | --- |
| start(planId,templateId) | 创建一个绑定当前已接受修订的 SessionLog；一位用户最多一个 `in_progress`；其他已暂停 Session 可以恢复 | 无安全检查启动受限计划/动作；把打开预览当开始 |
| pause / resume | `in_progress ↔ paused`；暂停计时不累计，恢复到原动作/组；恢复前重查安全门禁 | 断网或退出窗口隐式「完成训练」 |
| 组完成 | 由用户明确操作/陈述追加 `scope=set, kind=completed`；实际次数/时长可与目标不同 | 通过下一组/下一项/播放示范自动推断完成 |
| 跳过一组/整个动作 | 用户明确选择对应作用范围，记录 `skipped`；跳过动作不能伪造每组已经做完/逐组跳过的事实 | 以导航状态推断跳过 |
| 误操作更正 | 追加 `corrected` 指向原 actionId，投影使用最新有效动作；保留审计 | 就地删除原完成/跳过记录 |
| finish / 提前结束 | 用户明确结束；`endedReason=finished` 或 `stopped_early`；完成比例从有效逐组动作派生，可能是部分完成 | 到达最后一个动作自动把所有组标完成 |
| 训练中风险 | 按 SafetyState 门禁策略安全暂停或结束，保留已记录完成事实；不批量标跳过 | 以危险为由静默修改计划处方或抹除历史 |
| 训练反馈 | 原样保存主观反馈；明确本人身体症状/变化经 BodyState 另行持久化并回传 sourceIds | 直接把「练后不舒服」写成临床确诊、疗效或计划自动调整 |

- 断线、关闭窗口或 Agent 中断不是业务结束。恢复时保留 sessionLogId 和已有动作；不可确认的离线训练时间不算作已完成/有效计时，恢复 UI 以保守暂停状态要求用户继续确认。
- 并发终端按单一 Session 与 expectedRevision / idempotencyKey 序列化，重复完成/提交只生效一次；同日重新训练从新的 sessionLogId 开始。
- 开始/恢复/进入下一动作及安全限制变更后均需重新核查匹配门禁；仅 `advisory` 可提示后继续，其余具体见 [SafetyState](./safety.md)。
- Trainer 的 `Today / Move / Feedback / Paused / Completed` 是**执行页展示状态**；`Draft` 只指未被接受的**计划预览**，不属于 TrainingSessionLog 或 TrainingPlan.status。

## 4.5 跨领域交互（TP-D11～D12）

- BodyState/Diagnosis 变化 → 找出相关计划、展示去重复核建议 → Agent 可以生成具体 PlanProposal → 用户接受才修订。用户拒绝后即使诊断未变也不能偷改计划；存在新的相关证据才允许重新提案。
- SafetyState 变化 → 立即影响开始/继续执行门禁，**不等待用户同意处方修订**；是否暂停训练和是否更新处方是两项不同权限。已有风险限制不阻止用户保存计划本身，但必须阻止受限执行；草案形成后发生相关重大风险变化则旧提案失效、需重新审阅。
- 计划关联多个医学 ConditionEntry 是上下文关联，不暗示某一个诊断必然导致动作禁忌或训练有效。Action、3D Demo 和训练时间先后不证明病因/疗效。
- 同步修订与安全门禁由 Go 服务最终核验；Agent 只能建议和调工具，不能自主「接受建议」、伪造完成、降低风险或越过用户授权。

## 4.6 旧实现映射、工程待办（不再作为业务待决）

| 旧实现/差异 | 迁移边界 |
| --- | --- |
| `TrainingPlan` 旧字段 ConsultationID、TreatmentID/RevisionID、Phases(JSON) | 对齐独立计划、sessions[]、templateId 和 TrainingPlanRevision；不可把旧 Treatment 的暂停、复核状态原样映射到计划状态 |
| `TrainingLog` 的 `is_checked_in`、按日期 Exercises(JSON) | 只有历史明确记载完成/跳过才能迁入动作事实；不可把打卡布尔值推导为所有组完成；其余保留为 legacy_summary/source |
| `TrainingExecutionPanel` 用动作名称作为状态键、末项下一步批量置 completed | 改按 planExerciseId+setNumber+sessionLogId，确认操作才新增 Action；回归 IG-01/TP-12 |
| `training_service.go` 使用服务器 localDate(time.Now()) | 改用用户时区快照与 startedAt，兼容旧日期读取、按新实例 ID 写入；回归 IG-02/TP-13 |
| 旧 `Treatment.review_recommended/paused` 与反馈触发状态重算 | 复核建议是投影，Safety 是独立门禁；迁移时绝不无授权修改计划内容或状态；回归 IG-03 |
| `PlanChangeProposal` 旧名称和 DTO | 新领域以 `PlanProposal(create/update)` 统一；迁移时保留准确授权/历史映射，不将未确认草案转成有效计划 |

**实施门槛**：字段与状态的逻辑语义已决定，但物理 Schema、旧数据映射、迁移策略、计时精度、动作库来源、医学动作强度与禁忌的有效性仍需要工程实现和测试；本轮没有运行/修改业务代码。
