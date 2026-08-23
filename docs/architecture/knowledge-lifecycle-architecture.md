# Knowledge Lifecycle 架构设计

**文档版本**：v1.0
**更新日期**：2026-08-23
**状态**：核心生命周期与首轮 Published Governance 已实现
**适用范围**：视频知识入库、RAG、知识精修、向量索引、引用、知识质量管理

---

## Implementation Status

**当前状态**：核心生命周期与首轮 Published Retrieval / Citation / Grounding Governance 已实现（管理 UI 与自动生产观测仍未实现）

| 模块 | 状态 | 说明 |
|---|---|---|
| lifecycle_status 列 | ✅ 已完成 | knowledge_units 表添加 lifecycle_status 字段。Phase 07a 完成。 |
| quality_score / content_hash 列 | ✅ 已完成 | knowledge_units 表扩展。Phase 07a 完成。 |
| license_status 列 | ✅ 已完成 | knowledge_sources 表扩展。Phase 07a 完成。 |
| knowledge_publications 表 | ✅ 已完成 | 发布批次管理表。Phase 07a 完成。 |
| KnowledgePublication Repository | ✅ 已完成 | Go 侧 repository 实现。 |
| KnowledgeSourceRegistry | 未实现 | 知识源注册和管理。 |
| 发布批次工作流 | ✅ 核心已完成 | Go `KnowledgePublicationService` 事务化执行 reviewed → published 与显式 rollback；当前通过 operator CLI 驱动，管理 UI 未实现。 |
| 质量阈值门控 | ✅ 核心已完成 | 发布前强制 review/lifecycle、quality ≥ 0.90、content hash、external support、claim review、license 与 provenance gate。 |
| 检索质量评估 | ✅ 首轮完成 | unpublished retrieval + published positive/negative retrieval、citation identity/provenance、claim grounding pilot 已建立。 |
| Publication observations / gate | ✅ 首轮完成 | migration 51 + Go observation service/CLI；按 immutable publication identity 汇总并输出 continue / hold / rollback。 |

**相关 Phase**：07a → 归档于 `docs/plan/archive/implementation/`

---

## 1. 背景

BodySense 的 RAG 知识来源主要来自体态、康复、训练相关视频。现有 Python 侧已经有较完整的入库能力：

- ASR Provider：whisper.cpp、FunASR、API provider。
- `VideoIngestionPipeline`：抽音频、ASR、切分、clip 导出、generated pack。
- `AICurator`：AI 精修知识单元。
- `KnowledgeLibrary`：写入 PostgreSQL / pgvector，并支持检索。
- `curated_pack.json` 和 `generated_pack.json` 两类产物。

当前主要问题不是“没有管道”，而是知识从原始视频到线上检索结果之间，还缺一个正式生命周期：

```txt
source -> generated -> curated -> reviewed -> embedded -> published -> deprecated
```

如果没有生命周期治理，会出现：

- generated 内容未人工确认就被检索使用。
- ASR 噪声进入线上回答。
- 知识单元缺少来源和证据片段。
- embedding 与原文版本不一致。
- 无法回滚某次知识发布。
- citation 指向不稳定。
- 质量差的数据影响工具调用和训练建议。

本设计目标是建立 **Knowledge Lifecycle**，把知识库从“能检索”升级为“可发布、可审计、可回滚、可评估”的工程系统。

---

## 2. 设计目标

1. **知识有状态**
   每条知识单元都有 lifecycle status。

2. **线上只用 published 知识**
   RAG 默认只检索已发布版本。

3. **来源可追溯**
   视频知识必须能追溯 source/timestamp/evidence excerpt；文本知识必须能追溯 repository/Git commit/path/heading/line range。

4. **发布可回滚**
   知识发布按 batch/version 管理。

5. **质量可度量**
   知识单元有质量评分、审核状态和问题标记。

6. **与 Tool Runtime 对齐**
   `search_knowledge` 工具只返回满足质量门槛的结果。

---

## 3. 生命周期状态

