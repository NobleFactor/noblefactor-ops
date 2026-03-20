// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package goast provides Go AST operations as a Starlark receiver.
//
// +devlore:access=immediate
package goast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	tmpl "text/template"

	"github.com/NobleFactor/devlore-cli/pkg/op"
	"github.com/NobleFactor/noblefactor-ops/internal/provider/goast/doctaxonomy"
)

// Provider provides Go AST operations as a Starlark receiver.
//
// +devlore:access=immediate
type Provider struct {
	op.ProviderBase
	fileCache sync.Map // path → *parsedFile (AST cache)
}

// NewProvider creates a new Provider.
//
// Parameters:
//   - ctx: the provider context.
//
// Returns:
//   - *Provider: the new provider.
func NewProvider(ctx op.Context) *Provider {
	return &Provider{ProviderBase: op.NewProviderBase(ctx)}
}

// region EXPORTED METHODS

// region Behaviors

// Fallible actions

// Callable introspects a named function type declaration and returns its parameter list, return type, and doc comment
// (including directives).
//
// Parameters:
//   - path: the file or directory path.
//   - name: the function type name to find.
//
// Returns:
//   - CallableResult: the function type information.
//   - error: non-nil if the type is not found or cannot be parsed.
func (p *Provider) Callable(path, name string) (CallableResult, error) {
	files, err := collectGoFiles(path)
	if err != nil {
		return CallableResult{}, fmt.Errorf("goast.callable: %w", err)
	}

	for _, file := range files {
		_, node, err := p.parseFile(file)
		if err != nil {
			continue
		}

		for _, decl := range node.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != name {
					continue
				}

				ft, ok := ts.Type.(*ast.FuncType)
				if !ok {
					continue
				}

				// Build params list.
				var params []ParamDetail
				if ft.Params != nil {
					for _, field := range ft.Params.List {
						typeStr := typeToString(field.Type)
						if len(field.Names) == 0 {
							params = append(params, ParamDetail{
								Type: typeStr,
							})
						} else {
							for _, ident := range field.Names {
								params = append(params, ParamDetail{
									Name: ident.Name,
									Type: typeStr,
								})
							}
						}
					}
				}

				returns := returnTypeString(ft.Results)

				// Doc comment: prefer TypeSpec.Doc, fall back to GenDecl.Doc. Use
				// commentGroupRaw to preserve directive lines.
				var cg *ast.CommentGroup
				if ts.Doc != nil {
					cg = ts.Doc
				} else if genDecl.Doc != nil {
					cg = genDecl.Doc
				}

				return CallableResult{
					Name:    name,
					Doc:     commentGroupRaw(cg),
					Params:  params,
					Returns: returns,
				}, nil
			}
		}
	}

	return CallableResult{}, fmt.Errorf("goast.callable: function type %q not found in %s", name, path)
}

// Calls returns function/method calls within a scope.
//
// Parameters:
//   - scope: the encoded scope string (path::name).
//   - name: optional filter — only return calls to this function name.
//
// Returns:
//   - []CallResult: the discovered calls.
//   - error: non-nil if the scope cannot be resolved.
//
// +devlore:defaults name=
func (p *Provider) Calls(scope, name string) ([]CallResult, error) {
	fset, body, err := p.findScopeBody(scope)
	if err != nil {
		return nil, fmt.Errorf("goast.calls: %w", err)
	}

	var result []CallResult
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		var funcName, qualifier, fullName string
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			funcName = fn.Name
			fullName = fn.Name
		case *ast.SelectorExpr:
			funcName = fn.Sel.Name
			if x, ok := fn.X.(*ast.Ident); ok {
				qualifier = x.Name
			}
			fullName = typeToString(call.Fun)
		}

		if funcName == "" {
			return true
		}

		if name != "" && funcName != name {
			return true
		}

		var args []CallArg
		for i, arg := range call.Args {
			strVal := ""
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				strVal = strings.Trim(lit.Value, `"`)
			}

			identName := ""
			switch a := arg.(type) {
			case *ast.Ident:
				identName = a.Name
			case *ast.SelectorExpr:
				identName = a.Sel.Name
			}

			args = append(args, CallArg{
				Position:    i,
				StringValue: strVal,
				IdentName:   identName,
			})
		}

		result = append(result, CallResult{
			Name:      funcName,
			Qualifier: qualifier,
			FullName:  fullName,
			Line:      fset.Position(call.Pos()).Line,
			Args:      args,
		})

		return true
	})

	return result, nil
}

