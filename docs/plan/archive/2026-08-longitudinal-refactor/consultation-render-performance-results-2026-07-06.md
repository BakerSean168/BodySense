# 问诊工作台渲染性能优化阶段结果记录

> 日期：2026-07-06  
> 范围：Phase 1、Phase 2、Phase 3 核心实现，Phase 4 最小回归测试

## 已完成项

### 1. 首屏壳体稳定化

已落地：

- `AuthBootstrap` 启动期 session 校验
- `authStore` hydration / auth resolved / verify session 状态
- `ProtectedRoute` 在 hydration 和校验期间渲染 shell skeleton，而不是白屏

对应文件：

- `apps/web/src/components/AuthBootstrap.tsx`
- `apps/web/src/stores/authStore.ts`
- `apps/web/src/components/ProtectedRoute.tsx`
- `apps/web/src/components/layout/AppShellSkeleton.tsx`

### 2. 问诊页去整页 loading

已落地：

- 删除 `ConsultationPage` 顶层整页 spinner 分支
- 保留工作台 header、左侧历史栏、右侧 panel 框架常驻
- 下沉为局部骨架和局部错误态

对应文件：

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/consultation/components/SessionHistorySidebarSkeleton.tsx`
- `apps/web/src/features/consultation/components/ChatPanelSkeleton.tsx`
- `apps/web/src/features/consultation/components/InfoPanelSkeleton.tsx`

### 3. 历史会话切换优化

已落地：

- `useConsultationThreadQuery` 使用 `placeholderData`
- 历史会话卡片 `pointerenter` / `pointerdown` 预取线程
- 切换中保留旧内容并叠加 `PanelTransitionOverlay`
- 仅在 displayed thread ready 后切换 chat runtime

对应文件：

- `apps/web/src/features/consultation/hooks/useConsultationThreadQuery.ts`
- `apps/web/src/features/consultation/components/SessionCard.tsx`
- `apps/web/src/features/consultation/components/SessionHistorySidebar.tsx`
- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

### 4. 路由与问诊重模块拆包

已落地：

- 页面级 `React.lazy`
- 问诊页独立 chunk
- `AssistantChatPanel`、`InfoPanel`、`DiagnosisPanel` 独立 chunk

对应文件：

- `apps/web/src/App.tsx`
- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

## 当前构建结果

### 初始参考值

较早的 production build 结果：

- `dist/assets/index-DcP559Xv.js`：`1,097.07 kB` minified / `319.93 kB` gzip

这是“所有页面同步 import、问诊重模块仍在主包”阶段的入口体积。

### 当前结果

最新 production build 结果：

- `dist/assets/index-CIKvpylI.js`：`347.90 kB` minified / `108.21 kB` gzip
- `dist/assets/ConsultationPage-ivMvDQFi.js`：`176.52 kB` minified / `56.85 kB` gzip
- `dist/assets/AssistantChatPanel-DgrXEywf.js`：`416.19 kB` minified / `122.16 kB` gzip
- `dist/assets/InfoPanel-CRpwgIyT.js`：`21.87 kB` minified / `4.10 kB` gzip
- `dist/assets/DiagnosisPanel-DEs5u6nn.js`：`9.59 kB` minified / `2.68 kB` gzip

### 结果解读

1. 主入口包已从约 `1.10 MB` 降到约 `347.90 kB`
2. 问诊页壳体与聊天运行时已分离，不再一起阻塞入口执行
3. 当前最重的运行时代码已被压到 `AssistantChatPanel` 独立 chunk
4. `ConsultationPage` 本身已缩到约 `176.52 kB`，更适合先出工作台外壳再补聊天运行时

## 自动化验证结果

已通过：

```bash
pnpm --filter @bodysense/web build
pnpm --filter @bodysense/web exec vitest run \
  src/components/__tests__/ProtectedRoute.test.tsx \
  src/features/consultation/components/__tests__/SessionCard.test.tsx \
  src/features/consultation/pages/__tests__/ConsultationPage.test.tsx
