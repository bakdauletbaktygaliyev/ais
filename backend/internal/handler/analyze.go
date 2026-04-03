package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ais/backend/internal/parser"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type analyzeRequest struct {
	URL string `json:"url" binding:"required"`
}

func (h *Handler) Analyze(c *gin.Context) {
	var req analyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	repoURL := normalizeURL(req.URL)
	name := extractRepoName(repoURL)
	id := uuid.New().String()

	if _, err := h.db.Exec(
		`INSERT INTO projects (id, url, name, status) VALUES ($1, $2, $3, 'pending')`,
		id, repoURL, name,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	go h.runAnalysis(id, repoURL)

	c.JSON(http.StatusAccepted, gin.H{"id": id, "status": "pending", "name": name})
}

func (h *Handler) runAnalysis(id, repoURL string) {
	h.updateStatus(id, "analyzing", "")

	graph, fileTree, err := parser.ParseRepo(repoURL, h.cloneDir)
	if err != nil {
		h.updateStatus(id, "error", err.Error())
		return
	}

	graphJSON, _ := json.Marshal(graph)
	treeJSON, _ := json.Marshal(fileTree)

	h.db.Exec(
		`UPDATE projects SET status='done', graph_data=$1, file_tree=$2, updated_at=NOW() WHERE id=$3`,
		string(graphJSON), string(treeJSON), id,
	)
}
