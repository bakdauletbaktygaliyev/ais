package main

import (
	"log"
	"os"

	"github.com/ais/backend/internal/db"
	"github.com/ais/backend/internal/handler"
	"github.com/ais/backend/internal/middleware"
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

	// Public auth routes — no JWT required
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/verify", h.Verify)
		auth.POST("/login", h.Login)
	}

	// Internal routes — for service-to-service calls (no JWT)
	internal := r.Group("/internal")
	{
		internal.GET("/projects/:id", h.InternalGetProject)
		internal.GET("/projects/:id/graph", h.InternalGetGraph)
	}

	// Protected API routes — JWT required
	api := r.Group("/api")
	api.Use(middleware.RequireAuth())
	{
		api.POST("/analyze", h.Analyze)
		api.GET("/projects", h.ListProjects)
		api.GET("/projects/:id", h.GetProject)
		api.GET("/projects/:id/graph", h.GetGraph)
		api.GET("/projects/:id/file", h.GetFile)
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
