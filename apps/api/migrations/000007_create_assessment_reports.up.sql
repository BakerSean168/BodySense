CREATE TABLE assessment_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(40) NOT NULL
        CHECK (status IN ('completed', 'insufficient_information')),
    health_grade VARCHAR(5) NOT NULL
        CHECK (health_grade IN ('A', 'B', 'C', 'D')),
    dimension_scores JSONB NOT NULL DEFAULT '{}',
    observations JSONB NOT NULL DEFAULT '[]',
    summary TEXT NOT NULL DEFAULT '',
    information_gaps JSONB NOT NULL DEFAULT '[]',
    safety_notes JSONB NOT NULL DEFAULT '[]',
    body_state_revision BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_assessment_reports_user_created
    ON assessment_reports(user_id, created_at DESC);
