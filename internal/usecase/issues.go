package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github-discord-bot/internal/domain/entity"
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

// getDecryptedToken はユーザー設定を取得し、トークンを復号化して返します
func (u *IssuesUsecase) getDecryptedToken(ctx context.Context, guildID, channelID, userID string) (string, error) {
	setting, err := u.repo.Find(ctx, guildID, channelID, userID)
	if err != nil {
		return "", err
	}
	if setting == nil {
		return "", ErrTokenNotFound
	}

	token, err := u.crypto.Decrypt(setting.EncryptedToken)
	if err != nil {
		return "", err
	}

	return token, nil
}

// getSettingAndToken はユーザー設定を取得し、トークンを復号化して両方を返します
func (u *IssuesUsecase) getSettingAndToken(ctx context.Context, guildID, channelID, userID string) (*entity.UserSetting, string, error) {
	setting, err := u.repo.Find(ctx, guildID, channelID, userID)
	if err != nil {
		return nil, "", err
	}
	if setting == nil {
		return nil, "", ErrTokenNotFound
	}

	token, err := u.crypto.Decrypt(setting.EncryptedToken)
	if err != nil {
		return nil, "", err
	}

	return setting, token, nil
}

func (u *IssuesUsecase) GetAssignedIssues(ctx context.Context, guildID, channelID, userID string) ([]github.Issue, *github.RateLimitInfo, error) {
	setting, token, err := u.getSettingAndToken(ctx, guildID, channelID, userID)
	if err != nil {
		return nil, nil, err
	}

	client := github.NewClient(token)
	issues, rateLimit, err := client.GetAllAssignedIssues()
	if err != nil {
		return nil, rateLimit, err
	}

	// Apply excluded repositories filter for assign command
	filteredIssues := u.filterExcludedRepositories(issues, setting.ExcludedAssignRepositories)
	return filteredIssues, rateLimit, nil
}

func (u *IssuesUsecase) GetRepositoryIssues(ctx context.Context, guildID, channelID, userID, owner, repo string) ([]github.Issue, *github.RateLimitInfo, error) {
	token, err := u.getDecryptedToken(ctx, guildID, channelID, userID)
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

func (u *IssuesUsecase) GetAllRepositoriesIssues(ctx context.Context, guildID, channelID, userID string) ([]github.Issue, *github.RateLimitInfo, error) {
	setting, token, err := u.getSettingAndToken(ctx, guildID, channelID, userID)
	if err != nil {
		return nil, nil, err
	}

	client := github.NewClient(token)

	// Get all user repositories
	repos, rateLimit, err := client.GetAllUserRepositories()
	if err != nil {
		return nil, rateLimit, err
	}

	// Fetch issues from repositories with exclusion filtering
	allIssues, rl := fetchIssuesFromRepositories(client, repos, setting.ExcludedIssuesRepositories)
	if rl != nil {
		rateLimit = rl
	}

	return allIssues, rateLimit, nil
}

func (u *IssuesUsecase) GetUserIssues(ctx context.Context, guildID, channelID, userID, username string) ([]github.Issue, *github.RateLimitInfo, error) {
	setting, token, err := u.getSettingAndToken(ctx, guildID, channelID, userID)
	if err != nil {
		return nil, nil, err
	}

	client := github.NewClient(token)

	// Get all repositories for the specific user
	repos, rateLimit, err := client.GetAllSpecificUserRepositories(username)
	if err != nil {
		return nil, rateLimit, err
	}

	// Fetch issues from repositories with exclusion filtering
	allIssues, rl := fetchIssuesFromRepositories(client, repos, setting.ExcludedIssuesRepositories)
	if rl != nil {
		rateLimit = rl
	}

	return allIssues, rateLimit, nil
}

func splitRepoFullName(fullName string) []string {
	return strings.SplitN(fullName, "/", 2)
}

// fetchIssuesFromRepositories は複数のリポジトリからIssueを取得する共通ロジックです
func fetchIssuesFromRepositories(client *github.Client, repos []github.Repository, excludedRepos []string) ([]github.Issue, *github.RateLimitInfo) {
	var allIssues []github.Issue
	var rateLimit *github.RateLimitInfo

	for _, repo := range repos {
		// Skip excluded repositories using pattern matching
		if isRepositoryExcluded(repo.FullName, excludedRepos) {
			continue
		}

		// Extract owner and repo name
		parts := splitRepoFullName(repo.FullName)
		if len(parts) != 2 {
			continue
		}

		owner := parts[0]
		repoName := parts[1]

		// Get issues for this repository
		issues, rl, err := client.GetAllRepositoryIssues(owner, repoName)
		if err != nil {
			// Skip repositories with errors (e.g., permission issues)
			continue
		}

		if rl != nil {
			rateLimit = rl
		}

		// Add repository info to each issue
		for idx := range issues {
			if issues[idx].Repository == nil {
				issues[idx].Repository = &github.Repository{FullName: repo.FullName}
			}
		}

		allIssues = append(allIssues, issues...)
	}

	return allIssues, rateLimit
}

func (u *IssuesUsecase) filterExcludedRepositories(issues []github.Issue, excludedRepos []string) []github.Issue {
	if len(excludedRepos) == 0 {
		return issues
	}

	var filtered []github.Issue
	for _, issue := range issues {
		if issue.Repository != nil {
			if !isRepositoryExcluded(issue.Repository.FullName, excludedRepos) {
				filtered = append(filtered, issue)
			}
		} else {
			filtered = append(filtered, issue)
		}
	}

	return filtered
}

func isRepositoryExcluded(repoFullName string, excludePatterns []string) bool {
	for _, pattern := range excludePatterns {
		if matchesExcludePattern(repoFullName, pattern) {
			return true
		}
	}
	return false
}

func matchesExcludePattern(repoFullName, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	repoFullName = strings.TrimSpace(repoFullName)

	// Exact match: "owner/repo"
	if pattern == repoFullName {
		return true
	}

	// Organization wildcard: "owner/*"
	if strings.HasSuffix(pattern, "/*") {
		owner := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(repoFullName, owner+"/")
	}

	// Organization match: "owner" (treated as "owner/*")
	if !strings.Contains(pattern, "/") {
		return strings.HasPrefix(repoFullName, pattern+"/")
	}

	return false
}
