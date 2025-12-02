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

func (u *SettingUsecase) GetToken(ctx context.Context, guildID, channelID, userID string) (string, error) {
	setting, err := u.repo.Find(ctx, guildID, channelID, userID)
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

func (u *SettingUsecase) SaveNotificationChannel(ctx context.Context, guildID, channelID, userID, notificationChannelID string) error {
	setting, err := u.repo.Find(ctx, guildID, channelID, userID)
	if err != nil {
		return err
	}
	if setting == nil {
		setting = &entity.UserSetting{
			GuildID:   guildID,
			ChannelID: channelID,
			UserID:    userID,
		}
	}

	setting.NotificationChannelID = notificationChannelID
	setting.UpdatedAt = time.Now()

	return u.repo.Save(ctx, setting)
}

func (u *SettingUsecase) SaveExcludedRepositories(ctx context.Context, guildID, channelID, userID string, repositories []string, commandType string) error {
	// Validate commandType
	if commandType != "issues" && commandType != "assign" {
		return fmt.Errorf("invalid commandType: %s (must be 'issues' or 'assign')", commandType)
	}

	setting, err := u.repo.Find(ctx, guildID, channelID, userID)
	if err != nil {
		return err
	}
	if setting == nil {
		setting = &entity.UserSetting{
			GuildID:   guildID,
			ChannelID: channelID,
			UserID:    userID,
		}
	}

	if commandType == "issues" {
		setting.ExcludedIssuesRepositories = repositories
	} else if commandType == "assign" {
		setting.ExcludedAssignRepositories = repositories
	}

	setting.UpdatedAt = time.Now()

	return u.repo.Save(ctx, setting)
}

func (u *SettingUsecase) GetExcludedRepositories(ctx context.Context, guildID, channelID, userID string, commandType string) ([]string, error) {
	// Validate commandType
	if commandType != "issues" && commandType != "assign" {
		return nil, fmt.Errorf("invalid commandType: %s (must be 'issues' or 'assign')", commandType)
	}

	setting, err := u.repo.Find(ctx, guildID, channelID, userID)
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
