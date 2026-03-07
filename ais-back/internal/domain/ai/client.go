package ai

import "context"

// FileContent represents a source file to be indexed by the AI service.
type FileContent struct {
	Path     string
	Content  string
	Language string
}

// ChatMessage represents a single message in a conversation history.
type ChatMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

// ChatRequest encapsulates a user's chat request with context.
type ChatRequest struct {
	RepoID  string
	Message string
	NodeID  string
	History []*ChatMessage
}

// ChatToken is a single streaming token from the AI response.
type ChatToken struct {
	Token      string
	Done       bool
	References []*FileRef
}

// FileRef points to a specific location in source code.
type FileRef struct {
	FilePath  string
	StartLine int
	EndLine   int
}

// SearchResult is a semantically similar code chunk returned by vector search.
type SearchResult struct {
	ChunkID   string
	FilePath  string
	Content   string
	Score     float32
	StartLine int
	EndLine   int
	ChunkType string
	Name      string
}

// Client defines the port for all interactions with the ais-ai gRPC service.
type Client interface {
	// IndexRepository sends files to be chunked, embedded, and stored in Qdrant.
	IndexRepository(ctx context.Context, repoID string, files []*FileContent) (int32, error)

	// Chat streams AI response tokens for a user question over the given channel.
	// The channel is closed when streaming completes or an error occurs.
	Chat(ctx context.Context, req *ChatRequest, tokenCh chan<- *ChatToken) error

	// SearchSimilar performs semantic similarity search over a repo's code chunks.
	SearchSimilar(ctx context.Context, repoID, query string, topK int) ([]*SearchResult, error)

	// Close releases gRPC connection resources.
	Close() error
}
