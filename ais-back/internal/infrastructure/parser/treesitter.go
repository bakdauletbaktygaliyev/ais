package parser

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/bakdaulet/ais/ais-back/internal/domain/analysis"
	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
	"go.uber.org/zap"
)

// extensionLanguageMap maps file extensions to languages.
var extensionLanguageMap = map[string]domainrepo.Language{
	".ts":  domainrepo.LanguageTypeScript,
	".tsx": domainrepo.LanguageTypeScript,
	".js":  domainrepo.LanguageJavaScript,
	".jsx": domainrepo.LanguageJavaScript,
	".mjs": domainrepo.LanguageJavaScript,
	".cjs": domainrepo.LanguageJavaScript,
	".go":  domainrepo.LanguageGo,
}

// TreeSitterParser implements the analysis.Parser port using Tree-sitter grammars.
type TreeSitterParser struct {
	tsParser *TypeScriptParser
	jsParser *JavaScriptParser
	goParser *GoParser
	log      *logger.Logger
}

// NewTreeSitterParser constructs a TreeSitterParser with all language sub-parsers.
func NewTreeSitterParser(log *logger.Logger) (*TreeSitterParser, error) {
	l := log.WithComponent("treesitter_parser")

	tsP, err := NewTypeScriptParser(l)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to init TypeScript parser", err)
	}

	jsP, err := NewJavaScriptParser(l)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to init JavaScript parser", err)
	}

	goP, err := NewGoParser(l)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrCodeInternalError, "failed to init Go parser", err)
	}

	return &TreeSitterParser{
		tsParser: tsP,
		jsParser: jsP,
		goParser: goP,
		log:      l,
	}, nil
}

// ParseFile parses a single source file and extracts AST information.
func (p *TreeSitterParser) ParseFile(
	ctx context.Context,
	path string,
	content []byte,
	lang domainrepo.Language,
) (*analysis.ParsedFile, error) {
	if len(content) == 0 {
		return &analysis.ParsedFile{
			Path:     path,
			Language: lang,
		}, nil
	}

	var result *analysis.ParsedFile
	var err error

	switch lang {
	case domainrepo.LanguageTypeScript:
		result, err = p.tsParser.Parse(ctx, path, content)
	case domainrepo.LanguageJavaScript:
		result, err = p.jsParser.Parse(ctx, path, content)
	case domainrepo.LanguageGo:
		result, err = p.goParser.Parse(ctx, path, content)
	default:
		return nil, apperrors.Newf(apperrors.ErrCodeParseError, "unsupported language: %s", lang)
	}

	if err != nil {
		p.log.Warn("parse error (non-fatal)",
			zap.String("path", path),
			zap.String("lang", string(lang)),
			zap.Error(err),
		)
		// Return partial result if available
		if result != nil {
			return result, nil
		}
		return &analysis.ParsedFile{
			Path:      path,
			Language:  lang,
			RawSource: string(content),
		}, nil
	}

	result.RawSource = string(content)
	result.SizeBytes = int64(len(content))
	return result, nil
}

// SupportedExtensions returns all supported file extensions.
func (p *TreeSitterParser) SupportedExtensions() []string {
	exts := make([]string, 0, len(extensionLanguageMap))
	for ext := range extensionLanguageMap {
		exts = append(exts, ext)
	}
	return exts
}

// DetectLanguage returns the language for a given file extension.
func (p *TreeSitterParser) DetectLanguage(ext string) (domainrepo.Language, bool) {
	lang, ok := extensionLanguageMap[strings.ToLower(ext)]
	return lang, ok
}

// ---------------------------------------------------------------------------
// Language Detector
// ---------------------------------------------------------------------------

// LangDetector implements analysis.LangDetector by walking file extensions.
type LangDetector struct {
	log *logger.Logger
}

// NewLangDetector creates a new language detector.
func NewLangDetector(log *logger.Logger) *LangDetector {
	return &LangDetector{log: log.WithComponent("lang_detector")}
}

