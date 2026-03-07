package memory

import (
	"context"
	"sync"
	"time"

	"github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
)

// RepoRepository is a thread-safe in-memory implementation of repository.RepoRepository.
// For production, replace with a PostgreSQL or Redis-backed implementation.
type RepoRepository struct {
	mu    sync.RWMutex
	repos map[string]*repository.Repo
}

// NewRepoRepository creates a new in-memory repo repository.
func NewRepoRepository() *RepoRepository {
	return &RepoRepository{
		repos: make(map[string]*repository.Repo),
	}
}

// Create stores a new Repo.
func (r *RepoRepository) Create(ctx context.Context, repo *repository.Repo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.repos[repo.ID]; exists {
		return apperrors.Newf(apperrors.ErrCodeAlreadyExists, "repo %s already exists", repo.ID)
	}

	clone := *repo
	r.repos[repo.ID] = &clone
	return nil
}

// Update overwrites an existing Repo.
func (r *RepoRepository) Update(ctx context.Context, repo *repository.Repo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.repos[repo.ID]; !exists {
		return apperrors.Newf(apperrors.ErrCodeNotFound, "repo %s not found", repo.ID)
	}

	repo.UpdatedAt = time.Now()
	clone := *repo
	r.repos[repo.ID] = &clone
	return nil
}

// GetByID retrieves a Repo by ID.
func (r *RepoRepository) GetByID(ctx context.Context, id string) (*repository.Repo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	repo, ok := r.repos[id]
	if !ok {
		return nil, apperrors.Newf(apperrors.ErrCodeNotFound, "repo %s not found", id)
	}

	clone := *repo
	return &clone, nil
}

// GetByURL finds a repo by its GitHub URL.
func (r *RepoRepository) GetByURL(ctx context.Context, url string) (*repository.Repo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, repo := range r.repos {
		if repo.URL == url {
			clone := *repo
			return &clone, nil
		}
	}

	return nil, apperrors.Newf(apperrors.ErrCodeNotFound, "repo with URL %s not found", url)
}

// List returns a paginated list of repos sorted by creation time descending.
func (r *RepoRepository) List(ctx context.Context, limit, offset int) ([]*repository.Repo, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]*repository.Repo, 0, len(r.repos))
	for _, repo := range r.repos {
		clone := *repo
		all = append(all, &clone)
	}

	// Sort by CreatedAt descending
	for i := 0; i < len(all)-1; i++ {
		for j := i + 1; j < len(all); j++ {
			if all[i].CreatedAt.Before(all[j].CreatedAt) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	total := int64(len(all))

	if offset >= len(all) {
		return []*repository.Repo{}, total, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], total, nil
}

// Delete removes a Repo by ID.
func (r *RepoRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.repos[id]; !exists {
		return apperrors.Newf(apperrors.ErrCodeNotFound, "repo %s not found", id)
	}

	delete(r.repos, id)
	return nil
}
