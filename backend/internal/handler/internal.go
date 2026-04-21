package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ais/backend/internal/parser"
	"github.com/gin-gonic/gin"
)

// InternalGetProject returns a project by ID without user_id filtering.
// Used for service-to-service calls (e.g. ai-service).
func (h *Handler) InternalGetProject(c *gin.Context) {
	row := h.db.QueryRow(
		`SELECT id, url, name, status, error_msg, created_at
		 FROM projects WHERE id = $1`, c.Param("id"),
	)
	var pid, url, name, status, errMsg string
	var createdAt time.Time
	if err := row.Scan(&pid, &url, &name, &status, &errMsg, &createdAt); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": pid, "url": url, "name": name,
		"status": status, "error": errMsg, "created_at": createdAt,
	})
}

// InternalGetGraph returns the graph for a project by ID without user_id filtering.
// Used for service-to-service calls (e.g. ai-service).
func (h *Handler) InternalGetGraph(c *gin.Context) {
	row := h.db.QueryRow(
		`SELECT graph_data FROM projects WHERE id = $1`, c.Param("id"),
	)

	var raw sql.NullString
	if err := row.Scan(&raw); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if !raw.Valid || raw.String == "" {
		c.JSON(http.StatusOK, parser.GraphData{Nodes: []parser.Node{}, Edges: []parser.Edge{}})
		return
	}

	var graph parser.GraphData
	if err := json.Unmarshal([]byte(raw.String), &graph); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "corrupt graph data"})
		return
	}

	c.JSON(http.StatusOK, filterGraph(graph, c.Query("path")))
}
