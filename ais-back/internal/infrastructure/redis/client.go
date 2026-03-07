package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

const (
	keyPrefixCommitHash = "ais:commit:"
	keyPrefixRepoStatus = "ais:status:"
)

// Client wraps the go-redis client.
type Client struct {
	rdb *redis.Client
	ttl time.Duration
	log *logger.Logger
}

// NewClient creates and verifies a Redis client connection.
func NewClient(url, password string, db int, opts ClientOptions, log *logger.Logger) (*Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeInternalError, err,
			"invalid Redis URL: %s", url)
	}

	if password != "" {
		opt.Password = password
	}
	opt.DB = db
	opt.MaxRetries = opts.MaxRetries
	opt.DialTimeout = opts.DialTimeout
	opt.ReadTimeout = opts.ReadTimeout
	opt.WriteTimeout = opts.WriteTimeout
	opt.PoolSize = opts.PoolSize

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeInternalError, err, "cannot connect to Redis")
	}

	log.Info("Redis connection established", zap.String("url", url))

	return &Client{
		rdb: rdb,
		ttl: opts.CacheTTL,
		log: log.WithComponent("redis_client"),
	}, nil
}

// ClientOptions holds configuration for the Redis client.
type ClientOptions struct {
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
	CacheTTL     time.Duration
}

// Close shuts down the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping checks Redis connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// ---------------------------------------------------------------------------
// CacheRepository implementation
// ---------------------------------------------------------------------------

// CacheRepo implements domainrepo.CacheRepository backed by Redis.
type CacheRepo struct {
	client *Client
	log    *logger.Logger
}

// NewCacheRepo creates a new Redis-backed CacheRepository.
func NewCacheRepo(client *Client, log *logger.Logger) *CacheRepo {
	return &CacheRepo{
		client: client,
		log:    log.WithComponent("cache_repo"),
	}
}

// GetRepoIDByCommitHash returns the repoID associated with a commit hash if cached.
func (r *CacheRepo) GetRepoIDByCommitHash(ctx context.Context, hash string) (string, error) {
	key := keyPrefixCommitHash + hash
	val, err := r.client.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // cache miss — not an error
		}
		r.log.Warn("Redis get error", zap.String("key", key), zap.Error(err))
		return "", apperrors.Wrapf(apperrors.ErrCodeCacheError, err, "cache get failed for hash %s", hash)
	}
	return val, nil
}

// SetCommitHash stores the mapping commitHash → repoID with TTL.
func (r *CacheRepo) SetCommitHash(ctx context.Context, repoID, hash string) error {
	key := keyPrefixCommitHash + hash
	if err := r.client.rdb.Set(ctx, key, repoID, r.client.ttl).Err(); err != nil {
		r.log.Warn("Redis set error", zap.String("key", key), zap.Error(err))
		return apperrors.Wrapf(apperrors.ErrCodeCacheError, err,
			"cache set failed for hash %s", hash)
	}
	r.log.Debug("commit hash cached",
		zap.String("hash", hash), zap.String("repo_id", repoID))
	return nil
}

// InvalidateRepo removes all cache entries for a given repository.
func (r *CacheRepo) InvalidateRepo(ctx context.Context, repoID string) error {
	// We need to scan for all commit hash keys that point to this repoID
	// This is a best-effort operation — errors are logged but not fatal
	iter := r.client.rdb.Scan(ctx, 0, keyPrefixCommitHash+"*", 100).Iterator()
	var keysToDelete []string

	for iter.Next(ctx) {
		key := iter.Val()
		val, err := r.client.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		if val == repoID {
			keysToDelete = append(keysToDelete, key)
		}
	}

	if len(keysToDelete) > 0 {
		if err := r.client.rdb.Del(ctx, keysToDelete...).Err(); err != nil {
			r.log.Warn("Redis del error during invalidation",
				zap.String("repo_id", repoID), zap.Error(err))
		}
	}

	// Also remove status key
	statusKey := keyPrefixRepoStatus + repoID
	_ = r.client.rdb.Del(ctx, statusKey).Err()

	r.log.Debug("repo cache invalidated",
		zap.String("repo_id", repoID),
		zap.Int("keys_removed", len(keysToDelete)))

	return nil
}

// GetRepoStatus retrieves the current analysis status from cache.
func (r *CacheRepo) GetRepoStatus(ctx context.Context, repoID string) (domainrepo.RepoStatus, error) {
	key := keyPrefixRepoStatus + repoID
	val, err := r.client.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // not in cache
		}
		return "", apperrors.Wrapf(apperrors.ErrCodeCacheError, err,
			"failed to get repo status from cache for %s", repoID)
	}
	return domainrepo.RepoStatus(val), nil
}

// SetRepoStatus stores the current analysis status in cache with short TTL.
func (r *CacheRepo) SetRepoStatus(ctx context.Context, repoID string, status domainrepo.RepoStatus) error {
	key := keyPrefixRepoStatus + repoID
	ttl := r.client.ttl
	if status == domainrepo.StatusAnalyzing {
		ttl = 2 * time.Hour // shorter TTL for in-progress analyses
	}
	if err := r.client.rdb.Set(ctx, key, string(status), ttl).Err(); err != nil {
		return apperrors.Wrapf(apperrors.ErrCodeCacheError, err,
			"failed to set repo status in cache for %s", repoID)
	}
	return nil
}

// SetWithTTL stores an arbitrary string value with a custom TTL.
func (r *CacheRepo) SetWithTTL(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := r.client.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return apperrors.Wrapf(apperrors.ErrCodeCacheError, err, "cache set failed for key %s", key)
	}
	return nil
}

// Get retrieves a string value by key.
func (r *CacheRepo) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", apperrors.Wrapf(apperrors.ErrCodeCacheError, err, "cache get failed for key %s", key)
	}
	return val, nil
}

// Delete removes a key from cache.
func (r *CacheRepo) Delete(ctx context.Context, key string) error {
	if err := r.client.rdb.Del(ctx, key).Err(); err != nil {
		return apperrors.Wrapf(apperrors.ErrCodeCacheError, err, "cache delete failed for key %s", key)
	}
	return nil
}

// BuildKey constructs a namespaced Redis key.
func BuildKey(parts ...string) string {
	return fmt.Sprintf("ais:%s", joinParts(parts...))
}

func joinParts(parts ...string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ":"
		}
		result += p
	}
	return result
}
