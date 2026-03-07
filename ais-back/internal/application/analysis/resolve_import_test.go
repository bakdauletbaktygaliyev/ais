package analysis_test

import (
	"testing"

	appanalysis "github.com/bakdaulet/ais/ais-back/internal/application/analysis"
)

func TestResolveImport_RelativeTS(t *testing.T) {
	pathIndex := map[string]string{
		"/src/app/core/services/api.service.ts":      "node-1",
		"/src/app/core/models/index.ts":              "node-2",
		"/src/app/features/graph/graph.component.ts": "node-3",
		"/src/app/features/graph/index.ts":           "node-4",
	}

	tests := []struct {
		name       string
		sourceFile string
		importSrc  string
		wantID     string
	}{
		{
			name:       "relative .ts import resolves",
			sourceFile: "/src/app/features/graph/graph.component.ts",
			importSrc:  "../../core/services/api.service",
			wantID:     "node-1",
		},
		{
			name:       "dot resolves to index.ts",
			sourceFile: "/src/app/features/graph/graph.component.ts",
			importSrc:  ".",
			wantID:     "node-4",
		},
		{
			name:       "relative import to models index",
			sourceFile: "/src/app/features/graph/graph.component.ts",
			importSrc:  "../../core/models",
			wantID:     "node-2",
		},
		{
			name:       "external npm package is skipped",
			sourceFile: "/src/app/features/graph/graph.component.ts",
			importSrc:  "rxjs",
			wantID:     "",
		},
		{
			name:       "angular scoped package is skipped",
			sourceFile: "/src/app/features/graph/graph.component.ts",
			importSrc:  "@angular/core",
			wantID:     "",
		},
		{
			name:       "non-existent relative import returns empty",
			sourceFile: "/src/app/features/graph/graph.component.ts",
			importSrc:  "./does-not-exist",
			wantID:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := appanalysis.ResolveImportForTest(tc.sourceFile, tc.importSrc, pathIndex)
			if tc.wantID == "" {
				// We expect no resolved node — check nothing matched
				if got != "" {
					t.Errorf("ResolveImportForTest(%q, %q) = %q; want empty", tc.sourceFile, tc.importSrc, got)
				}
			} else {
				resolvedID := pathIndex[got]
				if resolvedID != tc.wantID {
					t.Errorf("ResolveImportForTest(%q, %q) resolved to path %q (id=%q); want id=%q",
						tc.sourceFile, tc.importSrc, got, resolvedID, tc.wantID)
				}
			}
		})
	}
}

func TestResolveImport_ExternalSkipped(t *testing.T) {
	pathIndex := map[string]string{
		"/cmd/server/main.go": "go-node-1",
	}

	externals := []string{"context", "fmt", "os", "encoding/json", "github.com/gin-gonic/gin"}
	for _, pkg := range externals {
		t.Run(pkg, func(t *testing.T) {
			got := appanalysis.ResolveImportForTest("/cmd/server/main.go", pkg, pathIndex)
			if got != "" {
				t.Errorf("external package %q should return empty, got %q", pkg, got)
			}
		})
	}
}