# BodySense V2 · Penpot 可编辑原型交接与验收检查点

**状态：已验证 20 个 Canvas 组合场景、77 条本地导航及 7 组/20 步 Viewer 点击；领域写入、独立滚动、视觉与完整移动端验收仍未签收，因此任何设计门禁仍不得标记 ready。**

时间：2026-10-13。工程分支：`design/v2-viewmodels-penpot-20261013`；基础为 PR #247 的 `refactor/schema-first-v2-20261013`，不涉及旧 UI PR #244 工作树。文件与页面以 [当前 Product 设计稿](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) 为准。

## 2026-10-13 最新现场审计

官方 Penpot MCP 从 Product 页面真实图层及 Interaction 对象回读 **20 个组合场景（Desktop 11 / Mobile 9）、77 条本地 `navigate-to` 导航、7 个正式 Flow 起点**。每条导航的操作形状 ID 与目标画板 ID、设计场景及来源约束均写入 [`penpot-live-scenes-manifest.json`](./penpot-live-scenes-manifest.json)，并由新增加的 `scripts/design/validate-v2-penpot-live-scenes.mjs` 和 8 项安全测试交叉验证。

Viewer 浏览器已执行 **7 组/20 步点击回归**：两端 Dock 跨窗切换、移动端普通聊天打开关闭、等待用户回答时关闭／重新打开，以及已选择但未提交的答案关闭／重开仍保留选择。所有测试都有时间与真实画板到达证据。此处的“同一 Run”只是合成设计数据的状态演示；真实 Run ID、提交回执、医学诊断/训练成功/擦除成功不由 Penpot 模拟。中间内容已能显示，但**顶部/底部固定且中段独立滚动、真实 390px viewport、键盘和像素级视觉验收尚未通过**。

以下更早检查点描述的是分阶段产物，阅读时请以上述最新现场审计为准。

## 已有实际写入证据

- `penpot-scene-manifest.json`：从 Penpot MCP 成功创建时返回的 shape IDs 汇总 **42 个可编辑 V2 状态画板**，区分 desktop/mobile，保留每项真实定位 ID；其中最初 5 个画板有原始交互 Shape 定位。不可仅凭数量推定全部可交互或对齐。
- `penpot-scene-manifest.json.components`：创建了 **14 个真实可复用 Penpot 组件**，含按钮主/次/幽灵、Chat/关闭/用户图标、状态、局部通知、单选项、证据来源行。与既有字体、窗体色彩 token 同步，非静态截图。
- Overlay 的独立 QA 画板已经通过 Viewer 点击验证：打开 First overlay → 切换 Second overlay → 关闭。该结果只涵盖 Overlay 操作机制，与主体业务 Flow 验收不同。
- 首批七个 Desktop 组合场景已扩展至 **11 个桌面、9 个移动端**共享人体 Canvas 场景。77 条 `navigate-to` 及 7 个 Flow 起点由官方 MCP 实际回读，不以脚本的预期输出代替证据；独立记录的 7 组／20 步 Viewer 点击全部通过，但完整业务条件 Flow 仍未签收。
- 悬浮窗组合清除了继承的错误裁切／固定属性，Viewer 中 Records 的五标签、事实卡、来源与局部提示已正常显示。分类、Dock、Chat Draft、等待回答、已选未提交答案的关闭／重开等局部导航已做真实 Viewer 点击回归；独立中段滚动、完整窄屏视觉仍须额外验收。原型不代表后端提交、训练或擦除完成。
- 早期 `/home/ubuntu/.tmp/bodysense-v2-scene-engine.js` 是未执行的脚本候选，含“提交回答立即转为失败/成功”“接受提案直接当作 DomainCommitReceipt”“训练动作或资料擦除直接跳到成功”等不满足领域模型的硬编码示意。**严禁直接执行该脚本生成伪流程。**

## 已落实工程契约

- `penpot-flow-contract.json`：6 组 Flow、37 条转移及每条的本地行为/明确命令/后端条件；Diagnosis 的 `decision_recorded` 与 `committed` 分离，Trainer 的实际 `complete_set` 与点击“下一个”分离，资产删除的 `retryable_failed` 与已核验 `completed` 分离。
- `scripts/design/validate-v2-penpot-handoff.mjs`：核查具体已存在的画板和组件 ID、所有边的两端与触发方式、**禁止无条件跳到保存/擦除/医学结果**。7 条负面和正面测试验证这些静态设计约束。工具只读这两份仓库内 JSON，不与 Penpot API 写入产生混淆。
- 真实 Records/Habits 表面由 `apps/web/src/features/body-canvas/model/bodyFactV2Adapter.ts` 将已经经过生成客户端校验的 BodyFact API 事实转换为 `boundedBodyFactVM`，仅暴露确实返回的来源 evidence ID、字段和状态。来源属性、Observations、Hypotheses、Episodes、ConcernGroups 都明确标记为尚未接入，**不是成功查询得到的空数组**。
- `BodyFactRecords` 消费 bounded VM 的显示状态，与已运行的 owner-bound QueryClient 和修订校验共用同一真实 ledger；不添加持久化副本，不更改后端接口或 SQL Schema。

