CREATE TABLE IF NOT EXISTS knowledge_sources (
    id BIGSERIAL PRIMARY KEY,
    source_key VARCHAR(200) NOT NULL UNIQUE,
    source_type VARCHAR(50) NOT NULL,
    title VARCHAR(500) NOT NULL,
    author VARCHAR(255) NOT NULL,
    problem_slug VARCHAR(100) NOT NULL,
    problem_display_name VARCHAR(255) NOT NULL,
    original_file_path TEXT NOT NULL,
    language VARCHAR(20) NOT NULL DEFAULT 'zh',
    duration_sec DOUBLE PRECISION,
    transcript_provider VARCHAR(100),
    transcript_model VARCHAR(100),
    transcript_file_path TEXT,
    ingest_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS knowledge_segments (
    id BIGSERIAL PRIMARY KEY,
    source_id BIGINT NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    segment_index INTEGER NOT NULL,
    start_sec DOUBLE PRECISION NOT NULL,
    end_sec DOUBLE PRECISION NOT NULL,
    transcript TEXT NOT NULL,
    normalized_transcript TEXT NOT NULL,
    confidence DOUBLE PRECISION,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(source_id, segment_index)
);

CREATE TABLE IF NOT EXISTS knowledge_units (
    id BIGSERIAL PRIMARY KEY,
    source_id BIGINT NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    unit_key VARCHAR(200) NOT NULL UNIQUE,
    problem_slug VARCHAR(100) NOT NULL,
    category VARCHAR(100) NOT NULL,
    unit_type VARCHAR(50) NOT NULL,
    title VARCHAR(500) NOT NULL,
    summary TEXT NOT NULL,
    body_markdown TEXT NOT NULL,
    source_start_sec DOUBLE PRECISION NOT NULL,
    source_end_sec DOUBLE PRECISION NOT NULL,
    evidence_segment_indices INTEGER[] NOT NULL DEFAULT '{}',
    tags TEXT[] NOT NULL DEFAULT '{}',
    transcript_excerpt TEXT NOT NULL DEFAULT '',
    review_status VARCHAR(50) NOT NULL DEFAULT 'generated',
    embedding VECTOR(1536),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS knowledge_clips (
    id BIGSERIAL PRIMARY KEY,
    source_id BIGINT NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    source_unit_id BIGINT REFERENCES knowledge_units(id) ON DELETE SET NULL,
    clip_key VARCHAR(200) NOT NULL UNIQUE,
    clip_type VARCHAR(50) NOT NULL,
    title VARCHAR(500) NOT NULL,
    file_path TEXT NOT NULL,
    start_sec DOUBLE PRECISION NOT NULL,
    end_sec DOUBLE PRECISION NOT NULL,
    transcript_excerpt TEXT NOT NULL DEFAULT '',
    notes TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_sources_problem_slug ON knowledge_sources(problem_slug);
CREATE INDEX IF NOT EXISTS idx_knowledge_segments_source_id ON knowledge_segments(source_id, segment_index);
CREATE INDEX IF NOT EXISTS idx_knowledge_units_problem_slug ON knowledge_units(problem_slug);
CREATE INDEX IF NOT EXISTS idx_knowledge_units_unit_type ON knowledge_units(unit_type);
CREATE INDEX IF NOT EXISTS idx_knowledge_units_source_id ON knowledge_units(source_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_clips_source_id ON knowledge_clips(source_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_clips_source_unit_id ON knowledge_clips(source_unit_id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_attribute a
        JOIN pg_class c ON c.oid = a.attrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'knowledge_units'
          AND a.attname = 'embedding'
          AND NOT a.attisdropped
          AND format_type(a.atttypid, a.atttypmod) <> 'vector(1536)'
    ) THEN
        ALTER TABLE knowledge_units
            ALTER COLUMN embedding TYPE VECTOR(1536)
            USING NULL::VECTOR(1536);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'update_knowledge_sources_updated_at'
    ) THEN
        CREATE TRIGGER update_knowledge_sources_updated_at
            BEFORE UPDATE ON knowledge_sources
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'update_knowledge_units_updated_at'
    ) THEN
        CREATE TRIGGER update_knowledge_units_updated_at
            BEFORE UPDATE ON knowledge_units
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'update_knowledge_clips_updated_at'
    ) THEN
        CREATE TRIGGER update_knowledge_clips_updated_at
            BEFORE UPDATE ON knowledge_clips
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;
