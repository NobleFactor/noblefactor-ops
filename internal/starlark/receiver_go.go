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

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// GoReceiver provides Go source parsing operations.
// Implements starlark.Value and starlark.HasAttrs.
type GoReceiver struct {
	BaseReceiver
}

// NewGoReceiver creates a new GoReceiver.
func NewGoReceiver() *GoReceiver {
	return &GoReceiver{BaseReceiver: NewBaseReceiver("go")}
}

// Attr implements starlark.HasAttrs.
func (r *GoReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "parse_starlark_bindings":
		return MakeAttr("go.parse_starlark_bindings", r.parseStarlarkBindings), nil
	case "parse_migrate_knowledge":
		return MakeAttr("go.parse_migrate_knowledge", r.parseMigrateKnowledge), nil
	case "parse_execution_ops":
		return MakeAttr("go.parse_execution_ops", r.parseExecutionOps), nil
	case "parse_execution_schema":
		return MakeAttr("go.parse_execution_schema", r.parseExecutionSchema), nil
	case "metrics":
		return MakeAttr("go.metrics", r.metrics), nil
	case "deps":
		return MakeAttr("go.deps", r.deps), nil
	default:
		return nil, NoSuchAttrError("go", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *GoReceiver) AttrNames() []string {
	return []string{"deps", "metrics", "parse_execution_ops", "parse_execution_schema", "parse_migrate_knowledge", "parse_starlark_bindings"}
}

// =============================================================================
// STARLARK BINDINGS PARSING
// =============================================================================

// Binding represents a discovered Starlark binding.
type Binding struct {
	Namespace string // Parent namespace (e.g., "plan.package")
	Name      string // Method name (e.g., "install")
	FullName  string // Full binding name (e.g., "plan.package.install")
	Line      int    // Line number in source file
	Handler   string // Handler function name (e.g., "packageInstallBuiltin")
	Mutates   bool   // True if the handler creates execution.Node or modifies graph
}

// Namespace represents a discovered Starlark namespace.
type Namespace struct {
	Name   string // Namespace name (e.g., "plan", "system.platform")
	Parent string // Parent namespace if nested
	Line   int    // Line number in source file
}

// parseStarlarkBindings parses Go source files and extracts Starlark binding definitions.
func (r *GoReceiver) parseStarlarkBindings(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.parse_starlark_bindings", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("go.parse_starlark_bindings: %w", err)
	}

	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("go.parse_starlark_bindings: reading dir: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				files = append(files, filepath.Join(path, entry.Name()))
			}
		}
	} else {
		files = []string{path}
	}

	var allBindings []starlark.Value
	var allNamespaces []starlark.Value
	seenBindings := make(map[string]bool)
	seenNamespaces := make(map[string]bool)

	for _, file := range files {
		bindings, namespaces, err := parseGoFile(file)
		if err != nil {
			return nil, fmt.Errorf("go.parse_starlark_bindings: parsing %s: %w", file, err)
		}

		for _, b := range bindings {
			key := b.FullName
			if !seenBindings[key] {
				seenBindings[key] = true
				allBindings = append(allBindings, bindingToStarlark(b, file))
			}
		}

		for _, ns := range namespaces {
			if !seenNamespaces[ns.Name] {
				seenNamespaces[ns.Name] = true
				allNamespaces = append(allNamespaces, namespaceToStarlark(ns, file))
			}
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"bindings":   starlark.NewList(allBindings),
		"namespaces": starlark.NewList(allNamespaces),
	}), nil
}

// parseGoFile parses a single Go file and extracts binding information.
func parseGoFile(path string) ([]Binding, []Namespace, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	contentStr := string(content)

	mutatingFuncs := findMutatingFunctions(node, fset, contentStr)
	v := newBindingVisitor(fset, contentStr, mutatingFuncs)
	ast.Walk(v, node)
	v.inferNamespaces()

	return v.bindings, v.namespaces, nil
}

