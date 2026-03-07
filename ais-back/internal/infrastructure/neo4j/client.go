package neo4j

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"

	"github.com/bakdaulet/ais/ais-back/internal/domain/graph"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

// Client wraps the neo4j driver with application-specific helpers.
type Client struct {
	driver neo4j.DriverWithContext
	dbName string
	log    *logger.Logger
}

// NewClient creates a Neo4j client and verifies connectivity.
func NewClient(uri, user, password, database string, log *logger.Logger) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, password, ""))
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to create Neo4j driver", err)
	}

	c := &Client{
		driver: driver,
		dbName: database,
		log:    log.WithComponent("neo4j_client"),
	}

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "cannot connect to Neo4j", err)
	}

	log.Info("Neo4j connection established", zap.String("uri", uri), zap.String("db", database))

	return c, nil
}

// Close closes the driver connection.
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

// NewSession creates a new database session.
func (c *Client) NewSession(ctx context.Context) neo4j.SessionWithContext {
	return c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.dbName,
	})
}

// ExecuteWrite runs a write transaction with retry.
func (c *Client) ExecuteWrite(
	ctx context.Context,
	fn func(tx neo4j.ManagedTransaction) (any, error),
) (any, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, fn)
	if err != nil {
		return nil, mapNeo4jError(err)
	}
	return result, nil
}

// ExecuteRead runs a read transaction.
func (c *Client) ExecuteRead(
	ctx context.Context,
	fn func(tx neo4j.ManagedTransaction) (any, error),
) (any, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, fn)
	if err != nil {
		return nil, mapNeo4jError(err)
	}
	return result, nil
}

// Ping verifies the Neo4j connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	return c.driver.VerifyConnectivity(ctx)
}

