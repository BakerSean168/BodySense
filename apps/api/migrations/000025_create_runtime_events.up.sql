CREATE TABLE runtime_events (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    run_id          UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    turn_id         UUID,
    seq             INT NOT NULL,
    channel         VARCHAR(40) NOT NULL,
    type            VARCHAR(120) NOT NULL,
    ids             JSONB NOT NULL DEFAULT '{}',
    payload         JSONB NOT NULL DEFAULT '{}',
    source          VARCHAR(20) NOT NULL DEFAULT 'go',
    replayable      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, seq)
);

CREATE INDEX idx_runtime_events_run_seq
    ON runtime_events (run_id, seq ASC);

CREATE INDEX idx_runtime_events_conversation_created
    ON runtime_events (conversation_id, created_at DESC);

CREATE INDEX idx_runtime_events_type_created
    ON runtime_events (type, created_at DESC);
