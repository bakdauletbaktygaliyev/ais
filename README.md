# AIS — Architecture Insight System

> Understand any GitHub repository in minutes. Visualise dependency graphs, detect cycles, and chat with your code using AI.

[![CI](https://github.com/bakdaulet/ais/actions/workflows/ci.yml/badge.svg)](https://github.com/bakdaulet/ais/actions/workflows/ci.yml)

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│  ais-front  Angular 17 SPA (Cytoscape.js graph, Monaco editor)  │
└────────────────────────────┬─────────────────────────────────────┘
                             │  REST + WebSocket
┌────────────────────────────▼─────────────────────────────────────┐
│  ais-back   Go 1.24  · Gin · Neo4j · Redis · go-git · Tree-sitter│
└────────────────────────────┬─────────────────────────────────────┘
                             │  gRPC
┌────────────────────────────▼─────────────────────────────────────┐
│  ais-ai     Python 3.12 · Claude API · Voyage AI · Qdrant        │
└──────────────────────────────────────────────────────────────────┘

Infrastructure: Neo4j (graph) · Redis (cache) · Qdrant (vectors)
```

## Quick Start

### Prerequisites
- Docker & docker-compose
- Anthropic API key — [console.anthropic.com](https://console.anthropic.com)
- Voyage AI API key — [dash.voyageai.com](https://dash.voyageai.com)

### 1. Clone and configure
```bash
git clone https://github.com/bakdaulet/ais.git
cd ais
cp .env.example .env
# Edit .env and fill in ANTHROPIC_API_KEY, VOYAGE_API_KEY
```

### 2. Start the stack
```bash
docker-compose up -d
```

### 3. Open the app
Navigate to [http://localhost:4200](http://localhost:4200) and paste any public GitHub URL.

---

## Services

| Service    | Language   | Port  | Purpose                                     |
|------------|------------|-------|---------------------------------------------|
| ais-front  | Angular 17 | 4200  | UI: graph visualisation, code viewer, chat  |
| ais-back   | Go 1.24    | 8080  | API, WebSocket, analysis pipeline           |
| ais-ai     | Python 3.12| 50051 | gRPC: embeddings, RAG, Claude chat          |
| Neo4j      | –          | 7687  | Dependency graph storage                    |
| Redis      | –          | 6379  | Repo status cache                           |
| Qdrant     | –          | 6333  | Vector store for semantic search            |

## Development

### ais-back
```bash
cd ais-back
go mod download
go run ./cmd/server
```

### ais-ai
```bash
cd ais-ai
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python -m src.main
```

### ais-front
```bash
cd ais-front
npm install
npm start   # starts ng serve on :4200 with proxy to :8080
```

## Testing

```bash
# Go
cd ais-back && go test ./...

# Python
cd ais-ai && pytest

# Angular
cd ais-front && npx tsc --noEmit
```

## Analysis Pipeline

```
POST /api/v1/repos  →  validate → clone → detect lang → walk FS
                        → parse AST → build graph → index AI → done
                                             ↕ WebSocket progress events
```

Each step emits a `progress` WebSocket event (0–100%) visible on the analysis page.

## Graph Navigation

- **Double-click** any directory node to drill down
- **Single-click** to open the detail panel (imports, metrics)
- **Breadcrumb** trail for navigation back up the tree
- Cycle nodes highlighted with a red dashed border
- Fan-in / Fan-out metrics on every file node

## License

MIT



```
ais/
├── ais-back/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── domain/
│   │   │   ├── repository/
│   │   │   │   ├── entity.go
│   │   │   │   └── repository.go (port)
│   │   │   ├── graph/
│   │   │   │   ├── entity.go
│   │   │   │   └── repository.go (port)
│   │   │   ├── analysis/
│   │   │   │   ├── entity.go
│   │   │   │   ├── pipeline.go
│   │   │   │   └── service.go (port)
│   │   │   └── ai/
│   │   │       └── client.go (port)
│   │   ├── application/
│   │   │   ├── analysis/
│   │   │   │   ├── usecase.go
│   │   │   │   └── handler.go
│   │   │   ├── graph/
│   │   │   │   ├── usecase.go
│   │   │   │   └── handler.go
│   │   │   └── chat/
│   │   │       ├── usecase.go
│   │   │       └── handler.go
│   │   ├── infrastructure/
│   │   │   ├── neo4j/
│   │   │   │   ├── client.go
│   │   │   │   ├── graph_repo.go
│   │   │   │   └── queries.go
│   │   │   ├── redis/
│   │   │   │   ├── client.go
│   │   │   │   └── cache_repo.go
│   │   │   ├── github/
│   │   │   │   └── client.go
│   │   │   ├── git/
│   │   │   │   └── cloner.go
│   │   │   ├── parser/
│   │   │   │   ├── treesitter.go
│   │   │   │   ├── typescript.go
│   │   │   │   ├── javascript.go
│   │   │   │   └── golang.go
│   │   │   └── grpc/
│   │   │       └── ai_client.go
│   │   └── delivery/
│   │       ├── http/
│   │       │   ├── router.go
│   │       │   ├── middleware/
│   │       │   │   ├── cors.go
│   │       │   │   ├── logger.go
│   │       │   │   └── recovery.go
│   │       │   └── handlers/
│   │       │       ├── analysis.go
│   │       │       ├── graph.go
│   │       │       └── health.go
│   │       └── websocket/
│   │           ├── hub.go
│   │           ├── client.go
│   │           └── handler.go
│   ├── pkg/
│   │   ├── config/
│   │   │   └── config.go
│   │   ├── logger/
│   │   │   └── logger.go
│   │   └── errors/
│   │       └── errors.go
│   ├── proto/
│   │   └── ai/
│   │       └── ai.proto
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
├── ais-ai/
│   ├── src/
│   │   ├── main.py
│   │   ├── domain/
│   │   │   ├── __init__.py
│   │   │   ├── chunk.py
│   │   │   └── embedding.py
│   │   ├── application/
│   │   │   ├── __init__.py
│   │   │   ├── indexing_service.py
│   │   │   ├── chat_service.py
│   │   │   └── search_service.py
│   │   ├── infrastructure/
│   │   │   ├── __init__.py
│   │   │   ├── voyage_client.py
│   │   │   ├── qdrant_client.py
│   │   │   ├── claude_client.py
│   │   │   └── neo4j_client.py
│   │   ├── delivery/
│   │   │   ├── __init__.py
│   │   │   └── grpc_server.py
│   │   └── proto/
│   │       ├── ai_pb2.py
│   │       └── ai_pb2_grpc.py
│   ├── tests/
│   ├── proto/
│   │   └── ai.proto
│   ├── requirements.txt
│   ├── pyproject.toml
│   └── Dockerfile
│
├── ais-front/
│   ├── src/
│   │   ├── app/
│   │   │   ├── core/
│   │   │   │   ├── services/
│   │   │   │   │   ├── api.service.ts
│   │   │   │   │   ├── websocket.service.ts
│   │   │   │   │   └── graph-state.service.ts
│   │   │   │   ├── models/
│   │   │   │   │   ├── graph.model.ts
│   │   │   │   │   ├── node.model.ts
│   │   │   │   │   └── chat.model.ts
│   │   │   │   └── interceptors/
│   │   │   │       └── error.interceptor.ts
│   │   │   ├── features/
│   │   │   │   ├── home/
│   │   │   │   │   ├── home.component.ts
│   │   │   │   │   └── home.component.html
│   │   │   │   ├── analysis/
│   │   │   │   │   ├── analysis.component.ts
│   │   │   │   │   └── analysis.component.html
│   │   │   │   ├── graph/
│   │   │   │   │   ├── graph.component.ts
│   │   │   │   │   ├── graph.component.html
│   │   │   │   │   ├── cytoscape-config.ts
│   │   │   │   │   └── graph-layout.ts
│   │   │   │   ├── node-detail/
│   │   │   │   │   ├── node-detail.component.ts
│   │   │   │   │   └── node-detail.component.html
│   │   │   │   ├── code-viewer/
│   │   │   │   │   ├── code-viewer.component.ts
│   │   │   │   │   └── code-viewer.component.html
│   │   │   │   └── chat/
│   │   │   │       ├── chat.component.ts
│   │   │   │       └── chat.component.html
│   │   │   ├── shared/
│   │   │   │   ├── components/
│   │   │   │   │   ├── breadcrumb/
│   │   │   │   │   ├── progress-bar/
│   │   │   │   │   ├── metrics-panel/
│   │   │   │   │   └── loading-spinner/
│   │   │   │   └── pipes/
│   │   │   ├── app.component.ts
│   │   │   ├── app.config.ts
│   │   │   └── app.routes.ts
│   │   ├── environments/
│   │   │   ├── environment.ts
│   │   │   └── environment.prod.ts
│   │   ├── styles/
│   │   │   └── global.scss
│   │   └── index.html
│   ├── angular.json
│   ├── package.json
│   ├── tsconfig.json
│   └── Dockerfile
│
├── proto/
│   └── ai/
│       └── ai.proto            ← shared proto definition
│
├── docker-compose.yml
├── .github/
│   └── workflows/
│       ├── backend.yml
│       ├── ai-service.yml
│       └── frontend.yml
└── README.md
```