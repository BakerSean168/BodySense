CREATE TABLE document_extraction_runs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    upload_id UUID NOT NULL REFERENCES user_uploads(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    configuration_id VARCHAR(80) NOT NULL,
    mechanism_revision VARCHAR(120) NOT NULL,
    document_sha256 CHAR(64) NOT NULL,
    result_sha256 CHAR(64) NOT NULL,
    raw_text_sha256 CHAR(64) NOT NULL,
    indicator_snapshot JSONB NOT NULL DEFAULT '[]',
    source_summary JSONB NOT NULL DEFAULT '{}',
    mechanism_provenance JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT document_extraction_runs_document_sha256_check CHECK (document_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT document_extraction_runs_result_sha256_check CHECK (result_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT document_extraction_runs_raw_text_sha256_check CHECK (raw_text_sha256 ~ '^[0-9a-f]{64}$')
);

CREATE INDEX idx_document_extraction_runs_upload_created
    ON document_extraction_runs(upload_id, created_at DESC);
CREATE INDEX idx_document_extraction_runs_user_created
    ON document_extraction_runs(user_id, created_at DESC);
CREATE INDEX idx_document_extraction_runs_configuration
    ON document_extraction_runs(configuration_id, created_at DESC);
