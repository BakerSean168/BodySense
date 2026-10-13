# BodySense V2 — 展示与交互契约

Status: TARGET CONTRACT / DESIGN HANDOFF
Version: 2.0.0
Baseline: 57aaf8974a835964db074dda7b39f3b80f162a86
Owner: Track A contract review + Track B implementation; transport adapters belong to Track C.

本文落实 [四线主计划](../../plan/active/2026-10-13-v2-parallel-master.md) 的展示契约，不新增业务决策、不替代 `docs/product/domain`。`ready` 的界面资源状态也不等于 Active Plan 中 `view_model` 门禁已验收。

## 1. 唯一可执行定义与消费方式

- `packages/contracts/src/presentation-v2.ts`：16 个查询、查询参数和命令入口。
- `packages/contracts/src/presentation-v2/{common,bodyState,chat,diagnosis,training,personAssets}.ts`：Zod 严格运行时结构；TypeScript 类型从同一结构推导，不再手写平行接口。
- 包入口：`@bodysense/contracts/presentation-v2`。不会把新名称塞入现有 stream-event 根导出。
- `packages/contracts/src/presentation-v2.fixtures.ts`：**合成设计/测试数据**。不得成为生产请求失败后的回退内容，不得把示例病名或动作写入用户账户。
- `packages/contracts/src/presentation-v2.test.ts`：结构和负面语义验证；纳入现有 contracts Nx test target。

使用方式：从生成的网络客户端取得数据，经明确适配器映射，再以对应查询 schema `parse`；组件只读 VM，命令经服务层发送。后端尚无目标查询时显示 unavailable/disabled，而不是运行 fixture。

```ts
import { presentationV2Queries } from "@bodysense/contracts/presentation-v2";
const records = presentationV2Queries["person.records"].parse(projection);
```

`projection` 的**完整五域生产构造器**仍是后续工程交付，不以强制类型断言冒充。在现有 API 范围内，Records/Habits 已由 `bodyFactV2Adapter.ts` 做了真实的 bounded bridge：输入为真实 BodyFact 生成 DTO 经 `projectBodyFactSurface` 投影后的 owner-bound 数据，输出经 `boundedBodyFactVM` 严格校验，随后被 `BodyFactRecords.tsx` 消费。它明确标记只包含 facts；source metadata / Observations / Hypotheses / Episodes / ConcernGroups 尚无读 API，不能伪造为已核验、也不能误判为真正空集合。完整的 `person.records`/`person.habits` schema 仍保持单独目标契约，须待 joined server projection 后切换。现有 HTTP 权威仍为 `packages/contracts/openapi/bodysense.v1.openapi.yaml`，事件权威仍为 `packages/contracts/schemas/stream-event.v1.schema.json`；本次未修改 HTTP/Proto/事件 wire format。

## 2. 共用边界

### 2.1 读取与显示

每个查询返回判别联合：`loading | empty | ready | partial | stale | offline | recovering | failed | permission_denied`。`empty` 必须是鉴权成功后的真实空结果；拒绝权限不得携带缓存内容。断网/失败保留可展示的旧内容时必须标识最后获取时间，不使用正常成功外观。

资源 `meta.sourceVersions` 绑定所属领域、目标身份和 revision；仅用于刷新、比较和防陈旧。技术 revision、eventId、toolCallId 不放入普通用户正文。观察/发生时间与记录时间分别展示；时间不详显示“不详”，不能借用 `recordedAt`。日期精度、原始时间表达和时区保留。

所有可行动按钮使用 `capability`：enabled 或 disabled + 可理解原因。Loading 不偷偷提供可点击保存。错误以局部、可恢复文案为主；风险阻断才使用安全警示语义。

### 2.2 命令、身份与回执

客户端命令不接受 `userId`、任意 actor、任意临床认证字段。当前用户、证据来源及工具身份由鉴权后的 Go 服务解析。修改已存在对象必须带预期 revision 和幂等键；创建带幂等键；浏览/查询不产生写入。

