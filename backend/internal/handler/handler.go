package handler

import "database/sql"

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	db       *sql.DB
	cloneDir string
}

func New(db *sql.DB, cloneDir string) *Handler {
	return &Handler{db: db, cloneDir: cloneDir}
}

func (h *Handler) updateStatus(id, status, errMsg string) {
	h.db.Exec(
		`UPDATE projects SET status=$1, error_msg=$2, updated_at=NOW() WHERE id=$3`,
		status, errMsg, id,
	)
}
