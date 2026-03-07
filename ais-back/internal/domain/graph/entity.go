package graph

import "context"

// NodeType enumerates the types of nodes stored in Neo4j.
type NodeType string

const (
	NodeTypeRepo     NodeType = "Repo"
	NodeTypeDir      NodeType = "Dir"
	NodeTypeFile     NodeType = "File"
	NodeTypeFunction NodeType = "Function"
	NodeTypeClass    NodeType = "Class"
)

// EdgeType enumerates the relationship types between nodes.
type EdgeType string

const (
	EdgeTypeHasRoot    EdgeType = "HAS_ROOT"
	EdgeTypeHasChild   EdgeType = "HAS_CHILD"
	EdgeTypeHasFile    EdgeType = "HAS_FILE"
	EdgeTypeHasFunction EdgeType = "HAS_FUNCTION"
	EdgeTypeHasClass   EdgeType = "HAS_CLASS"
	EdgeTypeImports    EdgeType = "IMPORTS"
	EdgeTypeCalls      EdgeType = "CALLS"
	EdgeTypeExtends    EdgeType = "EXTENDS"
	EdgeTypeImplements EdgeType = "IMPLEMENTS"
)

// GraphNode represents a node in the dependency graph.
type GraphNode struct {
	ID          string   `json:"id"`
	RepoID      string   `json:"repoId"`
	Type        NodeType `json:"type"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	FanIn       int      `json:"fanIn"`
	FanOut      int      `json:"fanOut"`
	HasCycle    bool     `json:"hasCycle"`
	HasChildren bool     `json:"hasChildren"`
	StartLine   int      `json:"startLine,omitempty"`
	EndLine     int      `json:"endLine,omitempty"`
	Language    string   `json:"language,omitempty"`
	Size        int64    `json:"size,omitempty"`
}

// GraphEdge represents a directed relationship between two nodes.
type GraphEdge struct {
	ID       string   `json:"id"`
	SourceID string   `json:"sourceId"`
	TargetID string   `json:"targetId"`
	Type     EdgeType `json:"type"`
	Line     int      `json:"line,omitempty"`
}

// NodeDetail contains rich detail about a single node for the detail panel.
type NodeDetail struct {
	Node      *GraphNode    `json:"node"`
	Imports   []*GraphNode  `json:"imports"`
	ImportedBy []*GraphNode `json:"importedBy"`
	Functions []*GraphNode  `json:"functions"`
	Classes   []*GraphNode  `json:"classes"`
	Callers   []*GraphNode  `json:"callers"`
	Callees   []*GraphNode  `json:"callees"`
	Metrics   *NodeMetrics  `json:"metrics"`
}

// NodeMetrics contains computed graph metrics for a node.
type NodeMetrics struct {
	FanIn        int     `json:"fanIn"`
	FanOut       int     `json:"fanOut"`
	Coupling     float64 `json:"coupling"`
	IsInCycle    bool    `json:"isInCycle"`
	CycleMembers []string `json:"cycleMembers,omitempty"`
	Depth        int     `json:"depth"`
}

// GraphView represents a subgraph returned for a drill-down query.
type GraphView struct {
	Nodes  []*GraphNode `json:"nodes"`
	Edges  []*GraphEdge `json:"edges"`
	Parent *GraphNode   `json:"parent,omitempty"`
}

// GraphMetrics contains repository-level graph statistics.
type GraphMetrics struct {
	RepoID        string  `json:"repoId"`
	NodeCount     int     `json:"nodeCount"`
	EdgeCount     int     `json:"edgeCount"`
	FileCount     int     `json:"fileCount"`
	DirCount      int     `json:"dirCount"`
	FunctionCount int     `json:"functionCount"`
	ClassCount    int     `json:"classCount"`
	CycleCount    int     `json:"cycleCount"`
	MaxFanIn      int     `json:"maxFanIn"`
	MaxFanOut     int     `json:"maxFanOut"`
	AvgFanIn      float64 `json:"avgFanIn"`
	AvgFanOut     float64 `json:"avgFanOut"`
}

// CycleResult describes a detected import cycle.
type CycleResult struct {
	Nodes []string `json:"nodes"`
	Edges []string `json:"edges"`
	Length int     `json:"length"`
}

// ShortestPathResult describes a path between two nodes.
type ShortestPathResult struct {
	Nodes []*GraphNode `json:"nodes"`
	Edges []*GraphEdge `json:"edges"`
	Length int         `json:"length"`
}

// ---------------------------------------------------------------------------
// Port
// ---------------------------------------------------------------------------

// GraphRepository defines all persistence operations on the graph database.
type GraphRepository interface {
	// Node operations
	SaveNode(ctx context.Context, node *GraphNode) error
	SaveNodes(ctx context.Context, nodes []*GraphNode) error
	GetNodeByID(ctx context.Context, nodeID string) (*GraphNode, error)
	GetNodeByPath(ctx context.Context, repoID, path string) (*GraphNode, error)

	// Edge operations
	SaveEdge(ctx context.Context, edge *GraphEdge) error
	SaveEdges(ctx context.Context, edges []*GraphEdge) error

	// Graph navigation
	GetRootView(ctx context.Context, repoID string) (*GraphView, error)
	GetChildrenView(ctx context.Context, nodeID string) (*GraphView, error)
	GetNodeDetail(ctx context.Context, nodeID string) (*NodeDetail, error)

	// File source code
	GetFileSourceByID(ctx context.Context, fileID string) (string, error)
	GetFileSourceByPath(ctx context.Context, repoID, path string) (string, error)

	// Analysis queries
	FindCycles(ctx context.Context, repoID string) ([]*CycleResult, error)
	GetMetrics(ctx context.Context, repoID string) (*GraphMetrics, error)
	GetShortestPath(ctx context.Context, fromNodeID, toNodeID string) (*ShortestPathResult, error)
	GetTopFanIn(ctx context.Context, repoID string, limit int) ([]*GraphNode, error)
	GetTopFanOut(ctx context.Context, repoID string, limit int) ([]*GraphNode, error)

	// Maintenance
	UpdateFanMetrics(ctx context.Context, repoID string) error
	MarkCycleNodes(ctx context.Context, repoID string, cyclePaths [][]*GraphNode) error
	DeleteRepo(ctx context.Context, repoID string) error

	// Health
	Ping(ctx context.Context) error
}