// Detect scans the repository directory to determine the primary language.
func (d *LangDetector) Detect(
	ctx context.Context,
	repoPath string,
) (domainrepo.Language, domainrepo.MonorepoType, error) {
	counts := map[domainrepo.Language]int{}
	monorepoType := domainrepo.MonorepoNone

	err := walkForDetection(repoPath, func(path string, isDir bool) error {
		if isDir {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if lang, ok := extensionLanguageMap[ext]; ok {
			counts[lang]++
		}

		// Detect monorepo tooling
		base := filepath.Base(path)
		switch base {
		case "turbo.json":
			monorepoType = domainrepo.MonorepoTurborepo
		case "nx.json":
			if monorepoType == domainrepo.MonorepoNone {
				monorepoType = domainrepo.MonorepoNX
			}
		case "pnpm-workspace.yaml":
			if monorepoType == domainrepo.MonorepoNone {
				monorepoType = domainrepo.MonorepoPnpm
			}
		}

		return nil
	})
	if err != nil {
		return domainrepo.LanguageUnknown, monorepoType,
			apperrors.Wrapf(apperrors.ErrCodeInternalError, err, "language detection walk failed")
	}

	// Check for Go modules
	if _, err := checkFileExists(filepath.Join(repoPath, "go.mod")); err == nil {
		monorepoType = domainrepo.MonorepoGoModules
	}

	// Determine dominant language
	primary := domainrepo.LanguageUnknown
	maxCount := 0
	for lang, count := range counts {
		if count > maxCount {
			maxCount = count
			primary = lang
		}
	}

	// Check if mixed (TypeScript + JavaScript commonly coexist, treat as TS)
	tsCount := counts[domainrepo.LanguageTypeScript]
	jsCount := counts[domainrepo.LanguageJavaScript]
	goCount := counts[domainrepo.LanguageGo]

	if tsCount > 0 && jsCount > 0 && goCount == 0 {
		// TypeScript project with some JS files — primary is TS
		primary = domainrepo.LanguageTypeScript
	} else if tsCount > 0 && goCount > 0 {
		primary = domainrepo.LanguageMixed
	} else if jsCount > 0 && goCount > 0 {
		primary = domainrepo.LanguageMixed
	}

	d.log.Info("language detected",
		zap.String("primary", string(primary)),
		zap.String("monorepo", string(monorepoType)),
		zap.Int("ts_files", tsCount),
		zap.Int("js_files", jsCount),
		zap.Int("go_files", goCount),
	)

	return primary, monorepoType, nil
}

// ---------------------------------------------------------------------------
// FSWalker
// ---------------------------------------------------------------------------

// FSWalker implements analysis.FSWalker using filepath.Walk.
type FSWalker struct {
	maxFileSize int64
	log         *logger.Logger
}

// NewFSWalker creates a new filesystem walker.
func NewFSWalker(maxFileSize int64, log *logger.Logger) *FSWalker {
	return &FSWalker{
		maxFileSize: maxFileSize,
		log:         log.WithComponent("fs_walker"),
	}
}

// Walk traverses the directory tree, skipping specified directories.
// Returns channels for entries and errors.
func (w *FSWalker) Walk(
	ctx context.Context,
	repoPath string,
	skipDirs []string,
) (<-chan *analysis.FSEntry, <-chan error) {
	entryCh := make(chan *analysis.FSEntry, 256)
	errCh := make(chan error, 1)

	skipSet := make(map[string]bool, len(skipDirs))
	for _, d := range skipDirs {
		skipSet[d] = true
	}

	go func() {
		defer close(entryCh)
		defer close(errCh)

		err := walkDirectory(ctx, repoPath, repoPath, skipSet, w.maxFileSize, func(entry *analysis.FSEntry) {
			select {
			case entryCh <- entry:
			case <-ctx.Done():
			}
		})

		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	return entryCh, errCh
}