## 尚待关闭的可测问题

| 工作项 | 当前证据 | 通过门槛 |
| --- | --- | --- |
| PD-BASE-01 | 14 个组件、42 个源画板、20 个组合场景、77 条连线、7 组/20 步 Viewer QA、Overlay QA | 完整交互、独立滚动、390px 实机视口、键盘、视觉与业务状态验收通过前保持 in_progress |
| PD-BS-01 | Records/Habits 的 bounded API → VM → UI 已接；Episode 仍为目标契约 | 当前 owner、partial-empty/stale、来源不详、Episode 依据与跨窗口跳转均有桌面/390px 真正 Viewer 证据 |
| PD-PC-01 | React 原 Run Ask User 已有实现；Penpot 区分普通 Chat、Ask User、答案已选、waiting/selected 关闭与重开；Viewer 两端验收选中态保留 | Run ID、interaction ID、后端提交/失败重试及真实滚动仍须联合验收 |
| PD-DX-01 | 有 sequential proposal 与 atomic commit 候选画板 | 只有领域确认回执出现后才到提交成功画板；Dock 计数来自新的 adopted 查询，医学来源真实可追溯 |
| PD-TP-01 | 有 plan/proposal/trainer/safety 画板 | 安全 gate、set instance 身份、pause/resume、截断时间和移动端不会假造完成 |
| PD-ASSET-01 | 原件删除/关联数据擦除画板已创建 | 补齐 `propagating` 中间状态，完整 scope verification 后才能显示 finished |
| IR 交接 | 类型、契约单测、Penpot 画板来源已记录 | 浏览器键盘/滚动/中文字体/文本溢出/实际 viewport 与后端权限、并发恢复一起验收 |

**连接恢复与稳定性：** 2026-10-13 的官方 Penpot 插件曾进入 `no heartbeat`。只关闭并重新打开原编辑器标签页后，恢复 MCP 能读写真实 Product 设计；Viewer 仍保留。发现一次性调用 42 次 `findShapeById` 易冻结编辑器，改为从 `penpot-scene-manifest.json` 轻量恢复缓存，并以 1–3 个场景为小批次创建设计。自动化辅助脚本位于 Oracle2 `/home/ubuntu/.tmp/bodysense-v2-{hydrate-lite,compose-core,fix-floating-overflow}.js`，属于临时可恢复会话代码；**Penpot 页面中已写入的组件、画板与交互是持久的，插件 `storage` 不是。** 真实 Viewer 已验证 Home → Records，但不等于其他业务 Flow 全部通过。

## 下一次编辑的执行顺序

1. 从 [`penpot-live-scenes-manifest.json`](./penpot-live-scenes-manifest.json) 的 20 个根画板和 77 条已保存交互 ID 定位继续；先只读校验 Product 根图层及组件，严禁重复生成既有组合场景。
2. 以 `penpot-flow-contract.json` 的业务条件为准，从现有 Canvas 场景挂 Dock/浮窗导航，再连接本地非提交交互。**API 尚不存在时只做条件化概念原型并标注；不可用的按钮必须明确禁用**。
3. 为 Chat 回复/诊断批量回执/训练组完成/擦除 verifier 的结果分支分别创建被授权的实际成功和失败场景；补齐缺失的 loading、empty、permission_denied、stale、offline 代表性变体，不重复粘贴一套同质静态屏幕。
4. 做真实 Penpot Viewer 桌面 + 390px 的点击、滚动、Overlay 关闭后 Run 持续、缩放/剪裁/文字溢出校验。记录实际缺陷与截图，不用静态形状数量代替验收。
5. 经过 Track A 审核后才更新 `docs/plan/active/v2-handoff-state.json` 相应领域状态。主 Canonical/ENG-BS-02 写锁期间，不跨 worktree 修改 Schema 和迁移。

执行本地契约检查：

```bash
node scripts/design/validate-v2-penpot-handoff.mjs
node --test scripts/design/validate-v2-penpot-handoff.test.mjs
pnpm exec vitest run --config apps/web/vite.config.ts
```
