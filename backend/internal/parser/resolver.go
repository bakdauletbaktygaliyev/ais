package parser

import (
	"path/filepath"
	"strings"
)

func resolveEdges(nodes []Node, importMap map[string][]string, repoPath string) []Edge {
	pathIndex := map[string]bool{}
	for _, n := range nodes {
		pathIndex[n.Path] = true
	}

	var edges []Edge
	seenEdges := map[string]bool{}

	for _, n := range nodes {
		if n.Type != "file" {
			continue
		}
		imports := importMap[n.Path]
		for _, imp := range imports {
			resolved := resolveImport(n.Path, imp, pathIndex, repoPath)
			if resolved == "" || resolved == n.Path {
				continue
			}
			key := n.Path + "->" + resolved
			if seenEdges[key] {
				continue
			}
			seenEdges[key] = true
			edges = append(edges, Edge{
				Source: n.Path,
				Target: resolved,
				Type:   "import",
			})
		}
	}

	return edges
}

func resolveImport(fromPath, importPath string, pathIndex map[string]bool, repoPath string) string {
	if importPath == "" {
		return ""
	}

	fromDir := filepath.Dir(fromPath)

	// Try relative path
	if strings.HasPrefix(importPath, ".") {
		candidate := filepath.Join(fromDir, importPath)
		candidate = filepath.Clean(candidate)
		if pathIndex[candidate] {
			return candidate
		}
		for _, ext := range []string{".go", ".py", ".ts", ".js", ".tsx", ".jsx"} {
			if pathIndex[candidate+ext] {
				return candidate + ext
			}
		}
		if pathIndex[candidate+"/index.ts"] {
			return candidate + "/index.ts"
		}
		if pathIndex[candidate+"/index.js"] {
			return candidate + "/index.js"
		}
	}

	// Try matching by last component
	parts := strings.Split(strings.ReplaceAll(importPath, "\\", "/"), "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		for path := range pathIndex {
			base := filepath.Base(path)
			withoutExt := strings.TrimSuffix(base, filepath.Ext(base))
			if withoutExt == last || base == last {
				return path
			}
		}
	}

	return ""
}