// EnsureConstraints creates uniqueness constraints for graph nodes.
func (c *Client) EnsureConstraints(ctx context.Context) error {
	constraints := []string{
		`CREATE CONSTRAINT IF NOT EXISTS FOR (r:Repo) REQUIRE r.id IS UNIQUE`,
		`CREATE CONSTRAINT IF NOT EXISTS FOR (d:Dir) REQUIRE d.id IS UNIQUE`,
		`CREATE CONSTRAINT IF NOT EXISTS FOR (f:File) REQUIRE f.id IS UNIQUE`,
		`CREATE CONSTRAINT IF NOT EXISTS FOR (fn:Function) REQUIRE fn.id IS UNIQUE`,
		`CREATE CONSTRAINT IF NOT EXISTS FOR (c:Class) REQUIRE c.id IS UNIQUE`,
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS FOR (r:Repo) ON (r.id)`,
		`CREATE INDEX IF NOT EXISTS FOR (d:Dir) ON (d.repoId, d.path)`,
		`CREATE INDEX IF NOT EXISTS FOR (f:File) ON (f.repoId, f.path)`,
		`CREATE INDEX IF NOT EXISTS FOR (fn:Function) ON (fn.repoId, fn.name)`,
		`CREATE INDEX IF NOT EXISTS FOR (c:Class) ON (c.repoId, c.name)`,
	}

	session := c.NewSession(ctx)
	defer session.Close(ctx)

	for _, q := range append(constraints, indexes...) {
		_, err := session.Run(ctx, q, nil)
		if err != nil {
			c.log.Warn("constraint/index creation warning",
				zap.String("query", q), zap.Error(err))
		}
	}

	c.log.Info("Neo4j constraints and indexes ensured")
	return nil
}

// ---------------------------------------------------------------------------
// GraphRepository implementation
// ---------------------------------------------------------------------------

// GraphRepo implements the graph.GraphRepository port backed by Neo4j.
type GraphRepo struct {
	client *Client
	log    *logger.Logger
}

// NewGraphRepo creates a new Neo4j-backed GraphRepository.
func NewGraphRepo(client *Client, log *logger.Logger) *GraphRepo {
	return &GraphRepo{
		client: client,
		log:    log.WithComponent("graph_repo"),
	}
}

// SaveNode creates or updates a node in the graph.
func (r *GraphRepo) SaveNode(ctx context.Context, node *graph.GraphNode) error {
	query := buildMergeNodeQuery(node.Type)
	params := nodeToParams(node)

	_, err := r.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// SaveNodes creates or updates multiple nodes in a single transaction.
func (r *GraphRepo) SaveNodes(ctx context.Context, nodes []*graph.GraphNode) error {
	if len(nodes) == 0 {
		return nil
	}

	// Batch by node type for efficiency
	byType := map[graph.NodeType][]*graph.GraphNode{}
	for _, n := range nodes {
		byType[n.Type] = append(byType[n.Type], n)
	}

	for nodeType, batch := range byType {
		query := buildMergeNodeQuery(nodeType)
		_, err := r.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			for _, node := range batch {
				if _, err := tx.Run(ctx, query, nodeToParams(node)); err != nil {
					return nil, fmt.Errorf("failed to save node %s: %w", node.ID, err)
				}
			}
			return nil, nil
		})
		if err != nil {
			return apperrors.Wrapf(apperrors.ErrCodeGraphError, err,
				"failed to save %d %s nodes", len(batch), nodeType)
		}
	}

	return nil
}

// SaveEdge creates a relationship between two nodes.
func (r *GraphRepo) SaveEdge(ctx context.Context, edge *graph.GraphEdge) error {
	query := buildMergeEdgeQuery(edge.Type)
	params := edgeToParams(edge)

	_, err := r.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// SaveEdges creates multiple relationships in a single transaction.
func (r *GraphRepo) SaveEdges(ctx context.Context, edges []*graph.GraphEdge) error {
	if len(edges) == 0 {
		return nil
	}

	// Batch by edge type
	byType := map[graph.EdgeType][]*graph.GraphEdge{}
	for _, e := range edges {
		byType[e.Type] = append(byType[e.Type], e)
	}

	for edgeType, batch := range byType {
		query := buildMergeEdgeQuery(edgeType)
		_, err := r.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			for _, edge := range batch {
				if _, err := tx.Run(ctx, query, edgeToParams(edge)); err != nil {
					return nil, fmt.Errorf("failed to save edge %s->%s: %w",
						edge.SourceID, edge.TargetID, err)
				}
			}
			return nil, nil
		})
		if err != nil {
			return apperrors.Wrapf(apperrors.ErrCodeGraphError, err,
				"failed to save %d %s edges", len(batch), edgeType)
		}
	}

	return nil
}

// GetNodeByID retrieves a single node by its ID.
func (r *GraphRepo) GetNodeByID(ctx context.Context, nodeID string) (*graph.GraphNode, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			`MATCH (n {id: $id}) RETURN n`,
			map[string]any{"id": nodeID},
		)
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, err
		}
		nodeVal, ok := record.Get("n")
		if !ok {
			return nil, fmt.Errorf("node not found")
		}
		return nodeFromRecord(nodeVal.(neo4j.Node)), nil
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeNotFound, err, "node %s not found", nodeID)
	}
	return result.(*graph.GraphNode), nil
}

// GetNodeByPath retrieves a File or Dir node by its repo and path.
func (r *GraphRepo) GetNodeByPath(ctx context.Context, repoID, path string) (*graph.GraphNode, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			`MATCH (n {repoId: $repoId, path: $path}) RETURN n LIMIT 1`,
			map[string]any{"repoId": repoID, "path": path},
		)
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, err
		}
		nodeVal, ok := record.Get("n")
		if !ok {
			return nil, fmt.Errorf("node not found")
		}
		return nodeFromRecord(nodeVal.(neo4j.Node)), nil
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeNotFound, err,
			"node at path %s in repo %s not found", path, repoID)
	}
	return result.(*graph.GraphNode), nil
}

// GetRootView returns the top-level directory nodes for a repository.
func (r *GraphRepo) GetRootView(ctx context.Context, repoID string) (*graph.GraphView, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, queryGetRootView, map[string]any{"repoId": repoID})
		if err != nil {
			return nil, err
		}
		return collectGraphView(ctx, res)
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeGraphError, err,
			"failed to get root view for repo %s", repoID)
	}
	return result.(*graph.GraphView), nil
}

// GetChildrenView returns direct children of a node (sub-directories and files).
func (r *GraphRepo) GetChildrenView(ctx context.Context, nodeID string) (*graph.GraphView, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, queryGetChildrenView, map[string]any{"nodeId": nodeID})
		if err != nil {
			return nil, err
		}
		return collectGraphView(ctx, res)
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeGraphError, err,
			"failed to get children for node %s", nodeID)
	}
	return result.(*graph.GraphView), nil
}

// GetNodeDetail returns rich detail about a node for the detail panel.
func (r *GraphRepo) GetNodeDetail(ctx context.Context, nodeID string) (*graph.NodeDetail, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Main node
		nodeRes, err := tx.Run(ctx,
			`MATCH (n {id: $id}) RETURN n`,
			map[string]any{"id": nodeID},
		)
		if err != nil {
			return nil, err
		}
		nodeRecord, err := nodeRes.Single(ctx)
		if err != nil {
			return nil, fmt.Errorf("node %s not found: %w", nodeID, err)
		}
		nodeVal, _ := nodeRecord.Get("n")
		mainNode := nodeFromRecord(nodeVal.(neo4j.Node))

		detail := &graph.NodeDetail{
			Node: mainNode,
		}

		// Imports (files this file imports)
		importsRes, err := tx.Run(ctx, queryGetImports, map[string]any{"id": nodeID})
		if err == nil {
			detail.Imports = collectNodes(ctx, importsRes, "dep")
		}

		// ImportedBy (files that import this file)
		importedByRes, err := tx.Run(ctx, queryGetImportedBy, map[string]any{"id": nodeID})
		if err == nil {
			detail.ImportedBy = collectNodes(ctx, importedByRes, "importer")
		}

		// Functions
		functionsRes, err := tx.Run(ctx, queryGetFunctions, map[string]any{"id": nodeID})
		if err == nil {
			detail.Functions = collectNodes(ctx, functionsRes, "fn")
		}

		// Classes
		classesRes, err := tx.Run(ctx, queryGetClasses, map[string]any{"id": nodeID})
		if err == nil {
			detail.Classes = collectNodes(ctx, classesRes, "cls")
		}

		// Metrics
		detail.Metrics = &graph.NodeMetrics{
			FanIn:  mainNode.FanIn,
			FanOut: mainNode.FanOut,
		}
		if mainNode.FanOut+mainNode.FanIn > 0 {
			detail.Metrics.Coupling = float64(mainNode.FanIn) / float64(mainNode.FanIn+mainNode.FanOut)
		}
		detail.Metrics.IsInCycle = mainNode.HasCycle

		return detail, nil
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeGraphError, err,
			"failed to get detail for node %s", nodeID)
	}
	return result.(*graph.NodeDetail), nil
}

// GetFileSourceByID returns the raw source content of a file node.
func (r *GraphRepo) GetFileSourceByID(ctx context.Context, fileID string) (string, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			`MATCH (f:File {id: $id}) RETURN f.source AS source`,
			map[string]any{"id": fileID},
		)
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, fmt.Errorf("file %s not found", fileID)
		}
		src, _ := record.Get("source")
		if src == nil {
			return "", nil
		}
		return src.(string), nil
	})
	if err != nil {
		return "", apperrors.Wrapf(apperrors.ErrCodeNotFound, err, "file %s not found", fileID)
	}
	return result.(string), nil
}

// GetFileSourceByPath returns source by repo and path.
func (r *GraphRepo) GetFileSourceByPath(ctx context.Context, repoID, path string) (string, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			`MATCH (f:File {repoId: $repoId, path: $path}) RETURN f.source AS source`,
			map[string]any{"repoId": repoID, "path": path},
		)
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, fmt.Errorf("file not found")
		}
		src, _ := record.Get("source")
		if src == nil {
			return "", nil
		}
		return src.(string), nil
	})
	if err != nil {
		return "", apperrors.Wrapf(apperrors.ErrCodeNotFound, err,
			"file at %s not found in repo %s", path, repoID)
	}
	return result.(string), nil
}

// FindCycles finds all import cycles in a repository using DFS.
func (r *GraphRepo) FindCycles(ctx context.Context, repoID string) ([]*graph.CycleResult, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, queryFindCycles, map[string]any{"repoId": repoID})
		if err != nil {
			return nil, err
		}

		var cycles []*graph.CycleResult
		records, err := res.Collect(ctx)
		if err != nil {
			return nil, err
		}

		for _, record := range records {
			cycleVal, ok := record.Get("cycle")
			if !ok {
				continue
			}
			cycleNodes := toStringSlice(cycleVal)
			if len(cycleNodes) > 0 {
				cycles = append(cycles, &graph.CycleResult{
					Nodes:  cycleNodes,
					Length: len(cycleNodes),
				})
			}
		}

		return cycles, nil
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeGraphError, err, "cycle detection failed")
	}

	var cycles []*graph.CycleResult
	if result != nil {
		cycles = result.([]*graph.CycleResult)
	}
	return cycles, nil
}

// GetMetrics returns aggregate graph metrics for a repository.
func (r *GraphRepo) GetMetrics(ctx context.Context, repoID string) (*graph.GraphMetrics, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, queryGetMetrics, map[string]any{"repoId": repoID})
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, err
		}

		metrics := &graph.GraphMetrics{RepoID: repoID}
		if v, ok := record.Get("fileCount"); ok && v != nil {
			metrics.FileCount = int(v.(int64))
		}
		if v, ok := record.Get("dirCount"); ok && v != nil {
			metrics.DirCount = int(v.(int64))
		}
		if v, ok := record.Get("functionCount"); ok && v != nil {
			metrics.FunctionCount = int(v.(int64))
		}
		if v, ok := record.Get("classCount"); ok && v != nil {
			metrics.ClassCount = int(v.(int64))
		}
		if v, ok := record.Get("edgeCount"); ok && v != nil {
			metrics.EdgeCount = int(v.(int64))
		}
		if v, ok := record.Get("cycleCount"); ok && v != nil {
			metrics.CycleCount = int(v.(int64))
		}
		if v, ok := record.Get("maxFanIn"); ok && v != nil {
			metrics.MaxFanIn = int(v.(int64))
		}
		if v, ok := record.Get("maxFanOut"); ok && v != nil {
			metrics.MaxFanOut = int(v.(int64))
		}
		metrics.NodeCount = metrics.FileCount + metrics.DirCount + metrics.FunctionCount + metrics.ClassCount

		return metrics, nil
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeGraphError, err,
			"failed to get metrics for repo %s", repoID)
	}
	return result.(*graph.GraphMetrics), nil
}

// GetShortestPath finds the shortest import path between two file nodes.
func (r *GraphRepo) GetShortestPath(
	ctx context.Context,
	fromNodeID, toNodeID string,
) (*graph.ShortestPathResult, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, queryShortestPath,
			map[string]any{"from": fromNodeID, "to": toNodeID})
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, fmt.Errorf("no path found")
		}

		pathVal, ok := record.Get("path")
		if !ok {
			return nil, fmt.Errorf("no path in result")
		}

		p := pathVal.(neo4j.Path)
		spr := &graph.ShortestPathResult{Length: len(p.Nodes)}
		for _, n := range p.Nodes {
			spr.Nodes = append(spr.Nodes, nodeFromRecord(n))
		}
		for _, rel := range p.Relationships {
			spr.Edges = append(spr.Edges, edgeFromRelationship(rel))
		}
		return spr, nil
	})
	if err != nil {
		return nil, apperrors.Wrapf(apperrors.ErrCodeGraphError, err, "shortest path query failed")
	}
	return result.(*graph.ShortestPathResult), nil
}

// GetTopFanIn returns nodes with the highest fan-in (most depended upon).
func (r *GraphRepo) GetTopFanIn(ctx context.Context, repoID string, limit int) ([]*graph.GraphNode, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			`MATCH (f:File {repoId: $repoId})
			 RETURN f ORDER BY f.fanIn DESC LIMIT $limit`,
			map[string]any{"repoId": repoID, "limit": limit},
		)
		if err != nil {
			return nil, err
		}
		return collectNodes(ctx, res, "f"), nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]*graph.GraphNode), nil
}

// GetTopFanOut returns nodes with the highest fan-out (most dependencies).
func (r *GraphRepo) GetTopFanOut(ctx context.Context, repoID string, limit int) ([]*graph.GraphNode, error) {
	result, err := r.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			`MATCH (f:File {repoId: $repoId})
			 RETURN f ORDER BY f.fanOut DESC LIMIT $limit`,
			map[string]any{"repoId": repoID, "limit": limit},
		)
		if err != nil {
			return nil, err
		}
		return collectNodes(ctx, res, "f"), nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]*graph.GraphNode), nil
}

// UpdateFanMetrics recalculates fan-in/fan-out for all file nodes in a repo.
func (r *GraphRepo) UpdateFanMetrics(ctx context.Context, repoID string) error {
	queries := []string{
		// Update fan-out: number of files this file imports
		`MATCH (f:File {repoId: $repoId})
		 OPTIONAL MATCH (f)-[:IMPORTS]->(dep:File)
		 WITH f, count(dep) AS fanOut
		 SET f.fanOut = fanOut`,

		// Update fan-in: number of files that import this file
		`MATCH (f:File {repoId: $repoId})
		 OPTIONAL MATCH (importer:File)-[:IMPORTS]->(f)
		 WITH f, count(importer) AS fanIn
		 SET f.fanIn = fanIn`,

		// Update hasChildren for Dir nodes
		`MATCH (d:Dir {repoId: $repoId})
		 OPTIONAL MATCH (d)-[:HAS_CHILD|HAS_FILE]->(child)
		 WITH d, count(child) AS childCount
		 SET d.hasChildren = (childCount > 0)`,
	}

	for _, q := range queries {
		_, err := r.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, q, map[string]any{"repoId": repoID})
			return nil, err
		})
		if err != nil {
			return apperrors.Wrapf(apperrors.ErrCodeGraphError, err, "fan metric update failed")
		}
	}

	return nil
}

// MarkCycleNodes sets hasCycle=true on all nodes that are members of cycles.
func (r *GraphRepo) MarkCycleNodes(
	ctx context.Context,
	repoID string,
	cyclePaths [][]*graph.GraphNode,
) error {
	cycleNodeIDs := map[string]bool{}
	for _, path := range cyclePaths {
		for _, n := range path {
			cycleNodeIDs[n.ID] = true
		}
	}

	if len(cycleNodeIDs) == 0 {
		return nil
	}

	ids := make([]string, 0, len(cycleNodeIDs))
	for id := range cycleNodeIDs {
		ids = append(ids, id)
	}

	_, err := r.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			`UNWIND $ids AS id
			 MATCH (n {id: id})
			 SET n.hasCycle = true`,
			map[string]any{"ids": ids},
		)
		return nil, err
	})
	if err != nil {
		return apperrors.Wrapf(apperrors.ErrCodeGraphError, err, "failed to mark cycle nodes")
	}

	return nil
}

// DeleteRepo removes all nodes and relationships for a repository.
func (r *GraphRepo) DeleteRepo(ctx context.Context, repoID string) error {
	_, err := r.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			`MATCH (n {repoId: $repoId}) DETACH DELETE n`,
			map[string]any{"repoId": repoID},
		)
		return nil, err
	})
	if err != nil {
		return apperrors.Wrapf(apperrors.ErrCodeGraphError, err,
			"failed to delete repo %s from graph", repoID)
	}
	return nil
}

// Ping checks the Neo4j connection.
func (r *GraphRepo) Ping(ctx context.Context) error {
	return r.client.Ping(ctx)
}

// mapNeo4jError converts Neo4j driver errors into typed DomainErrors.
func mapNeo4jError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case neo4j.IsNeo4jError(err):
		neo4jErr, _ := err.(*neo4j.Neo4jError)
		if neo4jErr != nil {
			switch {
			case neo4jErr.Code == "Neo.ClientError.Schema.ConstraintValidationFailed":
				return apperrors.Wrap(apperrors.ErrCodeAlreadyExists, "constraint violation", err)
			case neo4jErr.Code == "Neo.ClientError.Statement.EntityNotFound":
				return apperrors.Wrap(apperrors.ErrCodeNotFound, "entity not found in graph", err)
			}
		}
		return apperrors.Wrap(apperrors.ErrCodeGraphError, "Neo4j error: "+msg, err)
	default:
		return apperrors.Wrap(apperrors.ErrCodeGraphError, "graph database error", err)
	}
}