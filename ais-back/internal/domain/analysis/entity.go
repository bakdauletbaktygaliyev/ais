package analysis

import (
	"context"
	"time"

	"github.com/bakdaulet/ais/ais-back/internal/domain/repository"
)

// PipelineStep enumerates each step in the analysis pipeline.
type PipelineStep string

const (
	StepValidate   PipelineStep = "validate"
	StepClone      PipelineStep = "clone"
	StepDetect     PipelineStep = "detect"
	StepWalkFS     PipelineStep = "walk_fs"
	StepParseAST   PipelineStep = "parse_ast"
	StepBuildGraph PipelineStep = "build_graph"
	StepIndexAI    PipelineStep = "index_ai"
	StepDone       PipelineStep = "done"
	StepError      PipelineStep = "error"
)

// StepProgress maps each pipeline step to its progress range.
var StepProgress = map[PipelineStep][2]int{
	StepValidate:   {0, 5},
	StepClone:      {5, 20},
	StepDetect:     {20, 30},
	StepWalkFS:     {30, 50},
	StepParseAST:   {50, 70},
	StepBuildGraph: {70, 85},
	StepIndexAI:    {85, 95},
	StepDone:       {95, 100},
}

// ProgressEvent is emitted by the pipeline at each step transition.
type ProgressEvent struct {
	RepoID   string       `json:"repoId"`
	Step     PipelineStep `json:"step"`
	Progress int          `json:"progress"`
	Message  string       `json:"message"`
	Error    string       `json:"error,omitempty"`
	At       time.Time    `json:"at"`
}

// ParsedFile holds all AST-extracted information from a single source file.
type ParsedFile struct {
	Path      string
	RepoID    string
	Language  repository.Language
	Imports   []*Import
	Exports   []*Export
	Functions []*Function
	Classes   []*Class
	RawSource string
	SizeBytes int64
}

// Import represents a single import declaration.
type Import struct {
	Source      string   // raw import path (e.g. "./utils" or "fmt")
	ResolvedPath string  // absolute path after resolution
	Names       []string // named imports (e.g. ["useState", "useEffect"])
	IsDefault   bool
	IsNamespace bool
	IsRelative  bool
	Line        int
}

// Export represents a single export declaration.
type Export struct {
	Name      string
	Kind      ExportKind
	Line      int
}

// ExportKind describes what is being exported.
type ExportKind string

const (
	ExportKindFunction  ExportKind = "function"
	ExportKindClass     ExportKind = "class"
	ExportKindInterface ExportKind = "interface"
	ExportKindType      ExportKind = "type"
	ExportKindVariable  ExportKind = "variable"
	ExportKindDefault   ExportKind = "default"
)

// Function represents a function or method declaration.
type Function struct {
	Name       string
	IsMethod   bool
	IsExported bool
	IsAsync    bool
	Receiver   string   // for Go methods: receiver type name
	Calls      []string // names of functions called within this function
	StartLine  int
	EndLine    int
	Signature  string
}

// Class represents a class, struct, or interface declaration.
type Class struct {
	Name        string
	Kind        ClassKind
	IsExported  bool
	Extends     string
	Implements  []string
	Methods     []*Function
	Fields      []*Field
	StartLine   int
	EndLine     int
}

// ClassKind distinguishes between class-like constructs.
type ClassKind string

const (
	ClassKindClass     ClassKind = "class"
	ClassKindInterface ClassKind = "interface"
	ClassKindStruct    ClassKind = "struct"
	ClassKindType      ClassKind = "type"
	ClassKindEnum      ClassKind = "enum"
)

// Field represents a struct field or class property.
type Field struct {
	Name      string
	TypeName  string
	IsExported bool
	Line      int
}

// FSEntry represents a file or directory discovered during filesystem traversal.
type FSEntry struct {
	Path      string
	Name      string
	IsDir     bool
	SizeBytes int64
	Language  repository.Language
}

// ResolvedImport represents an import that has been resolved to a graph node ID.
type ResolvedImport struct {
	SourceFileID string
	TargetFileID string
	Line         int
}

// ---------------------------------------------------------------------------
// Ports
// ---------------------------------------------------------------------------

// Cloner defines the contract for cloning git repositories.
type Cloner interface {
	// Clone performs a shallow clone (depth=1) into destPath. Returns commit hash.
	Clone(ctx context.Context, repoURL, destPath string) (commitHash string, err error)

	// Cleanup removes the cloned directory.
	Cleanup(ctx context.Context, destPath string) error
}

// FSWalker defines the contract for traversing a repository's file system.
type FSWalker interface {
	// Walk traverses the directory tree starting at repoPath,
	// skipping configured directories. Sends entries via the channel.
	Walk(ctx context.Context, repoPath string, skipDirs []string) (<-chan *FSEntry, <-chan error)
}

// LangDetector defines the contract for detecting the primary language.
type LangDetector interface {
	// Detect analyzes the repository directory and returns the dominant language.
	Detect(ctx context.Context, repoPath string) (repository.Language, repository.MonorepoType, error)
}

// Parser defines the contract for AST-based source code parsing.
type Parser interface {
	// ParseFile parses a single source file and returns extracted AST information.
	ParseFile(ctx context.Context, path string, content []byte, lang repository.Language) (*ParsedFile, error)

	// SupportedExtensions returns the list of file extensions this parser handles.
	SupportedExtensions() []string

	// DetectLanguage returns the language for a given file extension.
	DetectLanguage(ext string) (repository.Language, bool)
}

// ProgressEmitter defines how pipeline steps emit progress events.
type ProgressEmitter interface {
	Emit(event *ProgressEvent)
}
