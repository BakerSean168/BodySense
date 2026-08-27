ALTER TABLE user_profiles
    DROP COLUMN IF EXISTS sleep_pattern,
    DROP COLUMN IF EXISTS activity_pattern,
    DROP COLUMN IF EXISTS birth_date;
