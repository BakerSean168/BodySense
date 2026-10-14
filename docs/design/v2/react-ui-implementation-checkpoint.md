# BodySense V2 · React Body Canvas UI 交接与验收检查点

**状态：第一批 UI 实施与真实浏览器回归通过；等待 stacked PR 的 CI 和跨领域接口接续，不能标记完整 V2 Ready 或部署生产。**

日期：2026-10-13。实现分支 `feat/v2-react-canvas-penpot-20261013`，隔离 worktree `/home/dev/projects/bodysense-v2-react-ui`，基础为 `design/v2-viewmodels-penpot-20261013` 的 `a3b8db87e`（Design PR #250，继承 Schema PR #247）。本分支不写入 Penpot、Schema worktree 或部署环境。

## 唯一设计依据与实际实现

- 官方 [Penpot · BodySense Product](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2)。读入同分支的 [可编辑场景清单](./penpot-live-scenes-manifest.json)，按 1280×900 桌面和 390×844 移动端几何参考，而不是沿用旧 V5.5 对话/人体左右分屏。
- **固定底层 3D Canvas** 不卸载。Dock 收敛为 Person、TrainingPlan、Diagnosis、Chat 四入口；Trainer 从已接受计划中进入而不占第五个 Dock 按钮。Dock 图标与快捷键从同一业务元数据获取，不另建按钮标签映射；Diagnosis 数字角标只采用当前后端投影的已确认条目，不计算待确认 AI 候选。
- **窗口组件统一**：`BodyCanvasWorkspace` + `BusinessWindowHost`/`BusinessSheetHost` 共享桌面／移动布局和隐藏但挂载的任务内容。Diagnosis/TrainingPlan 顶部为真实 ID 选择器 + 独立 Chat 入口 + Close；Chat 顶部为当前本地时间（明确不代表消息创建时间）；Person 为身份头像＋昵称触发器和五分类。
- **Diagnosis**：当前只显示一个选中、用户已确认的候选条目；待确认提案不进入病情列表；证据、原因、缺口、整体计划边界和安全提示根据已有真实字段显示，无前端虚构临床确诊或自动承诺持久化。
- **TrainingPlan**：选择器移至共享标题栏，只列当前投影为 accepted 的计划；未接受的草稿不会开启 Trainer。方案接受、执行和安全阻断仍服从原服务端 capability/safety 约束；浏览计划和打开对话不产生写入。
- **响应式层级**：Desktop 优先尺寸 Person 780×680、Diagnosis 560×680、TrainingPlan 560×720、Chat 560×640，受窗口高度与 Dock 空间约束；390px Mobile 内边距为 16px、窗口可自滚动、底部 Dock 四入口持续可见。动作播放条/提示/移动工具栏与 Dock 有明确间距约束，避免遮挡。

## 验收证据（真实执行，非拟定）

| 验收 | 结果 | 范围 |
| --- | --- | --- |
| React/Vitest Web 回归 | **73 个文件，440/440 PASS** | 含新 Dock、标题选择器、Diagnosis 可见条目、拒绝未接受计划执行、旧版 UI 行为回归迁移 |
| TypeScript 类型检查 | **PASS** | `pnpm exec tsc -p apps/web/tsconfig.json --noEmit` |
| Vite Web 生产构建 | **PASS** | `pnpm exec vite build --config apps/web/vite.config.ts` |
| 正式 API + Chromium E2E | **PASS** | `E2E_API_BASE_URL=http://127.0.0.1:20101 E2E_BASE_URL=http://127.0.0.1:20580 pnpm exec playwright test apps/web/e2e/body-canvas-v3.spec.ts --project=chromium --workers=1 --reporter=line`；隔离注册、记录、区域选择、Chat、动作演示、真实 Equipment 持久化、桌面/移动响应式 |
| 额外浏览器布局 QA | **PASS** | 1280×900、390×844 四个真实 React 窗口轮换；真实 API E2E 再覆盖 1280×720、1024×650、390×844 动作条与 Dock 的矩形不重叠、移动动作切换与标题不重叠 |
| Penpot 逐像素验证 | **OPEN** | 当前只按场景组件与关键几何基准实现；设计师并行完成精细化后做最终像素级比对 |
| 领域授权端到端 | **OPEN** | Diagnosis V2 原子采纳、Training V2 逐组执行、Chat Run 事件、资料擦除的服务端权威状态仍由对应领域工程线完成 |

最终日志：`.runtime/v2-react-ui/{full-web-tests-final.log,web-tsc-final.log,web-build-final.log,body-canvas-e2e-trainer-final.log}`；初期几何验收另见 `body-canvas-v2-geometry-e2e.log` 和 `final-visual-check.log`。Playwright 图片证据由该隔离工作树的 `test-results/` 产生。临时合成 HTML/Fixture 仅为初期纯窗口几何验证，**不得随 PR 提交**。

## 导航与 Trainer 的额外防绕过检查

- 统一快捷键 `I → Person`、`P → TrainingPlan`、`D → Diagnosis`、`/ → Chat`；`T → Trainer` 只导航，**不表示开始训练**。已有 `C` 等历史键仍保留兼容。
- Trainer 挂载旧 `TrainingExecutionPanel` 前需要**本页面明确点击 Start 记录的精确 planId**与当前 `accepted` 计划一致、Go workspace 允许执行、当前无安全阻断，且计划读取非 Loading/Failed。仅通过 URL `?view=trainer`、快捷键 `T`、外部 `windowRequest` 或持久化 planId，不构成进入条件。
- 入口未满足时显示有原因的非执行占位和“查看训练计划”导航。Plan/Capability/Safety 改变会使执行面板 fail-closed；跨计划重新进入用稳定 `key=planId` 隔离局部训练状态。真实执行命令仍需 Go 持久授权，前端 Guard 不是临床/训练业务授权回执。
- 新增 `TrainerEntryGuard` 组件测试、快捷键模型回归、ConsultationPage 深链/快捷键组件验收和真实浏览器 `T/I/Escape` 场景。本次没有重写遗留 Trainer 内的逐组与训练反馈数据模型，该部分归 ENG-TP-01。

## 与并行实现的边界

1. **不把旧 Candidate 投影冒充已完全实现的 Diagnosis V2**。目前 `projectDiagnosisView` 是真实旧 DTO 的保守适配，缺少稳定 ConditionEntry、历史回放、明确临床来源等。后端新契约落地后替换 adapter，UI 结构保持。
2. **不把旧 TreatmentRevision 当作完整 Training V2**。当前显示受 `projectTrainingPlans` 对旧 accepted/current 的识别限制，多份正式方案/模板/训练逐组会话仍待后端 ViewModel 与命令对接。
3. **Person Records/Habits** 继续沿用前一设计 PR 的真实 BodyFact 有界适配器，保留失败、缺失信息和已确认/尚未确认的状态差异；没有重新实现平行数据存储。
4. **不自行签收设计/后端/发布门禁**。本分支进入 PR 后只可证明 UI 切换、尺寸、可用性及旧后端的已覆盖真实 E2E；完整 Domain/API/CI 发布须由 Track D 依据当前精确 SHA 再核验。

下一轮：接收另一个 Penpot 会话最终确认的组件样式和交互细节，针对真实 React 再做像素级对齐与可访问性/滚动验收；同时对接正式的 Diagnosis、Training、PersonAssets 查询模型，去掉旧 transport 的临时适配边界。