// bindingVisitor implements ast.Visitor to extract Starlark bindings.
type bindingVisitor struct {
	fset             *token.FileSet
	lines            []string
	mutatingFuncs    map[string]bool
	currentNamespace string
	newBuiltinRe     *regexp.Regexp
	fromStringDictRe *regexp.Regexp
	bindings         []Binding
	namespaces       []Namespace
}

func newBindingVisitor(fset *token.FileSet, content string, mutatingFuncs map[string]bool) *bindingVisitor {
	return &bindingVisitor{
		fset:             fset,
		lines:            strings.Split(content, "\n"),
		mutatingFuncs:    mutatingFuncs,
		newBuiltinRe:     regexp.MustCompile(`NewBuiltin\s*\(\s*"([^"]+)"\s*,\s*(?:(\w+\.)?(\w+)\s*\)|func\s*\()`),
		fromStringDictRe: regexp.MustCompile(`FromStringDict\s*\(\s*starlark\.String\s*\(\s*"([^"]+)"`),
	}
}

func (v *bindingVisitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}
	switch x := n.(type) {
	case *ast.FuncDecl:
		v.visitFuncDecl(x)
	case *ast.CallExpr:
		v.visitCallExpr(x)
	}
	return v
}

func (v *bindingVisitor) visitFuncDecl(fn *ast.FuncDecl) {
	if fn.Name == nil {
		return
	}
	name := fn.Name.Name
	if strings.HasSuffix(name, "Struct") || strings.HasSuffix(name, "Module") {
		baseName := strings.TrimSuffix(strings.TrimSuffix(name, "Struct"), "Module")
		v.currentNamespace = camelToSnake(baseName)
	}
}

func (v *bindingVisitor) visitCallExpr(call *ast.CallExpr) {
	line := v.fset.Position(call.Pos()).Line
	if line <= 0 || line > len(v.lines) {
		return
	}
	lineContent := v.lines[line-1]

	v.extractBinding(lineContent, line)
	v.extractNamespace(lineContent, line)
}

func (v *bindingVisitor) extractBinding(lineContent string, line int) {
	matches := v.newBuiltinRe.FindStringSubmatch(lineContent)
	if len(matches) <= 1 {
		return
	}

	bindingName := matches[1]
	handlerName := ""
	if len(matches) > 3 && matches[3] != "" {
		handlerName = matches[3]
	}

	isInlineFunc := strings.Contains(lineContent, ", func(")

	binding := Binding{
		Name:     extractMethodName(bindingName),
		FullName: bindingName,
		Line:     line,
		Handler:  handlerName,
		Mutates:  !isInlineFunc && v.mutatingFuncs[handlerName],
	}

	if idx := strings.LastIndex(bindingName, "."); idx > 0 {
		binding.Namespace = bindingName[:idx]
	}

	v.bindings = append(v.bindings, binding)
}

func (v *bindingVisitor) extractNamespace(lineContent string, line int) {
	matches := v.fromStringDictRe.FindStringSubmatch(lineContent)
	if len(matches) <= 1 {
		return
	}

	nsName := matches[1]
	ns := Namespace{
		Name: nsName,
		Line: line,
	}

	if v.currentNamespace != "" && v.currentNamespace != nsName {
		ns.Parent = v.currentNamespace
		ns.Name = v.currentNamespace + "." + nsName
	}

	v.namespaces = append(v.namespaces, ns)
}

func (v *bindingVisitor) inferNamespaces() {
	seen := make(map[string]bool)
	for _, ns := range v.namespaces {
		seen[ns.Name] = true
	}

	for _, b := range v.bindings {
		if b.Namespace == "" {
			continue
		}
		parts := strings.Split(b.Namespace, ".")
		for i := range parts {
			ns := strings.Join(parts[:i+1], ".")
			if !seen[ns] {
				seen[ns] = true
				v.namespaces = append(v.namespaces, Namespace{Name: ns, Line: 0})
			}
		}
	}
}

