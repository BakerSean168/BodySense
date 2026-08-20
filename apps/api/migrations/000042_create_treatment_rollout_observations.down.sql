DROP TABLE IF EXISTS treatment_rollout_observations;

ALTER TABLE treatment_revisions
    DROP COLUMN IF EXISTS rollout_provenance;
