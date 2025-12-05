package usecase

import (
	"context"
	"fmt"
	"time"

	"github-discord-bot/internal/domain/entity"
	"github-discord-bot/internal/domain/repository"
	"github-discord-bot/internal/infrastructure/crypto"
	"github-discord-bot/internal/infrastructure/github"
)

type SettingUsecase struct {
	repo   repository.UserSettingRepository
	crypto *crypto.AESCrypto
}

func NewSettingUsecase(repo repository.UserSettingRepository, crypto *crypto.AESCrypto) *SettingUsecase {
	return &SettingUsecase{
		repo:   repo,
		crypto: crypto,
	}
}

// ensureSettingWithChannel は設定が存在する場合はchannelIDを更新し、存在しない場合は新規作成します
func ensureSettingWithChannel(setting *entity.UserSetting, guildID, channelID, userID string) *entity.UserSetting {
	if setting == nil {
		return &entity.UserSetting{
			GuildID:   guildID,
			ChannelID: channelID,
			UserID:    userID,
		}
	}
	if setting.ChannelID == "" || setting.ChannelID != channelID {
		setting.ChannelID = channelID
	}
	return setting
}

// validateIssuesOrAssignType はcommandTypeが"issues"または"assign"であることを検証します
func validateIssuesOrAssignType(commandType string) error {
	if commandType != "issues" && commandType != "assign" {
		return fmt.Errorf("invalid commandType: %s (must be 'issues' or 'assign')", commandType)
	}
	return nil
}

func (u *SettingUsecase) SaveToken(ctx context.Context, guildID, channelID, userID, token string) error {
	// Validate token with GitHub API
	client := github.NewClient(token)
	if err := client.ValidateToken(); err != nil {
		return err
	}

	// Encrypt token
	encrypted, err := u.crypto.Encrypt(token)
	if err != nil {
		return err
	}

	setting := &entity.UserSetting{
		GuildID:        guildID,
		ChannelID:      channelID,
		UserID:         userID,
		EncryptedToken: encrypted,
		UpdatedAt:      time.Now(),
	}

	return u.repo.Save(ctx, setting)
}

func (u *SettingUsecase) GetToken(ctx context.Context, guildID, userID string) (string, error) {
	setting, err := u.repo.FindByGuildAndUser(ctx, guildID, userID)
	if err != nil {
		return "", err
	}
	if setting == nil {
		return "", nil
	}

	return u.crypto.Decrypt(setting.EncryptedToken)
}

func (u *SettingUsecase) GetUserSetting(ctx context.Context, guildID, userID string) (*entity.UserSetting, error) {
	return u.repo.FindByGuildAndUser(ctx, guildID, userID)
}

func (u *SettingUsecase) SaveNotificationChannel(ctx context.Context, guildID, channelID, userID, commandType, notificationChannelID string) error {
	// Allow commandType "", "all", "issues", "assign"
	if commandType != "" && commandType != "all" && commandType != "issues" && commandType != "assign" {
		return fmt.Errorf("invalid commandType: %s (must be '', 'all', 'issues' or 'assign')", commandType)
	}

	setting, err := u.repo.FindByGuildAndUser(ctx, guildID, userID)
	if err != nil {
		return err
	}

	setting = ensureSettingWithChannel(setting, guildID, channelID, userID)

	var scopes []string
	switch commandType {
	case "issues":
		scopes = []string{"issues"}
	case "assign":
		scopes = []string{"assign"}
	default:
		// 共通設定: issues と assign スコープのみを設定（all は deprecated のため除外）
		scopes = []string{"issues", "assign"}
	}

	for _, scope := range scopes {
		if err := u.repo.SaveNotificationChannelSetting(ctx, guildID, userID, scope, notificationChannelID); err != nil {
			return err
		}
	}

	setting.UpdatedAt = time.Now()

	return u.repo.Save(ctx, setting)
}

func (u *SettingUsecase) ClearNotificationChannels(ctx context.Context, guildID, userID string) error {
	return u.repo.ClearNotificationChannels(ctx, guildID, userID)
}

func (u *SettingUsecase) SaveExcludedRepositories(ctx context.Context, guildID, channelID, userID string, repositories []string, commandType string) error {
	if err := validateIssuesOrAssignType(commandType); err != nil {
		return err
	}

	setting, err := u.repo.FindByGuildAndUser(ctx, guildID, userID)
	if err != nil {
		return err
	}

	setting = ensureSettingWithChannel(setting, guildID, channelID, userID)

	if commandType == "issues" {
		setting.ExcludedIssuesRepositories = repositories
	} else if commandType == "assign" {
		setting.ExcludedAssignRepositories = repositories
	}

	setting.UpdatedAt = time.Now()

	return u.repo.Save(ctx, setting)
}

func (u *SettingUsecase) GetExcludedRepositories(ctx context.Context, guildID, userID string, commandType string) ([]string, error) {
	if err := validateIssuesOrAssignType(commandType); err != nil {
		return nil, err
	}

	setting, err := u.repo.FindByGuildAndUser(ctx, guildID, userID)
	if err != nil {
		return nil, err
	}
	if setting == nil {
		return []string{}, nil
	}

	if commandType == "issues" {
		return setting.ExcludedIssuesRepositories, nil
	} else if commandType == "assign" {
		return setting.ExcludedAssignRepositories, nil
	}

	return []string{}, nil
}
