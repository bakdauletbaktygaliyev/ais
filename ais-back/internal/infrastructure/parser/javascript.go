package parser

import (
	"context"
	"fmt"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"

	"github.com/bakdaulet/ais/ais-back/internal/domain/analysis"
	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
	"go.uber.org/zap"
)

// JavaScriptParser handles JavaScript/JSX source files.
type JavaScriptParser struct {
	pool *sync.Pool
	log  *logger.Logger
}

// NewJavaScriptParser creates a new JavaScript parser.
func NewJavaScriptParser(log *logger.Logger) (*JavaScriptParser, error) {
	lang := javascript.GetLanguage()
	return &JavaScriptParser{
		pool: &sync.Pool{
			New: func() interface{} {
				p := sitter.NewParser()
				p.SetLanguage(lang)
				return p
			},
		},
		log: log.WithComponent("js_parser"),
	}, nil
}

// Parse parses a JavaScript file and extracts AST information.
func (p *JavaScriptParser) Parse(
	ctx context.Context,
	path string,
	content []byte,
) (*analysis.ParsedFile, error) {
	parser := p.pool.Get().(*sitter.Parser)
	defer p.pool.Put(parser)

	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter JS parse failed for %s: %w", path, err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		p.log.Debug("JavaScript parse had errors (non-fatal)",
			zap.String("path", path))
	}

	result := &analysis.ParsedFile{
		Path:     path,
		Language: domainrepo.LanguageJavaScript,
	}

	iter := sitter.NewIterator(root, sitter.BFSMode)
	err = iter.ForEach(func(node *sitter.Node) error {
		switch node.Type() {
		case "import_statement":
			imp := p.extractImport(node, content)
			if imp != nil {
				result.Imports = append(result.Imports, imp)
			}

		case "call_expression":
			// CommonJS require() calls
			if imp := p.extractRequire(node, content); imp != nil {
				result.Imports = append(result.Imports, imp)
			}

		case "export_statement":
			exports := p.extractExports(node, content)
			result.Exports = append(result.Exports, exports...)

		case "function_declaration":
			fn := p.extractFunction(node, content)
			if fn != nil {
				result.Functions = append(result.Functions, fn)
			}

		case "lexical_declaration", "variable_declaration":
			// Arrow functions and function expressions
			fns := p.extractVariableFunctions(node, content)
			result.Functions = append(result.Functions, fns...)

		case "class_declaration":
			cls := p.extractClass(node, content)
			if cls != nil {
				result.Classes = append(result.Classes, cls)
			}
		}
		return nil
	})

	if err != nil {
		p.log.Warn("JS AST traversal error",
			zap.String("path", path), zap.Error(err))
	}

	return result, nil
}

func (p *JavaScriptParser) extractImport(node *sitter.Node, content []byte) *analysis.Import {
	imp := &analysis.Import{
		Line: int(node.StartPoint().Row) + 1,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "string":
			raw := strings.Trim(child.Content(content), `"'`+"`")
			imp.Source = raw
			imp.IsRelative = strings.HasPrefix(raw, ".") || strings.HasPrefix(raw, "/")
		case "import_clause":
			imp.Names = p.extractImportClauseNames(child, content)
		case "namespace_import":
			imp.IsNamespace = true
		}
	}

	if imp.Source == "" {
		return nil
	}
	return imp
}

func (p *JavaScriptParser) extractImportClauseNames(node *sitter.Node, content []byte) []string {
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

func (p *JavaScriptParser) extractRequire(node *sitter.Node, content []byte) *analysis.Import {
	// Pattern: require("./something")
	if node.ChildCount() < 2 {
		return nil
	}

	fnNode := node.Child(0)
	if fnNode == nil || fnNode.Content(content) != "require" {
		return nil
	}

	argsNode := findChildByType(node, "arguments")
	if argsNode == nil || argsNode.ChildCount() < 1 {
		return nil
	}

	// First argument
	for i := 0; i < int(argsNode.ChildCount()); i++ {
		arg := argsNode.Child(i)
		if arg.Type() == "string" {
			raw := strings.Trim(arg.Content(content), `"'`+"`")
			if raw != "" {
				return &analysis.Import{
					Source:     raw,
					IsRelative: strings.HasPrefix(raw, "."),
					Line:       int(node.StartPoint().Row) + 1,
				}
			}
		}
	}

	return nil
}

func (p *JavaScriptParser) extractExports(node *sitter.Node, content []byte) []*analysis.Export {
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
		case "lexical_declaration", "variable_declaration":
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

func (p *JavaScriptParser) extractFunction(node *sitter.Node, content []byte) *analysis.Function {
	name := getChildContent(node, "identifier", content)
	if name == "" {
		return nil
	}

	fn := &analysis.Function{
		Name:      name,
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
	}

	// Check for async
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(i).Type() == "async" {
			fn.IsAsync = true
			break
		}
	}

	if body := findChildByType(node, "statement_block"); body != nil {
		fn.Calls = p.extractCalls(body, content)
	}

	return fn
}

func (p *JavaScriptParser) extractVariableFunctions(
	node *sitter.Node,
	content []byte,
) []*analysis.Function {
	var fns []*analysis.Function
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() != "variable_declarator" {
			continue
		}

		nameNode := findChildByType(child, "identifier")
		if nameNode == nil {
			continue
		}

		// Check for arrow function or function expression
		var funcNode *sitter.Node
		for j := 0; j < int(child.ChildCount()); j++ {
			c := child.Child(j)
			if c.Type() == "arrow_function" || c.Type() == "function" {
				funcNode = c
				break
			}
		}

		if funcNode == nil {
			continue
		}

		fn := &analysis.Function{
			Name:      nameNode.Content(content),
			StartLine: int(funcNode.StartPoint().Row) + 1,
			EndLine:   int(funcNode.EndPoint().Row) + 1,
		}

		// async detection
		for j := 0; j < int(funcNode.ChildCount()); j++ {
			if funcNode.Child(j).Type() == "async" {
				fn.IsAsync = true
				break
			}
		}

		if body := findChildByType(funcNode, "statement_block"); body != nil {
			fn.Calls = p.extractCalls(body, content)
		}

		fns = append(fns, fn)
	}
	return fns
}

func (p *JavaScriptParser) extractClass(node *sitter.Node, content []byte) *analysis.Class {
	name := getChildContent(node, "identifier", content)
	if name == "" {
		return nil
	}

	cls := &analysis.Class{
		Name:      name,
		Kind:      analysis.ClassKindClass,
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "class_heritage":
			for j := 0; j < int(child.ChildCount()); j++ {
				h := child.Child(j)
				if h.Type() == "extends_clause" {
					cls.Extends = getFirstIdentifier(h, content)
				}
			}
		case "class_body":
			cls.Methods = p.extractMethods(child, content)
		}
	}

	return cls
}

func (p *JavaScriptParser) extractMethods(body *sitter.Node, content []byte) []*analysis.Function {
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
					m.Calls = p.extractCalls(body, content)
				}
				methods = append(methods, m)
			}
		}
	}
	return methods
}

func (p *JavaScriptParser) extractCalls(node *sitter.Node, content []byte) []string {
	var calls []string
	seen := map[string]bool{}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "call_expression" {
			fn := n.Child(0)
			if fn != nil {
				name := fn.Content(content)
				if !strings.Contains(name, ".") && !seen[name] && name != "require" {
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