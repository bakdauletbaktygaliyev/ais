package parser

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

func ParseRepo(repoURL, cloneDir string) (*GraphData, []FileNode, error) {
	repoPath := filepath.Join(cloneDir, uuid.New().String())
	defer os.RemoveAll(repoPath)

	cmd := exec.Command("git", "clone", "--depth=1", "--single-branch", repoURL, repoPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, nil, &CloneError{Msg: string(out)}
	}

	nodes, importMap := collectNodes(repoPath)
	edges := resolveEdges(nodes, importMap, repoPath)
	fileTree := buildFileTree(repoPath, nodes)

	return &GraphData{Nodes: nodes, Edges: edges}, fileTree, nil
}
