# Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add email+password auth with JWT tokens; isolate each user's projects so they see only their own.

**Architecture:** Auth lives entirely in the Go backend — two new public endpoints (`/api/auth/register`, `/api/auth/login`) generate JWTs; a Gin middleware validates tokens and injects `user_id` into the request context; all project handlers filter by that `user_id`. The Angular frontend stores the token in `localStorage`, attaches it via an HTTP interceptor, and guards routes with `authGuard`.

**Tech Stack:** Go/Gin, `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, Angular 17 standalone components, Angular functional interceptors/guards.

---

## File Map

**Backend — new files:**
- `backend/internal/db/users.go` — `CreateUser`, `GetUserByEmail`
- `backend/internal/handler/auth.go` — `Register`, `Login` handlers
- `backend/internal/middleware/auth.go` — `AuthMiddleware`, `ValidateToken`, `Claims`
- `backend/internal/middleware/auth_test.go` — unit tests for `ValidateToken`

**Backend — modified files:**
- `backend/go.mod` — add `github.com/golang-jwt/jwt/v5`
- `backend/internal/db/db.go` — create `users` table; add `user_id` column to `projects`
- `backend/internal/handler/handler.go` — add `jwtSecret` field; update `New()`
- `backend/main.go` — add auth routes; apply `AuthMiddleware` to protected group; pass `JWT_SECRET` env
- `backend/internal/handler/analyze.go` — read `user_id` from Gin context; pass to `INSERT`
- `backend/internal/handler/project.go` — filter all queries by `user_id`
- `backend/internal/handler/graph.go` — filter `graph_data` query by `user_id`
- `backend/docker-compose.yml` → `docker-compose.yml` — add `JWT_SECRET` env to backend service

**Frontend — new files:**
- `frontend/src/app/models/auth.model.ts` — `AuthResponse`, `LoginRequest`, `RegisterRequest`
- `frontend/src/app/services/auth.service.ts` — `AuthService`
- `frontend/src/app/guards/auth.guard.ts` — `authGuard` (functional)
- `frontend/src/app/interceptors/auth.interceptor.ts` — `authInterceptor` (functional)
- `frontend/src/app/pages/auth/login/login.component.ts`
- `frontend/src/app/pages/auth/login/login.component.html`
- `frontend/src/app/pages/auth/login/login.component.css`
- `frontend/src/app/pages/auth/register/register.component.ts`
- `frontend/src/app/pages/auth/register/register.component.html`
- `frontend/src/app/pages/auth/register/register.component.css`

**Frontend — modified files:**
- `frontend/src/app/app.routes.ts` — add `/login`, `/register`; apply `authGuard`
- `frontend/src/app/app.config.ts` — register interceptor with `withInterceptors`
- `frontend/src/app/pages/home/home.component.ts` — inject `AuthService`; add `logout()`
- `frontend/src/app/pages/home/home.component.html` — add logout button

---

## Task 1: Add JWT dependency to Go module

**Files:**
- Modify: `backend/go.mod`

- [ ] **Step 1: Add the JWT package**

```bash
cd backend
go get github.com/golang-jwt/jwt/v5
```

Expected output: line added to `go.mod` like `github.com/golang-jwt/jwt/v5 v5.x.x`

- [ ] **Step 2: Tidy module**

```bash
go mod tidy
```

Expected: no errors.

- [ ] **Step 3: Compile check**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add golang-jwt/jwt/v5 dependency"
```

---

## Task 2: Update DB schema

**Files:**
- Modify: `backend/internal/db/db.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/db/db_test.go`:

```go
package db_test

import (
	"os"
	"testing"

	"github.com/ais/backend/internal/db"
)

func TestInit_CreatesUsersTable(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	database, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Init(database); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	// verify users table exists
	var exists bool
	err = database.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='users')`,
	).Scan(&exists)
	if err != nil || !exists {
		t.Fatal("users table not found after Init")
	}
	// verify user_id column exists in projects
	err = database.QueryRow(
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name='projects' AND column_name='user_id'
		)`,
	).Scan(&exists)
	if err != nil || !exists {
		t.Fatal("user_id column not found in projects after Init")
	}
}
```

- [ ] **Step 2: Run test to confirm skip (no DB in unit environment)**

