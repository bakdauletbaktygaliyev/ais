package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"

	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	
)

var repoURLPattern = regexp.MustCompile(
	`^(?:https?://)?github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?(?:/.*)?$`,
)

// Client implements the domainrepo.GitHubClient port.
type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
	log        *logger.Logger
}

// NewClient creates a new GitHub API client.
func NewClient(token, baseURL string, timeout time.Duration, log *logger.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		token:   token,
		baseURL: strings.TrimRight(baseURL, "/"),
		log:     log.WithComponent("github_client"),
	}
}

// ValidateRepo checks that the repository exists, is public, and is within size limits.
func (c *Client) ValidateRepo(ctx context.Context, repoURL string, maxSizeMB int64) (*domainrepo.RepoMetadata, error) {
	owner, name, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInvalidInput, "invalid GitHub URL", err)
	}

	c.log.Info("validating repository",
		zap.String("owner", owner),
		zap.String("name", name),
	)

	apiURL := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, name)
	req, err := c.newRequest(ctx, http.MethodGet, apiURL)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to create request", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "GitHub API request failed", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to read response body", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusNotFound:
		return nil, apperrors.New(apperrors.ErrCodeRepoNotFound,
			fmt.Sprintf("repository %s/%s not found or is private", owner, name))
	case http.StatusForbidden, http.StatusUnauthorized:
		return nil, apperrors.New(apperrors.ErrCodeRepoPrivate,
			fmt.Sprintf("repository %s/%s is private or access is forbidden", owner, name))
	case http.StatusTooManyRequests:
		return nil, apperrors.New(apperrors.ErrCodeGitHubRateLimit, "GitHub API rate limit exceeded")
	default:
		return nil, apperrors.Newf(apperrors.ErrCodeInternalError,
			"GitHub API returned unexpected status %d", resp.StatusCode)
	}

	var apiRepo githubRepoResponse
	if err := json.Unmarshal(body, &apiRepo); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to parse GitHub response", err)
	}

	if apiRepo.Private {
		return nil, apperrors.New(apperrors.ErrCodeRepoPrivate,
			fmt.Sprintf("repository %s/%s is private", owner, name))
	}

	if apiRepo.Archived {
		c.log.Warn("repository is archived, proceeding with analysis",
			zap.String("repo", fmt.Sprintf("%s/%s", owner, name)))
	}

	// GitHub API returns size in KB
	sizeKB := int64(apiRepo.Size)
	sizeMB := sizeKB / 1024
	if sizeMB > maxSizeMB {
		return nil, apperrors.Newf(apperrors.ErrCodeRepoTooLarge,
			"repository size %dMB exceeds maximum allowed size of %dMB", sizeMB, maxSizeMB)
	}

	cloneURL := apiRepo.CloneURL
	if c.token != "" {
		// Inject token into clone URL for authenticated cloning
		cloneURL = fmt.Sprintf("https://%s@github.com/%s/%s.git", c.token, owner, name)
	}

	metadata := &domainrepo.RepoMetadata{
		Owner:         apiRepo.Owner.Login,
		Name:          apiRepo.Name,
		DefaultBranch: apiRepo.DefaultBranch,
		Description:   apiRepo.Description,
		StarCount:     apiRepo.StargazersCount,
		ForkCount:     apiRepo.ForksCount,
		SizeKB:        sizeKB,
		Private:       apiRepo.Private,
		Archived:      apiRepo.Archived,
		CloneURL:      cloneURL,
	}

	c.log.Info("repository validated successfully",
		zap.String("repo", fmt.Sprintf("%s/%s", owner, name)),
		zap.Int64("size_kb", sizeKB),
		zap.String("default_branch", metadata.DefaultBranch),
	)

	return metadata, nil
}

// GetLatestCommit returns the SHA of the latest commit on the given branch.
func (c *Client) GetLatestCommit(ctx context.Context, owner, name, branch string) (string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.baseURL, owner, name, branch)
	req, err := c.newRequest(ctx, http.MethodGet, apiURL)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to create request", err)
	}

	// Use the lightweight commit endpoint
	req.Header.Set("Accept", "application/vnd.github.sha")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrCodeInternalError, "GitHub API request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", apperrors.Newf(apperrors.ErrCodeInternalError,
			"GitHub API returned status %d when fetching commit", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to read response body", err)
	}

	// With the sha media type, GitHub returns the bare SHA as plain text
	sha := strings.TrimSpace(string(body))
	if len(sha) == 40 || len(sha) == 64 {
		return sha, nil
	}

	// Fallback: parse as JSON
	var commitResp struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &commitResp); err == nil && commitResp.SHA != "" {
		return commitResp.SHA, nil
	}

	return "", apperrors.New(apperrors.ErrCodeInternalError, "failed to extract commit SHA from GitHub response")
}

// ParseRepoURL extracts owner and name from a GitHub URL.
func ParseRepoURL(repoURL string) (owner, name string, err error) {
	return parseRepoURL(repoURL)
}

func parseRepoURL(repoURL string) (owner, name string, err error) {
	matches := repoURLPattern.FindStringSubmatch(repoURL)
	if matches == nil {
		return "", "", fmt.Errorf("not a valid GitHub repository URL: %s", repoURL)
	}
	return matches[1], matches[2], nil
}

func (c *Client) newRequest(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "AIS-Architecture-Insight-System/1.0")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return req, nil
}

// ---------------------------------------------------------------------------
// GitHub API response types
// ---------------------------------------------------------------------------

type githubRepoResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Archived      bool   `json:"archived"`
	Fork          bool   `json:"fork"`
	Size          int    `json:"size"` // in KB per GitHub docs
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	StargazersCount int  `json:"stargazers_count"`
	ForksCount    int    `json:"forks_count"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// helper field types for logger usage
func String(key, val string) interface{} { return nil }
func Int64(key string, val int64) interface{} { return nil }
func Int(key string, val int) interface{} { return nil }
