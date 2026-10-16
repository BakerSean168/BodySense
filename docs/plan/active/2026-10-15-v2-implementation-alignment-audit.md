# BodySense V2 · 实施—文档对齐审计与下一轮执行顺序（2026-10-15）

Status: **ACTIVE / ALIGNMENT CHECKPOINT**（从属于 [V2 Master](./2026-10-13-v2-parallel-master.md) 与 [Execution Reorganization](./2026-10-14-v2-execution-reorganization.md)；不是新的业务权威，不提升任何 `v2-handoff-state.json` Gate）。
审计时间：2026-10-15 02:20–02:40 UTC（北京时间 10:20–10:40）。审计者：Hark（独立只读审计，GCP `gcp-dev-01`，自有工作树 `.hark-worktrees/bodysense-plan-alignment`，未触碰任何其他 writer 的工作树、锁、调度或发布通道）。

## 1. 对外部审计报告的核实结论

外部审计（同日 ChatGPT 生成）总体**属实**。逐项核实：

| 审计断言                                                     | 核实结果                                                                                                                                                                 | 证据                               |
| ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------- |
| `origin/main` = `e4160a9c7`，PR #273 于 10-14 23:33 CST 合并 | ✅ 属实                                                                                                                                                                  | `git log origin/main`              |
| 默认工作树停在 `0e287ab12`，落后 main                        | ✅ 属实，落后 **81** 个提交，另有未跟踪 `.zcode/`                                                                                                                        | `git rev-list --count`             |
| 共 14 个已登记工作树                                         | ✅ 属实（含 `.forgeflow-control/bodysense`，落后 361 提交，属历史遗留）                                                                                                  | `git worktree list`                |
| Track A 最后活动约 08:08 CST，Track B 约 02:35 CST           | ✅ 与 Agent Fabric 作业记录一致：Domain 工作树最后作业 10-15 00Z（08 CST），React 工作树最后作业 10-14 18Z（02 CST）                                                     | `~/.local/state/dev-host-mcp/jobs` |
| Track C / D 今天仍在执行                                     | ✅ 属实：V12 工作树 01–02Z 连续作业；Track D 01:39–02:11Z 作业                                                                                                           | 同上                               |
| V12 正在跑 `go test -race`                                   | ⚠️ **已过时**：该 Race 测试已于 02:16Z 以 exit 0 结束（repository/httpapi/consultation），随后 `go vet` 通过。V12 新增 3 个测试提交 **尚未推送、没有远程分支和 PR**      | 作业 exit-code；`git ls-remote`    |
| PR #266 / #267 已合并                                        | ✅ 属实（20:13 / 20:31 CST）                                                                                                                                             | `gh pr list`                       |
| PR #274 未取得 CI 成功证据                                   | ⚠️ **已过时**：exact-head `681edbd8e` CI run 38593599317 在已执行的 10 个 lane 全部 PASS；**PG18 baseline/recovery 两个 child 为 SKIPPED**；PR 仍 OPEN/MERGEABLE，未合并 | `gh pr checks 274`                 |
| handoff 状态登记滞后                                         | ✅ 属实，且比报告描述更广（见 §2）                                                                                                                                       | —                                  |
| 多个工作树有未提交内容                                       | ✅ 属实：V12（`.venv`）、Domain（1 个文档修改）、QA（staged 脚本 + `.venv`）、Track D（3 个 staged PC-001 文件）、React（3 个 staged 源文件 + `.react-writer-lease/`）   | `git status`                       |
| 没有持续运行 ForgeFlow/Pi Worker 证据                        | ✅ 属实；OPS-001 仍 blocked                                                                                                                                              | queue.json                         |

**报告遗漏的关键事实：**

1. **Track D 正被平台安全控制阻断写入。** `trackd15-20261015-0939-acceptance.md` 记录：`git commit`、原生 PG18 动态脚本、`pnpm contracts:verify` 三项 START BLOCKED。Track D 的“运行中”实际上只能产出 staged 补丁和只读检查，**成果无法落地**。这是比“Track A/B 停用”更直接的吞吐瓶颈。
2. **V12 owner 工作树违反了现行准入规则。** 其基线是 #255 之前的旧提交，落后 main 78 个提交；按 `validate-v2-ticket-queue` 规则，in_progress 的 writer 必须从冻结 main SHA 起步。V12 收尾必须先 rebase。
3. **交付重心偏向治理产物。** 自 #255（main 基线 `6bbc36acb`）以来，main 上变更行数：生产代码 **302**，测试 1124，`scripts/plan` **3724**，文档 1247。即一天内生产代码不到 **5%**，约 95% 是测试、计划脚本与文档（详见 §4）。

