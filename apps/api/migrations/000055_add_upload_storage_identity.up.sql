-- 000055: User upload durable storage identity
--
-- file_path was a host-filesystem coordinate and therefore could not survive
-- API container/ECS loss. New persistence owns storage by backend + opaque key.
-- Existing rows are migrated only when they match the historical local layout;
-- unexpected paths fail closed instead of being guessed.

ALTER TABLE user_uploads
    ADD COLUMN storage_backend VARCHAR(20),
    ADD COLUMN storage_key VARCHAR(500);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM user_uploads
        WHERE file_path IS NULL
           OR file_path !~ '^uploads/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/[^/]+$'
           OR position(chr(92) in file_path) > 0
    ) THEN
        RAISE EXCEPTION 'user_uploads contains a legacy file_path that cannot be safely converted to UploadStorage identity';
    END IF;
END $$;

UPDATE user_uploads
SET storage_backend = 'local',
    storage_key = substring(file_path FROM 9);

ALTER TABLE user_uploads
    ALTER COLUMN storage_backend SET NOT NULL,
    ALTER COLUMN storage_key SET NOT NULL,
    ADD CONSTRAINT ck_user_uploads_storage_backend
        CHECK (storage_backend IN ('local', 'oss')),
    ADD CONSTRAINT ck_user_uploads_storage_key
        CHECK (
            storage_key <> ''
            AND length(storage_key) <= 492
            AND storage_key !~ '^/'
            AND storage_key !~ '(^|/)\.\.(/|$)'
            AND position(chr(92) in storage_key) = 0
        );

ALTER TABLE user_uploads DROP COLUMN file_path;

CREATE UNIQUE INDEX uq_user_uploads_storage_identity
    ON user_uploads (storage_backend, storage_key);
