-- TP-D1 PREVIEW ONLY. Not an executable production migration.
-- Apply only in a synthetic, throwaway PG18 database. Physical removal
-- must not be promoted until the default-off Go compatibility path,
-- active-plan UI selection and rollback/tenant acceptance have passed.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = 'training_plans'
      AND indexname = 'idx_training_plans_user_active'
  ) THEN
    RAISE EXCEPTION 'TP-D1 prerequisite: legacy active index missing';
  END IF;
END $$;

-- Non-unique idx_training_plans_user_id remains for owner lookup.
DROP INDEX public.idx_training_plans_user_active;
