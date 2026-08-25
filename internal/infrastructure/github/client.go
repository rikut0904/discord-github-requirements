package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	httpClient *http.Client
	token      string
}

const maxPerPage = 100

type Issue struct {
	Number     int         `json:"number"`
	Title      string      `json:"title"`
	HTMLURL    string      `json:"html_url"`
	State      string      `json:"state"`
	UpdatedAt  time.Time   `json:"updated_at"`
	Labels     []Label     `json:"labels"`
	Assignees  []User      `json:"assignees"`
	Repository *Repository `json:"repository"`
}

type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type User struct {
	Login string `json:"login"`
}

type Repository struct {
	FullName string `json:"full_name"`
}

type DependabotPullRequest struct {
	Number        int         `json:"number"`
	Title         string      `json:"title"`
	HTMLURL       string      `json:"html_url"`
	UpdatedAt     time.Time   `json:"updated_at"`
	RepositoryURL string      `json:"repository_url"`
	Author        User        `json:"user"`
	Repository    *Repository `json:"-"`
}

type pullRequestSearchResponse struct {
	Items []DependabotPullRequest `json:"items"`
}

type RateLimitInfo struct {
	Remaining int
	ResetAt   time.Time
}

type GitHubError struct {
	StatusCode int
	Message    string
}

func (e *GitHubError) Error() string {
	return fmt.Sprintf("GitHub API error: %d - %s", e.StatusCode, e.Message)
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
	}
}

// doRequest はGitHub APIへの汎用的なHTTPリクエストを実行します
func (c *Client) doRequest(url string, result interface{}) (*RateLimitInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rateLimit := parseRateLimit(resp)

	if resp.StatusCode != http.StatusOK {
		return rateLimit, &GitHubError{
			StatusCode: resp.StatusCode,
			Message:    getErrorMessage(resp.StatusCode),
		}
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return rateLimit, err
		}
	}

	return rateLimit, nil
}

func (c *Client) GetAssignedIssues(page, perPage int) ([]Issue, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/issues?page=%d&per_page=%d&state=open", page, perPage)

	var issues []Issue
	rateLimit, err := c.doRequest(url, &issues)
	return issues, rateLimit, err
}

func (c *Client) GetRepositoryIssues(owner, repo string, page, perPage int) ([]Issue, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?page=%d&per_page=%d&state=open", owner, repo, page, perPage)

	var issues []Issue
	rateLimit, err := c.doRequest(url, &issues)
	return issues, rateLimit, err
}

func (c *Client) ValidateToken() error {
	_, err := c.doRequest("https://api.github.com/user", nil)
	return err
}

func (c *Client) GetAllAssignedIssues() ([]Issue, *RateLimitInfo, error) {
	return collectAllPages(func(page int) ([]Issue, *RateLimitInfo, error) {
		return c.GetAssignedIssues(page, maxPerPage)
	})
}

// GetDependabotPullRequests はDependabotが作成したオープンPRを検索します。
func (c *Client) GetDependabotPullRequests(page, perPage int) ([]DependabotPullRequest, *RateLimitInfo, error) {
	query := url.Values{}
	query.Set("q", "is:pr is:open author:app/dependabot")
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(perPage))

	var result pullRequestSearchResponse
	rateLimit, err := c.doRequest("https://api.github.com/search/issues?"+query.Encode(), &result)
	if err != nil {
		return nil, rateLimit, err
	}

	for idx := range result.Items {
		result.Items[idx].Repository = repositoryFromAPIURL(result.Items[idx].RepositoryURL)
	}
	return result.Items, rateLimit, nil
}

func (c *Client) GetAllDependabotPullRequests() ([]DependabotPullRequest, *RateLimitInfo, error) {
	repositories, rateLimit, err := c.GetAllUserRepositories()
	if err != nil {
		return nil, rateLimit, err
	}

	type repositoryResult struct {
		pullRequests []DependabotPullRequest
		rateLimit    *RateLimitInfo
		err          error
	}
	results := make(chan repositoryResult, len(repositories))
	semaphore := make(chan struct{}, 10)
	var waitGroup sync.WaitGroup

	for _, repository := range repositories {
		repository := repository
		parts := strings.SplitN(repository.FullName, "/", 2)
		if len(parts) != 2 {
			continue
		}

		waitGroup.Add(1)
		go func(owner, repo, fullName string) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			pullRequests, repositoryRateLimit, err := c.GetAllRepositoryPullRequests(owner, repo)
			if err != nil {
				results <- repositoryResult{rateLimit: repositoryRateLimit, err: err}
				return
			}
			filtered := make([]DependabotPullRequest, 0, len(pullRequests))
			for _, pullRequest := range pullRequests {
				if strings.EqualFold(pullRequest.Author.Login, "dependabot[bot]") {
					pullRequest.Repository = &Repository{FullName: fullName}
					filtered = append(filtered, pullRequest)
				}
			}
			results <- repositoryResult{pullRequests: filtered, rateLimit: repositoryRateLimit}
		}(parts[0], parts[1], repository.FullName)
	}

	waitGroup.Wait()
	close(results)

	var dependabotPullRequests []DependabotPullRequest
	for result := range results {
		if result.err != nil {
			return nil, result.rateLimit, result.err
		}
		if result.rateLimit != nil {
			rateLimit = result.rateLimit
		}
		dependabotPullRequests = append(dependabotPullRequests, result.pullRequests...)
	}

	return dependabotPullRequests, rateLimit, nil
}

