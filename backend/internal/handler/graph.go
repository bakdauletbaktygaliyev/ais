package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ais/backend/internal/parser"
	"github.com/gin-gonic/gin"
)

func (h *Handler) GetGraph(c *gin.Context) {
	userID := c.GetString("user_id")
	row := h.db.QueryRow(
		`SELECT graph_data FROM projects WHERE id = $1 AND user_id = $2`, c.Param("id"), userID,
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

// filterGraph dispatches to the appropriate filter based on path.
func filterGraph(graph parser.GraphData, path string) parser.GraphData {
	if path == "" || path == "/" || path == "." {
		return filterByDepth(graph, 0)
	}
	return filterByParentPath(graph, path)
}

// filterByDepth returns nodes at targetDepth and aggregates import edges
// between their depth=0 ancestors (cross-module dependencies).
func filterByDepth(graph parser.GraphData, targetDepth int) parser.GraphData {
	visible := map[string]bool{}
	var nodes []parser.Node
	for _, n := range graph.Nodes {
		if n.Depth == targetDepth {
			nodes = append(nodes, n)
			visible[n.ID] = true
		}
	}

	ancestorOf := buildAncestorMap(graph.Nodes, targetDepth, visible)
	edges := aggregateEdges(graph.Edges, ancestorOf)

	return parser.GraphData{
		Nodes: nodesOrEmpty(nodes),
		Edges: edgesOrEmpty(edges),
	}
}

// filterByParentPath returns direct children of dirPath and aggregates
// import edges between those children.
func filterByParentPath(graph parser.GraphData, dirPath string) parser.GraphData {
	visible := map[string]bool{}
	var nodes []parser.Node
	for _, n := range graph.Nodes {
		parent := filepath.Dir(n.Path)
		if parent == "." {
			parent = ""
		}
		if parent == dirPath {
			nodes = append(nodes, n)
			visible[n.ID] = true
		}
	}

	// Map every node inside dirPath to its direct child of dirPath.
	ancestorOf := map[string]string{}
	prefix := dirPath + "/"
	for _, n := range graph.Nodes {
		parent := filepath.Dir(n.Path)
		if parent == "." {
			parent = ""
		}
		if parent == dirPath {
			ancestorOf[n.ID] = n.ID
		} else if strings.HasPrefix(n.Path, prefix) {
			rest := n.Path[len(prefix):]
			first := strings.SplitN(rest, "/", 2)[0]
			anc := prefix + first
			if visible[anc] {
				ancestorOf[n.ID] = anc
			}
		}
	}

	return parser.GraphData{
		Nodes: nodesOrEmpty(nodes),
		Edges: edgesOrEmpty(aggregateEdges(graph.Edges, ancestorOf)),
	}
}

// ancestorAtDepth walks a node's path to find its ancestor at targetDepth.
func ancestorAtDepth(nodePath string, nodeDepth, targetDepth int, visible map[string]bool) string {
	if nodeDepth == targetDepth {
		return nodePath
	}
	if nodeDepth > targetDepth {
		parts := strings.Split(nodePath, "/")
		if len(parts) > targetDepth {
			anc := strings.Join(parts[:targetDepth+1], "/")
			if visible[anc] {
				return anc
			}
		}
	}
	return ""
}

// buildAncestorMap maps every node ID to its ancestor at targetDepth.
func buildAncestorMap(nodes []parser.Node, targetDepth int, visible map[string]bool) map[string]string {
	m := map[string]string{}
	for _, n := range nodes {
		if anc := ancestorAtDepth(n.Path, n.Depth, targetDepth, visible); anc != "" {
			m[n.ID] = anc
		}
	}
	return m
}

// aggregateEdges deduplicates and lifts edges to visible ancestor nodes.
func aggregateEdges(edges []parser.Edge, ancestorOf map[string]string) []parser.Edge {
	seen := map[string]bool{}
	var result []parser.Edge
	for _, e := range edges {
		src, tgt := ancestorOf[e.Source], ancestorOf[e.Target]
		if src == "" || tgt == "" || src == tgt {
			continue
		}
		key := src + "→" + tgt
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, parser.Edge{Source: src, Target: tgt, Type: e.Type})
	}
	return result
}

func nodesOrEmpty(n []parser.Node) []parser.Node {
	if n == nil {
		return []parser.Node{}
	}
	return n
}

func edgesOrEmpty(e []parser.Edge) []parser.Edge {
	if e == nil {
		return []parser.Edge{}
	}
	return e
}