## 2. 文档—代码偏差清单（本次已修正 / 待 owner 修正）

| #   | 文档位置                                               | 偏差                                                                               | 处理                                                                                                    |
| --- | ------------------------------------------------------ | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| D1  | `tickets/v2/queue.json` `mainBaselineSha`              | 停在 `375e5a821`（#267），实际 main 已是 `e4160a9c7`（#270/#271/#272/#273 已合并） | **已修正**为 `e4160a9c7…`，旧值移入 `previousFrozenBaselineSha`                                         |
| D2  | `queue.json` 无 TP-D1 工单                             | #270/#271/#273（issue #268 阶段 1–2）已合并却不在 DAG 中                           | **已新增** `TP-D1-001`（done，仅接受为 dormant 代码）                                                   |
| D3  | `queue.json` 无 PR #274 工单                           | PlanProposal 预提交策略已有 CI 绿灯的 PR                                           | **已新增** `TP-PROP-001`（in_progress，base = 新冻结 main）                                             |
| D4  | `queue.json` `PC-001: queued`                          | #260–#262 审计与 #266 只读 seam 已合并；Track D 有未提交补丁                       | **已改为** blocked + 具体原因                                                                           |
| D5  | `queue.json` `BS-OBS-001: queued`                      | V12 正由 ENG-BS-04 owner 积极推进                                                  | **已改为** blocked（需 rebase/推送/PR）                                                                 |
| D6  | `TP-002` 依赖仅 TP-001                                 | 物理迁移必须建立在 TP-D1 与提案策略之上                                            | **已补充**依赖 `TP-D1-001`、`TP-PROP-001`                                                               |
| D7  | `v2-handoff-state.json` `latestExecution.baselineHead` | 停在 `375e5a821`                                                                   | **已更新**；Gate 状态**未改动**                                                                         |
| D8  | `v2-handoff-state.json` training-safety 域             | 五个 Gate 全部 `not_started`，未记录已合并的 dormant 兼容代码                      | **已新增** boundedSlice `training-multi-active-dormant-compat`（Gate 仍不变——兼容代码不构成 Gate 证据） |
| D9  | `README.md`（active 索引）                             | 写“Track A/C/D hourly enabled、React disabled”                                     | **已修正**：A/B 均停用、C/D 启用；并声明调度状态以调度器为准                                            |
| D10 | Master Plan 第 3 行                                    | 写“hourly continuation automation has been disabled”，与 README 矛盾               | **已修正**为指向调度事实与本审计                                                                        |
| D11 | `2026-10-14-v2-main-baseline-handoff.md`               | 宣称新工单基线为 `6bbc36acb`，已被取代                                             | **已加 SUPERSEDED 标注**                                                                                |
| D12 | Domain 工作树 `ds-dx-01` 修改未提交                    | 属于 Track A owner                                                                 | 未触碰；需 owner 提交或放弃                                                                             |
| D13 | Track D staged PC-001 补丁                             | 被平台阻断，无法提交                                                               | 未触碰；见 §5 P0-2                                                                                      |

## 3. 各域实际实施状态（以 main 为准）

| 域                        | main 上已落地                                                                                             | 仅在工作树 / PR                                                     | 仍未开始                                                             |
| ------------------------- | --------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- | -------------------------------------------------------------------- |
| BodyFact / Episode        | v1–v10 迁移、陈述账本、更正、Episode 创建/关联/状态/复发                                                  | —                                                                   | 旧模型切换（legacy cutover）                                         |
| BodyObservation           | v11（000011）直接报告；v12（000012）更正/撤回 + #263 外部修订冲突隐藏                                     | V12 owner 3 个跨租户幂等/并发测试提交（未推送）                     | v12 原生浏览器 E2E、正式 UI（UI-001）                                |
| Hypothesis / ConcernGroup | #264 参考契约 + JS 参考模型测试                                                                           | Domain 工作树 1 个未推送文档提交 + 1 个未提交修改                   | Go 领域模型、迁移、API                                               |
| Person / Chat             | Ask User 原 Run 恢复（000006）；#266 只读单会话 seam（**无运行时调用者**）；#267 分享退役开关（默认关闭） | Track D staged structured_answer 源身份校验                         | 唯一约束、串行创建、保留窗口（PC-RET-001）                           |
| Diagnosis V2              | 合约 handoff 文档（DS-DX-01）                                                                             | —                                                                   | ConditionEntry 持久化与决策事务（DX-001..003）                       |
| Training / Safety         | #270/#271/#273：dormant 多活动兼容、`LIMIT 2` fail-closed 读取、owner 行锁、离线迁移预演脚本              | #274 纯函数预提交策略（CI 绿，未合并）                              | 真实迁移、PlanProposal 运行时、Safety restriction 模型（TP-001）、UI |
| UserAsset / Media         | —                                                                                                         | —                                                                   | 全部（AS-001/002）                                                   |
| React / Penpot            | V2 Body Canvas 主框架（#255 合入）                                                                        | React 工作树 3 个 staged 文件（`useBusinessWindowFocus`），租约仍在 | 完整领域 UI、Penpot 视觉验收                                         |