```

覆盖点：

- `ProtectedRoute` 在 hydration / verifying 期间渲染 skeleton
- `SessionCard` 在 `pointerenter` / `pointerdown` 时触发预取
- `ConsultationPage` 在线程 pending 时保留工作台壳体，只显示局部骨架
- `ConsultationPage` 在线程切换中保留旧线程内容，并只叠加 panel overlay

## 浏览器级回放验证结果

已完成一轮基于 production preview 的页面级回放验证：

```bash
pnpm --filter @bodysense/web preview --host 0.0.0.0 --port 4173
node /tmp/pw-replay/replay.mjs
```

验证方式：

- 运行 production preview，而不是 Vite dev server
- 在浏览器侧拦截 `/api/v1/me`、`/api/v1/conversations`、`/api/v1/consultations/:id/thread`
- 人为注入慢鉴权、慢线程切换与 hover 预取场景

回放结果：

- 刷新 `/consultation/:id` 时，shell skeleton 先出现
- 壳体渲染后，问诊页 header 正常出现
- 线程未返回时显示 chat/info 局部 skeleton
- 未出现旧版整页 spinner 文案
- 首次 hover 历史会话卡片会提前触发一次线程请求
- 首次切换未缓存历史会话时，header 与 sidebar 保持稳定
- 切换过程中旧线程内容继续可见
- 切换过程中显示 panel overlay，而不是整页清空
- 目标线程完成后，新内容正常替换旧内容

回放脚本返回的关键结果：

```json
{
  "refresh": {
    "shellVisibleEarly": true,
    "shellVisibleAfterLoad": true,
    "chatSkeletonVisible": true,
    "infoSkeletonVisible": true,
    "fullPageSpinnerTextVisible": false
  },
  "switching": {
    "hoverPrefetchTriggered": true,
    "conv3RequestsAfterHover": 1,
    "headerStable": true,
    "sidebarStable": true,
    "oldThreadStillVisibleDuringSwitch": true,
    "panelOverlayCount": 4,
    "chatSkeletonDuringSwitch": false,
    "infoSkeletonDuringSwitch": false,
    "newThreadVisibleAfterSwitch": true
  }
}
```

## 评估但未保留的改动

### `@vitejs/plugin-react-oxc`

已评估，但未保留。

结论：

- 在当前项目中，production build 可通过
- 但 `vitest` 启动时要求 `rolldown-vite`
- 与当前测试链路不兼容

处理：

- 已回退到 `@vitejs/plugin-react`
- 后续若整体迁移到 rolldown，再重新评估

## 仍待完成

### 1. 人工性能复测

尚未补：

- 刷新 `/consultation` 的浏览器 Performance 录制
- 首次点击未缓存历史会话的录制
- 白屏时长、首次可见 shell、整页重绘次数的前后对比

已完成：

- VPS 已补齐 Playwright/Chromium 运行依赖
- 已在 production preview 上完成一轮浏览器页面回放验证

仍未补：

- 基于真实后端真实数据的人工体感复测
- 浏览器 DevTools Performance 时间线与 flame chart 录制

### 2. 验证清单

已补统一 checklist：

- `docs/plan/active/consultation-render-performance-validation-checklist.md`

覆盖内容包括：

- 刷新问诊页是否白屏
- 历史会话首次切换是否整页闪
- 入口包是否异常回涨
- 问诊 chunk 是否仍按需加载
- 新建会话后的 SSE 流是否稳定

### 3. 自动化与人工验证边界

当前终端环境已完成：

- production build
- vitest 最小回归集
- 浏览器复测脚本方案验证
- production preview 浏览器页面回放验证

当前终端环境未完成：

- 浏览器 DevTools Performance 录制
- 真实页面交互下的人工体感复测

因此当前已可确认“代码路径、打包结果、关键页面体验路径均达预期”，但 `P3-03` 仍需补齐真实业务数据下的人工 Performance 录制。

### 4. 更深层问诊运行时拆分

虽然问诊主 chunk 已明显下降，但 `AssistantChatPanel` chunk 仍约 `416.19 kB`。后续如继续优化，可进一步评估：

- markdown 渲染链路
- `assistant-ui` 运行时边界
- 仅在实际需要时加载诊断/右侧面板细节
