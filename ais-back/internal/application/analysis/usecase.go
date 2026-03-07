package analysis

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	domainai "github.com/bakdaulet/ais/ais-back/internal/domain/ai"
	"github.com/bakdaulet/ais/ais-back/internal/domain/analysis"
	"github.com/bakdaulet/ais/ais-back/internal/domain/graph"
	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

// UseCase orchestrates the full repository analysis pipeline.
type UseCase struct {
	repoRepo    domainrepo.RepoRepository
	cacheRepo   domainrepo.CacheRepository
	graphRepo   graph.GraphRepository
	gitHubClient domainrepo.GitHubClient
	cloner      analysis.Cloner
	fsWalker    analysis.FSWalker
	langDetector analysis.LangDetector
	parser      analysis.Parser
	aiClient    domainai.Client
	cloneBase   string
	skipDirs    []string
	parseWorkers int
	maxFileSizeBytes int64
	log         *logger.Logger
}

// UseCaseConfig holds all dependencies for the analysis use case.
type UseCaseConfig struct {
	RepoRepo         domainrepo.RepoRepository
	CacheRepo        domainrepo.CacheRepository
	GraphRepo        graph.GraphRepository
	GitHubClient     domainrepo.GitHubClient
	Cloner           analysis.Cloner
	FSWalker         analysis.FSWalker
	LangDetector     analysis.LangDetector
	Parser           analysis.Parser
	AIClient         domainai.Client
	CloneBase        string
	SkipDirs         []string
	ParseWorkers     int
	MaxFileSizeBytes int64
	Log              *logger.Logger
}

// NewUseCase creates a new analysis use case with all required dependencies.
func NewUseCase(cfg UseCaseConfig) *UseCase {
	return &UseCase{
		repoRepo:         cfg.RepoRepo,
		cacheRepo:        cfg.CacheRepo,
		graphRepo:        cfg.GraphRepo,
		gitHubClient:     cfg.GitHubClient,
		cloner:           cfg.Cloner,
		fsWalker:         cfg.FSWalker,
		langDetector:     cfg.LangDetector,
		parser:           cfg.Parser,
		aiClient:         cfg.AIClient,
		cloneBase:        cfg.CloneBase,
		skipDirs:         cfg.SkipDirs,
		parseWorkers:     cfg.ParseWorkers,
		maxFileSizeBytes: cfg.MaxFileSizeBytes,
		log:              cfg.Log.WithComponent("analysis_usecase"),
	}
}

