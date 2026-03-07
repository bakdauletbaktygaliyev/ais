package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bakdaulet/ais/ais-back/internal/delivery/http/handlers"
	"github.com/bakdaulet/ais/ais-back/internal/delivery/http/middleware"
	ws "github.com/bakdaulet/ais/ais-back/internal/delivery/websocket"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

// Router configures and returns a Gin HTTP router.
type Router struct {
	engine           *gin.Engine
	analysisHandler  *handlers.AnalysisHandler
	graphHandler     *handlers.GraphHandler
	healthHandler    *handlers.HealthHandler
	wsHub            *ws.Hub
	log              *logger.Logger
}

// NewRouter constructs the Gin router with all middleware and routes registered.
func NewRouter(
	analysisHandler *handlers.AnalysisHandler,
	graphHandler *handlers.GraphHandler,
	healthHandler *handlers.HealthHandler,
	wsHub *ws.Hub,
	log *logger.Logger,
	env string,
) *Router {
	if env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// Global middleware
	engine.Use(middleware.Recovery(log))
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Logger(log))
	engine.Use(middleware.CORS())

	r := &Router{
		engine:          engine,
		analysisHandler: analysisHandler,
		graphHandler:    graphHandler,
		healthHandler:   healthHandler,
		wsHub:           wsHub,
		log:             log,
	}

	r.registerRoutes()
	return r
}

// Handler returns the underlying http.Handler for use with net/http Server.
func (r *Router) Handler() http.Handler {
	return r.engine
}

func (r *Router) registerRoutes() {
	e := r.engine

	// Health probes (no auth required)
	e.GET("/health", r.healthHandler.Liveness)
	e.GET("/health/ready", r.healthHandler.Readiness)

	// WebSocket endpoint
	e.GET("/ws", func(c *gin.Context) {
		r.wsHub.ServeHTTP(c.Writer, c.Request)
	})

	// API v1
	api := e.Group("/api/v1")
	{
		// Repository analysis
		repos := api.Group("/repos")
		{
			repos.POST("", r.analysisHandler.SubmitRepo)
			repos.GET("/:id", r.analysisHandler.GetRepo)

			// Graph navigation
			repos.GET("/:id/graph", r.graphHandler.GetGraph)
			repos.GET("/:id/metrics", r.graphHandler.GetMetrics)
			repos.GET("/:id/cycles", r.graphHandler.GetCycles)
			repos.GET("/:id/path", r.graphHandler.GetShortestPath)
			repos.GET("/:id/search", r.graphHandler.SearchSimilar)
			repos.GET("/:id/top-fan-in", r.graphHandler.GetTopFanIn)
			repos.GET("/:id/top-fan-out", r.graphHandler.GetTopFanOut)

			// Node operations
			repos.GET("/:id/nodes/:nodeId", r.graphHandler.GetNodeChildren)
			repos.GET("/:id/nodes/:nodeId/detail", r.graphHandler.GetNodeDetail)

			// File source
			repos.GET("/:id/files/:fileId", r.graphHandler.GetFileSource)
		}
	}

	// 404 handler
	e.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "route not found",
			"code":  "NOT_FOUND",
		})
	})
}
