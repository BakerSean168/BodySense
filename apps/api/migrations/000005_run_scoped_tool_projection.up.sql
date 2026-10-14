-- Provider tool-call identifiers are scoped to an execution run, not global.
-- Retain raw provider IDs for exact tool response/Ask User recovery identity.
ALTER TABLE public.thread_projection_tool_calls
    DROP CONSTRAINT thread_projection_tool_calls_pkey,
    ADD CONSTRAINT thread_projection_tool_calls_pkey PRIMARY KEY (run_id, tool_call_id);
