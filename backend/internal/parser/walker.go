package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type nodeInfo struct {
	node    Node
	imports []string
}

func collectNodes(repoPath string) ([]Node, map[string][]string) {
	var nodes []Node
	importMap := map[string][]string{}
	dirChildCount := map[string]int{}

	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()
		if info.IsDir() && skipDirs[name] {
			return filepath.SkipDir
		}

		relPath, _ := filepath.Rel(repoPath, path)
		if relPath == "." {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(name))
		if !info.IsDir() && skipExts[ext] {
			return nil
		}

		depth := strings.Count(relPath, string(filepath.Separator))
		parent := filepath.Dir(relPath)
		if parent != "." {
			dirChildCount[parent]++
		} else {
			dirChildCount[""]++
		}

		n := Node{
			ID:    relPath,
			Name:  name,
			Path:  relPath,
			Depth: depth,
		}

		if info.IsDir() {
			n.Type = "directory"
		} else {
			n.Type = "file"
			n.Size = info.Size()
			n.Language = detectLanguage(name)
			lines, imports := extractFileInfo(path, n.Language)
			n.Lines = lines
			importMap[relPath] = imports
		}

		nodes = append(nodes, n)
		return nil
	})

	for i := range nodes {
		if nodes[i].Type == "directory" {
			nodes[i].Children = dirChildCount[nodes[i].Path]
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Depth != nodes[j].Depth {
			return nodes[i].Depth < nodes[j].Depth
		}
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "directory"
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, importMap
}

func extractFileInfo(path, language string) (int, []string) {
	f, err := os.Open(path)
	if err != nil {
		return 0, nil
	}
	defer f.Close()

	var lines int
	var imports []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		lines++
		if lines > 5000 {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if imp := extractImport(line, language); imp != "" {
			imports = append(imports, imp)
		}
	}

	return lines, imports
}