func (c *Client) GetRepositoryPullRequests(owner, repo string, page, perPage int) ([]DependabotPullRequest, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open&page=%d&per_page=%d", owner, repo, page, perPage)

	var pullRequests []DependabotPullRequest
	rateLimit, err := c.doRequest(url, &pullRequests)
	return pullRequests, rateLimit, err
}

func (c *Client) GetAllRepositoryPullRequests(owner, repo string) ([]DependabotPullRequest, *RateLimitInfo, error) {
	return collectAllPages(func(page int) ([]DependabotPullRequest, *RateLimitInfo, error) {
		return c.GetRepositoryPullRequests(owner, repo, page, maxPerPage)
	})
}

func repositoryFromAPIURL(repositoryURL string) *Repository {
	const prefix = "https://api.github.com/repos/"
	if !strings.HasPrefix(repositoryURL, prefix) {
		return nil
	}
	fullName := strings.TrimSuffix(strings.TrimPrefix(repositoryURL, prefix), "/")
	if fullName == "" {
		return nil
	}
	return &Repository{FullName: fullName}
}

func (c *Client) GetAllRepositoryIssues(owner, repo string) ([]Issue, *RateLimitInfo, error) {
	return collectAllPages(func(page int) ([]Issue, *RateLimitInfo, error) {
		return c.GetRepositoryIssues(owner, repo, page, maxPerPage)
	})
}

func (c *Client) GetUserRepositories(page, perPage int) ([]Repository, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/user/repos?page=%d&per_page=%d&affiliation=owner,collaborator,organization_member", page, perPage)

	var repos []Repository
	rateLimit, err := c.doRequest(url, &repos)
	return repos, rateLimit, err
}

// collectAllPages は汎用的なページネーション処理を行います
func collectAllPages[T any](fetch func(page int) ([]T, *RateLimitInfo, error)) ([]T, *RateLimitInfo, error) {
	var allItems []T
	var lastRateLimit *RateLimitInfo

	for page := 1; ; page++ {
		items, rateLimit, err := fetch(page)
		if err != nil {
			return nil, rateLimit, err
		}

		if rateLimit != nil {
			lastRateLimit = rateLimit
		}

		allItems = append(allItems, items...)

		if len(items) < maxPerPage {
			break
		}
	}

	return allItems, lastRateLimit, nil
}

func (c *Client) GetAllUserRepositories() ([]Repository, *RateLimitInfo, error) {
	return collectAllPages(func(page int) ([]Repository, *RateLimitInfo, error) {
		return c.GetUserRepositories(page, maxPerPage)
	})
}

// GetSpecificUserRepositories gets all repositories for a specific user
func (c *Client) GetSpecificUserRepositories(username string, page, perPage int) ([]Repository, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/repos?page=%d&per_page=%d&type=all", username, page, perPage)

	var repos []Repository
	rateLimit, err := c.doRequest(url, &repos)
	return repos, rateLimit, err
}

// GetAllSpecificUserRepositories gets all repositories for a specific user (all pages)
func (c *Client) GetAllSpecificUserRepositories(username string) ([]Repository, *RateLimitInfo, error) {
	return collectAllPages(func(page int) ([]Repository, *RateLimitInfo, error) {
		return c.GetSpecificUserRepositories(username, page, maxPerPage)
	})
}

func parseRateLimit(resp *http.Response) *RateLimitInfo {
	remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	resetUnix, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)

	return &RateLimitInfo{
		Remaining: remaining,
		ResetAt:   time.Unix(resetUnix, 0),
	}
}

func getErrorMessage(statusCode int) string {
	switch statusCode {
	case 401:
		return "認証に失敗しました。トークンが無効または期限切れです。"
	case 403:
		return "アクセスが拒否されました。権限を確認してください。"
	case 404:
		return "リソースが見つかりません。"
	case 422:
		return "入力内容に問題があります。"
	default:
		return "予期しないエラーが発生しました。"
	}
}
