-- A downgrade is safe only while every upload is still on the local adapter.
-- OSS-backed rows must be migrated back explicitly before rolling schema back.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM user_uploads WHERE storage_backend <> 'local') THEN
        RAISE EXCEPTION 'cannot downgrade upload storage schema while non-local upload objects exist';
    END IF;
END $$;

ALTER TABLE user_uploads ADD COLUMN file_path VARCHAR(500);

UPDATE user_uploads
SET file_path = 'uploads/' || storage_key;

ALTER TABLE user_uploads ALTER COLUMN file_path SET NOT NULL;

DROP INDEX IF EXISTS uq_user_uploads_storage_identity;
ALTER TABLE user_uploads
    DROP CONSTRAINT IF EXISTS ck_user_uploads_storage_key,
    DROP CONSTRAINT IF EXISTS ck_user_uploads_storage_backend,
    DROP COLUMN storage_key,
    DROP COLUMN storage_backend;
