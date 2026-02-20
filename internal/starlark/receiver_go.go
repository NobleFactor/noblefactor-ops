// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// GoReceiver provides Go source parsing operations.
// Implements starlark.Value and starlark.HasAttrs.
type GoReceiver struct {
	BaseReceiver
	fileCache sync.Map // path → *parsedFile
}

// parsedFile holds a cached parsed Go file.
type parsedFile struct {
	fset *token.FileSet
	node *ast.File
}

// NewGoReceiver creates a new GoReceiver.
func NewGoReceiver() *GoReceiver {
	return &GoReceiver{BaseReceiver: NewBaseReceiver("go")}
}

// parseFile parses a Go file with caching.
func (r *GoReceiver) parseFile(path string) (*token.FileSet, *ast.File, error) {
	if cached, ok := r.fileCache.Load(path); ok {
		pf := cached.(*parsedFile)
		return pf.fset, pf.node, nil
	}
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	r.fileCache.Store(path, &parsedFile{fset: fset, node: node})
	return fset, node, nil
}

// encodeScope creates an opaque scope string from a file path and function name.
func encodeScope(filePath, name string) string {
	return filePath + "::" + name
}

// decodeScope splits a scope string into file path and function name.
func decodeScope(scope string) (string, string, error) {
	parts := strings.SplitN(scope, "::", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid scope: %s", scope)
	}
	return parts[0], parts[1], nil
}

// findScopeBody resolves a scope string to the function/method body AST node.
func (r *GoReceiver) findScopeBody(scope string) (*token.FileSet, *ast.BlockStmt, error) {
	filePath, name, err := decodeScope(scope)
	if err != nil {
		return nil, nil, err
	}
	fset, node, err := r.parseFile(filePath)
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

// receiverTypeName extracts the type name from a receiver expression.
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

// optionalString extracts a Go string from an optional Starlark value.
func optionalString(v starlark.Value) string {
	if v == nil || v == starlark.None {
		return ""
	}
	s, _ := starlark.AsString(v)
	return s
}

// Attr implements starlark.HasAttrs.
func (r *GoReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "calls":
		return MakeAttr("go.calls", r.goCalls), nil
	case "composites":
		return MakeAttr("go.composites", r.goComposites), nil
	case "const_groups":
		return MakeAttr("go.const_groups", r.constGroups), nil
	case "deps":
		return MakeAttr("go.deps", r.deps), nil
	case "funcs":
		return MakeAttr("go.funcs", r.goFuncs), nil
	case "generate":
		return MakeAttr("go.generate", r.goGenerate), nil
	case "mapping":
		return MakeAttr("go.mapping", r.goMapping), nil
	case "methods":
		return MakeAttr("go.methods", r.goMethods), nil
	case "metrics":
		return MakeAttr("go.metrics", r.metrics), nil
	case "raw_string":
		return MakeAttr("go.raw_string", r.goRawString), nil
	case "return_string":
		return MakeAttr("go.return_string", r.goReturnString), nil
	case "structs":
		return MakeAttr("go.structs", r.goStructs), nil
	case "template":
		return MakeAttr("go.template", r.goTemplate), nil
	case "type_doc":
		return MakeAttr("go.type_doc", r.goTypeDoc), nil
	default:
		return nil, NoSuchAttrError("go", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *GoReceiver) AttrNames() []string {
	return []string{"calls", "composites", "const_groups", "deps", "funcs", "generate", "mapping", "methods", "metrics", "raw_string", "return_string", "structs", "template", "type_doc"}
}

// =============================================================================
// CODE METRICS
// =============================================================================

// FileMetrics holds code metrics for a file.
type FileMetrics struct {
	Path          string
	LOC           int
	SLOC          int
	Comments      int
	Blanks        int
	Functions     int
	Methods       int
	Structs       int
	Interfaces    int
	Types         int
	Constants     int
	Variables     int
	Imports       int
	TestFunctions int
}

func (r *GoReceiver) metrics(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.metrics", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.metrics: %w", err)
	}

	var allFiles []starlark.Value
	totals := FileMetrics{}

	for _, file := range files {
		fm, err := analyzeFileMetrics(file)
		if err != nil {
			continue
		}

		allFiles = append(allFiles, fileMetricsToStarlark(fm))

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

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"files":                starlark.NewList(allFiles),
		"file_count":           starlark.MakeInt(len(files)),
		"total_loc":            starlark.MakeInt(totals.LOC),
		"total_sloc":           starlark.MakeInt(totals.SLOC),
		"total_comments":       starlark.MakeInt(totals.Comments),
		"total_blanks":         starlark.MakeInt(totals.Blanks),
		"total_functions":      starlark.MakeInt(totals.Functions),
		"total_methods":        starlark.MakeInt(totals.Methods),
		"total_structs":        starlark.MakeInt(totals.Structs),
		"total_interfaces":     starlark.MakeInt(totals.Interfaces),
		"total_types":          starlark.MakeInt(totals.Types),
		"total_constants":      starlark.MakeInt(totals.Constants),
		"total_variables":      starlark.MakeInt(totals.Variables),
		"total_imports":        starlark.MakeInt(totals.Imports),
		"total_test_functions": starlark.MakeInt(totals.TestFunctions),
	}), nil
}

