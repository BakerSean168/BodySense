CREATE TABLE assessment_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    health_grade VARCHAR(5) NOT NULL,
    dimension_scores JSONB NOT NULL DEFAULT '{}',
    identified_issues JSONB NOT NULL DEFAULT '[]',
    improvement_summary JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_assessment_reports_user_id ON assessment_reports(user_id);
