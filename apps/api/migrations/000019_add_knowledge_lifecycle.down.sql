ALTER TABLE knowledge_units
    DROP COLUMN IF EXISTS quality_score,
    DROP COLUMN IF EXISTS content_hash,
    DROP COLUMN IF EXISTS published_version,
    DROP COLUMN IF EXISTS lifecycle_metadata;

ALTER TABLE knowledge_sources
    DROP COLUMN IF EXISTS license_status;

DROP TABLE IF EXISTS knowledge_publications;