`exactConsent` 绑定 interaction/run/tool/contentFingerprint/sourceVersions。明确的直接用户操作可由服务器生成直接授权证据；通过 AI 的确认不能脱离原 Run 挪作其他内容的授权。

`pending` 只表示请求已发出；`decision_recorded` 只表示选择已保存；`committed` 必须有真正 `DomainCommitReceipt`（领域、对象、revision、非空 eventIds、提交时间）。拒绝/冲突/失败不显示“已保存”，不清理用户草稿。幂等重复返回原回执或明确 unchanged，不再创建第二个对象。

跨领域动作逐项报告回执，不承诺 BodyState/Diagnosis/Training 之间不存在的全局事务。Python 的工具输出/分析文本不是提交证据。流断开后以稳定 Run 和序号恢复、重新读取所属领域，不凭“工具完成”动画推导业务成功。

### 2.3 刷新与本地状态

保存后的刷新由实际回执所属领域决定，重新取得权威投影；不得只更新一个计数留下过期选项。切换标签、打开窗口、选择身体部位、滚动、演示动作只更新本地 shell state。窗口关闭不等于取消 Run、拒绝 proposal 或结束训练。

缓存键必须包含当前认证主体，退出/换用户清理；读取发生权限变化时立即隐藏敏感缓存。列表 cursor 为不透明标记，不自行按 offset 推导。技术历史与用户可理解的经历/决定历史分开。

## 3. 五组领域的显示字段与动作

| 查询 / 窗口                            | 字段与默认阅读顺序                                                                                          | 动作、刷新与边界                                                                                                  |
| -------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `person.records` / Records             | 当前 facts、observations、user hypotheses；按症状经历分组。来源、发生时间、记录时间可展开；技术修订不抢正文 | record/correct/withdraw/temporal-change 后刷新 records、相关 episode/region；纠错不是“今天变好了”                 |
| `person.habits` / Habits               | 仅 lifestyle；原始频率/时长、来源与有效陈述状态                                                             | 复用 BodyState 命令和回执，不另建 Profile.habits                                                                  |
| `body.episode` / 经历详情              | 症状、部位/侧别、onset、unknown/ongoing/ended、basis facts、stateAsOf、时间线、关联依据                     | 查看/询问；更改链接必须具体 proposal + consent；后续复发可形成新 identity                                         |
| `body.region` / 人体选区               | 选区 + facts/observations/hypotheses/episodes/adopted conditions 引用                                       | 选中只是导航；不会产生症状事实，也不会得到“健康分”                                                                |
| `chat.current` / Chat                  | 唯一 conversation；稳定 seq；消息、Run 步骤、单个 composer、最近活动时间、裁剪提示                          | send/answer/cancel/recover；answer 精确恢复原 Run；不存在聊天列表或第二会话按钮                                   |
| `diagnosis.current` / Diagnosis        | 完整 adopted 集合；selector 显示病名；AI assessment、临床来源、证据、原因分析、处理方向、信息缺口分开       | 主计数仅 adopted 且只放 Dock；讨论不会自动触发正式改写                                                            |
| `diagnosis.history` / 历史             | 时间、历史 condition snapshot、采纳/撤回决定及理由                                                          | inactive 不等于已治愈/已排除；当前来源失效独立提示                                                                |
| `diagnosis.confirmation` / Chat 确认区 | 当前一张 proposal、序号/总数、内容指纹绑定、已记录选择、rechecking/superseded/expired/committing/committed  | 接受/拒绝后仅记录选择；affected 提案重新确认；真正批量提交后刷新 diagnosis current/history + chat                 |
| `training.plans` / Training            | 多个正式计划；accepted revision；独立 status；模板/stages/动作实例/sets/target/rest/constraints；独立 gate  | 选计划/模板只导航；status command 不借机解除安全限制                                                              |
| `training.proposal` / 计划建议         | create/update、完整预览/差异、依据、过期/失效原因、可接受状态                                               | accept/reject exact proposal；成功才刷新正式计划；不从 pending preview 启动 Trainer                               |
| `trainer.current` / Trainer            | SessionLog、固定计划修订/模板快照、训练日/时区快照、真实 actions、当前 set、已核验计时、feedback、gate      | start/pause/resume/complete-set/skip/correct-action/finish/feedback；每次完成对应稳定动作实例；完成前后台再验安全 |
| `safety.current` / 执行限制            | 风险级别、范围、来源、期限/复核点、解除条件、活动限制与最终 gate                                            | advisory 不是 block；restricted 只约束匹配范围；urgent 全局阻断并要求专业解除                                     |
| `person.identity` / 头像昵称弹层       | 可选昵称/头像/生日/自报 gender/locale/timeZone/unit preference；缺失为未填写                                | Profile patch 省略=不改，null=明确清空；不复制身高体重等健康事实                                                  |
| `person.equipment` / Equipment         | 资源名称、owned/borrowed/facility access、可用性/期限、受影响计划只读链接                                   | create/update 后刷新 equipment + affected plan review；不直接重写计划                                             |
| `person.assets` / Media & Reports      | 同一 assetId 的不同 facet；原件状态、extraction run、candidate admission 分层；文件/候选/来源位置           | 上传与提取分离；重试保留上次 run 来源；需要准入前不变成 Observation                                               |
| `person.erasure` / 删除任务            | scope fingerprint、将删/保留范围、独立来源、各类副本清理、阻断新用状态、验证回执                            | original-only 和 related-health erasure 为两个命令；失败继续禁用来源；所有范围核验后才显示完成                    |