func findMutatingFunctions(node *ast.File, fset *token.FileSet, content string) map[string]bool {
	mutating := make(map[string]bool)

	directMutationPatterns := []string{
		"execution.Node", "execution.Edge", "graph.Nodes", "graph.Edges",
		"b.graph.Nodes", "s.graph.Nodes", "&execution.Node",
	}

	interfaceMutationPatterns := []string{
		"s.PackageInstall", "s.PackageUpgrade", "s.PackageRemove", "s.PackageUpdate",
		"s.Configure", "s.Link", "s.Copy", "s.Mkdir", "s.Write", "s.Remove",
		"s.Download", "s.ArchiveExtract", "s.GitClone", "s.GitCheckout", "s.GitPull",
		"s.Service", "s.Shell", "s.DependsOn", ".bindings.",
	}

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}

		funcName := fn.Name.Name
		start := fset.Position(fn.Body.Pos()).Offset
		end := fset.Position(fn.Body.End()).Offset

		if start >= 0 && end <= len(content) {
			bodyContent := content[start:end]

			for _, pattern := range directMutationPatterns {
				if strings.Contains(bodyContent, pattern) {
					mutating[funcName] = true
					return true
				}
			}

			for _, pattern := range interfaceMutationPatterns {
				if strings.Contains(bodyContent, pattern) {
					mutating[funcName] = true
					return true
				}
			}
		}

		return true
	})

	return mutating
}

func extractMethodName(fullName string) string {
	if idx := strings.LastIndex(fullName, "."); idx >= 0 {
		return fullName[idx+1:]
	}
	return fullName
}

func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func bindingToStarlark(b Binding, file string) starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"namespace": starlark.String(b.Namespace),
		"name":      starlark.String(b.Name),
		"full_name": starlark.String(b.FullName),
		"file":      starlark.String(filepath.Base(file)),
		"line":      starlark.MakeInt(b.Line),
		"handler":   starlark.String(b.Handler),
		"mutates":   starlark.Bool(b.Mutates),
	})
}

func namespaceToStarlark(ns Namespace, file string) starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"name":   starlark.String(ns.Name),
		"parent": starlark.String(ns.Parent),
		"file":   starlark.String(filepath.Base(file)),
		"line":   starlark.MakeInt(ns.Line),
	})
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
// MIGRATION KNOWLEDGE PARSING
// =============================================================================

// MigrateKnowledge holds extracted migration knowledge from Go source.
type MigrateKnowledge struct {
	SourceSystems     []TypeConstant
	EncryptionSystems []TypeConstant
	RepoLayers        []TypeConstant
	Platforms         []string
	SystemPrompt      string
}

// TypeConstant represents a typed const declaration.
type TypeConstant struct {
	Name     string
	Value    string
	TypeName string
	Line     int
	File     string
}

func (r *GoReceiver) parseMigrateKnowledge(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.parse_migrate_knowledge", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("go.parse_migrate_knowledge: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("go.parse_migrate_knowledge: path must be a directory")
	}

	knowledge := &MigrateKnowledge{}

	analysisPath := filepath.Join(path, "analysis.go")
	if _, err := os.Stat(analysisPath); err == nil {
		if err := parseAnalysisFile(analysisPath, knowledge); err != nil {
			return nil, fmt.Errorf("go.parse_migrate_knowledge: parsing analysis.go: %w", err)
		}
	}

	planPath := filepath.Join(path, "plan.go")
	if _, err := os.Stat(planPath); err == nil {
		if err := parsePlanFile(planPath, knowledge); err != nil {
			return nil, fmt.Errorf("go.parse_migrate_knowledge: parsing plan.go: %w", err)
		}
	}

	return knowledgeToStarlark(knowledge), nil
}

