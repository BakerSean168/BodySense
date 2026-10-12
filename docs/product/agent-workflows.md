# BodySense · Agent 工作流与交互工具

**本文件定义 Agent 的处理流程、上下文与工具。** 业务规则见 [产品需求](./requirements.md)，字段与不变量见 [领域模型](./domain/README.md)，用户看到的界面见 [UI 与交互](./interface.md)。

## 1. Agent 的职责

Agent 负责解析用户意图，选择合适的证据采集和业务工具，解释操作结果并提出下一步建议。工具的结构化返回结果及业务后端负责最终事实和状态权威。

~~~text
用户消息 / 图片 / 选中的人体部位
         ↓
理解主体、部位、时间、事实/推测、意图
         ├── 普通知识问题 ──→ 直接解释（R-GEN-01）
         ├── 明确本人事实 ──→ BodyState 验证与自动保存（§3.1）
         ├── 歧义/同时间冲突 ──→ ask_user → 继续处理该字段
         ├── 正式分析请求 ──→ 分析/证据审查 → confirm_condition → Diagnosis 局部补丁（§3.2）
         ├── 用户明确更正元数据/撤回诊断 ──→ 轻量确认 → 直接编辑（§3.5）
         ├── 计划创建请求 ──→ 计划 Draft → 用户接受 → 创建已生效计划（§3.3）
         ├── 计划修改请求 ──→ 定位计划/验证授权 → 定向修改（§3.3）
         └── 发现训练影响 ──→ 只读影响检查 → 建议 → 用户决定（§3.4）
                                        ↓
                        返回实际提交状态 / 当前业务投影
~~~

3D 人体选区、图片上传和示范动作默认只作为交互上下文或原始资料，不能自动转换成医学事实。

## 2. 上下文装配

每次运行按当前任务装配：

- 相关当前 BodyState、有效安全限制与必要病程；
- 当前 Diagnosis 的相关病情及证据；
- 当次问题涉及的 TrainingPlan 与执行约束；
- 最近对话与按需检索的相关历史；
- 用户当前选中的人体部位、开放的业务窗口及工具模式。

不用每轮加载完整聊天、全部历史、所有计划；检索到的旧陈述必须与当前事实、更正及时间语义核对。

## 3. 逻辑工具与流程

工具名称是建议的职责命名，具体可映射到业务 API：

| 逻辑工具 | 允许的结果 | 关键防护 | 规则 |
| --- | --- | --- | --- |
| record_body_state | 自动提交明确本人事实与其症状关联、真实时间变化、用户本人提出的假设（BodyHypothesis） | 身份、来源、时间、幂等与冲突检查；Episode 状态只能由证据重建；不写入 AI 假设 | R-BS-01～10 |
| ask_user | 主动追问影响病程/历史判断的实质冲突，允许结构化选项和自由文本解释 | 绑定冲突 Fact/Episode、目标修订与待回答字段；用户明确回答可授权对应准确更正；未答时保持可恢复身份，不伪造状态 | R-BS-03、R-BS-10、INV-32 |
| inspect_evidence | 获取和审阅照片、资料、自测与知识证据 | 不能推定没有证据支持的医学事实 | R-DX-08 |
| analyze_conditions | 用户明确请求时创建正式 ConditionAnalysisRun，依据 BodyState/证据形成具名医学诊断候选及 create/update/set_disposition/update_relation 提案；运行中新 AI 假设只写 hypothesis_added | 固定有来源版本的证据基线；证据不足不制造医学病名；未经请求不启动正式 Run；严重风险可安全阻断；不写 BodyHypothesis | R-DX-02/07/10/11、INV-02/23 |
| confirm_condition | 在 Chat 中逐项展示准确的医学诊断提案及来源、证据、确定性和处理建议；用户决定 accept/reject 或继续补充 | 授权绑定 proposalId、内容指纹和证据版本；采纳不改变 AI suspected/likely，不制造临床来源；拒绝已有条目更新不撤回旧诊断；此工具只留决定不直接写 Diagnosis | R-DX-03～05/08/12、INV-06/18 |
| apply_diagnosis_patch | 提案全部结算且新证据复核完成后，原子提交所有有效决定，至多生成一次 DiagnosisRevision | 权限、来源声明/证据、医学身份、采纳、底版修订、安全门禁与幂等检查；safety_blocked Run 永不提交 | R-DX-01/12/14～16、INV-04～06/12/17/23 |
| edit_condition_entry | 用户明确请求更正不影响诊断医学身份的元数据、或撤回已有具名诊断时，轻量确认后写入 ConditionDirectEdit | 无准确用户请求不可调用；不能直接改医学名义/来源/AI 评估/证据或处理；症状缓解属于 BodyState，不再有 mark_resolved | R-DX-13、INV-18～19 |
| prepare_plan_draft | 根据用户目标与身体资料准备未接受的计划预览；创建 PlanProposal(kind=create) | 未经用户确认不创建有效 TrainingPlan，不能从 Draft 启动 Trainer | R-TP-06 |
| inspect_plan_impact | 检查新情况对所有相关计划的影响 | 只读，不修改训练内容 | R-TP-05、R-TP-08 |
| propose_plan_change | 创建 PlanProposal(kind=update)，包括准确差异、来源和内容指纹 | Agent 建议保持 pending；期限 7×24 小时，相关上下文变化可提前失效 | R-TP-04/10 |
| apply_plan_change | 提交指定 Plan 的局部修改或用户已接受的创建方案 | 验证 userId、proposalId、内容指纹、底版与最新 SafetyState；仅真正提交后报告已保存 | R-TP-03～04/06/10、INV-07 |
| execute_training | 开始、暂停、恢复、结束指定计划课模板；明确完成/跳过/更正某组或动作 | 后端安全门禁、sessionLogId、planExerciseId、时区快照、幂等和逐组真实操作审计 | R-EX-05～08、R-SAFE-07 |
| open_business_window | 在对应窗口呈现当前对象或工具 | 无任何持久化业务副作用 | — |

