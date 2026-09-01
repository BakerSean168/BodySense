-- assessment-output-v1 cannot faithfully represent v2's deliberate absence of
-- health grades/scores. Never manufacture a legacy grade during rollback.
-- Schema downgrade is safe only before a v2 report has been persisted.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM assessment_reports
        WHERE contract_revision = 'assessment-output-v2'
    ) THEN
        RAISE EXCEPTION
            'cannot downgrade assessment evidence contract while assessment-output-v2 reports exist';
    END IF;
END
$$;

ALTER TABLE assessment_reports
    DROP CONSTRAINT IF EXISTS assessment_reports_health_grade_check;

ALTER TABLE assessment_reports
    ALTER COLUMN health_grade SET NOT NULL,
    ALTER COLUMN dimension_scores SET NOT NULL;

ALTER TABLE assessment_reports
    ADD CONSTRAINT assessment_reports_health_grade_check
    CHECK (health_grade IN ('A', 'B', 'C', 'D'));

ALTER TABLE assessment_reports
    DROP COLUMN IF EXISTS evidence_gaps,
    DROP COLUMN IF EXISTS evidence_coverage,
    DROP COLUMN IF EXISTS contract_revision;
