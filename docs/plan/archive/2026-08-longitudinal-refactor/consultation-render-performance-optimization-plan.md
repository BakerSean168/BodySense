# 问诊工作台渲染性能优化方案

> 创建日期：2026-07-06
> 状态：待评审

---

## 一、背景

当前 BodySense 问诊工作台在体验上与 ChatGPT Web 存在明显差异：

- 刷新受保护页面时，BodySense 会出现一段白屏，然后一次性加载整页
- 问诊页点击历史会话时，首次切换常出现“左侧会话列表 + 右侧会话区”整体刷新闪烁
- 当大部分会话都点击过并进入缓存后，整页闪烁会减弱，只剩会话区局部更新
- 从网络面板看，BodySense 的总资源数不一定更多，但 DOM Content Loaded 反而更晚

这说明问题不主要在“请求数量”，而在“关键渲染路径设计”：

1. 首屏把过多工作放在可见 UI 之前
2. 页面切换时把过多区域绑定到同一个 loading 边界
3. 路由切换、鉴权校验、线程数据获取、聊天运行时初始化之间缺少分层

本方案的目标不是继续打补丁，而是把问诊页改造成“壳体先出现、数据渐进填充、局部区域更新”的稳定架构。

---

## 二、现象与根因分析

## 2.1 现象 A：刷新时白屏，侧边栏不能立即出现

### 当前现象

- 用户刷新 `/consultation` 或其他受保护页面时，页面短暂空白
- 只有在鉴权校验完成后，整个页面才开始渲染
- 与 ChatGPT Web 相比，缺少“外壳先出现、数据后补”的观感

### 代码侧根因

当前 `ProtectedRoute` 在鉴权完成前直接返回 `null`，导致整页没有任何可见内容：

- `apps/web/src/components/ProtectedRoute.tsx`

同时，所有受保护页面都被包在 `ProtectedRoute` 内：

- `apps/web/src/App.tsx`

这带来两个问题：

1. 可视 UI 被 `/api/v1/me` 校验串行阻塞
2. 页面级数据请求要等鉴权组件放行后才能开始

换句话说，当前受保护页面的首屏路径是：

```text
hydrate auth store
  -> mount ProtectedRoute
  -> request /api/v1/me
  -> verification success
  -> mount page
  -> request page data
  -> render full page
```

这天然会制造白屏和 waterfall。

---

## 2.2 现象 B：资源不算多，但 DCL 更慢

### 当前现象

- 从体感和构建结果看，前端主线程首屏执行压力偏大
- 页面不是渐进出现，而是“等待一阵后整体出来”

### 代码侧根因

当前前端路由仍是同步 import，全量页面都进入同一个启动包：

- `apps/web/src/App.tsx`

生产构建结果显示：

- `dist/assets/index-*.js` 约 `1097 KB` minified
- gzip 后约 `320 KB`

这说明当前 SPA 首包过大，问诊页相关重模块一并进入初始解析执行路径，包括但不限于：

- `assistant-ui`
- markdown 渲染
- 问诊工作台复合组件
- 诊断面板和健康特征面板

因此即使资源请求数不多，浏览器仍可能把时间耗在：

1. 下载主包
2. 解析大体积 JS
3. 执行模块初始化
4. 挂载完整路由树

结论是：当前 DCL 偏长更像是“主线程阻塞 + 首包过大”的问题，而不是“静态资源数量过多”的问题。

---

## 2.3 现象 C：首次切换历史会话时出现整页闪烁

### 当前现象

- 在问诊历史列表中点击一个此前未打开过的会话
- 左侧列表和右侧工作区会一起闪一下，像整页刷新
- 当多个会话都被点开并进入缓存后，这个现象减弱

### 代码侧根因

`ConsultationPage` 当前把“线程切换 loading”提升成了整页 loading：

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

具体表现：

1. 路由参数 `id` 变化时，会设置 `isPageLoading = true`
2. 只要 `isPageLoading` 为 `true`，页面直接走顶层 loading 分支
3. 顶层 loading 分支会替换当前工作台内容，而不是保留既有壳体

这会产生两个直接后果：

1. `MainLayout` 之下的问诊工作台被整块重绘
2. 左侧历史列表无法保持稳定可见，视觉上像“整页闪一下”

同时，线程查询没有显式预取策略：

- `apps/web/src/features/consultation/hooks/useConsultationThreadQuery.ts`
- `apps/web/src/features/consultation/hooks/useConversationsQuery.ts`

所以首次点击未缓存的历史会话时，流程通常是：

```text
click conversation
  -> route id changes
  -> page enters full loading branch
  -> request thread data
  -> request returns
  -> page remounts content
```

