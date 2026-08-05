# 问诊工作台渲染性能优化 — 实施任务拆分

> 进行中（2026-07-06）。基于 [consultation-render-performance-optimization-plan.md](./consultation-render-performance-optimization-plan.md) 生成，聚焦前端问诊工作台的可执行实施路径。

## 总体路线

本次优化按 4 个 Phase 推进，优先解决用户直接感知最强的问题，再处理预取和拆包，最后补充指标与回归防线。

```text
Phase 1 壳体稳定化
  -> Phase 2 历史会话切换优化
  -> Phase 3 路由级拆包
  -> Phase 4 指标与回归收口
```

建议顺序：

1. 先消灭白屏和整页闪烁
2. 再降低首次点击历史会话的等待
3. 再缩短首包和主线程阻塞
4. 最后固化基线和回归检查

## 当前状态（2026-07-06）

| Phase | 状态 | 当前结果 |
|-------|------|----------|
| Phase 0 基线记录 | 🟡 部分完成 | 已记录 production build 包体变化，尚未补浏览器 Performance 录制 |
| Phase 1 壳体稳定化 | ✅ 完成 | `AuthBootstrap`、`ProtectedRoute` 壳体骨架、问诊页去整页 loading、三块局部 skeleton |
| Phase 2 切换优化 | ✅ 核心完成 | thread placeholder 保留旧数据、会话预取、panel overlay、runtime 重建边界收口 |
| Phase 3 路由拆包 | ✅ 核心完成 | 页面级 `lazy()`、问诊页独立 chunk、`AssistantChatPanel` / `InfoPanel` / `DiagnosisPanel` 拆分 |
| Phase 4 回归与收口 | 🟡 进行中 | 已补 4 个最小回归测试，已补验证清单，尚未完成人工复测报告 |

---

## Phase 0：建立基线与观察面

**目标**：在改代码前先固定当前问题基线，避免后续只能凭体感判断。

### Task 0.1：记录首屏与切换基线

**范围**

- 记录刷新 `/consultation` 时的可见白屏时长
- 记录首次点击未缓存历史会话时的整页闪烁情况
- 记录生产构建后的入口包体积

**建议动作**

1. 本地 production build 一次，记录入口 JS/CSS 体积
2. 浏览器 Performance 面板录制两个场景：
   - 刷新 `/consultation`
   - 首次点击一个未缓存历史会话
3. 记录以下观测项：
   - 白屏开始到 Shell 首次出现
   - Shell 出现到会话线程可见
   - 历史会话点击后是否发生整页重绘

**输出物**

- 一份简短基线记录，写回当前文档或另附录

**验收标准**

- 有明确的“改造前”观测记录
- 包体数字和关键交互路径被留档

---

## Phase 1：壳体稳定化与白屏消除

**目标**：刷新受保护页面时不再白屏；问诊页切换线程时不再走整页 loading。

### Task 1.1：引入全局 Auth Bootstrap 状态

**当前状态**：✅ 已完成

**目标**

把“应用启动期鉴权校验”从每个 `ProtectedRoute` 内抽离出来，避免它成为页面渲染硬阻塞。

**涉及文件**

- `apps/web/src/App.tsx`
- `apps/web/src/components/ProtectedRoute.tsx`
- `apps/web/src/stores/authStore.ts`
- 可新增：
  - `apps/web/src/components/AuthBootstrap.tsx`
  - `apps/web/src/components/AuthGate.tsx`

**实施内容**

1. 增加全局 auth bootstrap 机制，负责：
   - 从持久化 store 恢复登录态
   - 应用启动时发起一次 `/api/v1/me` 校验
   - 暴露 `resolved` / `verified` / `verifying` 等状态
2. 保证 `/api/v1/me` 不在每个私有路由 mount 时重复触发
3. 将 `ProtectedRoute` 的职责收敛为：
   - 未登录时重定向
   - 校验中显示壳体级 placeholder
   - 失效时统一重定向登录页

**注意点**

1. 不能因为提早显示 shell 而泄露业务数据
2. auth 校验失败后仍要快速清理状态
3. 不要在多个组件重复维护“是否已完成鉴权”

