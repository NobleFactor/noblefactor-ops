// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package goast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode"
)

// =============================================================================
// PARSED FILE CACHE
// =============================================================================

// parsedFile holds a cached parsed Go file.
type parsedFile struct {
	fset *token.FileSet
	node *ast.File
}

// parseFile parses a Go file with caching.
//
// Parameters:
//   - path: the file path to parse.
//
// Returns:
//   - *token.FileSet: the file set.
//   - *ast.File: the parsed AST.
//   - error: non-nil if parsing fails.
func (p *Provider) parseFile(path string) (*token.FileSet, *ast.File, error) {
	if cached, ok := p.fileCache.Load(path); ok {
		pf := cached.(*parsedFile)
		return pf.fset, pf.node, nil
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	p.fileCache.Store(path, &parsedFile{fset: fset, node: node})

	return fset, node, nil
}

// =============================================================================
// SCOPE ENCODING
// =============================================================================

// encodeScope creates an opaque scope string from a file path and function name.
//
// Parameters:
//   - filePath: the absolute file path.
//   - name: the function or method name (e.g., "Foo" or "Provider.Foo").
//
// Returns:
//   - string: the encoded scope.
func encodeScope(filePath, name string) string {
	return filePath + "::" + name
}

// decodeScope splits a scope string into file path and function name.
//
// Parameters:
//   - scope: the encoded scope string.
//
// Returns:
//   - string: the file path.
//   - string: the function or method name.
//   - error: non-nil if the scope format is invalid.
func decodeScope(scope string) (string, string, error) {
	parts := strings.SplitN(scope, "::", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid scope: %s", scope)
	}

	return parts[0], parts[1], nil
}

// findScopeBody resolves a scope string to the function/method body AST node.
//
// Parameters:
//   - scope: the encoded scope string.
//
// Returns:
//   - *token.FileSet: the file set.
//   - *ast.BlockStmt: the function body.
//   - error: non-nil if the scope cannot be resolved.
func (p *Provider) findScopeBody(scope string) (*token.FileSet, *ast.BlockStmt, error) {
	filePath, name, err := decodeScope(scope)
	if err != nil {
		return nil, nil, err
	}

	fset, node, err := p.parseFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	parts := strings.SplitN(name, ".", 2)
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}

		if len(parts) == 2 {
			if fn.Name.Name != parts[1] || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}

			recvType := receiverTypeName(fn.Recv.List[0].Type)
			if strings.TrimPrefix(recvType, "*") == strings.TrimPrefix(parts[0], "*") {
				return fset, fn.Body, nil
			}
		} else {
			if fn.Name.Name == name && fn.Recv == nil {
				return fset, fn.Body, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("scope not found: %s", scope)
}

// =============================================================================
// FILE COLLECTION
// =============================================================================

// collectGoFiles returns Go source files for a path. If path is a file, returns
// it directly. If a directory, walks it recursively, skipping vendor, testdata,
// .git, and _test.go files.
//
// Parameters:
//   - path: the file or directory path.
//
// Returns:
//   - []string: the collected file paths.
//   - error: non-nil if the path cannot be accessed.
func collectGoFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && (d.Name() == "vendor" || d.Name() == "testdata" || d.Name() == ".git") {
			return filepath.SkipDir
		}

		if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") {
			files = append(files, p)
		}

		return nil
	})

	return files, err
}

// =============================================================================
// MODULE DETECTION
// =============================================================================

