-- Allow knowledge_publications to represent a publication batch.
-- Units link to a publication through knowledge_units.publication_id.
ALTER TABLE knowledge_publications
    ALTER COLUMN knowledge_unit_id DROP NOT NULL;

ALTER TABLE knowledge_publications
    ADD COLUMN IF NOT EXISTS publication_batch_key VARCHAR(200);

CREATE INDEX IF NOT EXISTS idx_knowledge_publications_batch_key
    ON knowledge_publications(publication_batch_key);
