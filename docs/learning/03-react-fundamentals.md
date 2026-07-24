# React 基础详解 — 从 BodySense 前端读懂 React 与 Hooks

> 学习版文档：真实源码保持整洁，这里是带逐行注释的"讲解副本"。
> 对照阅读的真实文件：
> - `apps/web/src/features/auth/components/LoginForm.tsx`（组件 + useState）
> - `apps/web/src/stores/authStore.ts`（Zustand 全局状态）
> - `apps/web/src/features/consultation/hooks/useConversationsQuery.ts`（TanStack Query）
> - `apps/web/src/features/consultation/hooks/useSSEProcessor.ts`（真实代码已有详尽中文注释，可直接读）

---

## 0. 先建立整体心智模型

React 的核心思想只有一句：**UI = f(state)**。
界面是状态的函数。你不去手动操作 DOM，而是**改状态**，React 自动重新渲染界面。

```text
用户操作（输入、点击）
   │
   ▼
改变 state（useState / Zustand）
   │
   ▼  React 侦测到 state 变化
   ▼
组件函数重新执行（re-render）→ 返回新的 JSX
   │
   ▼
React 对比新旧，只更新变化的 DOM
```

**Hook 是什么？** 是一类以 `use` 开头的函数，让函数组件拥有"记忆"和"副作用"能力。
规则（必须遵守）：
1. 只能在**组件函数体顶层**调用，不能放进 if / for / 嵌套函数里。
2. 只能在 React 组件或自定义 Hook 里调用。

> 为什么有这两条规则？因为 React 靠"调用顺序"来对应每个 Hook 的状态。顺序一变，状态就错位了。

---

## 1. 一个完整组件：LoginForm（useState + 事件 + 条件渲染）

```tsx
// 从 react 导入 useState 这个 Hook。
import { useState } from 'react';
// 从路由库导入 useNavigate（跳转页面用）。
import { useNavigate } from 'react-router';
// 导入全局 store 的 Hook（下一节讲）。
import { useAuthStore } from '@/stores/authStore'; // @/ 是指向 src/ 的路径别名
import { Input } from '@/components/ui/Input';
import { Button } from '@/components/ui/Button';

// 函数组件：一个返回 JSX 的函数，函数名首字母大写（React 约定）。
export function LoginForm() {
  // useNavigate() 返回一个跳转函数，调用 navigate('/x') 就切换路由。
  const navigate = useNavigate();

  // 从全局 store 里取出需要的字段和方法（解构赋值）。
  const { login, isLoading, error, clearError } = useAuthStore();

  // ===== useState：给组件添加"局部状态" =====
  // useState('') 返回一个数组：[当前值, 更新函数]，用解构接住。
  // 初始值 ''（空字符串）。email 是当前值，setEmail 是改它的唯一途径。
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  // 状态也可以是对象。这里存放"字段校验错误"。
  // <{...}> 是 TypeScript 泛型：告诉 useState 这个状态的类型。
  const [validationErrors, setValidationErrors] = useState<{
    email?: string;      // ? 表示可选字段
    password?: string;
  }>({});

  // 一个普通函数：做表单校验，返回是否通过。
  const validateForm = (): boolean => {
    const errors: { email?: string; password?: string } = {};
    if (!email) {
      errors.email = '请输入邮箱地址';
    } else if (!/\S+@\S+\.\S+/.test(email)) {  // 正则校验邮箱格式
      errors.email = '邮箱格式不正确';
    }
    if (!password) {
      errors.password = '请输入密码';
    }
    // ⚠️ 关键：改状态必须调用 setXxx，不能直接 validationErrors = errors。
    // 只有通过 setValidationErrors，React 才知道要重新渲染。
    setValidationErrors(errors);
    // Object.keys(obj) 取对象所有键，.length === 0 表示没有错误。
    return Object.keys(errors).length === 0;
  };

  // 表单提交处理函数。async 因为里面要 await 登录。
  // e: React.FormEvent 是事件对象的 TS 类型。
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();  // 阻止表单默认提交（否则页面会刷新）
    clearError();        // 清掉上次的全局错误
    if (!validateForm()) return;  // 校验不过就停

    try {
      await login(email, password);  // 调 store 的登录（异步）
      navigate('/dashboard');        // 成功 → 跳转
    } catch {
      // 失败：错误已经被 store 记到 error 里，这里什么都不用做
    }
  };

  // return 返回 JSX：看起来像 HTML，其实是 JS 表达式。
  return (
    // onSubmit={handleSubmit} 绑定提交事件。className 就是 HTML 的 class。
    <form onSubmit={handleSubmit} className="space-y-5">
      <Input
        id="email"
        type="email"
        label="邮箱地址"
        value={email}                              // 受控组件：值由 state 驱动
        onChange={(e) => setEmail(e.target.value)} // 输入时更新 state
        placeholder="your@email.com"
        error={validationErrors.email}             // 把校验错误传给子组件显示
      />
      <Input
        id="password"
        type="password"
        label="密码"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        error={validationErrors.password}
      />

      {/* 条件渲染：error 为真才渲染这个错误框。&& 短路：左假则整体不渲染。*/}
      {error && (
        <div className="rounded-xl bg-red-50 p-4 border border-red-100">
          <p className="text-sm font-medium text-red-800">{error}</p>
        </div>
      )}

      {/* isLoading 为真时按钮显示加载态并禁用（防重复提交）*/}
      <Button type="submit" isLoading={isLoading} className="w-full mt-2" size="lg">
        登录
      </Button>
    </form>
  );
}
```