而当会话已经被请求过，React Query 命中缓存后，等待时间显著缩短，所以整页闪烁会变轻。这也解释了“为什么所有会话都点击过后现象会改善”。

---

## 2.4 现象 D：开发环境下闪烁感更重

如果当前观察主要来自本地开发环境，还叠加了 React 19 `StrictMode` 的开发期双执行影响：

- `apps/web/src/main.tsx`

这会放大以下观感：

- mount/unmount 次数
- effect 执行次数
- 某些初始化逻辑的闪烁感

这不是生产根因，但会放大问题，容易误判。

---

## 三、优化目标

## 3.1 体验目标

目标对齐 ChatGPT Web 的核心体验，而不是机械对齐它的 DOM 或请求形式。

预期体验应为：

1. 刷新后 200 到 400ms 内看到稳定页面外壳
2. 左侧导航和问诊页基础骨架先出现，不等待完整数据
3. 会话列表可先出现容器和骨架，再渐进填充历史项
4. 点击历史会话时，左侧列表和顶部框架保持稳定，只更新会话内容区域
5. 即使目标线程尚未返回，也不允许出现整页白屏或整块闪烁

## 3.2 工程目标

1. 首屏路径从“串行阻塞”改为“壳体优先 + 数据并行”
2. 问诊页切换从“整页 loading”改为“面板级 loading”
3. 首包按路由拆分，问诊重模块只在进入问诊时加载
4. 用预测式预取降低首次切换历史会话的等待
5. 保持 URL 仍然是会话真值源，不引入额外会话状态混乱

## 3.3 非目标

1. 本次不引入 SSR 或服务端流式 HTML
2. 本次不重写整个路由系统
3. 本次不为了性能而大规模牺牲当前 UI 结构
4. 本次不引入复杂的虚拟列表，当前历史会话量级尚不需要

---

## 四、设计原则

1. Shell First
   - 先让用户看到稳定框架，再补数据
2. Stable Layout
   - 页面框架一旦出现，不因线程切换而整体卸载
3. Local Loading
   - loading 边界尽量下沉到最小必要区域
4. Intent-Based Prefetch
   - 用户有明确点击意图时，提前请求目标线程
5. Code Split by Route
   - 问诊页重依赖不应该进入所有页面首包
6. Cache as Accelerator, Not Truth
   - 缓存只负责加速，不替代 URL 作为当前会话真值

---

## 五、目标体验架构

## 5.1 目标首屏渲染流程

理想流程：

```text
load minimal app shell
  -> render router frame
  -> hydrate auth store
  -> render protected shell placeholder
  -> verify auth in background
  -> in parallel load route chunk + page queries
  -> fill sidebar skeleton
  -> fill thread panel skeleton
  -> hydrate real content
```

核心变化有两点：

1. 不再用 `/me` 校验阻塞所有可视内容
2. 不再等线程数据返回后才决定是否渲染工作台外壳

## 5.2 目标会话切换流程

```text
user hover/focus/pointerdown conversation item
  -> prefetch target thread

user click conversation item
  -> update route immediately
  -> keep sidebar and page frame stable
  -> keep previous panel visible or switch to skeleton overlay
  -> swap to target thread when data ready
```

这里最关键的是：点击历史会话不应触发整页分支切换，只应触发工作区内部内容切换。

---

## 六、优雅方案设计

## 6.1 方案一：鉴权从“阻塞页面”改为“引导页面”

### 当前问题

`ProtectedRoute` 目前承担了两个职责：

1. 权限守卫
2. 首屏鉴权加载控制

这两个职责耦合后，页面只能在校验完成后整体出现。

### 目标方案

引入全局 `AuthBootstrap` / `AuthGate`，将首屏鉴权与页面路由渲染解耦：

#### 方案结构

```text
App
  -> QueryClientProvider
  -> BrowserRouter
  -> AuthBootstrap
  -> Routes
```

`AuthBootstrap` 负责：

- 从持久化 store 恢复 auth 状态
- 在应用启动时发起一次 `/api/v1/me` 校验
- 暴露 `authResolved`、`authVerified`、`authUser` 等全局状态

`ProtectedRoute` 只负责：

- 未登录时重定向
- 已登录但尚未完成校验时，渲染受保护壳体级骨架，而不是 `null`

### 体验效果

- 刷新后用户能立刻看到应用框架
- 即使数据未到，也不会白屏
- 侧边栏、顶部栏、页面背景会先稳定出现

### 设计要求

1. 不在每个私有路由里重复调用 `/me`
2. 不在校验期间返回空节点
3. 如果 token 失效，再统一重定向登录页

---

