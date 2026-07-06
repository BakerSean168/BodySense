DROP INDEX IF EXISTS idx_knowledge_publications_batch_key;

ALTER TABLE knowledge_publications
    DROP COLUMN IF EXISTS publication_batch_key;

ALTER TABLE knowledge_publications
    ALTER COLUMN knowledge_unit_id SET NOT NULL;