func analyzeFileMetrics(path string) (FileMetrics, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FileMetrics{}, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return FileMetrics{}, err
	}

	fm := FileMetrics{Path: path}

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

func fileMetricsToStarlark(fm FileMetrics) starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":           starlark.String(fm.Path),
		"loc":            starlark.MakeInt(fm.LOC),
		"sloc":           starlark.MakeInt(fm.SLOC),
		"comments":       starlark.MakeInt(fm.Comments),
		"blanks":         starlark.MakeInt(fm.Blanks),
		"functions":      starlark.MakeInt(fm.Functions),
		"methods":        starlark.MakeInt(fm.Methods),
		"structs":        starlark.MakeInt(fm.Structs),
		"interfaces":     starlark.MakeInt(fm.Interfaces),
		"types":          starlark.MakeInt(fm.Types),
		"constants":      starlark.MakeInt(fm.Constants),
		"variables":      starlark.MakeInt(fm.Variables),
		"imports":        starlark.MakeInt(fm.Imports),
		"test_functions": starlark.MakeInt(fm.TestFunctions),
	})
}

// =============================================================================
// DEPENDENCY ANALYSIS
// =============================================================================

// FileDeps holds dependency information for a file.
type FileDeps struct {
	Path         string
	Package      string
	Imports      []ImportInfo
	InternalDeps []string
	ExternalDeps []string
	StdlibDeps   []string
}

// ImportInfo holds information about a single import.
type ImportInfo struct {
	Path  string
	Alias string
	Line  int
}

func (r *GoReceiver) deps(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.deps", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.deps: %w", err)
	}

	modulePath := detectModulePath(path)

	var allFiles []starlark.Value
	allImports := make(map[string]bool)
	allInternal := make(map[string]bool)
	allExternal := make(map[string]bool)
	allStdlib := make(map[string]bool)

	for _, file := range files {
		fd, err := analyzeFileDeps(file, modulePath)
		if err != nil {
			continue
		}

		allFiles = append(allFiles, fileDepsToStarlark(fd))

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

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"files":          starlark.NewList(allFiles),
		"module_path":    starlark.String(modulePath),
		"all_imports":    mapKeysToStarlarkList(allImports),
		"internal_deps":  mapKeysToStarlarkList(allInternal),
		"external_deps":  mapKeysToStarlarkList(allExternal),
		"stdlib_deps":    mapKeysToStarlarkList(allStdlib),
		"internal_count": starlark.MakeInt(len(allInternal)),
		"external_count": starlark.MakeInt(len(allExternal)),
		"stdlib_count":   starlark.MakeInt(len(allStdlib)),
	}), nil
}

