# 05 — 前端改造方案

## 概述

前端改造的核心目标：

1. 适配新的 SSE 协议（Go 定义的事件格式）
2. 实现懒创建流程（draft → conversation.created → router.replace）
3. 适配新的 API 端点和数据结构
4. 保持 `@assistant-ui/react` 库的使用

---

## 路由设计

```
/consultation                           → 重定向到 /consultation/new
/consultation/new                       → 新咨询草稿页（不入库）
/consultation/:id                       → 咨询会话页（id 为 conversationId）
```

| 场景 | 路由 | 行为 |
|------|------|------|
| 点击"新咨询" | `/consultation/new` | 显示空白聊天界面，不请求后端 |
| 发送首条消息 | `/consultation/new` → `/consultation/:id` | SSE 返回 `conversation.created` 后 router.replace |
| 打开已有会话 | `/consultation/:id` | GET 加载会话详情 + 消息历史 |
| 侧边栏点击 | 导航到 `/consultation/:id` | 加载对应会话 |

---

## 核心状态管理

### ConsultationPage 状态

```typescript
interface ConsultationPageState {
  // 当前会话
  session: ConsultationSession | null;

  // 草稿状态
  isDraft: boolean;           // true = 新建未保存
  clientDraftId: string | null;

  // 咨询领域数据
  extractedInfo: ExtractedInfo[];
  diagnoses: Diagnosis[];
  treatmentPlan: TreatmentPlan | null;

  // 侧边栏
  sessions: ConsultationSessionSummary[];

  // 流式状态
  isStreaming: boolean;
  streamingMessageId: string | null;
}
```

### 会话数据结构

```typescript
interface ConsultationSession {
  id: string;                     // conversationId (服务端生成)
  title: string | null;
  status: 'active' | 'archived';
  model: string;

  messages: ChatMessage[];
  metadata: Record<string, unknown>;

  // 咨询扩展
  consultation: {
    phase: ConsultationPhase;
    extractedInfo: ExtractedInfo[];
    diagnosis: DiagnosisAnalysis | null;
    treatmentPlan: TreatmentPlan | null;
  };

  createdAt: string;
  updatedAt: string;
  lastMessageAt: string | null;
}

interface ChatMessage {
  id: string;                     // 服务端正式 ID
  turnId: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  status: 'submitted' | 'streaming' | 'completed' | 'failed';
  parts: MessagePart[];
  inputTokens?: number;
  outputTokens?: number;
  createdAt: string;
}

type MessagePart =
  | { type: 'text'; text: string }
  | { type: 'source'; title: string; snippet?: string }
  | { type: 'tool-call'; tool: string; args: unknown }
  | { type: 'tool-result'; tool: string; result: unknown };
```

---

## 懒创建流程

### 新建咨询

```typescript
// ConsultationPage.tsx
function handleNewConsultation() {
  // 不请求后端，不入库
  setSession(null);
  setIsDraft(true);
  setClientDraftId(crypto.randomUUID());
  setMessages([]);
  setExtractedInfo([]);
  setDiagnoses([]);
  setTreatmentPlan(null);
  router.navigate('/consultation/new', { replace: true });
}
```

### 发送首条消息

```typescript
// useAssistantChatRuntime.ts
async function sendMessage(content: string) {
  const clientMessageId = `tmp_${crypto.randomUUID()}`;
  const requestId = crypto.randomUUID();

  // 乐观渲染
  addOptimisticMessage({
    id: clientMessageId,
    role: 'user',
    parts: [{ type: 'text', text: content }],
    status: 'submitted',
  });

  // 发送请求
  const response = await fetch('/api/v1/chat/send', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({
      conversationId: session?.id ?? null,
      clientDraftId: isDraft ? clientDraftId : undefined,
      clientMessageId,
      requestId,
      message: {
        role: 'user',
        parts: [{ type: 'text', text: content }],
      },
      context: {
        entry: 'consultation',
        profileId: profile?.id,
      },
      model: session?.model ?? 'qwen-max',
    }),
  });

  // 处理 SSE 流
  const reader = response.body!.getReader();
  const decoder = new TextDecoder();
  // ... 解析 SSE 事件
}
```

---

## SSE 事件处理

### 事件分发器