### 3.1 排序与分页

Records/Habits 默认显示当前 asserted 内容，按记录时间降序、稳定 ID 作为并列键；发生时间另列，不用不详时间伪造排序值。历史撤回/更正按需展开。Episode 详情按发生时间精度排序，同一时间或未知时以 recordedAt/稳定事件 ID 稳定排序并注明未知。

Diagnosis 当前查询返回整个 adopted 集合而不是一页；count 与 selector 必须来自同一结果。保持用户仍有效的选中项，否则选择集合首项；排序由服务端稳定返回。历史通过 cursor 独立分页。Training 保持仍有效的 plan/template 选择，计划 status 不随选中项变化。资产按 createdAt 降序/ID 并列键，用 cursor 分页；两分类视图可以出现同一个 assetId，但同一结果不得重复。

### 3.2 关键状态转换

- 陈述：未发送 → pending → receipt → asserted。歧义转 Ask User；撤回保留审计，不转成症状 absent。
- Episode：unknown 可以因为明确当前报告成为 ongoing；ended 必须有明确结束依据；没有消息/过期缓存不触发结束。
- Chat：idle → running → waiting_for_user → 原 Run running；交互过期/冲突先恢复或重读；关闭窗口不改变链条。
- Diagnosis：proposal → decision_recorded →（受影响内容重新核验）→ committing → **一个诊断批次回执**；安全终止零诊断提交。
- Training：create proposal → accepted plan；update proposal 过期或 base 变化不能应用；7×24h 是训练 proposal 上限，不是 Diagnosis 默认值。
- Trainer：in_progress ↔ paused → ended；跳过不是完成，查看下一动作不是完成，恢复时不能重算离线期间为真实完成时间。
- 资料：extraction succeeded ≠ admissible；original unavailable ≠ historical admission 被抹除；erasure retryable_failed ≠ completed。

## 4. 规则及测试追溯

