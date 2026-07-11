-- Posture photo AI analysis: store per-upload analysis result independently
-- from OCR (which is report-specific). analysis_status defaults to 'none' so
-- existing rows and non-photo uploads are unaffected.
ALTER TABLE user_uploads
    ADD COLUMN IF NOT EXISTS analysis_status varchar(20) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS analysis_result jsonb;

-- Supports "latest three-view posture analysis per user" lookups.
CREATE INDEX IF NOT EXISTS idx_user_uploads_user_type_created
    ON user_uploads (user_id, file_type, created_at DESC);
