CREATE TABLE IF NOT EXISTS user_settings (
    guild_id VARCHAR(32) NOT NULL,
    channel_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    encrypted_token TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, channel_id, user_id)
);

CREATE INDEX idx_user_settings_guild ON user_settings(guild_id);
CREATE INDEX idx_user_settings_channel ON user_settings(guild_id, channel_id);
