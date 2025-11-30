-- Add command-specific excluded repositories columns to user_settings table
ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS excluded_issues_repositories TEXT[] DEFAULT '{}';

ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS excluded_assign_repositories TEXT[] DEFAULT '{}';

-- Comment for clarity
COMMENT ON COLUMN user_settings.excluded_repositories IS 'Deprecated: Use excluded_issues_repositories and excluded_assign_repositories instead';
COMMENT ON COLUMN user_settings.excluded_issues_repositories IS 'Excluded repositories for /issues command';
COMMENT ON COLUMN user_settings.excluded_assign_repositories IS 'Excluded repositories for /assign command';
