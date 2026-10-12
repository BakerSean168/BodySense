# BodySense · 术语表

**本表统一产品文档、设计稿与代码讨论中的业务用语。** 写文档、Issue、测试名和 UI 文案时使用「标准术语」一列；「避免使用」一列中的说法不得再用来指代同一概念。代码标识符保持英文。

## 1. 业务对象

| 标准术语（英文 / 中文） | 定义 | 规则所在 | 避免使用 |
| --- | --- | --- | --- |
| **BodyState** / 身体状态 | 每位用户唯一、持续更新的身体事实、观察、假设与真实病程集合；明确的本人陈述自动写入 | [requirements §2.2](./requirements.md#22-bodystate-身体状态) · [domain §2](./domain/bodystate.md#2-bodystate) | 身体档案（指数据对象时）、问诊记录、会话状态 |
| **BodyFact** / 身体事实 | 用户明确陈述的五类事实；独立保存时间、来源和记录状态 asserted/superseded/withdrawn，不再使用 active/resolved/historical 的通用生命周期 | [domain §2](./domain/bodystate.md#2-bodystate) | 用事实记录状态表示真实病程状态 |
| **SymptomEpisode** / 症状经历 | 以稳定身份组织同一段症状经历的 BodyFact，unknown/ongoing/ended 为依据事实及关联事件重建的状态投影；复发建立可关联的新 Episode，不代表相同病因 | [domain §2.4](./domain/bodystate.md#24-symptomepisode身份关联及状态bs-d3d7) | 用 Diagnosis/Concern 代替一次症状经历 |
| **BodyObservation** / 观察 | 有来源的测量、自测、设备/报告观察；自报数据不等于临床核实，文件 OCR 结果须经证据管线准入 | [domain §2.5](./domain/bodystate.md#25-bodyobservationbodyhypothesisconcerngroup-与证据分级bs-d9) | 未核实 OCR 候选、AI 推测 |
| **BodyHypothesis** / 用户假设 | 用户本人提出的病因怀疑或可能疾病（`source = user`），自动记录；不具备确诊权威。AI 提出的假设不属于 BodyState，只在 ConditionAnalysisRun 中 | [requirements R-BS-06](./requirements.md#22-bodystate-身体状态) · [domain §2](./domain/bodystate.md#2-bodystate) | 候选诊断（DiagnosisAnalysis candidates，旧实现）；用「假设」指 AI 推测的持久记录 |
| **Diagnosis** / 综合诊断 | 每位用户唯一的综合诊断聚合，当前列表仅含用户采纳的具名医学诊断；inactive 条目在历史可追溯 | [requirements §2.3](./requirements.md#23-diagnosis-综合诊断) · [domain §3](./domain/diagnosis.md#3-diagnosis) | 综合分析、症状分组、按次诊断报告 |
| **ConditionEntry** / 医学诊断条目 | 一项具体医学疾病或综合征的具名诊断概念，关联来源声明、证据、AI 判断（可选）、用户采纳状态 adopted/inactive；不包含通用症状 lifecycle | [domain §3](./domain/diagnosis.md#3-diagnosis) | 纯症状标题、健康问题容器、以 Concern/ Episode 冒充医学诊断 |
| **ConditionAnalysisRun** / 正式分析运行 | 一次正式分析的追加式审计：请求、范围、底版修订、证据、AI 假设（创建时快照 + 之后追加的 `hypothesis_added`，均无权威）、产生的提案。创建时写入的基础记录与审计事件不可变；当前状态（open / committed / closed_without_change）是由事件推导的投影，按顺序回放基础记录与事件可完全重现 | [domain §3.1](./domain/diagnosis.md#31-正式分析审计对象) | DiagnosisAnalysis（新规格中不再使用此名） |
| **ConditionProposal** / 医学诊断提案 | 一次正式分析提出 create/update/set_disposition/update_relation 的准确变更，绑定医学身份、证据版本和内容指纹；确认前不生效 | [domain §3.1](./domain/diagnosis.md#31-正式分析审计对象) · [agent-workflows §3.2](./agent-workflows.md#32-diagnosis-正式分析与病情确认) | 已采纳诊断、症状记录、旧 endTarget/mark_resolved |
| **ConditionProposalDecision** / 用户提案决定 | 用户对准确诊断提案作出 accept/reject 的追加式审计；不调整医学 suspected/likely，继续补充不属于决定 | [domain §3.2](./domain/diagnosis.md#32-用户选择与数据映射) | 将用户采纳按钮视为医学诊断概率选择 |
| **ConditionDirectEdit** / 直接编辑审计 | 用户明确请求非实质展示元数据更正或撤回具体医学诊断，经轻量确认写入，不能变更医学判断或制造“已缓解” | [requirements R-DX-13](./requirements.md#23-diagnosis-综合诊断) · [domain §3.5](./domain/diagnosis.md#35-用户直接更正与撤回) | 用病程缓解表示诊断撤回、Agent 自行直接编辑 |
| **TrainingPlan** / 训练计划 | 一份完整训练处方（热身 → 正式训练 → 必要放松，含组数、次数/时长、频率）；每位用户可有多份 | [requirements §2.4](./requirements.md#24-trainingplan-训练计划) · [domain §4](./domain/training.md#4-trainingplan) | Treatment、治疗方案、训练模块、部位模块 |
| **PlanProposal** / 训练计划提案 | 统一未接受的首次创建 Draft（create）和 Agent 建议调整（update）；提案绑定内容指纹/底版/用户授权，最长 7 天未决自动失效，不接受不生效 | [domain §4](./domain/training.md#4-trainingplan) | PlanChangeProposal（旧逻辑 DTO）、Treatment revision、把 Draft 当已生效训练计划 |
| **Trainer** / 训练执行 | 打开某份 TrainingPlan 进行训练、计时与反馈的执行窗口 | [requirements §2.5](./requirements.md#25-训练执行与反馈) · [interface §2](./interface.md#2-业务窗口) | 训练流程（Workflow/Training，设计稿旧名） |
| **PlanSessionTemplate** / 课模板 | 计划内容中「一次课」的模板，含热身/正式/放松阶段；单课计划只有一个 | [domain §4](./domain/training.md#4-trainingplan) | TrainingSession（指计划内容时）、exerciseStages（已移入模板） |
| **TrainingSessionLog** / 训练记录 | 唯一 sessionLogId 的一次实际执行，绑定执行时计划修订、模板、用户时区训练日，逐组记录明确完成/跳过、撤销更正及反馈 | [requirements R-EX-05～08](./requirements.md#25-训练执行与反馈) · [domain §4](./domain/training.md#4-trainingplan) | TrainingLog（旧代码名）、计划模板、将打卡布尔值当作全组完成 |
| **Person** / 个人档案 | 展示个人资料与身体记录、器材、资料等的窗口与业务范围；五个分类 Records / Equipment / Media / Reports / Habits，身份在标题区与 Identity Popover | [requirements §2.6](./requirements.md#26-person-个人档案与当前状态) · [interface §3](./interface.md#3-person-个人档案) | Profile 窗口（设计稿旧名） |
| **Profile** / 基础资料 | Person 中相对稳定的身份与偏好数据（领域对象），显示在 Person 标题区与 Identity Popover；不含有时间语义的测量与习惯 | [domain §1](./domain/README.md#1-聚合关系) · [R-PS-04](./requirements.md#26-person-个人档案与当前状态) | 用 Profile 指整个 Person 窗口；把 Profile 当作第六个分类标签页 |
| **Chat** | 自然表达、提问、发起分析与授权计划修改的对话窗口，承载交互工具 | [interface §2](./interface.md#2-业务窗口) | Assistant（设计稿旧名）、Consultation / 问诊会话 |
| **Conversation** / 对话 | Chat 中的交流与工具活动记录；不是身体事实的权威存储 | [domain §1](./domain/README.md#1-聚合关系) | Consultation Session（旧实现） |
| **SafetyState / 安全阻断** | BodyState 中的权威风险状态：执行限制的风险等级、范围（全训练/计划/动作）、有效期、解除策略（`liftPolicy`：到期自动解除 / 用户复核 / 专业人员放行）与解除条件，Trainer 据此把关；产品等级 advisory/restricted/urgent，重叠按最严生效；不等于修改计划内容或状态 | [requirements R-SAFE-03](./requirements.md#27-安全) · [SafetyState](./domain/safety.md#23-safetystate-与执行限制) | 安全修订（不存在此概念）；用计划的 paused 状态表示安全阻断 |

## 2. 来源、医学判断与采纳用语

ConditionEntry 的来源、AI 医学判断和用户采纳互不混同，完整契约见 [domain §3](./domain/diagnosis.md#3-diagnosis)。

| 标准用语 | 含义 | 对应模型 |
| --- | --- | --- |
| 疑似（AI 分析） | Agent 的证据评估认为该诊断值得考虑但不确定；不表示临床确诊 | assessment.level = suspected |
| 较倾向支持（AI 分析） | Agent 对现有证据的定性评估较支持该诊断；不是校准概率或临床确诊 | assessment.level = likely |
| 医生诊断声明 | 医疗记录中的准确诊断措辞，可能含“考虑”“倾向”等保留意见；不必自动拥有 AI assessment | SourceClaimRef |
| 临床文件记载 | 已通过证据准入、且明确记载医生具名诊断的文件声明；仅有数值或图像测量不算医生诊断 | sourceKind = clinical_document |
| 用户转述医生意见 | 用户说医生如此诊断，不等于有文件支持 | sourceKind = user_reported_clinical |
| Agent 分析 | 正式用户请求下由 AI 形成的医学诊断候选及评估；AI 假设只在 Run 审计 | sourceKind = agent_analysis |
| 用户本人怀疑 | 用户明确提出的医学猜测，作为 BodyHypothesis 自动入库；不自动创建 ConditionEntry | sourceKind = user_suspicion |
| 采纳 | 用户接受提案的医学名称、证据边界与建议纳入当前 Diagnosis；不调整医学确定程度 | Decision = accept；disposition = adopted |
| 不采纳 | 拒绝本次提案，不等于医学排除，也不撤回已有条目 | Decision = reject |
| 停用 | 曾采纳但当前不再纳入有效诊断；理由可为撤回、被新结论取代、不再适用等 | disposition = inactive；DispositionChange |
| 撤回 | 用户明确要求将具名诊断退出当前采纳集合，经准确轻量确认 | ConditionDirectEdit.withdraw |
| 继续补充 | 暂不决定，新增事实先写 BodyState，再作为本 Run 证据复核提案 | pending，无 Decision |
| 需要复核 | 与诊断有关的新证据或来源资格变化；只是提示，不等于诊断失效 | 派生 reviewRecommended |
| 症状经历 | 按身体事实推导的 unknown/ongoing/ended；不再为 Diagnosis 建通用生命周期 | BodyState.SymptomEpisode |
| 正式分析 | 用户明确请求后进行、可能产生具名诊断提案的运行 | analyze_conditions → ConditionAnalysisRun |
| 分析安全阻断 | 正式分析遭遇需要停止分析的严重风险，运行终结、无局部 Diagnosis 提交 | ConditionAnalysisEvent.safety_blocked |
| 初步解释 | 普通 Chat 中讨论鉴别方向，不进入正式 Diagnosis | — |

用户采纳 AI 分析不等于医生确诊；采用临床来源也不代表用户重新进行了一次医学诊断。

## 3. Penpot / 旧代码名称对照

Penpot 文件 **BodySense · Product Design**（`24d9d841-759d-81bc-8008-b6423250c2b8`）的 Product 页（`ed4c6ec0-8f91-80cb-8008-c1e744bad64b`）仍沿用部分旧命名。阅读设计稿或旧代码时按下表换算：

| Penpot / 旧代码名称 | 新规格术语 | 说明 |
| --- | --- | --- |
| Assistant（Window/Assistant、Workflow/Assistant *） | Chat | 回答、空态、重试等状态沿用 |
| Assessment（Window/Assessment、Second condition、Collapsed） | Diagnosis | 「Assessment」在设计稿与旧代码中即 Diagnosis |
| Workflow/Assessment Input / Evidence / Evidence Empty / Upload Error / Result | —（旧流程） | 「上传 → 证据 → 结果」流程已废弃：上传资料归 Person 的 Media / Reports，分析在 Chat 中发起 |
| Workflow/Assessment Safety Hold | 安全阻断状态 | **保留**，表达安全阻断与就医提示 |
| Profile（Window/Profile、Profile/Category *、Workflow/Profile *） | Person | 五个分类 Records / Equipment / Media / Reports / Habits 沿用；身份信息在标题区与 Identity Popover |
| Training（Window/Training、Plan Legs / Functional / Picker） | TrainingPlan | 多份计划与计划选择器 |
| Workflow/Training Today / Draft / Move / Feedback / Paused / Completed | Trainer + Plan Draft | Today/Move/Feedback/Paused/Completed 属于执行视图；**Draft 是未接受的计划预览**，原型需改到创建/确认路径（TP-D2） |
| TrainingLog（旧代码） | TrainingSessionLog | 新规格以课模板与执行记录区分 |
| Treatment（旧代码，单一方案 + revisions） | TrainingPlan + PlanProposal | 新规格为多份计划、提案需用户接受 |
| Consultation / Consultation Session（旧代码） | Chat / Conversation | 不再以问诊会话为业务中心 |
| DiagnosisAnalysis / candidates（旧代码，按问诊生成） | ConditionAnalysisRun + ConditionProposal + ConditionProposalDecision → ConditionEntry | DiagnosisAnalysis 这个名字只保留给旧代码，新规格不再使用 |
| `certainty`、V1 单个 `source` / `conclusionStatus` / `lifecycle` | V2 多来源声明 + 可选 AI assessment + disposition + 用户 Decision | 旧字段需迁移并保留历史原意；V1 的 clinical_diagnosis 不再是 AI 确定程度，V1 的用户 likely/keep_as_suspected 不能当作医学证据等级 |
| Assessment report（旧代码） | Diagnosis | 不再生成按次的评估报告 |

## 4. 状态词的适用范围

- state/status/lifecycle：只属于对应业务对象，完整取值与转换详见 [领域契约](./domain/README.md)。
- UI state：窗口的 loading、empty、error、focus、minimized 等表示层状态，不持有身体事实或用户授权。
- Proposal decision：对准确提案及内容指纹的决定；关闭 UI 不等于 rejected。
- SafetyRestriction：独立于 TrainingPlan.status 的执行限制，不能静默重写计划。
- Person：组合读取窗口，不是新建的身体状态存储聚合。
- 待决语义：详见 [domain-decisions.md](./domain-decisions.md)，不能由旧代码名称反推。
