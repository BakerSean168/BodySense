# 问诊工作台历史记录区域重新设计方案

> 创建日期：2026-06-26
> 状态：待评审

---

## 一、概述

重新设计问诊工作台左侧历史记录区域，将现有的简单会话列表升级为三区布局（按钮区、置顶区、聊天区），引入扁平化卡片风格、AI 生成标题、置顶功能、会话分享等能力。同时引入 shadcn/ui 组件库，统一 UI 基础设施。

---

## 二、布局结构

侧边栏从上到下分为三个区域，用细分割线 + 区域小标题分隔：

```
┌─────────────────────────┐
│  [+ 开始新咨询]          │  ← 按钮区（主按钮）
│  [清空全部历史]          │  ← 文字按钮，带确认对话框
├─────────────────────────┤
│  📌 已置顶               │  ← 置顶区标题（仅有置顶会话时显示）
│  ┌───────────────────┐  │
│  │ 久坐颈椎酸痛咨询   │  │  ← 置顶会话卡片
│  │ 腰椎间盘突出咨询   │  │
│  └───────────────────┘  │
├─────────────────────────┤
│  全部会话               │  ← 聊天区标题（始终显示）
│  ┌───────────────────┐  │
│  │ 新咨询             │  │  ← 普通会话卡片
│  │ 膝盖疼痛问诊       │  │
│  │ 体态评估咨询       │  │
│  └───────────────────┘  │
└─────────────────────────┘
```

### 2.1 按钮区
- **开始新咨询**：主按钮，`rounded-full`，`bg-primary-700`，居上
- **清空全部历史**：较小文字按钮，放在下方，点击弹出确认对话框

### 2.2 置顶区
- 标题："📌 已置顶"
- 仅在有置顶会话时显示该区域
- 最多置顶 5 个会话，达到上限时提示"最多置顶 5 个会话，请先取消一个"

### 2.3 聊天区
- 标题："全部会话"
- 始终显示
- 不包含已置顶的会话（避免重复）

---

## 三、会话卡片设计

### 3.1 视觉风格（扁平化）
- 无边框（`border` 移除）
- 圆角 `rounded-lg`（8px），替代现有的 `rounded-[20px]`
- 默认背景：`bg-gray-50`
- 选中态：`bg-primary-50`，文字 `text-primary-900`
- 悬停态：背景色微变，无阴影
- 只显示标题，不显示日期、状态标签、body parts 等任何其他信息

### 3.2 交互行为
- **桌面端**：鼠标悬停时，在卡片右侧显示两个图标按钮
  - 📌 置顶/取消置顶
  - ⋯ 更多（弹出下拉菜单）
- **移动端**：按钮始终显示在卡片右侧（无 hover 事件）

### 3.3 会话标题
- **默认文本**："新咨询"
- **AI 自动生成**：会话首次持久化时（用户发送第一条消息、后端创建记录后），后端异步调用 LLM 根据对话内容生成简洁中文标题，存入 `title` 字段
- **用户重命名**：通过"更多"菜单触发内联编辑（inline editing）
  - 标题文本变为 input 框
  - 回车或点击外部保存，Esc 取消

---

## 四、下拉菜单（更多按钮）

### 4.1 菜单项
| 操作 | 说明 |
|------|------|
| 重命名 | 触发标题内联编辑 |
| 复制链接 | 生成分享链接并复制到剪贴板 |
| 取消分享 | 仅在已生成分享链接时显示，使链接失效 |
| 删除 | 弹出确认对话框后删除会话 |

### 4.2 样式
参考 ChatGPT 网页版的下拉菜单风格：
- 固定定位（`position: fixed`），跟随触发按钮
- 白色背景，圆角，轻微阴影
- 点击外部自动关闭
- 使用 shadcn/ui 的 `DropdownMenu` 组件

---

## 五、置顶功能

### 5.1 数据存储
后端 `consultation_sessions` 表新增字段：

```sql
ALTER TABLE consultation_sessions
ADD COLUMN pinned BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN pinned_at TIMESTAMP;
```

- `pinned`：是否置顶
- `pinned_at`：置顶时间，用于排序

