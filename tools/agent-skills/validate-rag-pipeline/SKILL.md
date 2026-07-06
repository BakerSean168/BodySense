# validate-rag-pipeline

验证 BodySense 的 RAG（检索增强生成）知识库管道的质量和效果。

## 触发条件

- 修改了 `apps/ai-service/src/rag/` 目录下的任何文件
- 修改了知识库数据或 Embedding 模型配置
- 新增了知识库条目
- 用户主动要求验证 RAG 效果

## 执行步骤

1. **检查知识库数据完整性**
   ```bash
   cd apps/ai-service
   uv run python -c "
   from src.rag.knowledge_library import get_knowledge_library
   lib = get_knowledge_library()
   print('KnowledgeLibrary loaded successfully')
   print(f'SearchResult fields: title, summary, body_markdown, source_title, tags, clips')
   "
   ```

2. **运行 RAG 相关单元测试**
   ```bash
   cd apps/ai-service
   uv run pytest tests/unit/test_embedding.py tests/unit/test_golden_cases.py -v
   ```

3. **验证搜索功能**
   ```bash
   cd apps/ai-service
   uv run python -c "
   import asyncio
   from src.rag.knowledge_library import get_knowledge_library

   async def test_search():
       lib = get_knowledge_library()
       results = await lib.search('肩颈酸胀', top_k=3)
       for r in results:
           print(f'  [{r.problem_slug}] {r.title} (score: {r.similarity:.3f})')
       print(f'Total results: {len(results)}')

   asyncio.run(test_search())
   "
   ```

4. **检查 Embedding 模型一致性**
   ```bash
   cd apps/ai-service
   uv run python -c "
   from src.rag.embedding import get_embedding_generator
   gen = get_embedding_generator()
   print(f'EmbeddingGenerator loaded')
   "
   ```

## 成功标准

- `KnowledgeLibrary` 可正常加载
- 搜索返回结构化结果（含 title、summary、body_markdown、source 等字段）
- Embedding 模型版本与配置一致
- 相关单元测试全部通过

## 失败处理

- 如果搜索无结果，检查知识库数据和 pgvector 扩展是否正常
- 如果 Embedding 模型不一致，需要全量重建 Embedding
- 如果单元测试失败，检查 `knowledge_library.py` 的 schema 变更
