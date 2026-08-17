# 「体悟」(BodySense) 体态健康助手 — 产品需求文档

> 文档版本：v2.0  
> 更新时间：2026-08-15  
> 状态：Active  
> 领域模型：[Longitudinal BodyState Domain Model](./architecture/longitudinal-body-state-domain.md)  
> 功能规格：[Longitudinal Body Health Workspace](./feature_spec_longitudinal_body_health.md)

---

## 1. 产品概述

### 1.1 产品简介

BodySense 是一款面向长期身体状态理解、体态改善与运动康复辅助的 AI 健康应用。

它不是一次性的“问诊报告生成器”，而是一个持续工作的个人身体状态空间：用户可以长期和 AI 对话，系统会把用户的身体感受、生活习惯、自测、体态分析、训练反馈以及后续变化逐步沉淀成一份持续演化的身体状态模型，并在需要时生成诊断可能性分析、改善方案和趋势判断。

核心闭环：

```text
持续交流 / 观察 / 训练
        ↓
Longitudinal BodyState
        ↓
Diagnosis Analysis
        ↓
Current Treatment / Improvement Plan
        ↓
训练、行为与结果反馈
        ↓
BodyState 更新
        ↓
重新分析与调整
```

### 1.2 一句话定位

**你的长期 AI 身体状态顾问——持续理解身体变化，分析可能问题，并根据反馈动态调整改善方向。**

### 1.3 核心价值

- **降低表达门槛**：用户可以使用自然语言描述“酸、紧、怪、不舒服”，AI 在需要时帮助转化为更准确的健康描述。
- **长期身体记忆**：系统记住的是结构化身体状态及其变化，而不仅是聊天记录。
- **时间维度分析**：能理解“以前有、后来好了、最近复发”“开始跑步两周后出现膝部不适”等演变关系。
- **证据驱动**：咨询过程中按需查询受控知识库并提供 citation；诊断综合已有事实、观察、证据和时间变化。
- **持续闭环**：Diagnosis 和 Treatment 不是一次性终点，会根据后续身体状态、训练执行和 outcome 重新评估。
- **可解释与可修正**：用户可以直接修改系统记录的身体状态；AI 推测和用户事实必须明确区分。

### 1.4 产品边界

BodySense 提供健康信息整理、体态/运动相关可能性分析、知识解释和改善建议，不替代专业医疗诊断或治疗。

当出现安全相关信息时，安全流程优先于普通体态分析和训练建议。

---

## 2. 目标用户

### 2.1 核心用户

**久坐办公 / 学习人群**

- 长时间电脑、手机或学习；
- 关注头前伸、圆肩、腰臀僵硬等体态问题；
- 希望了解日常习惯和身体变化之间可能的关系。

**运动 / 健身人群**

- 跑步、力量训练、球类等；
- 关注活动度、不适、训练后疼痛或动作代偿；
- 希望长期追踪训练与身体反馈。

**身体状态改善需求人群**

- 有轻度或慢性不适；
- 不一定具备专业康复知识；
- 希望先理解自身情况，并在必要时知道何时应寻求专业帮助。

### 2.2 用户特征

- 通常不能一开始就用专业术语描述身体问题；
- 身体状态会随时间、运动、工作和作息不断变化；
- 不希望频繁创建和管理很多“咨询会话”或“报告文件”；
- 更希望系统长期记住并帮助梳理变化；
- 需要解释“为什么系统这样判断”。

---

## 3. 核心产品模型

### 3.1 一个长期健康空间

产品层面，一个用户拥有一个长期健康工作区。

核心 UI：

```text
左侧：长期 AI 对话
右侧：当前身体状态工作台
```

用户不需要为不同身体问题频繁新建 Conversation。

系统内部可以使用多个 Turn、Run、checkpoint、Concern、BodyStateRevision 等对象，但这些不需要成为用户管理负担。

### 3.2 Conversation 不等于身体档案

Conversation 保存交互过程。

用户当前身体状态由独立的 Longitudinal BodyState 表达。

当旧聊天和用户后来明确修正的信息冲突时，当前结构化 BodyState 应成为后续 AI 推理的业务依据。

### 3.3 BodyState 是长期核心内容

BodyState 持续记录：

- 当前不适 / 症状；
- negative findings；
- 体态与活动观察；
- 自测结果；
- 生活方式与运动习惯；
- 历史伤病；
- 安全状态；
- 最近变化；
- AI 待验证 Hypothesis；
- 时间趋势。

用户看到的是当前状态和可理解的历史变化，不需要手动管理内部 revision。

---

## 4. 功能模块一：长期健康咨询工作台

这是 BodySense 的核心入口。

### 4.1 左侧 — 长期对话区

支持：

- 自然语言描述身体感受；
- 各种体态 / 康复 / 训练相关问题；
- AI 主动追问关键缺失信息；
- RAG 知识检索和 citation；
- 安全筛查；
- 自测指导；
- 解释当前身体状态、Diagnosis 和 Treatment。

