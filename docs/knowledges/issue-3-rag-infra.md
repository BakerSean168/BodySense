# Issue #3: 知识库 RAG 基础设施

## 概述

实现知识库的 RAG（检索增强生成）基础设施，包括 pgvector 向量存储、embedding 生成、语义检索和重排序。

## 技术架构

### 1. 数据库层 (PostgreSQL + pgvector)

**表结构**: `knowledge_entries`

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | 主键 |
| category | VARCHAR(100) | 知识类别 |
| title | VARCHAR(500) | 标题 |
| content | TEXT | 内容 |
| embedding | VECTOR(1536) | 向量嵌入 |
| source_video | VARCHAR(500) | 来源视频 |
| source_timestamp | VARCHAR(50) | 视频时间戳 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

**索引**:
- IVFFlat 向量索引（余弦相似度）
- category 索引
- created_at 索引

### 2. Python AI 服务

**模块结构**:
```
src/rag/
├── __init__.py
├── embedding.py      # Embedding 生成
├── retriever.py      # 语义检索
├── reranker.py       # 重排序
└── knowledge_base.py # 知识库管理
```

#### Embedding 生成 (`embedding.py`)

- 使用 OpenAI `text-embedding-3-small` 模型
- 生成 1536 维向量
- 支持批量生成
- 内置重试机制

```python
class EmbeddingGenerator:
    async def generate(self, text: str) -> list[float]
    async def generate_batch(self, texts: list[str]) -> list[list[float]]
    async def generate_with_retry(self, text: str, max_retries: int = 3) -> list[float]
```

#### 语义检索 (`retriever.py`)

- 基于 pgvector 余弦相似度搜索
- 返回 top-k 结果（含相似度分数）
- 支持按类别过滤

```python
class SemanticRetriever:
    async def search(self, query: str, top_k: int = 10, category: str = None) -> list[RetrievalResult]
```

#### 重排序 (`reranker.py`)

- 使用 LLM（GPT-4o-mini）进行相关性判断
- 对检索结果进行二次排序
- 返回 top-n 结果

```python
class Reranker:
    async def rerank(self, query: str, candidates: list[RetrievalResult], top_n: int = 3) -> list[RetrievalResult]
```

#### 知识库管理 (`knowledge_base.py`)

- 端到端知识管理
- 自动生成 embedding
- 支持批量入库

```python
class KnowledgeBase:
    async def add_entry(self, entry: KnowledgeEntryData) -> int
    async def add_entries_batch(self, entries: list[KnowledgeEntryData]) -> list[int]
    async def search(self, query: str, top_k: int = 10, top_n: int = 3, category: str = None) -> list[RetrievalResult]
    async def get_entry(self, entry_id: int) -> Optional[RetrievalResult]
    async def delete_entry(self, entry_id: int) -> bool
    async def count(self) -> int
```

### 3. Go 后端代理

**文件**: `apps/api/internal/handler/knowledge.go`

提供统一的知识库 API 代理端点，转发请求到 Python AI 服务。

**API 端点**:
- `POST /api/knowledge/entries` - 添加知识条目
- `POST /api/knowledge/search` - 语义检索
- `GET /api/knowledge/entries/:id` - 获取单个条目
- `DELETE /api/knowledge/entries/:id` - 删除条目
- `GET /api/knowledge/stats` - 获取统计信息

### 4. API 路由

**Python**: `apps/ai-service/src/api/routes/knowledge.py`

- `POST /api/knowledge/entries` - 添加知识条目
- `POST /api/knowledge/search` - 语义检索
- `GET /api/knowledge/entries/{id}` - 获取单个条目
- `DELETE /api/knowledge/entries/{id}` - 删除条目
- `GET /api/knowledge/stats` - 获取统计信息

## 配置

### 环境变量

```bash
# OpenAI API (用于 embedding 和 reranking)
OPENAI_API_KEY=sk-your-openai-api-key-here

# Embedding 模型配置
EMBEDDING_MODEL=text-embedding-3-small
EMBEDDING_DIMENSIONS=1536

# AI 服务 URL (Go 后端代理用)
AI_SERVICE_URL=http://localhost:8100
```

### 依赖

**Python** (`pyproject.toml`):
```toml
dependencies = [
    "fastapi>=0.136.0",
    "uvicorn>=0.34.0",
    "langchain>=1.0.0",
    "langchain-openai>=0.3.0",
    "psycopg[binary]>=3.2.0",
    "pgvector>=0.4.0",
    "redis>=5.2.0",
    "python-dotenv>=1.1.0",
]
```

**Go** (`go.mod`):
```
github.com/pgvector/pgvector-go v0.4.0
```

## 使用示例

### 添加知识条目

```bash
curl -X POST http://localhost:8080/api/knowledge/entries \
  -H "Content-Type: application/json" \
  -d '{
    "category": "posture",
    "title": "上交叉综合征",
    "content": "上交叉综合征是一种常见的体态问题...",
    "source_video": "https://example.com/video1",
    "source_timestamp": "10:30"
  }'
```

### 语义检索

```bash
curl -X POST http://localhost:8080/api/knowledge/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "肩膀疼怎么办",
    "top_k": 10,
    "top_n": 3
  }'
```

## 测试

### 单元测试

```bash
cd apps/ai-service
uv run pytest tests/unit/
```

### 端到端测试

```bash
./scripts/test-rag-pipeline.sh
```

## 性能优化

### 向量索引

- 使用 IVFFlat 索引（适合中等规模数据）
- 参数 `lists = 100`（可根据数据量调整）
- 数据量达到 1000+ 条后创建索引效果更佳

### 批量操作

- embedding 生成支持批量请求
- 数据库写入使用事务批量插入

### 缓存策略

- 可在 Go 后端添加 Redis 缓存
- 缓存热门查询结果

## 扩展性

### 切换 Embedding 模型

支持本地模型（如 m3e-base）作为备选：

```python
# 使用本地模型
generator = EmbeddingGenerator(model="m3e-base", dimension=768)
```

### 切换 Reranker

支持交叉编码器作为备选：

```python
# 使用交叉编码器
reranker = CrossEncoderReranker(model="cross-encoder/ms-marco-MiniLM-L-6-v2")
```

## 注意事项

1. **API Key 安全**: OpenAI API Key 应通过环境变量注入，不要硬编码
2. **向量维度**: 必须与 embedding 模型输出维度一致（1536）
3. **索引创建**: IVFFlat 索引需要在有数据后创建，空表索引效果差
4. **错误处理**: 所有模块都包含重试机制和降级策略
5. **连接管理**: 数据库连接使用懒加载，支持自动重连

## 相关文件

- 数据库迁移: `apps/api/migrations/000003_create_knowledge_entries.up.sql`
- Go 模型: `apps/api/internal/model/knowledge_entry.go`
- Go Repository: `apps/api/internal/repository/knowledge_repository.go`
- Go Handler: `apps/api/internal/handler/knowledge.go`
- Python RAG: `apps/ai-service/src/rag/`
- Python API: `apps/ai-service/src/api/routes/knowledge.py`
- 测试: `apps/ai-service/tests/unit/test_*.py`
- 端到端测试: `scripts/test-rag-pipeline.sh`

---

**最后更新**: 2026-06-21
**作者**: AI Assistant
