ALTER TABLE assessment_reports
    ADD COLUMN contract_revision VARCHAR(80) NOT NULL DEFAULT 'assessment-output-v1',
    ADD COLUMN evidence_coverage JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN evidence_gaps JSONB NOT NULL DEFAULT '[]';

ALTER TABLE assessment_reports
    ALTER COLUMN health_grade DROP NOT NULL,
    ALTER COLUMN dimension_scores DROP NOT NULL;

ALTER TABLE assessment_reports
    DROP CONSTRAINT IF EXISTS assessment_reports_health_grade_check;

ALTER TABLE assessment_reports
    ADD CONSTRAINT assessment_reports_health_grade_check
    CHECK (health_grade IS NULL OR health_grade IN ('A', 'B', 'C', 'D'));
