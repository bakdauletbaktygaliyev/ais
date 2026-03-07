package parser

import (
	"context"
	"fmt"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/bakdaulet/ais/ais-back/internal/domain/analysis"
	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
	"go.uber.org/zap"
)

// GoParser handles Go source files.
type GoParser struct {
	pool *sync.Pool // pool of parsers — each goroutine gets its own instance
	log  *logger.Logger
}

// NewGoParser creates a new Go parser.
func NewGoParser(log *logger.Logger) (*GoParser, error) {
	lang := golang.GetLanguage()
	return &GoParser{
		pool: &sync.Pool{
			New: func() interface{} {
				p := sitter.NewParser()
				p.SetLanguage(lang)
				return p
			},
		},
		log: log.WithComponent("go_parser"),
	}, nil
}

// Parse parses a Go file and extracts AST information.
func (p *GoParser) Parse(
	ctx context.Context,
	path string,
	content []byte,
) (*analysis.ParsedFile, error) {
	// Acquire a parser from the pool — safe for concurrent use
	parser := p.pool.Get().(*sitter.Parser)
	defer p.pool.Put(parser)

	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter Go parse failed for %s: %w", path, err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		p.log.Debug("Go parse had errors (non-fatal)", zap.String("path", path))
	}

	result := &analysis.ParsedFile{
		Path:     path,
		Language: domainrepo.LanguageGo,
	}

	// Extract package name
	packageName := ""
	iter := sitter.NewIterator(root, sitter.BFSMode)
	err = iter.ForEach(func(node *sitter.Node) error {
		switch node.Type() {
		case "package_clause":
			packageName = getChildContent(node, "package_identifier", content)
			if packageName == "" {
				// fallback
				for i := 0; i < int(node.ChildCount()); i++ {
					c := node.Child(i)
					if c.Type() == "identifier" {
						packageName = c.Content(content)
						break
					}
				}
			}

		case "import_declaration":
			imports := p.extractImports(node, content)
			result.Imports = append(result.Imports, imports...)

		case "function_declaration":
			fn := p.extractFunction(node, content)
			if fn != nil {
				result.Functions = append(result.Functions, fn)
				// Top-level exported functions become exports
				if fn.IsExported {
					result.Exports = append(result.Exports, &analysis.Export{
						Name: fn.Name, Kind: analysis.ExportKindFunction,
						Line: fn.StartLine,
					})
				}
			}

		case "method_declaration":
			fn := p.extractMethod(node, content)
			if fn != nil {
				result.Functions = append(result.Functions, fn)
			}

		case "type_declaration":
			classes := p.extractTypeDecl(node, content)
			result.Classes = append(result.Classes, classes...)
		}
		return nil
	})

	if err != nil {
		p.log.Warn("Go AST traversal error", zap.String("path", path), zap.Error(err))
	}

	// Tag all entities with package name
	_ = packageName

	return result, nil
}

func (p *GoParser) extractImports(node *sitter.Node, content []byte) []*analysis.Import {
	var imports []*analysis.Import
	line := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "import_spec":
			imp := p.extractImportSpec(child, content, line)
			if imp != nil {
				imports = append(imports, imp)
			}
		case "import_spec_list":
			for j := 0; j < int(child.ChildCount()); j++ {
				spec := child.Child(j)
				if spec.Type() == "import_spec" {
					imp := p.extractImportSpec(spec, content, int(spec.StartPoint().Row)+1)
					if imp != nil {
						imports = append(imports, imp)
					}
				}
			}
		}
	}

	return imports
}

func (p *GoParser) extractImportSpec(node *sitter.Node, content []byte, line int) *analysis.Import {
	imp := &analysis.Import{Line: line}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "interpreted_string_literal", "raw_string_literal":
			raw := strings.Trim(child.Content(content), `"`+"`")
			imp.Source = raw
			// Go internal packages don't start with a domain
			parts := strings.Split(raw, "/")
			imp.IsRelative = !strings.Contains(parts[0], ".")
		case "identifier", "dot", "blank_identifier":
			// import alias
			alias := child.Content(content)
			if alias != "." && alias != "_" {
				imp.Names = []string{alias}
			}
		}
	}

	if imp.Source == "" {
		return nil
	}
	return imp
}

func (p *GoParser) extractFunction(node *sitter.Node, content []byte) *analysis.Function {
	fn := &analysis.Function{
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			fn.Name = child.Content(content)
			fn.IsExported = len(fn.Name) > 0 && fn.Name[0] >= 'A' && fn.Name[0] <= 'Z'
		case "block":
			fn.Calls = p.extractGoCalls(child, content)
		}
	}

	if fn.Name == "" {
		return nil
	}
	return fn
}