### 5.2 排序逻辑
列表查询时按 `pinned DESC, pinned_at DESC, created_at DESC` 排序。前端分两组显示：置顶区（pinned=true）和聊天区（pinned=false）。

### 5.3 API 变更
- 新增 `PATCH /api/v1/consultation/{id}/pin` 切换置顶状态
- `GET /api/v1/consultation` 响应中包含 `pinned` 和 `pinned_at` 字段

---

## 六、会话分享功能

### 6.1 分享链接格式
`https://{domain}/consultation/share/{share_token}`

### 6.2 数据存储
新增 `consultation_shares` 表：

```sql
CREATE TABLE consultation_shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES consultation_sessions(id) ON DELETE CASCADE,
    share_token VARCHAR(32) UNIQUE NOT NULL,
    snapshot_messages JSONB NOT NULL,
    snapshot_diagnosis JSONB,
    snapshot_title VARCHAR(200),
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_consultation_shares_token ON consultation_shares(share_token);
CREATE INDEX idx_consultation_shares_session ON consultation_shares(session_id);
```

### 6.3 API 端点
| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/v1/consultation/{id}/share` | 生成分享链接，返回 `share_token` 和 `share_url` |
| DELETE | `/api/v1/consultation/{id}/share` | 取消分享，删除快照记录 |
| GET | `/api/v1/consultation/share/{share_token}` | 获取分享内容（无需认证） |

### 6.4 分享页面
- 路由：`/consultation/share/:token`
- 无需登录即可访问
- 展示内容：对话消息 + 诊断摘要（不显示 InfoPanel、BodyVisualization）
- 布局：居中卡片式（`max-w-2xl`）
  - 顶部：BodySense logo + "问诊分享" 标题
  - 中部：对话列表，聊天气泡样式与主应用一致
  - 诊断摘要：对话结束后用卡片展示
  - 底部："由 BodySense 智能问诊生成" + 注册/登录引导

### 6.5 分享链接生命周期
- 永不失效，除非用户主动取消分享或删除原会话
- 取消分享：通过"更多"菜单中的"取消分享"选项

---

## 七、后端变更

### 7.1 数据库迁移

**consultation_sessions 表新增字段：**
```sql
ALTER TABLE consultation_sessions
ADD COLUMN title VARCHAR(200),
ADD COLUMN pinned BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN pinned_at TIMESTAMP;
```

**新增 consultation_shares 表：**
```sql
CREATE TABLE consultation_shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES consultation_sessions(id) ON DELETE CASCADE,
    share_token VARCHAR(32) UNIQUE NOT NULL,
    snapshot_messages JSONB NOT NULL,
    snapshot_diagnosis JSONB,
    snapshot_title VARCHAR(200),
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
```

### 7.2 Go 模型变更

**ConsultationSession 模型新增字段：**
```go
Title    string     `gorm:"type:varchar(200)"`
Pinned   bool       `gorm:"not null;default:false"`
PinnedAt *time.Time
```

**新增 ConsultationShare 模型：**
```go
type ConsultationShare struct {
    ID               uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
    SessionID        uuid.UUID       `gorm:"type:uuid;not null;index"`
    ShareToken       string          `gorm:"type:varchar(32);uniqueIndex;not null"`
    SnapshotMessages json.RawMessage `gorm:"type:jsonb;not null"`
    SnapshotDiagnosis json.RawMessage `gorm:"type:jsonb"`
    SnapshotTitle    string          `gorm:"type:varchar(200)"`
    CreatedAt        time.Time       `gorm:"not null;default:now()"`
}
```

### 7.3 API 端点变更

| 方法 | 端点 | 变更类型 | 说明 |
|------|------|----------|------|
| PATCH | `/api/v1/consultation/{id}/pin` | 新增 | 切换置顶状态 |
| POST | `/api/v1/consultation/{id}/share` | 新增 | 生成分享链接 |
| DELETE | `/api/v1/consultation/{id}/share` | 新增 | 取消分享 |
| GET | `/api/v1/consultation/share/{share_token}` | 新增 | 获取分享内容（无需认证） |
| GET | `/api/v1/consultation` | 变更 | 响应新增 `pinned`、`pinned_at`、`title` 字段 |
| PUT | `/api/v1/consultation/{id}/title` | 新增 | 用户重命名标题 |

### 7.4 标题生成服务

在会话首次持久化时，异步调用 LLM 生成标题：
- 触发时机：创建会话记录后（`CreateSession` 方法末尾）
- Prompt：根据对话内容生成一句简洁的中文标题（不超过 20 字）
- 存储：更新 `consultation_sessions.title` 字段
- 失败处理：静默失败，不影响会话创建流程，前端回退显示"新咨询"

---

## 八、前端变更

### 8.1 引入 shadcn/ui

使用 `npx shadcn@latest init` 初始化，选择 Tailwind v4 兼容模式。需要添加的组件：
- `dropdown-menu` — 下拉菜单
- `dialog` — 确认对话框
- `sonner` — Toast 通知
- `button` — 按钮（如果现有按钮样式需要统一）

主题调整：将现有项目的 primary 色系（绿色调）映射到 shadcn 的 CSS 变量，保持视觉一致性。

### 8.2 新增/变更组件

| 组件 | 文件 | 变更类型 | 说明 |
|------|------|----------|------|
| `SessionHistorySidebar` | `components/SessionHistorySidebar.tsx` | 新增 | 从 ConsultationPage 中抽取的侧边栏组件 |
| `SessionCard` | `components/SessionCard.tsx` | 新增 | 单个会话卡片，支持置顶、更多菜单、内联编辑 |
| `SharePage` | `pages/SharePage.tsx` | 新增 | 分享页面，只读展示对话 + 诊断摘要 |
| `DropdownMenu` | `components/ui/dropdown-menu.tsx` | shadcn | 通用下拉菜单 |
| `Dialog` | `components/ui/dialog.tsx` | shadcn | 通用对话框 |
| `Toaster` | `components/ui/sonner.tsx` | shadcn | Toast 通知 |
| `ConsultationPage` | `pages/ConsultationPage.tsx` | 变更 | 重构侧边栏逻辑，抽取为独立组件 |

### 8.3 新增路由

```tsx
<Route path="/consultation/share/:token" element={<SharePage />} />
```

### 8.4 API 服务变更

`consultationService.ts` 新增方法：

```typescript
// 置顶
pinSession(sessionId: string): Promise<void>
unpinSession(sessionId: string): Promise<void>

