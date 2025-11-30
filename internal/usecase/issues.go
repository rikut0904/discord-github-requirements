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
	issues, rateLimit, err := client.GetAllAssignedIssues()
	if err != nil {
		return nil, rateLimit, err
	}

	// Apply excluded repositories filter
	filteredIssues := u.filterExcludedRepositories(issues, setting.ExcludedRepositories)
	return filteredIssues, rateLimit, nil
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

func (u *IssuesUsecase) filterExcludedRepositories(issues []github.Issue, excludedRepos []string) []github.Issue {
	if len(excludedRepos) == 0 {
		return issues
	}

	excludeMap := make(map[string]bool)
	for _, repo := range excludedRepos {
		excludeMap[repo] = true
	}

	var filtered []github.Issue
	for _, issue := range issues {
		if issue.Repository != nil {
			if !excludeMap[issue.Repository.FullName] {
				filtered = append(filtered, issue)
			}
		} else {
			filtered = append(filtered, issue)
		}
	}

	return filtered
}
