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

	userID := c.GetString("user_id")

	var exists int
	if err := h.db.QueryRow(`SELECT 1 FROM users WHERE id = $1`, userID).Scan(&exists); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found, please log in again"})
		return
	}

	repoURL := normalizeURL(req.URL)
	name := extractRepoName(repoURL)
	id := uuid.New().String()

	if _, err := h.db.Exec(
		`INSERT INTO projects (id, user_id, url, name, status) VALUES ($1, $2, $3, $4, 'pending')`,
		id, userID, repoURL, name,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	go h.runAnalysis(id, repoURL)
	c.JSON(http.StatusAccepted, gin.H{"id": id, "status": "pending", "name": name})
}

func (h *Handler) runAnalysis(id, repoURL string) {
	h.updateStatus(id, "analyzing", "")

	graph, fileTree, contents, err := parser.ParseRepo(repoURL, h.cloneDir)
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

	// Store file contents in bulk
	for path, content := range contents {
		h.db.Exec(
			`INSERT INTO file_contents (project_id, path, content) VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING`,
			id, path, content,
		)
	}
}
