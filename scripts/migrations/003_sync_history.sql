-- Migration 003: Extract sync history from app_settings into a separate table
--
-- Creates a sync_history table, migrates existing last_sync data,
-- and drops the last_sync_* columns from app_settings.

BEGIN;

-- Create sync_history table
CREATE TABLE IF NOT EXISTS sync_history (
    id                SERIAL PRIMARY KEY,
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    new_files         INT NOT NULL DEFAULT 0,
    updated_files     INT NOT NULL DEFAULT 0,
    deleted_files     INT NOT NULL DEFAULT 0,
    thumbnails_generated INT NOT NULL DEFAULT 0
);

-- Migrate existing last_sync data from app_settings (if any)
INSERT INTO sync_history (created_at, new_files, updated_files, deleted_files, thumbnails_generated)
SELECT
    COALESCE(last_sync_at, NOW()),
    COALESCE(last_sync_new, 0),
    COALESCE(last_sync_updated, 0),
    COALESCE(last_sync_deleted, 0),
    COALESCE(last_sync_thumbnails, 0)
FROM app_settings
WHERE last_sync_at IS NOT NULL;

-- Drop last_sync_* columns from app_settings
ALTER TABLE app_settings
    DROP COLUMN IF EXISTS last_sync_at,
    DROP COLUMN IF EXISTS last_sync_new,
    DROP COLUMN IF EXISTS last_sync_updated,
    DROP COLUMN IF EXISTS last_sync_deleted,
    DROP COLUMN IF EXISTS last_sync_thumbnails;

COMMIT;
