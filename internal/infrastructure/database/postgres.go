package database

import (
	"context"
	"database/sql"
	"time"

	"github-discord-bot/internal/domain/entity"
	"github-discord-bot/internal/domain/repository"

	"github.com/lib/pq"
)

type PostgresUserSettingRepository struct {
	db *sql.DB
}

func NewPostgresUserSettingRepository(db *sql.DB) repository.UserSettingRepository {
	return &PostgresUserSettingRepository{db: db}
}

func (r *PostgresUserSettingRepository) Save(ctx context.Context, setting *entity.UserSetting) error {
	query := `
		INSERT INTO user_settings (
			guild_id,
			channel_id,
			user_id,
			encrypted_token,
			excluded_repositories,
			excluded_issues_repositories,
			excluded_assign_repositories,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (guild_id, channel_id, user_id)
		DO UPDATE SET encrypted_token = COALESCE(EXCLUDED.encrypted_token, user_settings.encrypted_token),
		              excluded_repositories = COALESCE(EXCLUDED.excluded_repositories, user_settings.excluded_repositories),
		              excluded_issues_repositories = COALESCE(EXCLUDED.excluded_issues_repositories, user_settings.excluded_issues_repositories),
		              excluded_assign_repositories = COALESCE(EXCLUDED.excluded_assign_repositories, user_settings.excluded_assign_repositories),
		              updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		setting.GuildID,
		setting.ChannelID,
		setting.UserID,
		nullStringIfEmpty(setting.EncryptedToken),
		nullArrayIfNil(setting.ExcludedRepositories),
		nullArrayIfNil(setting.ExcludedIssuesRepositories),
		nullArrayIfNil(setting.ExcludedAssignRepositories),
		setting.UpdatedAt,
	)
	return err
}

func nullStringIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// nullArrayIfNil は配列がnilの場合にNULLを返し、空配列の場合はそのまま返します
// これにより、nilの場合は既存の値を保持（COALESCE）し、空配列の場合は明示的にクリアできます
func nullArrayIfNil(arr []string) interface{} {
	if arr == nil {
		return nil
	}
	return pq.Array(arr)
}

func (r *PostgresUserSettingRepository) Find(ctx context.Context, guildID, channelID, userID string) (*entity.UserSetting, error) {
	query := `
		SELECT guild_id, channel_id, user_id, encrypted_token, excluded_repositories, excluded_issues_repositories, excluded_assign_repositories, updated_at
		FROM user_settings
		WHERE guild_id = $1 AND channel_id = $2 AND user_id = $3
	`
	var setting entity.UserSetting
	var encryptedToken sql.NullString
	err := r.db.QueryRowContext(ctx, query, guildID, channelID, userID).Scan(
		&setting.GuildID,
		&setting.ChannelID,
		&setting.UserID,
		&encryptedToken,
		pq.Array(&setting.ExcludedRepositories),
		pq.Array(&setting.ExcludedIssuesRepositories),
		pq.Array(&setting.ExcludedAssignRepositories),
		&setting.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if encryptedToken.Valid {
		setting.EncryptedToken = encryptedToken.String
	}
	if setting.ExcludedRepositories == nil {
		setting.ExcludedRepositories = []string{}
	}
	if setting.ExcludedIssuesRepositories == nil {
		setting.ExcludedIssuesRepositories = []string{}
	}
	if setting.ExcludedAssignRepositories == nil {
		setting.ExcludedAssignRepositories = []string{}
	}

	if err := r.populateNotificationChannels(ctx, &setting); err != nil {
		return nil, err
	}

	return &setting, nil
}

func (r *PostgresUserSettingRepository) FindByGuildAndUser(ctx context.Context, guildID, userID string) (*entity.UserSetting, error) {
	query := `
		SELECT guild_id, channel_id, user_id, encrypted_token, excluded_repositories, excluded_issues_repositories, excluded_assign_repositories, updated_at
		FROM user_settings
		WHERE guild_id = $1 AND user_id = $2
		ORDER BY updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, guildID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aggregated *entity.UserSetting

	for rows.Next() {
		var (
			currentSetting entity.UserSetting
			encryptedToken sql.NullString
		)

		if err := rows.Scan(
			&currentSetting.GuildID,
			&currentSetting.ChannelID,
			&currentSetting.UserID,
			&encryptedToken,
			pq.Array(&currentSetting.ExcludedRepositories),
			pq.Array(&currentSetting.ExcludedIssuesRepositories),
			pq.Array(&currentSetting.ExcludedAssignRepositories),
			&currentSetting.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if encryptedToken.Valid {
			currentSetting.EncryptedToken = encryptedToken.String
		}
		if aggregated == nil {
			copySetting := currentSetting
			aggregated = &copySetting
			continue
		}

		mergeStringField(&aggregated.EncryptedToken, currentSetting.EncryptedToken)
		mergeStringSliceField(&aggregated.ExcludedRepositories, currentSetting.ExcludedRepositories)
		mergeStringSliceField(&aggregated.ExcludedIssuesRepositories, currentSetting.ExcludedIssuesRepositories)
		mergeStringSliceField(&aggregated.ExcludedAssignRepositories, currentSetting.ExcludedAssignRepositories)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if aggregated != nil {
		if aggregated.ExcludedRepositories == nil {
			aggregated.ExcludedRepositories = []string{}
		}
		if aggregated.ExcludedIssuesRepositories == nil {
			aggregated.ExcludedIssuesRepositories = []string{}
		}
		if aggregated.ExcludedAssignRepositories == nil {
			aggregated.ExcludedAssignRepositories = []string{}
		}
		if err := r.populateNotificationChannels(ctx, aggregated); err != nil {
			return nil, err
		}
	}

	return aggregated, nil
}

func (r *PostgresUserSettingRepository) ClearNotificationChannels(ctx context.Context, guildID, userID string) error {
	query := `DELETE FROM user_notification_channels WHERE guild_id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, guildID, userID)
	return err
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

func (r *PostgresUserSettingRepository) SaveNotificationChannelSetting(ctx context.Context, guildID, userID, scope, channelID string) error {
	query := `
		INSERT INTO user_notification_channels (guild_id, user_id, scope, channel_id, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (guild_id, user_id, scope)
		DO UPDATE SET channel_id = EXCLUDED.channel_id,
		             updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query, guildID, userID, scope, channelID)
	return err
}

func (r *PostgresUserSettingRepository) GetNotificationChannels(ctx context.Context, guildID, userID string) (map[string]string, error) {
	query := `
		SELECT scope, channel_id
		FROM user_notification_channels
		WHERE guild_id = $1 AND user_id = $2
	`
	rows, err := r.db.QueryContext(ctx, query, guildID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels := make(map[string]string)
	for rows.Next() {
		var scope, channelID string
		if err := rows.Scan(&scope, &channelID); err != nil {
			return nil, err
		}
		channels[scope] = channelID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return channels, nil
}

func (r *PostgresUserSettingRepository) populateNotificationChannels(ctx context.Context, setting *entity.UserSetting) error {
	channels, err := r.GetNotificationChannels(ctx, setting.GuildID, setting.UserID)
	if err != nil {
		return err
	}

	if ch, ok := channels["all"]; ok {
		setting.NotificationChannelID = ch
	}
	if ch, ok := channels["issues"]; ok {
		setting.NotificationIssuesChannelID = ch
	}
	if ch, ok := channels["assign"]; ok {
		setting.NotificationAssignChannelID = ch
	}

	return nil
}

func mergeStringField(dst *string, src string) {
	if *dst == "" && src != "" {
		*dst = src
	}
}

func mergeStringSliceField(dst *[]string, src []string) {
	if *dst == nil && src != nil {
		*dst = src
	}
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
