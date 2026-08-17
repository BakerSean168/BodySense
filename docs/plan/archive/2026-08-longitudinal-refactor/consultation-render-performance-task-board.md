# 问诊工作台渲染性能优化任务看板

> 进行中（2026-07-06）。基于 [consultation-render-performance-implementation-flow.md](./consultation-render-performance-implementation-flow.md) 继续下沉，提供可直接拆 issue 的执行看板。

## 当前完成概览

- 已完成：`P0-01`、`P0-02`、`P0-03`、`P1-01`、`P1-02`、`P1-03`、`P1-04`、`P2-01`、`P2-02`
- 已评估暂不实施：`P2-03`
- 待完成：`P3-03`

## 里程碑

### M1：先消灭体感最差的问题

状态：✅ 已完成

目标：

1. 刷新问诊页不白屏
2. 历史会话切换不整页闪
3. 侧边栏和工作台 shell 提前出现

包含任务：

- `P0-01`
- `P0-02`
- `P0-03`
- `P1-01`

### M2：让首次切换历史会话更快

状态：✅ 核心完成

目标：

1. 首次点击未缓存会话的等待缩短
2. 线程切换从“清空再出现”变成“局部过渡”

包含任务：

- `P1-02`
- `P1-03`
- `P1-04`

### M3：减轻首包和主线程压力

状态：✅ 核心完成

目标：

1. 问诊重模块不再进入同步首包
2. 非问诊页面刷新不再被问诊页拖慢

包含任务：

- `P2-01`
- `P2-02`
- `P2-03`

### M4：收口与防回归

状态：🟡 进行中

目标：

1. 有统一验证清单
2. 有最小自动化防线
3. 有改造前后对比结果

包含任务：

- `P3-01`
- `P3-02`
- `P3-03`

---

## 任务列表

## P0-01：全局鉴权启动器

**状态**：✅ 已完成

**优先级**：P0  
**阶段**：Phase 1  
**目标**：把 `/api/v1/me` 校验从页面阻塞点改成全局启动行为。

**涉及文件**

- `apps/web/src/App.tsx`
- `apps/web/src/components/ProtectedRoute.tsx`
- `apps/web/src/stores/authStore.ts`
- 新增候选：
  - `apps/web/src/components/AuthBootstrap.tsx`
  - `apps/web/src/components/AuthGate.tsx`

**交付物**

1. 全局 auth bootstrap 组件
2. 可复用的 auth resolved / verifying 状态
3. `ProtectedRoute` 新实现

**依赖**

- 无

**完成定义**

1. 刷新任一私有页面不再白屏
2. `/api/v1/me` 不再因每个私有路由重复触发
3. token 失效时仍能跳登录页

**建议验证**

```bash
pnpm --filter @bodysense/web build
```

手动验证：

1. 刷新 `/consultation`
2. 刷新 `/dashboard`
3. 本地清空后端用户或令 token 失效，再刷新私有页

---

## P0-02：受保护页面壳体骨架

**状态**：✅ 已完成

**优先级**：P0  
**阶段**：Phase 1  
**目标**：在鉴权中和路由 fallback 中渲染稳定 shell。

**涉及文件**

- `apps/web/src/components/layout/MainLayout.tsx`
- 新增候选：
  - `apps/web/src/components/layout/AppShellSkeleton.tsx`
  - `apps/web/src/components/layout/ConsultationShellSkeleton.tsx`

**交付物**

1. 通用 shell skeleton
2. 问诊页专用 shell skeleton

**依赖**

- `P0-01`

**完成定义**

1. 刷新问诊页先看到外壳
2. 布局抖动可控
3. 页面主体不再从空白直接跳到完整 UI

**建议验证**

手动验证：

1. 慢网条件下刷新 `/consultation`
2. 观察导航、工作台容器、面板骨架是否先出现

---

## P0-03：移除问诊页整页 loading

**状态**：✅ 已完成

**优先级**：P0  
**阶段**：Phase 1  
**目标**：把线程切换造成的页面级 loading 下沉到 panel 级。

**涉及文件**

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

**交付物**

