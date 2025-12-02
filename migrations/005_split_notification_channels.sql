ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS notification_issues_channel_id VARCHAR(32);

ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS notification_assign_channel_id VARCHAR(32);

UPDATE user_settings
SET notification_issues_channel_id = notification_channel_id
WHERE (notification_issues_channel_id IS NULL OR notification_issues_channel_id = '')
  AND notification_channel_id IS NOT NULL
  AND notification_channel_id != '';

UPDATE user_settings
SET notification_assign_channel_id = notification_channel_id
WHERE (notification_assign_channel_id IS NULL OR notification_assign_channel_id = '')
  AND notification_channel_id IS NOT NULL
  AND notification_channel_id != '';