```bash
cd backend
go test ./internal/db/... -v -run TestInit_CreatesUsersTable
```

Expected: `SKIP` (DATABASE_URL not set) or FAIL if DB is reachable but table missing.

- [ ] **Step 3: Update `db.go` to create users table and add user_id column**

Replace the entire `Init` function in `backend/internal/db/db.go`:

```go
func Init(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email         TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at    TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id          UUID PRIMARY KEY,
			url         TEXT NOT NULL,
			name        TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'pending',
			error_msg   TEXT DEFAULT '',
			graph_data  JSONB,
			file_tree   JSONB,
			created_at  TIMESTAMP DEFAULT NOW(),
			updated_at  TIMESTAMP DEFAULT NOW(),
			user_id     UUID REFERENCES users(id)
		)
	`)
	if err != nil {
		return err
	}
	// Migrate existing installations that lack user_id column
	_, err = database.Exec(`
		ALTER TABLE projects ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)
	`)
	return err
}
```

- [ ] **Step 4: Compile check**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat: add users table and user_id column to projects"
```

---

## Task 3: DB layer — users.go

**Files:**
- Create: `backend/internal/db/users.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/db/users_test.go`:

```go
package db_test

import (
	"os"
	"testing"

	"github.com/ais/backend/internal/db"
)

func TestCreateAndGetUser(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	database, _ := db.Connect(dsn)
	db.Init(database)

	email := "test_user_unique@example.com"
	// cleanup before and after
	database.Exec(`DELETE FROM users WHERE email=$1`, email)
	defer database.Exec(`DELETE FROM users WHERE email=$1`, email)

	id, err := db.CreateUser(database, email, "hashed_pw")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	u, err := db.GetUserByEmail(database, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if u.Email != email {
		t.Fatalf("got email %q, want %q", u.Email, email)
	}
	if u.PasswordHash != "hashed_pw" {
		t.Fatal("password_hash mismatch")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd backend && go test ./internal/db/... -v -run TestCreateAndGetUser
```

Expected: compilation error (`db.CreateUser` undefined) or SKIP if no DB.

- [ ] **Step 3: Create `backend/internal/db/users.go`**

```go
package db

import "database/sql"

type User struct {
	ID           string
	Email        string
	PasswordHash string
}

func CreateUser(database *sql.DB, email, passwordHash string) (string, error) {
	var id string
	err := database.QueryRow(
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, passwordHash,
	).Scan(&id)
	return id, err
}

func GetUserByEmail(database *sql.DB, email string) (*User, error) {
	u := &User{}
	err := database.QueryRow(
		`SELECT id, email, password_hash FROM users WHERE email=$1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash)
	if err != nil {
		return nil, err
	}
	return u, nil
}
```

- [ ] **Step 4: Compile check**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/db/users.go internal/db/users_test.go
git commit -m "feat: add CreateUser and GetUserByEmail db functions"
```

---

## Task 4: JWT middleware

**Files:**
- Create: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/auth_test.go`

- [ ] **Step 1: Write failing unit tests**

Create `backend/internal/middleware/auth_test.go`:

```go
package middleware_test

