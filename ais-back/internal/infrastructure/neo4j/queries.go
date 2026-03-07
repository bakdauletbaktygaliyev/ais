package neo4j

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/bakdaulet/ais/ais-back/internal/domain/graph"
)

// ---------------------------------------------------------------------------
// Cypher Queries
// ---------------------------------------------------------------------------

const queryGetRootView = `
MATCH (repo:Repo {id: $repoId})-[:HAS_ROOT]->(root:Dir)
MATCH (root)-[:HAS_CHILD|HAS_FILE]->(child)
OPTIONAL MATCH (child)-[r:IMPORTS]->(dep)
WHERE (root)-[:HAS_CHILD|HAS_FILE]->(dep)
RETURN child, collect(r) AS edges, collect(dep) AS deps
`

const queryGetChildrenView = `
MATCH (parent {id: $nodeId})-[:HAS_CHILD|HAS_FILE]->(child)
OPTIONAL MATCH (child)-[r:IMPORTS]->(dep)
WHERE (parent)-[:HAS_CHILD|HAS_FILE]->(dep)
RETURN child, collect(r) AS edges, collect(dep) AS deps, parent
`

const queryGetImports = `
MATCH (n {id: $id})-[:IMPORTS]->(dep:File)
RETURN dep
`

const queryGetImportedBy = `
MATCH (importer:File)-[:IMPORTS]->(n {id: $id})
RETURN importer
`

const queryGetFunctions = `
MATCH (n {id: $id})-[:HAS_FUNCTION]->(fn:Function)
RETURN fn
`

const queryGetClasses = `
MATCH (n {id: $id})-[:HAS_CLASS]->(cls:Class)
RETURN cls
`

const queryFindCycles = `
MATCH path = (f:File {repoId: $repoId})-[:IMPORTS*2..10]->(f)
RETURN [n IN nodes(path) | n.path] AS cycle
LIMIT 100
`

const queryGetMetrics = `
MATCH (n {repoId: $repoId})
WITH
  count(CASE WHEN 'File' IN labels(n) THEN 1 END) AS fileCount,
  count(CASE WHEN 'Dir' IN labels(n) THEN 1 END) AS dirCount,
  count(CASE WHEN 'Function' IN labels(n) THEN 1 END) AS functionCount,
  count(CASE WHEN 'Class' IN labels(n) THEN 1 END) AS classCount,
  count(CASE WHEN n.hasCycle = true THEN 1 END) AS cycleCount,
  max(CASE WHEN 'File' IN labels(n) THEN coalesce(n.fanIn, 0) END) AS maxFanIn,
  max(CASE WHEN 'File' IN labels(n) THEN coalesce(n.fanOut, 0) END) AS maxFanOut
OPTIONAL MATCH (:File {repoId: $repoId})-[r:IMPORTS]->(:File {repoId: $repoId})
RETURN fileCount, dirCount, functionCount, classCount, cycleCount,
       maxFanIn, maxFanOut, count(r) AS edgeCount
`

const queryShortestPath = `
MATCH (a:File {id: $from}), (b:File {id: $to})
MATCH path = shortestPath((a)-[:IMPORTS*]->(b))
RETURN path
`

// ---------------------------------------------------------------------------
// Query builders
// ---------------------------------------------------------------------------

func buildMergeNodeQuery(nodeType graph.NodeType) string {
	label := string(nodeType)
	return fmt.Sprintf(`
MERGE (n:%s {id: $id})
SET n += {
  repoId: $repoId,
  name: $name,
  path: $path,
  type: $type,
  fanIn: $fanIn,
  fanOut: $fanOut,
  hasCycle: $hasCycle,
  hasChildren: $hasChildren,
  startLine: $startLine,
  endLine: $endLine,
  language: $language,
  size: $size,
  source: $source
}`, label)
}

