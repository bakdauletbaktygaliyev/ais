package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sql.DB, error) {
	var database *sql.DB
	var err error
	for i := 0; i < 10; i++ {
		database, err = sql.Open("postgres", dsn)
		if err == nil {
			if err = database.Ping(); err == nil {
				return database, nil
			}
		}
		log.Printf("DB not ready, retrying (%d/10)...", i+1)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}

func Init(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id          UUID PRIMARY KEY,
			url         TEXT NOT NULL,
			name        TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'pending',
			error_msg   TEXT DEFAULT '',
			graph_data  JSONB,
			file_tree   JSONB,
			created_at  TIMESTAMP DEFAULT NOW(),
			updated_at  TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}
