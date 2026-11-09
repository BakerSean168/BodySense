-- Re-introduces the legacy schema shape for development rollback only.
-- Password hashes cannot be recovered from Supabase Auth; old logins will not work.
ALTER TABLE public.users ADD COLUMN IF NOT EXISTS password_hash varchar(255) NOT NULL DEFAULT '';
