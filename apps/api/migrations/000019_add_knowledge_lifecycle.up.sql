-- Knowledge publications: tracks published versions of knowledge units
CREATE TABLE knowledge_publications (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    knowledge_unit_id BIGINT NOT NULL REFERENCES knowledge_units(id) ON DELETE CASCADE,
    publication_key VARCHAR(200) NOT NULL UNIQUE,
    title VARCHAR(500) NOT NULL DEFAULT '',
    published_version INT NOT NULL DEFAULT 1,
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_by TEXT,
    created_by TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'published',
    metadata JSONB NOT NULL DEFAULT '{}',
    UNIQUE (knowledge_unit_id, published_version)
);

CREATE INDEX idx_knowledge_publications_unit_id ON knowledge_publications(knowledge_unit_id);
CREATE INDEX idx_knowledge_publications_status ON knowledge_publications(status);

-- Add lifecycle metadata columns to knowledge_units
-- All columns have passive defaults that preserve existing behavior
ALTER TABLE knowledge_units
    ADD COLUMN quality_score REAL DEFAULT 0.0,
    ADD COLUMN content_hash TEXT,
    ADD COLUMN published_version INT,
    ADD COLUMN lifecycle_metadata JSONB NOT NULL DEFAULT '{}';

-- Add license_status to knowledge_sources
ALTER TABLE knowledge_sources
    ADD COLUMN license_status VARCHAR(30) NOT NULL DEFAULT 'unknown';

CREATE INDEX idx_knowledge_units_quality_score ON knowledge_units(quality_score);
CREATE INDEX idx_knowledge_sources_license_status ON knowledge_sources(license_status);