// 分享
shareSession(sessionId: string): Promise<{ shareToken: string; shareUrl: string }>
unshareSession(sessionId: string): Promise<void>
getSharedSession(shareToken: string): Promise<SharedSession>

// 标题
renameSession(sessionId: string, title: string): Promise<void>
```

### 8.5 DTO 变更

`ConsultationSession` 类型新增字段：
```typescript
title?: string;
pinned: boolean;
pinned_at?: string;
share_token?: string;  // 如果已分享，包含此字段
```

---

## 九、移动端适配

- 移动端抽屉与桌面端侧边栏保持一致的三区布局
- 卡片上的置顶和更多按钮始终显示（不依赖 hover）
- 其他交互（下拉菜单、确认对话框、内联编辑）与桌面端一致

---

## 十、实施优先级

### P0 — 核心功能
1. 引入 shadcn/ui 并调整主题
2. 重构侧边栏为独立组件 `SessionHistorySidebar`
3. 新建 `SessionCard` 组件（扁平化卡片 + 标题显示）
4. 后端新增 `title`、`pinned`、`pinned_at` 字段
5. 实现三区布局（按钮区、置顶区、聊天区）
6. 实现置顶功能（前端 + 后端 API）
7. 实现会话重命名（内联编辑 + 后端 API）
8. 实现删除功能（确认对话框）

### P1 — 分享功能
9. 后端新增 `consultation_shares` 表和相关 API
10. 实现分享链接生成和复制
11. 实现分享页面 `SharePage`
12. 实现取消分享功能

### P2 — 体验优化
13. 移动端适配（按钮始终显示）
14. AI 标题生成服务
15. 列表排序优化
