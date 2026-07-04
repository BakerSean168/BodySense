CREATE TABLE thread_projection_tool_calls (
    tool_call_id      TEXT PRIMARY KEY,
    conversation_id   UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    run_id            UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    message_id        UUID REFERENCES messages(id) ON DELETE SET NULL,
    tool_name         TEXT NOT NULL,
    arguments         JSONB NOT NULL DEFAULT '{}',
    status            VARCHAR(30) NOT NULL,
    result            JSONB,
    error             JSONB,
    created_at        TIMESTAMPTZ NOT NULL,
    started_at        TIMESTAMPTZ NOT NULL,
    finished_at       TIMESTAMPTZ,
    metadata          JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_thread_projection_tool_calls_conversation_created
    ON thread_projection_tool_calls (conversation_id, created_at ASC);

CREATE INDEX idx_thread_projection_tool_calls_run_created
    ON thread_projection_tool_calls (run_id, created_at ASC);