func analyzeFileDeps(path, modulePath string) (FileDeps, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return FileDeps{}, err
	}

	fd := FileDeps{
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

		fd.Imports = append(fd.Imports, ImportInfo{
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

func isStdlib(importPath string) bool {
	if !strings.Contains(importPath, ".") {
		return true
	}
	if strings.HasPrefix(importPath, "golang.org/x/") {
		return false
	}
	return false
}

func fileDepsToStarlark(fd FileDeps) starlark.Value {
	var imports []starlark.Value
	for _, imp := range fd.Imports {
		imports = append(imports, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"path":  starlark.String(imp.Path),
			"alias": starlark.String(imp.Alias),
			"line":  starlark.MakeInt(imp.Line),
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":          starlark.String(fd.Path),
		"package":       starlark.String(fd.Package),
		"imports":       starlark.NewList(imports),
		"internal_deps": stringsToStarlarkList(fd.InternalDeps),
		"external_deps": stringsToStarlarkList(fd.ExternalDeps),
		"stdlib_deps":   stringsToStarlarkList(fd.StdlibDeps),
	})
}

// =============================================================================
// HELPERS
// =============================================================================

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

func stringsToStarlarkList(ss []string) starlark.Value {
	var list []starlark.Value
	for _, s := range ss {
		list = append(list, starlark.String(s))
	}
	return starlark.NewList(list)
}

func mapKeysToStarlarkList(m map[string]bool) starlark.Value {
	var list []starlark.Value
	for k := range m {
		list = append(list, starlark.String(k))
	}
	return starlark.NewList(list)
}

// =============================================================================
// AST QUERY PRIMITIVES
// =============================================================================

// goStructs returns struct definitions from Go source files.
func (r *GoReceiver) goStructs(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.structs", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.structs: %w", err)
	}

	var result []starlark.Value
	for _, file := range files {
		fset, node, err := r.parseFile(file)
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
				var fields []starlark.Value
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
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
					fields = append(fields, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
						"name":        starlark.String(field.Names[0].Name),
						"json_name":   starlark.String(jsonName),
						"type":        starlark.String(typeToString(field.Type)),
						"required":    starlark.Bool(required),
						"description": starlark.String(desc),
					}))
				}
				result = append(result, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
					"name":   starlark.String(ts.Name.Name),
					"file":   starlark.String(filepath.Base(file)),
					"line":   starlark.MakeInt(fset.Position(ts.Pos()).Line),
					"fields": starlark.NewList(fields),
				}))
			}
			return true
		})
	}
	return starlark.NewList(result), nil
}

// constGroups returns typed const groups from Go source files.
func (r *GoReceiver) constGroups(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var typeFilter starlark.Value
	if err := starlark.UnpackArgs("go.const_groups", args, kwargs, "path", &path, "type?", &typeFilter); err != nil {
		return nil, err
	}
	typeF := optionalString(typeFilter)

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.const_groups: %w", err)
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
		fset, node, err := r.parseFile(file)
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
							if typeF == "" || typeF == currentType {
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
				for i, name := range vs.Names {
					var value string
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok {
							value = strings.Trim(lit.Value, `"`)
						}
					}
					currentConsts = append(currentConsts, constEntry{
						name:  name.Name,
						value: value,
						line:  fset.Position(name.Pos()).Line,
					})
				}
			}
			if currentType != "" && len(currentConsts) > 0 {
				if typeF == "" || typeF == currentType {
					groups = append(groups, group{typeName: currentType, file: filepath.Base(file), consts: currentConsts})
				}
			}
			return true
		})
	}

	var result []starlark.Value
	for _, g := range groups {
		var constsList []starlark.Value
		for _, c := range g.consts {
			constsList = append(constsList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"name":  starlark.String(c.name),
				"value": starlark.String(c.value),
				"line":  starlark.MakeInt(c.line),
			}))
		}
		result = append(result, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"type_name": starlark.String(g.typeName),
			"file":      starlark.String(g.file),
			"constants": starlark.NewList(constsList),
		}))
	}
	return starlark.NewList(result), nil
}

