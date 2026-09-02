CREATE TABLE document_indicator_reviews (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    upload_id UUID NOT NULL REFERENCES user_uploads(id) ON DELETE CASCADE,
    extraction_run_id UUID NOT NULL REFERENCES document_extraction_runs(id) ON DELETE CASCADE,
    indicator_index INTEGER NOT NULL,
    action VARCHAR(20) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    reviewed_payload JSONB NOT NULL DEFAULT '{}',
    machine_candidate JSONB NOT NULL DEFAULT '{}',
    source_refs JSONB NOT NULL DEFAULT '[]',
    page_ref JSONB NOT NULL DEFAULT '{}',
    reviewer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT document_indicator_reviews_action_check
        CHECK (action IN ('confirm', 'correct', 'reject')),
    CONSTRAINT document_indicator_reviews_indicator_index_check
        CHECK (indicator_index >= 0),
    CONSTRAINT document_indicator_reviews_idempotency_key_check
        CHECK (idempotency_key <> '')
);
CREATE UNIQUE INDEX idx_document_indicator_reviews_idem
    ON document_indicator_reviews(user_id, extraction_run_id, indicator_index, idempotency_key);
CREATE INDEX idx_document_indicator_reviews_user_created
    ON document_indicator_reviews(user_id, created_at DESC);
CREATE INDEX idx_document_indicator_reviews_upload_created
    ON document_indicator_reviews(upload_id, created_at DESC);
CREATE INDEX idx_document_indicator_reviews_extraction_run
    ON document_indicator_reviews(extraction_run_id, indicator_index, created_at DESC);
