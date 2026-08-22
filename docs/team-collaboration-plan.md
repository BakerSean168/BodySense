# BodySense 双人协作开发计划

本文档定义两位开发者（以下简称 A 和 B）在 BodySense 项目中的分工协作策略。核心原则：**结对轮转、全栈覆盖、AI 共学、学习优先**。

---

## 1. 协作模型：结对轮转（Pair & Rotate）

两人始终一起工作（Pair Programming），但在每个 issue 内部按技术层轮转 Driver 角色。Driver 负责写代码和做决策，Navigator 负责审查、查文档、提出改进建议。每完成一个 layer 后交换角色。

### 1.1 角色定义

| 角色 | 职责 | 学习价值 |
|------|------|----------|
| **Driver** | 键盘在手，写代码、调试、做技术决策 | 深度实践：在真实问题中学习 API 用法和工具链 |
| **Navigator** | 实时审查、查文档、提出方案、记录决策 | 广度思考：从更高视角理解架构、发现遗漏、学习 code review |

### 1.2 轮转规则

每个 issue 按技术层（DB → Backend → AI → Frontend）拆分，每层由不同的 Driver 负责。以 issue #2（用户注册/登录）为例：

```
┌──────────────────────────────────────────────────────────────┐
│  Layer 1: DB Schema (users 表)                                │
│  Driver: B    Navigator: A          ← B 学习 SQL + migration │
│                                                              │
│  Layer 2: Go API (register/login/refresh + middleware)         │
│  Driver: A    Navigator: B          ← A 学习 Go + Gin + JWT  │
│                                                              │
│  Layer 3: React 前端 (登录/注册页 + 路由守卫 + store)           │
│  Driver: B    Navigator: A          ← B 学习 React + Router  │
│                                                              │
│  Layer 4: 集成测试 + E2E 验证                                  │
│  Driver: A    Navigator: B          ← A 学习测试框架          │
└──────────────────────────────────────────────────────────────┘
```

**下一个 issue 从 A 开始 Driver**，确保两人驱动次数均衡。具体的每个 issue 分工见第 7 节速查表。

### 1.3 知识传递仪式

每个 issue 完成后，执行一次简短的知识传递：

1. **Driver 写 Learning Note**：在 PR description 或 commit message 中记录本层的关键学习点（踩坑、API 用法、设计决策）
2. **Navigator 做 Mini Review**：用 5 分钟口头总结审查中发现的模式和反模式
3. **共建 Cheat Sheet**：在 `docs/notes/` 下维护一份共享笔记，按技术栈分类（react.md、go.md、python-ai.md）

---

## 2. Sprint 编排

将 13 个 issue 组织为 5 个 Sprint，每个 Sprint 有明确的学习主题和交付物。Sprint 之间安排 **Learning Day** 用于复盘和专题深入学习。

### Sprint 路线图

```
Sprint 1          Sprint 2           Sprint 3          Sprint 4          Sprint 5
基础设施+认证      数据+AI基础         核心产品           闭环体验           增长+迭代
[2-3 周]          [2-3 周]           [3-4 周]          [2-3 周]          [2-3 周]

#1 开发环境       #4 基础信息采集    #6 对话区          #9 历史记录        #11 训练计划
#2 注册/登录      #5 文件上传+OCR    #7 评估报告        #10 分析+方案      #12 进度追踪
#3 RAG 基础设施   #13 知识库填充     #8 信息面板                           全部复盘

   ↓ LD-1            ↓ LD-2             ↓ LD-3            ↓ LD-4            ↓ 毕业项目
```

**LD = Learning Day**：每个 Sprint 结束后安排 1-2 天的专题学习，不写产品代码，专注技术深入。

---

## 3. Sprint 详细计划

### Sprint 1：基础设施 + 认证

**学习主题**：Docker 容器化、数据库设计、JWT 鉴权、React 路由基础

| Issue | 内容 | Driver 轮转 | 关键技术点 |
|-------|------|-------------|------------|
| #1 | 开发环境基础设施 | A: Docker → B: DB 迁移 | Docker Compose, pgvector, golang-migrate |
| #2 | 用户注册/登录 + JWT | B: DB/Go → A: React → B: 集成 | bcrypt, JWT 双 token, Zustand, React Router guards |

