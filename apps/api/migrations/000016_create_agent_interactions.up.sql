CREATE TABLE agent_interactions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    tool_call_id TEXT NOT NULL,
    tool_name TEXT NOT NULL DEFAULT 'ask_user',
    question JSONB NOT NULL DEFAULT '{}',
    answer JSONB,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    answered_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    UNIQUE (run_id, tool_call_id)
);

CREATE INDEX idx_agent_interactions_run_id ON agent_interactions(run_id);
CREATE INDEX idx_agent_interactions_conversation_id ON agent_interactions(conversation_id);
CREATE INDEX idx_agent_interactions_status ON agent_interactions(status);