// goMethods returns method declarations from Go source files.
func (r *GoReceiver) goMethods(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var nameFilter, recvTypeFilter, returnsFilter starlark.Value
	if err := starlark.UnpackArgs("go.methods", args, kwargs,
		"path", &path, "name?", &nameFilter, "receiver_type?", &recvTypeFilter, "returns?", &returnsFilter); err != nil {
		return nil, err
	}
	nameF := optionalString(nameFilter)
	recvF := optionalString(recvTypeFilter)
	retF := optionalString(returnsFilter)

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.methods: %w", err)
	}

	var result []starlark.Value
	for _, file := range files {
		fset, node, err := r.parseFile(file)
		if err != nil {
			continue
		}
		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Name == nil {
				continue
			}
			if nameF != "" && fn.Name.Name != nameF {
				continue
			}
			recvType := receiverTypeName(fn.Recv.List[0].Type)
			if recvF != "" {
				if strings.HasPrefix(recvF, "*") {
					if recvType != recvF {
						continue
					}
				} else {
					if strings.TrimPrefix(recvType, "*") != recvF {
						continue
					}
				}
			}
			returns := returnTypeString(fn.Type.Results)
			if retF != "" && returns != retF {
				continue
			}
			doc := ""
			if fn.Doc != nil {
				doc = strings.TrimSpace(fn.Doc.Text())
			}
			scope := encodeScope(file, strings.TrimPrefix(recvType, "*")+"."+fn.Name.Name)
			result = append(result, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"name":          starlark.String(fn.Name.Name),
				"receiver_type": starlark.String(recvType),
				"returns":       starlark.String(returns),
				"params":        extractParams(fn.Type.Params),
				"file":          starlark.String(filepath.Base(file)),
				"line":          starlark.MakeInt(fset.Position(fn.Pos()).Line),
				"doc":           starlark.String(doc),
				"scope":         starlark.String(scope),
			}))
		}
	}
	return starlark.NewList(result), nil
}

// goFuncs returns function declarations (non-method) from Go source files.
func (r *GoReceiver) goFuncs(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var nameFilter starlark.Value
	if err := starlark.UnpackArgs("go.funcs", args, kwargs, "path", &path, "name?", &nameFilter); err != nil {
		return nil, err
	}
	nameF := optionalString(nameFilter)

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.funcs: %w", err)
	}

	var result []starlark.Value
	for _, file := range files {
		fset, node, err := r.parseFile(file)
		if err != nil {
			continue
		}
		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil {
				continue
			}
			if nameF != "" && fn.Name.Name != nameF {
				continue
			}
			returns := returnTypeString(fn.Type.Results)
			doc := ""
			if fn.Doc != nil {
				doc = strings.TrimSpace(fn.Doc.Text())
			}
			scope := encodeScope(file, fn.Name.Name)
			result = append(result, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"name":    starlark.String(fn.Name.Name),
				"returns": starlark.String(returns),
				"params":  extractParams(fn.Type.Params),
				"file":    starlark.String(filepath.Base(file)),
				"line":    starlark.MakeInt(fset.Position(fn.Pos()).Line),
				"doc":     starlark.String(doc),
				"scope":   starlark.String(scope),
			}))
		}
	}
	return starlark.NewList(result), nil
}

// goCalls returns function/method calls within a scope.
func (r *GoReceiver) goCalls(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var scope string
	var nameFilter starlark.Value
	if err := starlark.UnpackArgs("go.calls", args, kwargs, "scope", &scope, "name?", &nameFilter); err != nil {
		return nil, err
	}
	nameF := optionalString(nameFilter)

	fset, body, err := r.findScopeBody(scope)
	if err != nil {
		return nil, fmt.Errorf("go.calls: %w", err)
	}

	var result []starlark.Value
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
		if nameF != "" && funcName != nameF {
			return true
		}
		var argsList []starlark.Value
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
			argsList = append(argsList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"position":     starlark.MakeInt(i),
				"string_value": starlark.String(strVal),
				"ident_name":   starlark.String(identName),
			}))
		}
		result = append(result, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":      starlark.String(funcName),
			"qualifier": starlark.String(qualifier),
			"full_name": starlark.String(fullName),
			"line":      starlark.MakeInt(fset.Position(call.Pos()).Line),
			"args":      starlark.NewList(argsList),
		}))
		return true
	})
	return starlark.NewList(result), nil
}

