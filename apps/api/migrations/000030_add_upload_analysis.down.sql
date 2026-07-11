DROP INDEX IF EXISTS idx_user_uploads_user_type_created;
ALTER TABLE user_uploads
    DROP COLUMN IF EXISTS analysis_result,
    DROP COLUMN IF EXISTS analysis_status;
