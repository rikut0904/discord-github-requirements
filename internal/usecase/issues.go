package usecase

import (
	"context"
	"errors"

	"github-discord-bot/internal/domain/repository"
	"github-discord-bot/internal/infrastructure/crypto"
	"github-discord-bot/internal/infrastructure/github"
)

type IssuesUsecase struct {
	repo   repository.UserSettingRepository
	crypto *crypto.AESCrypto
}

func NewIssuesUsecase(repo repository.UserSettingRepository, crypto *crypto.AESCrypto) *IssuesUsecase {
	return &IssuesUsecase{
		repo:   repo,
		crypto: crypto,
	}
}

var ErrTokenNotFound = errors.New("token not registered")

func (u *IssuesUsecase) GetIssues(ctx context.Context, guildID, channelID, userID string, page, perPage int) ([]github.Issue, *github.RateLimitInfo, error) {
	setting, err := u.repo.Find(ctx, guildID, channelID, userID)
	if err != nil {
		return nil, nil, err
	}
	if setting == nil {
		return nil, nil, ErrTokenNotFound
	}

	token, err := u.crypto.Decrypt(setting.EncryptedToken)
	if err != nil {
		return nil, nil, err
	}

	client := github.NewClient(token)
	return client.GetIssues(page, perPage)
}
