ALTER TABLE body_state_facts
    ADD COLUMN body_region_id VARCHAR(80);

ALTER TABLE body_state_observations
    ADD COLUMN body_region_id VARCHAR(80);
