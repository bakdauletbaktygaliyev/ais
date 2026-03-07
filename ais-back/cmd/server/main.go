package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	appanalysis "github.com/bakdaulet/ais/ais-back/internal/application/analysis"
	appchat "github.com/bakdaulet/ais/ais-back/internal/application/chat"
	appgraph "github.com/bakdaulet/ais/ais-back/internal/application/graph"
	deliveryhttp "github.com/bakdaulet/ais/ais-back/internal/delivery/http"
	"github.com/bakdaulet/ais/ais-back/internal/delivery/http/handlers"
	"github.com/bakdaulet/ais/ais-back/internal/delivery/websocket"
	githubinfra "github.com/bakdaulet/ais/ais-back/internal/infrastructure/github"
	gitinfra "github.com/bakdaulet/ais/ais-back/internal/infrastructure/git"
	grpcinfra "github.com/bakdaulet/ais/ais-back/internal/infrastructure/grpc"
	"github.com/bakdaulet/ais/ais-back/internal/infrastructure/memory"
	neo4jinfra "github.com/bakdaulet/ais/ais-back/internal/infrastructure/neo4j"
	parserinfra "github.com/bakdaulet/ais/ais-back/internal/infrastructure/parser"
	redisinfra "github.com/bakdaulet/ais/ais-back/internal/infrastructure/redis"
	"github.com/bakdaulet/ais/ais-back/pkg/config"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

func main() {
	// ---- Load configuration ----
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// ---- Initialize logger ----
	log, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("starting ais-back",
		zap.String("env", cfg.App.Env),
		zap.String("port", cfg.App.Port),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ---- Infrastructure: Neo4j ----
	neo4jClient, err := neo4jinfra.NewClient(
		cfg.Neo4j.URI,
		cfg.Neo4j.User,
		cfg.Neo4j.Password,
		cfg.Neo4j.Database,
		log,
	)
	if err != nil {
		log.Fatal("failed to connect to Neo4j", zap.Error(err))
	}
	defer neo4jClient.Close(ctx)

	if err := neo4jClient.EnsureConstraints(ctx); err != nil {
		log.Warn("constraint setup warning", zap.Error(err))
	}

	// ---- Infrastructure: Redis ----
	redisClient, err := redisinfra.NewClient(
		cfg.Redis.URL,
		cfg.Redis.Password,
		cfg.Redis.DB,
		redisinfra.ClientOptions{
			MaxRetries:   cfg.Redis.MaxRetries,
			DialTimeout:  cfg.Redis.DialTimeout,
			ReadTimeout:  cfg.Redis.ReadTimeout,
			WriteTimeout: cfg.Redis.WriteTimeout,
			PoolSize:     cfg.Redis.PoolSize,
			CacheTTL:     cfg.Redis.CacheTTL,
		},
		log,
	)
	if err != nil {
		log.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	// ---- Infrastructure: gRPC AI client ----
	aiClient, err := grpcinfra.NewAIGRPCClient(
		cfg.GRPC.AIServiceAddr,
		cfg.GRPC.ConnTimeout,
		cfg.GRPC.RequestTimeout,
		cfg.GRPC.KeepaliveTime,
		cfg.GRPC.KeepaliveTimeout,
		log,
	)
	if err != nil {
		log.Warn("failed to connect to AI service (non-fatal, will retry)",
			zap.Error(err),
			zap.String("addr", cfg.GRPC.AIServiceAddr),
		)
		// Don't fatal — AI service may start later
	}

	// ---- Infrastructure: Parsers ----
	tsParser, err := parserinfra.NewTreeSitterParser(log)
	if err != nil {
		log.Fatal("failed to initialize tree-sitter parsers", zap.Error(err))
	}

	// ---- Infrastructure: Repositories ----
	graphRepo := neo4jinfra.NewGraphRepo(neo4jClient, log)
	cacheRepo := redisinfra.NewCacheRepo(redisClient, log)
	repoRepository := memory.NewRepoRepository()

	// ---- Infrastructure: GitHub & Git ----
	gitHubClient := githubinfra.NewClient(
		cfg.GitHub.Token,
		cfg.GitHub.BaseURL,
		cfg.GitHub.RequestTimeout,
		log,
	)

	cloner := gitinfra.NewGoGitCloner(
		cfg.GitHub.Token,
		cfg.Analysis.CloneTimeout,
		log,
	)

	fsWalker := parserinfra.NewFSWalker(cfg.Analysis.MaxFileSize, log)
	langDetector := parserinfra.NewLangDetector(log)

	// ---- Application layer ----
	analysisUC := appanalysis.NewUseCase(appanalysis.UseCaseConfig{
		RepoRepo:         repoRepository,
		CacheRepo:        cacheRepo,
		GraphRepo:        graphRepo,
		GitHubClient:     gitHubClient,
		Cloner:           cloner,
		FSWalker:         fsWalker,
		LangDetector:     langDetector,
		Parser:           tsParser,
		AIClient:         aiClient,
		CloneBase:        cfg.Analysis.CloneBasePath,
		SkipDirs:         cfg.Analysis.SkipDirs,
		ParseWorkers:     cfg.Analysis.ParseWorkers,
		MaxFileSizeBytes: cfg.Analysis.MaxFileSize,
		Log:              log,
	})

	graphUC := appgraph.NewUseCase(graphRepo, log)

	var chatUC *appchat.UseCase
	if aiClient != nil {
		chatUC = appchat.NewUseCase(aiClient, log)
	} else {
		// Create a no-op chat use case when AI service is unavailable
		chatUC = appchat.NewUseCase(nil, log)
	}

	// ---- Delivery: WebSocket Hub ----
	wsHub := websocket.NewHub(chatUC, log)
	go wsHub.Run(ctx)

	// ---- Delivery: HTTP Handlers ----
	analysisHandler := handlers.NewAnalysisHandler(analysisUC, wsHub, log)
	graphHandler := handlers.NewGraphHandler(graphUC, chatUC, log)
	healthHandler := handlers.NewHealthHandler(
		neo4jClient,
		redisClient,
		func() bool {
			if aiClient == nil {
				return false
			}
			return aiClient.IsHealthy()
		},
		log,
	)

	// ---- HTTP Router ----
	router := deliveryhttp.NewRouter(
		analysisHandler,
		graphHandler,
		healthHandler,
		wsHub,
		log,
		cfg.App.Env,
	)

	// ---- HTTP Server ----
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router.Handler(),
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}

	// Start server
	go func() {
		log.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// ---- Graceful shutdown ----
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutdown signal received, draining connections...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server forced shutdown", zap.Error(err))
	}

	cancel() // stop background goroutines

	if aiClient != nil {
		if err := aiClient.Close(); err != nil {
			log.Warn("gRPC client close error", zap.Error(err))
		}
	}

	log.Info("ais-back stopped gracefully")
}
