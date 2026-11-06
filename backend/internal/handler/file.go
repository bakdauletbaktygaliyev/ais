package handler

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetFile(c *gin.Context) {
	id := c.Param("id")
	filePath := strings.TrimPrefix(c.Query("path"), "/")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	userID := c.GetString("user_id")

	// Verify project belongs to this user
	var ownerCheck int
	if err := h.db.QueryRow(
		`SELECT 1 FROM projects WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&ownerCheck); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var content string
	err := h.db.QueryRow(
		`SELECT content FROM file_contents WHERE project_id = $1 AND path = $2`, id, filePath,
	).Scan(&content)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "file content not available (re-analyze the repository to enable file preview)"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"path": filePath, "content": content})
}
