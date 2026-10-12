# BodySense · 验收规范与实施顺序

**本文件是测试矩阵。** 每个用例引用其所验证的规则编号（`R-*` / `G-*` 见 [产品需求](./requirements.md)，`INV-*` 见 [领域模型 §6](./domain.md#6-运行时不变量)）；规则本身不在此重复。测试项是否通过，以真实实现后的单元、集成、端到端执行结果判定。

## 1. 业务验收矩阵

| ID | 用户输入 / 条件 | 必须满足的结果 | 规则 |
| --- | --- | --- | --- |
| BS-01 | 「我今天左肩痛」 | 无逐条确认直接保存；归属、左侧和时间正确 | R-BS-01、R-BS-02 |
| BS-02 | 「朋友今天左肩痛」 | 不加入用户本人 BodyState | R-GEN-01 |
| BS-03 | 「偏头痛是什么？」 | 解释知识，不新增本人疾病记录 | R-GEN-01 |
| BS-04 | 「我怀疑自己有偏头痛」 | 自动保存为 BodyHypothesis（`source = user`），不按临床确诊保存，不进入 Diagnosis | R-BS-06、INV-02 |
| BS-04b | Agent 在 Chat 中提出「可能是颈源性头痛」 | 不写入 BodyState；只出现在 ConditionAnalysisRun / 提案中 | R-BS-06、INV-02 |
| BS-05 | 「医生告诉我患偏头痛」 | BodyState 记录用户转述的健康史；若之后纳入 Diagnosis，`source = user_reported_clinical`，不显示为有文件支持 | R-DX-08、INV-11 |
| BS-06 | 「去年左肩痛，现在好了」 | 历史发生与当前恢复并存，不误判矛盾 | R-BS-04 |
| BS-07 | 「我之前左右记反了」 | 纠正错误记录，不生成虚构病程 | R-BS-04、INV-03 |
| BS-08 | 同时间同条件互斥记录 | ask_user 指出具体日期和条件；不覆盖不确定部分 | R-BS-04 |
| BS-09 | 同一条消息含多项明确事实和一处模糊 | 明确部分先保存，模糊部分独立待澄清 | R-BS-03 |
| BS-10 | 同一次写入重复提交 | 不出现重复 Fact 或重复 Revision | INV-09 |
| BS-11 | 用户在 Person 的 Reports 上传化验单 | 资料归入 Reports；可靠值按证据管线规则采纳为 BodyObservation，存疑值标为待核实；未核实值不作为可信临床证据；不自动产生或修改任何 Diagnosis 条目 | R-PS-03 |
| DX-01 | 用户仅闲聊症状 | 可以初步解释，不未经请求发布正式诊断 | R-DX-06、R-DX-07 |
| DX-02 | 用户请求重新分析病情 B，另有 A、C | B 改变，A/C 保持身份和内容，仍是一份 Diagnosis | R-DX-01、INV-04 |
| DX-03 | 一次分析对 A、B 各产生一个提案，用户分别决定 | 两个决定都完成后一次原子提交，整体修订只增加一次 | R-DX-01、R-DX-12、INV-05、INV-12 |
| DX-03b | 同一运行中 A 已决定、B 未决定，用户关闭 Chat | Diagnosis 不变；A 的决定已留档；B 保持 pending | R-DX-12、INV-12 |
| DX-03c | 接 DX-03b，用户重新进入对话 | 同一运行恢复 B；A 无需重选；B 决定后 A、B 一次提交 | R-DX-12 |
| DX-03d | 用户对 B 选择「继续补充」并提供新信息，新信息与已决定的 A 无关 | 新信息写入 BodyState 并作为本运行新证据；修订后的 B 作为新提案（旧提案 superseded）；A 经复核记为 unaffected，原决定照常生效；提交前 Diagnosis 不变 | R-DX-04、R-DX-12、INV-17 |
| DX-03e | 运行超时或用户说「其余先不处理」 | 未决定提案 expired；已作出的生效决定一次提交；无生效决定时不产生修订 | R-DX-12、INV-12 |
| DX-03f | 提交前 Diagnosis 已被另一事务修改 | 补丁被拒；受影响提案 expired 并重新核验展示，不静默覆盖 | R-DX-12、INV-09 |
| DX-03g | 接 DX-03d，但补充的新信息与已决定的 A 相关（如构成 A 的反证） | A 复核为 affected，记为 superseded（`evidence_changed`），原决定留档但不生效；同一运行重新提出 A 并引用新证据；用户重新决定前不提交；最终仍一次提交 | R-DX-12、INV-12、INV-17 |
| DX-03h | 提交时某个已决定提案缺少对其决定之后新证据 ID 的复核记录 | 补丁被拒，不产生 Diagnosis 修订，直到复核完成 | R-DX-12、INV-17 |
| DX-04 | 正式分析证据不足，或在产生提案前识别到需停止分析的严重风险 | 不制造确定性诊断，返回不足/阻断结果；不创建提案，运行以 `insufficient_evidence` / `safety_blocked` 事件结束（`state = closed_without_change`），不产生 Diagnosis 修订 | R-DX-10、INV-22 |
| DX-04b | 运行中 A 已决定、B 未决定时，识别到警示 / 需尽快就医类风险 | 立即显示医学安全提示，停止后续确认；B `expired`（`safety_blocked`）；A 的决定留档但不生效；`outcome = safety_blocked`、`state = closed_without_change`；无 DiagnosisRevision；运行不自动恢复，用户之后重新发起分析时创建新运行、提案须重新决定 | R-DX-10、R-SAFE-01、INV-23 |
| DX-04c | 运行中全部提案已决定（复核完成）但尚未提交时被安全阻断 | 不提交、无部分提交，Diagnosis 不变；全部决定留档但不生效；`outcome = safety_blocked` | R-DX-10、R-DX-12、INV-23 |
| DX-04d | 同一运行重复请求 `safety_blocked`（网络重试或 Agent 重复调用） | 只有一个 `safety_blocked` 事件，重复请求返回相同结果；不追加事件、不产生修订 | INV-09、INV-23 |
| DX-04e | 同一运行的提交与 `safety_blocked` 并发 | 只写入一个终结事件；阻断先写入时提交被拒、无 DiagnosisRevision；提交先写入时阻断被拒、修订保留，安全提示照常展示 | INV-05、INV-09、INV-23 |
| DX-04f | 运行已被安全阻断后，Agent 或客户端仍调用 apply_diagnosis_patch、记录决定或追加事件 | Go 后端拒绝，Diagnosis 不变，不依赖 Agent 提示是否遵守 | INV-23 |
| DX-04g | 分析进行中出现只影响训练执行的普通 SafetyState 限制 | 不中断正式分析，确认与提交照常；限制只由 Trainer 把关 | R-DX-10、R-SAFE-03、INV-08 |
| DX-05 | 页面查看当前诊断 | 只显示用户选择过的病情（大概率是 / 疑似保留），每条有完整名称/原因/处理方案和可读的状态标签；疑似条目列出待明确信息；不出现修订号/版本切换器 | R-DX-05、R-DX-09 |
| DX-06 | AI 分析出可能病情，用户尚未选择 | 只保留在 Chat 的病情提案中；不得混进 Diagnosis 列表 | R-DX-05 |
| DX-07 | 用户在确认工具中选「大概率是」 | 条目 `source = agent_analysis`、`conclusionStatus = likely`、`confirmation.decision = likely`；界面不称其为临床诊断 | R-DX-04、R-DX-08 |
| DX-07b | 用户选「可能是，但不确定，先保留」 | 条目 `conclusionStatus = suspected`、`decision = keep_as_suspected`，含待明确信息；窗口标为疑似 | R-DX-04 |
| DX-07c | 用户对新提案选「不是这个」 | 不创建条目；ConditionProposalDecision 记录 proposalId、内容指纹、证据 ID、`rejected` 与时间；相关 BodyState 症状事实保留 | R-DX-04、INV-13 |
| DX-07d | 用户对已有条目的更新提案选「不是这个」 | 只拒绝本次变更：条目内容、结论、`lifecycle`、`disposition` 均不变，仍在当前列表；决定留档；该决定本身不产生 Diagnosis 修订 | R-DX-04、INV-18 |
| DX-07e | 拒绝后再次分析，无相关新证据 ID（仅 BodyState 修订号变化） | 不再提出相同指纹的提案 | R-DX-04、INV-13 |
| DX-07f | 拒绝后出现与该提案相关的新证据 ID | 可再次提出，提案引用该新证据 | R-DX-04、INV-13 |
| DX-07g | 用户对已有条目的结束提案（`changeKind = end`，任一 `endTarget`）选「不是这个 / 不同意」 | 决定为 `rejected`；条目内容、`lifecycle` 与 `disposition` 均不变，仍在当前列表；决定留档 | R-DX-04、INV-18 |
| DX-07h | 结束提案 `endTarget = mark_resolved`，用户选「同意」 | 决定为 `accept_end`；提交后 `lifecycle = resolved`，`disposition`、`conclusionStatus`、`confirmation` 不变；一次 Diagnosis 修订 | R-DX-01、R-DX-12、INV-21 |
| DX-07i | 结束提案 `endTarget = mark_no_longer_applicable`，用户选「同意」 | 决定为 `accept_end`；提交后 `disposition.state = no_longer_applicable`，`reason` 为提案 `endReason`，`decisionId` 指向该决定；`lifecycle` 不变；条目与历史保留，不作为当前病情显示 | R-DX-01、INV-14、INV-21 |
| DX-07j | 同一条目上只有 `endTarget` 不同的两个结束提案；或对结束提案提交 `likely` 等非 `accept_end` 决定 | 两个提案 `contentFingerprint` 不同，对其一的决定不能用于另一个，提交只应用被批准的 `endTarget`；对结束提案的非 `accept_end` / `rejected` 决定被后端拒绝 | INV-13、INV-21 |
| DX-08 | 单个提案的运行中，用户选择继续补充或关闭界面 | Diagnosis 无内容修订；提案保持 pending；相关 BodyState 事实不丢失 | R-DX-04、INV-06 |
| DX-09 | 用户转述医生诊断的病情提案，用户选「纳入我的病情管理」 | `conclusionStatus = clinical_diagnosis`，`source` 仍为 `user_reported_clinical`；不变为 `clinical_document` | R-DX-08、INV-11 |
| DX-10 | 疑似条目后来有经管线采纳、明确记载该诊断的临床文件支持 | 新提案把同一 conditionId 升级为 `clinical_document`，用户选择 `include_clinical` 后生效；旧来源可追溯 | R-DX-08 |
| DX-11 | 证据只能描述现象、原因不明、只建议观察或就医 | 允许描述性名称、「原因不明 / 证据不足」与观察/检查/就医的处理方案；不编造病名、病因或训练处方 | R-DX-11 |
| DX-12 | 已采纳的化验 / 影像数值（如血尿酸升高、影像测量值）支持某个病情假设，文件本身未写明医生诊断 | 提案 `source = agent_analysis`，只能决定为 `likely` / `keep_as_suspected`；不得出现 `clinical_document` 或 `clinical_diagnosis`；数值列为提案证据 | R-DX-08、INV-11、INV-15 |
| DX-13 | 运行从 `open` 变为 `committed` / `closed_without_change`，或其中提案被取代、过期 | 运行与提案的当前状态随之更新；已写入的审计事件、提案内容与决定记录不被修改或删除，可完整回放 | INV-16 |
| DX-13b | 丢弃 Run / Proposal 的投影，仅按顺序回放基础记录、决定记录与事件（覆盖：提交、无变更关闭、证据不足、提案前与提案后的安全阻断、取代、过期、复核为 affected、含 `hypothesis_added` 的运行） | 重建出的 `state`、`outcome`、`evidenceIds`、`hypotheses`、`proposalIds`、`committedDiagnosisRevision`、`settledAt`、提案 `state` / 过期与取代原因、决定是否生效与原投影完全一致 | INV-16、INV-22 |
| DX-13c | 运行开始后，Agent 因新证据形成新的 AI 假设 | 追加 `hypothesis_added`（内容、证据 ID、时间）；`agentHypotheses` 快照不变；投影 `hypotheses` = 快照 + 追加事件，回放可重现；假设不写入 BodyState，也不直接进入 Diagnosis | INV-02、INV-16、INV-22 |
| DX-14 | 「把肩峰撞击从诊断里去掉」，用户在轻量确认中选确认 | 不创建 ConditionAnalysisRun；条目 `disposition = withdrawn_by_user`，`decisionId` 指向 ConditionDirectEdit（谁、什么、何时、原因）；条目与历史保留；一次 Diagnosis 修订；当前列表不再显示 | R-DX-13、INV-14、INV-19 |
| DX-14b | 「右肩已经好了」，用户确认 | 不创建分析运行；条目 `lifecycle = resolved`，`disposition` 不变；BodyState 记录该身体事实；一次 Diagnosis 修订 | R-DX-13、R-BS-01、INV-19 |
| DX-14c | 用户在轻量确认中取消或关闭 | 无 Diagnosis 修订 | R-DX-13 |
| DX-14d | Agent 未经用户明确请求调用直接编辑，或试图借更正改变 `source` / `conclusionStatus` | 后端拒绝；Diagnosis 不变 | R-DX-13、INV-11、INV-19 |
| DX-14e | 直接编辑提交时，另有一个进行中的分析运行 | 该运行提交时因底版变化被拒，受影响提案 expired 并重新核验 | R-DX-12、R-DX-13 |
| TP-01 | 用户明确说「修改臀腿计划的深蹲」 | 定位目标计划，只修改明确授权的内容 | R-TP-03 |
| TP-02 | 用户报告腿麻，但尚未授权计划修改 | 可以建议调整，所有 TrainingPlan 内容保持不变 | R-TP-04、R-TP-05 |
| TP-03 | 用户接受 Agent 指定提案 | 仅指定计划发生一次内容修订 | R-TP-04 |
| TP-04 | 用户选择「暂不修改」 | 提案 `rejected`；计划内容保持不变 | R-TP-04、INV-07 |
| TP-04b | 用户关闭提案界面后再回到 Chat | 提案仍 `pending`，可继续选择；期间计划内容不变 | R-TP-04、INV-07 |
| TP-04c | 提案超时，或计划底版修订已变化 | 提案 `expired`，接受操作被拒绝；计划内容不变 | R-TP-04、INV-07 |
| TP-05 | 计划提案接受请求网络重试 | 幂等，只产生一次修订 | INV-09 |
| TP-06 | 出现严重训练风险 | SafetyState 记录风险等级、范围、有效期、解除策略与解除条件；Trainer 阻止执行并显示「暂不宜执行」；`TrainingPlan.status` 与内容不变 | R-SAFE-01～03、INV-08 |
| TP-06b | 安全限制只作用于某个动作 | 只拦截该动作，同一计划其他动作可执行 | R-SAFE-03 |
| TP-06c | `liftPolicy = on_expiry` 的限制到达 `validUntil` | 自动解除，`liftBasis = expiry`；计划内容在整个过程中无修订 | R-SAFE-03、R-SAFE-04、INV-08、INV-20 |
| TP-06d | `liftPolicy = user_reverification` 的限制到达 `validUntil` | 提示用户复核，复核完成前仍拦截执行；用户经复核确认满足解除条件后解除，记录复核依据 | R-SAFE-04、INV-20 |
| TP-06e | 警示 / 建议就医类限制（默认 `professional_clearance`）到达 `validUntil` | 不自动解除，只提示复核；用户记录医生放行并给出来源后才解除；无来源时不解除 | R-SAFE-04、INV-20 |
| TP-06f | Agent 试图自行解除限制，或把 `liftPolicy` 放宽 | 后端拒绝，限制保持生效 | R-SAFE-04、INV-20 |
| TP-07 | 打开一份已接受训练计划 | 可见热身、正式训练及必要的放松，每个动作有名称、组数、次数/时长和整体频率 | R-TP-02 |
| TP-08 | 点击具体动作请求修改 | 定位到具体 Plan 与 exerciseId，其他动作和其他计划不被重建 | R-TP-01、R-TP-03 |
| TP-09 | 打开含 A/B 两个课模板的计划 | 两个 PlanSessionTemplate 各自显示热身/正式/放松；单课计划只有一个模板 | R-TP-02 |
| TP-10 | Trainer 中暂停、恢复、跳过一个动作 | 显示当前动作、组次与计时；恢复回到原位置；跳过记为 `skipped`；计划内容不变 | R-EX-03、R-TP-05 |
| TP-11 | 用户只浏览动作或播放演示，未点完成 | 不产生 `completed` 记录；记录按 planId、日期、exerciseId 可查 | R-EX-04 |
| UI-01 | 桌面关闭/收起浮动窗口 | 可查看持续存在的 3D 人体场景 | [interface §1](./interface.md#1-核心空间) |
| UI-02 | 人体点选一个部位 | 仅形成上下文，不自动记录异常/疼痛 | [interface §6](./interface.md#6-人体交互工具) |
| UI-03 | Chat → 人体工具 → 返回 Chat | 上下文与待回答交互保持一致 | [interface §1](./interface.md#1-核心空间) |
| UI-04 | Person 查看身体、器材、资料 | 五个分类标签页；身份在标题区与 Identity Popover；显示真实当前状态与恰当空态，不用演示数据 | R-PS-01、R-PS-04 |
| UI-05 | 移动端切换业务窗口 | 任务与选区可恢复，主要操作可触达 | [interface §7](./interface.md#7-平台与辅助功能) |
| UI-06 | 878px 等窄桌面窗口打开 Diagnosis | 标题、病情卡片及操作按钮都在可见范围内，不向右溢出 | [interface §7](./interface.md#7-平台与辅助功能) |
| UI-07 | Chat 展示病情确认工具 | 可以核对名称/原因/方案/来源/证据边界；三种选择与继续补充结果各不相同；多提案逐个决定；提交前不伪称已更新诊断 | R-DX-03、R-DX-04、R-DX-12 |
| SEC-01 | 使用别人的 planId / conditionId 写入 | 后端拒绝越权 | G-04 |
| SEC-02 | 两个并发分析或计划修改 | 以修订并发控制避免覆盖他人的最新变更 | INV-09 |

## 2. 实施工作顺序

| 阶段 | 交付物 | 验收重点 |
| --- | --- | --- |
| P1 领域契约 | BodyState 自动写入和时间/来源模型；Diagnosis 当前聚合；Plan 级授权模型 | 类型、权限、INV-01～23 的契约测试 |
| P2 自述记录 | Agent 提取 → Go 校验提交 → 自动反馈、澄清恢复；上传资料的证据采纳 | BS-01～11（含 BS-04b） |
| P3 Diagnosis | 用户请求触发分析；ConditionAnalysisRun / Proposal / Decision 审计；Chat 病情确认工具；ConditionEntry 原子补丁 | DX-01～14 |
| P4 TrainingPlan | 每份计划的课模板与阶段化动作、组次/频率、只读影响检查、用户授权、Trainer 执行记录与安全门禁 | TP-01～11 |
| P5 交互集成 | 3D 桌面、浮动 Chat、Person、Diagnosis 与 Trainer 的真实数据状态 | UI-01～07 |
| P6 稳定化 | 数据权限、并发、事件恢复、安全门禁及端到端回归 | SEC-01～02 与全矩阵 |

每阶段均以「真实输入 → Agent/工具 → 后端持久状态 → UI 读取」的端到端闭环作为完成定义。只通过类型检查或静态页面检查不足以宣称业务完成。

## 3. 独立审核点

- 对照业务文档核查 API 与数据库，不得假设模型草案已被实现。
- 使用真实/模拟用户案例验证相对时间、主体归属、因果表述和医学确定性。
- 检查一条消息中多次工具调用是否发生部分成功、重复、失败或取消。
- 验证未授权建议只存为 Proposal，不修改训练内容；检查所有写操作的用户授权与并发修订。
- 对照 Penpot 的实际组件与交互核验桌面/移动界面，不能只比较静态截图。