### 3.1 自动记录 BodyState

1. 读取当前消息与必要上下文，提取每个本人陈述。
2. 根据主体、时间、部位、语义和来源进行验证；五类 BodyFact 独立时间语义，症状与 SymptomEpisode 关系单独判定。
3. 将明确事实打包成一次或少量语义一致的幂等事务；High 的 Episode 关联可自动提交，Medium/Low 不强行合并病程。
4. 对直接影响病程判断的冲突优先调用 ask_user（明确候选解释、允许自由说明），仅阻断相关争议关联/字段；其他明确事实照常提交。用户准确回答后的合并拆分/关键更正按目标授权执行。未回答或网络中断时恢复原待回复交互；只有状态直接受冲突影响时 Episode 投影保守为 unknown，保留冲突依据。
5. 收到后端确认后反馈「已记录：……」；失败时明确说明，不伪称成功。

用户本人提出的怀疑（「我怀疑是……」）作为 BodyHypothesis（`source = user`）自动记录（R-BS-06）。第三人陈述、疾病知识提问、AI 自己提出的解释不写入 BodyState；AI 假设只记录在 ConditionAnalysisRun 中。已保存的事实可通过更正操作修正。

### 3.2 Diagnosis 正式分析与病情确认

1. 用户明确请求新分析、局部更新或全量重分析（R-DX-02）；建立 ConditionAnalysisRun 并记录请求、BodyState/Diagnosis 基线和输入证据版本。普通聊天中推测、自动写事实或发现新证据，不自动启动正式 Run。
2. Agent 获取必要的 BodyFact、SymptomEpisode、BodyObservation、用户本人 BodyHypothesis 及通过证据管线准入的材料。一个证据可以被多个候选医学诊断引用，须区别 supporting/counter/context，不把相关时间顺序当作因果证明。AI 运行开始后的新假设写入 hypothesis_added 事件，不写 BodyState。
3. 形成具名医学诊断候选及候选间的鉴别依据。证据不足以提出具体名称时如实说明、以 insufficient_evidence 结束；不以纯症状说明制造 ConditionEntry。对具名候选可以提出 create、update（同一医学诊断概念）、set_disposition 和 update_relation 提案；实质不同医学概念必须创建不同 conditionId。
4. Agent 的医学定性评估只有 suspected/likely，必须说明证据、反证和信息缺口。临床声明按真实来源及医生原始措辞另行保留；用户转述不伪称文件支持，文件数值不等于医生诊断。用户的确认不会改变这些确定性和来源判断（R-DX-08）。
5. confirm_condition 在 Chat 逐项展示医学名称、来源声明、证据及不确定性、原因、建议、变更前后与当前身份；用户分别 **采纳 / 不采纳 / 继续补充**。采纳或拒绝存 ConditionProposalDecision（accept/reject），尚未提交时 Diagnosis 不变；拒绝针对已有条目的 update、set_disposition 或 relation 提案不影响旧条目（INV-18）。
6. 补充事实先进入 BodyState，作为本 Run 的新证据；Agent 以新证据重新检查所有此前已决定提案。unaffected 保留旧决定；affected 创建新的准确提案、使旧决定失效并重新询问（INV-17）。关闭窗口不等于拒绝；pending 可在同一 Run 恢复。超时与用户明确结束余项按领域契约处理（R-DX-12）。
7. 所有提案已结算、复核及并发、安全校验完成后，apply_diagnosis_patch 作为一个原子事务提交有效决定，最多产生一次 DiagnosisRevision；新证据只触发建议复核，不自动写 ConditionEntry；没有有效变更则 closed_without_change。
8. 若确认中出现必须停止分析的严重风险，Agent 立即提示适当医疗帮助并终结为 safety_blocked：已决定内容只留审计、不生效，待确认提案失效，此 Run 不生成 DiagnosisRevision。Trainer 专用的普通限制不自动停止正式分析（INV-23）。
9. 成功提交后读取完整当前 Diagnosis，不以局部响应覆盖整个聚合；只在确认后报告已保存。多设备同时提交时拒绝过期补丁、保留决定并重新核验，而非静默覆盖。

