package graph

import (
	"context"

	"github.com/bakdaulet/ais/ais-back/internal/domain/graph"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

// UseCase handles all graph navigation and query operations.
type UseCase struct {
	graphRepo graph.GraphRepository
	log       *logger.Logger
}

// NewUseCase creates a new graph use case.
func NewUseCase(graphRepo graph.GraphRepository, log *logger.Logger) *UseCase {
	return &UseCase{
		graphRepo: graphRepo,
		log:       log.WithComponent("graph_usecase"),
	}
}

// GetRootView returns the top-level graph view for a repository.
func (u *UseCase) GetRootView(ctx context.Context, repoID string) (*graph.GraphView, error) {
	view, err := u.graphRepo.GetRootView(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return view, nil
}

// GetChildrenView returns the direct children of a node for drill-down navigation.
func (u *UseCase) GetChildrenView(ctx context.Context, nodeID string) (*graph.GraphView, error) {
	view, err := u.graphRepo.GetChildrenView(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return view, nil
}

// GetNodeDetail returns comprehensive detail about a specific node.
func (u *UseCase) GetNodeDetail(ctx context.Context, nodeID string) (*graph.NodeDetail, error) {
	detail, err := u.graphRepo.GetNodeDetail(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// GetFileSource returns the raw source code for a file node.
func (u *UseCase) GetFileSource(ctx context.Context, fileID string) (string, error) {
	source, err := u.graphRepo.GetFileSourceByID(ctx, fileID)
	if err != nil {
		return "", err
	}
	return source, nil
}

// GetFileSourceByPath returns source code for a file identified by path.
func (u *UseCase) GetFileSourceByPath(ctx context.Context, repoID, path string) (string, error) {
	source, err := u.graphRepo.GetFileSourceByPath(ctx, repoID, path)
	if err != nil {
		return "", err
	}
	return source, nil
}

// GetCycles returns all detected import cycles in a repository.
func (u *UseCase) GetCycles(ctx context.Context, repoID string) ([]*graph.CycleResult, error) {
	cycles, err := u.graphRepo.FindCycles(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return cycles, nil
}

// GetMetrics returns aggregate graph statistics for a repository.
func (u *UseCase) GetMetrics(ctx context.Context, repoID string) (*graph.GraphMetrics, error) {
	metrics, err := u.graphRepo.GetMetrics(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

// GetShortestPath finds the shortest import path between two nodes.
func (u *UseCase) GetShortestPath(ctx context.Context, fromID, toID string) (*graph.ShortestPathResult, error) {
	if fromID == toID {
		return nil, apperrors.New(apperrors.ErrCodeInvalidInput, "source and target nodes must be different")
	}
	result, err := u.graphRepo.GetShortestPath(ctx, fromID, toID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetTopFanIn returns the most-depended-upon files.
func (u *UseCase) GetTopFanIn(ctx context.Context, repoID string, limit int) ([]*graph.GraphNode, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	return u.graphRepo.GetTopFanIn(ctx, repoID, limit)
}

// GetTopFanOut returns files with the most dependencies.
func (u *UseCase) GetTopFanOut(ctx context.Context, repoID string, limit int) ([]*graph.GraphNode, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	return u.graphRepo.GetTopFanOut(ctx, repoID, limit)
}
