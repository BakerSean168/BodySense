# BodySense · TrainingPlan 与 Trainer 领域契约

## 4. TrainingPlan

~~~ts
type TrainingPlan = {
  planId: string;
  userId: string;
  title: string;
  goal: string;
  relatedConditionIds: string[];
  relatedConcernIds: string[];
  status: "active" | "paused" | "completed";   // 计划自身状态；安全限制不改写它
  currentAcceptedRevisionId: string;
  currentContent: {
    sessions: PlanSessionTemplate[];  // 单次课计划只有一个模板；A/B 日等多课计划有多个
    frequency: { sessionsPerWeek?: number; description?: string };
    constraints: string[];
  };
};

type PlanSessionTemplate = {          // 计划中「一次课」的模板，不是实际执行
  templateId: string;
  title?: string;                     // 如「A 日 · 下肢」
  stages: {
    phase: "warmup" | "main" | "cooldown";
    exercises: {
      exerciseId: string;
      name: string;
      sets: number;
      repetitions?: number;
      durationSeconds?: number;
      restSeconds?: number;
    }[];
  }[];
};

type TrainingSessionLog = {           // 一次实际执行的训练记录（Trainer 产生）
  sessionLogId: string;
  planId: string;
  planRevisionId: string;
  templateId: string;
  date: string;                       // 用户时区的训练日期
  startedAt: string;
  endedAt?: string;
  state: "in_progress" | "paused" | "ended";
  exerciseRecords: {
    exerciseId: string;
    setNumber: number;
    outcome: "completed" | "skipped";
    actualRepetitions?: number;
    actualDurationSeconds?: number;
    recordedAt: string;               // 来自用户的明确完成 / 跳过操作或陈述
  }[];
  feedback?: string;                  // 主观反馈；身体变化另按 R-BS-01 写入 BodyState
};

type PlanChangeProposal = {
  proposalId: string;
  planId: string;
  basedOnRevisionId: string;
  initiatedBy: "user" | "agent";
  reason: string;
  proposedChanges: PlanPatch[];
  state: "pending" | "accepted" | "rejected" | "expired";
  expiredReason?: "timeout" | "base_revision_changed";
  userAuthorizationId?: string;
};
~~~

- 每份 Plan 独立修订；每个动作与训练记录关联明确的 planId（R-TP-01）。
- `currentContent` 必须能直接呈现 R-TP-02 要求的完整处方：每个 PlanSessionTemplate 按热身 → 正式训练 → 必要放松排列，逐个动作名称、组数、次数/时长、必要休息，加上整体训练频率。
- PlanSessionTemplate 是计划内容；TrainingSessionLog 是执行事实，二者不混用。执行记录不回写模板。
- TrainingSessionLog 按 planId、日期、exerciseId 定位；只有用户明确的完成 / 跳过操作或陈述才形成 `exerciseRecords`，不根据点击、浏览或演示播放推断完成（R-EX-04）。
- 同一份计划的动作/组次/频率/排程只有一个当前有效权威副本。
- 用户明确请求的修改（R-TP-03）直接提交并保留该请求的授权来源。
- Agent 建议（R-TP-04）仅创建 Proposal；接受时检查对应方案、用户身份、底版修订和决定状态，才允许提交。Proposal 状态：
  - 关闭界面或离开对话：保持 `pending`，可恢复；
  - 用户明确选择「暂不修改」：`rejected`；
  - 超时或计划底版修订已变化：`expired`（超时时长见 [Q-17](../open-questions.md)）。
  只有 `accepted` 产生内容修订。
- 安全执行限制由 BodyState `SafetyState` 持有，不等同于修改计划内容（R-SAFE-02、R-SAFE-03）。
- TrainingSessionLog 和身体变化回写只触发影响检查，不直接修改计划（R-TP-05）。

### 4.1 已定属性与缺少的结构

| 对象 | 已有逻辑属性 | 当前尚未充分确定 |
| --- | --- | --- |
| TrainingPlan | planId、userId、title、goal、relatedConditionIds、relatedConcernIds、status、currentAcceptedRevisionId、currentContent | status 转换授权、排期与完整 Revision Schema（DC-010/012） |
| CurrentPlanContent | sessions[]、frequency、constraints | 频率不等于日历排期，不能自动推断漏练补练 |
| PlanSessionTemplate | templateId、title、stages(warmup/main/cooldown)、每个动作 exerciseId/name/sets/repetitions/duration/rest | 同名动作与修订身份规则 DC-011 |
| TrainingSessionLog | sessionLogId、planId、planRevisionId、templateId、date、startedAt、endedAt、state、exerciseRecords、feedback | 提前终止/跳过粒度、跨午夜归属 DC-013/022 |
| PlanChangeProposal | proposalId、planId、basedOnRevisionId、initiatedBy、reason、proposedChanges、state、expiredReason、userAuthorizationId | PlanPatch 具体 DTO 未获最终确认 |

