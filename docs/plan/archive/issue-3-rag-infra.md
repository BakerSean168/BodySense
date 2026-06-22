# Issue #3: 知识库 RAG 基础设施实施计划

**Issue**: [ai] 知识库 RAG 基础设施：embedding + 语义检索  
**分支**: `feat/3-rag-infra`  
**创建日期**: 2026-06-21  
**状态**: 进行中

---

## 📋 需求概述

实现知识库的 RAG（检索增强生成）基础设施，包括 pgvector 向量存储、embedding 生成、语义检索和重排序。这是所有 AI 功能（评估报告、对话问诊、训练计划）的底层依赖。

---

## 🎯 验收标准

- [ ] DB: knowledge_entries 表创建，含 category/title/content/embedding/source_video/source_timestamp 字段
- [ ] DB: pgvector 扩展启用，IVFFlat 向量索引创建
- [ ] Python: embedding 生成接口可将文本转为 1536 维向量
- [ ] Python: 语义检索接口返回 top-k 相关知识条目（含相关性分数）
- [ ] Python: reranker 对检索结果做二次筛选
- [ ] Python: 知识入库 API（写入条目 + 自动生成 embedding）
- [ ] Go: 代理端点转发知识库搜索请求到 Python 服务
- [ ] 端到端：写入 10 条测试知识 → 语义搜索返回最相关的 3 条

---

## 🏗️ 实施步骤

### 阶段 1：数据库层 (DB)

**目标**: 创建 knowledge_entries 表和 pgvector 索引

#### 1.1 创建数据库迁移文件
- 文件: `apps/api/migrations/000003_create_knowledge_entries.up.sql`
- 内容:
  ```sql
  -- 启用 pgvector 扩展
  CREATE EXTENSION IF NOT EXISTS vector;

  -- 创建 knowledge_entries 表
  CREATE TABLE knowledge_entries (
      id BIGSERIAL PRIMARY KEY,
      category VARCHAR(100) NOT NULL,
      title VARCHAR(500) NOT NULL,
      content TEXT NOT NULL,
      embedding vector(1536),
      source_video VARCHAR(500),
      source_timestamp VARCHAR(50),
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
  );

  -- 创建 IVFFlat 向量索引（余弦相似度）
  CREATE INDEX idx_knowledge_entries_embedding
  ON knowledge_entries
  USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 100);

  -- 创建其他常用索引
  CREATE INDEX idx_knowledge_entries_category ON knowledge_entries(category);
  CREATE INDEX idx_knowledge_entries_created_at ON knowledge_entries(created_at);
  ```

- 回滚文件: `apps/api/migrations/000003_create_knowledge_entries.down.sql`
  ```sql
  DROP TABLE IF EXISTS knowledge_entries;
  ```

#### 1.2 更新 Go 模型
- 文件: `apps/api/internal/model/knowledge_entry.go`
- 定义 GORM 模型 struct

#### 1.3 更新 Go Repository
- 文件: `apps/api/internal/repository/knowledge_repository.go`
- 实现基本的 CRUD 操作

**验证**: 运行数据库迁移，确认表和索引创建成功

---

### 阶段 2：Python AI 服务基础架构

**目标**: 搭建 Python RAG 模块的基础结构

#### 2.1 创建 RAG 模块结构
```
apps/ai-service/src/rag/
├── __init__.py
├── embedding.py      # Embedding 生成
├── retriever.py      # 语义检索
├── reranker.py       # 重排序
└── knowledge_base.py # 知识库管理
```

#### 2.2 配置管理
- 文件: `apps/ai-service/src/config.py`
- 添加 embedding 模型配置:
  ```python
  EMBEDDING_MODEL: str = "text-embedding-3-small"  # 或 m3e-base
  EMBEDDING_DIMENSION: int = 1536
  TOP_K: int = 10
  TOP_N: int = 3  # reranker 后返回的数量
  ```

#### 2.3 依赖管理
- 更新 `apps/ai-service/pyproject.toml`
- 添加依赖:
  ```toml
  dependencies = [
      "openai>=1.0.0",           # OpenAI embedding API
      "sentence-transformers",   # 本地 embedding 模型（备选）
      "numpy",
      "sqlalchemy",
      "asyncpg",
      "pgvector",
  ]
  ```

**验证**: 依赖安装成功，模块可导入

---

### 阶段 3：Embedding 生成模块

**目标**: 实现文本转向量的功能

#### 3.1 实现 embedding.py
- 文件: `apps/ai-service/src/rag/embedding.py`
- 功能:
  - 支持 OpenAI API (text-embedding-3-small)
  - 支持本地模型 (m3e-base) 作为备选
  - 批量 embedding 支持
  - 错误处理和重试机制

