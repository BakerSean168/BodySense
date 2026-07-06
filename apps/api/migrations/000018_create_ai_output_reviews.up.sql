CREATE TABLE ai_output_reviews (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    run_id UUID REFERENCES runs(id) ON DELETE SET NULL,
    job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
    output_type VARCHAR(50) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'accepted',
    issues JSONB NOT NULL DEFAULT '[]',
    validated_output JSONB,
    raw_output JSONB,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_output_reviews_run_id ON ai_output_reviews(run_id);
CREATE INDEX idx_ai_output_reviews_job_id ON ai_output_reviews(job_id);
CREATE INDEX idx_ai_output_reviews_conversation_id ON ai_output_reviews(conversation_id);
CREATE INDEX idx_ai_output_reviews_status ON ai_output_reviews(status);