## 4. 代码质量评审（e4160a9c7 vs 6bbc36acb，及 PR #274）

**总体：** 新增生产代码质量较高——fail-closed、身份不泄露的哨兵错误、严格的 owner 作用域、`LIMIT 2` 有界查询、默认保持旧行为，测试覆盖正负向与 PG18 原生路径。PR #274 的 7 天 TTL 来自 `docs/product/domain/README.md` INV-43，非臆造。主要问题集中在**锁选择、休眠代码累积和治理开销**。

### Q1（P1）· Owner 锁使用 `FOR UPDATE` 锁 `users` 行，会阻塞该用户所有子表外键插入

`TrainingRepository.LockOwnerForPlanActivation` 执行 `SELECT id FROM users WHERE id=? FOR UPDATE`。PostgreSQL 中 `FOR UPDATE` 与 `FOR KEY SHARE` 冲突，而**任何引用 `users(id)` 的外键插入**（消息、BodyFact、Observation、Episode 等）都会对父行取 `FOR KEY SHARE`。因此训练事务持锁期间，同一用户在其他设备上的聊天/记录写入会被阻塞直到训练事务结束；若另一事务先插入子行再进入训练流程，还可能形成锁等待链。

**建议：** 改为 `FOR NO KEY UPDATE`（GORM `clause.Locking{Strength: "NO KEY UPDATE"}`）——它在锁持有者之间仍然互斥，但不与外键的 `FOR KEY SHARE` 冲突；或改用事务级 advisory lock `pg_advisory_xact_lock(<namespace>, hashtext(user_id))`。在 `test-tp-d1-owner-lock-pg18.sh` 中增加“持锁期间同用户插入 BodyFact 不被阻塞”的负向用例。功能当前休眠，修正成本最低的窗口就是现在。归属：TP-002 前置项。

### Q2（P2）· 休眠代码在累积，缺少接线/过期登记

- `WithIndependentTrainingPlans()`：生产中无任何构造器使用。
- `ConversationRepository.GetSoleLiveByUserID`：无运行时调用者。
- `RetireChatShares`：默认关闭；开启时 OpenAPI 仍公布 share 端点（契约与路由行为不一致）。

每一项单独都合理，但没有统一登记“哪个工单负责接线、何时删除旧路径”。**建议：** 在 Master 或本文件维护一张 Dormant Feature Register（开关/选项、负责工单、接线条件、旧路径删除条件），并让 `plan:check` 校验每个 dormant 项都有 owner 工单。

### Q3（P2）· 领域规则存在 JS 参考模型与 Go 实现两套真相源

`scripts/plan/` 下有 `v2-trainer-safety-model`、`v2-chat-retention-model`、`v2-body-hypothesis-model` 等 JS 参考模型测试；同时 Go 端开始出现 `internal/domain/trainingplan`。参考模型适合在 Go 实现前固化语义，但 Go 版本落地后若两边独立演进将产生漂移。**建议：** 每个域一旦有 Go 领域包，就把 JS 模型的用例抽成共享 JSON fixture，由 Go 测试与 JS 测试共同消费；或在 Go 落地后将 JS 模型降级为归档。

### Q4（P2）· 治理产物/产品代码比例失衡

自 #255 以来 main 新增行数：`scripts/plan` 3724、docs 1247、测试 1124、生产代码 302。工单文档、证据 JSON、PG18 预演脚本本身质量很高，但它们挤占了 Agent 的执行时长。**建议：** 每个工单的“证据”默认只要求 exact-head CI + 一份简短 receipt；PG18 预演脚本仅在涉及迁移的工单中编写；禁止为已合并、已有 CI 的事实再生成新的叙述性文档段落。

