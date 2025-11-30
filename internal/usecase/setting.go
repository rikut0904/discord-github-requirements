package usecase

import (
	"context"
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

func (u *SettingUsecase) SaveExcludedRepositories(ctx context.Context, guildID, channelID, userID string, repositories []string) error {
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

	setting.ExcludedRepositories = repositories
	setting.UpdatedAt = time.Now()

	return u.repo.Save(ctx, setting)
}

func (u *SettingUsecase) GetExcludedRepositories(ctx context.Context, guildID, channelID, userID string) ([]string, error) {
	setting, err := u.repo.Find(ctx, guildID, channelID, userID)
	if err != nil {
		return nil, err
	}
	if setting == nil {
		return []string{}, nil
	}

	return setting.ExcludedRepositories, nil
}