**Sprint 1 交付检查清单**：
- Docker 一键启动 dev 环境
- 注册 → 登录 → 受保护路由的完整 E2E 流程
- 能解释 JWT Access/Refresh Token 的工作原理

**LD-1 学习专题**：
- Docker 网络原理、volume 持久化、多阶段构建
- 密码安全：bcrypt vs argon2，为什么不能自己实现加密
- JWT 安全实践：token 存储位置（cookie vs localStorage）、XSS/CSRF 防护

### Sprint 2：数据采集 + AI 基础

**学习主题**：文件处理、OCR、向量数据库、embedding、RAG 管道

这是 AI 学习的**第一个里程碑**——从零搭建一个可工作的 RAG 系统。

| Issue | 内容 | Driver 轮转 | 关键技术点 |
|-------|------|-------------|------------|
| #4 | 身体基础信息采集 | A: DB/Go → B: React | 分步表单设计、表单校验、BMI 自动计算 |
| #5 | 文件上传 + OCR | B: Go 上传 → A: Python OCR | 文件存储、multipart form、PaddleOCR |
| #3 | 知识库 RAG 基础设施 | A: DB/pgvector → B: Python embedding → A: 检索 API | pgvector IVFFlat, embedding 模型, cosine similarity, reranker |
| #13 | 知识库内容填充（并行启动） | 两人协作 | 视频字幕提取、文本分块策略、embedding 批量处理 |

**Sprint 2 交付检查清单**：
- 用户可完成完整的 onboarding 流程（基础信息 + 文件上传）
- 知识库可写入条目并进行语义检索（写入 → 搜索 → 返回相关结果）
- 能解释 RAG 的完整数据流：文档 → chunk → embed → store → retrieve → augment

**LD-2 学习专题**（AI 深度学习日）：
- Embedding 原理：什么是向量表示？为什么语义相近的文本在向量空间中距离近？
- 向量数据库对比：pgvector vs Milvus vs Qdrant，各自适用场景
- Chunking 策略：固定长度 vs 语义分块 vs 递归分块，对检索质量的影响
- 动手实验：用不同 chunk size 和 overlap 参数对比检索效果

### Sprint 3：核心产品体验

**学习主题**：LLM 流式输出、SSE 通信、Prompt Engineering、前端复杂交互

这是产品最核心的部分，也是 AI agent 开发学习的**第二个里程碑**——掌握 LLM 集成和 prompt 设计。

| Issue | 内容 | Driver 轮转 | 关键技术点 |
|-------|------|-------------|------------|
| #6 | 咨询对话区 | B: Python LLM/SSE → A: Go SSE 代理 → B: React 流式渲染 | LangChain, SSE, 流式渲染, Function Calling, 对话记忆管理 |
| #7 | 健康评估报告 | A: Python 评估 prompt → B: Go API → A: React 展示 | 结构化输出、RAG 上下文拼装、评估维度设计 |
| #8 | 信息面板 + 身体可视化 | B: React SSE 监听 → A: SVG 可视化 → B: 交互逻辑 | SVG 交互, CSS 动画, 事件驱动架构, 响应式布局 |

**Sprint 3 交付检查清单**：
- 用户可在咨询工作台与 AI 进行多轮对话，AI 回复流式渲染
- AI 能从对话中自动提取症状信息，实时更新右侧面板
- 健康评估报告可正常生成和展示
- 能解释 Function Calling 的工作原理和适用场景

**LD-3 学习专题**（Prompt Engineering 深入学习）：
- Prompt 设计模式：Few-shot、Chain-of-Thought、Self-Consistency、ReAct
- Function Calling / Tool Use：如何设计 tool schema，让 LLM 输出结构化数据
- 对话记忆策略：滑动窗口 vs 摘要压缩 vs 实体记忆，token 预算管理
- 动手实验：为咨询工作台的三个阶段（引导/分析/方案）分别设计 prompt 并对比效果

### Sprint 4：闭环体验

**学习主题**：诊断推理、方案生成、数据持久化、复杂业务流程编排