**验收标准**

1. 刷新受保护页面不再出现纯白空白态
2. `/api/v1/me` 在应用启动时至多发起一次校验
3. token 失效时仍能正确重定向登录页

### Task 1.2：定义受保护页面通用壳体级骨架

**当前状态**：✅ 已完成

**目标**

在鉴权未完成或路由 chunk 尚未完成前，提供稳定的应用框架骨架，而不是 `null`。

**涉及文件**

- `apps/web/src/components/layout/MainLayout.tsx`
- 可新增：
  - `apps/web/src/components/layout/AppShellSkeleton.tsx`
  - `apps/web/src/components/layout/ConsultationShellSkeleton.tsx`

**实施内容**

1. 为 `MainLayout` 配套设计一个简洁骨架版本
2. 让导航、顶部栏、内容容器可以先于真实数据出现
3. 问诊页可单独定义更接近真实布局的工作台骨架

**验收标准**

1. 刷新问诊页时，用户先看到稳定结构而不是空白
2. 骨架尺寸和真实布局接近，避免加载完成后大幅跳动

### Task 1.3：移除 `ConsultationPage` 顶层整页 loading 分支

**当前状态**：✅ 已完成

**目标**

把“线程尚未返回”从页面级 loading 改成工作台内部局部 loading。

**涉及文件**

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

**实施内容**

1. 删除或重构当前 `isPageLoading` 顶层分支
2. 页面顶层始终渲染：
   - `MainLayout`
   - 工作台 header
   - 左侧历史栏容器
   - chat panel 容器
   - info panel 容器
3. 将 loading 逻辑下沉到右侧 panel 级别

**注意点**

1. 不要破坏“新会话创建后保持 SSE 流”的现有机制
2. 顶层仍要保留错误态，但错误态也应尽量不破坏整体壳体

**验收标准**

1. 切换历史会话时，不再出现整个工作台被 spinner 替换的情况
2. header 和左侧历史栏常驻

### Task 1.4：为问诊页三块区域补局部骨架

**当前状态**：✅ 已完成

**目标**

把问诊页 loading 明确拆成：

1. 历史列表骨架
2. 聊天面板骨架
3. 信息面板骨架

**涉及文件**

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/consultation/components/SessionHistorySidebar.tsx`
- `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`
- `apps/web/src/features/consultation/components/InfoPanel.tsx`
- 可新增：
  - `apps/web/src/features/consultation/components/SessionHistorySidebarSkeleton.tsx`
  - `apps/web/src/features/consultation/components/ChatPanelSkeleton.tsx`
  - `apps/web/src/features/consultation/components/InfoPanelSkeleton.tsx`

**实施内容**

1. 历史列表未返回时显示列表卡片骨架
2. 首次进入某会话线程且无缓存时，聊天区显示局部骨架
3. 信息面板在无数据时显示结构化卡片骨架，而不是空白

**验收标准**

1. 问诊页任何时刻都不会因为数据缺失出现大面积空白
2. 切换过程只发生局部区域状态变化

---

## Phase 2：历史会话切换优化

**目标**：显著降低首次点击历史会话的等待和闪烁感。

### Task 2.1：为线程查询区分首次加载态与后台切换态

**当前状态**：✅ 已完成

**目标**

让线程 query 的状态语义更准确，不再把所有 fetching 都视为“整页不可用”。

**涉及文件**

- `apps/web/src/features/consultation/hooks/useConsultationThreadQuery.ts`
- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

**实施内容**

1. 区分：
   - 首次进入且无数据
   - 有旧数据但正在切换
   - 后台静默刷新
2. 优先使用 query 提供的状态而不是手写 `isPageLoading`
3. 评估 `placeholderData` / `keep previous data` 等策略，保留旧内容作为视觉占位

**验收标准**

1. 首次无缓存进入会话时，只有右侧面板显示局部 loading
2. 有旧线程数据时，切换过程中不会整块清空工作区

### Task 2.2：为会话卡片增加线程预取

**当前状态**：✅ 已完成

**目标**

把“点击后才请求”改成“用户表达意图后就预取”。

**涉及文件**

- `apps/web/src/features/consultation/components/SessionCard.tsx`
- `apps/web/src/features/consultation/components/SessionHistorySidebar.tsx`
- `apps/web/src/features/consultation/services/consultationQueryKeys.ts`
- `apps/web/src/lib/queryClient.ts`

**实施内容**

1. 在桌面端增加以下触发点之一或组合：
   - `pointerenter`
   - `focus`
   - `pointerdown`
2. 在移动端增加：
   - `touchstart` 或 `pointerdown`
3. 调用 `queryClient.prefetchQuery()` 预取目标线程
4. 增加防抖或去重保护，避免无意义预取洪泛

**注意点**

1. 只预取线程详情，不要把额外无关数据绑进去
2. 预取失败静默，不中断正常点击

**验收标准**

1. 鼠标悬停后点击历史会话，命中缓存概率明显提高
2. 首次点击未访问会话时的等待明显缩短

### Task 2.3：为切换中的 chat/info panel 加过渡态

**当前状态**：✅ 已完成

**目标**

避免“旧内容瞬间消失，新内容晚一点出现”的生硬跳变。

**涉及文件**

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`
- `apps/web/src/features/consultation/components/InfoPanel.tsx`