import (
	"testing"
	"time"

	"github.com/ais/backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test_secret_key"

func TestValidateToken_Valid(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.Claims{
		UserID: "user-123",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, _ := token.SignedString([]byte(testSecret))

	claims, err := middleware.ValidateToken(signed, testSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Fatalf("got UserID %q, want %q", claims.UserID, "user-123")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.Claims{
		UserID: "user-123",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})
	signed, _ := token.SignedString([]byte(testSecret))

	_, err := middleware.ValidateToken(signed, testSecret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.Claims{
		UserID: "user-123",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, _ := token.SignedString([]byte("other_secret"))

	_, err := middleware.ValidateToken(signed, testSecret)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
cd backend && go test ./internal/middleware/... -v
```

Expected: compilation error (`middleware` package doesn't exist yet).

- [ ] **Step 3: Create `backend/internal/middleware/auth.go`**

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func ValidateToken(tokenStr, secret string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	return claims, err
}

func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		claims, err := ValidateToken(strings.TrimPrefix(header, "Bearer "), secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}
```

- [ ] **Step 4: Run tests to confirm pass**

```bash
cd backend && go test ./internal/middleware/... -v
```

Expected: all three tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/middleware/auth.go internal/middleware/auth_test.go
git commit -m "feat: add JWT middleware with ValidateToken"
```

---

## Task 5: Auth handlers (register, login)

**Files:**
- Create: `backend/internal/handler/auth.go`

- [ ] **Step 1: Create `backend/internal/handler/auth.go`**

```go
package handler

import (
	"net/http"
	"time"

	"github.com/ais/backend/internal/db"
	"github.com/ais/backend/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	userID, err := db.CreateUser(h.db, req.Email, string(hash))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	token, err := h.generateToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"token": token, "user_id": userID})
}

func (h *Handler) Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
		return
	}

	user, err := db.GetUserByEmail(h.db, req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "user_id": user.ID})
}

func (h *Handler) generateToken(userID string) (string, error) {
	claims := middleware.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
```

- [ ] **Step 2: Compile check**

```bash
cd backend && go build ./...
```

Expected: compilation error — `h.jwtSecret` and `h.db` (jwtSecret field doesn't exist yet on Handler). That's fine; fix in next task.

- [ ] **Step 3: Commit stash**

Skip commit until Handler is updated (Task 6).

---

## Task 6: Update Handler struct and main.go routes

**Files:**
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/main.go`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Update `backend/internal/handler/handler.go`**

Replace the entire file:

```go
package handler

import "database/sql"

type Handler struct {
	db        *sql.DB
	cloneDir  string
	jwtSecret string
}

func New(db *sql.DB, cloneDir, jwtSecret string) *Handler {
	return &Handler{db: db, cloneDir: cloneDir, jwtSecret: jwtSecret}
}

func (h *Handler) updateStatus(id, status, errMsg string) {
	h.db.Exec(
		`UPDATE projects SET status=$1, error_msg=$2, updated_at=NOW() WHERE id=$3`,
		status, errMsg, id,
	)
}
```

- [ ] **Step 2: Update `backend/main.go`**

Replace the entire file:

```go
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

	jwtSecret := getEnv("JWT_SECRET", "change_me_in_production")
	h := handler.New(database, cloneDir, jwtSecret)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}))

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	auth := r.Group("/api/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
	}

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(jwtSecret))
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
```

- [ ] **Step 3: Add JWT_SECRET to docker-compose.yml**

In `docker-compose.yml`, under `backend.environment`, add:

```yaml
      JWT_SECRET: ${JWT_SECRET:-change_me_in_production}
```

The `backend.environment` block should look like:

```yaml
    environment:
      DATABASE_URL: postgres://ais:ais_secret@postgres:5432/ais?sslmode=disable
      CLONE_DIR: /tmp/repos
      PORT: 8080
      JWT_SECRET: ${JWT_SECRET:-change_me_in_production}
```

- [ ] **Step 4: Compile check**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/handler.go internal/handler/auth.go main.go ../docker-compose.yml
git commit -m "feat: add register/login endpoints and JWT middleware wiring"
```

---

## Task 7: Filter project handlers by user_id

**Files:**
- Modify: `backend/internal/handler/project.go`
- Modify: `backend/internal/handler/analyze.go`
- Modify: `backend/internal/handler/graph.go`

- [ ] **Step 1: Update `backend/internal/handler/project.go`**

Replace the entire file:

```go
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListProjects(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := h.db.Query(
		`SELECT id, url, name, status, error_msg, created_at FROM projects WHERE user_id=$1 ORDER BY created_at DESC`,
		userID,
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
	id := c.Param("id")
	row := h.db.QueryRow(
		`SELECT id, url, name, status, error_msg, created_at FROM projects WHERE id=$1 AND user_id=$2`,
		id, userID,
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
	h.db.Exec(`DELETE FROM projects WHERE id=$1 AND user_id=$2`, c.Param("id"), userID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

- [ ] **Step 2: Update `backend/internal/handler/analyze.go`**

Replace the `Analyze` method (the `runAnalysis` method stays the same):

```go
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
	repoURL := normalizeURL(req.URL)
	name := extractRepoName(repoURL)
	id := uuid.New().String()

	if _, err := h.db.Exec(
		`INSERT INTO projects (id, url, name, status, user_id) VALUES ($1, $2, $3, 'pending', $4)`,
		id, repoURL, name, userID,
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
```

- [ ] **Step 3: Update `backend/internal/handler/graph.go`**

Change only the first query in `GetGraph` to also filter by `user_id`. Replace the `GetGraph` function:

```go
func (h *Handler) GetGraph(c *gin.Context) {
	userID := c.GetString("user_id")
	row := h.db.QueryRow(
		`SELECT graph_data FROM projects WHERE id=$1 AND user_id=$2`,
		c.Param("id"), userID,
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
```

- [ ] **Step 4: Compile check**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 5: Run all tests**

```bash
cd backend && go test ./...
```

Expected: middleware tests PASS; db tests SKIP (no DATABASE_URL).

- [ ] **Step 6: Commit**

```bash
git add internal/handler/project.go internal/handler/analyze.go internal/handler/graph.go
git commit -m "feat: filter all project queries by user_id"
```

---

## Task 8: Frontend auth model and service

**Files:**
- Create: `frontend/src/app/models/auth.model.ts`
- Create: `frontend/src/app/services/auth.service.ts`

- [ ] **Step 1: Create `frontend/src/app/models/auth.model.ts`**

```typescript
export interface AuthResponse {
  token: string;
  user_id: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
}
```

- [ ] **Step 2: Create `frontend/src/app/services/auth.service.ts`**

```typescript
import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, tap } from 'rxjs';
import { AuthResponse } from '../models/auth.model';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly TOKEN_KEY = 'ais_token';
  private readonly USER_KEY = 'ais_user_id';
  private readonly api = '/api/auth';

  constructor(private http: HttpClient) {}

  register(email: string, password: string): Observable<AuthResponse> {
    return this.http.post<AuthResponse>(`${this.api}/register`, { email, password }).pipe(
      tap(res => this.store(res))
    );
  }

  login(email: string, password: string): Observable<AuthResponse> {
    return this.http.post<AuthResponse>(`${this.api}/login`, { email, password }).pipe(
      tap(res => this.store(res))
    );
  }

  logout(): void {
    localStorage.removeItem(this.TOKEN_KEY);
    localStorage.removeItem(this.USER_KEY);
  }

  isLoggedIn(): boolean {
    return !!localStorage.getItem(this.TOKEN_KEY);
  }

  getToken(): string | null {
    return localStorage.getItem(this.TOKEN_KEY);
  }

  private store(res: AuthResponse): void {
    localStorage.setItem(this.TOKEN_KEY, res.token);
    localStorage.setItem(this.USER_KEY, res.user_id);
  }
}
```

- [ ] **Step 3: Compile check**

```bash
cd frontend && npx ng build --configuration=production 2>&1 | tail -20
```

Expected: no errors related to the new files.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/models/auth.model.ts frontend/src/app/services/auth.service.ts
git commit -m "feat: add AuthService and auth models"
```

---

## Task 9: Frontend HTTP interceptor

**Files:**
- Create: `frontend/src/app/interceptors/auth.interceptor.ts`

- [ ] **Step 1: Create the interceptors directory and file**

Create `frontend/src/app/interceptors/auth.interceptor.ts`:

```typescript
import { HttpInterceptorFn, HttpErrorResponse } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';
import { AuthService } from '../services/auth.service';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  const router = inject(Router);
  const token = auth.getToken();

  const authReq = token
    ? req.clone({ setHeaders: { Authorization: `Bearer ${token}` } })
    : req;

  return next(authReq).pipe(
    catchError((err: HttpErrorResponse) => {
      if (err.status === 401) {
        auth.logout();
        router.navigate(['/login']);
      }
      return throwError(() => err);
    })
  );
};
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/app/interceptors/auth.interceptor.ts
git commit -m "feat: add auth HTTP interceptor"
```

---

## Task 10: Frontend auth guard

**Files:**
- Create: `frontend/src/app/guards/auth.guard.ts`

- [ ] **Step 1: Create `frontend/src/app/guards/auth.guard.ts`**

```typescript
import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService } from '../services/auth.service';

export const authGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  if (auth.isLoggedIn()) return true;
  return router.createUrlTree(['/login']);
};
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/app/guards/auth.guard.ts
git commit -m "feat: add auth route guard"
```

---

## Task 11: Login component

**Files:**
- Create: `frontend/src/app/pages/auth/login/login.component.ts`
- Create: `frontend/src/app/pages/auth/login/login.component.html`
- Create: `frontend/src/app/pages/auth/login/login.component.css`

- [ ] **Step 1: Create `login.component.ts`**

```typescript
import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { AuthService } from '../../../services/auth.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './login.component.html',
  styleUrls: ['./login.component.css']
})
export class LoginComponent {
  email = '';
  password = '';
  error = '';
  loading = false;

  constructor(private auth: AuthService, private router: Router) {}

  submit() {
    this.error = '';
    if (!this.email || !this.password) {
      this.error = 'Email and password are required';
      return;
    }
    this.loading = true;
    this.auth.login(this.email, this.password).subscribe({
      next: () => this.router.navigate(['/']),
      error: (err) => {
        this.loading = false;
        this.error = err.error?.error || 'Login failed';
      }
    });
  }
}
```

- [ ] **Step 2: Create `login.component.html`**

```html
<div class="auth-page">
  <div class="auth-card">
    <div class="logo">
      <span class="logo-icon">◈</span>
      <span class="logo-text">AIS</span>
    </div>
    <h1>Sign in</h1>

    <form (ngSubmit)="submit()">
      <div class="field">
        <label for="email">Email</label>
        <input
          id="email"
          type="email"
          [(ngModel)]="email"
          name="email"
          placeholder="you@example.com"
          autocomplete="email"
          [disabled]="loading"
        />
      </div>
      <div class="field">
        <label for="password">Password</label>
        <input
          id="password"
          type="password"
          [(ngModel)]="password"
          name="password"
          placeholder="••••••••"
          autocomplete="current-password"
          [disabled]="loading"
        />
      </div>
      <div class="error-msg" *ngIf="error">{{ error }}</div>
      <button type="submit" class="btn-primary" [disabled]="loading">
        <span *ngIf="!loading">Sign in</span>
        <span *ngIf="loading" class="spinner"></span>
      </button>
    </form>

    <p class="switch-link">
      Don't have an account? <a routerLink="/register">Register</a>
    </p>
  </div>
</div>
```

- [ ] **Step 3: Create `login.component.css`**

```css
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg, #0f1117);
}

.auth-card {
  background: #1a1d27;
  border: 1px solid #2a2d3a;
  border-radius: 12px;
  padding: 2.5rem;
  width: 100%;
  max-width: 400px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
}

.logo-icon {
  font-size: 1.5rem;
  color: #6c63ff;
}

.logo-text {
  font-size: 1.2rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  color: #e0e0e0;
}

h1 {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: 1.5rem;
  color: #e0e0e0;
}

.field {
  margin-bottom: 1rem;
}

.field label {
  display: block;
  font-size: 0.85rem;
  color: #9ca3af;
  margin-bottom: 0.4rem;
}

.field input {
  width: 100%;
  padding: 0.6rem 0.8rem;
  background: #0f1117;
  border: 1px solid #2a2d3a;
  border-radius: 6px;
  color: #e0e0e0;
  font-size: 0.95rem;
  box-sizing: border-box;
}

.field input:focus {
  outline: none;
  border-color: #6c63ff;
}

.error-msg {
  color: #ef4444;
  font-size: 0.85rem;
  margin-bottom: 0.75rem;
}

.btn-primary {
  width: 100%;
  padding: 0.7rem;
  background: #6c63ff;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 1rem;
  cursor: pointer;
  margin-top: 0.5rem;
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.switch-link {
  text-align: center;
  margin-top: 1.25rem;
  font-size: 0.875rem;
  color: #9ca3af;
}

.switch-link a {
  color: #6c63ff;
  text-decoration: none;
}

.spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/pages/auth/login/
git commit -m "feat: add login page component"
```

---

## Task 12: Register component

**Files:**
- Create: `frontend/src/app/pages/auth/register/register.component.ts`
- Create: `frontend/src/app/pages/auth/register/register.component.html`
- Create: `frontend/src/app/pages/auth/register/register.component.css`

- [ ] **Step 1: Create `register.component.ts`**

```typescript
import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { AuthService } from '../../../services/auth.service';

@Component({
  selector: 'app-register',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './register.component.html',
  styleUrls: ['./register.component.css']
})
export class RegisterComponent {
  email = '';
  password = '';
  error = '';
  loading = false;

  constructor(private auth: AuthService, private router: Router) {}

  submit() {
    this.error = '';
    if (!this.email || !this.password) {
      this.error = 'Email and password are required';
      return;
    }
    if (this.password.length < 8) {
      this.error = 'Password must be at least 8 characters';
      return;
    }
    this.loading = true;
    this.auth.register(this.email, this.password).subscribe({
      next: () => this.router.navigate(['/']),
      error: (err) => {
        this.loading = false;
        this.error = err.error?.error || 'Registration failed';
      }
    });
  }
}
```

- [ ] **Step 2: Create `register.component.html`**

```html
<div class="auth-page">
  <div class="auth-card">
    <div class="logo">
      <span class="logo-icon">◈</span>
      <span class="logo-text">AIS</span>
    </div>
    <h1>Create account</h1>

    <form (ngSubmit)="submit()">
      <div class="field">
        <label for="email">Email</label>
        <input
          id="email"
          type="email"
          [(ngModel)]="email"
          name="email"
          placeholder="you@example.com"
          autocomplete="email"
          [disabled]="loading"
        />
      </div>
      <div class="field">
        <label for="password">Password <span class="hint">(min 8 chars)</span></label>
        <input
          id="password"
          type="password"
          [(ngModel)]="password"
          name="password"
          placeholder="••••••••"
          autocomplete="new-password"
          [disabled]="loading"
        />
      </div>
      <div class="error-msg" *ngIf="error">{{ error }}</div>
      <button type="submit" class="btn-primary" [disabled]="loading">
        <span *ngIf="!loading">Create account</span>
        <span *ngIf="loading" class="spinner"></span>
      </button>
    </form>

    <p class="switch-link">
      Already have an account? <a routerLink="/login">Sign in</a>
    </p>
  </div>
</div>
```

- [ ] **Step 3: Create `register.component.css`**

Copy the same CSS as `login.component.css` — the styles are identical. Paste the full contents:

```css
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg, #0f1117);
}

.auth-card {
  background: #1a1d27;
  border: 1px solid #2a2d3a;
  border-radius: 12px;
  padding: 2.5rem;
  width: 100%;
  max-width: 400px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
}

.logo-icon { font-size: 1.5rem; color: #6c63ff; }
.logo-text { font-size: 1.2rem; font-weight: 700; letter-spacing: 0.1em; color: #e0e0e0; }

h1 { font-size: 1.5rem; font-weight: 600; margin-bottom: 1.5rem; color: #e0e0e0; }

.field { margin-bottom: 1rem; }

.field label { display: block; font-size: 0.85rem; color: #9ca3af; margin-bottom: 0.4rem; }

.hint { font-size: 0.75rem; color: #6b7280; }

.field input {
  width: 100%;
  padding: 0.6rem 0.8rem;
  background: #0f1117;
  border: 1px solid #2a2d3a;
  border-radius: 6px;
  color: #e0e0e0;
  font-size: 0.95rem;
  box-sizing: border-box;
}

.field input:focus { outline: none; border-color: #6c63ff; }

.error-msg { color: #ef4444; font-size: 0.85rem; margin-bottom: 0.75rem; }

.btn-primary {
  width: 100%;
  padding: 0.7rem;
  background: #6c63ff;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 1rem;
  cursor: pointer;
  margin-top: 0.5rem;
}

.btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }

.switch-link { text-align: center; margin-top: 1.25rem; font-size: 0.875rem; color: #9ca3af; }
.switch-link a { color: #6c63ff; text-decoration: none; }

.spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/pages/auth/register/
git commit -m "feat: add register page component"
```

---

## Task 13: Wire routes, interceptor, and logout

**Files:**
- Modify: `frontend/src/app/app.routes.ts`
- Modify: `frontend/src/app/app.config.ts`
- Modify: `frontend/src/app/pages/home/home.component.ts`
- Modify: `frontend/src/app/pages/home/home.component.html`

- [ ] **Step 1: Update `app.routes.ts`**

Replace the entire file:

```typescript
import { Routes } from '@angular/router';
import { HomeComponent } from './pages/home/home.component';
import { ProjectComponent } from './pages/project/project.component';
import { LoginComponent } from './pages/auth/login/login.component';
import { RegisterComponent } from './pages/auth/register/register.component';
import { authGuard } from './guards/auth.guard';

export const routes: Routes = [
  { path: 'login', component: LoginComponent },
  { path: 'register', component: RegisterComponent },
  { path: '', component: HomeComponent, canActivate: [authGuard] },
  { path: 'project/:id', component: ProjectComponent, canActivate: [authGuard] },
  { path: '**', redirectTo: '' }
];
```

- [ ] **Step 2: Update `app.config.ts`**

Replace the entire file:

```typescript
import { ApplicationConfig } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { routes } from './app.routes';
import { authInterceptor } from './interceptors/auth.interceptor';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(routes),
    provideHttpClient(withInterceptors([authInterceptor])),
  ]
};
```

- [ ] **Step 3: Update `home.component.ts`**

Replace the entire file:

```typescript
import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { ApiService, Project } from '../../services/api.service';
import { AuthService } from '../../services/auth.service';

@Component({
  selector: 'app-home',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './home.component.html',
  styleUrls: ['./home.component.css']
})
export class HomeComponent implements OnInit {
  repoUrl = '';
  loading = false;
  error = '';
  projects: Project[] = [];

  constructor(
    private api: ApiService,
    private auth: AuthService,
    private router: Router
  ) {}

  ngOnInit() {
    this.loadProjects();
  }

  loadProjects() {
    this.api.getProjects().subscribe({
      next: (p) => this.projects = p,
      error: () => {}
    });
  }

  analyze() {
    if (!this.repoUrl.trim()) return;
    this.loading = true;
    this.error = '';
    this.api.analyze(this.repoUrl.trim()).subscribe({
      next: (res) => {
        this.loading = false;
        this.router.navigate(['/project', res.id]);
      },
      error: (err) => {
        this.loading = false;
        this.error = err.error?.error || 'Failed to start analysis';
      }
    });
  }

  openProject(id: string) {
    this.router.navigate(['/project', id]);
  }

  deleteProject(id: string, event: Event) {
    event.stopPropagation();
    this.api.deleteProject(id).subscribe({
      next: () => this.loadProjects(),
      error: () => {}
    });
  }

  logout() {
    this.auth.logout();
    this.router.navigate(['/login']);
  }

  statusLabel(status: string): string {
    const map: Record<string, string> = {
      pending: 'Queued',
      analyzing: 'Analyzing…',
      done: 'Ready',
      error: 'Error'
    };
    return map[status] || status;
  }
}
```

- [ ] **Step 4: Update `home.component.html`**

Add a logout button to the header. Replace the opening `<div class="home">` and `<div class="hero">` block with:

```html
<div class="home">
  <div class="top-bar">
    <button class="btn-logout" (click)="logout()">Sign out</button>
  </div>
  <div class="hero">
```

Close the `<div class="top-bar">` before the `<div class="hero">` — the rest of the file stays the same. The full updated file:

```html
<div class="home">
  <div class="top-bar">
    <button class="btn-logout" (click)="logout()">Sign out</button>
  </div>

  <div class="hero">
    <div class="logo">
      <span class="logo-icon">◈</span>
      <span class="logo-text">AIS</span>
    </div>
    <h1>Architecture Insight System</h1>
    <p class="subtitle">Transform any repository into an interactive, navigable architecture map</p>

    <div class="analyze-form">
      <div class="input-row">
        <input
          type="text"
          [(ngModel)]="repoUrl"
          placeholder="https://github.com/user/repo"
          (keydown.enter)="analyze()"
          [disabled]="loading"
          class="url-input"
        />
        <button (click)="analyze()" [disabled]="loading || !repoUrl.trim()" class="btn-primary">
          <span *ngIf="!loading">Analyze</span>
          <span *ngIf="loading" class="spinner"></span>
        </button>
      </div>
      <div class="error-msg" *ngIf="error">{{ error }}</div>
      <div class="examples">
        <span class="examples-label">Try:</span>
        <a (click)="repoUrl='https://github.com/gin-gonic/gin'">gin-gonic/gin</a>
        <a (click)="repoUrl='https://github.com/expressjs/express'">expressjs/express</a>
        <a (click)="repoUrl='https://github.com/pallets/flask'">pallets/flask</a>
      </div>
    </div>
  </div>

  <div class="projects-section" *ngIf="projects.length > 0">
    <h2>Recent Projects</h2>
    <div class="projects-grid">
      <div
        *ngFor="let p of projects"
        class="project-card"
        [class.status-done]="p.status === 'done'"
        [class.status-error]="p.status === 'error'"
        [class.status-loading]="p.status === 'pending' || p.status === 'analyzing'"
        (click)="openProject(p.id)"
      >
        <div class="card-header">
          <span class="project-name">{{ p.name }}</span>
          <span class="status-badge" [attr.data-status]="p.status">{{ statusLabel(p.status) }}</span>
        </div>
        <div class="project-url">{{ p.url }}</div>
        <div class="card-footer">
          <span class="date">{{ p.created_at | date:'MMM d, y' }}</span>
          <button class="btn-delete" (click)="deleteProject(p.id, $event)">Delete</button>
        </div>
      </div>
    </div>
  </div>

  <div class="features">
    <div class="feature">
      <div class="feature-icon">🗺️</div>
      <h3>Interactive Graph</h3>
      <p>Explore your codebase as a force-directed graph with drill-down navigation</p>
    </div>
    <div class="feature">
      <div class="feature-icon">🔗</div>
      <h3>Dependency Analysis</h3>
      <p>Visualize import relationships between modules and files</p>
    </div>
    <div class="feature">
      <div class="feature-icon">🤖</div>
      <h3>AI Assistant</h3>
      <p>Ask questions about architecture, logic, and structure in plain language</p>
    </div>
  </div>
</div>
```

- [ ] **Step 5: Add `.btn-logout` style to `home.component.css`**

Append to `frontend/src/app/pages/home/home.component.css`:

```css
.top-bar {
  display: flex;
  justify-content: flex-end;
  padding: 1rem 2rem 0;
}

.btn-logout {
  background: transparent;
  border: 1px solid #2a2d3a;
  color: #9ca3af;
  padding: 0.4rem 1rem;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.85rem;
}

.btn-logout:hover {
  border-color: #6c63ff;
  color: #e0e0e0;
}
```

- [ ] **Step 6: Build check**

```bash
cd frontend && npx ng build --configuration=production 2>&1 | tail -30
```

Expected: `Application bundle generation complete` with no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/app/app.routes.ts frontend/src/app/app.config.ts \
        frontend/src/app/pages/home/home.component.ts \
        frontend/src/app/pages/home/home.component.html \
        frontend/src/app/pages/home/home.component.css
git commit -m "feat: wire auth guard, interceptor, routes, and logout button"
```

---

## Task 14: End-to-end smoke test

- [ ] **Step 1: Reset DB and rebuild**

```bash
docker-compose down -v
docker-compose up --build -d
```

Wait ~20 seconds for postgres to init.

- [ ] **Step 2: Register a user**

```bash
curl -s -X POST http://localhost/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"password123"}' | python3 -m json.tool
```

Expected: `{"token": "...", "user_id": "..."}`

- [ ] **Step 3: Try accessing projects without token (should fail)**

```bash
curl -s http://localhost/api/projects
```

Expected: `{"error":"unauthorized"}`

- [ ] **Step 4: Access projects with token**

```bash
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"password123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

curl -s -H "Authorization: Bearer $TOKEN" http://localhost/api/projects
```

Expected: `[]` (empty list for new user)

- [ ] **Step 5: Open browser and verify UI flow**

1. Navigate to http://localhost — should redirect to `/login`
2. Register a new account — should land on home page
3. Analyze a repo — should appear in project list
4. Click "Sign out" — should redirect to `/login`
5. Log back in — should see same project in list

- [ ] **Step 6: Final commit**

```bash
git add .
git commit -m "feat: authentication and user isolation complete"
```
