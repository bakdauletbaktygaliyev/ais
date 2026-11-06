package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListProjects(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := h.db.Query(
		`SELECT id, url, name, status, error_msg, created_at
		 FROM projects WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	projects := []gin.H{}
	for rows.Next() {
		var id, url, name, status, errMsg string
		var createdAt time.Time
		rows.Scan(&id, &url, &name, &status, &errMsg, &createdAt)
		projects = append(projects, gin.H{
			"id": id, "url": url, "name": name,
			"status": status, "error": errMsg, "created_at": createdAt,
		})
	}
	c.JSON(http.StatusOK, projects)
}

func (h *Handler) GetProject(c *gin.Context) {
	userID := c.GetString("user_id")
	row := h.db.QueryRow(
		`SELECT id, url, name, status, error_msg, created_at
		 FROM projects WHERE id = $1 AND user_id = $2`, c.Param("id"), userID,
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

func (h *Handler) DeleteProject(c *gin.Context) {
	userID := c.GetString("user_id")
	h.db.Exec(`DELETE FROM projects WHERE id = $1 AND user_id = $2`, c.Param("id"), userID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