```python
class EmbeddingGenerator:
    async def generate(self, text: str) -> list[float]:
        """生成单个文本的 embedding"""
        pass

    async def generate_batch(self, texts: list[str]) -> list[list[float]]:
        """批量生成 embedding"""
        pass
```

#### 3.2 单元测试
- 文件: `apps/ai-service/tests/unit/test_embedding.py`
- 测试用例:
  - 单文本 embedding 生成
  - 批量 embedding 生成
  - 维度验证 (1536)
  - 错误处理

**验证**: 运行单元测试，确认 embedding 生成正确

---

### 阶段 4：语义检索模块

**目标**: 实现基于 pgvector 的语义检索

#### 4.1 实现 retriever.py
- 文件: `apps/ai-service/src/rag/retriever.py`
- 功能:
  - 连接 PostgreSQL + pgvector
  - 余弦相似度搜索
  - 返回 top-k 结果（含相似度分数）

```python
class SemanticRetriever:
    async def search(self, query: str, top_k: int = 10) -> list[dict]:
        """
        语义检索
        返回: [{id, category, title, content, similarity, source_video, source_timestamp}]
        """
        pass
```

#### 4.2 单元测试
- 文件: `apps/ai-service/tests/unit/test_retriever.py`
- 测试用例:
  - 相似度计算正确性
  - top-k 返回数量
  - 空结果处理

**验证**: 插入测试数据，验证检索结果正确

---

### 阶段 5：Reranker 模块

**目标**: 实现检索结果的二次排序

#### 5.1 实现 reranker.py
- 文件: `apps/ai-service/src/rag/reranker.py`
- 功能:
  - 基于交叉编码器的重排序
  - 或基于 LLM 的相关性判断
  - 返回 top-n 结果

```python
class Reranker:
    async def rerank(self, query: str, candidates: list[dict], top_n: int = 3) -> list[dict]:
        """
        重排序
        输入: 查询 + 候选列表
        输出: 重排序后的 top-n 结果
        """
        pass
```

#### 5.2 单元测试
- 文件: `apps/ai-service/tests/unit/test_reranker.py`

**验证**: 验证重排序效果

---

### 阶段 6：知识库管理模块

**目标**: 实现知识条目的入库和管理

#### 6.1 实现 knowledge_base.py
- 文件: `apps/ai-service/src/rag/knowledge_base.py`
- 功能:
  - 知识条目入库（自动生成 embedding）
  - 批量入库
  - 按类别查询
  - 删除和更新

```python
class KnowledgeBase:
    async def add_entry(self, entry: dict) -> int:
        """添加知识条目，自动生成 embedding"""
        pass

    async def search(self, query: str, top_k: int = 10, top_n: int = 3) -> list[dict]:
        """端到端检索：embedding → 检索 → rerank"""
        pass
```

#### 6.2 单元测试
- 文件: `apps/ai-service/tests/unit/test_knowledge_base.py`

**验证**: 端到端测试入库和检索流程

---

### 阶段 7：Python API 路由

**目标**: 提供 REST API 接口

#### 7.1 创建知识库路由
- 文件: `apps/ai-service/src/api/routes/knowledge.py`
- 端点:
  ```
  POST   /api/knowledge/entries      # 添加知识条目
  POST   /api/knowledge/search       # 语义检索
  GET    /api/knowledge/entries/{id}  # 获取单个条目
  DELETE /api/knowledge/entries/{id}  # 删除条目
  ```

#### 7.2 请求/响应模型
- 文件: `apps/ai-service/src/models/knowledge.py`
- Pydantic 模型定义

#### 7.3 集成测试
- 文件: `apps/ai-service/tests/integration/test_knowledge_api.py`

**验证**: API 端点可正常调用

---

### 阶段 8：Go 后端代理

**目标**: Go 后端提供统一的知识库搜索代理端点

#### 8.1 创建 Handler
- 文件: `apps/api/internal/handler/knowledge.go`
- 功能:
  - 代理转发到 Python AI 服务
  - 统一鉴权中间件
  - 错误处理

#### 8.2 创建 Service
- 文件: `apps/api/internal/service/knowledge_service.go`
- 功能:
  - 调用 Python 服务的 HTTP 客户端
  - 响应缓存（可选）

#### 8.3 路由注册
- 更新 `apps/api/internal/handler/router.go`
- 添加知识库相关路由

#### 8.4 单元测试
- 文件: `apps/api/internal/handler/knowledge_test.go`

**验证**: 通过 Go 后端访问知识库 API

---

### 阶段 9：端到端测试

**目标**: 验证完整流程

#### 9.1 准备测试数据
- 创建 10 条测试知识条目
- 涵盖不同类别（体态问题、运动建议、饮食建议等）

#### 9.2 执行端到端测试
1. 通过 API 写入 10 条测试知识
2. 执行语义搜索查询
3. 验证返回最相关的 3 条结果
4. 验证相似度分数合理