| 产品权威                                                                        | VM 模块                       | 重点证据                                                                                                       |
| ------------------------------------------------------------------------------- | ----------------------------- | -------------------------------------------------------------------------------------------------------------- |
| [BodyState BS-D1～D10](../../product/domain/bodystate.md) §§2.2–2.8             | bodyState/common              | 五类事实；未知时间；用户 hypothesis；已准入 observation；episode ending basis；不创建 BodyConcern              |
| [Person/Chat PC-D1～D9](../../product/domain/person-chat.md) §§1–3              | chat/personAssets             | composer 互斥、原 Run/interaction、稳定 seq；极简 Profile；分类不复制 asset；擦除完整范围                      |
| [跨领域 CD-D1～D4](../../product/domain/person-chat.md) §4                      | common/personAssets/diagnosis | Go receipt、拒绝用户注入、精确指纹、历史准入与原件可核验独立                                                   |
| [Diagnosis](../../product/domain/diagnosis.md) §§3.0–3.9                        | diagnosis                     | medicalName/assessment/source/disposition 分层；四种 proposal；计数 adopted-only；无 lifecycle；安全终止不继续 |
| [Training TP-D1～D12](../../product/domain/training.md) §§4.1–4.5               | training                      | 多计划/模板/实例；proposal 生命周期；训练日快照；真实动作日志                                                  |
| [Safety SAFE-D1～D8](../../product/domain/safety.md) §§2.3–2.9                  | training                      | 独立 gate；unknown/stale fail-closed；urgent 专业解除；不编辑 plan.status                                      |
| [产品验收](../../product/acceptance.md)、[交互规范](../../product/interface.md) | 全部                          | 与各条原文共同评审；contract tests 不能替代真实 API、跨设备、浏览器验收                                        |

执行：`pnpm exec vitest run packages/contracts/src/presentation-v2.test.ts`；`pnpm nx typecheck @bodysense/contracts`。若工作树并行开发，使用各自依赖目录；不能让自动安装写入其他工作树的 node_modules。

## 5. 尚待 Track C/D 的真实接线，不得误报

BodyFact 当前 generated `list/record/correct/withdraw` 服务中，Records/Habits 的 **bounded real transport → VM → React** 已接通，并有 7 个新的 Adapter 专项测试及既有组件行为测试；Ask User 保留原 Run 恢复。由于旧接口不包含关联诊断来源和 Episode 证据，不应假装已经产出 `presentationV2Queries['person.records']` 的完整 `ready` 内容。其余目标读取/命令、HTTP 路由、Proto payload、鉴权/并发/幂等重放、event→query invalidation，须由相应领域的工程 ticket 补齐。

| 责任              | 尚待验证                                                                                           |
| ----------------- | -------------------------------------------------------------------------------------------------- |
| DS-BS-01 / ENG-BS | 当前 BodyFactRecords DTO → Records/Habits；Episode 并行实现后与本契约对照；不覆盖另一 writer       |
| DS-PC-01 / ENG-PC | Chat 单会话唯一约束、三游标 retention、跨设备 stale answer；这三个游标不是新 UI 输入项             |
| DS-DX-01 / ENG-DX | V2 aggregate/query/receipt、顺序确认与批量事务；不能把旧 assessment DTO 强行转成新 ConditionEntry  |
| DS-TP / ENG-TP    | Plan/Trainer/Safety transport 和每次动作前的后端 gate；前端 `canRequestExecution` 只是保守展示守卫 |
| DS-PE / ENG-PE    | Profile/Equipment/UserAsset transport；完整 source-admission、erasure propagation 与 verifier      |
| IR 各领域         | 真实登录、权限撤销、断网重入、跨设备争用、同一天多 Session、跨日保留训练日、视觉/键盘/移动端       |

因此本次提供完整的**目标展示词汇和可校验形状**，不宣称五个领域已经端到端实现；统一登记仍位于 `docs/plan/active/v2-handoff-state.json`，未完成 typed endpoints 的领域不能仅凭本文变更为门禁 ready。

Q-17（Diagnosis proposal 的时效策略）仍不填固定数值；Q-19（多提案确认布局）本次由设计稿给出候选方案，但不能伪造用户最终签收。训练和擦除的安全阈值/检测规则仍由后端权威策略决定，前端不增加医学阈值。