// detectModulePath finds the Go module path by walking up from startPath looking for go.mod.
//
// Parameters:
//   - startPath: the file or directory to start searching from.
//
// Returns:
//   - string: the module path, or empty string if not found.
func detectModulePath(startPath string) string {
	dir := startPath

	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	for {
		modPath := filepath.Join(dir, "go.mod")
		if content, err := os.ReadFile(modPath); err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module "))
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return ""
}

// isStdlib returns true if the import path belongs to the Go standard library.
//
// Parameters:
//   - importPath: the import path to check.
//
// Returns:
//   - bool: true if stdlib.
func isStdlib(importPath string) bool {
	if !strings.Contains(importPath, ".") {
		return true
	}

	if strings.HasPrefix(importPath, "golang.org/x/") {
		return false
	}

	return false
}

// =============================================================================
// AST TYPE HELPERS
// =============================================================================

// typeToString formats any Go AST type expression as a string.
//
// Parameters:
//   - expr: the AST expression to format.
//
// Returns:
//   - string: the formatted type string.
func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + typeToString(t.Key) + "]" + typeToString(t.Value)
	case *ast.Ellipsis:
		return "..." + typeToString(t.Elt)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "any"
		}
		return "interface{...}"
	case *ast.FuncType:
		var params []string
		if t.Params != nil {
			for _, p := range t.Params.List {
				ts := typeToString(p.Type)
				n := len(p.Names)
				if n == 0 {
					n = 1
				}
				for range n {
					params = append(params, ts)
				}
			}
		}

		ret := returnTypeString(t.Results)
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}

		return "func(" + strings.Join(params, ", ") + ") " + ret
	case *ast.ChanType:
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + typeToString(t.Value)
		case ast.RECV:
			return "<-chan " + typeToString(t.Value)
		default:
			return "chan " + typeToString(t.Value)
		}
	case *ast.IndexExpr:
		return typeToString(t.X) + "[" + typeToString(t.Index) + "]"
	case *ast.IndexListExpr:
		var indices []string
		for _, idx := range t.Indices {
			indices = append(indices, typeToString(idx))
		}
		return typeToString(t.X) + "[" + strings.Join(indices, ", ") + "]"
	default:
		return "unknown"
	}
}

// receiverTypeName extracts the type name from a receiver expression.
//
// Parameters:
//   - expr: the receiver type expression.
//
// Returns:
//   - string: the type name, with "*" prefix for pointer receivers.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	case *ast.Ident:
		return t.Name
	}

	return ""
}

// returnTypeString formats a function's return type list as a string.
//
// Parameters:
//   - results: the return field list.
//
// Returns:
//   - string: the formatted return type, or empty for void.
func returnTypeString(results *ast.FieldList) string {
	if results == nil || len(results.List) == 0 {
		return ""
	}

	if len(results.List) == 1 {
		return typeToString(results.List[0].Type)
	}

	var types []string
	for _, r := range results.List {
		types = append(types, typeToString(r.Type))
	}

	return "(" + strings.Join(types, ", ") + ")"
}

// =============================================================================
// COMMENT HELPERS
// =============================================================================

// commentGroupRaw returns the full text of a comment group, preserving directive
// lines (//tool:directive) that ast.CommentGroup.Text() strips since Go 1.21.
//
// Parameters:
//   - cg: the comment group to extract text from.
//
// Returns:
//   - string: the trimmed comment text.
func commentGroupRaw(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}

	var lines []string
	for _, c := range cg.List {
		text := c.Text
		if strings.HasPrefix(text, "//") {
			text = strings.TrimPrefix(text, "//")
			// Strip at most one leading space (standard Go comment style).
			if len(text) > 0 && text[0] == ' ' {
				text = text[1:]
			}
		}
		lines = append(lines, text)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// =============================================================================
// DOC COMMENT PARSING
// =============================================================================

// parseParamDocs extracts parameter documentation from a method doc comment. It
// looks for a "Parameters:" section and parses "- name: description" lines.
// Returns the doc with the Parameters: section removed, and a map of param name
// to description.
//
// Parameters:
//   - doc: the raw doc comment text.
//
// Returns:
//   - string: the doc with Parameters section removed.
//   - map[string]string: parameter name to description mapping.
func parseParamDocs(doc string) (string, map[string]string) {
	docs := make(map[string]string)
	if doc == "" {
		return doc, docs
	}

	lines := strings.Split(doc, "\n")
	var descLines []string
	inParams := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Parameters:" {
			inParams = true
			continue
		}

		if inParams {
			if strings.HasPrefix(trimmed, "- ") {
				entry := strings.TrimPrefix(trimmed, "- ")
				if colonIdx := strings.Index(entry, ": "); colonIdx > 0 {
					docs[entry[:colonIdx]] = entry[colonIdx+2:]
				}
				continue
			}

			if trimmed == "" {
				inParams = false
				continue
			}

			// Non-entry line ends the Parameters section.
			inParams = false
			descLines = append(descLines, line)
			continue
		}

		descLines = append(descLines, line)
	}

	cleanDoc := strings.TrimSpace(strings.Join(descLines, "\n"))

	return cleanDoc, docs
}

