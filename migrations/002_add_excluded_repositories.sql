-- Add excluded_repositories column to user_settings table
ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS excluded_repositories TEXT[] DEFAULT '{}';

-- Make encrypted_token nullable since we might only save excluded repositories
ALTER TABLE user_settings
ALTER COLUMN encrypted_token DROP NOT NULL;
