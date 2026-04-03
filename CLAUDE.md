# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is AIS

Architecture Insight System — a platform that turns any GitHub repository into an interactive, navigable dependency graph. Users submit a repo URL, the backend clones and parses it, and the frontend renders a force-directed graph with drill-down navigation and an AI chat assistant.

## Running the system

```bash
cp .env.example .env          # set ANTHROPIC_API_KEY
docker-compose up --build     # first run
docker-compose up             # subsequent runs
docker-compose down -v        # full reset including postgres volume
```

Frontend: http://localhost
Backend API: http://localhost/api/
AI service: http://localhost/ai/

## Commands

### Backend (Go)
```bash
cd backend
go build ./...                # compile check
go vet ./...                  # lint
go test ./...                 # run all tests
```

### Frontend (Angular)
```bash
cd frontend
npm install --legacy-peer-deps
npm run build:prod            # production build (used in Docker)
npx ng serve                  # local dev server (requires backend running separately)
```

### Rebuild individual services
```bash
docker-compose build backend
docker-compose build frontend
docker-compose up -d backend frontend
```

## Architecture

### Request flow
1. User submits GitHub URL → `POST /api/analyze` → backend clones repo to `/tmp/repos/<uuid>` (temp dir, auto-deleted after parse)
2. Background goroutine runs `parser.ParseRepo` → stores full graph JSON in `projects.graph_data` (PostgreSQL JSONB)
3. Frontend polls `GET /api/projects/:id` until `status=done`
4. `GET /api/projects/:id/graph?path=` returns a filtered view; the **full graph is stored once**, filtering is done at query time in `filterByDepth` / `filterByParentPath`
5. AI chat: frontend → `POST /ai/chat` → Python service fetches project context from backend, calls Claude API, returns answer

### Graph filtering logic (backend/main.go)
This is the core algorithm. At any navigation level, edges are **aggregated** — if file `a/b.go` imports `c/d.go`, at the root view this shows as `a → c`. Key functions:
- `filterByDepth(graph, 0)` — root view: nodes at depth=0, edges aggregated to depth=0 ancestors
- `filterByParentPath(graph, "src")` — drill-down view: direct children of `src/`, edges aggregated to those children
- `ancestorAtDepth` — utility that walks a path to find the ancestor at a given depth

### Parser (backend/parser/)
- `parser.go` — walks file tree (skipping `.git`, `node_modules`, `vendor`, etc.), calls `extractFileInfo` per file, resolves imports to internal repo paths, builds `GraphData{Nodes, Edges}`
- `languages.go` — language detection by extension, import extraction via regex for Go/Python/JS/TS/Java/Rust/C

Import resolution strategy: tries relative path first, then fuzzy-matches by last path component against all known file/dir paths. Cross-package Go imports like `"github.com/x/y/render"` resolve to the `render/` directory node via last-component matching.

### Frontend (Angular 17 standalone)
- `ApiService` — all HTTP calls; backend proxied at `/api/`, AI service at `/ai/` via nginx
- `GraphComponent` — D3 v7 force-directed graph. Single-click highlights connected edges (outgoing=orange, incoming=green, unrelated=dimmed). Double-click on directory triggers drill-down. `resetSelection()` on background click.
- `ProjectComponent` — polls project status, owns navigation state (`currentPath`, `breadcrumbs`), passes graph slice to `GraphComponent`
- nginx (`frontend/nginx.conf`) proxies `/api/` → `backend:8080` and `/ai/` → `ai-service:8000`

### AI service (Python/FastAPI — ai-service/main.py)
Stateless: each `/chat` request fetches project metadata + current graph view from the backend, builds a text context (file tree + edges), and sends to `claude-opus-4-6`. No vector DB / embeddings — context is passed directly. Requires `ANTHROPIC_API_KEY` env var.

### Database
Single table `projects` with JSONB columns `graph_data` and `file_tree`. No migrations — table is created with `CREATE TABLE IF NOT EXISTS` on startup.

## Key constraints
- Only public Git repos are supported (no auth for cloning)
- Repos are cloned with `--depth=1 --single-branch` and deleted immediately after parsing
- Files >5000 lines are truncated during import extraction
- Binary/media file extensions are skipped (see `skipExts` in `parser.go`)
- `docker-compose down -v` is required when switching postgres major versions (the volume stores the initialized version)
