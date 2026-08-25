DROP INDEX IF EXISTS idx_knowledge_sources_content_hash;
DROP INDEX IF EXISTS idx_knowledge_sources_registered_by;

ALTER TABLE knowledge_sources
    DROP COLUMN IF EXISTS registered_at,
    DROP COLUMN IF EXISTS registered_by,
    DROP COLUMN IF EXISTS provenance,
    DROP COLUMN IF EXISTS source_version,
    DROP COLUMN IF EXISTS canonical_url,
    DROP COLUMN IF EXISTS content_hash;

DROP INDEX IF EXISTS idx_users_role;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;
ALTER TABLE users DROP COLUMN IF EXISTS role;