### 4.2 专业表达辅助

用户说：

> “小腿怪怪的。”

AI 可以引导：

> 更接近紧绷、酸胀、刺痛、麻木、无力、灼热，还是其他感觉？

用户无需提前完成专业课程。

产品可以提供可选基础知识：

- 身体平面；
- 常见疼痛/不适描述；
- 身体部位与 landmark；
- 自测说明。

主要体验采用 just-in-time education。

### 4.3 右侧 — BodyState Workbench

建议展示：

```text
当前身体状态
身体部位 / Concern
症状与不适
negative findings
姿态/自测 Observation
生活/运动因素
最近变化
AI Hypotheses
Safety
Latest Diagnosis
Current Treatment
```

用户可以直接修正结构化信息。

### 4.4 修正与变化

系统应区分：

**修正：**

> “刚才说错了，是左边不是右边。”

和：

**身体后来变化：**

> “以前左边疼，现在已经好了。”

前者表示旧记录错误；后者表示历史状态曾经真实存在。

---

## 5. 功能模块二：长期身体状态与趋势

### 5.1 自动更新

用户无需点击“保存咨询结果”。

经过接受/确认的身体信息自动进入长期 BodyState，并形成系统内部 revision。

### 5.2 时间追踪

产品应能回答：

- 这个问题什么时候出现？
- 最近是改善还是恶化？
- 是否曾完全缓解后又复发？
- 运动、久坐或作息是什么时候改变的？
- 某个 Diagnosis 当时基于什么身体状态？
- 某个 Treatment 执行后身体发生了什么变化？

### 5.3 多来源状态

身体状态不仅来自聊天，也可以来自：

- 用户右侧手动编辑；
- ask_user 结构化回答；
- 体态照片分析；
- 自测；
- 训练反馈；
- 上传的报告；
- 未来设备数据。

这些来源必须保留 provenance。

---

## 6. 功能模块三：Diagnosis 可能性分析

### 6.1 触发

用户可以在工作台中点击生成当前 Diagnosis Analysis。

Diagnosis 本身不要求独立聊天页面。

### 6.2 输入

Diagnosis 基于：

- 当前明确 BodyState snapshot/revision；
- 相关时间历史；
- Observations；
- 已收集 Evidence；
- Safety 信息。

默认不重新把所有历史问题 broad RAG 一遍。

遇到新 Concern、Evidence gap、冲突或安全不确定性时，可进行 targeted retrieval。

### 6.3 Candidate 数量

**不设置固定最大数量。**

简单问题可能只有 1 个 Candidate。

长期全身状态可能有 7、8 个甚至更多 Candidate。

信息不足或 Safety 阻塞时，也允许 0 个 Candidate，并明确说明原因。

### 6.4 按 Concern 展示

复杂分析应按身体 Concern/区域组织，而不是只做一个 Top-N 列表。

示例：

```text
头颈
- 头前伸姿态倾向
- 其他相关可能性

臀髋
- 久坐相关功能性不适倾向
- 活动度相关问题

膝踝
- 跑步相关膝部负荷问题
- 踝活动受限相关表现
```

### 6.5 Candidate 解释

Candidate 应逐步支持：

- 名称；
- confidence；
- 当前 impact / severity（适用时）；
- 支持事实；
- 支持 Observation；
- Evidence / citations；
- counterevidence；
- differential；
- missing information；
- safety notes。

### 6.6 用户判断

用户可对每个 Candidate 标记：

- 符合我的情况；
- 不确定；
- 目前不符合。

完整 DiagnosisAnalysis 始终保留，未选 Candidate 不删除。

---

## 7. Diagnosis 历史

Diagnosis 历史承担“历史分析记录”的主要产品角色。

用户可以查看：

- 每次分析时间；
- 当时的 BodyState revision；
- 当时所有 Candidate；
- 用户当时的确认/不确定状态；
- 后续分析相对之前发生了什么变化。

例如：

```text
8 月：臀腿问题较明显，头颈中度倾向
10 月：臀腿明显改善，新增右膝问题
11 月：臀腿轻度复发，原“久坐主因”假设证据减弱
```

产品不再要求额外生成一个 MedicalRecord 来重复这些信息。

---

## 8. 功能模块四：Treatment / Improvement Plan

### 8.1 当前方案

用户主要看到一个 Current Treatment / Improvement Plan。

方案可包含：

- 训练动作；
- 活动度练习；
- 日常习惯建议；
- 训练频率和强度；
- 注意事项；
- 复评条件；
- Safety 提示。

### 8.2 输入

方案应关联：

- BodyState revision；
- DiagnosisAnalysis；
- 用户约束 / 偏好；
- Safety；
- 相关 Evidence。

### 8.3 方案不是静态终点

Treatment 可以：

- active；
- review recommended；
- paused；
- superseded；
- completed。

新身体信息不会让 AI 静默修改当前已接受方案；重要变化应触发 review。

