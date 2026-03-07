package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	domainai "github.com/bakdaulet/ais/ais-back/internal/domain/ai"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
	pb "github.com/bakdaulet/ais/ais-back/proto/ai"
)

// AIGRPCClient implements the domainai.Client port via gRPC.
type AIGRPCClient struct {
	conn    *grpc.ClientConn
	client  pb.AIServiceClient
	timeout time.Duration
	log     *logger.Logger
}

// NewAIGRPCClient establishes a gRPC connection to the AI service.
func NewAIGRPCClient(
	addr string,
	connTimeout time.Duration,
	requestTimeout time.Duration,
	keepaliveTime time.Duration,
	keepaliveTimeout time.Duration,
	log *logger.Logger,
) (*AIGRPCClient, error) {
	l := log.WithComponent("ai_grpc_client")

	dialCtx, cancel := context.WithTimeout(context.Background(), connTimeout)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second, // ping at most every 30s
			Timeout:             10 * time.Second,
			PermitWithoutStream: false, // only ping when there are active streams
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64*1024*1024), // 64MB
			grpc.MaxCallSendMsgSize(64*1024*1024),
		),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeInternalError, err,
			"failed to connect to AI service at %s", addr)
	}

	l.Info("gRPC connection to AI service established", zap.String("addr", addr))

	return &AIGRPCClient{
		conn:    conn,
		client:  pb.NewAIServiceClient(conn),
		timeout: requestTimeout,
		log:     l,
	}, nil
}

// IndexRepository sends repository files to the AI service for indexing.
func (c *AIGRPCClient) IndexRepository(
	ctx context.Context,
	repoID string,
	files []*domainai.FileContent,
) (int32, error) {
	if len(files) == 0 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	pbFiles := make([]*pb.FileContent, 0, len(files))
	for _, f := range files {
		pbFiles = append(pbFiles, &pb.FileContent{
			Path:     f.Path,
			Content:  f.Content,
			Language: f.Language,
		})
	}

	c.log.Info("indexing repository",
		zap.String("repo_id", repoID),
		zap.Int("file_count", len(files)),
	)

	resp, err := c.client.IndexRepository(ctx, &pb.IndexRequest{
		RepoId: repoID,
		Files:  pbFiles,
	})
	if err != nil {
		return 0, apperrors.Wrapf(apperrors.ErrCodeAIServiceError, err,
			"AI service IndexRepository failed for repo %s", repoID)
	}

	if !resp.Success {
		return 0, apperrors.Newf(apperrors.ErrCodeAIServiceError,
			"AI service indexing failed: %s", resp.Error)
	}

	c.log.Info("repository indexed successfully",
		zap.String("repo_id", repoID),
		zap.Int32("chunks_indexed", resp.ChunksIndexed),
	)

	return resp.ChunksIndexed, nil
}

// Chat streams AI response tokens for a user question.
// Tokens are sent to tokenCh and the channel is closed when streaming completes.
func (c *AIGRPCClient) Chat(
	ctx context.Context,
	req *domainai.ChatRequest,
	tokenCh chan<- *domainai.ChatToken,
) error {
	defer close(tokenCh)

	streamCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Convert history
	pbHistory := make([]*pb.ChatMessage, 0, len(req.History))
	for _, msg := range req.History {
		pbHistory = append(pbHistory, &pb.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	stream, err := c.client.Chat(streamCtx, &pb.ChatRequest{
		RepoId:  req.RepoID,
		Message: req.Message,
		NodeId:  req.NodeID,
		History: pbHistory,
	})
	if err != nil {
		return apperrors.Wrapf(apperrors.ErrCodeAIServiceError, err, "Chat gRPC call failed")
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			if ctx.Err() != nil {
				return nil // context cancelled, not an error
			}
			return apperrors.Wrapf(apperrors.ErrCodeAIServiceError, err, "Chat stream error")
		}

		token := &domainai.ChatToken{
			Token: chunk.Token,
			Done:  chunk.Done,
		}

		// Convert file references
		for _, ref := range chunk.References {
			token.References = append(token.References, &domainai.FileRef{
				FilePath:  ref.FilePath,
				StartLine: int(ref.StartLine),
				EndLine:   int(ref.EndLine),
			})
		}

		select {
		case tokenCh <- token:
		case <-ctx.Done():
			return nil
		}

		if chunk.Done {
			return nil
		}
	}
}

// SearchSimilar performs semantic similarity search over a repo's code chunks.
func (c *AIGRPCClient) SearchSimilar(
	ctx context.Context,
	repoID, query string,
	topK int,
) ([]*domainai.SearchResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.SearchSimilar(ctx, &pb.SearchRequest{
		RepoId: repoID,
		Query:  query,
		TopK:   int32(topK),
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeAIServiceError, err, "SearchSimilar gRPC call failed")
	}

	results := make([]*domainai.SearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		results = append(results, &domainai.SearchResult{
			ChunkID:   r.ChunkId,
			FilePath:  r.FilePath,
			Content:   r.Content,
			Score:     r.Score,
			StartLine: int(r.StartLine),
			EndLine:   int(r.EndLine),
			ChunkType: r.ChunkType,
			Name:      r.Name,
		})
	}

	return results, nil
}

// IsHealthy checks if the gRPC connection is in a usable state.
func (c *AIGRPCClient) IsHealthy() bool {
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// Close releases the gRPC connection.
func (c *AIGRPCClient) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("failed to close gRPC connection: %w", err)
	}
	c.log.Info("gRPC connection closed")
	return nil
}
