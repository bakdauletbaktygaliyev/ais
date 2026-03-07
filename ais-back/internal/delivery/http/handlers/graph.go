package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	appgraph "github.com/bakdaulet/ais/ais-back/internal/application/graph"
	appchat "github.com/bakdaulet/ais/ais-back/internal/application/chat"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

// GraphHandler handles HTTP requests for graph navigation and querying.
type GraphHandler struct {
	graphUC *appgraph.UseCase
	chatUC  *appchat.UseCase
	log     *logger.Logger
}

// NewGraphHandler creates a new GraphHandler.
func NewGraphHandler(
	graphUC *appgraph.UseCase,
	chatUC *appchat.UseCase,
	log *logger.Logger,
) *GraphHandler {
	return &GraphHandler{
		graphUC: graphUC,
		chatUC:  chatUC,
		log:     log.WithComponent("graph_handler"),
	}
}

// GetGraph godoc
// GET /api/v1/repos/:id/graph
// Returns root-level graph nodes and edges.
func (h *GraphHandler) GetGraph(c *gin.Context) {
	repoID := c.Param("id")
	if repoID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "repo id is required"))
		return
	}

	view, err := h.graphUC.GetRootView(c.Request.Context(), repoID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes":  view.Nodes,
		"edges":  view.Edges,
		"parent": view.Parent,
	})
}

// GetNodeChildren godoc
// GET /api/v1/repos/:id/nodes/:nodeId
// Returns direct children of a node for drill-down navigation.
func (h *GraphHandler) GetNodeChildren(c *gin.Context) {
	nodeID := c.Param("nodeId")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "nodeId is required"))
		return
	}

	view, err := h.graphUC.GetChildrenView(c.Request.Context(), nodeID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes":  view.Nodes,
		"edges":  view.Edges,
		"parent": view.Parent,
	})
}

// GetNodeDetail godoc
// GET /api/v1/repos/:id/nodes/:nodeId/detail
// Returns comprehensive detail for the detail panel.
func (h *GraphHandler) GetNodeDetail(c *gin.Context) {
	nodeID := c.Param("nodeId")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "nodeId is required"))
		return
	}

	detail, err := h.graphUC.GetNodeDetail(c.Request.Context(), nodeID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, detail)
}

// GetFileSource godoc
// GET /api/v1/repos/:id/files/:fileId
// Returns the raw source code of a file node.
func (h *GraphHandler) GetFileSource(c *gin.Context) {
	fileID := c.Param("fileId")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "fileId is required"))
		return
	}

	source, err := h.graphUC.GetFileSource(c.Request.Context(), fileID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"source":  source,
		"fileId":  fileID,
	})
}

// GetCycles godoc
// GET /api/v1/repos/:id/cycles
// Returns all detected import cycles.
func (h *GraphHandler) GetCycles(c *gin.Context) {
	repoID := c.Param("id")
	if repoID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "repo id is required"))
		return
	}

	cycles, err := h.graphUC.GetCycles(c.Request.Context(), repoID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cycles": cycles,
		"count":  len(cycles),
	})
}

// GetMetrics godoc
// GET /api/v1/repos/:id/metrics
func (h *GraphHandler) GetMetrics(c *gin.Context) {
	repoID := c.Param("id")
	if repoID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "repo id is required"))
		return
	}

	metrics, err := h.graphUC.GetMetrics(c.Request.Context(), repoID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// GetTopFanIn godoc
// GET /api/v1/repos/:id/top-fan-in?limit=10
func (h *GraphHandler) GetTopFanIn(c *gin.Context) {
	repoID := c.Param("id")
	limit := parseIntParam(c, "limit", 10)

	nodes, err := h.graphUC.GetTopFanIn(c.Request.Context(), repoID, limit)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// GetTopFanOut godoc
// GET /api/v1/repos/:id/top-fan-out?limit=10
func (h *GraphHandler) GetTopFanOut(c *gin.Context) {
	repoID := c.Param("id")
	limit := parseIntParam(c, "limit", 10)

	nodes, err := h.graphUC.GetTopFanOut(c.Request.Context(), repoID, limit)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// GetShortestPath godoc
// GET /api/v1/repos/:id/path?from=nodeId&to=nodeId
func (h *GraphHandler) GetShortestPath(c *gin.Context) {
	fromID := c.Query("from")
	toID := c.Query("to")

	if fromID == "" || toID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "from and to query params are required"))
		return
	}

	result, err := h.graphUC.GetShortestPath(c.Request.Context(), fromID, toID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// SearchSimilar godoc
// GET /api/v1/repos/:id/search?q=query&topK=5
func (h *GraphHandler) SearchSimilar(c *gin.Context) {
	repoID := c.Param("id")
	query := c.Query("q")
	topK := parseIntParam(c, "topK", 5)

	if query == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "q query param is required"))
		return
	}

	results, err := h.chatUC.SearchSimilar(c.Request.Context(), repoID, query, topK)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func errorResponse(code, message string) gin.H {
	return gin.H{
		"error":   message,
		"code":    code,
	}
}

func respondError(c *gin.Context, err error) {
	status := apperrors.HTTPStatus(err)
	if de, ok := err.(*apperrors.DomainError); ok {
		c.JSON(status, errorResponse(string(de.Code), de.Message))
		return
	}
	c.JSON(status, errorResponse("INTERNAL_ERROR", "an unexpected error occurred"))
}

func parseIntParam(c *gin.Context, key string, defaultVal int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}
