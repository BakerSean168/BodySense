# BodySense V2 · Penpot 可编辑原型交接与验收检查点

**状态：设计生产有可核实产物；业务 Flow、像素级和窄屏验收未通过，因此 PD-BASE-01 等设计门禁不得标记 ready。**

时间：2026-10-13。工程分支：`design/v2-viewmodels-penpot-20261013`；基础为 PR #247 的 `refactor/schema-first-v2-20261013`，不涉及旧 UI PR #244 工作树。文件与页面以 [当前 Product 设计稿](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) 为准。

## 已有实际写入证据

- `penpot-scene-manifest.json`：从 Penpot MCP 成功创建时返回的 shape IDs 汇总 **42 个可编辑 V2 状态画板**，区分 desktop/mobile，保留每项真实定位 ID；其中最初 5 个画板有原始交互 Shape 定位。不可仅凭数量推定全部可交互或对齐。
- `penpot-scene-manifest.json.components`：创建了 **14 个真实可复用 Penpot 组件**，含按钮主/次/幽灵、Chat/关闭/用户图标、状态、局部通知、单选项、证据来源行。与既有字体、窗体色彩 token 同步，非静态截图。
- Overlay 的独立 QA 画板已经通过 Viewer 点击验证：打开 First overlay → 切换 Second overlay → 关闭。该结果只涵盖 Overlay 操作机制，与主体业务 Flow 验收不同。
- **没有创建 V2 正式业务 Flow 起点和完整跨窗口连线的可验证记录**。早期 `/home/ubuntu/.tmp/bodysense-v2-scene-engine.js` 是未执行的脚本候选，含“提交回答立即转为失败/成功”“接受提案直接当作 DomainCommitReceipt”“训练动作或资料擦除直接跳到成功”等不满足领域模型的硬编码示意。**严禁直接执行该脚本生成伪流程。**

## 已落实工程契约

- `penpot-flow-contract.json`：6 组 Flow、37 条转移及每条的本地行为/明确命令/后端条件；Diagnosis 的 `decision_recorded` 与 `committed` 分离，Trainer 的实际 `complete_set` 与点击“下一个”分离，资产删除的 `retryable_failed` 与已核验 `completed` 分离。
- `scripts/design/validate-v2-penpot-handoff.mjs`：核查具体已存在的画板和组件 ID、所有边的两端与触发方式、**禁止无条件跳到保存/擦除/医学结果**。7 条负面和正面测试验证这些静态设计约束。工具只读这两份仓库内 JSON，不与 Penpot API 写入产生混淆。
- 真实 Records/Habits 表面由 `apps/web/src/features/body-canvas/model/bodyFactV2Adapter.ts` 将已经经过生成客户端校验的 BodyFact API 事实转换为 `boundedBodyFactVM`，仅暴露确实返回的来源 evidence ID、字段和状态。来源属性、Observations、Hypotheses、Episodes、ConcernGroups 都明确标记为尚未接入，**不是成功查询得到的空数组**。
- `BodyFactRecords` 消费 bounded VM 的显示状态，与已运行的 owner-bound QueryClient 和修订校验共用同一真实 ledger；不添加持久化副本，不更改后端接口或 SQL Schema。

## 尚待关闭的可测问题

| 工作项 | 当前证据 | 通过门槛 |
| --- | --- | --- |
| PD-BASE-01 | 14 个组件、42 个状态画板、Overlay QA；设计源可编辑 | 继续在当前 Product/Components 页面建立全局人体 Canvas + Dock + 独立悬浮窗的真实 Flow，并做 Viewer 逐步点击 |
| PD-BS-01 | Records/Habits 的 bounded API → VM → UI 已接；Episode 仍为目标契约 | 当前 owner、partial-empty/stale、来源不详、Episode 依据与跨窗口跳转均有桌面/390px 真正 Viewer 证据 |
| PD-PC-01 | 同一 Run Ask User 已有真实 React 实现；Penpot 创建了选项/重试/Running 状态画板 | 插件在 Designer 里完成固定位置/滚动/选择/提交/错误分支，且真实 Run 确认不新建会话 |
| PD-DX-01 | 有 sequential proposal 与 atomic commit 候选画板 | 只有领域确认回执出现后才到提交成功画板；Dock 计数来自新的 adopted 查询，医学来源真实可追溯 |
| PD-TP-01 | 有 plan/proposal/trainer/safety 画板 | 安全 gate、set instance 身份、pause/resume、截断时间和移动端不会假造完成 |
| PD-ASSET-01 | 原件删除/关联数据擦除画板已创建 | 补齐 `propagating` 中间状态，完整 scope verification 后才能显示 finished |
| IR 交接 | 类型、契约单测、Penpot 画板来源已记录 | 浏览器键盘/滚动/中文字体/文本溢出/实际 viewport 与后端权限、并发恢复一起验收 |

**环境阻塞：** Oracle2 官方 Penpot 编辑器标签页在 2026-10-13 返回 `The Penpot plugin tab appears to be suspended by the browser (no heartbeat...)`。尝试将 editor tab bring-to-front 和单页 reload 后仍无插件 heartbeat。已成功写入的形状定位证据来自本次历史成功执行记录；**当前不可据此称设计稿交互已重新现场通过。** 重新恢复官方插件连接后，从 `penpot-scene-manifest.json` 直接定位现有画板，不删除、重建或覆盖旧 Product 结构。

## 下一次编辑的执行顺序

1. 先只读检查 Product/Components 的 manifest ID 是否仍存在，并检查画板尺寸、溢出、可编辑组件实例关系。
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
