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
-- 注意：IVFFlat 索引需要在有一定数据量后才能有效工作
-- 初始时可以先不创建索引，等数据量达到 1000+ 条后再创建
-- CREATE INDEX idx_knowledge_entries_embedding
-- ON knowledge_entries
-- USING ivfflat (embedding vector_cosine_ops)
-- WITH (lists = 100);

-- 创建其他常用索引
CREATE INDEX idx_knowledge_entries_category ON knowledge_entries(category);
CREATE INDEX idx_knowledge_entries_created_at ON knowledge_entries(created_at);

-- 创建更新时间触发器
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_knowledge_entries_updated_at
    BEFORE UPDATE ON knowledge_entries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