### 4.2 已定状态和合法变化

| 对象 | 状态与触发 | 不可越过的边界 |
| --- | --- | --- |
| TrainingPlan | active / paused / completed | 初始、暂停、恢复、重新激活的状态转移与审计尚需 DC-012；安全限制不能自动修改此字段 |
| TrainingSessionLog | in_progress → paused → in_progress；进行后可 ended | 结束不自动等于全部动作完成；结束原因尚待 DC-013 |
| exerciseRecords | completed / skipped | 只在用户明确完成或跳过时记录；不是浏览、演示或「下一项」自动完成 |
| PlanChangeProposal | pending → accepted / rejected / expired | 只接受被准确授权的提案才更新当前计划；关闭窗口保持 pending，可恢复 |
| SafetyRestriction | 独立作用于 Trainer 的开始/继续 | 不修改计划内容；新风险出现时必须重新检查门禁 |

训练日志始终关联执行时的 planRevisionId 和 templateId，不得用后来的模板覆盖旧训练事实。date 按**用户时区的训练日期**，startedAt/endedAt 表达时间点；跨午夜和用户切换时区规则仍需 DC-022。

### 4.3 事件、权限、UI 与验收

可追踪事件包括计划修改、提案决定、课程开始、组完成、跳过、暂停、继续、结束和用户反馈。每次写入必须定位 planId、课程模板与目标动作/组，检查用户、底版和幂等键。保存日志不能静默改写其他计划。

**UI 状态**：计划 active/paused/completed；执行 in_progress/paused/ended；动作/组真实完成与跳过；提案 pending/accepted/rejected/expired；安全暂不宜执行；Penpot Draft 仍为 Q-15/DC-014 待决。验收沿用 TP-01～11，新增 TP-12～15；不为待决的漏练/补练和跳过粒度预先设定行为。

### 4.4 当前模型字段与新领域的映射（As-Is）

| 当前代码 | 持久结构 / 状态 | 与目标不一致或未定 |
| --- | --- | --- |
| [TrainingPlan](../../../apps/api/internal/model/training_plan.go) | ID、UserID、ConsultationID、TreatmentID/RevisionID、Status、Goal、DurationWeeks、CurrentWeek、Phases(JSON)、CreatedAt | 以旧单 Treatment 为中心，缺新规格的 `sessions[]`/稳定动作身份/独立 `TrainingPlanRevision`；不能直接用 `Phases` 当新课模板（DC-052/053） |
| [TrainingLog](../../../apps/api/internal/model/training_plan.go) | ID、UserID、PlanID、TreatmentRevisionID、InterventionID、Date、Exercises(JSON)、Notes、IsCheckedIn、OutcomeRecordedAt、CreatedAt | 旧 `is_checked_in` 与「一组已完成」不是同一事实；缺新 sessionLog state、模板身份、明确结束原因（DC-013/054） |
| [Treatment](../../../apps/api/internal/model/treatment.go) | 状态 active/review_recommended/paused/superseded/completed；Revision 为 proposed/accepted/rejected | `review_recommended`/`paused` 不能简单映射为新 TrainingPlan.status；治疗方案暂停可能源于安全或复核（DC-055） |
| [TrainingExecutionPanel](../../../apps/web/src/features/training/components/TrainingExecutionPanel.tsx) | exerciseStates 以动作名称作键，最后一次「下一项」把所有项置为 true | 违反 INV-26 的显式完成原则；还存在同名动作碰撞风险，待独立修复（IG-01） |
| [TrainingService](../../../apps/api/internal/service/training_service.go) | `localDate(time.Now())` 按服务端进程时间获取 Date；反馈会调用 Treatment.RecordOutcome | 与用户时区日期目标不符；状态变化需与影响检查分离（IG-02/03） |

### 4.5 尚缺的状态转换规格

| 业务事件 | 已定后果 | 待产品决定 |
| --- | --- | --- |
| 开始新的课模板 | 产生关联 planId、revisionId、templateId 的执行日志 | 是否允许同一日多次、并行会话、如何合并（DC-056） |
| pause / resume | 暂停不改变计划内容，继续回到正确动作/组次 | 时钟是否累计、跨设备恢复、超时规则（DC-057） |
| 下一组 / 下一动作 | 导航不制造完成记录 | 跳过粒度与撤销误操作（DC-013/058） |
| ended | 结束本次训练，保留明细 | 完成、主动提前结束、系统中断是否独立 `outcome`（DC-059） |
| 训练计划重新激活 | 计划身份、修订和历史保留 | complete/paused 的合法重启与多计划竞争（DC-012/060） |
| 频率 / 课模板变化 | 仅在精确授权后改变接受的计划版本 | 未来排期与既有执行日志的关联方式（DC-010/061） |
