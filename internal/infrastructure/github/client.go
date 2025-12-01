package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

func (c *Client) GetAssignedIssues(page, perPage int) ([]Issue, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/issues?page=%d&per_page=%d&state=open", page, perPage)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	rateLimit := parseRateLimit(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, rateLimit, &GitHubError{
			StatusCode: resp.StatusCode,
			Message:    getErrorMessage(resp.StatusCode),
		}
	}

	var issues []Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, rateLimit, err
	}

	return issues, rateLimit, nil
}

func (c *Client) GetRepositoryIssues(owner, repo string, page, perPage int) ([]Issue, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?page=%d&per_page=%d&state=open", owner, repo, page, perPage)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	rateLimit := parseRateLimit(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, rateLimit, &GitHubError{
			StatusCode: resp.StatusCode,
			Message:    getErrorMessage(resp.StatusCode),
		}
	}

	var issues []Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, rateLimit, err
	}

	return issues, rateLimit, nil
}

func (c *Client) ValidateToken() error {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &GitHubError{
			StatusCode: resp.StatusCode,
			Message:    getErrorMessage(resp.StatusCode),
		}
	}

	return nil
}

func (c *Client) GetAllAssignedIssues() ([]Issue, *RateLimitInfo, error) {
	return c.collectAllIssues(func(page int) ([]Issue, *RateLimitInfo, error) {
		return c.GetAssignedIssues(page, maxPerPage)
	})
}

func (c *Client) GetAllRepositoryIssues(owner, repo string) ([]Issue, *RateLimitInfo, error) {
	return c.collectAllIssues(func(page int) ([]Issue, *RateLimitInfo, error) {
		return c.GetRepositoryIssues(owner, repo, page, maxPerPage)
	})
}

func (c *Client) GetUserRepositories(page, perPage int) ([]Repository, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/user/repos?page=%d&per_page=%d&affiliation=owner,collaborator,organization_member", page, perPage)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	rateLimit := parseRateLimit(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, rateLimit, &GitHubError{
			StatusCode: resp.StatusCode,
			Message:    getErrorMessage(resp.StatusCode),
		}
	}

	var repos []Repository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, rateLimit, err
	}

	return repos, rateLimit, nil
}

func (c *Client) GetAllUserRepositories() ([]Repository, *RateLimitInfo, error) {
	var allRepos []Repository
	var lastRateLimit *RateLimitInfo

	for page := 1; ; page++ {
		repos, rateLimit, err := c.GetUserRepositories(page, maxPerPage)
		if err != nil {
			return nil, rateLimit, err
		}

		if rateLimit != nil {
			lastRateLimit = rateLimit
		}

		allRepos = append(allRepos, repos...)

		if len(repos) < maxPerPage {
			break
		}
	}

	return allRepos, lastRateLimit, nil
}

// GetSpecificUserRepositories gets all repositories for a specific user
func (c *Client) GetSpecificUserRepositories(username string, page, perPage int) ([]Repository, *RateLimitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/repos?page=%d&per_page=%d&type=all", username, page, perPage)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	rateLimit := parseRateLimit(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, rateLimit, &GitHubError{
			StatusCode: resp.StatusCode,
			Message:    getErrorMessage(resp.StatusCode),
		}
	}

	var repos []Repository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, rateLimit, err
	}

	return repos, rateLimit, nil
}

// GetAllSpecificUserRepositories gets all repositories for a specific user (all pages)
func (c *Client) GetAllSpecificUserRepositories(username string) ([]Repository, *RateLimitInfo, error) {
	var allRepos []Repository
	var lastRateLimit *RateLimitInfo

	for page := 1; ; page++ {
		repos, rateLimit, err := c.GetSpecificUserRepositories(username, page, maxPerPage)
		if err != nil {
			return nil, rateLimit, err
		}

		if rateLimit != nil {
			lastRateLimit = rateLimit
		}

		allRepos = append(allRepos, repos...)

		if len(repos) < maxPerPage {
			break
		}
	}

	return allRepos, lastRateLimit, nil
}

func (c *Client) collectAllIssues(fetch func(page int) ([]Issue, *RateLimitInfo, error)) ([]Issue, *RateLimitInfo, error) {
	var allIssues []Issue
	var lastRateLimit *RateLimitInfo

	for page := 1; ; page++ {
		issues, rateLimit, err := fetch(page)
		if err != nil {
			return nil, rateLimit, err
		}

		if rateLimit != nil {
			lastRateLimit = rateLimit
		}

		allIssues = append(allIssues, issues...)

		if len(issues) < maxPerPage {
			break
		}
	}

	return allIssues, lastRateLimit, nil
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