1. 无顶层整页 spinner 的 `ConsultationPage`
2. 常驻的 header / sidebar / panel frame

**依赖**

- `P0-01`

**完成定义**

1. 点击历史会话时 header 不消失
2. 左侧历史栏不消失
3. 不再返回整页 loading 分支

**建议验证**

手动验证：

1. 切换多个未缓存历史会话
2. 观察左侧列表和顶部区域是否稳定

---

## P1-01：问诊页局部骨架组件

**状态**：✅ 已完成

**优先级**：P0  
**阶段**：Phase 1  
**目标**：为 sidebar/chat/info 三块区域补齐局部骨架。

**涉及文件**

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/consultation/components/SessionHistorySidebar.tsx`
- `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`
- `apps/web/src/features/consultation/components/InfoPanel.tsx`
- 新增候选：
  - `SessionHistorySidebarSkeleton.tsx`
  - `ChatPanelSkeleton.tsx`
  - `InfoPanelSkeleton.tsx`

**交付物**

1. 列表骨架
2. 聊天区骨架
3. 信息面板骨架

**依赖**

- `P0-03`

**完成定义**

1. 首次进入问诊页时三块区域都有稳定占位
2. 切换未缓存线程时只有右侧 panel 进入 loading

**建议验证**

手动验证：

1. 强制慢网
2. 清空缓存后打开 `/consultation/:id`

---

## P1-02：线程查询状态重构

**状态**：✅ 已完成

**优先级**：P1  
**阶段**：Phase 2  
**目标**：让 query 状态区分首次加载、切换中、后台刷新。

**涉及文件**

- `apps/web/src/features/consultation/hooks/useConsultationThreadQuery.ts`
- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

**交付物**

1. 更准确的线程查询状态语义
2. 去除 `isPageLoading` 对顶层渲染的控制
3. placeholder/previous-data 方案

**依赖**

- `P0-03`

**完成定义**

1. 首次无数据时显示局部 skeleton
2. 切换中有旧数据时不清空整个工作区

**建议验证**

手动验证：

1. 打开未缓存历史会话
2. 再切换已缓存历史会话
3. 对比两种路径的 UI 行为

---

## P1-03：历史会话线程预取

**状态**：✅ 已完成

**优先级**：P1  
**阶段**：Phase 2  
**目标**：在 hover/focus/pointerdown 时预取线程详情。

**涉及文件**

- `apps/web/src/features/consultation/components/SessionCard.tsx`
- `apps/web/src/features/consultation/components/SessionHistorySidebar.tsx`
- `apps/web/src/features/consultation/services/consultationQueryKeys.ts`

**交付物**

1. `prefetchQuery` 接入
2. 触发策略与去重保护

**依赖**

- `P1-02`

**完成定义**

1. 用户点击前已有较高概率命中缓存
2. 无无意义请求洪泛

**建议验证**

手动验证：

1. 打开 DevTools Network
2. hover 某历史会话
3. 观察线程请求是否提前发出

---

## P1-04：切换过渡态与运行时边界校正

**状态**：✅ 已完成

**优先级**：P1  
**阶段**：Phase 2  
**目标**：保留旧 panel 内容并叠加轻量过渡，同时确保 chat runtime 重建只发生在 chat panel。

**涉及文件**

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`

**交付物**

1. panel loading veil 或 overlay
2. runtime 重建边界收口

**依赖**

- `P1-02`

**完成定义**

1. 切换中不出现“整页刷新感”
2. 新会话创建后的 SSE 流仍稳定

**建议验证**

手动验证：

1. 新建会话发送首条消息
2. 观察转正后的 streaming 是否连续
3. 切换历史会话观察是否只局部更新

---

## P2-01：页面级路由懒加载

**状态**：✅ 已完成

**优先级**：P2  
**阶段**：Phase 3  
**目标**：把各页面从同步 import 改成按路由加载。

**涉及文件**

- `apps/web/src/App.tsx`

**交付物**

1. `lazy()` 路由
2. `Suspense` fallback

**依赖**

- `P0-01`
- `P0-02`

**完成定义**

1. 问诊页及其他大页面不再全进同步首包
2. fallback 体验可接受

**建议验证**