### 这里出现的核心概念

| 概念 | 说明 |
|---|---|
| **函数组件** | 返回 JSX 的函数，首字母大写 |
| **JSX** | `<div>` 语法，`{}` 里嵌 JS 表达式，`className` 代替 `class` |
| **props** | 父传子的参数，如 `value`、`onChange`、`error` |
| **受控组件** | 表单值由 state 驱动（`value={email}`），改动走 `onChange` |
| **事件处理** | `onClick`、`onChange`、`onSubmit`，值是函数 |
| **条件渲染** | `{cond && <X/>}` 或三元 `{cond ? <A/> : <B/>}` |
| **列表渲染** | `{arr.map(item => <X key={item.id}/>)}`（注意 key） |

---

## 2. useState 深入：为什么必须用 setState

```tsx
const [count, setCount] = useState(0);

// ❌ 错误：直接改变量，React 不知道，界面不更新
count = count + 1;

// ✅ 正确：调用更新函数，触发重新渲染
setCount(count + 1);

// ✅ 更安全：基于"上一个值"更新（连续更新/异步场景必须这样）
setCount(prev => prev + 1);
```

**为什么？** 每次渲染，组件函数会**重新执行一遍**，`useState` 返回当前这一轮的值。React 在内部记住状态，`setXxx` 是你唯一能通知 React "状态变了，请重渲染"的方式。直接赋值只是改了个局部变量，React 毫不知情。

> 心智模型：把每次渲染想象成一张"快照"。这一张快照里的 `email`、`count` 都是常量。要进入"下一张快照"，只能调 `setXxx`。

---

## 3. Zustand 全局状态 — 跨组件共享（authStore）

`useState` 的状态只属于单个组件。多个页面都要用的登录状态（token、用户信息），需要**全局 store**。这个项目用 **Zustand**。

```tsx
import { create } from 'zustand';
import { persist } from 'zustand/middleware';  // 持久化中间件：自动存 localStorage

// 先用 TS interface 描述 store 的形状：有哪些状态 + 哪些方法。
interface AuthState {
  user: User | null;             // 状态：当前用户，未登录为 null
  accessToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  // 方法（Actions）：返回 Promise<void> 的是异步操作
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  clearError: () => void;
}

// ⚠️ 模块级变量（不在 store 里）：用来做"并发去重锁"。
// 因为 store 之外也是普通 JS 模块，可以放这种运行期单例。
let refreshPromise: Promise<boolean> | null = null;

// create<AuthState>()(...) 创建 store。
// persist(初始化函数, 配置) 给它包上持久化能力。
export const useAuthStore = create<AuthState>()(
  persist(
    // (set, get) => ({...})：set 改状态，get 读当前状态。
    (set, get) => ({
      // ── 初始状态 ──
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,

      // ── login 方法：一次完整的异步业务 ──
      login: async (email, password) => {
        set({ isLoading: true, error: null }); // set 合并更新部分字段
        try {
          const response = await fetch(apiUrl('/api/v1/auth/login'), {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password }), // 对象转 JSON 字符串
          });
          const data = await safeJson<{ access_token?: string }>(response);
          if (!response.ok) {
            throw new Error(data?.message || '登录失败');
          }
          // 登录成功：写入 token，标记已登录。
          set({
            accessToken: data.access_token,
            isAuthenticated: true,
            isLoading: false,
            error: null,
          });
          await get().fetchUser();  // get() 拿到最新 store，调另一个方法
        } catch (error) {
          // 失败：把错误信息写进 error，供组件显示。
          set({
            isLoading: false,
            error: error instanceof Error ? error.message : '登录失败',
          });
          throw error;  // 再抛出去，让组件的 catch 也能感知
        }
      },

      logout: () => {
        // ... 调后端注销接口，然后清空本地状态
        set({ user: null, accessToken: null, isAuthenticated: false });
      },

      clearError: () => set({ error: null }),
    }),
    // ── persist 配置 ──
    {
      name: 'auth-storage',  // localStorage 的键名
      // partialize：只持久化这几个字段（isLoading/error 不需要存）。
      partialize: (state) => ({
        accessToken: state.accessToken,
        isAuthenticated: state.isAuthenticated,
        user: state.user,
      }),
    }
  )
);
```