func parseAnalysisFile(path string, knowledge *MigrateKnowledge) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	filename := filepath.Base(path)

	ast.Inspect(node, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			return true
		}

		var currentType string

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			if valueSpec.Type != nil {
				if ident, ok := valueSpec.Type.(*ast.Ident); ok {
					currentType = ident.Name
				}
			}

			for i, name := range valueSpec.Names {
				if i >= len(valueSpec.Values) {
					continue
				}

				basicLit, ok := valueSpec.Values[i].(*ast.BasicLit)
				if !ok || basicLit.Kind != token.STRING {
					continue
				}

				value := strings.Trim(basicLit.Value, `"`)
				line := fset.Position(name.Pos()).Line

				tc := TypeConstant{
					Name:     name.Name,
					Value:    value,
					TypeName: currentType,
					Line:     line,
					File:     filename,
				}

				switch currentType {
				case "SourceSystem":
					knowledge.SourceSystems = append(knowledge.SourceSystems, tc)
				case "EncryptionSystem":
					knowledge.EncryptionSystems = append(knowledge.EncryptionSystems, tc)
				case "RepoLayer":
					knowledge.RepoLayers = append(knowledge.RepoLayers, tc)
				}
			}
		}

		return true
	})

	return nil
}

func parsePlanFile(path string, knowledge *MigrateKnowledge) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	contentStr := string(content)

	promptStart := strings.Index(contentStr, "func buildSystemPrompt() string {")
	if promptStart == -1 {
		return nil
	}

	searchStart := promptStart
	backtickStart := strings.Index(contentStr[searchStart:], "`")
	if backtickStart == -1 {
		return nil
	}
	backtickStart += searchStart

	backtickEnd := strings.Index(contentStr[backtickStart+1:], "`")
	if backtickEnd == -1 {
		return nil
	}
	backtickEnd += backtickStart + 1

	knowledge.SystemPrompt = contentStr[backtickStart+1 : backtickEnd]

	lines := strings.Split(knowledge.SystemPrompt, "\n")
	inPlatformSection := false
	for i, line := range lines {
		if strings.Contains(line, "Known platforms:") {
			colonIdx := strings.Index(line, ":")
			if colonIdx != -1 {
				platformStr := strings.TrimSpace(line[colonIdx+1:])
				if platformStr != "" {
					platforms := strings.Split(platformStr, ",")
					for _, p := range platforms {
						p = strings.TrimSpace(p)
						if p != "" {
							knowledge.Platforms = append(knowledge.Platforms, p)
						}
					}
					break
				}
			}
			inPlatformSection = true
			continue
		}

		if inPlatformSection {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				entry := strings.TrimPrefix(trimmed, "- ")
				platform := entry
				if spaceIdx := strings.Index(entry, " "); spaceIdx > 0 {
					platform = entry[:spaceIdx]
				}
				if parenIdx := strings.Index(platform, "("); parenIdx > 0 {
					platform = platform[:parenIdx]
				}
				platform = strings.TrimSpace(platform)
				if platform != "" {
					knowledge.Platforms = append(knowledge.Platforms, platform)
				}
			} else if trimmed == "" || !strings.HasPrefix(trimmed, "-") {
				if i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "##") {
					inPlatformSection = false
				} else if trimmed != "" && !strings.HasPrefix(trimmed, "-") {
					inPlatformSection = false
				}
			}
		}
	}

	return nil
}

func knowledgeToStarlark(k *MigrateKnowledge) starlark.Value {
	var sourceSystems []starlark.Value
	for _, s := range k.SourceSystems {
		sourceSystems = append(sourceSystems, typeConstToStarlark(s))
	}

	var encryptionSystems []starlark.Value
	for _, e := range k.EncryptionSystems {
		encryptionSystems = append(encryptionSystems, typeConstToStarlark(e))
	}

	var repoLayers []starlark.Value
	for _, r := range k.RepoLayers {
		repoLayers = append(repoLayers, typeConstToStarlark(r))
	}

	var platforms []starlark.Value
	for _, p := range k.Platforms {
		platforms = append(platforms, starlark.String(p))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"source_systems":     starlark.NewList(sourceSystems),
		"encryption_systems": starlark.NewList(encryptionSystems),
		"repo_layers":        starlark.NewList(repoLayers),
		"platforms":          starlark.NewList(platforms),
		"system_prompt":      starlark.String(k.SystemPrompt),
	})
}

