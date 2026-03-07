package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	App      AppConfig
	Neo4j    Neo4jConfig
	Redis    RedisConfig
	GitHub   GitHubConfig
	GRPC     GRPCConfig
	Analysis AnalysisConfig
	Log      LogConfig
}

type AppConfig struct {
	Port         string
	Env          string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type Neo4jConfig struct {
	URI      string
	User     string
	Password string
	Database string
	MaxConn  int
}

type RedisConfig struct {
	URL          string
	Password     string
	DB           int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
	CacheTTL     time.Duration
}

type GitHubConfig struct {
	Token          string
	BaseURL        string
	RequestTimeout time.Duration
}

type GRPCConfig struct {
	AIServiceAddr    string
	ConnTimeout      time.Duration
	RequestTimeout   time.Duration
	KeepaliveTime    time.Duration
	KeepaliveTimeout time.Duration
}

type AnalysisConfig struct {
	MaxRepoSizeMB  int64
	CloneBasePath  string
	CloneTimeout   time.Duration
	ParseWorkers   int
	SkipDirs       []string
	MaxFileSize    int64
}

type LogConfig struct {
	Level  string
	Format string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	cfg := &Config{}

	// App
	cfg.App.Port = v.GetString("APP_PORT")
	cfg.App.Env = v.GetString("APP_ENV")
	cfg.App.ReadTimeout = v.GetDuration("APP_READ_TIMEOUT")
	cfg.App.WriteTimeout = v.GetDuration("APP_WRITE_TIMEOUT")
	cfg.App.IdleTimeout = v.GetDuration("APP_IDLE_TIMEOUT")

	// Neo4j
	cfg.Neo4j.URI = v.GetString("NEO4J_URI")
	cfg.Neo4j.User = v.GetString("NEO4J_USER")
	cfg.Neo4j.Password = v.GetString("NEO4J_PASSWORD")
	cfg.Neo4j.Database = v.GetString("NEO4J_DATABASE")
	cfg.Neo4j.MaxConn = v.GetInt("NEO4J_MAX_CONN")

	if cfg.Neo4j.URI == "" {
		return nil, fmt.Errorf("NEO4J_URI is required")
	}

	// Redis
	cfg.Redis.URL = v.GetString("REDIS_URL")
	cfg.Redis.Password = v.GetString("REDIS_PASSWORD")
	cfg.Redis.DB = v.GetInt("REDIS_DB")
	cfg.Redis.MaxRetries = v.GetInt("REDIS_MAX_RETRIES")
	cfg.Redis.DialTimeout = v.GetDuration("REDIS_DIAL_TIMEOUT")
	cfg.Redis.ReadTimeout = v.GetDuration("REDIS_READ_TIMEOUT")
	cfg.Redis.WriteTimeout = v.GetDuration("REDIS_WRITE_TIMEOUT")
	cfg.Redis.PoolSize = v.GetInt("REDIS_POOL_SIZE")
	cfg.Redis.CacheTTL = v.GetDuration("REDIS_CACHE_TTL")

	if cfg.Redis.URL == "" {
		return nil, fmt.Errorf("REDIS_URL is required")
	}

	// GitHub
	cfg.GitHub.Token = v.GetString("GITHUB_TOKEN")
	cfg.GitHub.BaseURL = v.GetString("GITHUB_BASE_URL")
	cfg.GitHub.RequestTimeout = v.GetDuration("GITHUB_REQUEST_TIMEOUT")

	// GRPC
	cfg.GRPC.AIServiceAddr = v.GetString("GRPC_AI_ADDR")
	cfg.GRPC.ConnTimeout = v.GetDuration("GRPC_CONN_TIMEOUT")
	cfg.GRPC.RequestTimeout = v.GetDuration("GRPC_REQUEST_TIMEOUT")
	cfg.GRPC.KeepaliveTime = v.GetDuration("GRPC_KEEPALIVE_TIME")
	cfg.GRPC.KeepaliveTimeout = v.GetDuration("GRPC_KEEPALIVE_TIMEOUT")

	if cfg.GRPC.AIServiceAddr == "" {
		return nil, fmt.Errorf("GRPC_AI_ADDR is required")
	}

	// Analysis
	cfg.Analysis.MaxRepoSizeMB = v.GetInt64("MAX_REPO_SIZE_MB")
	cfg.Analysis.CloneBasePath = v.GetString("CLONE_BASE_PATH")
	cfg.Analysis.CloneTimeout = v.GetDuration("CLONE_TIMEOUT")
	cfg.Analysis.ParseWorkers = v.GetInt("PARSE_WORKERS")
	cfg.Analysis.MaxFileSize = v.GetInt64("MAX_FILE_SIZE_BYTES")
	cfg.Analysis.SkipDirs = []string{
		"node_modules", ".git", "vendor", "dist", ".next",
		"build", ".cache", "__pycache__", ".pytest_cache",
		"coverage", ".nyc_output", "target", "out",
	}

	// Log
	cfg.Log.Level = v.GetString("LOG_LEVEL")
	cfg.Log.Format = v.GetString("LOG_FORMAT")

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	// App
	v.SetDefault("APP_PORT", "8080")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_READ_TIMEOUT", "30s")
	v.SetDefault("APP_WRITE_TIMEOUT", "30s")
	v.SetDefault("APP_IDLE_TIMEOUT", "120s")

	// Neo4j
	v.SetDefault("NEO4J_URI", "bolt://localhost:7687")
	v.SetDefault("NEO4J_USER", "neo4j")
	v.SetDefault("NEO4J_PASSWORD", "password")
	v.SetDefault("NEO4J_DATABASE", "neo4j")
	v.SetDefault("NEO4J_MAX_CONN", 50)

	// Redis
	v.SetDefault("REDIS_URL", "redis://localhost:6379")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("REDIS_MAX_RETRIES", 3)
	v.SetDefault("REDIS_DIAL_TIMEOUT", "5s")
	v.SetDefault("REDIS_READ_TIMEOUT", "3s")
	v.SetDefault("REDIS_WRITE_TIMEOUT", "3s")
	v.SetDefault("REDIS_POOL_SIZE", 20)
	v.SetDefault("REDIS_CACHE_TTL", "24h")

	// GitHub
	v.SetDefault("GITHUB_BASE_URL", "https://api.github.com")
	v.SetDefault("GITHUB_REQUEST_TIMEOUT", "15s")

	// GRPC
	v.SetDefault("GRPC_AI_ADDR", "localhost:50051")
	v.SetDefault("GRPC_CONN_TIMEOUT", "10s")
	v.SetDefault("GRPC_REQUEST_TIMEOUT", "300s")
	v.SetDefault("GRPC_KEEPALIVE_TIME", "30s")
	v.SetDefault("GRPC_KEEPALIVE_TIMEOUT", "10s")

	// Analysis
	v.SetDefault("MAX_REPO_SIZE_MB", 500)
	v.SetDefault("CLONE_BASE_PATH", "/tmp/ais")
	v.SetDefault("CLONE_TIMEOUT", "120s")
	v.SetDefault("PARSE_WORKERS", 8)
	v.SetDefault("MAX_FILE_SIZE_BYTES", 1048576) // 1MB

	// Log
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")
}
