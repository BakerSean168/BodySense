-- =============================================================
-- 000013: Session Redesign — new conversation / message / run tables
-- =============================================================

-- Drop the old consultation_sessions table from migration 000006
-- before re-creating it with the new schema.
DROP TABLE IF EXISTS consultation_sessions CASCADE;

-- 1. conversations -------------------------------------------------------

CREATE TABLE conversations (
    id                      UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id                 UUID NOT NULL,

    -- basic info
    title                   TEXT,
    title_status            VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- pending / generating / generated

    status                  VARCHAR(20) NOT NULL DEFAULT 'active',
    -- active / archived / deleted

    -- pin (generic feature)
    pinned                  BOOLEAN NOT NULL DEFAULT FALSE,
    pinned_at               TIMESTAMPTZ,

    -- model config
    default_model           TEXT,
    system_prompt_version   TEXT,

    -- provider bookkeeping
    provider                    TEXT,
    provider_conversation_id    TEXT,
    provider_last_response_id   TEXT,

    -- active stream state (disconnect recovery)
    active_run_id           UUID,
    active_stream_id        TEXT,

    -- summary for long-context compression
    summary                 TEXT,

    -- flexible extension
    metadata                JSONB NOT NULL DEFAULT '{}',

    -- timestamps
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_message_at         TIMESTAMPTZ,
    deleted_at              TIMESTAMPTZ
);

CREATE INDEX idx_conversations_user_last
    ON conversations (user_id, last_message_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_conversations_pinned
    ON conversations (user_id, pinned, pinned_at DESC)
    WHERE deleted_at IS NULL AND pinned = TRUE;

-- 2. messages ------------------------------------------------------------

CREATE TABLE messages (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    conversation_id     UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    turn_id             UUID NOT NULL,

    parent_message_id   UUID,
    role                VARCHAR(20) NOT NULL,       -- user / assistant / system / tool
    status              VARCHAR(20) NOT NULL DEFAULT 'completed',
    -- submitted / streaming / completed / failed / aborted

    seq                 INT NOT NULL,

    -- content (multimodal)
    parts               JSONB NOT NULL DEFAULT '[]',
    content_text        TEXT,                       -- plain-text copy for full-text search

    -- provider info
    model               TEXT,
    provider            TEXT,
    provider_message_id TEXT,
    provider_response_id TEXT,

    -- token usage
    input_tokens        INT,
    output_tokens       INT,
    total_tokens        INT,

    -- error
    error               JSONB,
    metadata            JSONB NOT NULL DEFAULT '{}',

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (conversation_id, seq)
);

CREATE INDEX idx_messages_conversation_seq ON messages (conversation_id, seq);
CREATE INDEX idx_messages_turn            ON messages (turn_id);
CREATE INDEX idx_messages_conversation_role ON messages (conversation_id, role);

-- 3. runs ----------------------------------------------------------------

CREATE TABLE runs (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    conversation_id     UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    turn_id             UUID NOT NULL,

    request_id          TEXT NOT NULL,
    user_id             UUID NOT NULL,

    status              VARCHAR(20) NOT NULL DEFAULT 'running',
    -- running / completed / failed / cancelled

    model               TEXT NOT NULL,
    provider            TEXT,
    provider_response_id TEXT,

    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ,

    error               JSONB,
    usage               JSONB,
    metadata            JSONB NOT NULL DEFAULT '{}',

    UNIQUE (user_id, request_id)
);

CREATE INDEX idx_runs_conversation ON runs (conversation_id);
CREATE UNIQUE INDEX idx_runs_one_running_per_conversation
    ON runs (conversation_id)
    WHERE status = 'running';
CREATE INDEX idx_runs_turn         ON runs (turn_id);

-- 4. conversation_shares -------------------------------------------------

CREATE TABLE conversation_shares (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    conversation_id     UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    share_token         VARCHAR(32) UNIQUE NOT NULL,
    snapshot_messages   JSONB NOT NULL,
    snapshot_title      VARCHAR(200),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_conversation_shares_token        ON conversation_shares (share_token);
CREATE INDEX idx_conversation_shares_conversation ON conversation_shares (conversation_id);

-- 5. consultation_sessions (new, domain extension) -----------------------

CREATE TABLE consultation_sessions (
    conversation_id     UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,

    phase               VARCHAR(30) NOT NULL DEFAULT 'collecting',
    -- collecting / ready_for_analysis / analysis_ready

    extracted_info      JSONB NOT NULL DEFAULT '[]',

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at            TIMESTAMPTZ
);

CREATE INDEX idx_consultation_phase
    ON consultation_sessions (phase);
