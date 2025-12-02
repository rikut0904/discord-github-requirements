CREATE TABLE IF NOT EXISTS user_notification_channels (
    guild_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    scope VARCHAR(16) NOT NULL,
    channel_id VARCHAR(32) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, user_id, scope),
    CHECK (scope IN ('all', 'issues', 'assign'))
);

CREATE INDEX idx_user_notification_channels_scope ON user_notification_channels(scope);