// =============================================================================
// PARAMETER EXTRACTION
// =============================================================================

// extractParams converts a function's parameter list to a slice of ParamDetail structs.
//
// Parameters:
//   - params: the AST parameter field list.
//   - paramDocs: optional map of parameter name to doc string.
//
// Returns:
//   - []ParamDetail: the extracted parameter details.
func extractParams(params *ast.FieldList, paramDocs map[string]string) []ParamDetail {
	var result []ParamDetail
	if params == nil {
		return result
	}

	for i, field := range params.List {
		isLast := i == len(params.List)-1
		_, isEllipsis := field.Type.(*ast.Ellipsis)
		variadic := isLast && isEllipsis

		typeStr := typeToString(field.Type)
		if variadic {
			typeStr = strings.TrimPrefix(typeStr, "...")
		}

		if len(field.Names) == 0 {
			result = append(result, ParamDetail{
				Type:     typeStr,
				Variadic: variadic,
			})
		} else {
			for _, name := range field.Names {
				doc := ""
				if paramDocs != nil {
					doc = paramDocs[name.Name]
				}

				result = append(result, ParamDetail{
					Name:     name.Name,
					Type:     typeStr,
					Variadic: variadic,
					Doc:      doc,
				})
			}
		}
	}

	return result
}

// =============================================================================
// RETURN VALUE EXTRACTION
// =============================================================================

// extractReturnString extracts the first string literal from a return statement.
//
// Parameters:
//   - body: the function body block.
//
// Returns:
//   - string: the extracted string, or empty if not found.
func extractReturnString(body *ast.BlockStmt) string {
	if body == nil || len(body.List) == 0 {
		return ""
	}

	for _, stmt := range body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}

		lit, ok := ret.Results[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}

		return strings.Trim(lit.Value, `"`)
	}

	return ""
}

// extractReturnStrings extracts string elements from a []string{...} return statement.
//
// Parameters:
//   - body: the function body block.
//
// Returns:
//   - []string: the extracted strings, or nil if not found.
func extractReturnStrings(body *ast.BlockStmt) []string {
	if body == nil || len(body.List) == 0 {
		return nil
	}

	for _, stmt := range body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}

		comp, ok := ret.Results[0].(*ast.CompositeLit)
		if !ok {
			continue
		}

		// Verify the type is []string.
		arr, ok := comp.Type.(*ast.ArrayType)
		if !ok || arr.Len != nil {
			continue
		}

		ident, ok := arr.Elt.(*ast.Ident)
		if !ok || ident.Name != "string" {
			continue
		}

		var result []string
		for _, elt := range comp.Elts {
			lit, ok := elt.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			result = append(result, strings.Trim(lit.Value, `"`))
		}

		return result
	}

	return nil
}

// =============================================================================
// JSON TAG PARSING
// =============================================================================

// parseJSONTag extracts the JSON field name and required status from a struct tag.
//
// Parameters:
//   - tag: the raw struct tag string.
//
// Returns:
//   - string: the JSON field name.
//   - bool: true if the field is required (no omitempty).
func parseJSONTag(tag string) (string, bool) {
	jsonRe := regexp.MustCompile(`json:"([^"]*)"`)
	match := jsonRe.FindStringSubmatch(tag)
	if match == nil {
		return "", false
	}

	parts := strings.Split(match[1], ",")
	name := parts[0]
	required := true

	for _, part := range parts[1:] {
		if part == "omitempty" {
			required = false
		}
	}

	return name, required
}

