# BodySense · 阶段三 Person / Chat + 跨领域模型最终评审（2026-10-13）

> **结论：产品级领域建模通过（PC-D1～PC-D9、CD-D1～CD-D4），准许回填权威规格；技术实施与上线验收尚未完成。**
>
> 本文件记录产品决定、证据边界与实施差异，**不与 [权威 Person/Chat 领域契约](../domain/person-chat.md) 并列作为第二份规则来源**。最初候选稿的多 Conversation、并行自由补充与“是否保留公开分享”建议已被正式决定取代；早期候选可通过 Git 历史查阅。本文整合基线为阶段二 Training/Safety 已决版本 `13d236f37`。

## 1. 已确认的 Person / Chat 决策

| ID | 产品决定 | 约束与影响 |
| --- | --- | --- |
| PC-D1 | 极简、可选 Profile 与 Preferences | 仅稳定的个人资料/显示偏好；身体测量、病史、习惯在 BodyState；时区/单位变更不重写历史 |
| PC-D2 | EquipmentAsset 为可使用器材资产 | 每周使用次数是 BodyFact；器材不可用只触发影响复核、不自动改变处方 |
| PC-D3 | 一个 UserAsset，Media/Reports 多视图 | 文件、ExtractionRun、证据准入三种生命周期分离 |
| PC-D4 | **每用户唯一持续 Conversation** | 不新建、切换、归档、置顶多段会话；一次 AgentRun 不等于一段新会话 |
| PC-D5 | **串行运行与阻断式 Ask User** | Ask User 直接替换普通输入框；回答/自由解释只属于原工具，重试/恢复原 Run |
| PC-D6 | 结构化事实 + 必要原话片段 | 原 Message 可滚动清理；领域持有可追溯的最小证据；纠错、撤回、隐私擦除分离 |
| PC-D7 | 两种删除语义 | 移除原始文件 vs 擦除文件及关联健康数据；后者按来源传播、避免误删独立来源 |
| PC-D8 | 目标产品暂不考虑公开分享健康对话 | 旧 ConversationShare 快照/令牌需治理和退役，不代表现网已关闭 |
| PC-D9 | 单会话采用滚动历史窗口 | 按消息语义处理、上下文保留和物理清理三个边界推进，不永久保存整段对话 |

## 2. 已确认的跨领域契约

| ID | 规则 | 禁止捷径 |
| --- | --- | --- |
| CD-D1 | SourceRef 稳定身份、用户归属、版本、时间及证据定位；被清理的原消息先转为领域持有的必要来源片段 | 依赖已删除 messageId、复制第二份 BodyState |
| CD-D2 | Go 各领域独立原子提交；跨域可靠事件、幂等/补偿和真实提交回执 | 将 ToolCall.succeeded 当领域提交、假称跨域总事务 |
| CD-D3 | 用户权限对应准确目标、提案、内容指纹、版本和交互身份；普通清晰本人事实照常自动记录 | 借旧授权更改另一个 Diagnosis 条目或计划 |
| CD-D4 | **采用 A：原件删除但保留健康数据时，历史准入继续有效，标记原件不可用**；新操作若必须可核验原件则不可仅凭历史数据完成 | 原件已删仍宣称可核验；将历史准入等同医学确诊或自动专业放行 |

## 3. 领域总览与不可混合的边界

~~~text
User (1) ├─ Profile? + Preferences
         ├─ EquipmentAsset[]
         ├─ UserAsset[] ── ExtractionRun[] ── EvidenceCandidate[] / Admission[]
         ├─ Conversation (1) ── Message[] (rolling)
         │                     ├─ AgentRun[] (serialized active run)
         │                     ├─ PendingInteraction? (composer replacement)
         │                     └─ RetentionCursor
         ├─ BodyState (1) ── Fact / Observation / Episode / SafetyState
         ├─ Diagnosis (1) ── ConditionEntry / AnalysisRun / Proposal / Decision
         └─ TrainingPlan[] ── Revision / Proposal / TrainingSessionLog
PersonView = Profile + BodyState Records/Habits + Equipment + UserAsset Media/Reports
~~~

