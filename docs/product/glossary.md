# BodySense · 术语表

**本表统一产品文档、设计稿与代码讨论中的业务用语。** 写文档、Issue、测试名和 UI 文案时使用「标准术语」一列；「避免使用」一列中的说法不得再用来指代同一概念。代码标识符保持英文。

## 1. 业务对象

| 标准术语（英文 / 中文） | 定义 | 规则所在 | 避免使用 |
| --- | --- | --- | --- |
| **BodyState** / 身体状态 | 每位用户唯一、持续更新的身体事实、观察、假设与真实病程集合；明确的本人陈述自动写入 | [requirements §2.2](./requirements.md#22-bodystate-身体状态) · [domain §2](./domain.md#2-bodystate) | 身体档案（指数据对象时）、问诊记录、会话状态 |
| **BodyFact** / 身体事实 | BodyState 中由用户自述、直接编辑或结构化回答形成的本人事实 | [domain §2](./domain.md#2-bodystate) | 症状记录（泛称时可用，但不作对象名） |
| **BodyObservation** / 观察 | 有方法与来源的测量、自测、检查或报告观察 | [domain §2.1](./domain.md#21-写入语义与证据分类) | — |
| **BodyHypothesis** / 假设 | AI 或用户提出的病因解释或可能疾病；不具备确诊权威 | [domain §2.1](./domain.md#21-写入语义与证据分类) | 候选诊断（DiagnosisAnalysis candidates，旧实现） |
| **Diagnosis** / 综合诊断 | 每位用户唯一的当前综合结果，只由用户确认过的 ConditionEntry 组成 | [requirements §2.3](./requirements.md#23-diagnosis-综合诊断) · [domain §3](./domain.md#3-diagnosis) | 综合分析、评估、评估报告、Assessment、问诊诊断、诊断报告 |
| **ConditionEntry** / 病情条目 | Diagnosis 内具有稳定身份的单个病情：名称、原因分析、处理方案、确定性与确认记录 | [domain §3](./domain.md#3-diagnosis) | 候选病情、诊断候选、局部诊断报告 |
| **病情提案**（confirm_condition 的内容） | Agent 分析得出、尚待用户在 Chat 中选择的病名/原因/处理方案 | [agent-workflows §3.2](./agent-workflows.md#32-diagnosis-正式分析与病情确认) | 已确认诊断、初步诊断（当作已写入时） |
| **TrainingPlan** / 训练计划 | 一份完整训练处方（热身 → 正式训练 → 必要放松，含组数、次数/时长、频率）；每位用户可有多份 | [requirements §2.4](./requirements.md#24-trainingplan-训练计划) · [domain §4](./domain.md#4-trainingplan) | Treatment、治疗方案、训练模块、部位模块 |
| **PlanChangeProposal** / 计划调整提案 | Agent 主动提出的某份计划的具体修改，用户「接受调整 / 暂不修改」前不生效 | [domain §4](./domain.md#4-trainingplan) | Treatment revision、方案修订（指提案时） |
| **Trainer** / 训练执行 | 打开某份 TrainingPlan 进行训练、计时与反馈的执行窗口 | [requirements §2.5](./requirements.md#25-训练执行与反馈) · [interface §2](./interface.md#2-业务窗口) | 训练流程（Workflow/Training，设计稿旧名） |
| **TrainingLog** / 训练记录 | 按计划、日期与动作记录的真实完成情况与主观反馈 | [domain §4](./domain.md#4-trainingplan) | — |
| **Person** / 个人档案 | 展示个人资料与身体记录、器材、资料等的窗口与业务范围；分类 Records / Equipment / Media / Reports / Habits | [requirements §2.6](./requirements.md#26-person-个人档案与当前状态) · [interface §3](./interface.md#3-person-个人档案) | Profile 窗口（设计稿旧名） |
| **Profile** / 基础资料 | Person 中相对稳定的身份与偏好数据（领域对象）；不含有时间语义的测量与习惯 | [domain §1](./domain.md#1-聚合关系) | 用 Profile 指整个 Person 窗口 |
| **Chat** | 自然表达、提问、发起分析与授权计划修改的对话窗口，承载交互工具 | [interface §2](./interface.md#2-业务窗口) | Assistant（设计稿旧名）、Consultation / 问诊会话 |
| **Conversation** / 对话 | Chat 中的交流与工具活动记录；不是身体事实的权威存储 | [domain §1](./domain.md#1-聚合关系) | Consultation Session（旧实现） |
| **SafetyState / 安全阻断** | 对不适当训练执行的独立限制与就医提示；不等于修改计划内容 | [requirements §2.7](./requirements.md#27-安全) | 安全修订（不存在此概念） |

## 2. 确定性与确认用语

| 标准用语 | 含义 | 对应字段 |
| --- | --- | --- |
| 大概率是 | 用户在病情确认工具中选择「大概率是这个」 | `certainty = user_confirmed_agent_analysis`，`confirmation.decision = likely` |
| 疑似 · 你选择保留 | 用户选择「可能是，但不确定，先保留」；仍属于用户确认过的状态 | `certainty = user_kept_as_suspected`，`confirmation.decision = keep_as_suspected` |
| 不是 | 用户选择「不是这个，不用保留」；不入库，记录否定 | 无 ConditionEntry |
| 继续补充 | 用户暂不选择，补充信息；Diagnosis 不变 | — |
| 用户转述的临床诊断 | 用户说「医生告诉我……」 | `user_reported_clinical_diagnosis` |
| 临床文件记载 | 有对应医疗材料或可靠来源 | `clinically_documented` |
| 用户认可 Agent 分析 | 用户同意纳入个人管理记录，**不等于临床确诊** | — |
| 正式分析 | 用户请求/授权后进行、可能写入 Diagnosis 的分析 | `analyze_conditions` |
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
| Profile（Window/Profile、Profile/Category *、Workflow/Profile *） | Person | 分类 Equipment / Media / Reports / Habits 沿用 |
| Training（Window/Training、Plan Legs / Functional / Picker） | TrainingPlan | 多份计划与计划选择器 |
| Workflow/Training Today / Draft / Move / Feedback / Paused / Completed | Trainer | 训练执行各状态 |
| Treatment（旧代码，单一方案 + revisions） | TrainingPlan + PlanChangeProposal | 新规格为多份计划、提案需用户接受 |
| Consultation / Consultation Session（旧代码） | Chat / Conversation | 不再以问诊会话为业务中心 |
| DiagnosisAnalysis candidates（旧代码，按问诊生成） | 病情提案 → ConditionEntry | 新规格中 DiagnosisAnalysis 仅指内部不可变的分析审计记录 |
| Assessment report（旧代码） | Diagnosis | 不再生成按次的评估报告 |