```typescript
// useSSEProcessor.ts
type SSEEventHandler = (event: SSEEvent) => void;

interface SSEEvent {
  event: string;
  data: unknown;
}

function useSSEProcessor(handlers: Record<string, SSEEventHandler>) {
  return {
    processLine(line: string) {
      if (line.startsWith('event: ')) {
        currentEvent = line.slice(7).trim();
      } else if (line.startsWith('data: ')) {
        const data = JSON.parse(line.slice(6));
        handlers[currentEvent]?.({ event: currentEvent, data });
      }
    },
  };
}
```

### 各事件处理逻辑

```typescript
const sseHandlers: Record<string, SSEEventHandler> = {
  'conversation.created': (e) => {
    const { conversationId, replacesDraftId } = e.data;

    // 替换路由
    router.replace(`/consultation/${conversationId}`);

    // 更新本地状态
    setSession(prev => ({ ...prev!, id: conversationId }));
    setIsDraft(false);
    setClientDraftId(null);
  },

  'message.persisted': (e) => {
    const { clientMessageId, messageId } = e.data;

    // 替换临时 ID 为正式 ID
    updateMessageId(clientMessageId, messageId);
  },

  'message.created': (e) => {
    const { messageId, role, status, turnId } = e.data;

    if (role === 'assistant') {
      addMessage({
        id: messageId,
        turnId,
        role: 'assistant',
        status: 'streaming',
        parts: [],
      });
      setStreamingMessageId(messageId);
    }
  },

  'text.delta': (e) => {
    const { messageId, delta } = e.data;
    appendTextDelta(messageId, delta);
  },

  'extracted_info': (e) => {
    const { info } = e.data;
    setExtractedInfo(prev => {
      const existing = prev.findIndex(i => i.bodyPart === info.body_part);
      if (existing >= 0) {
        const updated = [...prev];
        updated[existing] = mapExtractedInfo(info);
        return updated;
      }
      return [...prev, mapExtractedInfo(info)];
    });
  },

  'phase_change': (e) => {
    const { from, to, reason, rejected } = e.data;
    if (!rejected) {
      setSession(prev => ({
        ...prev!,
        consultation: { ...prev!.consultation, phase: to },
      }));
    }
  },

  'citation': (e) => {
    const { citation } = e.data;
    // 追加到当前 assistant 消息的引用列表
    appendCitation(streamingMessageId, citation);
  },

  'red_flag': (e) => {
    const { flag } = e.data;
    appendRedFlag(streamingMessageId, flag);
  },

  'tool.call': (e) => {
    const { tool } = e.data;
    // 可选：显示"正在检索..."状态
    setToolStatus({ tool, status: 'calling' });
  },

  'tool.result': (e) => {
    setToolStatus(null);
  },

  'message.completed': (e) => {
    const { messageId, usage } = e.data;
    updateMessageStatus(messageId, 'completed', usage);
    setStreamingMessageId(null);
    setIsStreaming(false);
  },

  'message.failed': (e) => {
    const { messageId, error } = e.data;
    updateMessageStatus(messageId, 'failed', { error });
    setStreamingMessageId(null);
    setIsStreaming(false);
  },

  'title.generated': (e) => {
    const { conversationId, title } = e.data;
    updateSessionTitle(conversationId, title);
    refreshSessionList();  // 侧边栏更新
  },

  'done': () => {
    // 流结束，清理状态
  },
};
```

---

## AssistantChatPanel 适配

### 与 @assistant-ui/react 集成

当前使用 `useLocalRuntime` + 自定义 `ChatModelAdapter`。新协议下需要：

1. **Adapter 的 `run` 方法**改为解析新 SSE 协议
2. **初始消息加载**改为从新 API 获取（parts 格式）
3. **消息合并逻辑**适配新的 `ChatMessage` 结构

```typescript
// useAssistantChatRuntime.ts
class ConsultationChatAdapter implements ChatModelAdapter {
  async *run({ messages, abortSignal }: ChatModelRunOptions) {
    const lastMessage = messages[messages.length - 1];
    const content = extractTextContent(lastMessage);

    const response = await fetch('/api/v1/chat/send', {
      method: 'POST',
      signal: abortSignal,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        conversationId: this.conversationId,
        clientMessageId: `tmp_${crypto.randomUUID()}`,
        requestId: crypto.randomUUID(),
        message: { role: 'user', parts: [{ type: 'text', text: content }] },
        model: this.model,
      }),
    });

    // 解析 SSE 流
    const reader = response.body!.getReader();
    let fullText = '';

    // ... 逐行解析，yield assistant-ui 格式的 parts

    yield {
      content: [{ type: 'text', text: fullText }],
    };
  }
}
```

