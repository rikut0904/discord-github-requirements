ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS excluded_dependabot_repositories TEXT[] DEFAULT '{}'::TEXT[];

ALTER TABLE user_notification_channels
    DROP CONSTRAINT IF EXISTS user_notification_channels_scope_check;

ALTER TABLE user_notification_channels
    ADD CONSTRAINT user_notification_channels_scope_check
    CHECK (scope IN ('all', 'issues', 'assign', 'dependabot'));
