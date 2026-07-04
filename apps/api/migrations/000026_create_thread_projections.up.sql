CREATE TABLE thread_projections (
    conversation_id          UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    user_id                  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title                    TEXT,
    title_status             VARCHAR(20) NOT NULL DEFAULT 'pending',
    status                   VARCHAR(20) NOT NULL DEFAULT 'active',
    pinned                   BOOLEAN NOT NULL DEFAULT FALSE,
    pinned_at                TIMESTAMPTZ,
    default_model            TEXT,
    active_run_id            UUID,
    last_message_at          TIMESTAMPTZ,
    metadata                 JSONB NOT NULL DEFAULT '{}',
    phase                    VARCHAR(30) NOT NULL DEFAULT 'collecting',
    extracted_info           JSONB NOT NULL DEFAULT '[]',
    diagnosis                JSONB,
    treatment_plan           JSONB,
    pending_interactions     JSONB NOT NULL DEFAULT '[]',
    conversation_created_at  TIMESTAMPTZ NOT NULL,
    conversation_updated_at  TIMESTAMPTZ NOT NULL,
    session_created_at       TIMESTAMPTZ NOT NULL,
    session_updated_at       TIMESTAMPTZ NOT NULL,
    ended_at                 TIMESTAMPTZ,
    refreshed_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_thread_projections_user_updated
    ON thread_projections (user_id, conversation_updated_at DESC);

CREATE TABLE thread_projection_messages (
    message_id             UUID PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    conversation_id        UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    turn_id                UUID NOT NULL,
    run_id                 UUID,
    parent_message_id      UUID,
    seq                    INT NOT NULL,
    role                   VARCHAR(20) NOT NULL,
    status                 VARCHAR(20) NOT NULL,
    parts                  JSONB NOT NULL DEFAULT '[]',
    content_text           TEXT NOT NULL DEFAULT '',
    model                  TEXT,
    provider               TEXT,
    provider_message_id    TEXT,
    provider_response_id   TEXT,
    input_tokens           INT,
    output_tokens          INT,
    total_tokens           INT,
    error                  JSONB,
    metadata               JSONB NOT NULL DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    UNIQUE (conversation_id, seq)
);

CREATE INDEX idx_thread_projection_messages_conversation_seq
    ON thread_projection_messages (conversation_id, seq ASC);
