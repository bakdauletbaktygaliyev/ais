package parser

import (
	"path/filepath"
	"regexp"
	"strings"
)

var langMap = map[string]string{
	".go":    "go",
	".py":    "python",
	".ts":    "typescript",
	".tsx":   "typescript",
	".js":    "javascript",
	".jsx":   "javascript",
	".java":  "java",
	".rs":    "rust",
	".cpp":   "cpp",
	".c":     "c",
	".h":     "c",
	".cs":    "csharp",
	".rb":    "ruby",
	".php":   "php",
	".swift": "swift",
	".kt":    "kotlin",
	".scala": "scala",
	".sh":    "shell",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".md":    "markdown",
	".html":  "html",
	".css":   "css",
	".scss":  "scss",
	".sql":   "sql",
	".proto": "protobuf",
	".tf":    "terraform",
	".toml":  "toml",
	".xml":   "xml",
	".dart":  "dart",
	".vue":   "vue",
}

func detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	base := strings.ToLower(filename)
	switch base {
	case "dockerfile":
		return "dockerfile"
	case "makefile", "gnumakefile":
		return "makefile"
	case ".gitignore", ".dockerignore":
		return "ignore"
	case "jenkinsfile":
		return "groovy"
	}
	return "text"
}

var (
	goImportRe      = regexp.MustCompile(`"([^"]+)"`)
	pyImportRe      = regexp.MustCompile(`^(?:from\s+([\w.]+)\s+import|import\s+([\w.]+))`)
	jsImportRe      = regexp.MustCompile(`(?:import\s+.*?\s+from\s+['"]([^'"]+)['"]|require\(['"]([^'"]+)['"]\))`)
	javaImportRe    = regexp.MustCompile(`^import\s+([\w.]+);`)
	rustUseRe       = regexp.MustCompile(`^use\s+([\w:]+)`)
	cIncludeLocalRe = regexp.MustCompile(`^#include\s+"([^"]+)"`)
)

func extractImport(line, language string) string {
	line = strings.TrimSpace(line)
	switch language {
	case "go":
		if strings.HasPrefix(line, "import") || strings.HasPrefix(line, `"`) {
			matches := goImportRe.FindStringSubmatch(line)
			if len(matches) > 1 {
				return matches[1]
			}
		}
	case "python":
		matches := pyImportRe.FindStringSubmatch(line)
		if len(matches) > 2 {
			if matches[1] != "" {
				return strings.ReplaceAll(matches[1], ".", "/")
			}
			if matches[2] != "" {
				return strings.ReplaceAll(matches[2], ".", "/")
			}
		}
	case "javascript", "typescript":
		matches := jsImportRe.FindStringSubmatch(line)
		if len(matches) > 2 {
			if matches[1] != "" {
				return matches[1]
			}
			if matches[2] != "" {
				return matches[2]
			}
		}
	case "java":
		matches := javaImportRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			return strings.ReplaceAll(matches[1], ".", "/")
		}
	case "rust":
		matches := rustUseRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			return strings.ReplaceAll(matches[1], "::", "/")
		}
	case "c", "cpp":
		matches := cIncludeLocalRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}
