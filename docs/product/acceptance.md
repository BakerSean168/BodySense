# BodySense · 验收规范与实施顺序

**本文件是测试矩阵。** 每个用例引用其所验证的规则编号（`R-*` / `G-*` 见 [产品需求](./requirements.md)，`INV-*` 见 [领域模型 §6](./domain/README.md#6-运行时不变量)）；规则本身不在此重复。测试项是否通过，以真实实现后的单元、集成、端到端执行结果判定。

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
| BS-12 | OCR 提取值为 needs_review 或 rejected | 不形成可信 BodyObservation，不能自动用于临床确诊 | R-PS-03、INV-24 |
| BS-13 | 同一消息既有明确事实又有一个歧义 | 明确部分一次幂等提交，歧义部分独立待澄清，不虚构时间 | R-BS-03～05 |
| BS-14 | AI 在正式分析中提出假设 | 只记录在 Run 的基础记录/追加事件，不能写为用户 BodyHypothesis | R-BS-06、INV-02 |
| BS-15 | ConcernGroups 重分组且事实来源未变 | 不新增、不删除或改写原 BodyFact/Observation | INV-25 |
| BS-16 | 「昨天头痛」，今天未提及 | 昨日事实 asserted，Episode 当前是否持续可为 unknown；不自动标记 ended 或 ongoing | R-BS-07、INV-30～31 |
| BS-17 | 「好一些了，但抬手仍疼」 | 保存真实程度变化，Episode 仍可 ongoing，不将改善当作结束 | R-BS-07、INV-31 |
| BS-18 | 「休息不疼，抬手疼」 | 保留不同触发条件，不误判为同一时段互斥记录或反复结束/新发作 | R-BS-04/07 |
| BS-19 | 「10 月 6 日已完全好了，10 月 12 日又疼」 | 形成两个有历史关联的 Episode；不改动原事实、不假设同病因 | R-BS-07、INV-31 |
| BS-20 | 「其实上次说好了是我记错了」 | 更正旧结束依据并重新计算原 Episode，不制造一次虚假复发 | R-BS-04/07、INV-03/31 |
| BS-21 | 仅凭同一部位、时间接近提出不明确的 Episode 关联 | 明确 BodyFact 先保存；Medium/Low 不直接合并身份，实质影响时 Ask User | R-BS-03/07/10、INV-31/32 |
| BS-22 | 两项同时间、同条件、同侧的有效症状陈述互相矛盾 | Agent 主动 Ask User：纠错旧事实、纠错新事实、实际复发、条件不同、自行说明、暂不确定；不简单按最新时间覆盖 | R-BS-04/10、INV-32 |
| BS-23 | BS-22 的 Ask User 因断网没有得到回答 | 待回复交互可恢复；状态确被冲突影响才降为 unknown，保留冲突/先前依据；其他明确事实不丢失 | R-BS-10、INV-29/32 |
| BS-24 | 今天才补录上周症状加重 | 实际加重按上周排序，系统记录在今天的修订；按旧 Revision 回放仍只看到当时已知信息 | R-BS-05/07/10、INV-30～31 |
| BS-25 | 用户没有授权却由 Agent 发起 Episode 合并/拆分 | Go 拒绝关键历史修改；现有事实及关系不变 | R-BS-07、INV-31 |
| BS-26 | 用户明确同意合并或拆分两段 Episode | 旧身份与事实/下游引用可追踪，新 Episode 各自基于有效依据重新计算，不复制旧结束状态 | R-BS-07、INV-31 |
| BS-27 | Episode 关联的唯一结束 Fact 被用户撤回 | 不保留无依据的 ended；重新推导；不能自动放宽 SafetyState 限制 | R-BS-07/10、INV-31 |
| BS-28 | 「我怀疑是偏头痛」后说「这个猜测不对」 | Hypothesis 撤回或更正，关联的症状 Fact 和 Observation 不因猜测撤回而删除 | R-BS-06/09、INV-02/33 |
| BS-29 | 用户在 Chat 说「每周跑三次」，在 Person 的 Habits 查看 | 自动形成 lifestyle Fact 并由 Habits 显示，同一份权威数据；改变频率保留前后时间阶段 | R-BS-08、R-PS-05、INV-28/34 |
| BS-30 | Trainer 记录深蹲已完成，用户另说「训练后膝痛」 | 完成事实仍由 TrainingSessionLog 持有，膝痛属于 symptom Fact，关联训练来源但不重复造完成记录 | R-BS-08、R-EX-04、INV-26 |
| BS-31 | 用户自报体重与报告中 OCR 待核实数值 | 自报可作为注明来源的 Observation；未经准入 OCR 不成为可信报告观察，不能自动升级临床来源或 Diagnosis | R-BS-09、INV-24/33 |
| BS-32 | 用户明确要求删除医疗报告与其衍生健康数据 | 立即阻止相关来源新使用，持久且可重试地擦除原件、抽取/缓存与需删除的关联个人健康数据；独立有效来源不误删，最小脱敏回执并校验传播完成；未实现前测试不得标通过 | R-BS-10、R-PS-09、INV-34/51 |
| DXV2-01 | 用户只叙述“右臀痛、弯腰受限、跑步后疼痛” | BodyState 记录事实/Episode；无医学病名时不创建 ConditionEntry | R-DX-01/11、INV-35 |
| DXV2-02 | 用户说“我怀疑是偏头痛” | BodyHypothesis 记为用户假设，不自动写 Diagnosis | R-BS-06、INV-02 |
| DXV2-03 | Agent 在普通 Chat 中提出医学方向 | 只能初步讨论；没有用户正式分析与采纳，不创建 ConditionEntry | R-DX-02/07 |
| DXV2-04 | 用户请求正式分析但证据不足以提出具体医学名称 | Run 以 insufficient_evidence 结算，不以纯症状标题制造正式提案或条目 | R-DX-10/11、INV-35 |
| DXV2-05 | 用户采纳带 suspected 的医学诊断提案 | 新 ConditionEntry = adopted；AI 评估仍 suspected，来源不变 | R-DX-04/08、INV-06/37 |
| DXV2-06 | 用户尝试用确认操作把 suspected 改成 likely | 后端不根据 accept 调整 AI 判断级别；需要有证据的正式分析更新 | R-DX-04/08、INV-11/37 |
| DXV2-07 | 医生文件写“考虑 A” | 原始不确定措辞保留，不标成已临床确诊；无需造假 AI level | R-DX-08/16、INV-15/37 |
| DXV2-08 | 用户转述医生诊断，但没有相应医疗文件 | sourceKind=user_reported_clinical，不冒充 clinical_document | R-DX-08/16、INV-15 |
| DXV2-09 | 同一 BodyFact / Episode 被两个不同医学诊断引用 | 允许多对多关系，证据不复制、不得据此断言两个病同时成立 | R-DX-14、INV-36/39 |
| DXV2-10 | AI 提案同时有支持、反对与背景证据 | evidenceLinks 支持 supporting/counter/context，并有独立关联理由 | R-DX-03/14、INV-36 |
| DXV2-11 | 新的检查、症状或医学文件证据到达 | 新资料只进入来源权威，相关诊断提示复核，ConditionEntry 与已采纳证据链不自动修订 | R-DX-06/17、INV-38 |
| DXV2-12 | 用户报告某次疼痛 Episode 已结束 | BodyState 按证据推导 ended；Diagnosis 不自动标 resolved/inactive 或治愈 | R-BS-07、R-DX-13、INV-14 |
| DXV2-13 | 同一 Run 有两项新医学诊断提案，用户分别 accept 与 reject | 未采纳候选只留审计；有效决定完成后一次原子提交，至多一个 DiagnosisRevision | R-DX-04/12、INV-05/12 |
| DXV2-14 | 多提案 A 决定、B pending 时关闭 Chat 并重开 | A 决定留档但尚未提交，B 恢复 pending；关闭不等于 reject | R-DX-12、INV-12/29 |
| DXV2-15 | B 补充相关新证据，影响此前已接受的 A | A 决定 rechecked affected、提案 superseded，必须新提案再确认；无关决定免重选 | R-DX-12、INV-17 |
| DXV2-16 | 用户主动要求停止其余提案，或 Run 超时 | 剩余 pending 过期；已决定且仍有效的提案整体提交；无有效变更不制造新 Revision | R-DX-12、INV-05/12 |
| DXV2-17 | 新检查使诊断医学概念从 A 变成不同的 B | 新建 B conditionId；A 身份及旧来源/决定留档，替代须单独确认 | R-DX-01/14、INV-04/39 |
| DXV2-18 | 用户接受新增 B，但拒绝停用原 A | B 为 adopted，A 仍 adopted；不能静默认为旧诊断被医学排除 | R-DX-04/14、INV-18/39 |
| DXV2-19 | 用户明确要求撤回 A 并完成轻量确认 | 不创建正式分析 Run，A→inactive、写 user_withdrawal 审计；BodyState 保持不变 | R-DX-13/15、INV-14/19 |
| DXV2-20 | 用户撤回后又希望重新采纳 A | 由新 Run 评估当前证据并确认，保持 conditionId 和旧撤回历史 | R-DX-15、INV-39 |
| DXV2-21 | 互相冲突的两份临床声明 | 两个来源、时间和原始措辞并存，不以较新覆盖较旧或伪称医学排除 | R-DX-16、INV-15 |
| DXV2-22 | 另一设备已更改 Diagnosis，旧 Run 再提交 | 底版冲突拒绝提交，保留决定、重新核验，不静默覆盖 | R-DX-12、INV-09/12 |
| DXV2-23 | 运行中出现严重、需要终止分析的风险 | safety_blocked 终结；无部分提交，未决定失效、已决定不生效，Safety 独立执行 | R-DX-10、INV-23 |
| DXV2-24 | 同一新证据被多次检查复核相关性 | 提示去重、不建 DiagnosisReviewTask；完成正式复核且不更改条目也可关闭提示而不造空修订 | R-DX-17、INV-38 |
| DXV2-25 | 旧来源被纠错/撤销 | 当前资格反映改变并提示复核；旧历史快照可追溯，诊断不自动重写 | R-DX-06/16、INV-15/38 |
| DXV2-26 | 恢复 Run 投影，含 hypothesis_added、证据补充、拒绝、取代、超时、安全阻断 | 从基础记录、用户决定、有序事件可重建全部状态与有效性；AI 假设不进入 BodyHypothesis | INV-16/22、INV-02 |
| DXV2-27 | 用户直接编辑试图改医学诊断本质/证据强度，或 Agent 自主撤回 | 后端拒绝；只允许用户请求的非实质元数据纠错/准确撤回，取消不写入 | R-DX-13、INV-19 |
| DXV2-28 | 用户要求真正删除报告与衍生数据 | 不将撤回诊断或陈述撤回误当隐私擦除；来源失效后及时复核诊断证据及关联 Safety，不自动改已确认诊断或解除限制，清除须有核验 | R-PS-09、R-CD-04、INV-51 |
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
| TP-12 | Trainer 导航到最后一个动作，前面并未明确全部完成 | 不得批量把整份训练标 completed | R-EX-04、INV-26 |
| TP-13 | 服务器和用户位于不同时区/日期 | 日志 date 为用户训练日期，关联准确的模板与计划修订 | INV-27 |
| TP-14 | 训练途中出现匹配下一动作的安全限制 | 继续执行前重新执行门禁；restricted 命中则暂停受影响执行，urgent 安全终止，保留真实已完成记录 | R-SAFE-03/07、INV-08/45 |
| TP-15 | 保存训练反馈，旧 Treatment 同时更新状态 | 不未经用户授权修改当前 TrainingPlan 内容；新旧状态映射须单独审核 | R-TP-05、INV-07 |
| TP-16 | Agent 生成训练方案，用户仅预览 Draft 就关闭 | 不新增 TrainingPlan，不产生第一份有效 Revision；恢复后仍可继续准确提案 | R-TP-06、INV-40 |
| TP-17 | 用户接受准确创建草案，同时网络重试 | 仅创建一份 TrainingPlan 与首个不可变 Revision，其他计划不受影响 | R-TP-06/08/10、INV-09/43 |
| TP-18 | A/B 模板中出现两次同名动作或改组次 | 不同 planExerciseId 分别定位；仅调整剂量保留条目 ID，替换动作创建新条目 ID | R-TP-08、INV-41 |
| TP-19 | 用户仅设置「每周 3 次」，错过某一天 | frequency_only 不生成漏练、补练、skipped 或 Completed Session；显式日历课次不执行也不伪造 skipped | R-TP-09、INV-42 |
| TP-20 | 用户在相同时区一日内先后执行同一模板两次，第二次跨午夜 | 产生不同 sessionLogId；各自固定开始时本地日期与 revision/template，不自动分裂跨午夜历史 | R-EX-05、INV-27/41/42 |
| TP-21 | 用户点击「下一项」/播放动作演示/断线，再回来继续 | 未明确完成或跳过的组没有 Action；继续原 Session，计时不虚构；误触完成可追加 corrected 审计 | R-EX-06/07、INV-26/42 |
| TP-22 | Agent 提议调整后其他设备修改计划或出现相关新风险，或超过 7 天 | 准确提案 expired 并不可接受；计划内容不变，若重新提出需新授权 | R-TP-10、INV-43 |
| TP-23 | 用户将 completed 计划重新启用；另有两个 active 计划 | 重新启用保留原 planId 和历史；多 active 合法，同一用户最多一个 in_progress Trainer 实例 | R-TP-07、R-EX-07 |
| TP-24 | 正常完成、提前结束或风险导致终止 | SessionLog endedReason 不同，已完成的组保持事实；未做动作不自动记完成/跳过；计划状态与处方均不变 | R-EX-07、R-SAFE-07、INV-45 |
| TP-25 | 训练后反馈「右肩更痛」，Agent 建议调整计划 | 原主观反馈存在 Trainer，明确身体陈述进入 BodyState；建议本身不修改计划，安全门禁独立执行 | R-EX-08、R-TP-05、INV-07 |
| UI-01 | 桌面关闭/收起浮动窗口 | 可查看持续存在的 3D 人体场景 | [interface §1](./interface.md#1-核心空间) |
| UI-02 | 人体点选一个部位 | 仅形成上下文，不自动记录异常/疼痛 | [interface §6](./interface.md#6-人体交互工具) |
| UI-03 | Chat → 人体工具 → 返回 Chat | 上下文与待回答交互保持一致 | [interface §1](./interface.md#1-核心空间) |
| UI-04 | Person 查看身体、器材、资料 | 五个分类标签页；身份在标题区与 Identity Popover；显示真实当前状态与恰当空态，不用演示数据 | R-PS-01、R-PS-04 |
| UI-05 | 移动端切换业务窗口 | 任务与选区可恢复，主要操作可触达 | [interface §7](./interface.md#7-平台与辅助功能) |
| UI-06 | 878px 等窄桌面窗口打开 Diagnosis | 标题、病情卡片及操作按钮都在可见范围内，不向右溢出 | [interface §7](./interface.md#7-平台与辅助功能) |
| UI-07 | Chat 展示具名医学诊断确认工具 | 可以核对病名、AI 疑似强度（只读）、临床来源、证据与建议；采纳/不采纳/继续补充各自独立；逐项决定、统一提交 | R-DX-03/04/08/12 |
| SAFE-01 | 专业人员放行策略，只有任意文字说明而无来源 | 后端不能以此清除限制，仍需有权威解除依据 | R-SAFE-04、INV-20 |
| SAFE-02 | 非自动解除策略超过复核时间 | 仅请求复核，不可当作解除 | R-SAFE-04、INV-20 |
| SAFE-03 | 只有普通训练限制、正在进行正式分析 | 影响 Trainer 执行但不自动 safety_blocked 终结 Diagnosis | R-DX-10、INV-23 |
| SAFE-04 | 用户同一动作受 advisory 和 restricted 两条限制 | 门禁选择 restricted 的 block，分别显示来源与原因；解除 advisory 不放行 restricted | R-SAFE-05/06、INV-44 |
| SAFE-05 | 全局 urgent 与局部 restricted 并存，解除局部限制 | 全局仍阻断所有计划；urgent 不得靠时间或 Agent 自行解除 | R-SAFE-05/06/08、INV-20/44 |
| SAFE-06 | 训练中正在执行的动作命中 restricted | 安全暂停 paused(safety_hold)，保留已完成 Action，不产生虚假的 skipped 或计划修订 | R-SAFE-07、INV-45 |
| SAFE-07 | 训练中新增 urgent 风险 | ended(safety_stopped)，阻断新训练；需停止正式诊断时仍遵守 INV-23 的零提交终结 | R-SAFE-05/07、INV-23/45 |
| SAFE-08 | 旧风险的来源证据被纠正，同时另一设备新增更高风险 | 按 restrictionId 条件修改，不能整体覆盖新增限制；不直接把证据纠正当解除 | R-SAFE-08、INV-09/44 |
| SAFE-09 | 临床来源矛盾、上传文件仅 OCR 候选、专业放行没有明确适用范围 | 专业放行待复核，限制保持有效；用户转述仅按用户来源等级展示 | R-SAFE-04/08、INV-20 |
| SAFE-10 | 安全门禁校验暂不可用，用户试图开始或继续匹配动作 | 不宣称「安全」，后端不授权继续受影响动作，提示重试；原训练事实保持 | R-SAFE-07/08 |
| UI-08 | Chat 有待确认提案，用户选择继续补充 | Ask User 选项及允许的自由解释整体替换普通聊天输入区；补充只答复当前 interaction/run，不开启并行 Chat Run，刷新后可恢复 | R-CHAT-02/03、INV-29/47 |
| UI-09 | Person 为空、加载失败或报告证据待核实 | 区分 empty/loading/error、admissible/needs_review/rejected；不以演示值替代 | R-PS-01～03、INV-24/28 |
| UI-10 | 多提案中 A 已决定、B 未决定，关闭后再回来 | A 留档未生效、B 保持 pending 且可恢复 | R-DX-12、INV-29 |
| SEC-01 | 使用别人的 planId / conditionId 写入 | 后端拒绝越权 | G-04 |
| SEC-02 | 两个并发分析或计划修改 | 以修订并发控制避免覆盖他人的最新变更 | INV-09 |

| PC3-01 | 新用户第一次发起聊天、多次返回 Chat | 始终一份稳定 Conversation，首次可惰性创建；没有创建/选择/归档多会话入口 | R-CHAT-01、INV-46 |
| PC3-02 | 电脑端与手机端同时发普通消息 | 后端只允许一个当前 Chat Run；冲突有可恢复反馈，不生成第二 Conversation 或覆盖原执行 | R-CHAT-02、INV-46 |
| PC3-03 | Agent 调用 Ask User 并请求用户选择 | 普通输入整体替换为该工具选项/允许的解释控件；期间不能发送另一条普通消息 | R-CHAT-03、INV-47 |
| PC3-04 | Ask User 待答中刷新、收起、重连或更换设备 | 恢复原 interactionId/runId/toolCallId、工具问题与已有决定；未答不视为 reject；重复有效答案幂等 | R-CHAT-03、INV-29/47 |
| PC3-05 | Agent 用已过期或旧版工具授权修改另一个目标 | 后端拒绝过期/越权/指纹不匹配请求，领域 Revision 不增加 | R-CD-03、INV-47/52 |
| PC3-06 | Chat 表述明确的生活习惯且后续查看 Person/Habits | 只在 BodyState 有一份长期事实及必要来源片段；没有第二份习惯副本 | R-PS-05、R-CHAT-06、INV-28/49 |
| PC3-07 | 已处理旧聊天超过滚动窗口并符合清理条件 | 语义处理与真实提交先完成，原话证据安全转存；消息和可控制的 Checkpoint/缓存清理后才前移 prunedThroughSeq | R-CHAT-04/05/06、INV-48 |
| PC3-08 | 待答 Ask User、活动 ToolCall 或未结算 Diagnosis Proposal 位于历史边界 | 清理游标不得跨越活动依赖，不留下孤立工具结果或悬空来源引用 | R-CHAT-05、INV-48 |
| PC3-09 | 消息本身没有任何长期价值（如普通知识问答） | 完成处理后无需生成 BodyFact；可按窗口策略进入安全清理候选，无永久摘要膨胀 | R-GEN-01、R-CHAT-04 |
| PC3-10 | 用户纠正已从历史 Chat 清理的旧身体陈述 | 仍能基于 BodyState 结构化事实和必要原话来源准确纠错；不凭空制造病程变化 | R-CHAT-06、R-BS-04、INV-48 |
| PC3-11 | 用户编辑 Profile 中语言、时区、显示单位 | 身份字段可选、权限受控，旧 Observation 原值/单位与 Trainer 历史训练日不变 | R-PS-06、INV-49 |
| PC3-12 | 器材变为不可用且该器材用于既有计划 | 只改变 EquipmentAsset 与产生影响检查/建议，当前 TrainingPlanRevision 不变 | R-PS-07、INV-49 |
| PC3-13 | 上传一张兼具图片和检查报告意义的文件 | Media/Reports 共享同一个 assetId 和原件；显示各自视图，不重复删除实体 | R-PS-08、INV-49 |
| PC3-14 | 文件 OCR 成功，但医学指标仍 needs_review/rejected | 明确区分 asset/processing/admission；未经准入的值不能成为可信 BodyObservation 或 Diagnosis | R-PS-03/08、INV-24/50 |
| PC3-15 | 用户仅选择「移除原始文件」 | 删除源文件与不再需要的完整 OCR 缓存，保留已合法准入的必要结构化健康观察与历史版本；UI 标记原件不可用并说明仍留存的健康数据 | R-PS-09/10、INV-50 |
| PC3-16 | 用户移除原文件后尝试用历史数值作专业放行 | 历史观察可按原资格参考，但无可核验原件不能满足要求原件的放行策略，Safety 门禁保持 | R-PS-10、R-SAFE-08、INV-50 |
| PC3-17 | 用户明确「删除文件及关联健康数据」且存在其他独立来源 | 先禁止目标来源新使用，可靠擦除关联数据及受控副本；独立来源仍有效，Diagnosis 仅提示复核而不静默修改，Safety 不自动解除 | R-PS-09、R-CD-04、INV-51 |
| PC3-18 | 文件对象删除成功、衍生库/索引或 Checkpoint 清理失败 | 删除任务处于可重试失败而非 completed，来源仍禁止使用；重试幂等不误删独立资料 | R-PS-09、INV-51 |
| PC3-19 | 新用户寻找聊天公开分享按钮；旧分享令牌仍存在 | 目标 UI 无公开健康对话分享；旧令牌与快照需实测撤销/治理，不因为新 UI 隐藏就宣称已退役 | R-CHAT-07、INV-53 |
| PC3-20 | 用户越权访问别人的 Profile、资产、Conversation 或待答 Interaction | Go 后端基于 userId 拒绝，不依赖前端过滤；不泄露健康原文/文件 | G-04、R-CD-03 |
| CD3-01 | 相同身体陈述的两次重试产生同一领域操作 | 真实领域提交幂等，最多一个相关 Revision/持久事实；成功才给业务回执 | R-CD-02、INV-52 |
| CD3-02 | ToolCall succeeded，但 Go 领域事务失败 | Chat 不报告「已保存」，有真实失败/可重试结果，业务 Revision 未增加 | R-CD-02、INV-52 |
| CD3-03 | 单条自然语言同时涉及已完成训练记录和身体疼痛，BodyState 暂时失败 | Trainer 真实完成成功与 BodyState 写入失败分项反馈并可重试；不宣称全域原子提交 | R-CD-02、R-EX-08、INV-52 |
| CD3-04 | BodyState 事实变更传播至相关 Diagnosis 和 TrainingPlan | 可靠去重事件可重放，仅触发可授权复核/影响建议；不自动改 ConditionEntry/处方 | R-CD-01/02/04、INV-52 |
| CD3-05 | 来源消息已物理清理，Diagnosis 或 BodyState 继续引用旧健康陈述 | SourceRef 指向领域持有的必要原话证据及版本而非已删除 Message；跨用户引用拒绝 | R-CD-01、R-CHAT-06、INV-48 |
| CD3-06 | 医疗报告原件被移除，历史准入候选被新分析引用 | 资格与原件可用性双维度可见；允许适当历史引用，禁止假称可重新核验文件 | R-PS-10、INV-50 |
| CD3-07 | 受擦除影响的来源被旧缓存、Checkpoint 或二次事件再次使用 | 立即禁止新使用；旧副本/重放不复活个人信息，擦除完成状态需各受控步骤核验 | R-PS-09、R-CD-04、INV-51 |
| CD3-08 | Diagnosis 原子 Patch 同时收到其他设备的直接更正 | 过期 expectedRevision 拒绝并复核准确提案，不复用旧决定、不覆盖其他修订 | R-CD-03、INV-09/52 |

## 2. 实施工作顺序

| 阶段 | 交付物 | 验收重点 |
| --- | --- | --- |
| P1 领域契约 | BodyState 自动写入和时间/来源模型；Diagnosis 当前聚合；Plan 级授权模型、SafetyState 与 Person/Chat 及跨域契约 | 类型、权限、INV-01～53 的契约测试（新增 INV-46～53） |
| P2 自述记录 | Agent 提取 → Go 校验提交 → 自动反馈、澄清恢复；上传资料的证据采纳 | BS-01～11（含 BS-04b） |
| P3 Diagnosis V2 | 用户请求触发具名诊断分析；Run/Proposal/Decision 审计；来源/确定程度独立；Chat 采纳工具；一次原子提交 | DXV2-01～28 |
| P4 TrainingPlan | 每份计划的课模板与阶段化动作、组次/频率、只读影响检查、用户授权、Trainer 执行记录与安全门禁 | TP-01～25、SAFE-01～10 |
| P5 交互集成 | 3D 桌面、浮动 Chat/Ask User 串行替换输入、Person 单一文件资产、Diagnosis 与 Trainer 的真实数据状态 | UI-01～10、PC3-01～20 |
| P6 稳定化 | 旧 Conversation/分享迁移、滚动历史/Checkpoint 清理、双语义隐私删除、跨域可靠事件、权限、并发及端到端回归 | CD3-01～08、PC3-07～20、SEC-01～02 与全矩阵 |

每阶段均以「真实输入 → Agent/工具 → 后端持久状态 → UI 读取」的端到端闭环作为完成定义。只通过类型检查或静态页面检查不足以宣称业务完成。

## 3. 独立审核点

- 对照业务文档核查 API 与数据库，不得假设模型草案已被实现。
- 使用真实/模拟用户案例验证相对时间、主体归属、因果表述和医学确定性。
- 检查一条消息中多次工具调用是否发生部分成功、重复、失败或取消。
- 验证未授权建议只存为 Proposal，不修改训练内容；检查所有写操作的用户授权与并发修订。
- 对照 Penpot 的实际组件与交互核验桌面/移动界面，不能只比较静态截图。