func typeConstToStarlark(tc TypeConstant) starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"name":      starlark.String(tc.Name),
		"value":     starlark.String(tc.Value),
		"type_name": starlark.String(tc.TypeName),
		"file":      starlark.String(tc.File),
		"line":      starlark.MakeInt(tc.Line),
	})
}

// =============================================================================
// EXECUTION OPERATIONS PARSING
// =============================================================================

// ExecutionOp represents an execution operation extracted from Go source.
type ExecutionOp struct {
	Name     string
	TypeName string
	Line     int
	File     string
}

func (r *GoReceiver) parseExecutionOps(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.parse_execution_ops", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("go.parse_execution_ops: %w", err)
	}

	var files []string
	if info.IsDir() {
		opsPath := filepath.Join(path, "ops.go")
		if _, err := os.Stat(opsPath); err == nil {
			files = append(files, opsPath)
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("go.parse_execution_ops: reading dir: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") &&
				!strings.HasSuffix(entry.Name(), "_test.go") &&
				entry.Name() != "ops.go" {
				files = append(files, filepath.Join(path, entry.Name()))
			}
		}
	} else {
		files = []string{path}
	}

	var ops []ExecutionOp
	seen := make(map[string]bool)

	for _, file := range files {
		fileOps, err := parseOpsFile(file)
		if err != nil {
			continue
		}
		for _, op := range fileOps {
			if !seen[op.Name] {
				seen[op.Name] = true
				ops = append(ops, op)
			}
		}
	}

	sortOps(ops)

	var opsList []starlark.Value
	for _, op := range ops {
		opsList = append(opsList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":      starlark.String(op.Name),
			"type_name": starlark.String(op.TypeName),
			"file":      starlark.String(op.File),
			"line":      starlark.MakeInt(op.Line),
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"operations": starlark.NewList(opsList),
		"count":      starlark.MakeInt(len(ops)),
	}), nil
}

func parseOpsFile(path string) ([]ExecutionOp, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(path)
	var ops []ExecutionOp

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return true
		}

		if fn.Name == nil || fn.Name.Name != "Name" {
			return true
		}

		recvType := ""
		switch t := fn.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				recvType = ident.Name
			}
		case *ast.Ident:
			recvType = t.Name
		}

		if recvType == "" || !strings.HasSuffix(recvType, "Op") {
			return true
		}

		if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
			return true
		}
		if ident, ok := fn.Type.Results.List[0].Type.(*ast.Ident); !ok || ident.Name != "string" {
			return true
		}

		opName := extractReturnString(fn.Body)
		if opName == "" {
			return true
		}

		ops = append(ops, ExecutionOp{
			Name:     opName,
			TypeName: recvType,
			Line:     fset.Position(fn.Pos()).Line,
			File:     filename,
		})

		return true
	})

	return ops, nil
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

func sortOps(ops []ExecutionOp) {
	for i := 0; i < len(ops)-1; i++ {
		for j := i + 1; j < len(ops); j++ {
			if ops[i].Name > ops[j].Name {
				ops[i], ops[j] = ops[j], ops[i]
			}
		}
	}
}

// =============================================================================
// EXECUTION SCHEMA PARSING
// =============================================================================

// StructField represents a field extracted from a Go struct.
type StructField struct {
	Name        string
	JSONName    string
	Type        string
	Required    bool
	Description string
}

// StructDef represents a Go struct definition.
type StructDef struct {
	Name   string
	Fields []StructField
}

// ConstValue represents a const value extracted from Go.
type ConstValue struct {
	Name  string
	Value string
}