// Composites returns composite literals within a scope.
//
// Parameters:
//   - scope: the encoded scope string (path::name).
//   - typeName: optional filter — only return composites of this type.
//
// Returns:
//   - []CompositeResult: the discovered composite literals.
//   - error: non-nil if the scope cannot be resolved.
//
// +devlore:defaults typeName=
func (p *Provider) Composites(scope, typeName string) ([]CompositeResult, error) {
	fset, body, err := p.findScopeBody(scope)
	if err != nil {
		return nil, fmt.Errorf("goast.composites: %w", err)
	}

	var result []CompositeResult
	ast.Inspect(body, func(n ast.Node) bool {
		comp, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		tn := ""
		if comp.Type != nil {
			tn = typeToString(comp.Type)
		}

		if typeName != "" && tn != typeName {
			return true
		}

		fields := map[string]any{}
		for _, elt := range comp.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}

			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}

			switch v := kv.Value.(type) {
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					fields[key.Name] = strings.Trim(v.Value, `"`)
				} else {
					fields[key.Name] = v.Value
				}
			case *ast.CompositeLit:
				var elems []string
				for _, elem := range v.Elts {
					if lit, ok := elem.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						elems = append(elems, strings.Trim(lit.Value, `"`))
					} else {
						elems = append(elems, typeToString(elem))
					}
				}
				fields[key.Name] = elems
			default:
				fields[key.Name] = typeToString(kv.Value)
			}
		}

		result = append(result, CompositeResult{
			TypeName: tn,
			Line:     fset.Position(comp.Pos()).Line,
			Fields:   fields,
		})

		return true
	})

	return result, nil
}

// ConstGroups returns typed const groups from Go source files.
//
// Parameters:
//   - path: the file or directory path.
//   - typeName: optional filter — only return groups of this type.
//
// Returns:
//   - []ConstGroupResult: the discovered constant groups.
//   - error: non-nil if the path cannot be accessed.
//
// +devlore:defaults typeName=
func (p *Provider) ConstGroups(path, typeName string) ([]ConstGroupResult, error) {
	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("goast.const_groups: %w", err)
	}

	type constEntry struct {
		name  string
		value string
		line  int
	}
	type group struct {
		typeName string
		file     string
		consts   []constEntry
	}

	var groups []group
	for _, file := range files {
		fset, node, err := p.parseFile(file)
		if err != nil {
			continue
		}

		ast.Inspect(node, func(n ast.Node) bool {
			genDecl, ok := n.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				return true
			}

			var currentType string
			var currentConsts []constEntry

			for _, spec := range genDecl.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}

				if vs.Type != nil {
					if ident, ok := vs.Type.(*ast.Ident); ok {
						if currentType != "" && currentType != ident.Name && len(currentConsts) > 0 {
							if typeName == "" || typeName == currentType {
								groups = append(groups, group{typeName: currentType, file: filepath.Base(file), consts: currentConsts})
							}
							currentConsts = nil
						}
						currentType = ident.Name
					}
				}

				if currentType == "" {
					continue
				}

				for i, n := range vs.Names {
					var value string
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok {
							value = strings.Trim(lit.Value, `"`)
						}
					}

					currentConsts = append(currentConsts, constEntry{
						name:  n.Name,
						value: value,
						line:  fset.Position(n.Pos()).Line,
					})
				}
			}

			if currentType != "" && len(currentConsts) > 0 {
				if typeName == "" || typeName == currentType {
					groups = append(groups, group{typeName: currentType, file: filepath.Base(file), consts: currentConsts})
				}
			}

			return true
		})
	}

	var result []ConstGroupResult
	for _, g := range groups {
		var consts []ConstDetail
		for _, c := range g.consts {
			consts = append(consts, ConstDetail{
				Name:  c.name,
				Value: c.value,
				Line:  c.line,
			})
		}

		result = append(result, ConstGroupResult{
			TypeName:  g.typeName,
			File:      g.file,
			Constants: consts,
		})
	}

	return result, nil
}

