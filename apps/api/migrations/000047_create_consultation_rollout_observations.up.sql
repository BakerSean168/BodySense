CREATE TABLE consultation_rollout_observations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    stage VARCHAR(24) NOT NULL,
    champion_configuration_id VARCHAR(80) NOT NULL,
    challenger_configuration_id VARCHAR(80) NOT NULL,
    canary_bps INTEGER NOT NULL DEFAULT 0,
    decision_identity_match BOOLEAN NOT NULL DEFAULT FALSE,
    replay_input_frozen BOOLEAN NOT NULL DEFAULT FALSE,
    shadow_error TEXT,
    comparison JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_consultation_rollout_observations_pair_stage
    ON consultation_rollout_observations (
        champion_configuration_id,
        challenger_configuration_id,
        stage,
        canary_bps,
        created_at DESC
    );

-- One observation per (run, challenger) pair prevents duplicate counting.
CREATE UNIQUE INDEX uq_consultation_rollout_observation_pair
    ON consultation_rollout_observations (run_id, challenger_configuration_id);
