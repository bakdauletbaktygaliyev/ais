package parser

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/bakdaulet/ais/ais-back/internal/domain/analysis"
	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
	"go.uber.org/zap"
)

// TypeScriptParser handles TypeScript/TSX source files.
type TypeScriptParser struct {
	pool *sync.Pool
	log  *logger.Logger
}

// NewTypeScriptParser creates a new TypeScript parser.
func NewTypeScriptParser(log *logger.Logger) (*TypeScriptParser, error) {
	lang := typescript.GetLanguage()
	return &TypeScriptParser{
		pool: &sync.Pool{
			New: func() interface{} {
				p := sitter.NewParser()
				p.SetLanguage(lang)
				return p
			},
		},
		log: log.WithComponent("ts_parser"),
	}, nil
}

// Parse parses a TypeScript file and extracts AST information.
func (p *TypeScriptParser) Parse(
	ctx context.Context,
	path string,
	content []byte,
) (*analysis.ParsedFile, error) {
	parser := p.pool.Get().(*sitter.Parser)
	defer p.pool.Put(parser)

	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse failed for %s: %w", path, err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		p.log.Debug("TypeScript parse had errors (non-fatal)",
			zap.String("path", path))
	}

	result := &analysis.ParsedFile{
		Path:     path,
		Language: domainrepo.LanguageTypeScript,
	}

	// Walk the AST
	iter := sitter.NewIterator(root, sitter.BFSMode)
	err = iter.ForEach(func(node *sitter.Node) error {
		switch node.Type() {
		case "import_statement":
			imp := p.extractImport(node, content)
			if imp != nil {
				result.Imports = append(result.Imports, imp)
			}

		case "export_statement":
			exports := p.extractExports(node, content)
			result.Exports = append(result.Exports, exports...)

		case "function_declaration", "function":
			if node.Parent() != nil && node.Parent().Type() == "export_statement" {
				break // handled by export_statement
			}
			fn := p.extractFunction(node, content, false)
			if fn != nil {
				result.Functions = append(result.Functions, fn)
			}

		case "lexical_declaration", "variable_declaration":
			// Arrow functions assigned to variables
			fns := p.extractArrowFunctions(node, content)
			result.Functions = append(result.Functions, fns...)

		case "class_declaration":
			cls := p.extractClass(node, content)
			if cls != nil {
				result.Classes = append(result.Classes, cls)
			}

		case "interface_declaration":
			iface := p.extractInterface(node, content)
			if iface != nil {
				result.Classes = append(result.Classes, iface)
			}

		case "type_alias_declaration":
			ta := p.extractTypeAlias(node, content)
			if ta != nil {
				result.Classes = append(result.Classes, ta)
			}
		}
		return nil
	})

	if err != nil {
		p.log.Warn("AST traversal error", zap.String("path", path), zap.Error(err))
	}

	return result, nil
}

func (p *TypeScriptParser) extractImport(node *sitter.Node, content []byte) *analysis.Import {
	imp := &analysis.Import{
		Line: int(node.StartPoint().Row) + 1,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "string", "string_fragment":
			raw := child.Content(content)
			raw = strings.Trim(raw, `"'`+"`")
			imp.Source = raw
			imp.IsRelative = strings.HasPrefix(raw, ".") || strings.HasPrefix(raw, "/")

		case "import_clause":
			imp.Names = p.extractImportNames(child, content)

		case "namespace_import":
			imp.IsNamespace = true
		}
	}

	if imp.Source == "" {
		return nil
	}
	return imp
}

func (p *TypeScriptParser) extractImportNames(node *sitter.Node, content []byte) []string {
	var names []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			names = append(names, child.Content(content))
		case "named_imports":
			for j := 0; j < int(child.ChildCount()); j++ {
				spec := child.Child(j)
				if spec.Type() == "import_specifier" {
					// Get the local name (last identifier in specifier)
					for k := int(spec.ChildCount()) - 1; k >= 0; k-- {
						n := spec.Child(k)
						if n.Type() == "identifier" {
							names = append(names, n.Content(content))
							break
						}
					}
				}
			}
		}
	}
	return names
}

func (p *TypeScriptParser) extractExports(node *sitter.Node, content []byte) []*analysis.Export {
	var exports []*analysis.Export
	line := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "function_declaration":
			name := getChildContent(child, "identifier", content)
			if name != "" {
				exports = append(exports, &analysis.Export{
					Name: name, Kind: analysis.ExportKindFunction, Line: line,
				})
			}
		case "class_declaration":
			name := getChildContent(child, "identifier", content)
			if name != "" {
				exports = append(exports, &analysis.Export{
					Name: name, Kind: analysis.ExportKindClass, Line: line,
				})
			}
		case "interface_declaration":
			name := getChildContent(child, "type_identifier", content)
			if name != "" {
				exports = append(exports, &analysis.Export{
					Name: name, Kind: analysis.ExportKindInterface, Line: line,
				})
			}
		case "lexical_declaration", "variable_declaration":
			// export const foo = ...
			for j := 0; j < int(child.ChildCount()); j++ {
				decl := child.Child(j)
				if decl.Type() == "variable_declarator" {
					name := getChildContent(decl, "identifier", content)
					if name != "" {
						exports = append(exports, &analysis.Export{
							Name: name, Kind: analysis.ExportKindVariable, Line: line,
						})
					}
				}
			}
		case "default":
			exports = append(exports, &analysis.Export{
				Name: "default", Kind: analysis.ExportKindDefault, Line: line,
			})
		}
	}

	return exports
}