// Deps analyzes import dependencies for Go source files at the given path.
//
// Parameters:
//   - path: the file or directory path.
//
// Returns:
//   - DepsResult: the aggregated dependency information.
//   - error: non-nil if the path cannot be accessed.
func (p *Provider) Deps(path string) (DepsResult, error) {
	files, err := collectGoFiles(path)
	if err != nil {
		return DepsResult{}, fmt.Errorf("goast.deps: %w", err)
	}

	modulePath := detectModulePath(path)

	var allFiles []FileDep
	allImports := make(map[string]bool)
	allInternal := make(map[string]bool)
	allExternal := make(map[string]bool)
	allStdlib := make(map[string]bool)

	for _, file := range files {
		fd, err := analyzeFileDeps(file, modulePath)
		if err != nil {
			continue
		}

		allFiles = append(allFiles, fd)

		for _, imp := range fd.Imports {
			allImports[imp.Path] = true
		}

		for _, dep := range fd.InternalDeps {
			allInternal[dep] = true
		}

		for _, dep := range fd.ExternalDeps {
			allExternal[dep] = true
		}

		for _, dep := range fd.StdlibDeps {
			allStdlib[dep] = true
		}
	}

	return DepsResult{
		Files:         allFiles,
		ModulePath:    modulePath,
		AllImports:    mapKeys(allImports),
		InternalDeps:  mapKeys(allInternal),
		ExternalDeps:  mapKeys(allExternal),
		StdlibDeps:    mapKeys(allStdlib),
		InternalCount: len(allInternal),
		ExternalCount: len(allExternal),
		StdlibCount:   len(allStdlib),
	}, nil
}

// Format formats Go source code via go/format.
//
// Parameters:
//   - code: the Go source code to format.
//
// Returns:
//   - string: the formatted source code.
//   - error: non-nil if the code cannot be formatted.
func (p *Provider) Format(code string) (string, error) {
	formatted, err := format.Source([]byte(code))
	if err != nil {
		return "", fmt.Errorf("goast.format: %w", err)
	}

	return string(formatted), nil
}

// Funcs returns function declarations (non-method) from Go source files.
//
// Parameters:
//   - path: the file or directory path.
//   - name: optional filter — only return functions with this name.
//
// Returns:
//   - []FuncResult: the discovered functions.
//   - error: non-nil if the path cannot be accessed.
//
// +devlore:defaults name=
func (p *Provider) Funcs(path, name string) ([]FuncResult, error) {
	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("goast.funcs: %w", err)
	}

	var result []FuncResult
	for _, file := range files {
		fset, node, err := p.parseFile(file)
		if err != nil {
			continue
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil {
				continue
			}

			if name != "" && fn.Name.Name != name {
				continue
			}

			returns := returnTypeString(fn.Type.Results)
			rawDoc := ""
			if fn.Doc != nil {
				rawDoc = commentGroupRaw(fn.Doc)
			}

			pNames := astParamNames(fn.Type.Params)
			rTypes := astReturnTypes(fn.Type.Results)
			funcDoc := parseFuncDocSafe(rawDoc, pNames, rTypes)

			scope := encodeScope(file, fn.Name.Name)

			result = append(result, FuncResult{
				Name:    fn.Name.Name,
				Returns: returns,
				Params:  extractParams(fn.Type.Params, funcDoc.ParamDocs()),
				File:    filepath.Base(file),
				Line:    fset.Position(fn.Pos()).Line,
				Doc:     rawDoc,
				Comment: funcDoc,
				Scope:   scope,
			})
		}
	}

	return result, nil
}

