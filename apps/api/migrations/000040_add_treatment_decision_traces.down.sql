ALTER TABLE treatment_revisions
    DROP COLUMN IF EXISTS acceptance_decision_trace,
    DROP COLUMN IF EXISTS generation_decision_trace;
