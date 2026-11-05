-- S1 rollback only drops the inert table; no notes are served by this slice.
DROP TABLE IF EXISTS public.episodic_notes;
DROP INDEX IF EXISTS public.conversations_identity_owner_uidx;
