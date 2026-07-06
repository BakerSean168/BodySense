# 04 — Python AI 服务接口契约

## 概述

Python AI 服务是一个**无状态的 LLM 执行引擎**。它不管理会话、不写数据库、不了解客户端协议。Go API 是唯一的编排器和 DB 写入者。

---

## 接口 1：聊天流式生成

```
POST /api/chat/stream
Content-Type: application/json
```

### 请求

```json
{
  "messages": [
    {
      "role": "system",
      "content": "你是 BodySense 体态健康顾问..."
    },
    {
      "role": "user",
      "content": "我最近腰有点不舒服"
    }
  ],
  "context": {
    "user_id": "user_xxx",
    "session_id": "conv_xxx",
    "profile": {
      "age": 30,
      "gender": "male",
      "occupation": "程序员"
    },
    "extracted_info": [
      {"body_part": "腰部", "symptom_type": "不适", "duration": "一周"}
    ],
    "phase": "collecting",
    "previous_response_id": "resp_xxx"
  },
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "search_knowledge",
        "description": "搜索知识库获取相关健康资料",
        "parameters": {
          "type": "object",
          "properties": {
            "query": {"type": "string"}
          },
          "required": ["query"]
        }
      }
    },
    {
      "type": "function",
      "function": {
        "name": "extract_symptoms",
        "description": "从用户描述中提取结构化症状信息",
        "parameters": {
          "type": "object",
          "properties": {
            "body_part": {"type": "string"},
            "symptom_type": {"type": "string"},
            "duration": {"type": "string"},
            "trigger": {"type": "string"},
            "severity": {"type": "string"}
          },
          "required": ["body_part", "symptom_type"]
        }
      }
    }
  ],
  "model": "qwen-max",
  "stream": true
}
```

### 响应：结构化 JSON 行流

Python 返回 `Content-Type: application/x-ndjson`，每行一个 JSON 对象。

**事件类型：**

```jsonl
{"type":"text_delta","delta":"我来帮你"}
{"type":"text_delta","delta":"分析一下"}
{"type":"tool_call","id":"call_001","tool":"search_knowledge","args":{"query":"腰部不适原因"}}
{"type":"tool_result","id":"call_001","tool":"search_knowledge","result":{"items":[...]}}
{"type":"tool_call","id":"call_002","tool":"extract_symptoms","args":{"body_part":"腰部","symptom_type":"不适"}}
{"type":"tool_result","id":"call_002","tool":"extract_symptoms","result":{"body_part":"腰部","symptom_type":"不适","severity":"unknown"}}
{"type":"phase_change","phase":"ready_for_analysis","reason":"已收集足够症状信息"}
{"type":"red_flag","flag":{"type":"red_flag","message":"建议尽快就医","severity":"high"}}
{"type":"citation","citation":{"title":"腰痛康复指南","snippet":"..."}}
{"type":"done","response_id":"resp_xxx","usage":{"input_tokens":1234,"output_tokens":567}}
```

### 事件详细定义

| type | 字段 | 说明 |
|------|------|------|
| `text_delta` | `delta: string` | 流式文本增量 |
| `tool_call` | `id, tool, args` | 工具调用发起 |
| `tool_result` | `id, tool, result` | 工具执行结果 |
| `phase_change` | `phase, reason` | 阶段转换提议 |
| `extracted_info` | `info: object` | 提取的结构化症状信息 |
| `red_flag` | `flag: object` | 安全警告 |
| `citation` | `citation: object` | 知识库引用 |
| `done` | `response_id, usage` | 流结束，包含 response_id 和 token 用量 |

---

## 接口 2：诊断分析

```
POST /api/diagnosis/analyze
Content-Type: application/json
```

### 请求

```json
{
  "extracted_info": [
    {"body_part": "腰部", "symptom_type": "不适", "duration": "一周", "trigger": "久坐"}
  ],
  "profile": {
    "age": 30,
    "gender": "male",
    "occupation": "程序员"
  },
  "rag_context": "知识库检索结果的 markdown 格式文本",
  "rag_results": [
    {"title": "腰痛常见原因", "similarity": 0.85, "content": "..."}
  ],
  "model": "qwen-max"
}
```

### 响应

