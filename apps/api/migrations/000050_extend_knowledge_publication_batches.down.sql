DROP INDEX IF EXISTS idx_knowledge_publications_rollback_of;

ALTER TABLE knowledge_publications
    DROP COLUMN IF EXISTS summary,
    DROP COLUMN IF EXISTS unit_count,
    DROP COLUMN IF EXISTS rollback_of;