### Q5（P3）· 小问题

- `GetActivePlanByUserID` 中 `errors.New("valid training plan owner required")` 未定义为哨兵，与同文件其他错误风格不一致，调用方无法 `errors.Is`。
- PR #274 中 `DecisionKind` 使用裸字符串 `"accept"`，建议与 `ProposalKind` 一样定义类型常量。
- PR #274 的 CI 跳过了 PG18 children——纯函数包可以接受，但合并说明中应写明“无迁移，PG18 skip 合理”，避免将来被误读为全量绿灯。
- context witness（`independentTrainingOwnerLockKey`）只证明“曾在某事务中取锁”，不证明与当前 `txCtx` 是同一事务；当前调用路径正确，但后续接线时应保证 witness 只在同一 `WithinTransaction` 闭包中派生。

## 5. 优化后的下一轮实施顺序

原则不变：不重新设计体系，以 main 为唯一共享基线，最多 3 个独立 writer，Gate 只由证据提升。调整点是**先解除写入阻断和基线漂移，再扩大产出**。

### P0 — 先让成果能落地（今天）

1. **P0-1 V12 收尾（BS-OBS-001）**：owner 将 3 个未推送提交 rebase 到 `e4160a9c7`，推送、开 PR、exact-head CI（必须包含 PG18 children），随后跑 v12 更正/撤回的原生浏览器 E2E；合并后释放 `ENG-BS-04` 锁，BS-OBS-001 → done，解锁 UI-001。
2. **P0-2 解除 Track D 写入阻断**：确认平台安全控制拒绝的是哪一类操作（commit / docker / pnpm），由用户在调度端调整权限，或把 Track D 改为“只出 patch 文件 + 由 Track C/人工提交”的模式。在解除前，Track D 不应再承担任何需要提交的工单。
3. **P0-3 合并 PR #274（TP-PROP-001）**：先修 Q5 两个小问题（可选），确认无迁移后按正常分支保护合并。
4. **P0-4 对齐文档**：合入本分支（queue/handoff/README/Master 修正）。之后每次合并 PR 时由 Integration owner 同步更新 `mainBaselineSha` 与 `latestExecution`，而不是事后批量补记。

### P1 — 恢复四线职责（P0 完成后）

| 线                | 下一个工单                                                                            | 前置                                                             | 说明                                                          |
| ----------------- | ------------------------------------------------------------------------------------- | ---------------------------------------------------------------- | ------------------------------------------------------------- |
| A · Domain/Schema | **BS-HYP-001**（Hypothesis/ConcernGroup 投影）→ **DX-001**（Diagnosis 读取/决策契约） | owner 先处理 Domain 工作树未提交修改                             | 新建工作树从 `e4160a9c7` 起步；不再修改已收敛的 BodyFact 契约 |
| B · React UI      | **UI-001**（V12 Person 记录与 Episode UI 状态）                                       | BS-OBS-001 done；React 工作树 staged 改动需 owner 提交或放弃租约 | 只消费已批准 ViewModel                                        |
| C · Backend       | **TP-001**（restriction 级 Safety）→ **TP-002**（含 Q1 锁修正、真实多活动迁移）       | TP-PROP-001 合并                                                 | 之后再进入 DX-002                                             |
| D · Integration   | **PC-001** 剩余部分 + **QA-001** + 每次合并后的基线同步                               | P0-2 解除                                                        | 维持 staging 晋升阻断                                         |

### P2 — 工作树卫生（不得强制清理他人工作）

- 可在 owner 确认后移除：`bodysense-observation-v11`（已合入，干净）、`bodysense-v2-ci-int002`（INT-002 done，干净）、`bodysense-v2-integration`（INT-001 done，干净）、`bodysense-v2-pc-singleton`（detached，干净）。
- `.forgeflow-control/bodysense` 落后 361 提交，若 ForgeFlow 控制面不再使用应归档。
- 默认工作树 `~/projects/bodysense` 不应再作为状态查看入口；查看进度一律用 `git show origin/main:<path>`。

## 6. 本次审计未做的事

未修改任何调度任务、其他 writer 的工作树或锁、GitHub PR 状态、staging/production；未提升任何 Gate；未运行 PG18 或浏览器测试（只引用已有作业 exit code）。ChatGPT 调度器本身的启停状态无法从 GCP 直接读取，§1 中的 Track 活动时间以 Agent Fabric 作业记录为证。