---

## API Service 层

### consultationService.ts 改造

```typescript
export const consultationApi = {
  // 发送消息（返回 SSE Response）
  sendMessage(params: {
    conversationId: string | null;
    clientDraftId?: string;
    clientMessageId: string;
    requestId: string;
    message: { role: string; parts: MessagePart[] };
    context?: Record<string, unknown>;
    model?: string;
  }): Promise<Response> {
    return authFetch('/api/v1/chat/send', {
      method: 'POST',
      body: JSON.stringify(params),
    });
  },

  // 获取会话列表
  listConversations(params?: { cursor?: string; limit?: number }) {
    return authFetch('/api/v1/conversations', { params });
  },

  // 获取会话详情
  getConversation(id: string) {
    return authFetch(`/api/v1/conversations/${id}`);
  },

  // 删除会话
  deleteConversation(id: string) {
    return authFetch(`/api/v1/conversations/${id}`, { method: 'DELETE' });
  },

  // 获取咨询详情（含领域数据）
  getConsultation(id: string) {
    return authFetch(`/api/v1/consultations/${id}`);
  },

  // 更新提取信息
  updateExtractedInfo(id: string, info: ExtractedInfo[]) {
    return authFetch(`/api/v1/consultations/${id}/extracted-info`, {
      method: 'PUT',
      body: JSON.stringify({ extracted_info: info }),
    });
  },

  // 生成诊断
  analyzeDiagnosis(id: string) {
    return authFetch(`/api/v1/consultations/${id}/diagnosis`, {
      method: 'POST',
    });
  },

  // 确认诊断
  confirmDiagnosis(id: string, diagnosis: Diagnosis) {
    return authFetch(`/api/v1/consultations/${id}/confirm`, {
      method: 'PUT',
      body: JSON.stringify({ diagnosis }),
    });
  },

  // 生成治疗方案
  generateTreatment(id: string) {
    return authFetch(`/api/v1/consultations/${id}/treatment`, {
      method: 'POST',
    });
  },

  // 异步生成标题
  generateTitle(id: string) {
    return authFetch(`/api/v1/conversations/${id}/title`, {
      method: 'POST',
    });
  },
};
```

---

## 文件结构变更

```
apps/web/src/features/consultation/
├── components/
│   ├── AssistantChatPanel.tsx      // 保留，适配新协议
│   ├── InfoPanel.tsx               // 保留
│   ├── DiagnosisPanel.tsx          // 保留
│   ├── TreatmentPlanView.tsx       // 保留
│   ├── RedFlagBanner.tsx           // 保留
│   ├── SessionSidebar.tsx          // 改造，适配新 API
│   ├── ChatInput.tsx               // 删除（已废弃）
│   └── ChatMessage.tsx             // 删除（已废弃）
├── hooks/
│   ├── useAssistantChatRuntime.ts  // 重写，适配新 SSE 协议
│   ├── useChatSSE.ts               // 重写，新事件格式
│   └── useSSEProcessor.ts          // 新增，SSE 事件分发器
├── pages/
│   └── ConsultationPage.tsx        // 改造，懒创建 + 新数据结构
├── services/
│   └── consultationService.ts      // 改造，新 API 端点
├── types/
│   └── consultation.ts             // 改造，新数据类型
└── index.ts                        // 更新导出
```

---

## 与现有实现的对比

| 维度 | 现有实现 | 新方案 |
|------|----------|--------|
| 会话创建 | 前端生成 UUID，`is_new` 标记 | 草稿态，SSE 返回后 router.replace |
| 消息 ID | 无正式 ID | 临时 ID → SSE 返回正式 ID 替换 |
| SSE 协议 | Python 定义，Go 转发 | Go 定义，事件更丰富 |
| 会话加载 | `session.Messages` JSONB 数组 | 独立消息 API，parts 格式 |
| 错误处理 | 流中断，无明确反馈 | `message.failed` 事件 + 重试按钮 |
| 标题生成 | 无 | 异步 SSE 通知 `title.generated` |
