# 问诊工作台渲染性能验证清单

> 日期：2026-07-06  
> 用途：作为 `P3-01` 的正式输出，统一人工复测与回归检查口径。

## 使用说明

每次涉及以下任一改动时，都应至少执行本清单一次：

- `ProtectedRoute` / auth bootstrap
- `ConsultationPage` 渲染边界
- 历史会话预取与切换逻辑
- 路由懒加载与 chunk 拆分
- 聊天运行时或问诊工作台骨架

建议在以下两种环境下各执行一轮：

1. 本地 production build + preview
2. 接近真实部署环境的 staging / dev server

---

## A. 刷新问诊页

### 场景

刷新：

- `/consultation`
- `/consultation/:id`

### 检查点

1. 页面不能出现纯白空白态
2. 应先看到应用 shell 或问诊工作台骨架
3. 左侧结构、顶部结构、右侧面板容器应早于线程内容出现
4. 不应出现旧版“整页 spinner 占满页面”的行为

### 通过标准

- 刷新后用户可立刻感知页面框架存在
- 白屏如果仍存在，只能是极短瞬时，不得成为明显等待阶段

---

## B. 历史会话首次切换

### 场景

1. 进入一个已有多个历史会话的账号
2. 选择一条此前未访问、未缓存的历史会话

### 检查点

1. 左侧历史栏不应整体消失或重挂载
2. 顶部 header 不应整体消失
3. 右侧聊天区允许出现局部切换遮罩或骨架
4. 不应出现“像浏览器整页刷新”的闪烁

### 通过标准

- 左侧和顶部持续稳定
- 右侧内容切换有明确反馈，但只是局部过渡

---

## C. 历史会话二次切换

### 场景

1. 先点开某历史会话，使其进入缓存
2. 再切到另一条
3. 再切回刚才那条

### 检查点

1. 再次切回缓存会话时应接近即时
2. 不应再次出现首次切换时那种明显等待
3. 选中态、标题、面板过渡应一致

### 通过标准

- 缓存命中后的切换明显比首次切换更快
- 不应回退成整页闪烁

---

## D. 历史会话预取

### 场景

桌面端打开 DevTools Network，鼠标移到历史会话卡片上，再点击。

### 检查点

1. hover / pointerdown 后应能看到目标线程请求提前发出
2. 点击时不应再额外出现明显的等待空档
3. 不应因为简单移动鼠标就产生大量重复线程请求

### 通过标准

- 预取行为可观察到
- 无请求洪泛

---

## E. 新建会话转正

### 场景

1. 进入 `/consultation`
2. 发送第一条消息
3. 观察从草稿态到真实 `conversationId` 的切换

### 检查点

1. 首条消息发送后页面不应整页闪烁
2. SSE 流不应中断
3. 新会话应被乐观插入左侧列表
4. URL 更新后聊天仍连续

### 通过标准

- 新会话转正只表现为局部上下文切换
- 生成中的回复持续稳定

---

## F. 路由拆包

### 场景

执行 production build，检查输出产物。

### 检查点

1. 入口包不应重新回到问诊重模块全量打包状态
2. `ConsultationPage` 应保持独立 chunk
3. `AssistantChatPanel` 这类重运行时应保持独立 chunk

### 当前参考值

- 入口包约 `347.90 kB`
- `ConsultationPage` 约 `176.52 kB`
- `AssistantChatPanel` 约 `416.19 kB`

### 通过标准

- 入口包未异常回涨
- 问诊相关 chunk 仍按需加载

---

## G. 自动化回归

### 当前最小回归集

```bash
pnpm --filter @bodysense/web exec vitest run \
  src/components/__tests__/ProtectedRoute.test.tsx \
  src/features/consultation/components/__tests__/SessionCard.test.tsx \
  src/features/consultation/pages/__tests__/ConsultationPage.test.tsx
```

### 检查点

1. `ProtectedRoute` 在 hydration / verifying 期间渲染 skeleton
2. `SessionCard` 会触发预取回调
3. `ConsultationPage` 在线程 pending 时保留工作台外壳

### 通过标准

- 三组测试全部通过

---

## H. 记录模板

每次人工复测建议至少记录以下内容：

```md
日期：
环境：
提交/分支：

刷新 /consultation：
- 白屏：有 / 无
- shell 首次出现：主观快 / 中 / 慢
- 是否出现整页 spinner：是 / 否

首次切换未缓存历史会话：
- 左侧闪烁：有 / 无
- 顶部闪烁：有 / 无
- 右侧过渡：自然 / 可接受 / 生硬

缓存后二次切换：
- 接近即时：是 / 否

构建结果：
- index:
- ConsultationPage:
- AssistantChatPanel:

结论：
```

---

## 当前限制

截至 2026-07-06，这份清单已定义完成，但实际浏览器 Performance 录制和人工复测结论尚未补录，因此：

- `P3-01` 可视为完成
- `P3-03` 仍保持待完成

补充说明：

- 当前终端环境已尝试通过临时 Playwright Chromium 做页面级复测
- 但浏览器启动失败，缺少系统共享库 `libatk-1.0.so.0`
- 当前机器也没有可直接调用的系统浏览器
- 因此本清单中的真实页面录制，仍需在具备完整浏览器依赖的开发机或 CI 浏览器环境中执行