**实施内容**

1. 当目标线程正在加载且旧内容存在时：
   - 保留旧内容
   - 叠加轻量 loading veil，或
   - 只对消息区做 skeleton overlay
2. 标题和会话选中状态立即更新，避免用户误判当前会话

**验收标准**

1. 切换过程的视觉反馈明确，但不过度打断
2. 不再出现“像浏览器整页刷新”的体感

### Task 2.4：校正聊天运行时的重建边界

**当前状态**：✅ 已完成

**目标**

保证会话切换时，必要的 runtime 重建只影响 chat panel，不影响整个工作台。

**涉及文件**

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`

**实施内容**

1. 审核 `chatSessionKey`、`justCreatedRef`、`activeConversationIdRef` 的职责边界
2. 保证 `AssistantChatPanel` 的 key 变化不会外溢成整页重挂载
3. 保留新会话创建后的流式稳定性

**验收标准**

1. 切换历史会话时，只有聊天运行时局部更新
2. 新建会话转正后 SSE 流不中断

---

## Phase 3：路由级拆包与启动路径减负

**目标**：降低入口 chunk 体积和主线程首屏解析执行压力。

### Task 3.1：为页面级路由引入 `React.lazy`

**当前状态**：✅ 已完成

**目标**

把问诊页及其他大页面从同步 import 改成按路由加载。

**涉及文件**

- `apps/web/src/App.tsx`

**实施内容**

1. 将页面组件改为 `lazy(() => import(...))`
2. 对每个大页面配置合适的 `Suspense` fallback
3. 问诊页 fallback 尽量复用已做好的 shell/skeleton

**验收标准**

1. 问诊页重依赖不再进入所有路由的同步首包
2. 非问诊路由刷新时不再被问诊模块拖慢

### Task 3.2：审查问诊页重依赖的 chunk 边界

**当前状态**：✅ 已完成

**目标**

确认 `assistant-ui`、markdown、诊断区等确实被压到问诊路由 chunk，而不是残留在入口包。

**涉及文件**

- `apps/web/src/features/consultation/components/*`
- `apps/web/vite.config.ts`

**实施内容**

1. 构建后检查 chunk 结构
2. 如有必要，对个别重组件再做进一步 lazy
3. 保持拆包不破坏现有渲染顺序和运行时逻辑

**验收标准**

1. 入口包体积相对当前基线显著下降
2. 问诊 chunk 独立存在且按需加载

### Task 3.3：评估 Vite React 插件优化项

**当前状态**：🟡 已评估，暂不实施

**目标**

处理当前构建输出中的已知提示，降低转换链路负担。

**涉及文件**

- `apps/web/vite.config.ts`
- `apps/web/package.json`

**实施内容**

1. 评估从 `@vitejs/plugin-react` 切换到 `@vitejs/plugin-react-oxc`
2. 审查现有 `optimizeDeps` 配置是否仍必要
3. 确认升级不会破坏测试或构建行为

**验收标准**

1. 构建正常
2. 插件切换后无明显兼容性问题
3. 构建和启动路径至少不变差

---

## Phase 4：指标、验证与回归防线

**目标**：把这次优化从一次性修复变成可持续保持的工程基线。

### Task 4.1：补性能验证清单

**当前状态**：✅ 已完成

**目标**

为后续每次性能相关改动提供统一检查口径。

**建议清单**

1. 刷新 `/consultation` 是否白屏
2. 历史会话首次切换是否整页闪烁
3. 入口 chunk 是否异常回涨
4. 问诊 chunk 是否按需加载
5. 新建会话后的 SSE 流是否稳定

**输出物**

- 可追加到当前文档末尾，或另建小型 checklist 文档

**实际落地**

- `docs/plan/active/consultation-render-performance-validation-checklist.md`

### Task 4.2：补前端回归测试

**当前状态**：✅ 已完成

**目标**

为关键行为补上最基本的自动化防线。

**建议测试点**

1. `ProtectedRoute` 在 verifying 期间渲染骨架而不是 `null`
2. `ConsultationPage` 在切换线程时不返回整页 spinner
3. `SessionCard` 触发预取逻辑
4. `ConsultationPage` 切换线程时保留旧内容并叠加局部 overlay
5. 路由 lazy fallback 渲染正常

**涉及文件**

- `apps/web/src/components/*`
- `apps/web/src/features/consultation/components/__tests__/*`
- `apps/web/src/features/consultation/pages/*`

### Task 4.3：做一次改造后基线复测

**当前状态**：⏳ 待完成

**目标**

确认优化确实改善了用户体验，而不是只改变了代码结构。

**实施内容**

1. 重跑 production build
2. 重录刷新 `/consultation` 的性能轨迹
3. 重录首次点击未缓存历史会话的轨迹
4. 与 Phase 0 基线逐项对比

**当前阻塞**

1. 当前终端环境已可完成 build、vitest 与临时 Playwright 脚本准备
2. 但临时 Chromium 启动失败，缺少系统共享库 `libatk-1.0.so.0`
3. 当前机器没有可直接替代的系统浏览器

**结论**

1. 代码级与打包级验证已完成
2. 浏览器真实轨迹录制仍需转到具备完整浏览器依赖的环境执行

**验收标准**

1. 白屏消失或显著缩短
2. 整页闪烁消失，退化为局部过渡
3. 入口包体积下降

---

## 建议任务切片

为了便于提交和回滚，建议按下面粒度拆分：

1. `perf(web): add auth bootstrap and protected shell skeleton`
2. `perf(web): remove consultation full-page loading state`
3. `feat(web): add consultation panel skeleton states`
4. `perf(web): prefetch consultation threads from history sidebar`
5. `perf(web): preserve panel layout during conversation switches`
6. `perf(web): lazy-load route pages and split consultation chunk`
7. `test(web): add render performance regression coverage`
8. `docs: add consultation render performance implementation flow`

---

## 依赖关系

```text
Task 1.1 -> Task 1.2 -> Task 1.3 -> Task 1.4
Task 1.3 -> Task 2.1 -> Task 2.3
Task 2.1 -> Task 2.2
Task 2.1 -> Task 2.4
Phase 1 complete -> Phase 3
Phase 2 complete -> Phase 4
```

说明：

1. `AuthBootstrap` 和受保护壳体骨架要先落地，否则仍会被白屏阻塞
2. 局部 loading 边界先下沉，预取和过渡态才有明确挂载位置
3. 路由拆包最好在 Phase 1 稳定后进行，便于隔离问题来源

---

## 最小可交付版本

如果需要先做一轮最小但高收益的优化，建议只做以下 4 项：

1. Task 1.1：全局 auth bootstrap
2. Task 1.3：删除问诊页整页 loading
3. Task 1.4：补问诊页局部 skeleton
4. Task 2.2：历史会话线程预取

这 4 项完成后，用户能最先感知到的改善是：

1. 刷新不白屏
2. 侧边栏更早出现
3. 点击历史会话不再整页闪
4. 首次切换历史会话更快
