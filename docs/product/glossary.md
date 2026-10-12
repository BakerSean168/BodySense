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
| **Diagnosis** / 综合诊断 | 每位用户唯一的当前综合结果，只由用户确认过的 ConditionEntry 组成 | [requirements §2.3](./requirements.md#23-diagnosis-综合诊断) · [domain §3](./domain/diagnosis.md#3-diagnosis) | 综合分析、评估、评估报告、Assessment、问诊诊断、诊断报告 |
| **ConditionEntry** / 病情条目 | Diagnosis 内具有稳定身份的单个病情：名称、原因分析、处理方案、来源（`source`）、结论状态（`conclusionStatus`）、确认记录（`confirmation`）、病程（`lifecycle`）与采纳状态（`disposition`） | [domain §3](./domain/diagnosis.md#3-diagnosis) | 候选病情、诊断候选、局部诊断报告；`certainty`（已拆分的旧字段） |
| **ConditionAnalysisRun** / 正式分析运行 | 一次正式分析的追加式审计：请求、范围、底版修订、证据、AI 假设（创建时快照 + 之后追加的 `hypothesis_added`，均无权威）、产生的提案。创建时写入的基础记录与审计事件不可变；当前状态（open / committed / closed_without_change）是由事件推导的投影，按顺序回放基础记录与事件可完全重现 | [domain §3.1](./domain/diagnosis.md#31-正式分析审计对象) | DiagnosisAnalysis（新规格中不再使用此名） |
| **ConditionProposal** / 病情提案 | 一次运行提出的单个条目新增/更新/结束，带内容指纹与证据 ID，尚待用户在 Chat 中决定；结束提案须写明结束目标（`endTarget`：标记缓解 / 不再适用）与原因 | [domain §3.1](./domain/diagnosis.md#31-正式分析审计对象) · [agent-workflows §3.2](./agent-workflows.md#32-diagnosis-正式分析与病情确认) | 已确认诊断、初步诊断（当作已写入时） |
| **ConditionProposalDecision** / 用户决定记录 | 用户对某个提案的决定（含拒绝）的追加式审计；用于「不重复提议」判定 | [domain §3.1](./domain/diagnosis.md#31-正式分析审计对象) | 「否定记录」（未定义存储时的旧说法） |
| **ConditionDirectEdit** / 直接编辑记录 | 用户明确要求更正、撤回或标记缓解某个条目时，经轻量确认写入的用户决定审计（谁、什么、何时、原因）；不经正式分析 | [requirements R-DX-13](./requirements.md#23-diagnosis-综合诊断) · [domain §3.5](./domain/diagnosis.md#35-用户直接更正撤回与标记缓解) | 用「撤回提案」指直接编辑 |
| **TrainingPlan** / 训练计划 | 一份完整训练处方（热身 → 正式训练 → 必要放松，含组数、次数/时长、频率）；每位用户可有多份 | [requirements §2.4](./requirements.md#24-trainingplan-训练计划) · [domain §4](./domain/training.md#4-trainingplan) | Treatment、治疗方案、训练模块、部位模块 |
| **PlanChangeProposal** / 计划调整提案 | Agent 主动提出的某份计划的具体修改，用户「接受调整 / 暂不修改」前不生效 | [domain §4](./domain/training.md#4-trainingplan) | Treatment revision、方案修订（指提案时） |
| **Trainer** / 训练执行 | 打开某份 TrainingPlan 进行训练、计时与反馈的执行窗口 | [requirements §2.5](./requirements.md#25-训练执行与反馈) · [interface §2](./interface.md#2-业务窗口) | 训练流程（Workflow/Training，设计稿旧名） |
| **PlanSessionTemplate** / 课模板 | 计划内容中「一次课」的模板，含热身/正式/放松阶段；单课计划只有一个 | [domain §4](./domain/training.md#4-trainingplan) | TrainingSession（指计划内容时）、exerciseStages（已移入模板） |
| **TrainingSessionLog** / 训练记录 | 一次实际执行：按计划、日期与动作记录用户明确完成/跳过的情况与主观反馈 | [requirements R-EX-04](./requirements.md#25-训练执行与反馈) · [domain §4](./domain/training.md#4-trainingplan) | TrainingLog（旧代码名）；把课模板与训练记录混称 |
| **Person** / 个人档案 | 展示个人资料与身体记录、器材、资料等的窗口与业务范围；五个分类 Records / Equipment / Media / Reports / Habits，身份在标题区与 Identity Popover | [requirements §2.6](./requirements.md#26-person-个人档案与当前状态) · [interface §3](./interface.md#3-person-个人档案) | Profile 窗口（设计稿旧名） |
| **Profile** / 基础资料 | Person 中相对稳定的身份与偏好数据（领域对象），显示在 Person 标题区与 Identity Popover；不含有时间语义的测量与习惯 | [domain §1](./domain/README.md#1-聚合关系) · [R-PS-04](./requirements.md#26-person-个人档案与当前状态) | 用 Profile 指整个 Person 窗口；把 Profile 当作第六个分类标签页 |
| **Chat** | 自然表达、提问、发起分析与授权计划修改的对话窗口，承载交互工具 | [interface §2](./interface.md#2-业务窗口) | Assistant（设计稿旧名）、Consultation / 问诊会话 |
| **Conversation** / 对话 | Chat 中的交流与工具活动记录；不是身体事实的权威存储 | [domain §1](./domain/README.md#1-聚合关系) | Consultation Session（旧实现） |
| **SafetyState / 安全阻断** | BodyState 中的权威风险状态：执行限制的风险等级、范围（计划/动作）、有效期、解除策略（`liftPolicy`：到期自动解除 / 用户复核 / 专业人员放行）与解除条件，Trainer 据此把关；界面显示「暂不宜执行」；不等于修改计划内容 | [requirements R-SAFE-03](./requirements.md#27-安全) · [SafetyState](./domain/safety.md#23-safetystate-与执行限制) | 安全修订（不存在此概念）；用计划的 paused 状态表示安全阻断 |

## 2. 来源、结论与确认用语

ConditionEntry 把旧 `certainty` 拆成独立维度（[domain §3](./domain/diagnosis.md#3-diagnosis)）：

| 标准用语 | 含义 | 对应字段 |
| --- | --- | --- |
| 大概率是 | 用户在确认工具中选择「大概率是这个」：接受把这一可能性记为「大概率」，**不是临床诊断** | `conclusionStatus = likely`，`confirmation.decision = likely` |
| 疑似 · 你选择保留 | 用户选择「可能是，但不确定，先保留」；仍属于用户确认过的状态 | `conclusionStatus = suspected`，`confirmation.decision = keep_as_suspected` |
| 纳入我的病情管理 | 临床来源病情经用户同意纳入个人管理；不是对医生诊断的再判断 | `conclusionStatus = clinical_diagnosis`，`confirmation.decision = include_clinical` |
| 不是 | 用户选择「不是这个」：不采纳本次提案，**不等于医学上已排除**；针对已有条目时只拒绝这次变更，条目不变 | `ConditionProposalDecision.decision = rejected` |
| 同意结束 | 用户同意结束提案写明的结束目标：标记为已缓解或不再适用；只应用该目标 | `ConditionProposalDecision.decision = accept_end` → `lifecycle = resolved`（`mark_resolved`）或 `disposition = no_longer_applicable`（`mark_no_longer_applicable`） |
| 撤回 | 用户明确要求把某个条目从当前诊断中去掉，经轻量确认生效；条目与历史保留 | ConditionDirectEdit（`withdraw`）→ `disposition = withdrawn_by_user` |
| 继续补充 | 用户暂不决定，补充信息；Diagnosis 不变，提案保持待定；新信息作为本运行新证据，提交前复核已决定的提案 | 无决定记录 |
| 临床文件记载 | 有经证据管线采纳、明确记载临床医生诊断的医疗文件；只含化验 / 影像数值的文件是分析证据，不算临床文件记载 | `source = clinical_document` |
| 用户转述的临床诊断 | 用户说「医生告诉我……」，无文件支持 | `source = user_reported_clinical` |
| Agent 分析 | Agent 在正式分析中得出 | `source = agent_analysis` |
| 用户的怀疑 | 源自用户本人提出的假设 | `source = user_suspicion` |
| 病程 | 病情的真实发展：进行中 / 观察中 / 已缓解 | `lifecycle = active / monitoring / resolved` |
| 采纳状态 | 该解释是否仍被采纳：采纳 / 用户撤回 / 不再适用 | `disposition.state = adopted / withdrawn_by_user / no_longer_applicable` |
| 用户认可 Agent 分析 | 用户同意纳入个人管理记录，**不等于临床确诊** | — |
| 正式分析 | 用户请求/授权后进行、可能写入 Diagnosis 的分析 | `analyze_conditions` → ConditionAnalysisRun |
| 分析安全阻断 | 正式分析中出现需要停止分析的严重风险（警示 / 需尽快就医类），运行终止、不写入 Diagnosis；与 Trainer 的执行限制（SafetyState）不是一回事 | `ConditionAnalysisEvent.safety_blocked` → `outcome = safety_blocked` |
| 初步解释 | 普通 Chat 中对可能方向的讨论，不写入 Diagnosis | — |

「评估」只用于描述医生或专业人员的线下诊治（如「建议就医进行专业检查」），不得用于指代 Diagnosis 或 Agent 的分析。

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
| Workflow/Training Today / Draft / Move / Feedback / Paused / Completed | Trainer | 训练执行各状态（[interface §5.1](./interface.md#51-trainer-训练执行)）；Draft 保留，含义待 Penpot 核对（[Q-15](./open-questions.md)） |
| TrainingLog（旧代码） | TrainingSessionLog | 新规格以课模板与执行记录区分 |
| Treatment（旧代码，单一方案 + revisions） | TrainingPlan + PlanChangeProposal | 新规格为多份计划、提案需用户接受 |
| Consultation / Consultation Session（旧代码） | Chat / Conversation | 不再以问诊会话为业务中心 |
| DiagnosisAnalysis / candidates（旧代码，按问诊生成） | ConditionAnalysisRun + ConditionProposal + ConditionProposalDecision → ConditionEntry | DiagnosisAnalysis 这个名字只保留给旧代码，新规格不再使用 |
| `certainty`（旧规格字段） | `source` + `conclusionStatus` + `confirmation` | `user_confirmed_agent_analysis` → `agent_analysis` + `likely`；`user_kept_as_suspected` → `conclusionStatus = suspected`；`clinically_documented` → `source = clinical_document`；`user_reported_clinical_diagnosis` → `source = user_reported_clinical` |
| Assessment report（旧代码） | Diagnosis | 不再生成按次的评估报告 |

## 4. 状态词的适用范围

- state/status/lifecycle：只属于对应业务对象，完整取值与转换详见 [领域契约](./domain/README.md)。
- UI state：窗口的 loading、empty、error、focus、minimized 等表示层状态，不持有身体事实或用户授权。
- Proposal decision：对准确提案及内容指纹的决定；关闭 UI 不等于 rejected。
- SafetyRestriction：独立于 TrainingPlan.status 的执行限制，不能静默重写计划。
- Person：组合读取窗口，不是新建的身体状态存储聚合。
- 待决语义：详见 [domain-decisions.md](./domain-decisions.md)，不能由旧代码名称反推。
