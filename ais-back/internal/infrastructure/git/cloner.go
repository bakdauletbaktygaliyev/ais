package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
	"go.uber.org/zap"
)

// GoGitCloner implements the analysis.Cloner port using go-git.
type GoGitCloner struct {
	githubToken string
	timeout     time.Duration
	log         *logger.Logger
}

// NewGoGitCloner creates a new cloner.
func NewGoGitCloner(githubToken string, timeout time.Duration, log *logger.Logger) *GoGitCloner {
	return &GoGitCloner{
		githubToken: githubToken,
		timeout:     timeout,
		log:         log.WithComponent("git_cloner"),
	}
}

// Clone performs a depth-1 shallow clone of the repository into destPath.
// Returns the HEAD commit hash on success.
func (c *GoGitCloner) Clone(ctx context.Context, repoURL, destPath string) (string, error) {
	// Ensure the base directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return "", apperrors.Wrapf(apperrors.ErrCodeInternalError, err,
			"failed to create clone base directory: %s", filepath.Dir(destPath))
	}

	// Remove any previous clone at this path
	if _, err := os.Stat(destPath); err == nil {
		if err := os.RemoveAll(destPath); err != nil {
			return "", apperrors.Wrapf(apperrors.ErrCodeInternalError, err,
				"failed to remove existing clone directory: %s", destPath)
		}
	}

	c.log.Info("starting shallow clone",
		zap.String("url", repoURL),
		zap.String("dest", destPath),
	)

	cloneCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cloneOpts := &gogit.CloneOptions{
		URL:          repoURL,
		Depth:        1,
		SingleBranch: true,
		Tags:         gogit.NoTags,
		Progress:     nil, // suppress verbose output
	}

	// Add authentication if a token is provided
	if c.githubToken != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "x-token",
			Password: c.githubToken,
		}
	}

	repo, err := gogit.PlainCloneContext(cloneCtx, destPath, false, cloneOpts)
	if err != nil {
		// Classify the error
		if err == context.DeadlineExceeded || err == context.Canceled {
			return "", apperrors.Newf(apperrors.ErrCodeTimeout,
				"git clone timed out after %s", c.timeout)
		}

		errMsg := err.Error()
		switch {
		case containsAny(errMsg, "authentication required", "could not read Username"):
			return "", apperrors.New(apperrors.ErrCodeRepoPrivate,
				"repository requires authentication — only public repositories are supported")
		case containsAny(errMsg, "repository not found", "not found"):
			return "", apperrors.New(apperrors.ErrCodeRepoNotFound,
				"repository not found")
		case containsAny(errMsg, "no space left", "disk quota"):
			return "", apperrors.New(apperrors.ErrCodeInternalError,
				"insufficient disk space for clone operation")
		default:
			return "", apperrors.Wrapf(apperrors.ErrCodeInternalError, err, "git clone failed")
		}
	}

	// Extract HEAD commit hash
	ref, err := repo.Head()
	if err != nil {
		return "", apperrors.Wrapf(apperrors.ErrCodeInternalError, err, "failed to get HEAD reference")
	}

	commitHash := ref.Hash().String()

	// Get the actual commit for logging
	commit, err := repo.CommitObject(plumbing.NewHash(commitHash))
	if err == nil {
		c.log.Info("clone completed successfully",
			zap.String("commit_hash", commitHash),
			zap.String("commit_message", truncate(commit.Message, 72)),
			zap.Time("commit_time", commit.Author.When),
			zap.String("dest", destPath),
		)
	} else {
		c.log.Info("clone completed",
			zap.String("commit_hash", commitHash),
			zap.String("dest", destPath),
		)
	}

	return commitHash, nil
}

// Cleanup removes the cloned repository directory.
func (c *GoGitCloner) Cleanup(ctx context.Context, destPath string) error {
	if destPath == "" || destPath == "/" {
		return apperrors.New(apperrors.ErrCodeInvalidInput, "invalid cleanup path: empty or root")
	}

	// Safety check: only allow cleanup under /tmp/ais
	if !isValidClonePath(destPath) {
		return apperrors.Newf(apperrors.ErrCodeInvalidInput,
			"cleanup path %s is outside allowed clone directory", destPath)
	}

	c.log.Info("cleaning up clone directory", zap.String("path", destPath))

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return nil // already gone
	}

	if err := os.RemoveAll(destPath); err != nil {
		return apperrors.Wrapf(apperrors.ErrCodeInternalError, err,
			"failed to remove clone directory: %s", destPath)
	}

	c.log.Info("cleanup completed", zap.String("path", destPath))
	return nil
}

// BuildClonePath constructs the local path for a repository clone.
func BuildClonePath(basePath, repoID string) string {
	return filepath.Join(basePath, repoID)
}

func isValidClonePath(path string) bool {
	validPrefixes := []string{"/tmp/ais/", "/tmp/ais-"}
	for _, prefix := range validPrefixes {
		if len(path) > len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	// also allow /tmp/ais exactly
	return path == "/tmp/ais"
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	// Remove newlines for logging
	for i, c := range s {
		if c == '\n' || c == '\r' {
			s = s[:i]
			break
		}
	}
	if len(s) <= maxLen {
		return s
	}
	return fmt.Sprintf("%s...", s[:maxLen-3])
}