// goComposites returns composite literals within a scope.
func (r *GoReceiver) goComposites(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var scope string
	var typeFilter starlark.Value
	if err := starlark.UnpackArgs("go.composites", args, kwargs, "scope", &scope, "type?", &typeFilter); err != nil {
		return nil, err
	}
	typeF := optionalString(typeFilter)

	fset, body, err := r.findScopeBody(scope)
	if err != nil {
		return nil, fmt.Errorf("go.composites: %w", err)
	}

	var result []starlark.Value
	ast.Inspect(body, func(n ast.Node) bool {
		comp, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		typeName := ""
		if comp.Type != nil {
			typeName = typeToString(comp.Type)
		}
		if typeF != "" && typeName != typeF {
			return true
		}
		fields := starlark.StringDict{}
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
					fields[key.Name] = starlark.String(strings.Trim(v.Value, `"`))
				} else {
					fields[key.Name] = starlark.String(v.Value)
				}
			case *ast.CompositeLit:
				var elems []starlark.Value
				for _, elem := range v.Elts {
					if lit, ok := elem.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						elems = append(elems, starlark.String(strings.Trim(lit.Value, `"`)))
					} else {
						elems = append(elems, starlark.String(typeToString(elem)))
					}
				}
				fields[key.Name] = starlark.NewList(elems)
			default:
				fields[key.Name] = starlark.String(typeToString(kv.Value))
			}
		}
		result = append(result, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"type_name": starlark.String(typeName),
			"line":      starlark.MakeInt(fset.Position(comp.Pos()).Line),
			"fields":    starlarkstruct.FromStringDict(starlarkstruct.Default, fields),
		}))
		return true
	})
	return starlark.NewList(result), nil
}

// goReturnString extracts the string literal from a return statement in a scope.
func (r *GoReceiver) goReturnString(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var scope string
	if err := starlark.UnpackArgs("go.return_string", args, kwargs, "scope", &scope); err != nil {
		return nil, err
	}
	_, body, err := r.findScopeBody(scope)
	if err != nil {
		return nil, fmt.Errorf("go.return_string: %w", err)
	}
	return starlark.String(extractReturnString(body)), nil
}

// goRawString extracts the first backtick string literal from a scope.
func (r *GoReceiver) goRawString(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var scope string
	if err := starlark.UnpackArgs("go.raw_string", args, kwargs, "scope", &scope); err != nil {
		return nil, err
	}
	_, body, err := r.findScopeBody(scope)
	if err != nil {
		return nil, fmt.Errorf("go.raw_string: %w", err)
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
	return starlark.String(rawStr), nil
}

// goTypeDoc returns the doc comment for a named type declaration.
func (r *GoReceiver) goTypeDoc(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var nameVal starlark.Value
	if err := starlark.UnpackArgs("go.type_doc", args, kwargs, "path", &path, "name?", &nameVal); err != nil {
		return nil, err
	}
	name := "Provider"
	if s := optionalString(nameVal); s != "" {
		name = s
	}

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.type_doc: %w", err)
	}

	for _, file := range files {
		_, node, err := r.parseFile(file)
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
				// Doc comment can live on the TypeSpec (grouped) or the GenDecl (standalone).
				// We use commentGroupRaw instead of Text() because Text() strips
				// directive-style comments (//tool:directive with no space after //).
				var cg *ast.CommentGroup
				if ts.Doc != nil {
					cg = ts.Doc
				} else if genDecl.Doc != nil {
					cg = genDecl.Doc
				}
				return starlark.String(commentGroupRaw(cg)), nil
			}
		}
	}

	return starlark.String(""), nil
}

// =============================================================================
// SHARED HELPERS
// =============================================================================

// commentGroupRaw returns the full text of a comment group, preserving directive
// lines (//tool:directive) that ast.CommentGroup.Text() strips since Go 1.21.
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

// extractParams converts a function's parameter list to a Starlark list of param structs.
func extractParams(params *ast.FieldList) starlark.Value {
	var result []starlark.Value
	if params == nil {
		return starlark.NewList(result)
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
			result = append(result, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"name":     starlark.String(""),
				"type":     starlark.String(typeStr),
				"variadic": starlark.Bool(variadic),
			}))
		} else {
			for _, name := range field.Names {
				result = append(result, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
					"name":     starlark.String(name.Name),
					"type":     starlark.String(typeStr),
					"variadic": starlark.Bool(variadic),
				}))
			}
		}
	}
	return starlark.NewList(result)
}

func parseJSONTag(tag string) (name string, required bool) {
	jsonRe := regexp.MustCompile(`json:"([^"]*)"`)
	match := jsonRe.FindStringSubmatch(tag)
	if match == nil {
		return "", false
	}

	parts := strings.Split(match[1], ",")
	name = parts[0]
	required = true

	for _, part := range parts[1:] {
		if part == "omitempty" {
			required = false
		}
	}

	return name, required
}