**组件里怎么用？**

```tsx
// 取整个 store（任意字段变都会重渲染）
const { login, isLoading, error } = useAuthStore();

// ✅ 更优：用"选择器"只订阅需要的字段，减少无谓重渲染
const login = useAuthStore((s) => s.login);
const isLoading = useAuthStore((s) => s.isLoading);
```

**为什么用 Zustand 而不是 useState？**
- `useState` 的状态出了组件就没了；登录态需要**跨页面共享**。
- Zustand 是"组件外的状态容器"，任何组件用 `useAuthStore()` 都能读到同一份。
- `persist` 让状态存进 localStorage，刷新页面不丢登录。
- 相比 Redux，Zustand 几乎没有样板代码。

---

## 4. 常用 Hook 全景（含"为什么用"）

### useState — 局部状态
```tsx
const [value, setValue] = useState(initial);
```
**用在**：表单输入、开关、计数、任何"会变且影响 UI"的组件内数据。

### useEffect — 副作用（与外部世界同步）
```tsx
import { useEffect } from 'react';

useEffect(() => {
  // 这里放"副作用"：订阅、定时器、手动 DOM 操作、发请求等。
  const timer = setInterval(() => console.log('tick'), 1000);

  // return 一个"清理函数"：组件卸载或依赖变化前调用，防内存泄漏。
  return () => clearInterval(timer);
}, [/* 依赖数组 */]);
// 依赖数组的三种写法：
//   []          → 只在挂载时跑一次（+ 卸载时清理）
//   [a, b]      → a 或 b 变化时重新跑
//   不写（省略）  → 每次渲染都跑（几乎不该这样）
```
**为什么用**：组件渲染应当是"纯"的（只根据 state/props 算 UI）。一切"对外界产生影响"的操作（网络、定时器、订阅）都要隔离进 `useEffect`，并配套清理，避免泄漏和竞态。

### useRef — 不触发渲染的"盒子" / 引用 DOM
```tsx
import { useRef } from 'react';

const inputRef = useRef<HTMLInputElement>(null); // 引用 DOM
const countRef = useRef(0);                      // 存一个可变值

// 改 ref.current 不会触发重渲染（这是它和 useState 的核心区别）。
countRef.current += 1;
inputRef.current?.focus(); // 直接操作 DOM
```
**为什么用**：
- 需要一个"在多次渲染间保持、但改动不需要重渲染"的值（如计时器 id、上一次的值、去重标记）。
- 需要直接访问 DOM 节点（聚焦、测量、滚动）。

> 在本项目 `useSSEProcessor` / `useAssistantChatRuntime` 里，`useRef` 被用来保存流式处理过程中的可变引用（如当前消息累加缓冲），因为这些改动不该每次都触发界面重渲染。

### useCallback — 缓存函数
```tsx
import { useCallback } from 'react';

const handleClick = useCallback(() => {
  doSomething(id);
}, [id]); // 只有 id 变化时才生成新函数
```
**为什么用**：组件每次渲染都会**重新创建函数**。如果把函数传给用了 `React.memo` 的子组件，或作为其他 Hook 的依赖，每次新函数会导致子组件无谓重渲染 / effect 反复触发。`useCallback` 让函数"身份稳定"。

### useMemo — 缓存计算结果
```tsx
import { useMemo } from 'react';

const sorted = useMemo(() => {
  return bigList.slice().sort(compare); // 昂贵计算
}, [bigList]); // bigList 不变就复用上次结果
```
**为什么用**：避免每次渲染都重复做昂贵计算（排序、过滤、聚合）。

> 记忆口诀：**useMemo 缓存"值"，useCallback 缓存"函数"**。两者都靠依赖数组决定是否复用。不要滥用——只在确有性能问题或需要稳定引用时用。

---

## 5. 自定义 Hook — 把逻辑抽出来复用（useConversationsQuery）

自定义 Hook 就是"名字以 use 开头、内部调用其他 Hook 的函数"。它把可复用逻辑从组件里抽离。

