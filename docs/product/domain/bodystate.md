# BodySense · BodyState 领域契约

## 2. BodyState

~~~ts
type BodyState = {
  userId: string;
  currentRevision: number;       // 内部字段
  currentFacts: BodyFact[];
  observations: BodyObservation[];
  hypotheses: BodyHypothesis[];
  concernGroups: DerivedConcernGroup[];  // 组织投影，不单独持有身体事实
  safetyState: SafetyState;
};

type BodyFact = {
  id: string;
  statement: string;
  bodyRegionIds: string[];
  subject: "self";
  kind: "symptom" | "health_history" | "activity" | "lifestyle" | "training_feedback";
  source: { kind: "user_message" | "direct_edit" | "structured_answer"; id: string };
  validFrom?: string;
  validTo?: string;
  observedAt?: string;
  recordedAt: string;
  timePrecision: "exact" | "day" | "range" | "approximate" | "unknown";
  lifecycle: "active" | "resolved" | "historical";
  correctedFactId?: string;
  supersedesFactId?: string;
};

type BodyHypothesis = {          // 只保存用户本人提出的怀疑（R-BS-06）
  id: string;
  statement: string;             // 如「我怀疑是偏头痛」
  source: { kind: "user"; messageId: string };
  relatedFactIds: string[];
  bodyRegionIds: string[];
  recordedAt: string;
};
~~~

> SafetyState 是 BodyState 的组成部分，完整限制与解除契约见 [safety.md](./safety.md)。

### 2.1 写入语义与证据分类

满足 R-BS-02 的用户自述，经过结构验证后直接保存，无需事前人工批准。系统必须核查来源消息、用户身份、主体归属、时间语义、字段可解释性及幂等标识。

| 分类 | 含义 | 权威边界 |
| --- | --- | --- |
| 用户自述（BodyFact） | 用户明确报告的现象、健康史或生活习惯 | 自动记录不等于临床客观核实 |
| Observation | 有方法及来源的检查、自测、测量或报告观察；报告识别值只有按 [证据管线](../../architecture/health-document-evidence-pipeline.md) 规则采纳后才形成 Observation（R-PS-03） | 可查看方法与来源；未核实 / 低于阈值的值不是可信临床证据 |
| Hypothesis（BodyHypothesis） | 用户本人提出的病因怀疑或可能疾病，`source = user`，自动记录 | 不具备确诊权威；AI 提出的假设只存在于 ConditionAnalysisRun / ConditionProposal 审计，不写入 BodyState |
| Evidence | 支持或反驳假设的文献、报告、测量等 | 必须有可追溯来源 |

存在不可消除的关键歧义时，只对相关字段创建 Clarification；同一条消息的其他明确内容可以先提交（R-BS-03）。

### 2.2 时间与更正操作

| 操作 | 当前投影 | 审计和时间线 |
| --- | --- | --- |
| Add | 新增事实 | 记录来源与适用时间 |
| TemporalChange | 更新有效状态 | 保留旧状态的真实时间区间 |
| Correct | 更正错误记录 | 保留纠错痕迹，错误数据不当病程 |
| Duplicate | 无实质改变或增加证据 | 不能制造多份相同事件 |
| Clarify | 暂不写入模糊字段 | 记录待回答的问题、目标和来源 |

- 至少区分身体状态有效时间、观察时间、记录时间、原始证据形成时间。描述「最近」「去年」「一周前」时保留原始精度并使用用户时区；不能为了填充字段而编造日期（R-BS-05）。
- 一个语义完整的事务可以增删改多个 BodyFact，但只形成一次 BodyStateRevision。

### 2.3 属性完整性、来源和未决字段

| 对象 | 已定义的属性与保证 | 仍缺少的正式逻辑 Schema |
| --- | --- | --- |
| BodyState | userId / currentRevision / currentFacts / observations / hypotheses / concernGroups / safetyState | 单次事务的 BodyStateRevision DTO |
| BodyFact | id、statement、bodyRegionIds、subject、kind、source、validFrom/validTo、observedAt、recordedAt、timePrecision、lifecycle、纠错关系 ID | historical 与 resolved 的严格转换边界（DC-019） |
| BodyHypothesis | id、statement、source=user、relatedFactIds、bodyRegionIds、recordedAt | 纠正、反证、撤回和历史状态（DC-020） |
| BodyObservation | 有来源、方法、形成/观察时间、可追溯的证据准入依据；OCR 未准入候选不能作为可信观察 | 原始值/单位与规范化结构、审计 DTO 尚未定稿；不得直接复制旧 Assessment Schema |
| DerivedConcernGroup | 派生关联事实和观察的组织视图，不独立存储身体真值 | 身份、合并/拆分规则及缓存失效方式 |

上述未定义类型并非伪装成已经可编译的完整契约。实现前需先确定字段所有权，避免事实在 Person、Chat 与 BodyState 多处写入。

### 2.4 已定状态转移、触发与权限

| 状态/操作 | 触发与合法变化 | 授权、审计、副作用 |
| --- | --- | --- |
| 新建 BodyFact | 明确描述自己的现象/习惯或历史 | Agent 提取后由 Go 校验主体、时间、来源与幂等；无须事前逐条确认 |
| active → resolved | 用户表述同一真实病程已经缓解 | TemporalChange；保留历史区间，不当作纠错 |
| 事实纠错 | 用户澄清此前录入有误（含左右侧） | Correct；记录纠错关系，禁止伪造曾真实发生的症状转移 |
| historical | 已有 lifecycle 枚举 | 与 resolved/active 的转换尚未定稿（DC-019），不能凭代码习惯猜测 |
| 新建 BodyHypothesis | 用户本人明确提出怀疑 | source=user；AI 自己提出的假设只能存于分析审计 |
| BodyHypothesis 撤回 | 用户撤回自己的猜测 | 生命周期待决 DC-020；不可删除来源和审计 |
| 证据候选 → BodyObservation | 证据管线自动准入或用户核实并满足准入门槛 | 未准入值留在报告候选；不能写为医生诊断 |
| ConcernGroups 变化 | 相关事实、观察变化 | 仅重算组织投影，不制造新事实 |

有效时间、观察时间、记录时间、证据产生时间应分别保留；描述模糊时仅阻断歧义字段，其他明确项可同事务写入。一个完整的语义事务只产生一个 BodyState 修订。

### 2.5 UI 状态和验收

记录应支持：当前/已缓解/历史、用户提出的怀疑、事实更正、待澄清、来源证据可追溯、OCR 待核实、保存成功和保存失败。产品默认不展示内部修订号。验收沿用 BS-01～11，新增 BS-12～15；对于 DC-019/020 未定转换不写伪造的通过条件。
