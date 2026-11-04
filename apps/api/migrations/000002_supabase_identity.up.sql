-- Supabase Auth is the credential authority. BodySense stores only a business principal.
ALTER TABLE public.users DROP COLUMN IF EXISTS password_hash;