### 3.3 训练计划创建与用户主动修改

**首次创建**：用户提出目标 → 读取必要身体事实、诊断背景、器材及安全限制 → Agent 生成完整 PlanSessionTemplate 草案（Draft）→ 用户查看并明确「使用此计划」→ Go 校验最新证据、相关安全上下文和授权后提交首个已接受 Revision；保存计划不等于允许执行受限动作，关闭 Draft 不会在后台偷偷创建计划。具体见 TP-D2。

**修改已有计划**：

1. 从用户请求中识别 planId、templateId、planExerciseId 与动作/次数/时间等修改范围，不根据动作名称误改同名条目。
2. 校验 BodyState 的最新安全约束、当前计划修订与用户权限。
3. 对没有风险或实质歧义的明确用户指令，以原授权直接生成精确变更并提交指定计划的新 Revision；关键歧义/医学风险时先澄清。
4. 返回实际变更内容；其他计划完全不变。

请求内容模糊时使用 ask_user；破坏性或高风险变更执行额外必要保护，不反复请求用户已经明确表达的同一授权（R-TP-03）。

### 3.4 Agent 主动提出计划调整

1. BodyState 出现与既有训练计划相关的新信息。
2. inspect_plan_impact 检查受影响的具体计划和动作。
3. 向用户展示「为什么建议检查」「影响哪个计划」「拟改什么」「可不修改」。
4. 生成待决定提案，用户选择「接受调整」或「暂不修改」。
5. **只有明确接受后**调用 apply_plan_change。
6. 用户选择「暂不修改」（提案 rejected）、关闭界面（提案保持 pending，可恢复）、7×24 小时未决、底版或相关健康/风险上下文变化（提案 expired）或网络失败时，计划内容保持原样（状态定义见 [domain §4](./domain/training.md#4-trainingplan)）。


### 3.4a Trainer 执行与真实记录

1. 由用户选择已生效的 TrainingPlan 与具体 PlanSessionTemplate，Go 校验用户、当前 Revision 与 SafetyState，开启新的 sessionLogId。Draft 不能执行；同一用户最多一个 in_progress。
2. 开始、恢复、前进到下一受影响动作和安全变化时重新检查 TrainerGate；advisory 提示，restricted 对匹配范围阻断，urgent 全训练终止。
3. 只把用户明确完成/跳过的组或动作追加到 SessionAction，误操作追加更正；浏览演示、下一项、断线、关闭窗口不产生 completed/skipped/ended。
4. 保存真实暂停区间、正常/提前/安全结束原因、用户训练日的 IANA 时区快照；跨午夜不拆开执行实例。
5. 主观训练反馈留在 Trainer，明确身体事实另外写入 BodyState；需要更新训练方案时走 inspect_plan_impact → Proposal → 用户授权，安全阻断无需用户批准改处方。

### 3.5 用户直接更正与撤回诊断

1. 用户明确要求更正某个诊断的非实质展示元数据，或者撤回具体已采纳的医学诊断（R-DX-13）；不创建 ConditionAnalysisRun。
2. 定位准确的 conditionId。名称有歧义、不同侧别或存在多个候选时，ask_user 先澄清操作范围。
3. edit_condition_entry 展示准确的目标与字段差异，用户确认后才调用 Go 业务提交 ConditionDirectEdit；取消、关闭及失败不修改 Diagnosis，写入成功后才报告成功。
4. withdraw 将 disposition 从 adopted 改为 inactive 并以 user_withdrawal 留存历史；不删除旧医学结论及 BodyState 事实。correct_metadata 不得偷偷修订医学概念、诊断确定程度、来源或处理方向。
5. 用户说「症状已经好了」时，按 §3.1 写入 BodyState 并按真实证据更新 Episode，而不是借 Diagnosis 的旧 mark_resolved 路径变更。
6. Agent 如果根据新证据认为医学结论应改变，只能建议用户正式重新分析，不能伪造直接编辑请求。并发直接编辑与正式 Run 必须核验 Diagnosis 基线版本。

## 4. 交互工具状态

每次待回答问题或待授权提案必须有稳定交互 ID、关联用户、关联消息、关联业务对象、过期/恢复语义。系统需支持：

- accept：应用准确的授权目标；
- reject：用户明确拒绝（「不是这个」「暂不修改」），保留原业务内容并记录决定；
- close：关闭界面或离开对话**不是拒绝也不是接受**，交互保持 pending；
- resume：以同一状态恢复对话任务（含未决定的病情提案与计划提案）；
- expire：训练提案 7×24 小时未决、业务底版或相关安全上下文变化时失效，不产生内容修改；
- retry：幂等重试，重复回答不二次提交；
- stale：业务底版已变化，或新证据影响了已决定的病情提案时，重新核验与展示；
- fail：显示错误，避免虚假的成功反馈。

病情提案的状态与提交规则见 [domain §3.3](./domain/diagnosis.md#33-多提案确认与原子提交)，计划提案见 [domain §4](./domain/training.md#4-trainingplan)。

模型可对无效工具参数进行有限次纠错，不能绕过后端校验。所有正式分析及训练内容改动需可追溯到触发的用户请求或批准操作。

## 5. 安全处理

识别到需要就医检查的风险信号时，立即提供适当、安全的下一步；advisory 只提醒、restricted 阻断作用范围、urgent 全训练阻断并可能按 INV-23 终结正式分析；如有必要，在 BodyState SafetyState 中发布带风险等级、范围、有效期、解除策略与解除条件的执行限制，由 Trainer 执行门禁（R-SAFE-01、R-SAFE-03）。需专业评估的限制和 urgent 默认专业人员放行；仅有审核的低风险定期规则允许 on_expiry；到达复核时点时，Agent 向用户发起复核提示，但不能自行降低等级或解除限制，只能记录用户完成的复核或用户提供的专业放行及来源（R-SAFE-04，规则见 [SafetyState 领域](./domain/safety.md)）。不能把模型建议伪装成临床诊断，也不能将症状与训练时间相邻误写为已证明的因果关系（R-EX-02）。安全执行门禁与训练计划授权保持独立（R-SAFE-02）。正式分析进行中出现需要停止分析的严重风险时，按 §3.2 中断该运行（`safety_blocked`）；只影响训练执行的普通限制只由 Trainer 把关，不中断分析（R-DX-10）。

## 6. 状态权威与未定操作边界

BodyState 的事实必须由 Go 校验入库；报告候选只在准入后变为可信 Observation；AI 假设仅属于 ConditionAnalysisRun。Diagnosis 的 Run/Proposal/Decision 具有稳定身份和原子提交边界；收起/关闭窗口不等于业务决定。TrainingPlan、Trainer 日志和 SafetyState 独立拥有状态，不能混用。Person 是组合视图，Conversation 只是交互通道。

pending 时必须支持具有正确上下文的「继续补充」操作；是否允许普通输入框并行消息及如何绑定仍待 DC-017。详细状态见 [domain](./domain/README.md)，其他未决项见 [domain-decisions](./domain-decisions.md)。