// Methods returns method declarations from Go source files.
//
// Parameters:
//   - path: the file or directory path.
//   - name: optional filter — only return methods with this name.
//   - receiverType: optional filter — only return methods on this receiver type.
//   - returns: optional filter — only return methods with this return type string.
//
// Returns:
//   - []MethodResult: the discovered methods.
//   - error: non-nil if the path cannot be accessed.
//
// +devlore:defaults name=,receiverType=,returns=
func (p *Provider) Methods(path, name, receiverType, returns string) ([]MethodResult, error) {
	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("goast.methods: %w", err)
	}

	var result []MethodResult
	for _, file := range files {
		fset, node, err := p.parseFile(file)
		if err != nil {
			continue
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Name == nil {
				continue
			}

			if name != "" && fn.Name.Name != name {
				continue
			}

			recvType := receiverTypeName(fn.Recv.List[0].Type)
			if receiverType != "" {
				if strings.HasPrefix(receiverType, "*") {
					if recvType != receiverType {
						continue
					}
				} else {
					if strings.TrimPrefix(recvType, "*") != receiverType {
						continue
					}
				}
			}

			retStr := returnTypeString(fn.Type.Results)
			if returns != "" && retStr != returns {
				continue
			}

			rawDoc := ""
			if fn.Doc != nil {
				rawDoc = commentGroupRaw(fn.Doc)
			}

			pNames := astParamNames(fn.Type.Params)
			rTypes := astReturnTypes(fn.Type.Results)
			funcDoc := parseFuncDocSafe(rawDoc, pNames, rTypes)

			scope := encodeScope(file, strings.TrimPrefix(recvType, "*")+"."+fn.Name.Name)

			result = append(result, MethodResult{
				Name:         fn.Name.Name,
				ReceiverType: recvType,
				Returns:      retStr,
				Params:       extractParams(fn.Type.Params, funcDoc.ParamDocs()),
				File:         filepath.Base(file),
				Line:         fset.Position(fn.Pos()).Line,
				Doc:          rawDoc,
				Comment:      funcDoc,
				Scope:        scope,
			})
		}
	}

	return result, nil
}

// Metrics computes code metrics for Go source files at the given path.
//
// Parameters:
//   - path: the file or directory path.
//
// Returns:
//   - MetricsResult: the aggregated metrics.
//   - error: non-nil if the path cannot be accessed.
func (p *Provider) Metrics(path string) (MetricsResult, error) {
	files, err := collectGoFiles(path)
	if err != nil {
		return MetricsResult{}, fmt.Errorf("goast.metrics: %w", err)
	}

	var allFiles []FileMetric
	var totals FileMetric

	for _, file := range files {
		fm, err := analyzeFileMetrics(file)
		if err != nil {
			continue
		}

		allFiles = append(allFiles, fm)

		totals.LOC += fm.LOC
		totals.SLOC += fm.SLOC
		totals.Comments += fm.Comments
		totals.Blanks += fm.Blanks
		totals.Functions += fm.Functions
		totals.Methods += fm.Methods
		totals.Structs += fm.Structs
		totals.Interfaces += fm.Interfaces
		totals.Types += fm.Types
		totals.Constants += fm.Constants
		totals.Variables += fm.Variables
		totals.Imports += fm.Imports
		totals.TestFunctions += fm.TestFunctions
	}

	return MetricsResult{
		Files:              allFiles,
		FileCount:          len(files),
		TotalLOC:           totals.LOC,
		TotalSLOC:          totals.SLOC,
		TotalComments:      totals.Comments,
		TotalBlanks:        totals.Blanks,
		TotalFunctions:     totals.Functions,
		TotalMethods:       totals.Methods,
		TotalStructs:       totals.Structs,
		TotalInterfaces:    totals.Interfaces,
		TotalTypes:         totals.Types,
		TotalConstants:     totals.Constants,
		TotalVariables:     totals.Variables,
		TotalImports:       totals.Imports,
		TotalTestFunctions: totals.TestFunctions,
	}, nil
}

// RawString extracts the first backtick string literal from a scope.
//
// Parameters:
//   - scope: the encoded scope string (path::name).
//
// Returns:
//   - string: the raw string content.
//   - error: non-nil if the scope cannot be resolved.
func (p *Provider) RawString(scope string) (string, error) {
	_, body, err := p.findScopeBody(scope)
	if err != nil {
		return "", fmt.Errorf("goast.raw_string: %w", err)
	}

	var rawStr string
	ast.Inspect(body, func(n ast.Node) bool {
		if rawStr != "" {
			return false
		}

		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}

		if strings.HasPrefix(lit.Value, "`") {
			rawStr = strings.Trim(lit.Value, "`")
			return false
		}

		return true
	})

	return rawStr, nil
}