```txt
raw
  原始视频或文本资源已登记

transcribed
  ASR 完成

generated
  自动切分生成知识底稿

curated
  AI 或人工完成精修

reviewed
  人工审核通过

embedded
  已生成 embedding

published
  已进入线上检索范围

deprecated
  不再推荐使用

rejected
  不允许进入知识库
```

---

## 4. 总体架构

```txt
Knowledge Admin / CLI
  - register source
  - start ingestion job
  - review curated units
  - publish / rollback
        |
        v
Go API
  - KnowledgeLifecycle Runtime
  - job orchestration
  - publish state
  - admin audit
        |
        v
Python AI Service
  - VideoIngestionPipeline
  - ASR
  - Splitter
  - AICurator
  - EmbeddingGenerator
  - KnowledgeLibrary
        |
        v
PostgreSQL + pgvector + file artifacts
```

---

## 5. Module 设计

### 5.0 Online KnowledgeLibrary async boundary

当前在线 Python RAG 路径不允许在 `async def` 中隐藏同步 PostgreSQL 或本地 transformer 推理：

- FastAPI lifespan 调用 `initialize_knowledge_library()` / `shutdown_knowledge_library()`；
- `KnowledgeLibrary` 持有一个有界 `AsyncConnectionPool`（默认 min=1 / max=8，可配置），启动连接等待上限为 5 秒；
- pgvector 通过 `register_vector_async` 在 async connection configure hook 注册；
- `search`、`list_sources`、`stats` 使用 async connection/cursor；
- `ingest_generated_pack` 的 source/segments/units/clips 仍在**同一个 async transaction** 内原子提交/回滚；
- 不允许 lazy singleton accessor 自己做网络 I/O；测试可显式注入非 owned pool；
- local `SentenceTransformer` 初始化和 `encode()` 用 `asyncio.to_thread`，并受 `LOCAL_EMBEDDING_MAX_CONCURRENCY` semaphore 限制；远程 embedding 继续使用原生 async API。

这些规则属于 runtime resource ownership；它们不改变 knowledge publication/review authority。

### 5.1 KnowledgeSourceRegistry

登记原始来源。

字段：

```txt
source_id
source_type
title
author
problem_slug
original_uri
license_status
language
status
```

职责：

- 管理来源元数据。
- 避免重复入库。
- 记录授权状态。

### 5.2 KnowledgeIngestionJob

由 Job Runtime 管理。

Job type：

```txt
knowledge.ingest_video
knowledge.curate_pack
knowledge.embed_pack
knowledge.publish_batch
```

对应 Python pipeline：

```txt
VideoIngestionPipeline.ingest
AICurator.refine_pack
KnowledgeLibrary.ingest_generated_pack
EmbeddingGenerator.generate_batch
```

### 5.3 KnowledgePack

KnowledgePack 是文件产物层。

```txt
generated_pack.json
curated_pack.json
reviewed_pack.json
```

原则：

- 文件产物保留完整中间过程。
- 数据库保存可查询状态和线上索引。
- pack 里必须有 schema version。

### 5.4 KnowledgePublication

发布批次 Module。

```txt
publication_id
version
source_ids
unit_ids
status
published_at
rollback_of
```

职责：

- 将 reviewed/embedded 知识切换到 published。
- 支持回滚到上一版本。
- 记录发布人和发布时间。

---

## 6. 数据库设计

### 6.1 knowledge_sources