#### 9.3 编写自动化测试脚本
- 文件: `scripts/test-rag-pipeline.sh`

**验证**: 端到端测试通过

---

## 📁 文件清单

### 数据库
- `apps/api/migrations/000003_create_knowledge_entries.up.sql`
- `apps/api/migrations/000003_create_knowledge_entries.down.sql`
- `apps/api/internal/model/knowledge_entry.go`
- `apps/api/internal/repository/knowledge_repository.go`

### Python AI 服务
- `apps/ai-service/src/rag/__init__.py`
- `apps/ai-service/src/rag/embedding.py`
- `apps/ai-service/src/rag/retriever.py`
- `apps/ai-service/src/rag/reranker.py`
- `apps/ai-service/src/rag/knowledge_base.py`
- `apps/ai-service/src/api/routes/knowledge.py`
- `apps/ai-service/src/models/knowledge.py`
- `apps/ai-service/src/config.py` (更新)

### Go 后端
- `apps/api/internal/handler/knowledge.go`
- `apps/api/internal/service/knowledge_service.go`
- `apps/api/internal/handler/router.go` (更新)

### 测试
- `apps/ai-service/tests/unit/test_embedding.py`
- `apps/ai-service/tests/unit/test_retriever.py`
- `apps/ai-service/tests/unit/test_reranker.py`
- `apps/ai-service/tests/unit/test_knowledge_base.py`
- `apps/ai-service/tests/integration/test_knowledge_api.py`
- `apps/api/internal/handler/knowledge_test.go`
- `scripts/test-rag-pipeline.sh`

---

## 🔧 技术选型

### Embedding 模型
- **首选**: OpenAI `text-embedding-3-small` (1536 维)
  - 优点: 质量高、API 稳定
  - 缺点: 需要 API key、有成本
- **备选**: `m3e-base` (本地模型)
  - 优点: 无 API 成本、隐私性好
  - 缺点: 需要 GPU、部署复杂

**决策**: 先用 OpenAI API 实现，后续可切换到本地模型

### Reranker 方案
- **方案 A**: 交叉编码器 (cross-encoder)
  - 使用 `sentence-transformers` 库
  - 精度高但速度较慢
- **方案 B**: LLM 相关性判断
  - 使用 GPT-4 或 Claude 判断相关性
  - 更灵活但成本更高

**决策**: 先用交叉编码器，必要时切换到 LLM

### 数据库
- PostgreSQL 16 + pgvector 扩展
- IVFFlat 索引（适合中等规模数据）

---

## ⚠️ 风险与应对

| 风险 | 影响 | 应对措施 |
|------|------|----------|
| OpenAI API 限流 | Embedding 生成变慢 | 实现重试机制、批量请求 |
| pgvector 性能瓶颈 | 检索变慢 | 优化索引参数、考虑 HNSW 索引 |
| Embedding 模型切换 | 需要重新入库 | 设计模型版本管理、支持批量重嵌入 |
| 测试数据不足 | 验证不充分 | 准备多样化的测试数据集 |

---

## 📅 时间估算

| 阶段 | 预计时间 | 依赖 |
|------|----------|------|
| 阶段 1: 数据库层 | 2 小时 | 无 |
| 阶段 2: Python 基础架构 | 1 小时 | 无 |
| 阶段 3: Embedding 模块 | 2 小时 | 阶段 2 |
| 阶段 4: 语义检索模块 | 3 小时 | 阶段 1, 3 |
| 阶段 5: Reranker 模块 | 2 小时 | 阶段 4 |
| 阶段 6: 知识库管理 | 2 小时 | 阶段 3, 4, 5 |
| 阶段 7: Python API | 2 小时 | 阶段 6 |
| 阶段 8: Go 代理 | 2 小时 | 阶段 7 |
| 阶段 9: 端到端测试 | 2 小时 | 阶段 8 |
| **总计** | **18 小时** | |

---

## ✅ 完成检查清单

- [ ] 数据库迁移成功执行
- [ ] pgvector 扩展和索引创建成功
- [ ] Embedding 生成接口工作正常
- [ ] 语义检索返回正确结果
- [ ] Reranker 有效筛选 top-3
- [ ] 知识入库 API 功能完整
- [ ] Go 代理端点正常工作
- [ ] 端到端测试通过
- [ ] 单元测试覆盖率 > 80%
- [ ] 代码通过 lint 和 typecheck
- [ ] 文档更新完成

---

## 📝 备注

- 本计划基于 issue #3 的验收标准制定
- 实施过程中可能根据实际情况调整
- 遇到问题及时记录和沟通
- 完成后将本文件移至 `docs/plan/archive/`

---

**最后更新**: 2026-06-21  
**作者**: AI Assistant