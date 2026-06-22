-- 删除触发器
DROP TRIGGER IF EXISTS update_knowledge_entries_updated_at ON knowledge_entries;
DROP FUNCTION IF EXISTS update_updated_at_column();

-- 删除表
DROP TABLE IF EXISTS knowledge_entries;

-- 注意：不删除 vector 扩展，因为可能被其他表使用
-- 如果需要完全清理，可以执行: DROP EXTENSION IF EXISTS vector;
