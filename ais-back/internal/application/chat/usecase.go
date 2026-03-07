package chat

import (
	"context"

	"go.uber.org/zap"

	domainai "github.com/bakdaulet/ais/ais-back/internal/domain/ai"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

// TokenEvent is a single streaming token sent to the WebSocket client.
type TokenEvent struct {
	Token      string           `json:"token"`
	Done       bool             `json:"done"`
	References []*domainai.FileRef `json:"references,omitempty"`
}

// ChatRequest encapsulates a user's chat message with full context.
type ChatRequest struct {
	RepoID  string
	Message string
	NodeID  string
	History []*domainai.ChatMessage
}

// UseCase handles AI chat for a repository.
type UseCase struct {
	aiClient domainai.Client
	log      *logger.Logger
}

// NewUseCase creates a new chat use case.
func NewUseCase(aiClient domainai.Client, log *logger.Logger) *UseCase {
	return &UseCase{
		aiClient: aiClient,
		log:      log.WithComponent("chat_usecase"),
	}
}

// StreamChat sends a chat request to the AI service and streams tokens to tokenCh.
// The caller owns the tokenCh and should read from it until it is closed.
func (u *UseCase) StreamChat(
	ctx context.Context,
	req *ChatRequest,
	tokenCh chan<- *TokenEvent,
) error {
	if req.RepoID == "" {
		return apperrors.New(apperrors.ErrCodeInvalidInput, "repoId is required")
	}
	if req.Message == "" {
		return apperrors.New(apperrors.ErrCodeInvalidInput, "message is required")
	}

	u.log.Info("streaming chat request",
		zap.String("repo_id", req.RepoID),
		zap.String("message_preview", truncateStr(req.Message, 80)),
	)

	// Create a bridge channel from AI domain tokens to TokenEvents
	aiTokenCh := make(chan *domainai.ChatToken, 64)

	// Run the gRPC streaming call in a goroutine
	var streamErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamErr = u.aiClient.Chat(ctx, &domainai.ChatRequest{
			RepoID:  req.RepoID,
			Message: req.Message,
			NodeID:  req.NodeID,
			History: req.History,
		}, aiTokenCh)
	}()

	// Bridge domain tokens to TokenEvents
	for token := range aiTokenCh {
		event := &TokenEvent{
			Token:      token.Token,
			Done:       token.Done,
			References: token.References,
		}

		select {
		case tokenCh <- event:
		case <-ctx.Done():
			<-done
			return ctx.Err()
		}

		if token.Done {
			break
		}
	}

	<-done

	if streamErr != nil {
		u.log.Error("chat stream error",
			zap.String("repo_id", req.RepoID),
			zap.Error(streamErr),
		)
		return streamErr
	}

	return nil
}

// SearchSimilar performs semantic similarity search in the repository.
func (u *UseCase) SearchSimilar(
	ctx context.Context,
	repoID, query string,
	topK int,
) ([]*domainai.SearchResult, error) {
	if repoID == "" {
		return nil, apperrors.New(apperrors.ErrCodeInvalidInput, "repoId is required")
	}
	if query == "" {
		return nil, apperrors.New(apperrors.ErrCodeInvalidInput, "query is required")
	}
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}

	return u.aiClient.SearchSimilar(ctx, repoID, query, topK)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