```sql
CREATE TABLE knowledge_sources (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    source_key      TEXT NOT NULL UNIQUE,
    source_type     VARCHAR(40) NOT NULL,
    title           TEXT NOT NULL,
    author          TEXT,
    problem_slug    TEXT NOT NULL,
    original_uri    TEXT,
    artifact_dir    TEXT,
    license_status  VARCHAR(40) NOT NULL DEFAULT 'unknown',
    status          VARCHAR(40) NOT NULL DEFAULT 'raw',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.2 knowledge_units

当前已有 `knowledge_entries`，建议演进或新增状态字段：

```sql
ALTER TABLE knowledge_entries
ADD COLUMN IF NOT EXISTS source_id UUID REFERENCES knowledge_sources(id),
ADD COLUMN IF NOT EXISTS unit_key TEXT,
ADD COLUMN IF NOT EXISTS lifecycle_status VARCHAR(40) NOT NULL DEFAULT 'generated',
ADD COLUMN IF NOT EXISTS quality_score NUMERIC,
ADD COLUMN IF NOT EXISTS review_status VARCHAR(40) NOT NULL DEFAULT 'unreviewed',
ADD COLUMN IF NOT EXISTS evidence JSONB NOT NULL DEFAULT '{}',
ADD COLUMN IF NOT EXISTS publication_id UUID,
ADD COLUMN IF NOT EXISTS content_hash TEXT;
```

### 6.3 knowledge_publications

```sql
CREATE TABLE knowledge_publications (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    version         TEXT NOT NULL UNIQUE,
    status          VARCHAR(40) NOT NULL,
    summary         TEXT,
    unit_count      INT NOT NULL DEFAULT 0,
    created_by      UUID REFERENCES users(id),
    published_at    TIMESTAMPTZ,
    rollback_of     UUID REFERENCES knowledge_publications(id),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 7. 质量门槛

线上 published 知识必须满足：

```txt
lifecycle_status = published
publication_id exists
published_version exists
review_status in (reviewed, approved, curated)
quality_score >= threshold
content_hash matches current body
evidence excerpt exists
source provenance exists (video timestamp or Markdown locator)
source/external-evidence license is not rejected
claim_admissibility.publication_eligible = true (for governed Thought Forest claims)
claim_review.decision = approved (for governed Thought Forest claims)
```

`search_knowledge` 工具默认过滤：

```sql
WHERE lifecycle_status = 'published'
  AND publication_id IS NOT NULL
  AND published_version IS NOT NULL
  AND review_status IN ('reviewed', 'approved', 'curated')
```

开发环境可以开启 `include_generated=true`，但必须在 context_trace 中标记。

---

## 8. 引用设计

每个 SearchResult 应返回：

```json
{
  "unit_id": "...",
  "title": "...",
  "summary": "...",
  "source": {
    "title": "...",
    "author": "...",
    "timestamp": "03:12-04:20",
    "clip_url": "...",
    "source_key": "..."
  },
  "evidence_excerpt": "...",
  "quality_score": 0.91
}
```

前端 citation 不直接依赖 RAG chunk 文本，而依赖稳定 `unit_id` 和 source metadata。

---

## 9. 发布流程

```txt
1. register source
2. run knowledge.ingest_video
3. generated_pack.json
4. run knowledge.curate_pack
5. curated_pack.json
6. human review
7. run knowledge.embed_pack
8. create publication
9. publish
10. search_knowledge can retrieve
```

Mermaid：

```mermaid
sequenceDiagram
    participant A as Admin
    participant GO as Go API
    participant JOB as Job Runtime
    participant PY as Python AI
    participant DB as PostgreSQL

    A->>GO: Register source
    GO->>DB: insert knowledge_sources
    A->>GO: Start ingestion
    GO->>JOB: enqueue knowledge.ingest_video
    JOB->>PY: run VideoIngestionPipeline
    PY-->>JOB: generated_pack artifact
    JOB->>DB: update source generated
    A->>GO: Review and publish
    GO->>DB: mark units published
```

---

## 10. 与 AI Output Governance 的关系

知识精修输出类型：

```txt
knowledge.curated_unit
```

必须经过：

- schema validation。
- evidence excerpt 检查。
- toxic / noisy ASR 清洗检查。
- source timestamp 检查。
- claim 是否能被 transcript 支持。

只有治理通过的 unit 才能进入 reviewed / embedded。

---

## 11. 测试和评估

### 11.1 Pipeline 测试

```txt
apps/ai-service/tests/unit/test_video_pipeline.py
apps/ai-service/tests/unit/test_curated_source.py
apps/ai-service/tests/unit/test_knowledge_library.py
```

### 11.2 检索质量评估

维护 query set：

```txt
docs/knowledges/eval/queries.jsonl
```

字段：

```json
{
  "query": "肩关节弹响怎么自测",
  "expected_problem_slug": "shoulder-joint",
  "expected_unit_types": ["self_test"],
  "must_include_terms": ["肩胛", "锁骨"]
}
```

指标：

```txt
top1 relevance
top3 recall
citation completeness
knowledge_gap accuracy
```

### 11.3 Published Knowledge regression

Stage 6 增加 `bodysense.published-knowledge-eval.v1`，只走默认 published retrieval，
不使用 `include_unpublished`。首批 pilot 同时包含：

```txt
positive query
→ exact published unit
→ exact publication id / key / version
→ claim review identity
→ Markdown provenance
→ expected claim terms grounded

negative query
→ no result
→ no citation
```

当前 hashing embedding 是确定性本地开发检索，不是经过校准的 semantic model。Stage 6 实测中，
完全无关的 `PostgreSQL 索引` query cosine 一度高于 `什么是疼痛`，因此禁止用一个拍脑袋的
固定 cosine threshold 作为安全边界。当前策略是：

```txt
source_type = thought_forest_note
AND lifecycle = published
AND embedding_provider = hashing
→ semantic candidate 必须再满足 meaningful lexical anchor
→ 才能成为 citation candidate
```

此规则只约束 hashing + published Thought Forest；视频知识与未来经校准的 semantic embedding
保持各自策略，避免把临时本地 embedding 的行为误当成通用检索定律。

### 11.4 Citation trust boundary

`source.citation.added` 对 Thought Forest published citation 现在必须包含：

```txt
unit_key
source_key / source_type
lifecycle_status = published
publication_id
publication_key / publication_batch_key
published_version
claim_id / claim_review_id
source_locator(repository/git/path/line range)
```

Python citation emitter 保留这些字段，Go AIClient 在协议 trust boundary 再验证一次。Legacy video
citation 继续兼容旧 contract。

### 11.5 Publication observations and deny-first gate

`knowledge_publication_observations` 按 immutable `publication_id` 记录 retrieval / citation / grounding /
identity / provenance 结果。当前 gate：

```txt
identity mismatch
OR provenance invalid
OR citation invalid
OR grounding rejected
OR negative query returned published result
→ rollback

retrieval miss
OR grounding degraded
OR execution error
→ hold

clean window + >= 5 observations + positive hit exists
→ continue
```

Gate 只给 operator recommendation，不自动执行 rollback。真正状态变更仍由 Go
`KnowledgePublicationService.RollbackBatch` 显式完成。当前 Stage 6 vertical slice 使用
`predeploy_eval` observation；表和 service 已保留 `observation_kind`，自动生产 runtime observation
仍需要未来把最终回答 claim ↔ evidence attribution 契约接入，不能仅凭“出现 citation”假装完成 grounding。

---

## 12. 分阶段落地

### Phase 1：状态字段和过滤

- 给 knowledge entries 增加 lifecycle / review / quality 字段。
- `search_knowledge` 默认只查 published。

### Phase 2：Source Registry

- 新增 `knowledge_sources`。
- 视频 pipeline 输出 source_id。

### Phase 3：Publication

- 新增 `knowledge_publications`。
- 支持发布批次。

### Phase 4：Review 工具

- CLI 或简单 admin 页面审核 generated/curated units。
- 支持 reject / approve / edit。

### Phase 5：检索评估

- 建立 query set。
- CI 或手动命令跑检索质量。

---

## 13. 成功标准

落地后应满足：

1. generated 知识不会误入线上检索。
2. 每条 citation 能追溯到 source、timestamp、evidence。
3. 知识发布可以按批次回滚。
4. RAG 检索质量可以用 query set 评估。
5. `search_knowledge` 工具只返回满足质量门槛的结果。
6. 视频入库从脚本能力升级为可管理生命周期。