func (p *GoParser) extractMethod(node *sitter.Node, content []byte) *analysis.Function {
	fn := &analysis.Function{
		IsMethod:  true,
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "field_identifier":
			fn.Name = child.Content(content)
			fn.IsExported = len(fn.Name) > 0 && fn.Name[0] >= 'A' && fn.Name[0] <= 'Z'
		case "parameter_list":
			// First parameter_list in a method_declaration is the receiver
			if fn.Receiver == "" {
				fn.Receiver = p.extractReceiverType(child, content)
			}
		case "block":
			fn.Calls = p.extractGoCalls(child, content)
		}
	}

	if fn.Name == "" {
		return nil
	}
	return fn
}

func (p *GoParser) extractReceiverType(params *sitter.Node, content []byte) string {
	for i := 0; i < int(params.ChildCount()); i++ {
		child := params.Child(i)
		if child.Type() == "parameter_declaration" {
			for j := 0; j < int(child.ChildCount()); j++ {
				c := child.Child(j)
				if c.Type() == "type_identifier" || c.Type() == "pointer_type" {
					return c.Content(content)
				}
			}
		}
	}
	return ""
}

func (p *GoParser) extractTypeDecl(node *sitter.Node, content []byte) []*analysis.Class {
	var classes []*analysis.Class

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_spec" {
			cls := p.extractTypeSpec(child, content)
			if cls != nil {
				classes = append(classes, cls)
			}
		}
	}

	return classes
}

func (p *GoParser) extractTypeSpec(node *sitter.Node, content []byte) *analysis.Class {
	cls := &analysis.Class{
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "type_identifier":
			cls.Name = child.Content(content)
			cls.IsExported = len(cls.Name) > 0 && cls.Name[0] >= 'A' && cls.Name[0] <= 'Z'

		case "struct_type":
			cls.Kind = analysis.ClassKindStruct
			cls.Fields = p.extractStructFields(child, content)

		case "interface_type":
			cls.Kind = analysis.ClassKindInterface
			// Extract interface methods
			for j := 0; j < int(child.ChildCount()); j++ {
				m := child.Child(j)
				if m.Type() == "method_spec" {
					methodName := getChildContent(m, "field_identifier", content)
					if methodName != "" {
						cls.Methods = append(cls.Methods, &analysis.Function{
							Name:      methodName,
							IsMethod:  true,
							StartLine: int(m.StartPoint().Row) + 1,
							EndLine:   int(m.EndPoint().Row) + 1,
						})
					}
				}
			}

		default:
			if cls.Kind == "" {
				cls.Kind = analysis.ClassKindType
			}
		}
	}

	if cls.Name == "" {
		return nil
	}
	if cls.Kind == "" {
		cls.Kind = analysis.ClassKindType
	}

	return cls
}

func (p *GoParser) extractStructFields(node *sitter.Node, content []byte) []*analysis.Field {
	var fields []*analysis.Field

	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		return fields
	}

	for i := 0; i < int(fieldList.ChildCount()); i++ {
		child := fieldList.Child(i)
		if child.Type() != "field_declaration" {
			continue
		}

		field := &analysis.Field{
			Line: int(child.StartPoint().Row) + 1,
		}

		for j := 0; j < int(child.ChildCount()); j++ {
			c := child.Child(j)
			switch c.Type() {
			case "field_identifier":
				field.Name = c.Content(content)
				field.IsExported = len(field.Name) > 0 && field.Name[0] >= 'A' && field.Name[0] <= 'Z'
			case "type_identifier", "pointer_type", "slice_type", "map_type",
				"qualified_type", "array_type", "interface_type":
				field.TypeName = c.Content(content)
			}
		}

		if field.Name != "" {
			fields = append(fields, field)
		}
	}

	return fields
}

func (p *GoParser) extractGoCalls(body *sitter.Node, content []byte) []string {
	var calls []string
	seen := map[string]bool{}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "call_expression" {
			fn := n.Child(0)
			if fn != nil {
				name := fn.Content(content)
				// Extract only the function name portion (not receiver)
				if idx := strings.LastIndex(name, "."); idx >= 0 {
					name = name[idx+1:]
				}
				if name != "" && !seen[name] {
					calls = append(calls, name)
					seen[name] = true
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(body)

	return calls
}