## 6.2 方案二：把 `MainLayout` 变成真正稳定的 Shell

### 当前问题

当前问诊页的 loading 分支发生在 `ConsultationPage` 顶层。线程切换时，页面内容整体被替换，导致：

- 问诊页工作台闪烁
- 左侧会话区看起来也像刷新了一次

### 目标方案

问诊页应拆成三层：

1. 永久外壳层
   - `MainLayout`
   - 页面 header
   - 桌面端侧边栏容器
   - 移动端抽屉容器
2. 数据骨架层
   - 历史列表 skeleton
   - 聊天面板 skeleton
   - 信息面板 skeleton
3. 实际内容层
   - conversation list
   - assistant chat runtime
   - info panel / diagnosis panel

### 关键规则

1. 切换线程时不允许 `ConsultationPage` 返回整页 spinner
2. 左侧历史列表和顶部 header 始终常驻
3. 仅聊天面板和信息面板允许局部进入“加载态”

### 建议的视觉策略

首次进入问诊页：

- 左侧列表区域先出现固定容器和若干 skeleton card
- 右侧聊天区先出现静态骨架和输入框框架
- 信息面板先出现骨架卡片

切换历史会话：

- 左侧当前选中项立即高亮
- 右侧保留旧内容，叠加轻微 loading veil，或切成局部 skeleton
- 新线程返回后平滑替换，不出现白屏

这是比“整个区域一起消失再出现”更优雅的过渡。

---

## 6.3 方案三：线程切换采用面板级数据状态，而不是页面级数据状态

### 当前问题

`isPageLoading` 实际上把“线程数据还没到”错误建模成了“整页不可用”。

### 目标方案

把状态拆成三类：

1. `page shell ready`
   - 页面框架是否可渲染
2. `conversation list ready`
   - 左侧历史列表是否可展示
3. `active thread ready`
   - 当前会话线程是否可展示

### 设计要点

`ConsultationPage` 顶层不再维护“整页 loading”概念，而改为：

- 列表 query 自己管理列表骨架
- 线程 query 自己管理线程骨架
- 聊天 runtime 只在目标线程 ready 后挂载或切换

### 推荐的实现方向

1. 用 React Query 的 `placeholderData` 或等价策略保留上一个线程的可见内容
2. 用 query 的 `isFetching` 区分“后台切换中”与“首次无数据”
3. 首次无数据时显示局部 skeleton
4. 有旧数据且正在切换时显示 overlaid loading state，而不是清空整个 panel

### 结果

- 首次访问某历史会话时：只有右侧内容区域进入加载态
- 后续访问缓存命中时：几乎只发生数据替换，没有明显闪烁

---

## 6.4 方案四：对历史会话引入意图预取

### 当前问题

历史会话线程目前基本是“点了再拉”，所以首次点击未缓存会话时延迟最明显。

### 目标方案

在以下时机预取目标线程：

1. `pointerenter`
2. `focus`
3. `pointerdown`
4. 移动端列表项触摸开始时

### 预取策略

对 `SessionCard` 或其上层列表组件增加：

- `queryClient.prefetchQuery(threadKey)`

并添加基础保护：

1. 已有 fresh cache 不重复预取
2. 同时只预取少量最近目标
3. 预取失败静默，不干扰点击

### 预期收益

1. 鼠标移动到目标会话上时，请求已提前飞出
2. 用户点击时更大概率直接命中缓存或进入极短等待
3. 首次点击历史会话的闪烁感明显下降

---

## 6.5 方案五：按路由做代码拆分，缩短首屏主线程压力

### 当前问题

所有页面同步 import，导致问诊页重模块进入统一入口包。

### 目标方案

对页面级路由使用 `React.lazy` + `Suspense`：

- `DashboardPage`
- `ProfilePage`
- `ConsultationPage`
- `AssessmentPage`
- `HistoryPage`
- `TrainingPage`

其中重点是把问诊页及其重依赖独立成 chunk。

### 进一步优化

1. 优先让公共 shell、路由框架、auth 基础逻辑保留在主包
2. 把 `assistant-ui`、markdown、诊断区域等压到问诊 chunk
3. 可评估切换到 `@vitejs/plugin-react-oxc`，降低构建与转换开销

### 预期收益

1. 非问诊页面首屏不再被问诊重依赖拖累
2. 刷新时主包解析执行压力下降
3. DCL 和首屏可交互时间同步改善

---

## 6.6 方案六：让“新建会话”和“切换历史会话”共享一套稳定的工作台协议

### 当前问题

现在“新会话创建”和“已存在会话切换”走的是部分特殊逻辑：