| Issue | 内容 | Driver 轮转 | 关键技术点 |
|-------|------|-------------|------------|
| #9 | 会话保存 + 历史记录 | A: Go 持久化 → B: React 列表页 | JSONB 操作、分页查询、状态机管理 |
| #10 | 可能性分析 + 方案生成 | B: Python 诊断 prompt → A: Go 确认 API → B: React 交互 | 多阶段对话管理、置信度评估、RAG 方案检索 |
|  | 优化 + 补漏 | 两人协作 | 根据前三个 Sprint 的经验做重构和优化 |

**Sprint 4 交付检查清单**：
- 咨询会话自动保存，历史可查看
- 完整流程：对话 → 诊断选项 → 确认 → 生成方案
- 能解释 Agent Workflow 中"规划 → 执行 → 验证"的模式

**LD-4 学习专题**（AI Agent Workflow）：
- Agent 架构模式：Single-agent vs Multi-agent，Orchestrator 模式
- LangGraph 实战：构建有状态的 agent workflow（对话状态机）
- Tool/Function 编排：如何让 agent 自主决定调用哪个 tool
- 评估框架：如何衡量 AI 输出质量（faithfulness, relevance, safety）

### Sprint 5：训练 + 进度 + 毕业

**学习主题**：计划生成算法、数据可视化、AI 动态调整、产品闭环

| Issue | 内容 | Driver 轮转 | 关键技术点 |
|-------|------|-------------|------------|
| #11 | 训练计划 + 打卡 | A: Python 计划生成 → B: Go API → A: React 任务 UI | LLM 结构化计划输出、打卡逻辑、日期计算 |
| #12 | 进度追踪 + 复评 | B: Python 复评 → A: Go 统计 → B: React 图表 | Recharts/D3, AI 计划调整, 数据聚合 |

**Sprint 5 交付检查清单**：
- 完整产品闭环：注册 → 采集 → 评估 → 咨询 → 诊断 → 训练 → 打卡 → 复评
- 能解释整个系统中 AI 扮演的角色（不是"调 API"，而是理解每个环节的 AI 设计意图）

---

## 4. AI 学习路径（渐进式）

两位开发者共同沿以下路径深入学习 AI agent 开发。每个阶段与 Sprint 对齐，确保"学到就用"。

```
Phase 1              Phase 2              Phase 3              Phase 4
RAG 基础              Prompt 工程           Agent Workflow        生产级 AI
(Sprint 2)           (Sprint 3)           (Sprint 4)           (Sprint 5+)

┌─────────┐       ┌─────────────┐      ┌──────────────┐     ┌─────────────┐
│ Embedding│       │ 系统 Prompt  │      │ 状态机管理    │     │ 评估与监控   │
│ pgvector │       │ Few-shot    │      │ LangGraph    │     │ Faithfulness│
│ 语义检索  │──────→│ CoT 推理    │─────→│ 多 Tool 编排  │────→│ 安全防护     │
│ Reranker │       │ Function Call│      │ 对话策略     │     │ A/B 测试    │
│ 上下文拼装│       │ 结构化输出   │      │ 自我反思     │     │ 成本优化     │
└─────────┘       └─────────────┘      └──────────────┘     └─────────────┘
```

### 每阶段学习产出

| 阶段 | 核心概念 | 动手实验 | 输出物 |
|------|----------|----------|--------|
| Sprint 1 | 工程基础（Docker、JWT、Git 协作流程） | 搭建 dev 环境，完成第一个认证 E2E 流程 | 熟悉工具链，为 AI 学习打好工程基础 |
| Phase 1: RAG | 向量表示、相似度检索、chunking | 不同 chunk 策略的检索效果对比 | `docs/notes/rag-learning.md` |
| Phase 2: Prompt | 提示模式、tool schema、记忆管理 | 同一需求不同 prompt 设计的输出对比 | `docs/notes/prompt-engineering.md` |
| Phase 3: Agent | 状态机、tool 编排、对话策略 | 用 LangGraph 重构咨询工作台的对话管理 | `docs/notes/agent-workflow.md` |
| Phase 4: 生产 | 评估指标、安全防护、成本 | 建立评估数据集，跑 faithfulness/relevance 评分 | `docs/notes/ai-production.md` |

> Sprint 1 不涉及 AI 学习，专注工程基础。这很重要——扎实的工程基础（Docker、Git、CI/CD）是后续 AI 开发的前提。Phase 4 的学习穿插在 Sprint 5 的 Learning Day 中，并在 MVP 完成后的迭代中持续深入。

### 学习资源推荐

