package parser

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

func ParseRepo(repoURL, cloneDir string) (*GraphData, []FileNode, map[string]string, error) {
	repoPath := filepath.Join(cloneDir, uuid.New().String())
	defer os.RemoveAll(repoPath)

	cmd := exec.Command("git", "clone", "--depth=1", "--single-branch", repoURL, repoPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, nil, nil, &CloneError{Msg: string(out)}
	}

	nodes, importMap := collectNodes(repoPath)
	edges := resolveEdges(nodes, importMap, repoPath)
	fileTree := buildFileTree(repoPath, nodes)
	contents := collectContents(repoPath, nodes)

	return &GraphData{Nodes: nodes, Edges: edges}, fileTree, contents, nil
}

// collectContents reads each file node's raw content while the repo is still on disk.
func collectContents(repoPath string, nodes []Node) map[string]string {
	const maxBytes = 512 * 1024 // 512 KB per file
	result := make(map[string]string, len(nodes))
	for _, n := range nodes {
		if n.Type != "file" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoPath, n.Path))
		if err != nil {
			continue
		}
		if len(data) > maxBytes {
			data = data[:maxBytes]
		}
		result[n.Path] = string(data)
	}
	return result
}