// =============================================================================
// METRICS ANALYSIS
// =============================================================================

// analyzeFileMetrics computes code metrics for a single Go file.
//
// Parameters:
//   - path: the file path to analyze.
//
// Returns:
//   - FileMetric: the computed metrics.
//   - error: non-nil if the file cannot be read or parsed.
func analyzeFileMetrics(path string) (FileMetric, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FileMetric{}, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return FileMetric{}, err
	}

	fm := FileMetric{Path: path}

	lines := strings.Split(string(content), "\n")
	fm.LOC = len(lines)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			fm.Blanks++
		}
	}

	for _, cg := range node.Comments {
		for _, c := range cg.List {
			fm.Comments += strings.Count(c.Text, "\n") + 1
		}
	}

	fm.SLOC = fm.LOC - fm.Blanks - fm.Comments
	if fm.SLOC < 0 {
		fm.SLOC = 0
	}

	fm.Imports = len(node.Imports)

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Recv != nil {
				fm.Methods++
			} else {
				fm.Functions++
			}

			if strings.HasPrefix(x.Name.Name, "Test") || strings.HasPrefix(x.Name.Name, "Benchmark") {
				fm.TestFunctions++
			}
		case *ast.GenDecl:
			switch x.Tok {
			case token.TYPE:
				for _, spec := range x.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						fm.Types++

						switch ts.Type.(type) {
						case *ast.StructType:
							fm.Structs++
						case *ast.InterfaceType:
							fm.Interfaces++
						}
					}
				}
			case token.CONST:
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						fm.Constants += len(vs.Names)
					}
				}
			case token.VAR:
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						fm.Variables += len(vs.Names)
					}
				}
			}
		}

		return true
	})

	return fm, nil
}

// =============================================================================
// DEPENDENCY ANALYSIS
// =============================================================================

// analyzeFileDeps analyzes import dependencies for a single Go file.
//
// Parameters:
//   - path: the file path to analyze.
//   - modulePath: the Go module path for classifying internal deps.
//
// Returns:
//   - FileDep: the dependency analysis result.
//   - error: non-nil if the file cannot be parsed.
func analyzeFileDeps(path, modulePath string) (FileDep, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return FileDep{}, err
	}

	fd := FileDep{
		Path:    path,
		Package: node.Name.Name,
	}

	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		line := fset.Position(imp.Pos()).Line

		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}

		fd.Imports = append(fd.Imports, ImportDetail{
			Path:  importPath,
			Alias: alias,
			Line:  line,
		})

		switch {
		case isStdlib(importPath):
			fd.StdlibDeps = append(fd.StdlibDeps, importPath)
		case modulePath != "" && strings.HasPrefix(importPath, modulePath):
			fd.InternalDeps = append(fd.InternalDeps, importPath)
		default:
			fd.ExternalDeps = append(fd.ExternalDeps, importPath)
		}
	}

	return fd, nil
}

// =============================================================================
// TEMPLATE FUNCTIONS
// =============================================================================

// renderFuncs provides general-purpose template functions for Render.
var renderFuncs = template.FuncMap{
	"camelToSnake": renderCamelToSnake,
	"lcFirst":      renderLCFirst,
	"join":         strings.Join,
}

// renderCamelToSnake converts CamelCase Go names to snake_case.
//
// Parameters:
//   - s: the CamelCase string.
//
// Returns:
//   - string: the snake_case equivalent.
func renderCamelToSnake(s string) string {
	runes := []rune(s)
	var result []rune

	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) {
					result = append(result, '_')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					result = append(result, '_')
				}
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}

	return string(result)
}

// renderLCFirst lowercases the first character of a string.
//
// Parameters:
//   - s: the input string.
//
// Returns:
//   - string: the string with its first character lowercased.
func renderLCFirst(s string) string {
	if s == "" {
		return s
	}

	return strings.ToLower(s[:1]) + s[1:]
}
