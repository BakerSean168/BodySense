# BodySense V2 · 整体重构进度检查点 · 2026-10-17

- **性质：** 全项目、只读事实快照（不是新计划、不是合并或发布授权）
- **核验时间：** 2026-10-17 12:14–12:16 Asia/Shanghai
- **范围：** [Epic #301](https://github.com/BakerSean168/BodySense/issues/301) / [#302–#317](https://github.com/BakerSean168/BodySense/issues/302)，以及此前已合并的 V2 基础工作。
- **截至核验的 origin/main：** `51b6380fec608fe65a5feca554941fc88cc3731b`（PR #325 合并）。
- **决策来源：** [2026-10-16 remaining plan](./2026-10-16-v2-remaining-plan.md)、[V2 Master](./2026-10-13-v2-parallel-master.md)、[domain contracts](../../product/domain/README.md)、当前 GitHub Issue/PR/CI、T3 线程与 Git worktree。
- **时效声明：** 本文记录某一时点的事实；下次实施必须以实时 GitHub、源代码、CI、T3 状态重新核验，不从此文推断活跃 writer 或可合并性。

## 1. 整体结论

- **Epic #301：OPEN。** #302–#317 共 **16** 个子 Issue，其中 **3 CLOSED**（#302/#303/#304），**13 OPEN**。`3/16` 是任务关闭数，**不能转换为开发完成率**：任务大小不同，且早期已完成大量不包含在这些新 Issue 中的基础工作。
- 五个业务领域（BodyState、Conversation/Person、Diagnosis、Training/Safety、UserAssets/Privacy）的业务契约 gate 均为 `ready`，但 **0/5 领域拥有完整的六 gate 验收**。完整端到端 Product V2、QA #316、Release #317 都未验收。
- **已有三条 Draft 重构 PR**：Diagnosis [#322](https://github.com/BakerSean168/BodySense/pull/322)、TrainingPlan [#326](https://github.com/BakerSean168/BodySense/pull/326)、UserAsset [#327](https://github.com/BakerSean168/BodySense/pull/327)，均未合并；因此内容不得算作 main 正式交付。
- **开发与发布隔离**：V2 尚未在 staging/prod 完成发布资格认证；无生产/预发清库、无 staging-latest 自动晋升、无 V2 正式发布授权。已有的 main 候选镜像构建或 CI 绿色不等于 V2 发布。

### 合并进 main 的阶段性成果（不得重复实施）

1. 原生 **BodyFact、BodyObservation v11/v12、SymptomEpisode、BodyStateRevision、用户 BodyHypothesis / ConcernGroup** 的持久化与核心接口；PR6 A–E **legacy BodyState retirement 已完成合并**，参见 [PR6 closeout](../archive/2026-10-16-pr6-e-closeout.md) / PR #298。
2. 单用户 **Conversation / 单活动 Run** 基础和旧 Chat title/share 栈清理：Issue [#302](https://github.com/BakerSean168/BodySense/issues/302) 已 CLOSED；PR [#321](https://github.com/BakerSean168/BodySense/pull/321)、[#324](https://github.com/BakerSean168/BodySense/pull/324) 已合并。**不等于三游标 retention 已完成**。
3. 用户 Hypothesis 与 Observation 关联、准确更正和稳定 ConcernGroup ID：Issue [#304](https://github.com/BakerSean168/BodySense/issues/304) 已 CLOSED，PR [#320](https://github.com/BakerSean168/BodySense/pull/320) 合并。**不等于前端 Person 终态已完成**。
4. 分级/分作用域 Safety、pure TrainerGate、版本与用户证据绑定的 Safety lift：Issue [#303](https://github.com/BakerSean168/BodySense/issues/303) 已 CLOSED；PR [#319](https://github.com/BakerSean168/BodySense/pull/319) / [#325](https://github.com/BakerSean168/BodySense/pull/325) 合并。**不等于 TrainingPlan/Trainer 生命周期完成**。
5. 已合并 TP-PROP-001 [PR #274](https://github.com/BakerSean168/BodySense/pull/274) 等前置原语；不能把仅存在的纯 `EvaluateAcceptance` 模型误当最终业务 API。

## 2. 领域完成度（以整体业务闭环为单位，不给虚构百分比）

| 领域                            | main 上已经真实存在                                                                                            | 当前缺口 / Issue                                                                                                                                                                                                                                                      | 整体验收             |
| ------------------------------- | -------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------- |
| BodyState / Hypothesis / Safety | 原生证据账本、Episode、Observation 更正、Hypothesis/ConcernGroup、Go-only Safety 限制及解除；旧 BodyState 退役 | Chat AI suggestion → Go-committed user decision [#308](https://github.com/BakerSean168/BodySense/issues/308)；Person/Episode/Hypothesis 完整 UI + Penpot [#309](https://github.com/BakerSean168/BodySense/issues/309)；跨域 QA                                        | **未通过全部 gates** |
| Conversation / Person           | 一人一 Conversation/活动 Run、AskUser 原 Run 与用户回执路径、title/share 清理                                  | 三游标 retention、实际安全修剪与检查点清理 [#307](https://github.com/BakerSean168/BodySense/issues/307)；完整 Person/Profile/资产呈现、跨设备/窄屏 QA                                                                                                                 | **未通过全部 gates** |
| Diagnosis                       | 旧分析/候选与新领域契约已明确；**新 Diagnosis 聚合的 slice 1 在 Draft #322，尚不在 main**                      | [#305](https://github.com/BakerSean168/BodySense/issues/305) 原子决定事务 + Read/Decide API + 旧路径退役；[Agent API #310](https://github.com/BakerSean168/BodySense/issues/310)；[UI #312](https://github.com/BakerSean168/BodySense/issues/312)；端到端安全/回放 QA | **未通过全部 gates** |
| TrainingPlan / Trainer          | Safety/TrainerGate 域原语已合并；**多 active plan/proposal accept 的 slice 1 在 Draft #326，尚不在 main**      | [#311](https://github.com/BakerSean168/BodySense/issues/311) 新 Plan API、Treatment 退役与 stale context 修复；[Trainer #313](https://github.com/BakerSean168/BodySense/issues/313) per-set receipts；[UI #315](https://github.com/BakerSean168/BodySense/issues/315) | **未通过全部 gates** |
| UserAssets / Privacy            | 既有 upload/document review 等功能；**新 user_assets/source-removal slice 1 在 Draft #327，尚不在 main**       | [#306](https://github.com/BakerSean168/BodySense/issues/306) web 分类呈现 + source unavailable + Profile；[#314](https://github.com/BakerSean168/BodySense/issues/314) 原件/缓存/检查点/衍生数据的真正可验证擦除；后者≠source removal                                 | **未通过全部 gates** |

`v2-handoff-state.json` 截至此次核验仍反映先前的执行基线（`latestExecution.baselineHead=6687a9e26`），故其领域 gate 仍只作**待更新的旧审计账本**，不得把它的执行记录当作当前 git/main/PR 状态。正式 gate 只能凭新 exact-SHA 证据逐项提升。

## 3. Epic #301 全部子 Issue 当前状态（16 个）

| Issue                                                        | Ticket       | 已验事实 / 尚欠交付                                                                                  | 真正状态                       |
| ------------------------------------------------------------ | ------------ | ---------------------------------------------------------------------------------------------------- | ------------------------------ |
| [#302](https://github.com/BakerSean168/BodySense/issues/302) | PC-001       | PR #321 + #324，singleton Conversation / serial Run + title cleanup                                  | **CLOSED**                     |
| [#303](https://github.com/BakerSean168/BodySense/issues/303) | TP-001       | PR #319 + #325，Safety / TrainerGate / lift evidence                                                 | **CLOSED**                     |
| [#304](https://github.com/BakerSean168/BodySense/issues/304) | BS-HYP-001   | PR #320，Hypothesis observation link / correction / stable group id                                  | **CLOSED**                     |
| [#305](https://github.com/BakerSean168/BodySense/issues/305) | DX-002       | Draft PR #322 = slice 1（contract + persistence），后续 decision transaction 和接口/旧路径退役未交付 | **OPEN / 部分代码在 Draft**    |
| [#306](https://github.com/BakerSean168/BodySense/issues/306) | AS-001       | Draft PR #327 = slice 1（UserAsset / source removal），后续 web / Profile slices 未交付              | **OPEN / 部分代码在 Draft**    |
| [#307](https://github.com/BakerSean168/BodySense/issues/307) | PC-RET-001   | 三游标 Chat retention / pruning 未见对应已合并切片                                                   | **OPEN / 待实施**              |
| [#308](https://github.com/BakerSean168/BodySense/issues/308) | BS-AGENT-001 | AI source-bound suggestions → AskUser → Go commit 尚未完成                                           | **OPEN / 待实施**              |
| [#309](https://github.com/BakerSean168/BodySense/issues/309) | UI-001       | Person/Episode/Hypothesis/ConcernGroup 完整 UI + 390px/Penpot 验收未完成                             | **OPEN / 待实施**              |
| [#310](https://github.com/BakerSean168/BodySense/issues/310) | DX-003       | Agent proposals ingestion / direct correction；依赖 #305                                             | **OPEN / 依赖未解锁**          |
| [#311](https://github.com/BakerSean168/BodySense/issues/311) | TP-002       | Draft PR #326 = slice 1（多 plan / revision / proposal accept），新 Plan API 与 Treatment 退役未完成 | **OPEN / 部分代码在 Draft**    |
| [#312](https://github.com/BakerSean168/BodySense/issues/312) | UI-DX-001    | Diagnosis Window 的新 V2 决策 API、状态与视觉验收；依赖 #310                                         | **OPEN / 依赖未解锁**          |
| [#313](https://github.com/BakerSean168/BodySense/issues/313) | TP-003       | TrainerSession/per-set performed/skipped 用户动作回执；依赖 #311                                     | **OPEN / 依赖未解锁**          |
| [#314](https://github.com/BakerSean168/BodySense/issues/314) | AS-002       | 真正跨数据层/缓存/检查点可验证隐私擦除；依赖 #306 和 #307                                            | **OPEN / 依赖未解锁**          |
| [#315](https://github.com/BakerSean168/BodySense/issues/315) | UI-TP-001    | TrainingPlan/Trainer Windows V2 APIs + 窄屏/安全门禁；依赖 #313                                      | **OPEN / 依赖未解锁**          |
| [#316](https://github.com/BakerSean168/BodySense/issues/316) | QA-001       | 现有 PG18/E2E 与 PR6 25/25 是基础；五个领域 exact-SHA、second-device、390px 等全矩阵未完成           | **OPEN / 持续 QA，终验未通过** |
| [#317](https://github.com/BakerSean168/BodySense/issues/317) | REL-001      | 五域六 gate 全证据→预发 wipe/canary/回退→用户最终批准的生产切换                                      | **OPEN / 尚未进入发布验收**    |

**波次视图：** Wave 1 共 5 项，3 项已关、2 项部分实施（#305/#306）；Wave 2 仅 #311 有 Draft slice，其余未完成；Wave 3/4 仍因前置依赖未满足；Wave 5 全域 QA/Release 未收尾。不能把“Wave 有 Draft”写为“Wave completed”。

## 4. 当前三条 Draft PR 的准确门槛

| PR                                                                     | 本次远端 exact head                        | 状态 / GitHub CI                                                                                                                                                                    | 待办                                                                                                                                                        |
| ---------------------------------------------------------------------- | ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [#322](https://github.com/BakerSean168/BodySense/pull/322) · DX-002 s1 | `5a71d992017116c8673990c31e4c22bfdb3d4133` | Draft, CLEAN/MERGEABLE；**12/12 SUCCESS**，https://github.com/BakerSean168/BodySense/actions/runs/38994013493                                                                       | 独立审查记录/正常合并；成功也只解锁 #305 的后续 slice，不自动关闭 #305                                                                                      |
| [#326](https://github.com/BakerSean168/BodySense/pull/326) · TP-002 s1 | `43c0ace0555a20dff0fd57cfdb29f0e6ddee6ba2` | Draft, CLEAN/MERGEABLE；**12/12 SUCCESS**，https://github.com/BakerSean168/BodySense/actions/runs/38993530469                                                                       | 独立审查警示：LockContext 仍引用旧 `diagnosis_analyses`；确认新 Diagnosis evidence/context 变化是否能使旧 PlanProposal 失效；必要时修正、rebase/跑新 SHA CI |
| [#327](https://github.com/BakerSean168/BodySense/pull/327) · AS-001 s1 | `f8bb93851cd13c1992f3c54a2dc764d116ce3203` | Draft, MERGEABLE；**新 head CI 正在执行，尚未 12/12 全绿**（最近检查含 Affected Quality、PG18 Baseline、Browser E2E in progress），mergeStateStatus=BLOCKED（pending checks/Draft） | 不干扰当前 AS-001 writer/run；先等新 head CI，独立 review；与 #322/#326 合并同一 baseline/OpenAPI 时串行 rebase+重生成 contracts                            |

AS-001 同范围 T3 thread `fabric:1eef157589bdd5b3945ff0da0452c9625df85c2262f1e7ff14a4bd112fd7772f`、GCP 独占 worktree `bodysense-v2-handoff-as001-glm`，Windhub GLM-5.3。此次核查时 run #6 曾 `running`，随后状态读数为 `queued`（同一 run），且本地 HEAD 已与远端 Draft PR 对齐为 `f8bb93851`；**实时线程状态需下次核验，绝不由 queued 认定任务已终止或允许第二 writer**。DX-002 / TP-002 原线程分别 completed，但 PR 未 merged。

## 5. 后续依赖与项目整体完成门槛

1. **先收敛 Wave 1 中心 PR：** #322（DX-002 s1）与 #327（AS-001 s1），逐项独立审查、exact-head CI/保护规则和 shared schema 串行 rebase；接着各自完成其 Issue 剩余 slices。**不能只合并 slice 1 就关闭整张 Issue**。
2. **解锁已满足前置的 Wave 2：** PC-RET #307（依赖 #302 已 closed）、BS-AGENT #308 与 Person UI #309（依赖 #304 已 closed）可作为下一轮独立切片；TP-002 #311（依赖 #303 已 closed）继续对 #326 做跨 Diagnosis 验证并完成剩余 Plan API/Treatment 退役。
3. **之后串行完成新 Diagnosis → Trainer → Assets/Privacy 的端到端闭环：** #305 完成后 #310→#312；#311 完成后 #313→#315；#306 和 #307 完成后 #314。
4. **每域合并立即补 QA #316 的对应矩阵**：owner 404、来源与版本 CAS、幂等/并发、真实 PostgreSQL 18、Go Safety fail-closed、浏览器/390px、跨设备与真实回执。五域证据齐全后才可能完成 #316。
5. **REL-001 #317 最后**：用户未授权本阶段修改 staging/prod 数据、清库或部署；取得完整产品 QA、明确发布门槛与新的最终授权前，不推进任何 V2 环境晋升。

## 6. 执行与资源风险

- **当前模式：DELEGATED**。ChatGPT Web 独立 review/验收；GCP T3 Codex Team `gpt-6.1-sol` 优先，额度限制后使用用户已授权 Windhub `glm-5.3`。不同 worktree 可独立写，但合并共享 baseline/OpenAPI 必须串行 rebase、重新生成和按新 exact SHA 跑 CI；避免重复 writer。
- **当前 GCP 资源快照：** 31 GiB RAM，约 21 GiB available；Swap 8 GiB 接近打满，根盘 193 GiB 用量 88%（约 24 GiB 可用）。因此不要并发重型 Go race/PG18/Playwright 批量任务，以免 OOM/磁盘膨胀；不要未经授权清理其他 Agent 的工作区。
- **文档账本滞后**：`docs/plan/tickets/v2/queue.json` 仍将 #302/#303/#304 显示为 blocked/queued，`v2-handoff-state.json` 尚停在旧 baseline。可单独 docs-only PR 同步这三项 closed 和接受 SHA、准确记录其余 in-flight PR；**不得自动提高任何完整领域 gate 或把 Draft 当 done**。
- **持续性**：每小时任务只是一轮新的调度，不是无限保持同一 ChatGPT 会话在后台执行；单轮 CI/Agent/权限/网络/429 失败绝不能自行 disable 整个 scheduler。当前开发期禁止 staging/prod 数据操作和 release。

## 7. 下一次续跑入口

- 首先核查 `origin/main`、Epic #301 全部 Issue、Draft #322/#326/#327 的远端 SHA、CI/mergeable，以及 AS-001 T3 当前 run/worktree 独占。
- 不重复实施已合并的 PC-001、TP-001、BS-HYP-001 和 PR6；按最新依赖选择 #307/#308/#309 等已解锁独立切片。
- 本检查点本身不提供 commit/merge/QA 合格证明；也不要求中断已有 Agent。若现场源代码/PR 有新变化，更新事实、处理冲突再继续。