**Phase 1 — RAG**：
- LangChain RAG tutorial（官方文档，最权威的入门路径）
- pgvector 官方文档 + "How to Choose the Right Vector Database" 对比文章
- 论文：《Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks》

**Phase 2 — Prompt Engineering**：
- Anthropic 的 Prompt Engineering Guide（系统性最强）
- OpenAI Cookbook 中的 Function Calling 示例
- 论文：《Chain-of-Thought Prompting Elicits Reasoning in Large Language Models》

**Phase 3 — Agent Workflow**：
- LangGraph 官方教程（构建有状态 agent 的最佳实践）
- Anthropic 的 "Building Effective Agents" 博客（agent 设计哲学）
- 论文：《ReAct: Synergizing Reasoning and Acting in Language Models》

**Phase 4 — 生产级 AI**：
- RAGAS 框架（RAG 评估标准）
- Anthropic 的 Constitutional AI 和安全实践
- LLM 成本计算器 + 缓存策略设计

---

## 5. 每日开发节奏（Daily Ritual）

```
09:30 ─ Stand-up (15min)
        ├── 昨天做了什么？遇到什么卡点？
        ├── 今天要做什么？
        └── 确认今天 Driver 轮转顺序

09:45 ─ Context Loading
        ├── 一起读 AGENT.md 更新
        ├── git log 看最近变更
        └── 打开当前 issue，过一遍 acceptance criteria

10:00 ─ Session 1: Driver A (90min)
        ├── A 写代码，B 审查 + 查文档
        └── 遇到关键决策时暂停讨论

        ☕ 休息 15min

11:15 ─ Session 2: Driver B (90min)
        ├── B 写代码，A 审查 + 查文档
        └── 角色互换，保证两人都有键盘时间

        🍜 午休

14:00 ─ Session 3: 自由安排
        ├── 继续 pair programming（如果需要）
        ├── 或各自做 Learning Day 的预研
        └── 或写 Learning Note / 更新文档

16:00 ─ Wrap-up (15min)
        ├── 验证 acceptance criteria
        ├── commit（遵循 conventional commits）
        └── 记录今天的学习点到 docs/notes/
```

---

## 6. 代码协作规范

### 6.1 分支策略

采用简单的 trunk-based 开发（两人团队不需要复杂的 Git Flow）：

```
main ← 稳定分支，始终保持可部署状态
  └── feat/{issue-number}-{short-name}  ← 每个 issue 一个分支
       例：feat/2-auth-jwt
            feat/3-rag-infrastructure
```

- 每个 issue 开一个 feature 分支，命名 `feat/{issue-number}-{short-name}`
- 完成后提 PR，两人互相 review（Navigator 负责主要 review）
- 合并到 main 前必须通过 CI（lint + typecheck + test）

### 6.2 PR Review 规范

PR review 是学习中最重要的环节之一。好的 review 不只是找 bug，更是知识传递：

- **每个 PR 必须包含**：变更说明、测试结果、Learning Notes（本 PR 学到的关键点）
- **Reviewer 关注**：
  - 架构合理性（是否符合 AGENT.md 中定义的架构）
  - 代码可读性（命名、注释、结构）
  - 测试覆盖（至少覆盖 happy path + 一个 edge case）
  - 学习价值（这个 PR 有没有可以更好的写法？有没有更好的模式？）
- **Review 讨论**：用 GitHub PR comment 讨论，结论总结到 Learning Note

### 6.3 AI 辅助开发的分工

两人使用 AI 工具（Claude Code、QoderWork、GitHub Copilot）时遵循以下约定：

- **Driver 操作 AI**：Driver 负责与 AI 对话、审查 AI 输出、做最终决策
- **Navigator 验证 AI**：Navigator 对照 AGENT.md 检查 AI 输出是否违反约束
- **不盲目接受**：AI 生成的代码必须两人一起理解后再采纳，不能"看不懂但先跑起来"
- **记录 AI 决策**：当 AI 做了重要技术决策时，在 Learning Note 中记录"为什么选这个方案"

---

## 7. Issue 分工速查表

下表展示每个 issue 的 Driver 分配建议。原则：**同一 issue 内每层轮转，issue 之间起始 Driver 交替**。