- 通过 `chatSessionKey`
- 通过 `justCreatedRef`
- 通过 `activeConversationIdRef`

这些机制本身不一定错误，但如果 loading 边界太高，就会放大重挂载。

### 目标方案

保留 URL 为当前会话真值，但统一规范：

1. 侧边栏选中状态由 URL 直接驱动
2. 聊天 runtime 的重建只影响 chat panel，不影响整个工作台
3. 新会话转正时，只替换会话内容上下文，不触发整页 loading

### 设计结果

这样即使 `AssistantChatPanel` 在必要时按会话重建，也只会是“局部重建”，而不是“页面看起来像刷新了一次”。

---

## 七、推荐实施路径

## Phase 1：先消灭白屏和整页闪烁

范围：

1. 重构 `ProtectedRoute`，鉴权期间渲染骨架而不是 `null`
2. 去掉 `ConsultationPage` 顶层整页 loading 分支
3. 为问诊页定义 sidebar/chat/info 三块局部 skeleton

价值：

- 这是用户最直接感知的体验提升
- 不依赖后端变更
- 风险低，收益高

## Phase 2：补上预取和缓存切换策略

范围：

1. 历史会话 hover/focus/pointerdown prefetch
2. 线程 query 支持 placeholder/previous data 策略
3. 区分首次空态与后台切换态

价值：

- 直接改善“首次点击历史会话”体验
- 降低右侧 panel 的体感等待

## Phase 3：做路由级拆包

范围：

1. 页面级 `lazy()` 拆包
2. 路由 fallback 骨架
3. 评估问诊重依赖进一步拆分

价值：

- 解决 DCL 偏长和首包过重问题
- 让非问诊页面也同步获益

## Phase 4：打磨过渡动画和指标体系

范围：

1. 给面板切换加轻量级 loading veil / crossfade
2. 建立 DevTools 与 Lighthouse 基线
3. 补充性能回归检查清单

价值：

- 从“不卡顿”提升到“更自然”
- 防止后续迭代把性能问题再带回来

---

## 八、验收标准

## 8.1 刷新体验

1. 刷新 `/consultation` 时，页面不允许出现纯白空白态
2. 主布局和问诊页外壳应先于线程数据出现
3. 侧边栏容器和工作区容器在鉴权完成前已可见

## 8.2 历史会话切换体验

1. 点击未缓存历史会话时，不允许左侧列表整体闪烁
2. 点击已缓存历史会话时，应接近即时切换
3. 工作台 header 与侧边栏在切换过程中保持稳定

## 8.3 包体与加载体验

1. 问诊相关重模块不再全部进入根入口 chunk
2. 主入口 chunk 体积应显著下降
3. DCL、TBT、交互可用时间相对当前基线明显改善

## 8.4 行为正确性

1. token 失效时仍能正确跳转登录
2. 新建会话后 SSE 流和标题生成逻辑不被破坏
3. 切换历史会话后聊天、健康特征、诊断区域数据保持一致

---

## 九、风险与规避

## 9.1 风险：鉴权与页面显示解耦后，失效 token 暂时看到壳体

这是可接受的。因为壳体先出现并不等于业务数据泄露，只要：

1. 真正数据请求仍受 auth 控制
2. 校验失败后立即重定向
3. 壳体展示的是通用框架与骨架，而不是敏感内容

## 9.2 风险：placeholder/previous data 处理不当，可能短暂显示旧会话内容

规避方式：

1. 用明显但轻量的“切换中”覆盖层提示
2. 对标题、会话 id、主要元信息立即更新
3. 只保留旧内容作为视觉占位，不让用户误解成新会话内容

## 9.3 风险：过度优化导致实现复杂度上升

规避方式：

1. 优先做壳体和 loading 边界下沉
2. 不引入 SSR
3. 不做超前的虚拟化和复杂动画系统
4. 每个 Phase 都可独立验证收益

---

## 十、最终建议

本次问题的本质不是某个组件“渲染慢”，而是渲染架构当前过于依赖“数据 ready 才显示 UI”。

最优雅、也是最符合 ChatGPT 风格的方案是：

1. 用稳定 Shell 取代白屏等待
2. 用局部 Skeleton 取代整页 Spinner
3. 用 Prefetch 取代点击后才请求
4. 用路由拆包取代单入口大包

建议按以下优先级执行：

1. `AuthBootstrap + ProtectedRoute Skeleton`
2. `ConsultationPage` 去整页 loading，改为局部 loading
3. 历史会话线程预取
4. 路由级代码拆分

只要前两步完成，用户就会立刻感知到“刷新不白屏、切换不整页闪”的质变；后两步负责把这套体验从“看起来更好”进一步做成“真实更快”。

