package repository

import (
	"context"
	"time"
)

// Language represents a detected programming language in a repository.
type Language string

const (
	LanguageTypeScript  Language = "typescript"
	LanguageJavaScript  Language = "javascript"
	LanguageGo          Language = "go"
	LanguageMixed       Language = "mixed"
	LanguageUnknown     Language = "unknown"
)

// RepoStatus represents the lifecycle state of a repository analysis.
type RepoStatus string

const (
	StatusPending   RepoStatus = "pending"
	StatusAnalyzing RepoStatus = "analyzing"
	StatusReady     RepoStatus = "ready"
	StatusError     RepoStatus = "error"
)

// MonorepoType identifies the monorepo tooling detected in a repository.
type MonorepoType string

const (
	MonorepoNone      MonorepoType = "none"
	MonorepoTurborepo MonorepoType = "turborepo"
	MonorepoNX        MonorepoType = "nx"
	MonorepoPnpm      MonorepoType = "pnpm_workspaces"
	MonorepoYarnWorkspaces MonorepoType = "yarn_workspaces"
	MonorepoGoModules MonorepoType = "go_modules"
)

// Repo is the root domain entity for a repository analysis session.
type Repo struct {
	ID           string
	URL          string
	Owner        string
	Name         string
	DefaultBranch string
	CommitHash   string
	Description  string
	StarCount    int
	ForkCount    int
	SizeKB       int64
	Language     Language
	MonorepoType MonorepoType
	Status       RepoStatus
	ErrorMessage string
	FileCount    int
	DirCount     int
	FunctionCount int
	ClassCount   int
	CycleCount   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ReadyAt      *time.Time
}

// RepoMetadata holds information returned from GitHub API validation.
type RepoMetadata struct {
	Owner         string
	Name          string
	DefaultBranch string
	Description   string
	StarCount     int
	ForkCount     int
	SizeKB        int64
	Private       bool
	Archived      bool
	CloneURL      string
}

// CacheEntry holds the cached analysis state keyed by commit hash.
type CacheEntry struct {
	RepoID     string
	CommitHash string
	CachedAt   time.Time
}

// ---------------------------------------------------------------------------
// Ports (interfaces to be implemented by infrastructure)
// ---------------------------------------------------------------------------

// RepoRepository defines persistence operations for Repo entities.
type RepoRepository interface {
	Create(ctx context.Context, repo *Repo) error
	Update(ctx context.Context, repo *Repo) error
	GetByID(ctx context.Context, id string) (*Repo, error)
	GetByURL(ctx context.Context, url string) (*Repo, error)
	List(ctx context.Context, limit, offset int) ([]*Repo, int64, error)
	Delete(ctx context.Context, id string) error
}

// CacheRepository defines commit-hash-based caching operations.
type CacheRepository interface {
	// GetRepoIDByCommitHash returns the repoID if this commit was already analyzed.
	GetRepoIDByCommitHash(ctx context.Context, hash string) (string, error)

	// SetCommitHash caches the mapping of commitHash → repoID.
	SetCommitHash(ctx context.Context, repoID, hash string) error

	// InvalidateRepo removes all cache entries for a given repo.
	InvalidateRepo(ctx context.Context, repoID string) error

	// GetRepoStatus retrieves the current analysis status from cache.
	GetRepoStatus(ctx context.Context, repoID string) (RepoStatus, error)

	// SetRepoStatus stores the current analysis status in cache.
	SetRepoStatus(ctx context.Context, repoID string, status RepoStatus) error
}

// GitHubClient defines the port for GitHub API interactions.
type GitHubClient interface {
	// ValidateRepo checks that the repo exists, is public, and is within size limits.
	ValidateRepo(ctx context.Context, repoURL string, maxSizeMB int64) (*RepoMetadata, error)

	// GetLatestCommit returns the SHA of the latest commit on the default branch.
	GetLatestCommit(ctx context.Context, owner, name, branch string) (string, error)
}
