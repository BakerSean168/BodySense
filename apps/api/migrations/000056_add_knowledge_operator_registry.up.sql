-- Bound global Knowledge administration to an explicit user role and make
-- knowledge_sources a governed registry before ingestion.

ALTER TABLE users
    ADD COLUMN role VARCHAR(30) NOT NULL DEFAULT 'member';

ALTER TABLE users
    ADD CONSTRAINT chk_users_role
    CHECK (role IN ('member', 'operator'));

CREATE INDEX idx_users_role ON users(role);

ALTER TABLE knowledge_sources
    ADD COLUMN content_hash TEXT,
    ADD COLUMN canonical_url TEXT,
    ADD COLUMN source_version VARCHAR(100) NOT NULL DEFAULT 'v1',
    ADD COLUMN provenance JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN registered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN registered_at TIMESTAMPTZ;

CREATE INDEX idx_knowledge_sources_registered_by ON knowledge_sources(registered_by);
CREATE INDEX idx_knowledge_sources_content_hash ON knowledge_sources(content_hash);
