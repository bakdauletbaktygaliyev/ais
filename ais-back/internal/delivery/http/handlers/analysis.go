package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	appanalysis "github.com/bakdaulet/ais/ais-back/internal/application/analysis"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
	"github.com/bakdaulet/ais/ais-back/internal/delivery/websocket"
)

// AnalysisHandler handles HTTP requests related to repository analysis.
type AnalysisHandler struct {
	analysisUC *appanalysis.UseCase
	hub        *websocket.Hub
	log        *logger.Logger
}

// NewAnalysisHandler creates a new AnalysisHandler.
func NewAnalysisHandler(
	analysisUC *appanalysis.UseCase,
	hub *websocket.Hub,
	log *logger.Logger,
) *AnalysisHandler {
	return &AnalysisHandler{
		analysisUC: analysisUC,
		hub:        hub,
		log:        log.WithComponent("analysis_handler"),
	}
}

// SubmitRepo godoc
// POST /api/v1/repos
// Body: { "url": "https://github.com/owner/repo" }
func (h *AnalysisHandler) SubmitRepo(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "url is required"))
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if !strings.HasPrefix(req.URL, "https://github.com/") &&
		!strings.HasPrefix(req.URL, "http://github.com/") {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "only GitHub repositories are supported"))
		return
	}

	h.log.Info("repo submission received", zap.String("url", req.URL))

	repo, err := h.analysisUC.StartAnalysis(c.Request.Context(), req.URL, h.hub)
	if err != nil {
		status := apperrors.HTTPStatus(err)
		var code, msg string
		if de, ok := err.(*apperrors.DomainError); ok {
			code = string(de.Code)
			msg = de.Message
		} else {
			code = "INTERNAL_ERROR"
			msg = "failed to start analysis"
		}
		c.JSON(status, errorResponse(code, msg))
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"id":     repo.ID,
		"url":    repo.URL,
		"status": string(repo.Status),
	})
}

// GetRepo godoc
// GET /api/v1/repos/:id
func (h *AnalysisHandler) GetRepo(c *gin.Context) {
	repoID := c.Param("id")
	if repoID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("INVALID_INPUT", "repo id is required"))
		return
	}

	repo, err := h.analysisUC.GetRepo(c.Request.Context(), repoID)
	if err != nil {
		status := apperrors.HTTPStatus(err)
		var code, msg string
		if de, ok := err.(*apperrors.DomainError); ok {
			code = string(de.Code)
			msg = de.Message
		} else {
			code = "INTERNAL_ERROR"
			msg = "failed to get repo"
		}
		c.JSON(status, errorResponse(code, msg))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            repo.ID,
		"url":           repo.URL,
		"owner":         repo.Owner,
		"name":          repo.Name,
		"description":   repo.Description,
		"status":        string(repo.Status),
		"language":      string(repo.Language),
		"monorepoType":  string(repo.MonorepoType),
		"commitHash":    repo.CommitHash,
		"fileCount":     repo.FileCount,
		"dirCount":      repo.DirCount,
		"functionCount": repo.FunctionCount,
		"classCount":    repo.ClassCount,
		"cycleCount":    repo.CycleCount,
		"starCount":     repo.StarCount,
		"forkCount":     repo.ForkCount,
		"sizeKB":        repo.SizeKB,
		"errorMessage":  repo.ErrorMessage,
		"createdAt":     repo.CreatedAt,
		"updatedAt":     repo.UpdatedAt,
		"readyAt":       repo.ReadyAt,
	})
}