func (p *TypeScriptParser) extractFunction(
	node *sitter.Node,
	content []byte,
	isExported bool,
) *analysis.Function {
	name := getChildContent(node, "identifier", content)
	if name == "" {
		return nil
	}

	fn := &analysis.Function{
		Name:       name,
		IsExported: isExported,
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
	}

	// Extract function calls within the body
	if body := findChildByType(node, "statement_block"); body != nil {
		fn.Calls = p.extractCallExpressions(body, content)
	}

	return fn
}

func (p *TypeScriptParser) extractArrowFunctions(
	node *sitter.Node,
	content []byte,
) []*analysis.Function {
	var fns []*analysis.Function
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "variable_declarator" {
			nameNode := findChildByType(child, "identifier")
			valueNode := findChildByType(child, "arrow_function")
			if nameNode != nil && valueNode != nil {
				fn := &analysis.Function{
					Name:      nameNode.Content(content),
					StartLine: int(valueNode.StartPoint().Row) + 1,
					EndLine:   int(valueNode.EndPoint().Row) + 1,
				}
				// Check for async
				if prev := nameNode.PrevSibling(); prev != nil && prev.Type() == "async" {
					fn.IsAsync = true
				}
				// Extract calls
				if body := findChildByType(valueNode, "statement_block"); body != nil {
					fn.Calls = p.extractCallExpressions(body, content)
				}
				fns = append(fns, fn)
			}
		}
	}
	return fns
}

func (p *TypeScriptParser) extractCallExpressions(node *sitter.Node, content []byte) []string {
	var calls []string
	seen := map[string]bool{}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "call_expression" {
			fn := n.Child(0)
			if fn != nil {
				name := fn.Content(content)
				// Only keep simple function names, not method calls
				if !strings.Contains(name, ".") && !seen[name] {
					calls = append(calls, name)
					seen[name] = true
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return calls
}

func (p *TypeScriptParser) extractClass(node *sitter.Node, content []byte) *analysis.Class {
	name := getChildContent(node, "type_identifier", content)
	if name == "" {
		name = getChildContent(node, "identifier", content)
	}
	if name == "" {
		return nil
	}

	cls := &analysis.Class{
		Name:      name,
		Kind:      analysis.ClassKindClass,
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
	}

	// Extract extends/implements
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "class_heritage":
			for j := 0; j < int(child.ChildCount()); j++ {
				h := child.Child(j)
				switch h.Type() {
				case "extends_clause":
					cls.Extends = getFirstIdentifier(h, content)
				case "implements_clause":
					cls.Implements = getAllIdentifiers(h, content)
				}
			}
		case "class_body":
			cls.Methods = p.extractClassMethods(child, content)
		}
	}

	return cls
}

func (p *TypeScriptParser) extractInterface(node *sitter.Node, content []byte) *analysis.Class {
	name := getChildContent(node, "type_identifier", content)
	if name == "" {
		return nil
	}

	iface := &analysis.Class{
		Name:      name,
		Kind:      analysis.ClassKindInterface,
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
	}

	// Extract extends
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "extends_type_clause" {
			iface.Implements = getAllIdentifiers(child, content)
		}
	}

	return iface
}

func (p *TypeScriptParser) extractTypeAlias(node *sitter.Node, content []byte) *analysis.Class {
	name := getChildContent(node, "type_identifier", content)
	if name == "" {
		return nil
	}

	return &analysis.Class{
		Name:      name,
		Kind:      analysis.ClassKindType,
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
	}
}

func (p *TypeScriptParser) extractClassMethods(body *sitter.Node, content []byte) []*analysis.Function {
	var methods []*analysis.Function
	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child.Type() == "method_definition" {
			name := getChildContent(child, "property_identifier", content)
			if name == "" {
				name = getChildContent(child, "identifier", content)
			}
			if name != "" {
				m := &analysis.Function{
					Name:      name,
					IsMethod:  true,
					StartLine: int(child.StartPoint().Row) + 1,
					EndLine:   int(child.EndPoint().Row) + 1,
				}
				if body := findChildByType(child, "statement_block"); body != nil {
					m.Calls = p.extractCallExpressions(body, content)
				}
				methods = append(methods, m)
			}
		}
	}
	return methods
}

// ---------------------------------------------------------------------------
// Helpers (shared with javascript.go via same package)
// ---------------------------------------------------------------------------

func getChildContent(node *sitter.Node, childType string, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == childType {
			return child.Content(content)
		}
	}
	return ""
}

func findChildByType(node *sitter.Node, childType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == childType {
			return child
		}
	}
	return nil
}

func getFirstIdentifier(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" || child.Type() == "type_identifier" {
			return child.Content(content)
		}
	}
	return ""
}

func getAllIdentifiers(node *sitter.Node, content []byte) []string {
	var names []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" || child.Type() == "type_identifier" {
			names = append(names, child.Content(content))
		}
	}
	return names
}

// isSupportedTSFile checks if a file should be parsed as TypeScript.
func isSupportedTSFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".ts" || ext == ".tsx"
}