| Issue | 模块 | Layer 1 Driver | Layer 2 Driver | Layer 3 Driver | Layer 4 Driver |
|-------|------|---------------|---------------|---------------|---------------|
| #1 | 基础设施 | A: Docker | B: DB 迁移 | — | — |
| #2 | 认证 | B: DB schema | A: Go API | B: React | A: 集成测试 |
| #4 | 信息采集 | A: DB/Go | B: React | — | — |
| #5 | 文件+OCR | B: Go 上传 | A: Python OCR | B: React | — |
| #3 | RAG 基础 | A: DB/pgvector | B: Python embed | A: 检索 API | — |
| #13 | 知识库填充 | A: 脚本 | B: 数据整理 | 两人: 质量校验 | — |
| #6 | 对话区 | B: Python LLM | A: Go SSE | B: React 流式 | A: 记忆管理 |
| #7 | 评估报告 | A: Python prompt | B: Go API | A: React 展示 | — |
| #8 | 信息面板 | B: React 面板 | A: SVG 可视化 | B: 联动逻辑 | — |
| #9 | 历史记录 | A: Go 持久化 | B: React 列表 | — | — |
| #10 | 分析+方案 | B: Python 诊断 | A: Go 确认 | B: React 交互 | A: 方案生成 |
| #11 | 训练计划 | A: Python 生成 | B: Go API | A: React 任务 | B: 打卡 |
| #12 | 进度追踪 | B: Python 复评 | A: Go 统计 | B: React 图表 | — |

**统计**：按速查表统计，A 和 B 驱动的层数基本均衡（具体数字随 issue 拆分粒度微调，原则是同一 Sprint 内两人层数差不超过 2）。

---

## 8. 里程碑与自测

每个 Sprint 结束后，两人一起完成以下自测，确保学习成果扎实：

### Sprint 1 自测题

1. 解释 Docker Compose 中 `healthcheck` 的作用，为什么需要它？
2. JWT 的 Access Token 和 Refresh Token 分别存在哪里？过期后怎么处理？
3. 为什么密码要用 bcrypt 而不是 MD5？cost factor 是什么意思？

### Sprint 2 自测题

1. Embedding 是什么？为什么"圆肩"和"上交叉综合征"的向量距离近？
2. pgvector 的 IVFFlat 索引是怎么工作的？`lists = 100` 是什么意思？
3. RAG 管道中 reranker 的作用是什么？去掉它会有什么影响？
4. OCR 识别出错的场景有哪些？怎么设计 fallback？

### Sprint 3 自测题

1. SSE（Server-Sent Events）和 WebSocket 的区别是什么？为什么这里选 SSE？
2. Function Calling 是怎么工作的？LLM 怎么知道要调用哪个 function？
3. 对话记忆管理中，为什么不把全部历史都给 LLM？token 超了会怎样？
4. Prompt 中注入 RAG 上下文时，怎么避免"context stuffing"问题？
5. SVG 人体可视化中，如何用 CSS/JS 控制特定部位的高亮状态？`<path>` 和 `<g>` 元素的作用是什么？
6. 信息面板的 SSE 事件监听和对话消息渲染如何解耦？事件驱动架构在这里的优势是什么？

### Sprint 4 自测题

1. 咨询会话的状态机有几个状态？状态转换的触发条件是什么？
2. 可能性分析中，AI 的"置信度"是怎么得出的？是 LLM 自己判断还是有外部评估？
3. 如何确保 AI 的诊断输出不会遗漏重要症状？

### Sprint 5 自测题

1. 训练计划的结构化数据怎么设计？为什么用 JSONB 而不是关联表？
2. AI 动态调整计划时，怎么确保不会生成不安全的训练建议？
3. 进度面板中用了哪些图表类型？折线图、进度条、统计卡片各自适合展示什么数据？
4. 进度统计的 SQL 查询怎么写？连续打卡天数怎么计算（窗口函数 vs 应用层逻辑）？
5. 回顾整个系统，画出完整的数据流图：用户输入 → 各服务处理 → 最终输出

---

*文档版本：v1.0 | 更新日期：2026-06-19*
*关联文档：[PRD](PRD-体态健康AI助手.md) · [技术方案](architecture/technical-approach.md) · [AI 开发流程](ai-development-workflow.md) · [GitHub Issues](https://github.com/BakerSean168/BodySense/issues)*
