-- Complete knowledge publication batch metadata needed for explicit rollback governance.
ALTER TABLE knowledge_publications
    ADD COLUMN IF NOT EXISTS rollback_of UUID REFERENCES knowledge_publications(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS unit_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS summary TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_knowledge_publications_rollback_of
    ON knowledge_publications(rollback_of);
