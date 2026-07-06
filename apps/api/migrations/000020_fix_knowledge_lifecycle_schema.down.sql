-- Reverse 000020 fixes

-- Remove trigger
DROP TRIGGER IF EXISTS update_knowledge_publications_updated_at ON knowledge_publications;

-- Remove created_at/updated_at from knowledge_publications
ALTER TABLE knowledge_publications
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS updated_at;

-- Revert status default back to 'published' (original 000019 value)
ALTER TABLE knowledge_publications
    ALTER COLUMN status SET DEFAULT 'published';

-- Remove publication_id FK and column from knowledge_units
ALTER TABLE knowledge_units
    DROP CONSTRAINT IF EXISTS fk_knowledge_units_publication_id,
    DROP COLUMN IF EXISTS publication_id;

-- Remove lifecycle_status from knowledge_units
ALTER TABLE knowledge_units
    DROP COLUMN IF EXISTS lifecycle_status;

-- Revert license_status width
ALTER TABLE knowledge_sources
    ALTER COLUMN license_status TYPE VARCHAR(30);
