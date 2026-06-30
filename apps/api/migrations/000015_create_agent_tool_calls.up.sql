CREATE TABLE agent_tool_calls (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    tool_call_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    arguments JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(30) NOT NULL DEFAULT 'running',
    result JSONB,
    error JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    UNIQUE (run_id, tool_call_id)
);

CREATE INDEX idx_agent_tool_calls_run_id ON agent_tool_calls(run_id);
CREATE INDEX idx_agent_tool_calls_conversation_id ON agent_tool_calls(conversation_id);
