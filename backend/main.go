package main

import (
	"log"
	"os"

	"github.com/ais/backend/internal/db"
	"github.com/ais/backend/internal/handler"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	database, err := db.Connect(getEnv("DATABASE_URL", "postgres://ais:ais_secret@localhost:5432/ais?sslmode=disable"))
	if err != nil {
		log.Fatal("cannot connect to DB:", err)
	}
	if err := db.Init(database); err != nil {
		log.Fatal("cannot init DB:", err)
	}

	cloneDir := getEnv("CLONE_DIR", "/tmp/repos")
	os.MkdirAll(cloneDir, 0755)

	h := handler.New(database, cloneDir)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}))

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	api := r.Group("/api")
	{
		api.POST("/analyze", h.Analyze)
		api.GET("/projects", h.ListProjects)
		api.GET("/projects/:id", h.GetProject)
		api.GET("/projects/:id/graph", h.GetGraph)
		api.DELETE("/projects/:id", h.DeleteProject)
	}

	r.Run(":" + getEnv("PORT", "8080"))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
