package parser

import "path/filepath"

func buildFileTree(repoPath string, nodes []Node) []FileNode {
	nodeMap := map[string]*FileNode{}
	var roots []FileNode

	for _, n := range nodes {
		fn := FileNode{
			Path:     n.Path,
			Name:     n.Name,
			Type:     n.Type,
			Language: n.Language,
			Size:     n.Size,
			Lines:    n.Lines,
		}
		nodeMap[n.Path] = &fn
	}

	for _, n := range nodes {
		fn := nodeMap[n.Path]
		parent := filepath.Dir(n.Path)
		if parent == "." {
			roots = append(roots, *fn)
		} else {
			if parentNode, ok := nodeMap[parent]; ok {
				parentNode.Children = append(parentNode.Children, *fn)
			}
		}
	}

	return roots
}
