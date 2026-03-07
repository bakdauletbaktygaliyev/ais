package parser

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bakdaulet/ais/ais-back/internal/domain/analysis"
	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
)

// walkDirectory recursively walks a directory, calling fn for each entry.
func walkDirectory(
	ctx context.Context,
	rootPath string,
	currentPath string,
	skipSet map[string]bool,
	maxFileSize int64,
	fn func(*analysis.FSEntry),
) error {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return err
	}

	for _, de := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		name := de.Name()

		// Skip hidden files and directories
		if strings.HasPrefix(name, ".") && name != ".env" {
			continue
		}

		// Skip configured directories
		if de.IsDir() && skipSet[name] {
			continue
		}

		fullPath := filepath.Join(currentPath, name)
		relPath, _ := filepath.Rel(rootPath, fullPath)
		relPath = "/" + filepath.ToSlash(relPath)

		if de.IsDir() {
			fn(&analysis.FSEntry{
				Path:  relPath,
				Name:  name,
				IsDir: true,
			})
			// Recurse
			if err := walkDirectory(ctx, rootPath, fullPath, skipSet, maxFileSize, fn); err != nil {
				return err
			}
		} else {
			info, err := de.Info()
			if err != nil {
				continue
			}

			// Skip files exceeding size limit
			if maxFileSize > 0 && info.Size() > maxFileSize {
				continue
			}

			ext := strings.ToLower(filepath.Ext(name))
			lang, _ := detectLangFromExt(ext)

			fn(&analysis.FSEntry{
				Path:      relPath,
				Name:      name,
				IsDir:     false,
				SizeBytes: info.Size(),
				Language:  lang,
			})
		}
	}

	return nil
}

// walkForDetection performs a simple walk for language detection without channels.
func walkForDetection(rootPath string, fn func(path string, isDir bool) error) error {
	return filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		name := d.Name()

		// Skip hidden directories and known non-code dirs
		if d.IsDir() {
			skip := map[string]bool{
				"node_modules": true,
				".git":         true,
				"vendor":       true,
				"dist":         true,
				".next":        true,
				"build":        true,
				".cache":       true,
				"__pycache__":  true,
				"target":       true,
				"out":          true,
				"coverage":     true,
			}
			if skip[name] || (len(name) > 0 && name[0] == '.') {
				return filepath.SkipDir
			}
		}

		return fn(path, d.IsDir())
	})
}

// checkFileExists returns nil if the file exists.
func checkFileExists(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func detectLangFromExt(ext string) (domainrepo.Language, bool) {
	m := map[string]domainrepo.Language{
		".ts":  domainrepo.LanguageTypeScript,
		".tsx": domainrepo.LanguageTypeScript,
		".js":  domainrepo.LanguageJavaScript,
		".jsx": domainrepo.LanguageJavaScript,
		".mjs": domainrepo.LanguageJavaScript,
		".cjs": domainrepo.LanguageJavaScript,
		".go":  domainrepo.LanguageGo,
	}
	lang, ok := m[ext]
	return lang, ok
}
