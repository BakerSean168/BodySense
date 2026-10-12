-- Isolated-test rollback only; runtime consumers must be detached before use.
DROP TABLE public.body_fact_commits;
DROP TABLE public.body_state_outbox;
DROP TABLE public.body_facts;
DROP TABLE public.statement_evidence;