- **事实自动入库不等于医学诊断自动采纳。** Diagnosis 的用户批准、原子 Patch 与安全阻断遵守 DX-D5～D15；Training 与 Trainer 的处方/执行权限按 TP-D1～D12；SafetyState 唯一权威按 SAFE-D1～D8。
- Message、LangGraph Checkpoint、短期 WorkingContext 和长期 SourceEvidence 是不同存储目的；只截断 LLM 上下文不算删除。
- 文件准入、历史可用性、当前原件可核验性分轴记录。仅移除原件时，历史观察可在注明来源限制后参与适当分析；专业放行等需要新鲜、可核验医疗证明的场景不能自动沿用。
- 真正擦除某来源时先阻断再清理原件、OCR、派生证据、缓存/索引/Checkpoint，协调 BodyState、Diagnosis 复核和 SafetyState 保守限制。保留最小脱敏回执；不能用 withdrawn 或 soft-delete 伪称彻底擦除。

## 4. 仓库现状证据（静态检查，非实施验收）

| 代码位置 | 已观察到的行为 | 与目标差距 |
| --- | --- | --- |
| `apps/api/internal/model/conversation.go` | Title、Pinned、DeletedAt 等旧多会话元数据 | 需单会话唯一性与旧数据迁移 |
| `apps/api/internal/repository/message_repository.go` | 按 seq 读取完整历史 | 需滚动处理、游标、合法边界清理 |
| `apps/ai-service/src/runtime/consultation_thread.py` | `MAX_CONTEXT_TURNS=10`，发给模型时限制消息 | 不代表底层消息与 Checkpoint 已物理清除 |
| `apps/ai-service/src/runtime/checkpointing.py` | PostgreSQL Checkpointer 为 interrupt/resume 真值 | 清理必须保护未完成 Run，并移除安全清理区间的旧快照 |
| `apps/web/src/features/consultation/runtime/activeTurnSelectors.ts` | streaming/interrupted 锁定 composer | 不证明 Ask User 已替换底部输入区 |
| `apps/api/internal/model/user_profile.go` | Gender、BirthDate 等简要字段 | 需 Profile 最小契约/Preferences 的 DTO 与数据迁移 |
| `apps/api/internal/model/user_upload.go` | OCR/姿态分析状态与 UserUpload | 需 UserAsset/多分类视图、处理状态与准入分离 |
| `apps/api/internal/service/upload_service.go` | DeleteUpload 删除对象和上传 manifest | 尚未证明两种删除语义、衍生传播、历史准入保护已实现 |
| `apps/api/internal/model/conversation_share.go` | 旧可公开访问的分享快照 | 目标功能退役前盘点、关闭与旧令牌清理 |

静态源码不能证明生产环境是否仍保留所有 OCR/Run 快照或是否存在真实历史访问令牌；这些由实现阶段的环境/数据审计确认。**旧项目实现不应反向改变本轮已批准的业务方向。**

## 5. 完成标准与独立剩余工作

本轮已完成 **产品语义确认**，不等于已经完成：
1. 旧多 Conversation 迁移、单用户唯一约束、Run 和工具交互并发/跨端恢复。
2. 语义提取后持久化来源片段、`processedThroughSeq/contextStartSeq/prunedThroughSeq`、消息/Checkpoint/缓存联动清理。
3. Profile/Equipment/UserAsset 的物理表/API、原文件移除与完全擦除两个独立纵切及可重试验证。
4. SourceRef、DomainCommitReceipt、Outbox/等价可靠事件、跨域鉴权/修订冲突的全链路测试。
5. 旧共享接口/令牌治理、真实备份/审计/合规留存约束、涉及健康数据擦除的可核验证据。
6. Diagnosis 的提案期限 **Q-17** 与确认组件设计 **Q-19** 属各自后续问题；临床安全规则阈值仍需专业审核，并非阶段三新的开放决策。

详尽目标定义在 [domain/person-chat.md](../domain/person-chat.md)，端到端验收在 [acceptance.md](../acceptance.md) 的 `PC3-*` / `CD3-*`，实现差异在 [implementation-gaps.md](../implementation-gaps.md)。**本文件不证明测试已运行、迁移已完成或功能已部署。**