```bash
pnpm --filter @bodysense/web build
```

手动验证：

1. 刷新 `/dashboard`
2. 刷新 `/consultation`
3. 观察 chunk 请求顺序

---

## P2-02：问诊重依赖 chunk 审查

**状态**：✅ 已完成

**优先级**：P2  
**阶段**：Phase 3  
**目标**：确认 `assistant-ui`、markdown、诊断区真正被压到问诊 chunk。

**涉及文件**

- `apps/web/src/features/consultation/components/*`
- `apps/web/vite.config.ts`

**交付物**

1. 构建后 chunk 对照结果
2. 如有必要的二次 lazy 调整

**依赖**

- `P2-01`

**完成定义**

1. 入口包体积相对基线下降
2. 问诊相关重依赖集中到问诊 chunk

**建议验证**

```bash
pnpm --filter @bodysense/web build
```

---

## P2-03：Vite React 插件优化评估

**状态**：🟡 已评估，暂不实施

**优先级**：P3  
**阶段**：Phase 3  
**目标**：评估切到 `@vitejs/plugin-react-oxc`，顺便清理已知 deprecation。

**涉及文件**

- `apps/web/vite.config.ts`
- `apps/web/package.json`

**交付物**

1. 兼容性结论
2. 如可行则完成插件切换

**依赖**

- `P2-01`

**完成定义**

1. 构建正常
2. 无明显兼容问题
3. 不影响现有测试

**建议验证**

```bash
pnpm --filter @bodysense/web build
```

---

## P3-01：性能验证清单

**状态**：✅ 已完成

**优先级**：P2  
**阶段**：Phase 4  
**目标**：建立统一手动验证口径。

**交付物**

1. 刷新白屏检查项
2. 历史会话闪烁检查项
3. chunk 回涨检查项
4. SSE 回归检查项
5. 正式验证文档：`consultation-render-performance-validation-checklist.md`

**依赖**

- `P2-02`

---

## P3-02：最小自动化回归测试

**状态**：✅ 已完成

**优先级**：P2  
**阶段**：Phase 4  
**目标**：为关键行为补最小测试防线。

**涉及文件**

- `apps/web/src/components/*`
- `apps/web/src/features/consultation/components/__tests__/*`
- `apps/web/src/features/consultation/pages/*`

**交付物**

1. `ProtectedRoute` skeleton 渲染测试
2. `SessionCard` 预取触发测试
3. `ConsultationPage` 不走整页 spinner 的回归测试
4. `ConsultationPage` 会话切换 overlay 与旧线程保留测试

**依赖**

- `P1-03`

---

## P3-03：改造后复测

**状态**：⏳ 待完成

**优先级**：P2  
**阶段**：Phase 4  
**目标**：对比 Phase 0 基线，验证优化收益。

**交付物**

1. 改造后包体数字
2. 刷新问诊页对比结论
3. 首次切换历史会话对比结论
4. production preview 页面级回放验证结果

**当前进展**

1. 当前 VPS 已补齐 Playwright/Chromium 运行依赖
2. 已在 production preview 上完成页面级回放验证
3. 已确认刷新壳体先出现、首次切换只做局部 overlay、hover 预取生效

**剩余阻塞**

1. 尚未补真实业务数据下的人工体感复测
2. 尚未补 DevTools Performance 时间线录制

**下一步建议**

1. 用真实账号与真实历史会话再做一轮人工切换验证
2. 补录刷新 `/consultation` 与首次切换未缓存历史会话的 Performance 轨迹
3. 将人工结论追加到结果文档

**依赖**

- `P3-01`
- `P3-02`

---

## 建议开工顺序

如果按最短路径推进，建议直接按下面 6 个 issue 开工：

1. `P0-01` 全局鉴权启动器
2. `P0-02` 受保护页面壳体骨架
3. `P0-03` 移除问诊页整页 loading
4. `P1-01` 问诊页局部骨架组件
5. `P1-02` 线程查询状态重构
6. `P1-03` 历史会话线程预取

做完这 6 个任务，用户侧主要问题已经基本会消失，后面的拆包和回归属于进一步优化和固化。
