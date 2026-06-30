-- Fix knowledge lifecycle schema gaps from 000019
-- Reference: docs/implementation/phase-07a-knowledge-lifecycle-schema.md

-- 1. Add lifecycle_status to knowledge_units (the core lifecycle tracking column)
ALTER TABLE knowledge_units
    ADD COLUMN IF NOT EXISTS lifecycle_status VARCHAR(50) NOT NULL DEFAULT 'generated';

CREATE INDEX IF NOT EXISTS idx_knowledge_units_lifecycle_status ON knowledge_units(lifecycle_status);

-- 2. Add publication_id to knowledge_units (correct FK direction: units → publications)
-- First ensure the publications table exists, then add the FK column
ALTER TABLE knowledge_units
    ADD COLUMN IF NOT EXISTS publication_id UUID;

-- Add FK constraint only if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_knowledge_units_publication_id'
    ) THEN
        ALTER TABLE knowledge_units
            ADD CONSTRAINT fk_knowledge_units_publication_id
            FOREIGN KEY (publication_id) REFERENCES knowledge_publications(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_knowledge_units_publication_id ON knowledge_units(publication_id);

-- 3. Fix knowledge_publications.status default: 'published' → 'draft'
ALTER TABLE knowledge_publications
    ALTER COLUMN status SET DEFAULT 'draft';

-- 4. Add created_at and updated_at to knowledge_publications
ALTER TABLE knowledge_publications
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Add updated_at trigger for knowledge_publications
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'update_knowledge_publications_updated_at'
    ) THEN
        CREATE TRIGGER update_knowledge_publications_updated_at
            BEFORE UPDATE ON knowledge_publications
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

-- 5. Widen license_status to match plan spec (VARCHAR(30) → VARCHAR(50))
ALTER TABLE knowledge_sources
    ALTER COLUMN license_status TYPE VARCHAR(50);
