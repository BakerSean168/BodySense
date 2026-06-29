ALTER TABLE user_profiles ADD COLUMN IF NOT EXISTS schedule_mode VARCHAR(20) DEFAULT 'fixed_calendar';
