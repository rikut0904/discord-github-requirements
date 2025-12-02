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
			notification_channel_id,
			notification_issues_channel_id,
			notification_assign_channel_id,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (guild_id, channel_id, user_id)
		DO UPDATE SET encrypted_token = COALESCE(EXCLUDED.encrypted_token, user_settings.encrypted_token),
		              excluded_repositories = COALESCE(EXCLUDED.excluded_repositories, user_settings.excluded_repositories),
		              excluded_issues_repositories = COALESCE(EXCLUDED.excluded_issues_repositories, user_settings.excluded_issues_repositories),
		              excluded_assign_repositories = COALESCE(EXCLUDED.excluded_assign_repositories, user_settings.excluded_assign_repositories),
		              notification_channel_id = COALESCE(EXCLUDED.notification_channel_id, user_settings.notification_channel_id),
		              notification_issues_channel_id = COALESCE(EXCLUDED.notification_issues_channel_id, user_settings.notification_issues_channel_id),
		              notification_assign_channel_id = COALESCE(EXCLUDED.notification_assign_channel_id, user_settings.notification_assign_channel_id),
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
		nullStringIfEmpty(setting.NotificationChannelID),
		nullStringIfEmpty(setting.NotificationIssuesChannelID),
		nullStringIfEmpty(setting.NotificationAssignChannelID),
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
		SELECT guild_id, channel_id, user_id, encrypted_token, excluded_repositories, excluded_issues_repositories, excluded_assign_repositories, notification_channel_id, notification_issues_channel_id, notification_assign_channel_id, updated_at
		FROM user_settings
		WHERE guild_id = $1 AND channel_id = $2 AND user_id = $3
	`
	var setting entity.UserSetting
	var encryptedToken sql.NullString
	var notificationChannelID sql.NullString
	var notificationIssuesChannelID sql.NullString
	var notificationAssignChannelID sql.NullString
	err := r.db.QueryRowContext(ctx, query, guildID, channelID, userID).Scan(
		&setting.GuildID,
		&setting.ChannelID,
		&setting.UserID,
		&encryptedToken,
		pq.Array(&setting.ExcludedRepositories),
		pq.Array(&setting.ExcludedIssuesRepositories),
		pq.Array(&setting.ExcludedAssignRepositories),
		&notificationChannelID,
		&notificationIssuesChannelID,
		&notificationAssignChannelID,
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
	if notificationChannelID.Valid {
		setting.NotificationChannelID = notificationChannelID.String
	}
	if notificationIssuesChannelID.Valid {
		setting.NotificationIssuesChannelID = notificationIssuesChannelID.String
	}
	if notificationAssignChannelID.Valid {
		setting.NotificationAssignChannelID = notificationAssignChannelID.String
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
	return &setting, nil
}

func (r *PostgresUserSettingRepository) FindByGuildAndUser(ctx context.Context, guildID, userID string) (*entity.UserSetting, error) {
	query := `
		SELECT guild_id, channel_id, user_id, encrypted_token, excluded_repositories, excluded_issues_repositories, excluded_assign_repositories, notification_channel_id, notification_issues_channel_id, notification_assign_channel_id, updated_at
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
			currentSetting              entity.UserSetting
			encryptedToken              sql.NullString
			notificationChannelID       sql.NullString
			notificationIssuesChannelID sql.NullString
			notificationAssignChannelID sql.NullString
		)

		if err := rows.Scan(
			&currentSetting.GuildID,
			&currentSetting.ChannelID,
			&currentSetting.UserID,
			&encryptedToken,
			pq.Array(&currentSetting.ExcludedRepositories),
			pq.Array(&currentSetting.ExcludedIssuesRepositories),
			pq.Array(&currentSetting.ExcludedAssignRepositories),
			&notificationChannelID,
			&notificationIssuesChannelID,
			&notificationAssignChannelID,
			&currentSetting.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if encryptedToken.Valid {
			currentSetting.EncryptedToken = encryptedToken.String
		}
		if notificationChannelID.Valid {
			currentSetting.NotificationChannelID = notificationChannelID.String
		}
		if notificationIssuesChannelID.Valid {
			currentSetting.NotificationIssuesChannelID = notificationIssuesChannelID.String
		}
		if notificationAssignChannelID.Valid {
			currentSetting.NotificationAssignChannelID = notificationAssignChannelID.String
		}
		if currentSetting.ExcludedRepositories == nil {
			currentSetting.ExcludedRepositories = []string{}
		}
		if currentSetting.ExcludedIssuesRepositories == nil {
			currentSetting.ExcludedIssuesRepositories = []string{}
		}
		if currentSetting.ExcludedAssignRepositories == nil {
			currentSetting.ExcludedAssignRepositories = []string{}
		}

		if aggregated == nil {
			aggregated = &entity.UserSetting{
				GuildID:                     currentSetting.GuildID,
				ChannelID:                   currentSetting.ChannelID,
				UserID:                      currentSetting.UserID,
				EncryptedToken:              currentSetting.EncryptedToken,
				ExcludedRepositories:        currentSetting.ExcludedRepositories,
				ExcludedIssuesRepositories:  currentSetting.ExcludedIssuesRepositories,
				ExcludedAssignRepositories:  currentSetting.ExcludedAssignRepositories,
				NotificationChannelID:       currentSetting.NotificationChannelID,
				NotificationIssuesChannelID: currentSetting.NotificationIssuesChannelID,
				NotificationAssignChannelID: currentSetting.NotificationAssignChannelID,
				UpdatedAt:                   currentSetting.UpdatedAt,
			}
			continue
		}

		if aggregated.EncryptedToken == "" && currentSetting.EncryptedToken != "" {
			aggregated.EncryptedToken = currentSetting.EncryptedToken
		}
		if len(aggregated.ExcludedRepositories) == 0 && len(currentSetting.ExcludedRepositories) > 0 {
			aggregated.ExcludedRepositories = currentSetting.ExcludedRepositories
		}
		if len(aggregated.ExcludedIssuesRepositories) == 0 && len(currentSetting.ExcludedIssuesRepositories) > 0 {
			aggregated.ExcludedIssuesRepositories = currentSetting.ExcludedIssuesRepositories
		}
		if len(aggregated.ExcludedAssignRepositories) == 0 && len(currentSetting.ExcludedAssignRepositories) > 0 {
			aggregated.ExcludedAssignRepositories = currentSetting.ExcludedAssignRepositories
		}
		if aggregated.NotificationChannelID == "" && currentSetting.NotificationChannelID != "" {
			aggregated.NotificationChannelID = currentSetting.NotificationChannelID
		}
		if aggregated.NotificationIssuesChannelID == "" && currentSetting.NotificationIssuesChannelID != "" {
			aggregated.NotificationIssuesChannelID = currentSetting.NotificationIssuesChannelID
		}
		if aggregated.NotificationAssignChannelID == "" && currentSetting.NotificationAssignChannelID != "" {
			aggregated.NotificationAssignChannelID = currentSetting.NotificationAssignChannelID
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return aggregated, nil
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
