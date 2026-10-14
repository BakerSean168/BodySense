-- Restoring a globally unique key cannot discard valid per-run history.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.thread_projection_tool_calls
        GROUP BY tool_call_id HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'Cannot restore global tool identity while distinct runs share a tool_call_id';
    END IF;
END $$;
ALTER TABLE public.thread_projection_tool_calls
    DROP CONSTRAINT thread_projection_tool_calls_pkey,
    ADD CONSTRAINT thread_projection_tool_calls_pkey PRIMARY KEY (tool_call_id);