---

## 9. 功能模块五：训练、反馈与 Outcome

### 9.1 执行

用户可以查看和完成当前训练任务。

记录：

- 是否完成；
- 训练量；
- 难度；
- 是否出现不适；
- 主观感受；
- adherence。

### 9.2 Outcome 回流

例如：

```text
久坐 8h/day -> 4h/day
左臀酸胀 中 -> 轻 -> 缓解
开始跑步两周 -> 新增右膝不适
```

这些变化回到 BodyState，成为下一次 Diagnosis / Treatment Review 的输入。

### 9.3 相关性而非自动因果

产品可以表达：

> “症状改善与这段时间训练和久坐减少同时发生。”

不能仅凭时间顺序直接断言：

> “这个动作已经证明治好了问题。”

---

## 10. Safety

安全优先级高于普通体态咨询、Diagnosis 和 Treatment。

当出现重要安全信号时，系统可能：

- 追问关键安全信息；
- 输出安全提示；
- 暂停普通 Diagnosis；
- 标记当前 Treatment 需要 review / pause；
- 建议用户寻求专业医疗评估。

Safety 状态必须在右侧工作台明确展示。

---

## 11. 知识库与 Evidence

### 11.1 Knowledge Base

知识库用于：

- 健康知识解释；
- 自测说明；
- 体态和运动知识；
- 训练动作说明；
- Diagnosis / Treatment 的 Evidence 支撑。

### 11.2 Citation

对用户产生的重要知识性解释和分析尽可能提供 citation。

Citation 是 Evidence 的用户呈现，不应被误当成用户自身 Fact。

### 11.3 Knowledge Gap

知识不足时系统应明确表示不确定或需要更多信息，而不是伪造依据。

---

## 12. 页面与导航

目标核心入口：

1. **Health Workspace** — 长期 Conversation + BodyState Workbench。
2. **Diagnosis History** — 历史 Diagnosis 和对比。
3. **Treatment / Training** — 当前方案、训练执行、反馈。
4. **Body Trends** — 身体状态、活动和 Outcome 的长期趋势。
5. **Profile / Data** — 基础资料、上传、隐私设置。

这些入口第一版可以是同一工作台内的 Tab / 子路由，不要求立即拆成多个独立产品页面。

---

## 13. 历史与报告

### 13.1 不使用 MedicalRecord 作为核心业务对象

用户长期历史由：

- BodyState Timeline；
- Diagnosis History；
- Treatment History；
- Training / Outcome History；

共同表达。

### 13.2 HealthReport（未来可选）

如果用户需要下载、打印或分享，可以从选定状态动态生成 HealthReport。

HealthReport 是 export snapshot，不是另一份需要同步维护的 health truth。

---

## 14. 数据与隐私原则

- 用户拥有并可查看自己的身体状态数据。
- 用户可修正错误信息。
- “修正信息”“排除 AI 后续使用”“隐私永久删除”应被区分。
- 敏感健康数据传输和存储必须满足项目安全要求。
- AI 对话和健康数据不应在无授权情况下用于模型训练。
- 历史 provenance 的保留必须服从隐私和数据删除策略。

---

## 15. 技术架构原则

### React

负责：

- Conversation / BodyState / Diagnosis / Treatment presentation；
- 用户编辑和操作 intent；
- projection-driven UI。

### Go

负责：

- 鉴权；
- durable BodyState business truth；
- Diagnosis / Treatment identity 与业务状态；
- Runtime Event Log / projections；
- 并发/revision/business policy。

### Python

负责：

- LangGraph Agent Runtime；
- LLM / RAG / tool reasoning；
- BodyState mutation proposal；
- Diagnosis / Treatment typed reasoning。

详细 ownership 以 ADR 0002 和 ADR 0004 为准。

---

## 16. 核心验收标准

- 用户无需创建多份 consultation，也能持续管理不同身体问题。
- 对话与右侧工作台共享同一 BodyState truth。
- 用户修正后的信息下一轮 AI 能正确使用。
- 历史真实状态不会因当前改善而被覆盖。
- 系统能表示 resolved / recurrence / trend。
- Diagnosis 不限制最多 3 个 Candidate。
- Diagnosis 历史能够说明每次分析基于什么身体状态。
- 未确认 Candidate 不会丢失。
- Treatment 能因后续身体变化进入 review 状态。
- 训练 / 日常行为 Outcome 能反馈到 BodyState。
- Safety 能阻止不合适的普通分析/方案路径。
- 长期 Conversation 不要求把全部历史消息塞给模型。

---

## 17. 医疗免责声明

产品 UI、Diagnosis、Treatment 和可导出的健康报告均应明确：

> BodySense 提供的分析、评估和改善建议仅供健康管理和信息参考，不构成医疗诊断或治疗方案。如存在持续或严重疼痛、明显麻木/无力、急性损伤或其他令人担忧的情况，请及时寻求合格医疗专业人员的评估。
