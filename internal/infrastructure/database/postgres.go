package database

import (
	"context"
	"database/sql"
	"time"

	"github-discord-bot/internal/domain/entity"
	"github-discord-bot/internal/domain/repository"

	_ "github.com/lib/pq"
)

type PostgresUserSettingRepository struct {
	db *sql.DB
}

func NewPostgresUserSettingRepository(db *sql.DB) repository.UserSettingRepository {
	return &PostgresUserSettingRepository{db: db}
}

func (r *PostgresUserSettingRepository) Save(ctx context.Context, setting *entity.UserSetting) error {
	query := `
		INSERT INTO user_settings (guild_id, channel_id, user_id, encrypted_token, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (guild_id, channel_id, user_id)
		DO UPDATE SET encrypted_token = $4, updated_at = $5
	`
	_, err := r.db.ExecContext(ctx, query,
		setting.GuildID,
		setting.ChannelID,
		setting.UserID,
		setting.EncryptedToken,
		setting.UpdatedAt,
	)
	return err
}

func (r *PostgresUserSettingRepository) Find(ctx context.Context, guildID, channelID, userID string) (*entity.UserSetting, error) {
	query := `
		SELECT guild_id, channel_id, user_id, encrypted_token, updated_at
		FROM user_settings
		WHERE guild_id = $1 AND channel_id = $2 AND user_id = $3
	`
	var setting entity.UserSetting
	err := r.db.QueryRowContext(ctx, query, guildID, channelID, userID).Scan(
		&setting.GuildID,
		&setting.ChannelID,
		&setting.UserID,
		&setting.EncryptedToken,
		&setting.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *PostgresUserSettingRepository) Delete(ctx context.Context, guildID, channelID, userID string) error {
	query := `DELETE FROM user_settings WHERE guild_id = $1 AND channel_id = $2 AND user_id = $3`
	_, err := r.db.ExecContext(ctx, query, guildID, channelID, userID)
	return err
}

func (r *PostgresUserSettingRepository) DeleteByGuild(ctx context.Context, guildID string) error {
	query := `DELETE FROM user_settings WHERE guild_id = $1`
	_, err := r.db.ExecContext(ctx, query, guildID)
	return err
}

func (r *PostgresUserSettingRepository) DeleteByChannel(ctx context.Context, guildID, channelID string) error {
	query := `DELETE FROM user_settings WHERE guild_id = $1 AND channel_id = $2`
	_, err := r.db.ExecContext(ctx, query, guildID, channelID)
	return err
}

func InitDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
