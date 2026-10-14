# BodySense V2 · Penpot 可编辑原型交接与验收检查点

**状态：官方 Penpot 已核验 36 个 Canvas 组合场景、162 条本地导航、18 个 Flow 起点、14 个受服务端约束的命令控件；17 组／48 步 Viewer 导航及 Person 身份浮层的 2 步点击均通过。领域写入、独立滚动、视觉、完整移动端验收仍未签收，所有设计门禁继续保持 OPEN。**

时间：2026-10-13。工程分支：`design/v2-viewmodels-penpot-20261013`；基础为 PR #247 的 `refactor/schema-first-v2-20261013`，不涉及旧 UI PR #244 工作树。文件与页面以 [当前 Product 设计稿](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) 为准。

## 2026-10-13 · 最新 Penpot 实际回读／Viewer 验收

- **组合场景 36**：Desktop 27、Mobile 9，均为真实共享人体 Canvas 和可编辑窗口形状；既有 42 个独立 V2 状态源画板与 14 个组件继续保留，不重建截图。
- **本地导航 162 条、正式 Flow 起点 18 个**：均有官方 Penpot MCP 读回的源形状、目标场景 ID。所有转移只处理窗口、分类、选项与上下文，不把点击等同服务端授权业务提交。
- **业务按钮 14 项服务端守卫**：Diagnosis 的正式复核、TrainingPlan 接受/拒绝、Trainer 逐组完成/暂停/恢复/提前结束、资料删除范围及清理重试等均存储了命令与前置回执要求，原型中没有无条件成功导航；详见 [`penpot-live-scenes-manifest.json`](./penpot-live-scenes-manifest.json) `guardedCommands`。
- **真实 Viewer 17 组／48 步通过**：新增 Person 五分类连续切换、Diagnosis 当前→历史→上下文 Chat、已接受计划→调整预览→关闭、Trainer 正在进行／已暂停的关闭恢复、Safety scoped restriction、UserAsset 清理预览与失败重试、原件不可用、人体区域上下文；结合前一轮 Chat/Ask User 同 Run 合成状态恢复。身份 Popover 另有打开/关闭两步真实点击记录。每步具有实际点击的 Penpot hotspot shape ID、目标画板 ID、用例次序与时间戳，不能用造假的导航图替代。
- **画板坐标审计 36/36**：9 个右侧 560px 浮窗从 1440px 画板的 `x=840` 调整至 `x=672`，以便在 1280px Viewer 中保留至少 48px 的右缘空间；移动端 390px 源画板窗口保持 16px 左右边距。这里是官方形状的**设计坐标核验**，**不等于**已做 390px 实机浏览器截图、键盘、滚动或逐像素验收。
- **设计自动校验**：`validate-v2-penpot-live-scenes.mjs` 现在交叉验证 36 个场景、162 个唯一热点、17 条独立 Viewer 用例的 48 步连续轨迹、18 个起点、共享身份 Overlay 与 14 个受保护命令，以及画板几何边界；对错误目标 ID、伪造点击证据、越界窗口、把服务端动作当本地跳转等有负面测试。

**仍未完成：** 真正的业务 Run／DomainCommitReceipt／按范围清理回执、悬浮窗内部固定 Header/Footer 与中间滚动、全流程响应式／390px Viewer、键盘／Esc／焦点恢复、真实数据与 3D 局部选区集成、视觉像素对齐。上述 Local Viewer PASS 不提升 Domain 或集成发布 Gate 状态，不允许标记 `ready`。

## 2026-10-13 · 早一轮 20 场景审计（历史检查点）

官方 Penpot MCP 从 Product 页面真实图层及 Interaction 对象回读 **20 个组合场景（Desktop 11 / Mobile 9）、77 条本地 `navigate-to` 导航、7 个正式 Flow 起点**。每条导航的操作形状 ID 与目标画板 ID、设计场景及来源约束均写入 [`penpot-live-scenes-manifest.json`](./penpot-live-scenes-manifest.json)，并由新增加的 `scripts/design/validate-v2-penpot-live-scenes.mjs` 和 8 项安全测试交叉验证。

Viewer 浏览器已执行 **7 组/20 步点击回归**：两端 Dock 跨窗切换、移动端普通聊天打开关闭、等待用户回答时关闭／重新打开，以及已选择但未提交的答案关闭／重开仍保留选择。所有测试都有时间与真实画板到达证据。此处的“同一 Run”只是合成设计数据的状态演示；真实 Run ID、提交回执、医学诊断/训练成功/擦除成功不由 Penpot 模拟。中间内容已能显示，但**顶部/底部固定且中段独立滚动、真实 390px viewport、键盘和像素级视觉验收尚未通过**。

本节及下方旧数据是阶段性历史记录；**当前事实以本页首部 36 场景／162 连线／17 组 Viewer 及最新 Manifest 为准**。

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
| PD-BASE-01 | 14 个组件、42 个源画板、36 个组合场景、162 条连线、18 个起点、17 组/48 步 Viewer QA，身份 Overlay 2 步 QA，14 个命令守卫 | 完整端到端与授权、独立滚动、390px 浏览器视口、键盘、视觉及真实业务状态验收前保持 OPEN |
| PD-BS-01 | Records/Habits 的 bounded API → VM → UI 已接；Episode 仍为目标契约 | 当前 owner、partial-empty/stale、来源不详、Episode 依据与跨窗口跳转均有桌面/390px 真正 Viewer 证据 |
| PD-PC-01 | React 原 Run Ask User 已有实现；Penpot 区分普通 Chat、Ask User、答案已选、waiting/selected 关闭与重开；Viewer 两端验收选中态保留 | Run ID、interaction ID、后端提交/失败重试及真实滚动仍须联合验收 |
| PD-DX-01 | 有 sequential proposal 与 atomic commit 候选画板 | 只有领域确认回执出现后才到提交成功画板；Dock 计数来自新的 adopted 查询，医学来源真实可追溯 |
| PD-TP-01 | 有 plan/proposal/trainer/safety 画板 | 安全 gate、set instance 身份、pause/resume、截断时间和移动端不会假造完成 |
| PD-ASSET-01 | 原件删除/关联数据擦除画板已创建 | 补齐 `propagating` 中间状态，完整 scope verification 后才能显示 finished |
| IR 交接 | 类型、契约单测、Penpot 画板来源已记录 | 浏览器键盘/滚动/中文字体/文本溢出/实际 viewport 与后端权限、并发恢复一起验收 |

**连接恢复与稳定性：** 2026-10-13 的官方 Penpot 插件曾进入 `no heartbeat`。只关闭并重新打开原编辑器标签页后，恢复 MCP 能读写真实 Product 设计；Viewer 仍保留。发现一次性调用 42 次 `findShapeById` 易冻结编辑器，改为从 `penpot-scene-manifest.json` 轻量恢复缓存，并以 1–3 个场景为小批次创建设计。自动化辅助脚本位于 Oracle2 `/home/ubuntu/.tmp/bodysense-v2-{hydrate-lite,compose-core,fix-floating-overflow}.js`，属于临时可恢复会话代码；**Penpot 页面中已写入的组件、画板与交互是持久的，插件 `storage` 不是。** 真实 Viewer 已对 17 组局部交互完成点击验证，但不是业务 Flow 全部签收。

## 下一次编辑的执行顺序

1. 从 [`penpot-live-scenes-manifest.json`](./penpot-live-scenes-manifest.json) 的 36 个根画板、162 条本地交互与 14 个受保护业务控件精确 ID 定位继续；先只读核验 Product 图层，严禁重复生成既有组合画板。
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
