# BodySense · Person 与 Chat/Agent 领域边界

**性质：目标业务契约；不是当前代码数据库 Schema。** 这里明确已经批准的对象归属、交互状态与生命周期下限。缺乏产品决策的完整属性和状态枚举不在此擅自确定。参见 [需求 R-PS / G-02](../requirements.md)、[Agent 工作流](../agent-workflows.md)、[界面](../interface.md)、[领域决策登记](../domain-decisions.md)。

## 1. 聚合与职责

| 概念 | 类型/职责 | 唯一权威及隔离边界 |
| --- | --- | --- |
| User | 用户身份与授权主体 | 所有健康、资料和对话操作必须绑定当前 userId；G-04 |
| Profile | 相对稳定的身份、偏好 | Person 中的基础资料；带日期的体征、习惯和病程属于 BodyState |
| Person | 组合查询视图，而非第二份身体事实 | 汇集 Profile、Records、Equipment、Media、Reports、Habits，不复制 BodyState 或 Diagnosis 的权威真值 |
| Records | 身体记录的只读或经授权更正入口 | 真实记录归 BodyState/Diagnosis 等原聚合 |
| Habits | 有时间语义的生活习惯视图 | 由 BodyFact(kind=lifestyle) 等权威来源形成，新增/纠正途径待决 DC-007 |
| Equipment | 用户拥有/可使用器材 | 资产记录，必须有归属及创建/修改授权；完整字段和失效规则待决 DC-023 |
| Media / Reports | 用户资料与提取/复核状态 | 原始上传受私有归属约束；提取候选不直接成为 BodyObservation |
| Conversation | 对话及工具活动的持久通道 | 可追踪消息、Agent 运行、待回复交互；不是 BodyState 的唯一存储 |
| PendingInteraction | 挂起的人机确认/追问 | 必须绑定稳定 interactionId、所属用户、关联消息、具体业务对象；关闭窗口不等于拒绝 |

不能用 Person UI 的 Records/Habits 分类倒推出新的存储聚合，也不能让 Conversation 持有唯一身体事实（G-02）。

## 2. 当前已批准的状态语义

| 对象 | 已确定的状态/事实 | 允许的转换与后果 | 未决细节 |
| --- | --- | --- | --- |
| Profile | 身份资料可查看/编辑，显示于窗口标题及 Identity Popover | 经授权更新；不得把有时间语义的体征塞进 Profile | 身份字段及隐私显示 DC-015 |
| Media / Reports 证据候选 | admissible / needs_review / rejected（沿用证据管线） | 可靠抽取可按管线自动准入；needs_review 需核对；rejected 不能作为可信临床证据；原机器候选不可被审核结果原地改写 | 资料资产自身的上传/删除/保留状态 DC-023 |
| Habits/Records | 使用真实当前数据，含明确空态 | 不因打开 Person 窗口制造记录，不用演示值 | 是否允许自动记录及候选审核 DC-007/008 |
| Conversation | 历史消息和工具活动可追溯 | 离开/收起 Chat 后，待回复业务交互不丢失 | 单/多会话、归档/删除生命周期 DC-016 |
| PendingInteraction | pending（业务待回复的稳定身份）；已回答/拒绝、失效是语义事件 | accept、reject、close、resume、expire、retry、stale、fail 均遵循 [Agent 工作流 §4](../agent-workflows.md#4-交互工具状态)；close 不是 reject | 完整运行时存储结构、输入是否可并行补充 DC-017；提案超时 DC-018 |

**不要混淆**：打开、拖动、收起、加载等是窗口 UI 状态；Conversation 消息及 Agent Run 是持久运行状态；ConditionProposal/PlanChangeProposal 属于原领域的业务状态。不得把一个全局 UI `status` 同时用来表示三者。

## 3. 已确定的数据流与权限

1. 用户在 Chat 提供事实：Agent 解析后由 Go 业务服务校验来源、主体、时间和幂等性，写入 BodyState；仅在写入成功后反馈成功。
2. 用户在 Person 上传报告：原始资料归属私有资产；经 OCR/解析产生的候选进入准入或人工复核流程。仅达标的可信结果进入 BodyObservation；不存在「上传即诊断」。
3. 用户请求分析：Conversation 承载请求及 confirm_condition；用户决定记录属于 ConditionProposalDecision，不由聊天文本代替。
4. 关闭对话、切换业务窗口、返回人体选区，不得静默接受或拒绝业务授权，也不得使待回复交互失去身份。
5. 访问其他用户的资料、互动目标、训练计划、诊断条目必须被后端拒绝；不允许依靠前端过滤作为权限边界。

## 4. UI 精细化需要的状态

| UI | 必须可区分的状态 | 来源 |
| --- | --- | --- |
| Person 五标签页 | loading / error / empty / populated | [UI 与交互 §3](../interface.md#3-person-个人档案) |
| Reports 候选值 | admissible / needs_review / rejected；核对中 / 操作失败 | [证据管线](../../architecture/health-document-evidence-pipeline.md) |
| Chat 输入及工具卡 | 等待用户决定、继续补充、恢复上下文、交互过期、业务底版过时、工具调用失败 | [工作流 §4](../agent-workflows.md#4-交互工具状态) |
| Diagnosis 确认 | 提案 pending、已决定但未提交、已生效、被新证据取代、安全阻断 | [Diagnosis](./diagnosis.md) |
| 用户资料 | 当前内容、来源、最近写入是否成功；内部修订号默认隐藏 | R-PS-02 |

具体文案、组件、输入可用性与页面布局以 [Q-19](../open-questions.md)、[DC-015](../domain-decisions.md)、[DC-017](../domain-decisions.md) 讨论并在 Penpot 定稿。此处不新增 UI 模拟业务真值。

## 5. 生命周期缺口（不得假设已有定义）

- Profile：必填字段、同意/修改/清除身份信息及脱敏规则（DC-015）。
- Equipment：新增、删除、停用、历史使用的实体属性和生命周期（DC-023）。
- Media/Reports：上传、处理中、解析失败、重处理、删除及删除对证据溯源的影响（DC-023）。
- Conversation：创建、多会话切换、删除/归档、恢复挂起交互的身份关系（DC-016）。
- PendingInteraction：在待回复时继续发送普通聊天或补充材料的有效路径（DC-017）。
- Habits/Records：自动记录与人工审核的边界（DC-007/008）。

这些对象的完整字段表和状态机应在对应决策通过后补入此文档，再新增可执行契约测试；不能从旧代码或 Penpot 名称直接推断。