func buildMergeEdgeQuery(edgeType graph.EdgeType) string {
	switch edgeType {
	case graph.EdgeTypeHasRoot:
		return `
MATCH (src:Repo {id: $sourceId}), (tgt:Dir {id: $targetId})
MERGE (src)-[r:HAS_ROOT]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	case graph.EdgeTypeHasChild:
		return `
MATCH (src:Dir {id: $sourceId}), (tgt:Dir {id: $targetId})
MERGE (src)-[r:HAS_CHILD]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	case graph.EdgeTypeHasFile:
		return `
MATCH (src:Dir {id: $sourceId}), (tgt:File {id: $targetId})
MERGE (src)-[r:HAS_FILE]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	case graph.EdgeTypeHasFunction:
		return `
MATCH (src:File {id: $sourceId}), (tgt:Function {id: $targetId})
MERGE (src)-[r:HAS_FUNCTION]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	case graph.EdgeTypeHasClass:
		return `
MATCH (src:File {id: $sourceId}), (tgt:Class {id: $targetId})
MERGE (src)-[r:HAS_CLASS]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	case graph.EdgeTypeImports:
		return `
MATCH (src:File {id: $sourceId}), (tgt:File {id: $targetId})
MERGE (src)-[r:IMPORTS]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	case graph.EdgeTypeCalls:
		return `
MATCH (src:Function {id: $sourceId}), (tgt:Function {id: $targetId})
MERGE (src)-[r:CALLS]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	case graph.EdgeTypeExtends:
		return `
MATCH (src:Class {id: $sourceId}), (tgt:Class {id: $targetId})
MERGE (src)-[r:EXTENDS]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	case graph.EdgeTypeImplements:
		return `
MATCH (src:Class {id: $sourceId}), (tgt:Class {id: $targetId})
MERGE (src)-[r:IMPLEMENTS]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`

	default:
		return `
MATCH (src {id: $sourceId}), (tgt {id: $targetId})
MERGE (src)-[r:RELATED]->(tgt)
SET r.line = $line, r.edgeId = $edgeId`
	}
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

func nodeToParams(n *graph.GraphNode) map[string]any {
	return map[string]any{
		"id":          n.ID,
		"repoId":      n.RepoID,
		"name":        n.Name,
		"path":        n.Path,
		"type":        string(n.Type),
		"fanIn":       n.FanIn,
		"fanOut":      n.FanOut,
		"hasCycle":    n.HasCycle,
		"hasChildren": n.HasChildren,
		"startLine":   n.StartLine,
		"endLine":     n.EndLine,
		"language":    n.Language,
		"size":        n.Size,
		"source":      "", // source is stored separately via SaveFileSource
	}
}

func nodeToParamsWithSource(n *graph.GraphNode, source string) map[string]any {
	p := nodeToParams(n)
	p["source"] = source
	return p
}

func edgeToParams(e *graph.GraphEdge) map[string]any {
	return map[string]any{
		"edgeId":   e.ID,
		"sourceId": e.SourceID,
		"targetId": e.TargetID,
		"line":     e.Line,
	}
}

func nodeFromRecord(n neo4j.Node) *graph.GraphNode {
	props := n.Props
	node := &graph.GraphNode{}

	if v, ok := props["id"]; ok && v != nil {
		node.ID = v.(string)
	}
	if v, ok := props["repoId"]; ok && v != nil {
		node.RepoID = v.(string)
	}
	if v, ok := props["name"]; ok && v != nil {
		node.Name = v.(string)
	}
	if v, ok := props["path"]; ok && v != nil {
		node.Path = v.(string)
	}
	if v, ok := props["type"]; ok && v != nil {
		node.Type = graph.NodeType(v.(string))
	} else {
		// Infer from labels
		for _, label := range n.Labels {
			switch label {
			case "Repo", "Dir", "File", "Function", "Class":
				node.Type = graph.NodeType(label)
			}
		}
	}
	if v, ok := props["fanIn"]; ok && v != nil {
		node.FanIn = int(toInt64(v))
	}
	if v, ok := props["fanOut"]; ok && v != nil {
		node.FanOut = int(toInt64(v))
	}
	if v, ok := props["hasCycle"]; ok && v != nil {
		node.HasCycle, _ = v.(bool)
	}
	if v, ok := props["hasChildren"]; ok && v != nil {
		node.HasChildren, _ = v.(bool)
	}
	if v, ok := props["startLine"]; ok && v != nil {
		node.StartLine = int(toInt64(v))
	}
	if v, ok := props["endLine"]; ok && v != nil {
		node.EndLine = int(toInt64(v))
	}
	if v, ok := props["language"]; ok && v != nil {
		node.Language = v.(string)
	}
	if v, ok := props["size"]; ok && v != nil {
		node.Size = toInt64(v)
	}

	return node
}

func edgeFromRelationship(rel neo4j.Relationship) *graph.GraphEdge {
	edge := &graph.GraphEdge{
		Type: graph.EdgeType(rel.Type),
	}
	if v, ok := rel.Props["edgeId"]; ok && v != nil {
		edge.ID = v.(string)
	}
	if v, ok := rel.Props["line"]; ok && v != nil {
		edge.Line = int(toInt64(v))
	}
	return edge
}

func collectNodes(ctx context.Context, res neo4j.ResultWithContext, key string) []*graph.GraphNode {
	var nodes []*graph.GraphNode
	records, err := res.Collect(ctx)
	if err != nil {
		return nodes
	}
	for _, record := range records {
		val, ok := record.Get(key)
		if !ok || val == nil {
			continue
		}
		nodes = append(nodes, nodeFromRecord(val.(neo4j.Node)))
	}
	return nodes
}

func collectGraphView(ctx context.Context, res neo4j.ResultWithContext) (*graph.GraphView, error) {
	view := &graph.GraphView{}
	edgeSeen := map[string]bool{}

	records, err := res.Collect(ctx)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		childVal, ok := record.Get("child")
		if !ok || childVal == nil {
			continue
		}
		child := nodeFromRecord(childVal.(neo4j.Node))
		view.Nodes = append(view.Nodes, child)

		// Collect edges within this subgraph
		edgesVal, _ := record.Get("edges")
		if edgesVal != nil {
			if rels, ok := edgesVal.([]any); ok {
				for _, relAny := range rels {
					if rel, ok := relAny.(neo4j.Relationship); ok {
						key := fmt.Sprintf("%d", rel.Id)
						if !edgeSeen[key] {
							edgeSeen[key] = true
							view.Edges = append(view.Edges, edgeFromRelationship(rel))
						}
					}
				}
			}
		}

		// Parent node
		if parentVal, ok := record.Get("parent"); ok && parentVal != nil {
			view.Parent = nodeFromRecord(parentVal.(neo4j.Node))
		}
	}

	return view, nil
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok && !strings.Contains(s, "nil") {
			result = append(result, s)
		}
	}
	return result
}
