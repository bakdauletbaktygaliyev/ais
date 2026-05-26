package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/ais/backend/internal/parser"
	"github.com/gin-gonic/gin"
)

type DeadCodeFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
}

// entryPointPatterns are files that legitimately have no importers.
var entryPointPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^main\.(go|py|ts|js|rs|java|kt|c|cpp)$`),
	regexp.MustCompile(`(?i)^index\.(ts|js|tsx|jsx|html)$`),
	regexp.MustCompile(`(?i)^app\.(ts|js|tsx|jsx|py)$`),
	regexp.MustCompile(`(?i)^App\.(tsx|ts|jsx|js)$`),
	regexp.MustCompile(`(?i)\.(test|spec)\.(ts|js|tsx|jsx|go|py)$`),
	regexp.MustCompile(`(?i)_test\.(go|py)$`),
	regexp.MustCompile(`(?i)\.config\.(ts|js|mjs|cjs)$`),
	regexp.MustCompile(`^__init__\.py$`),
	regexp.MustCompile(`^mod\.rs$`),
	regexp.MustCompile(`^lib\.rs$`),
}

func isEntryPoint(name string) bool {
	for _, p := range entryPointPatterns {
		if p.MatchString(name) {
			return true
		}
	}
	return false
}

func computeDeadCode(graph parser.GraphData) []DeadCodeFile {
	hasIncoming := make(map[string]bool, len(graph.Edges))
	for _, e := range graph.Edges {
		hasIncoming[e.Target] = true
	}

	var dead []DeadCodeFile
	for _, n := range graph.Nodes {
		if n.Type != "file" || hasIncoming[n.ID] || isEntryPoint(n.Name) {
			continue
		}
		dead = append(dead, DeadCodeFile{
			ID:       n.ID,
			Name:     n.Name,
			Path:     n.Path,
			Language: n.Language,
			Lines:    n.Lines,
		})
	}
	return dead
}

func (h *Handler) GetDeadCode(c *gin.Context) {
	userID := c.GetString("user_id")
	row := h.db.QueryRow(
		`SELECT graph_data FROM projects WHERE id = $1 AND user_id = $2`,
		c.Param("id"), userID,
	)

	var raw sql.NullString
	if err := row.Scan(&raw); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if !raw.Valid || raw.String == "" {
		c.JSON(http.StatusOK, []DeadCodeFile{})
		return
	}

	var graph parser.GraphData
	if err := json.Unmarshal([]byte(raw.String), &graph); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "corrupt graph data"})
		return
	}

	dead := computeDeadCode(graph)
	if dead == nil {
		dead = []DeadCodeFile{}
	}
	c.JSON(http.StatusOK, dead)
}
