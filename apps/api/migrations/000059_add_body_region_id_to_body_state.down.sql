ALTER TABLE body_state_observations
    DROP COLUMN IF EXISTS body_region_id;

ALTER TABLE body_state_facts
    DROP COLUMN IF EXISTS body_region_id;