```json
{
  "diagnoses": [
    {
      "name": "腰肌劳损",
      "confidence": 0.75,
      "severity": "moderate",
      "basis": "患者久坐职业，腰部不适一周，无放射痛...",
      "typical_symptoms": ["腰部酸痛", "久坐加重", "活动后缓解"],
      "differential": ["腰椎间盘突出", "肾结石"]
    },
    {
      "name": "腰椎间盘突出",
      "confidence": 0.15,
      "severity": "moderate",
      "basis": "...",
      "typical_symptoms": ["..."],
      "differential": ["..."]
    }
  ],
  "citations": [
    {"title": "腰痛康复指南", "snippet": "..."}
  ]
}
```

---

## 接口 3：治疗方案生成

```
POST /api/diagnosis/treatment
Content-Type: application/json
```

### 请求

```json
{
  "diagnosis": {
    "name": "腰肌劳损",
    "confidence": 0.75,
    "severity": "moderate"
  },
  "extracted_info": [...],
  "profile": {...},
  "rag_context": "...",
  "rag_results": [...],
  "model": "qwen-max"
}
```

### 响应

```json
{
  "goal": "缓解腰部肌肉紧张，改善久坐姿势",
  "duration_weeks": 4,
  "correction_exercises": [
    {
      "name": "猫牛式伸展",
      "description": "四点跪撑，交替弓背和塌腰",
      "frequency": "每天2次，每次10个",
      "precautions": "动作缓慢，避免疼痛"
    }
  ],
  "daily_habits": [
    "每坐45分钟站起活动5分钟",
    "使用腰垫支撑"
  ],
  "nutrition_advice": "适量补充钙质和维生素D",
  "expected_timeline": "1-2周疼痛减轻，4周明显改善",
  "warning_signs": [
    "腿部放射痛或麻木",
    "排尿困难",
    "疼痛持续加重"
  ],
  "citations": [...]
}
```

---

## 接口 4：知识库搜索

```
POST /api/knowledge/search
Content-Type: application/json
```

### 请求

```json
{
  "query": "腰部不适 原因 治疗",
  "top_k": 5
}
```

### 响应

```json
{
  "results": [
    {
      "id": "kb_001",
      "title": "腰痛常见原因及康复方法",
      "content": "...",
      "category": "self_check",
      "similarity": 0.85
    }
  ]
}
```

---

## Go → Python 映射关系

Go 在调用 Python 前，从数据库加载上下文并构造请求：

```
Go 数据库                          Python 请求
─────────                          ────────────
conversations.default_model    →   model
messages (最近 N 条)             →   messages[]
consultation_sessions.phase    →   context.phase
consultation_sessions.info     →   context.extracted_info
profiles                       →   context.profile
conversations.provider_last_response_id → context.previous_response_id
```

## Python → Go 事件映射

Go 收到 Python 的结构化事件后，映射为客户端 SSE：

```
Python 事件                    客户端 SSE 事件
───────────                    ────────────────
text_delta                     text.delta
tool_call                      tool.call
tool_result                    tool.result
extracted_info                 extracted_info
phase_change                   phase_change (Go 校验后)
red_flag                       red_flag
citation                       citation
done                           message.completed + done
```

Go 额外生成的事件（Python 不涉及）：

```
conversation.created    — 新会话创建
message.persisted       — 用户消息落库
message.created         — assistant placeholder 创建
message.failed          — 生成失败
title.generated         — 标题异步生成完成
```

---

## Python 服务内部架构建议

```
┌─────────────────────────────────────┐
│          FastAPI App                │
│                                     │
│  POST /api/chat/stream             │
│    │                                │
│    ▼                                │
│  ChatAgent                          │
│    ├── build_prompt(messages, ctx)  │
│    ├── call_llm(stream=True)        │
│    ├── handle_tool_calls()          │
│    │     ├── search_knowledge()     │
│    │     ├── extract_symptoms()     │
│    │     └── re-prompt with results │
│    └── yield structured events      │
│                                     │
│  POST /api/diagnosis/analyze        │
│    └── DiagnosisService             │
│                                     │
│  POST /api/diagnosis/treatment      │
│    └── TreatmentService             │
│                                     │
│  POST /api/knowledge/search         │
│    └── KnowledgeService (RAG)       │
└─────────────────────────────────────┘
```

Python 服务是无状态的。所有持久化由 Go API 负责。Python 可以随时水平扩展或替换。
