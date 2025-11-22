package usecase

import (
	"context"
	"errors"
	"fmt"

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

func (u *IssuesUsecase) GetAssignedIssues(ctx context.Context, guildID, channelID, userID string) ([]github.Issue, *github.RateLimitInfo, error) {
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
	return client.GetAllAssignedIssues()
}

func (u *IssuesUsecase) GetRepositoryIssues(ctx context.Context, guildID, channelID, userID, owner, repo string) ([]github.Issue, *github.RateLimitInfo, error) {
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
	issues, rateLimit, err := client.GetAllRepositoryIssues(owner, repo)
	if issues != nil {
		fullName := fmt.Sprintf("%s/%s", owner, repo)
		for idx := range issues {
			if issues[idx].Repository == nil {
				issues[idx].Repository = &github.Repository{FullName: fullName}
			}
		}
	}
	return issues, rateLimit, err
}