// StartAnalysis initiates an async analysis pipeline for a GitHub URL.
// Returns the repo entity immediately and runs the pipeline in a background goroutine.
func (u *UseCase) StartAnalysis(
	ctx context.Context,
	repoURL string,
	emitter analysis.ProgressEmitter,
) (*domainrepo.Repo, error) {
	repoID := uuid.New().String()

	// Create placeholder repo entity
	repo := &domainrepo.Repo{
		ID:        repoID,
		URL:       repoURL,
		Status:    domainrepo.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := u.repoRepo.Create(ctx, repo); err != nil {
		return nil, fmt.Errorf("failed to create repo record: %w", err)
	}

	// Run pipeline async
	go func() {
		pipelineCtx, cancel := context.WithTimeout(
			context.Background(), 30*time.Minute,
		)
		defer cancel()

		if err := u.runPipeline(pipelineCtx, repo, emitter); err != nil {
			u.log.Error("analysis pipeline failed",
				zap.String("repo_id", repoID),
				zap.Error(err),
			)

			repo.Status = domainrepo.StatusError
			repo.ErrorMessage = err.Error()
			repo.UpdatedAt = time.Now()

			updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = u.repoRepo.Update(updateCtx, repo)

			emitter.Emit(&analysis.ProgressEvent{
				RepoID:  repoID,
				Step:    analysis.StepError,
				Progress: 0,
				Message: "Analysis failed",
				Error:   err.Error(),
				At:      time.Now(),
			})
		}
	}()

	return repo, nil
}

// GetRepo retrieves a repository by ID, checking cache first.
func (u *UseCase) GetRepo(ctx context.Context, repoID string) (*domainrepo.Repo, error) {
	repo, err := u.repoRepo.GetByID(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// runPipeline executes all pipeline steps sequentially.
func (u *UseCase) runPipeline(
	ctx context.Context,
	repo *domainrepo.Repo,
	emitter analysis.ProgressEmitter,
) error {
	log := u.log.WithRepoID(repo.ID)
	emit := func(step analysis.PipelineStep, progress int, msg string) {
		emitter.Emit(&analysis.ProgressEvent{
			RepoID:   repo.ID,
			Step:     step,
			Progress: progress,
			Message:  msg,
			At:       time.Now(),
		})
	}

	// Update status to analyzing
	repo.Status = domainrepo.StatusAnalyzing
	repo.UpdatedAt = time.Now()
	if err := u.repoRepo.Update(ctx, repo); err != nil {
		log.Warn("failed to update repo status", zap.Error(err))
	}

	// ---- Step 1: Validate ----
	emit(analysis.StepValidate, 0, "Validating repository...")
	log.Info("step 1: validating repository")

	metadata, err := u.gitHubClient.ValidateRepo(ctx, repo.URL, 500)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	repo.Owner = metadata.Owner
	repo.Name = metadata.Name
	repo.DefaultBranch = metadata.DefaultBranch
	repo.Description = metadata.Description
	repo.StarCount = metadata.StarCount
	repo.ForkCount = metadata.ForkCount
	repo.SizeKB = metadata.SizeKB

	// Check commit hash cache
	commitHash, err := u.gitHubClient.GetLatestCommit(ctx, metadata.Owner, metadata.Name, metadata.DefaultBranch)
	if err != nil {
		log.Warn("failed to get latest commit, proceeding without cache", zap.Error(err))
		commitHash = uuid.New().String() // fallback
	}

	if commitHash != "" {
		existingRepoID, _ := u.cacheRepo.GetRepoIDByCommitHash(ctx, commitHash)
		if existingRepoID != "" && existingRepoID != repo.ID {
			// This commit was already analyzed — copy/reuse
			log.Info("cache hit for commit hash, returning cached result",
				zap.String("commit_hash", commitHash),
				zap.String("cached_repo_id", existingRepoID),
			)
			emit(analysis.StepDone, 100, "Retrieved from cache")

			// Update repo to point at cached result
			cachedRepo, err := u.repoRepo.GetByID(ctx, existingRepoID)
			if err == nil {
				repo.CommitHash = cachedRepo.CommitHash
				repo.Status = domainrepo.StatusReady
				repo.FileCount = cachedRepo.FileCount
				repo.DirCount = cachedRepo.DirCount
				repo.FunctionCount = cachedRepo.FunctionCount
				repo.ClassCount = cachedRepo.ClassCount
				now := time.Now()
				repo.ReadyAt = &now
				repo.UpdatedAt = now
				_ = u.repoRepo.Update(ctx, repo)
			}
			return nil
		}
	}

	repo.CommitHash = commitHash
	_ = u.repoRepo.Update(ctx, repo)

	emit(analysis.StepValidate, 5, fmt.Sprintf("Repository validated: %s/%s", metadata.Owner, metadata.Name))

	// ---- Step 2: Clone ----
	emit(analysis.StepClone, 5, "Cloning repository...")
	log.Info("step 2: cloning repository",
		zap.String("url", metadata.CloneURL))

	clonePath := filepath.Join(u.cloneBase, repo.ID)
	cloneURL := metadata.CloneURL

	actualCommitHash, err := u.cloner.Clone(ctx, cloneURL, clonePath)
	if err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	if actualCommitHash != "" {
		repo.CommitHash = actualCommitHash
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := u.cloner.Cleanup(cleanupCtx, clonePath); err != nil {
			log.Warn("failed to cleanup clone directory", zap.Error(err))
		}
	}()

	emit(analysis.StepClone, 20, "Repository cloned successfully")

	// ---- Step 3: Detect language ----
	emit(analysis.StepDetect, 20, "Detecting language and monorepo type...")
	log.Info("step 3: detecting language")

	lang, monorepoType, err := u.langDetector.Detect(ctx, clonePath)
	if err != nil {
		log.Warn("language detection failed, defaulting to mixed", zap.Error(err))
		lang = domainrepo.LanguageMixed
	}

	repo.Language = lang
	repo.MonorepoType = monorepoType

	emit(analysis.StepDetect, 30, fmt.Sprintf("Detected language: %s (%s)", lang, monorepoType))
	log.Info("language detected",
		zap.String("language", string(lang)),
		zap.String("monorepo", string(monorepoType)),
	)

	// ---- Step 4: Walk filesystem ----
	emit(analysis.StepWalkFS, 30, "Traversing directory structure...")
	log.Info("step 4: walking filesystem")

	// Create repo node in graph
	repoNode := &graph.GraphNode{
		ID:     "repo:" + repo.ID,
		RepoID: repo.ID,
		Type:   graph.NodeTypeRepo,
		Name:   repo.Name,
		Path:   "/",
	}
	if err := u.graphRepo.SaveNode(ctx, repoNode); err != nil {
		return fmt.Errorf("failed to save repo node: %w", err)
	}

	// Walk FS and build directory/file nodes
	dirNodes, fileNodes, dirEdges, err := u.walkAndBuildGraph(ctx, clonePath, repo, repoNode.ID, emit)
	if err != nil {
		return fmt.Errorf("filesystem walk failed: %w", err)
	}

	repo.DirCount = len(dirNodes)
	repo.FileCount = len(fileNodes)

	emit(analysis.StepWalkFS, 50, fmt.Sprintf("Found %d directories, %d files", len(dirNodes), len(fileNodes)))
	log.Info("filesystem walk complete",
		zap.Int("dirs", len(dirNodes)),
		zap.Int("files", len(fileNodes)),
	)

	// ---- Step 5: Parse AST ----
	emit(analysis.StepParseAST, 50, fmt.Sprintf("Parsing %d source files...", len(fileNodes)))
	log.Info("step 5: parsing AST")

	parsedFiles, functionNodes, classNodes, structEdges, err := u.parseSourceFiles(
		ctx, clonePath, repo, fileNodes, dirEdges, emit,
	)
	if err != nil {
		return fmt.Errorf("AST parsing failed: %w", err)
	}

	repo.FunctionCount = len(functionNodes)
	repo.ClassCount = len(classNodes)

	emit(analysis.StepParseAST, 70, fmt.Sprintf(
		"Parsed %d files: %d functions, %d classes",
		len(parsedFiles), len(functionNodes), len(classNodes),
	))

	// ---- Step 6: Build graph ----
	emit(analysis.StepBuildGraph, 70, "Building dependency graph...")
	log.Info("step 6: building dependency graph")

	importEdges, cycles, err := u.buildDependencyGraph(ctx, repo, parsedFiles, fileNodes)
	if err != nil {
		return fmt.Errorf("graph building failed: %w", err)
	}

	// Save all structural edges
	allEdges := append(dirEdges, structEdges...)
	allEdges = append(allEdges, importEdges...)
	if err := u.graphRepo.SaveEdges(ctx, allEdges); err != nil {
		log.Warn("some edges failed to save", zap.Error(err))
	}

	// Save function and class nodes
	if err := u.graphRepo.SaveNodes(ctx, functionNodes); err != nil {
		log.Warn("some function nodes failed to save", zap.Error(err))
	}
	if err := u.graphRepo.SaveNodes(ctx, classNodes); err != nil {
		log.Warn("some class nodes failed to save", zap.Error(err))
	}

	// Update fan metrics
	if err := u.graphRepo.UpdateFanMetrics(ctx, repo.ID); err != nil {
		log.Warn("fan metric update failed", zap.Error(err))
	}

	// Mark cycle nodes
	if len(cycles) > 0 {
		cycleNodePaths := make([][]*graph.GraphNode, 0, len(cycles))
		for _, cycle := range cycles {
			var cycleNodes []*graph.GraphNode
			for _, path := range cycle.Nodes {
				n, err := u.graphRepo.GetNodeByPath(ctx, repo.ID, path)
				if err == nil && n != nil {
					cycleNodes = append(cycleNodes, n)
				}
			}
			if len(cycleNodes) > 0 {
				cycleNodePaths = append(cycleNodePaths, cycleNodes)
			}
		}
		if err := u.graphRepo.MarkCycleNodes(ctx, repo.ID, cycleNodePaths); err != nil {
			log.Warn("cycle node marking failed", zap.Error(err))
		}
	}

	repo.CycleCount = len(cycles)
	emit(analysis.StepBuildGraph, 85, fmt.Sprintf(
		"Graph built: %d import edges, %d cycles detected",
		len(importEdges), len(cycles),
	))

	// ---- Step 7: Index AI ----
	emit(analysis.StepIndexAI, 85, "Indexing code for AI search...")
	log.Info("step 7: indexing for AI")

	aiFiles := make([]*domainai.FileContent, 0, len(parsedFiles))
	for _, pf := range parsedFiles {
		if pf.RawSource != "" {
			aiFiles = append(aiFiles, &domainai.FileContent{
				Path:     pf.Path,
				Content:  pf.RawSource,
				Language: string(pf.Language),
			})
		}
	}

	chunksIndexed, err := u.aiClient.IndexRepository(ctx, repo.ID, aiFiles)
	if err != nil {
		log.Warn("AI indexing failed (non-fatal)", zap.Error(err))
	} else {
		log.Info("AI indexing complete", zap.Int32("chunks", chunksIndexed))
	}

	emit(analysis.StepIndexAI, 95, fmt.Sprintf("Indexed %d code chunks for AI search", chunksIndexed))

	// ---- Step 8: Done ----
	now := time.Now()
	repo.Status = domainrepo.StatusReady
	repo.ReadyAt = &now
	repo.UpdatedAt = now

	if err := u.repoRepo.Update(ctx, repo); err != nil {
		log.Warn("failed to update repo status to ready", zap.Error(err))
	}

	// Cache the commit hash
	if repo.CommitHash != "" {
		if err := u.cacheRepo.SetCommitHash(ctx, repo.ID, repo.CommitHash); err != nil {
			log.Warn("failed to cache commit hash", zap.Error(err))
		}
	}

	emit(analysis.StepDone, 100, fmt.Sprintf(
		"Analysis complete: %d files, %d functions, %d classes, %d cycles",
		repo.FileCount, repo.FunctionCount, repo.ClassCount, repo.CycleCount,
	))

	log.Info("analysis pipeline complete",
		zap.String("repo", repo.Name),
		zap.Int("files", repo.FileCount),
		zap.Int("functions", repo.FunctionCount),
		zap.Int("classes", repo.ClassCount),
		zap.Int("cycles", repo.CycleCount),
	)

	return nil
}

// walkAndBuildGraph traverses the filesystem and creates Dir/File nodes in Neo4j.
func (u *UseCase) walkAndBuildGraph(
	ctx context.Context,
	clonePath string,
	repo *domainrepo.Repo,
	repoNodeID string,
	emit func(analysis.PipelineStep, int, string),
) (
	dirNodes []*graph.GraphNode,
	fileNodes []*graph.GraphNode,
	edges []*graph.GraphEdge,
	err error,
) {
	entryCh, errCh := u.fsWalker.Walk(ctx, clonePath, u.skipDirs)

	// Map: relative path → node ID (for building parent-child edges)
	pathToNodeID := map[string]string{}
	pathToNodeID["/"] = repoNodeID

	// Root dir node
	rootDirNode := &graph.GraphNode{
		ID:       nodeID(repo.ID, "dir", "/"),
		RepoID:   repo.ID,
		Type:     graph.NodeTypeDir,
		Name:     repo.Name,
		Path:     "/",
		HasChildren: true,
	}
	dirNodes = append(dirNodes, rootDirNode)
	pathToNodeID["/"] = rootDirNode.ID

	edges = append(edges, &graph.GraphEdge{
		ID:       edgeID(repoNodeID, rootDirNode.ID, "HAS_ROOT"),
		SourceID: repoNodeID,
		TargetID: rootDirNode.ID,
		Type:     graph.EdgeTypeHasRoot,
	})

	// Save root node
	if err = u.graphRepo.SaveNode(ctx, rootDirNode); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to save root dir node: %w", err)
	}

	for entry := range entryCh {
		parentPath := filepath.Dir(entry.Path)
		if parentPath == "" || parentPath == "." {
			parentPath = "/"
		}

		parentID, ok := pathToNodeID[parentPath]
		if !ok {
			// Parent not yet seen, use root
			parentID = rootDirNode.ID
		}

		if entry.IsDir {
			node := &graph.GraphNode{
				ID:      nodeID(repo.ID, "dir", entry.Path),
				RepoID:  repo.ID,
				Type:    graph.NodeTypeDir,
				Name:    entry.Name,
				Path:    entry.Path,
			}
			pathToNodeID[entry.Path] = node.ID
			dirNodes = append(dirNodes, node)

			edges = append(edges, &graph.GraphEdge{
				ID:       edgeID(parentID, node.ID, "HAS_CHILD"),
				SourceID: parentID,
				TargetID: node.ID,
				Type:     graph.EdgeTypeHasChild,
			})

			if err := u.graphRepo.SaveNode(ctx, node); err != nil {
				u.log.Warn("failed to save dir node", zap.String("path", entry.Path), zap.Error(err))
			}
		} else {
			// Only track code files
			if entry.Language == domainrepo.LanguageUnknown {
				continue
			}

			node := &graph.GraphNode{
				ID:       nodeID(repo.ID, "file", entry.Path),
				RepoID:   repo.ID,
				Type:     graph.NodeTypeFile,
				Name:     entry.Name,
				Path:     entry.Path,
				Language: string(entry.Language),
				Size:     entry.SizeBytes,
			}
			pathToNodeID[entry.Path] = node.ID
			fileNodes = append(fileNodes, node)

			edgeType := graph.EdgeTypeHasFile
			edges = append(edges, &graph.GraphEdge{
				ID:       edgeID(parentID, node.ID, "HAS_FILE"),
				SourceID: parentID,
				TargetID: node.ID,
				Type:     edgeType,
			})

			if err := u.graphRepo.SaveNode(ctx, node); err != nil {
				u.log.Warn("failed to save file node", zap.String("path", entry.Path), zap.Error(err))
			}
		}
	}

	// Check for walk errors
	if walkErr, ok := <-errCh; ok && walkErr != nil {
		u.log.Warn("filesystem walk error (non-fatal)", zap.Error(walkErr))
	}

	return dirNodes, fileNodes, edges, nil
}

// parseSourceFiles reads and parses all source files with parallel workers.
func (u *UseCase) parseSourceFiles(
	ctx context.Context,
	clonePath string,
	repo *domainrepo.Repo,
	fileNodes []*graph.GraphNode,
	existingEdges []*graph.GraphEdge,
	emit func(analysis.PipelineStep, int, string),
) (
	parsedFiles []*analysis.ParsedFile,
	functionNodes []*graph.GraphNode,
	classNodes []*graph.GraphNode,
	structEdges []*graph.GraphEdge,
	err error,
) {
	type parseResult struct {
		parsed    *analysis.ParsedFile
		fileNodeID string
	}

	resultCh := make(chan *parseResult, len(fileNodes))
	sem := make(chan struct{}, u.parseWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var parseErrors []string

	total := len(fileNodes)
	processed := 0

	for _, fn := range fileNodes {
		wg.Add(1)
		go func(fileNode *graph.GraphNode) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Determine language
			lang := domainrepo.Language(fileNode.Language)
			if lang == "" {
				return
			}

			// Read file
			absPath := filepath.Join(clonePath, fileNode.Path)
			content, readErr := os.ReadFile(absPath)
			if readErr != nil {
				mu.Lock()
				parseErrors = append(parseErrors, fmt.Sprintf("read %s: %v", fileNode.Path, readErr))
				mu.Unlock()
				return
			}

			parsed, parseErr := u.parser.ParseFile(ctx, fileNode.Path, content, lang)
			if parseErr != nil {
				mu.Lock()
				parseErrors = append(parseErrors, fmt.Sprintf("parse %s: %v", fileNode.Path, parseErr))
				mu.Unlock()
			}

			if parsed != nil {
				parsed.RepoID = repo.ID
				parsed.RawSource = string(content)
				resultCh <- &parseResult{parsed: parsed, fileNodeID: fileNode.ID}
			}

			mu.Lock()
			processed++
			if processed%50 == 0 || processed == total {
				progress := 50 + (processed*20/total)
				emit(analysis.StepParseAST, progress,
					fmt.Sprintf("Parsing... %d/%d files", processed, total))
			}
			mu.Unlock()
		}(fn)
	}

	wg.Wait()
	close(resultCh)

	if len(parseErrors) > 0 {
		u.log.Warn("some files had parse errors",
			zap.Int("error_count", len(parseErrors)),
		)
	}

	for result := range resultCh {
		pf := result.parsed
		parsedFiles = append(parsedFiles, pf)

		// Build Function nodes
		for _, fn := range pf.Functions {
			fnNode := &graph.GraphNode{
				ID:        nodeID(repo.ID, "fn", fmt.Sprintf("%s:%s", pf.Path, fn.Name)),
				RepoID:    repo.ID,
				Type:      graph.NodeTypeFunction,
				Name:      fn.Name,
				Path:      pf.Path,
				StartLine: fn.StartLine,
				EndLine:   fn.EndLine,
				Language:  string(pf.Language),
			}
			functionNodes = append(functionNodes, fnNode)

			structEdges = append(structEdges, &graph.GraphEdge{
				ID:       edgeID(result.fileNodeID, fnNode.ID, "HAS_FUNCTION"),
				SourceID: result.fileNodeID,
				TargetID: fnNode.ID,
				Type:     graph.EdgeTypeHasFunction,
				Line:     fn.StartLine,
			})
		}

		// Build Class nodes
		for _, cls := range pf.Classes {
			clsNode := &graph.GraphNode{
				ID:        nodeID(repo.ID, "cls", fmt.Sprintf("%s:%s", pf.Path, cls.Name)),
				RepoID:    repo.ID,
				Type:      graph.NodeTypeClass,
				Name:      cls.Name,
				Path:      pf.Path,
				StartLine: cls.StartLine,
				EndLine:   cls.EndLine,
				Language:  string(pf.Language),
			}
			classNodes = append(classNodes, clsNode)

			structEdges = append(structEdges, &graph.GraphEdge{
				ID:       edgeID(result.fileNodeID, clsNode.ID, "HAS_CLASS"),
				SourceID: result.fileNodeID,
				TargetID: clsNode.ID,
				Type:     graph.EdgeTypeHasClass,
				Line:     cls.StartLine,
			})
		}
	}

	return parsedFiles, functionNodes, classNodes, structEdges, nil
}

// buildDependencyGraph resolves imports to file nodes and creates IMPORTS edges.
func (u *UseCase) buildDependencyGraph(
	ctx context.Context,
	repo *domainrepo.Repo,
	parsedFiles []*analysis.ParsedFile,
	fileNodes []*graph.GraphNode,
) ([]*graph.GraphEdge, []*graph.CycleResult, error) {
	// Build path → nodeID index
	pathIndex := map[string]string{}
	for _, fn := range fileNodes {
		pathIndex[fn.Path] = fn.ID
		// Also index by name without leading /
		pathIndex[strings.TrimPrefix(fn.Path, "/")] = fn.ID
	}

	var importEdges []*graph.GraphEdge

	for _, pf := range parsedFiles {
		sourceID, ok := pathIndex[pf.Path]
		if !ok {
			continue
		}

		for _, imp := range pf.Imports {
			if imp.Source == "" {
				continue
			}

			// Resolve import to a file path
			resolved := u.resolveImport(pf.Path, imp, pathIndex)
			if resolved == "" {
				continue
			}

			targetID, ok := pathIndex[resolved]
			if !ok {
				continue
			}

			if sourceID == targetID {
				continue // self-import
			}

			importEdges = append(importEdges, &graph.GraphEdge{
				ID:       edgeID(sourceID, targetID, "IMPORTS"),
				SourceID: sourceID,
				TargetID: targetID,
				Type:     graph.EdgeTypeImports,
				Line:     imp.Line,
			})
		}
	}

	// Deduplicate edges
	importEdges = deduplicateEdges(importEdges)

	// Save import edges
	if err := u.graphRepo.SaveEdges(ctx, importEdges); err != nil {
		u.log.Warn("some import edges failed to save", zap.Error(err))
	}

	// Detect cycles using Neo4j
	cycles, err := u.graphRepo.FindCycles(ctx, repo.ID)
	if err != nil {
		u.log.Warn("cycle detection failed", zap.Error(err))
	}

	return importEdges, cycles, nil
}

// resolveImport resolves an import source path to an absolute file path.
func (u *UseCase) resolveImport(
	sourceFilePath string,
	imp *analysis.Import,
	pathIndex map[string]string,
) string {
	src := imp.Source

	// Skip external packages (no leading ./ or /)
	if !imp.IsRelative && !strings.HasPrefix(src, "/") {
		return ""
	}

	// Get directory of source file
	sourceDir := filepath.Dir(sourceFilePath)

	// Resolve relative path
	var resolved string
	if strings.HasPrefix(src, ".") {
		resolved = filepath.Join(sourceDir, src)
	} else {
		resolved = src
	}

	// Normalize to forward slashes
	resolved = "/" + strings.TrimPrefix(filepath.ToSlash(resolved), "/")

	// Try exact match first
	if _, ok := pathIndex[resolved]; ok {
		return resolved
	}

	// Try with extensions
	extensions := []string{".ts", ".tsx", ".js", ".jsx", ".go"}
	for _, ext := range extensions {
		candidate := resolved + ext
		if _, ok := pathIndex[candidate]; ok {
			return candidate
		}
	}

	// Try index files
	indexCandidates := []string{
		resolved + "/index.ts",
		resolved + "/index.tsx",
		resolved + "/index.js",
		resolved + "/index.jsx",
	}
	for _, candidate := range indexCandidates {
		if _, ok := pathIndex[candidate]; ok {
			return candidate
		}
	}

	return ""
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func nodeID(repoID, nodeType, path string) string {
	h := sha256.New()
	io.WriteString(h, repoID+":"+nodeType+":"+path)
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func edgeID(sourceID, targetID, edgeType string) string {
	h := sha256.New()
	io.WriteString(h, sourceID+":"+edgeType+":"+targetID)
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func deduplicateEdges(edges []*graph.GraphEdge) []*graph.GraphEdge {
	seen := map[string]bool{}
	result := make([]*graph.GraphEdge, 0, len(edges))
	for _, e := range edges {
		key := fmt.Sprintf("%s->%s:%s", e.SourceID, e.TargetID, e.Type)
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}