```tsx
import { useQuery } from '@tanstack/react-query';
import { consultationApi } from '../services/consultationService';
import { consultationKeys } from '../services/consultationQueryKeys';

// 自定义 Hook：封装"获取会话列表"这件事。
export function useConversationsQuery() {
  // useQuery 是 TanStack Query 提供的 Hook，专门管"服务端数据"。
  return useQuery({
    // queryKey：这份数据的唯一标识（缓存键）。相同 key 共享缓存。
    queryKey: consultationKeys.conversations(),
    // queryFn：真正去拿数据的异步函数。
    queryFn: () => consultationApi.listConversations({ limit: 50 }),
    // select：对返回数据做转换，组件只拿到它真正需要的部分。
    select: (data) => data.conversations,
  });
}
```

组件里用起来极简：
```tsx
function ConversationList() {
  // 一行拿到：数据、加载态、错误态，还自动缓存/重试/后台刷新。
  const { data, isLoading, error } = useConversationsQuery();

  if (isLoading) return <Spinner />;
  if (error) return <ErrorBox />;
  return <ul>{data?.map(c => <li key={c.id}>{c.title}</li>)}</ul>;
}
```

**为什么用 TanStack Query 而不是自己 useEffect + useState 拉数据？**
- 自己写要手动管理 loading / error / 缓存 / 重试 / 去重 / 失效刷新，非常啰嗦且易错。
- `useQuery` 把这些全包了：自动缓存、窗口聚焦重新拉取、请求去重、后台更新。
- **关键区分**：Zustand 管"客户端状态"（登录态、UI 开关），TanStack Query 管"服务端状态"（从后端拉的数据）。两者职责不同，配合使用。

---

## 6. 进阶实战：手写 SSE 流式处理（已在真实代码里注释）

`apps/web/src/features/consultation/hooks/useSSEProcessor.ts` 是这个项目里**注释最完整**的前端文件（真实源码里就有详尽中文注释）。它展示了：
- `async` 函数 + `while` 循环读取 `ReadableStream`
- `TextDecoder` 处理流式二进制（含 `{ stream: true }` 防中文乱码）
- 用 `buffer` 缓存不完整的行、`split('\n')` + `pop()` 的经典流解析套路
- 事件映射表 + 回调分发的设计

建议直接打开那个文件精读，它是"把 JS 异步/流处理讲透"的最佳材料。这里不重复。

> 顺带一个可改进点（也是很好的练习）：该文件里散落着一些 `console.debug`/`console.warn` 调试语句（如第 147、181 行附近）。练手时可以把它们收敛成一个受 `import.meta.env.DEV` 控制的 `debug()` 开关——这正是项目审查里标记的 W-1 问题。

---

## 7. 常用库/概念总览

| 库/概念 | 作用 | 关键 API |
|---|---|---|
| React 核心 | UI = f(state) | `useState` `useEffect` `useRef` `useMemo` `useCallback` |
| Zustand | 客户端全局状态 | `create()` `persist` `set/get` |
| TanStack Query | 服务端数据状态 | `useQuery` `useMutation` `queryKey/queryFn` |
| react-router | 路由 | `useNavigate` `<Route>` `useParams` |
| TypeScript | 静态类型 | `interface` `type` 泛型 `<T>` `?:` 可选 |
| Tailwind CSS | 原子化样式 | `className="flex gap-2 ..."` |
| Vite | 构建/开发服务器 | `import.meta.env.DEV` |

---

## 8. 小结：React 基础清单（自测）

- [ ] "UI = f(state)" 是什么意思？为什么改 state 才能更新界面？
- [ ] Hook 的两条调用规则？为什么不能放进 if 里？
- [ ] `useState` 为什么必须用 `setXxx`，直接赋值为什么无效？`setCount(prev=>prev+1)` 何时必须用？
- [ ] `useEffect` 依赖数组 `[]` / `[a]` / 省略 三者区别？清理函数干嘛的？
- [ ] `useRef` 和 `useState` 的核心区别（改动是否触发渲染）？两个典型用途？
- [ ] `useMemo`（缓存值）vs `useCallback`（缓存函数），什么时候才该用？
- [ ] 受控组件是什么？`value` + `onChange` 的配合？
- [ ] 什么是自定义 Hook？为什么以 `use` 开头？
- [ ] Zustand（客户端状态）与 TanStack Query（服务端状态）分别管什么？
- [ ] 条件渲染 `{cond && <X/>}` 和列表渲染的 `key` 为什么重要？

> 下一步（Next rep）：打开 `apps/web/src/features/auth/components/RegisterForm.tsx`，它和 LoginForm 结构相似。**不看答案，自己给它逐行加注释**，重点标出每个 Hook 的作用和"为什么用"。