// Render executes a Go text/template against data and returns go/format-formatted Go source code.
//
// Parameters:
//   - template: the Go template string.
//   - data: the template data (any type — dict, list, string, etc.).
//
// Returns:
//   - string: the formatted Go source code.
//   - error: non-nil if template parsing, execution, or formatting fails.
func (p *Provider) Render(template string, data any) (string, error) {
	t, err := tmpl.New("render").Funcs(renderFuncs).Parse(template)
	if err != nil {
		return "", fmt.Errorf("goast.render: template parse: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("goast.render: template execution: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("goast.render: format error: %w\nraw output:\n%s", err, buf.String())
	}

	return string(formatted), nil
}

// ReturnString extracts the string literal from a return statement in a scope.
//
// Parameters:
//   - scope: the encoded scope string (path::name).
//
// Returns:
//   - string: the extracted string.
//   - error: non-nil if the scope cannot be resolved.
func (p *Provider) ReturnString(scope string) (string, error) {
	_, body, err := p.findScopeBody(scope)
	if err != nil {
		return "", fmt.Errorf("goast.return_string: %w", err)
	}

	return extractReturnString(body), nil
}

// ReturnStrings extracts string elements from a []string{...} return statement in a scope.
//
// Parameters:
//   - scope: the encoded scope string (path::name).
//
// Returns:
//   - []string: the extracted strings.
//   - error: non-nil if the scope cannot be resolved.
func (p *Provider) ReturnStrings(scope string) ([]string, error) {
	_, body, err := p.findScopeBody(scope)
	if err != nil {
		return nil, fmt.Errorf("goast.return_strings: %w", err)
	}

	return extractReturnStrings(body), nil
}

// RewrapComments rewraps all doc comment paragraphs in a Go file to fill to the specified column width. Indented code
// blocks (4+ spaces after "//") are preserved unchanged. Returns the modified file content.
//
// Parameters:
//   - path: the Go source file path.
//   - width: the target line width in columns (e.g., 120).
//
// Returns:
//   - string: the modified file content with rewrapped comments.
//   - error: non-nil if the file cannot be read or parsed.
func (p *Provider) RewrapComments(path string, width int) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("goast.rewrap_comments: %w", err)
	}

	result, err := rewriteFileFromSource(path, string(content), width)
	if err != nil {
		return "", fmt.Errorf("goast.rewrap_comments: %w", err)
	}

	return result, nil
}

// SortDeclarations reorders function/method declarations within a scope of a Go file. Preserves doc comments and blank
// lines attached to each declaration. Returns the modified file content.
//
// Parameters:
//   - path: the Go source file path.
//   - scope: the scope to sort within — "file" for all top-level declarations, or "lines:START-END" for a line range.
//   - order: the sort order — "alphabetical" sorts by declaration name.
//
// Returns:
//   - string: the modified file content with sorted declarations.
//   - error: non-nil if the file cannot be read, parsed, or the scope is invalid.
func (p *Provider) SortDeclarations(path, scope, order string) (string, error) {

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("goast.sort_declarations: %w", err)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("goast.sort_declarations: %w", err)
	}

	startLine, endLine, err := parseScopeRange(scope, len(strings.Split(string(content), "\n")))
	if err != nil {
		return "", fmt.Errorf("goast.sort_declarations: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Collect function declarations within the scope.
	type declBlock struct {
		name      string
		startLine int // 1-indexed, inclusive
		endLine   int // 1-indexed, inclusive
	}

	var blocks []declBlock
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}

		dStart := fset.Position(fn.Pos()).Line
		dEnd := fset.Position(fn.End()).Line

		// Include doc comment.
		if fn.Doc != nil {
			docStart := fset.Position(fn.Doc.Pos()).Line
			if docStart < dStart {
				dStart = docStart
			}
		}

		// Skip declarations outside the scope.
		if dStart < startLine || dEnd > endLine {
			continue
		}

		blocks = append(blocks, declBlock{
			name:      fn.Name.Name,
			startLine: dStart,
			endLine:   dEnd,
		})
	}

	if len(blocks) <= 1 {
		return string(content), nil
	}

	// Record the overall range of all blocks.
	overallStart := blocks[0].startLine
	overallEnd := blocks[0].endLine
	for _, b := range blocks {
		if b.startLine < overallStart {
			overallStart = b.startLine
		}
		if b.endLine > overallEnd {
			overallEnd = b.endLine
		}
	}

	// Extract text for each block before sorting.
	blockTexts := make(map[string]string, len(blocks))
	for _, b := range blocks {
		text := strings.Join(lines[b.startLine-1:b.endLine], "\n")
		blockTexts[b.name] = strings.TrimRight(text, " \t\n")
	}

	// Sort blocks.
	switch order {
	case "alphabetical", "":
		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].name < blocks[j].name
		})
	default:
		return "", fmt.Errorf("goast.sort_declarations: unknown order: %s", order)
	}

	// Build sorted content.
	var sortedParts []string
	for _, b := range blocks {
		sortedParts = append(sortedParts, blockTexts[b.name])
	}
	replacement := strings.Join(sortedParts, "\n\n")

	// Replace the overall range in the file.
	before := lines[:overallStart-1]
	after := lines[overallEnd:]

	var resultLines []string
	resultLines = append(resultLines, before...)
	resultLines = append(resultLines, strings.Split(replacement, "\n")...)
	resultLines = append(resultLines, after...)

	return strings.Join(resultLines, "\n"), nil
}

