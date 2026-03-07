package analysis

import "github.com/bakdaulet/ais/ais-back/internal/domain/analysis"

func ResolveImportForTest(sourceFile, importSrc string, pathIndex map[string]string) string {
    uc := &UseCase{}
    imp := &analysis.Import{
        Source:     importSrc,
        IsRelative: len(importSrc) > 0 && (importSrc[0] == '.' || importSrc[0] == '/'),
    }
    return uc.resolveImport(sourceFile, imp, pathIndex)
}