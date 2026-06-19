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
   from src.rag.knowledge_base import KnowledgeBase
   kb = KnowledgeBase()
   stats = kb.get_stats()
   print(f'总条目数: {stats[\"total_entries\"]}')
   print(f'有 Embedding 的条目: {stats[\"with_embedding\"]}')
   print(f'缺失 Embedding: {stats[\"missing_embedding\"]}')
   "
   ```

2. **运行召回率测试**
   ```bash
   uv run pytest tests/rag/test_retrieval.py -v
   ```

3. **运行答案质量测试**（需要 LLM API key）
   ```bash
   uv run pytest tests/rag/test_answer_quality.py -v
   ```

4. **检查 Embedding 模型一致性**
   ```bash
   uv run python -c "
   from src.rag.embeddings import get_embedding_model
   model = get_embedding_model()
   print(f'模型: {model.name}')
   print(f'维度: {model.dimensions}')
   "
   ```

## 成功标准

- 所有知识库条目都有对应的 Embedding 向量
- 召回率测试通过率 ≥ 80%
- 答案质量测试（如有配置 LLM）通过率 ≥ 70%
- Embedding 模型版本与配置一致

## 失败处理

- 如果有缺失 Embedding 的条目，运行 `uv run python scripts/rebuild_embeddings.py`
- 如果召回率低于阈值，检查知识库条目质量和分块策略
- 如果 Embedding 模型不一致，需要全量重建 Embedding