// Structs returns struct definitions from Go source files.
//
// Parameters:
//   - path: the file or directory path.
//
// Returns:
//   - []StructResult: the discovered struct definitions.
//   - error: non-nil if the path cannot be accessed.
func (p *Provider) Structs(path string) ([]StructResult, error) {
	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("goast.structs: %w", err)
	}

	var result []StructResult
	for _, file := range files {
		fset, node, err := p.parseFile(file)
		if err != nil {
			continue
		}

		ast.Inspect(node, func(n ast.Node) bool {
			genDecl, ok := n.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				return true
			}

			for _, spec := range genDecl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}

				var fields []FieldDetail
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
						// Embedded field.
						fieldType := typeToString(field.Type)
						fields = append(fields, FieldDetail{
							Name:     fieldType,
							Type:     fieldType,
							Embedded: true,
						})
						continue
					}

					jsonName := ""
					required := false
					if field.Tag != nil {
						tag := strings.Trim(field.Tag.Value, "`")
						jsonName, required = parseJSONTag(tag)
					}

					if jsonName == "-" {
						continue
					}

					desc := ""
					if field.Comment != nil {
						desc = strings.TrimSpace(field.Comment.Text())
					} else if field.Doc != nil {
						desc = strings.TrimSpace(field.Doc.Text())
					}

					fieldType := typeToString(field.Type)
					for _, ident := range field.Names {
						fields = append(fields, FieldDetail{
							Name:        ident.Name,
							JSONName:    jsonName,
							Type:        fieldType,
							Required:    required,
							Description: desc,
						})
					}
				}

				// Parse type doc with taxonomy.
				var typeDoc *doctaxonomy.TypeDoc
				if ts.Doc != nil {
					raw := commentGroupRaw(ts.Doc)
					if raw != "" {
						tp := doctaxonomy.NewTypeParser()
						typeDoc, _ = tp.ParseString("", raw)
					}
				} else if genDecl.Doc != nil && len(genDecl.Specs) == 1 {
					raw := commentGroupRaw(genDecl.Doc)
					if raw != "" {
						tp := doctaxonomy.NewTypeParser()
						typeDoc, _ = tp.ParseString("", raw)
					}
				}

				result = append(result, StructResult{
					Name:    ts.Name.Name,
					File:    filepath.Base(file),
					Line:    fset.Position(ts.Pos()).Line,
					Fields:  fields,
					Comment: typeDoc,
				})
			}

			return true
		})
	}

	return result, nil
}

// TypeDoc returns the doc comment for a named type declaration.
//
// Parameters:
//   - path: the file or directory path.
//   - name: optional type name (defaults to "Provider" if empty).
//
// Returns:
//   - string: the doc comment text.
//   - error: non-nil if the path cannot be accessed.
//
// +devlore:defaults name=
func (p *Provider) TypeDoc(path, name string) (string, error) {
	if name == "" {
		name = "Provider"
	}

	files, err := collectGoFiles(path)
	if err != nil {
		return "", fmt.Errorf("goast.type_doc: %w", err)
	}

	for _, file := range files {
		_, node, err := p.parseFile(file)
		if err != nil {
			continue
		}

		for _, decl := range node.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != name {
					continue
				}

				var cg *ast.CommentGroup
				if ts.Doc != nil {
					cg = ts.Doc
				} else if genDecl.Doc != nil {
					cg = genDecl.Doc
				}

				return commentGroupRaw(cg), nil
			}
		}
	}

	return "", nil
}

// endregion

// endregion

// =============================================================================
// UNEXPORTED HELPERS
// =============================================================================

// mapKeys returns the keys of a map as a string slice.
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}