func (r *GoReceiver) parseExecutionSchema(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.parse_execution_schema", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	graphPath := filepath.Join(path, "graph.go")
	structs, consts, err := parseStructDefs(graphPath)
	if err != nil {
		return nil, fmt.Errorf("go.parse_execution_schema: parsing graph.go: %w", err)
	}

	var ops []ExecutionOp
	seen := make(map[string]bool)

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("go.parse_execution_schema: reading dir: %w", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "ops") && strings.HasSuffix(entry.Name(), ".go") &&
			!strings.HasSuffix(entry.Name(), "_test.go") {
			fileOps, err := parseOpsFile(filepath.Join(path, entry.Name()))
			if err != nil {
				continue
			}
			for _, op := range fileOps {
				if !seen[op.Name] {
					seen[op.Name] = true
					ops = append(ops, op)
				}
			}
		}
	}
	sortOps(ops)

	result := starlark.StringDict{}

	for name, def := range structs {
		result[toSnakeCase(name)] = structDefToStarlark(def)
	}

	for typeName, values := range consts {
		var enumList []starlark.Value
		for _, v := range values {
			enumList = append(enumList, starlark.String(v.Value))
		}
		result[toSnakeCase(typeName)+"s"] = starlark.NewList(enumList)
	}

	var opsList []starlark.Value
	for _, op := range ops {
		opsList = append(opsList, starlark.String(op.Name))
	}
	result["operations"] = starlark.NewList(opsList)

	return starlarkstruct.FromStringDict(starlarkstruct.Default, result), nil
}

func parseStructDefs(path string) (structs map[string]StructDef, consts map[string][]ConstValue, err error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	structs = make(map[string]StructDef)
	consts = make(map[string][]ConstValue)

	var currentConstType string

	ast.Inspect(node, func(n ast.Node) bool {
		x, ok := n.(*ast.GenDecl)
		if ok {
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					structType, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					def := StructDef{Name: typeSpec.Name.Name}
					for _, field := range structType.Fields.List {
						if len(field.Names) == 0 {
							continue
						}

						sf := StructField{
							Name: field.Names[0].Name,
							Type: typeToString(field.Type),
						}

						if field.Tag != nil {
							tag := strings.Trim(field.Tag.Value, "`")
							sf.JSONName, sf.Required = parseJSONTag(tag)
						}

						if field.Comment != nil {
							sf.Description = strings.TrimSpace(field.Comment.Text())
						} else if field.Doc != nil {
							sf.Description = strings.TrimSpace(field.Doc.Text())
						}

						if sf.JSONName == "-" {
							continue
						}

						def.Fields = append(def.Fields, sf)
					}

					structs[def.Name] = def
				}
			} else if x.Tok == token.CONST {
				for _, spec := range x.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					if valueSpec.Type != nil {
						if ident, ok := valueSpec.Type.(*ast.Ident); ok {
							currentConstType = ident.Name
						}
					}

					for i, name := range valueSpec.Names {
						if currentConstType == "" {
							continue
						}

						var value string
						if i < len(valueSpec.Values) {
							if lit, ok := valueSpec.Values[i].(*ast.BasicLit); ok {
								value = strings.Trim(lit.Value, `"`)
							}
						}

						if value != "" {
							consts[currentConstType] = append(consts[currentConstType], ConstValue{
								Name:  name.Name,
								Value: value,
							})
						}
					}
				}
			}
		}
		return true
	})

	return structs, consts, nil
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
	default:
		return "unknown"
	}
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

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func structDefToStarlark(def StructDef) starlark.Value {
	var fieldsList []starlark.Value
	for _, f := range def.Fields {
		fieldsList = append(fieldsList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":        starlark.String(f.Name),
			"json_name":   starlark.String(f.JSONName),
			"type":        starlark.String(f.Type),
			"required":    starlark.Bool(f.Required),
			"description": starlark.String(f.Description),
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"name":   starlark.String(def.Name),
		"fields": starlark.NewList(fieldsList),
	})
}
