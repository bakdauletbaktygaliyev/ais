package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

// Pinger is implemented by infrastructure clients that support health checks.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles liveness and readiness probes.
type HealthHandler struct {
	neo4jPinger  Pinger
	redisPinger  Pinger
	grpcChecker  func() bool
	log          *logger.Logger
	startedAt    time.Time
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(
	neo4jPinger Pinger,
	redisPinger Pinger,
	grpcChecker func() bool,
	log *logger.Logger,
) *HealthHandler {
	return &HealthHandler{
		neo4jPinger: neo4jPinger,
		redisPinger: redisPinger,
		grpcChecker: grpcChecker,
		log:         log.WithComponent("health_handler"),
		startedAt:   time.Now(),
	}
}

// Liveness godoc
// GET /health
// Returns 200 if the service is alive.
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "ais-back",
		"timestamp": time.Now().UTC(),
		"uptime":    time.Since(h.startedAt).String(),
	})
}

// Readiness godoc
// GET /health/ready
// Returns 200 only if all infrastructure dependencies are reachable.
func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := map[string]string{}
	allHealthy := true

	// Neo4j
	if err := h.neo4jPinger.Ping(ctx); err != nil {
		checks["neo4j"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		checks["neo4j"] = "healthy"
	}

	// Redis
	if err := h.redisPinger.Ping(ctx); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		checks["redis"] = "healthy"
	}

	// gRPC AI service
	if h.grpcChecker != nil {
		if h.grpcChecker() {
			checks["ais-ai"] = "healthy"
		} else {
			checks["ais-ai"] = "unhealthy"
			// AI service not critical for basic operation
		}
	}

	status := http.StatusOK
	statusStr := "ready"
	if !allHealthy {
		status = http.StatusServiceUnavailable
		statusStr = "not ready"
	}

	c.JSON(status, gin.H{
		"status":    statusStr,
		"checks":    checks,
		"timestamp": time.Now().UTC(),
	})
}
