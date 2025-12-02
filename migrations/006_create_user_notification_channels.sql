CREATE TABLE IF NOT EXISTS user_notification_channels (
    guild_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    scope VARCHAR(16) NOT NULL,
    channel_id VARCHAR(32) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, user_id, scope)
);

-- migrate existing notification settings
INSERT INTO user_notification_channels (guild_id, user_id, scope, channel_id, updated_at)
SELECT guild_id, user_id, 'issues', notification_issues_channel_id, updated_at
FROM user_settings
WHERE notification_issues_channel_id IS NOT NULL AND notification_issues_channel_id != ''
ON CONFLICT (guild_id, user_id, scope) DO UPDATE
SET channel_id = EXCLUDED.channel_id,
    updated_at = EXCLUDED.updated_at;

INSERT INTO user_notification_channels (guild_id, user_id, scope, channel_id, updated_at)
SELECT guild_id, user_id, 'assign', notification_assign_channel_id, updated_at
FROM user_settings
WHERE notification_assign_channel_id IS NOT NULL AND notification_assign_channel_id != ''
ON CONFLICT (guild_id, user_id, scope) DO UPDATE
SET channel_id = EXCLUDED.channel_id,
    updated_at = EXCLUDED.updated_at;

INSERT INTO user_notification_channels (guild_id, user_id, scope, channel_id, updated_at)
SELECT guild_id, user_id, 'all', notification_channel_id, updated_at
FROM user_settings
WHERE notification_channel_id IS NOT NULL AND notification_channel_id != ''
ON CONFLICT (guild_id, user_id, scope) DO UPDATE
SET channel_id = EXCLUDED.channel_id,
    updated_at = EXCLUDED.updated_at;
