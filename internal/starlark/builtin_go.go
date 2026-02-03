// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

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

// goModule returns the go module with Go source parsing operations.
func goModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "go",
		Members: starlark.StringDict{
			"parse_starlark_bindings": starlark.NewBuiltin("go.parse_starlark_bindings", goParseStarlarkBindings),
			"parse_migrate_knowledge": starlark.NewBuiltin("go.parse_migrate_knowledge", goParseMigrateKnowledge),
			"parse_execution_ops":     starlark.NewBuiltin("go.parse_execution_ops", goParseExecutionOps),
			"parse_execution_schema":  starlark.NewBuiltin("go.parse_execution_schema", goParseExecutionSchema),
			"parse_devlore_api":       starlark.NewBuiltin("go.parse_devlore_api", goParseDevloreAPI),
			"metrics":                 starlark.NewBuiltin("go.metrics", goMetrics),
			"deps":                    starlark.NewBuiltin("go.deps", goDeps),
		},
	}
}

// goParseStarlarkBindings parses Go source files and extracts Starlark binding definitions.
// It finds NewBuiltin() and FromStringDict() calls to build a binding tree.
//
// Args:
//   - path: Path to a Go file or directory containing Go files
//
// Returns:
//   - A struct with:
//     - bindings: List of binding structs with {namespace, name, full_name, file, line}
//     - namespaces: List of namespace structs with {name, file, line}
func goParseStarlarkBindings(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		// Collect all .go files in directory
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

// parseGoFile parses a single Go file and extracts binding information.
func parseGoFile(path string) ([]Binding, []Namespace, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	// Read file content for regex matching (AST doesn't preserve raw string positions well)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	contentStr := string(content)

	// Find all functions that mutate (create execution.Node or modify graph)
	mutatingFuncs := findMutatingFunctions(node, fset, contentStr)

	// Create and run the visitor
	v := newBindingVisitor(fset, contentStr, mutatingFuncs)
	ast.Walk(v, node)

	// Infer additional namespaces from binding names
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

// newBindingVisitor creates a new binding visitor.
func newBindingVisitor(fset *token.FileSet, content string, mutatingFuncs map[string]bool) *bindingVisitor {
	return &bindingVisitor{
		fset:          fset,
		lines:         strings.Split(content, "\n"),
		mutatingFuncs: mutatingFuncs,
		// Pattern 1: NewBuiltin("name", handlerFunc) - named handler
		// Pattern 2: NewBuiltin("name", func(...) - inline anonymous function
		newBuiltinRe:     regexp.MustCompile(`NewBuiltin\s*\(\s*"([^"]+)"\s*,\s*(?:(\w+\.)?(\w+)\s*\)|func\s*\()`),
		fromStringDictRe: regexp.MustCompile(`FromStringDict\s*\(\s*starlark\.String\s*\(\s*"([^"]+)"`),
	}
}

// Visit implements ast.Visitor interface.
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

// visitFuncDecl tracks function context for namespace resolution.
func (v *bindingVisitor) visitFuncDecl(fn *ast.FuncDecl) {
	if fn.Name == nil {
		return
	}
	name := fn.Name.Name
	// Functions like platformStruct(), packageStruct() create sub-namespaces
	if strings.HasSuffix(name, "Struct") || strings.HasSuffix(name, "Module") {
		baseName := strings.TrimSuffix(strings.TrimSuffix(name, "Struct"), "Module")
		v.currentNamespace = camelToSnake(baseName)
	}
}

// visitCallExpr extracts NewBuiltin and FromStringDict calls.
func (v *bindingVisitor) visitCallExpr(call *ast.CallExpr) {
	line := v.fset.Position(call.Pos()).Line
	if line <= 0 || line > len(v.lines) {
		return
	}
	lineContent := v.lines[line-1]

	v.extractBinding(lineContent, line)
	v.extractNamespace(lineContent, line)
}

// extractBinding extracts a binding from a NewBuiltin call.
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
		// Inline functions are assumed non-mutating (system.* queries)
		// Named handlers check the mutatingFuncs map
		Mutates: !isInlineFunc && v.mutatingFuncs[handlerName],
	}

	// Determine namespace from full name
	if idx := strings.LastIndex(bindingName, "."); idx > 0 {
		binding.Namespace = bindingName[:idx]
	}

	v.bindings = append(v.bindings, binding)
}

// extractNamespace extracts a namespace from a FromStringDict call.
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

	// Determine parent namespace
	if v.currentNamespace != "" && v.currentNamespace != nsName {
		ns.Parent = v.currentNamespace
		ns.Name = v.currentNamespace + "." + nsName
	}

	v.namespaces = append(v.namespaces, ns)
}

// inferNamespaces adds namespaces inferred from binding names.
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

// findMutatingFunctions finds all functions that create execution.Node or modify graph.
// Returns a map of function name -> true if it mutates.
func findMutatingFunctions(node *ast.File, fset *token.FileSet, content string) map[string]bool {
	mutating := make(map[string]bool)

	// Patterns that indicate direct mutation (creating execution nodes or edges)
	directMutationPatterns := []string{
		"execution.Node",
		"execution.Edge",
		"graph.Nodes",
		"graph.Edges",
		"b.graph.Nodes",
		"s.graph.Nodes",
		"&execution.Node",
	}

	// Patterns that indicate indirect mutation (calling PlanBindings interface methods)
	// These interface methods are implemented in platform/*.go and create execution nodes
	// The methods are called on the receiver `s` which embeds PlanBindings
	interfaceMutationPatterns := []string{
		// Package operations
		"s.PackageInstall",
		"s.PackageUpgrade",
		"s.PackageRemove",
		"s.PackageUpdate",
		// File operations
		"s.Configure",
		"s.Link",
		"s.Copy",
		"s.Mkdir",
		"s.Write",
		"s.Remove",
		// Download and archive
		"s.Download",
		"s.ArchiveExtract",
		// Git operations
		"s.GitClone",
		"s.GitCheckout",
		"s.GitPull",
		// Service and shell
		"s.Service",
		"s.Shell",
		"s.DependsOn",
		// Also check for .bindings. pattern (alternative structure)
		".bindings.",
	}

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}

		funcName := fn.Name.Name

		// Get the function body's source range
		start := fset.Position(fn.Body.Pos()).Offset
		end := fset.Position(fn.Body.End()).Offset

		if start >= 0 && end <= len(content) {
			bodyContent := content[start:end]

			// Check for direct mutation patterns
			for _, pattern := range directMutationPatterns {
				if strings.Contains(bodyContent, pattern) {
					mutating[funcName] = true
					return true
				}
			}

			// Check for interface mutation patterns
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

// extractMethodName gets the method name from a full binding name.
func extractMethodName(fullName string) string {
	if idx := strings.LastIndex(fullName, "."); idx >= 0 {
		return fullName[idx+1:]
	}
	return fullName
}

// camelToSnake converts CamelCase to snake_case.
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

// bindingToStarlark converts a Binding to a Starlark struct.
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

// namespaceToStarlark converts a Namespace to a Starlark struct.
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
	LOC           int // Total lines
	SLOC          int // Source lines (non-blank, non-comment)
	Comments      int // Comment lines
	Blanks        int // Blank lines
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

// goMetrics calculates code metrics for Go source.
//
// Args:
//   - path: Path to a Go file or directory
//
// Returns:
//   - A struct with file-level and aggregate metrics
func goMetrics(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

		// Accumulate totals
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

// analyzeFileMetrics calculates metrics for a single file.
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

	// Count lines
	lines := strings.Split(string(content), "\n")
	fm.LOC = len(lines)

	// Count blanks
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			fm.Blanks++
		}
	}

	// Count comments from AST
	for _, cg := range node.Comments {
		for _, c := range cg.List {
			fm.Comments += strings.Count(c.Text, "\n") + 1
		}
	}

	fm.SLOC = fm.LOC - fm.Blanks - fm.Comments
	if fm.SLOC < 0 {
		fm.SLOC = 0
	}

	// Count imports
	fm.Imports = len(node.Imports)

	// Count declarations
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Recv != nil {
				fm.Methods++
			} else {
				fm.Functions++
			}
			// Check for test functions
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

// fileMetricsToStarlark converts FileMetrics to a Starlark struct.
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
	InternalDeps []string // Same module imports
	ExternalDeps []string // Third-party imports
	StdlibDeps   []string // Standard library imports
}

// ImportInfo holds information about a single import.
type ImportInfo struct {
	Path  string
	Alias string
	Line  int
}

// goDeps analyzes import dependencies for Go source.
//
// Args:
//   - path: Path to a Go file or directory
//
// Returns:
//   - A struct with dependency information
func goDeps(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.deps", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.deps: %w", err)
	}

	// Try to detect module path from go.mod
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

// analyzeFileDeps analyzes dependencies for a single file.
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

		// Classify import
		if isStdlib(importPath) {
			fd.StdlibDeps = append(fd.StdlibDeps, importPath)
		} else if modulePath != "" && strings.HasPrefix(importPath, modulePath) {
			fd.InternalDeps = append(fd.InternalDeps, importPath)
		} else {
			fd.ExternalDeps = append(fd.ExternalDeps, importPath)
		}
	}

	return fd, nil
}

// detectModulePath tries to find the Go module path from go.mod.
func detectModulePath(startPath string) string {
	dir := startPath
	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	// Walk up looking for go.mod
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

// isStdlib checks if an import path is a standard library package.
func isStdlib(importPath string) bool {
	// Standard library packages don't contain dots (except internal packages)
	if !strings.Contains(importPath, ".") {
		return true
	}
	// Also check for golang.org/x which are quasi-stdlib
	if strings.HasPrefix(importPath, "golang.org/x/") {
		return false // Treat as external, they need to be imported
	}
	return false
}

// fileDepsToStarlark converts FileDeps to a Starlark struct.
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

// collectGoFiles returns all Go files in a path (file or directory).
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
		// Skip vendor and testdata directories
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

// stringsToStarlarkList converts a string slice to a Starlark list.
func stringsToStarlarkList(ss []string) starlark.Value {
	var list []starlark.Value
	for _, s := range ss {
		list = append(list, starlark.String(s))
	}
	return starlark.NewList(list)
}

// mapKeysToStarlarkList converts map keys to a Starlark list.
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
	Name     string // Constant name (e.g., "SystemTuckr")
	Value    string // String value (e.g., "tuckr")
	TypeName string // Type name (e.g., "SourceSystem")
	Line     int
	File     string
}

// goParseMigrateKnowledge parses Go source files in the writ/migrate directory
// and extracts migration knowledge: system types, encryption types, platforms.
//
// Args:
//   - path: Path to the migrate directory (e.g., "internal/writ/migrate")
//
// Returns:
//   - A struct with:
//   - source_systems: List of {name, value, type, file, line}
//   - encryption_systems: List of {name, value, type, file, line}
//   - repo_layers: List of {name, value, type, file, line}
//   - platforms: List of platform name strings
//   - system_prompt: The raw system prompt text from plan.go
func goParseMigrateKnowledge(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Parse analysis.go for type constants
	analysisPath := filepath.Join(path, "analysis.go")
	if _, err := os.Stat(analysisPath); err == nil {
		if err := parseAnalysisFile(analysisPath, knowledge); err != nil {
			return nil, fmt.Errorf("go.parse_migrate_knowledge: parsing analysis.go: %w", err)
		}
	}

	// Parse plan.go for system prompt and platforms
	planPath := filepath.Join(path, "plan.go")
	if _, err := os.Stat(planPath); err == nil {
		if err := parsePlanFile(planPath, knowledge); err != nil {
			return nil, fmt.Errorf("go.parse_migrate_knowledge: parsing plan.go: %w", err)
		}
	}

	return knowledgeToStarlark(knowledge), nil
}

// parseAnalysisFile extracts typed constants from analysis.go.
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

		// Track the type for this const block
		var currentType string

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			// Check for typed const (e.g., SystemTuckr SourceSystem = "tuckr")
			if valueSpec.Type != nil {
				if ident, ok := valueSpec.Type.(*ast.Ident); ok {
					currentType = ident.Name
				}
			}

			// Extract const name and value
			for i, name := range valueSpec.Names {
				if i >= len(valueSpec.Values) {
					continue
				}

				// Get string value from basic lit
				basicLit, ok := valueSpec.Values[i].(*ast.BasicLit)
				if !ok || basicLit.Kind != token.STRING {
					continue
				}

				// Unquote the string value
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

// parsePlanFile extracts the system prompt and platform list from plan.go.
func parsePlanFile(path string, knowledge *MigrateKnowledge) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	contentStr := string(content)

	// Find buildSystemPrompt function and extract the prompt string
	// Look for the return statement with the raw string literal
	promptStart := strings.Index(contentStr, "func buildSystemPrompt() string {")
	if promptStart == -1 {
		return nil // Function not found, not an error
	}

	// Find the backtick-quoted string
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

	// Extract platforms from the prompt
	// Handles both formats:
	//   Single line: "Known platforms: Darwin, Linux, Windows"
	//   Multi-line:  "Known platforms:\n  - Darwin (macOS)\n  - Linux\n..."
	lines := strings.Split(knowledge.SystemPrompt, "\n")
	inPlatformSection := false
	for i, line := range lines {
		if strings.Contains(line, "Known platforms:") {
			// Check if platforms are on the same line (comma-separated)
			colonIdx := strings.Index(line, ":")
			if colonIdx != -1 {
				platformStr := strings.TrimSpace(line[colonIdx+1:])
				if platformStr != "" {
					// Single-line format: comma-separated
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
			// Multi-line format: start collecting from subsequent lines
			inPlatformSection = true
			continue
		}

		if inPlatformSection {
			trimmed := strings.TrimSpace(line)
			// Check for bullet point format "- Platform (description)"
			if strings.HasPrefix(trimmed, "- ") {
				// Extract platform name (first word after "- ")
				entry := strings.TrimPrefix(trimmed, "- ")
				// Platform name is everything before the first space or parenthesis
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
				// End of platform section (empty line or non-bullet line)
				// But skip if next line might continue (check for ## header)
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

// knowledgeToStarlark converts MigrateKnowledge to a Starlark struct.
func knowledgeToStarlark(k *MigrateKnowledge) starlark.Value {
	// Convert source systems
	var sourceSystems []starlark.Value
	for _, s := range k.SourceSystems {
		sourceSystems = append(sourceSystems, typeConstToStarlark(s))
	}

	// Convert encryption systems
	var encryptionSystems []starlark.Value
	for _, e := range k.EncryptionSystems {
		encryptionSystems = append(encryptionSystems, typeConstToStarlark(e))
	}

	// Convert repo layers
	var repoLayers []starlark.Value
	for _, r := range k.RepoLayers {
		repoLayers = append(repoLayers, typeConstToStarlark(r))
	}

	// Convert platforms
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

// typeConstToStarlark converts a TypeConstant to a Starlark struct.
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
	Name     string // Operation name (e.g., "copy", "expand", "rename")
	TypeName string // Go type name (e.g., "CopyOp", "ExpandOp")
	Line     int    // Line number where Name() is defined
	File     string // Source file
}

// goParseExecutionOps parses Go source files in the execution directory
// and extracts operation names from Op types.
//
// It looks for patterns like:
//
//	func (o *CopyOp) Name() string { return "copy" }
//
// Args:
//   - path: Path to the execution directory or ops.go file
//
// Returns:
//   - A struct with:
//   - operations: List of {name, type_name, file, line}
func goParseExecutionOps(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		// Look for ops.go in directory
		opsPath := filepath.Join(path, "ops.go")
		if _, err := os.Stat(opsPath); err == nil {
			files = append(files, opsPath)
		}
		// Also check for any other files with Op types
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
			continue // Skip files that fail to parse
		}
		for _, op := range fileOps {
			if !seen[op.Name] {
				seen[op.Name] = true
				ops = append(ops, op)
			}
		}
	}

	// Sort operations by name for consistent output
	sortOps(ops)

	// Convert to Starlark
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

// parseOpsFile parses a Go file and extracts execution operations.
// It looks for methods like: func (o *XxxOp) Name() string { return "xxx" }
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

		// Must be a method (have a receiver)
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return true
		}

		// Must be named "Name"
		if fn.Name == nil || fn.Name.Name != "Name" {
			return true
		}

		// Get receiver type name (e.g., "*CopyOp" -> "CopyOp")
		recvType := ""
		switch t := fn.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				recvType = ident.Name
			}
		case *ast.Ident:
			recvType = t.Name
		}

		// Must be an Op type
		if recvType == "" || !strings.HasSuffix(recvType, "Op") {
			return true
		}

		// Must return string
		if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
			return true
		}
		if ident, ok := fn.Type.Results.List[0].Type.(*ast.Ident); !ok || ident.Name != "string" {
			return true
		}

		// Extract the return value
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

// extractReturnString extracts the string literal from a simple return statement.
// Handles: return "value"
func extractReturnString(body *ast.BlockStmt) string {
	if body == nil || len(body.List) == 0 {
		return ""
	}

	// Look for return statement
	for _, stmt := range body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}

		// Get string literal
		lit, ok := ret.Results[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}

		// Unquote the string
		return strings.Trim(lit.Value, `"`)
	}

	return ""
}

// sortOps sorts operations alphabetically by name.
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
// DEVLORE API PARSING
// =============================================================================

// PlanBinding represents a Starlark plan binding with its slots and output.
type PlanBinding struct {
	Name       string            // Full binding name (e.g., "plan.file.configure")
	Slots      []string          // Slot names (can be filled with promise or immediate)
	SlotDocs   map[string]string // Slot documentation (slot name -> description)
	Operations []string          // Graph operations (e.g., ["render", "copy"])
	Output     string            // Output type: "promise" or "none"
	Doc        string            // Description from doc comment
	Usage      string            // Usage example from doc comment
	Returns    string            // Returns description from doc comment
	File       string            // Source file
	Line       int               // Line number
}

// goParseDevloreAPI parses Go source files in devlore-cli's starlark package
// and extracts the plan API: bindings, slots, and output types.
//
// It looks for:
//   - NewBuiltin("plan.xxx.yyy", ...) to find binding names
//   - FillSlot(node, graph, "slotName", ...) to find slot names
//   - NewOutput(...) returns to detect promise output
//
// Args:
//   - path: Path to the devlore-cli starlark directory (e.g., "internal/starlark")
//
// Returns:
//   - A struct with:
//   - bindings: List of {name, slots, output, file, line}
//   - namespaces: List of namespace strings
func goParseDevloreAPI(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.parse_devlore_api", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("go.parse_devlore_api: %w", err)
	}

	var files []string
	if info.IsDir() {
		// Collect all .go files in directory (including platform subdirs)
		err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "testdata" {
				return filepath.SkipDir
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("go.parse_devlore_api: walking dir: %w", err)
		}
	} else {
		files = []string{path}
	}

	var allBindings []PlanBinding
	seenBindings := make(map[string]bool)
	namespaces := make(map[string]bool)

	for _, file := range files {
		bindings, err := parseDevloreAPIFile(file)
		if err != nil {
			continue // Skip files that fail to parse
		}

		for _, b := range bindings {
			if !seenBindings[b.Name] {
				seenBindings[b.Name] = true
				allBindings = append(allBindings, b)

				// Extract namespace from binding name
				if idx := strings.LastIndex(b.Name, "."); idx > 0 {
					ns := b.Name[:idx]
					namespaces[ns] = true
					// Also add parent namespaces
					for {
						if idx := strings.LastIndex(ns, "."); idx > 0 {
							ns = ns[:idx]
							namespaces[ns] = true
						} else {
							break
						}
					}
				}
			}
		}
	}

	// Sort bindings by name
	sortPlanBindings(allBindings)

	// Build hierarchical structure: context → namespace → methods
	// This is THE canonical representation of the Starlark API
	plan := make(map[string][]starlark.Value)   // plan.file → [...methods]
	system := make(map[string][]starlark.Value) // system.file → [...methods]
	var violations []starlark.Value

	for _, b := range allBindings {
		// Check for violations
		if strings.HasPrefix(b.Output, "VIOLATION:") {
			violations = append(violations, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"name":  starlark.String(b.Name),
				"file":  starlark.String(b.File),
				"line":  starlark.MakeInt(b.Line),
				"error": starlark.String("uses StringDict instead of Attr receiver"),
			}))
			continue
		}

		// Parse binding name: context.namespace.method or context.method
		parts := strings.Split(b.Name, ".")
		if len(parts) < 2 {
			continue
		}

		context := parts[0] // "plan" or "system"
		var namespace, methodName string
		if len(parts) == 2 {
			namespace = "(root)"
			methodName = parts[1]
		} else {
			namespace = parts[1]
			methodName = parts[len(parts)-1]
		}

		// Build method entry
		method := bindingToHierarchicalStarlark(b, methodName)

		// Add to appropriate context
		switch context {
		case "plan":
			plan[namespace] = append(plan[namespace], method)
		case "system":
			system[namespace] = append(system[namespace], method)
		}
	}

	// Convert maps to Starlark dicts
	planDict := starlark.StringDict{}
	for ns, methods := range plan {
		planDict[ns] = starlark.NewList(methods)
	}

	systemDict := starlark.StringDict{}
	for ns, methods := range system {
		systemDict[ns] = starlark.NewList(methods)
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"valid":      starlark.Bool(len(violations) == 0),
		"plan":       starlarkstruct.FromStringDict(starlarkstruct.Default, planDict),
		"system":     starlarkstruct.FromStringDict(starlarkstruct.Default, systemDict),
		"violations": starlark.NewList(violations),
	}), nil
}

// bindingToHierarchicalStarlark converts a PlanBinding to a Starlark struct for hierarchical output.
func bindingToHierarchicalStarlark(b PlanBinding, methodName string) starlark.Value {
	var slotsList []starlark.Value
	for _, s := range b.Slots {
		slotsList = append(slotsList, starlark.String(s))
	}

	var opsList []starlark.Value
	for _, op := range b.Operations {
		opsList = append(opsList, starlark.String(op))
	}

	slotDocsDict := starlark.StringDict{}
	for slotName, slotDoc := range b.SlotDocs {
		slotDocsDict[slotName] = starlark.String(slotDoc)
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"name":       starlark.String(methodName),
		"full_name":  starlark.String(b.Name),
		"doc":        starlark.String(b.Doc),
		"usage":      starlark.String(b.Usage),
		"slots":      starlark.NewList(slotsList),
		"slot_docs":  starlarkstruct.FromStringDict(starlarkstruct.Default, slotDocsDict),
		"operations": starlark.NewList(opsList),
		"output":     starlark.String(b.Output),
		"returns":    starlark.String(b.Returns),
		"file":       starlark.String(b.File),
		"line":       starlark.MakeInt(b.Line),
	})
}

// parseDevloreAPIFile parses a single Go file and extracts plan bindings using AST.
// It also detects violations where bindings are registered via StringDict instead of Attr methods.
func parseDevloreAPIFile(path string) ([]PlanBinding, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(path)
	var bindings []PlanBinding
	seen := make(map[string]bool)

	// Find bindings via Attr methods (the CORRECT pattern)
	attrBindings := findAttrBindings(node, fset)
	for bindingName, methodName := range attrBindings {
		if !isAPIBinding(bindingName) {
			continue
		}
		if seen[bindingName] {
			continue
		}

		binding := extractBindingFromMethod(node, fset, methodName, bindingName, filename)
		if binding != nil {
			seen[bindingName] = true
			bindings = append(bindings, *binding)
		}
	}

	// Detect StringDict violations (the WRONG pattern)
	violations := findStringDictViolations(node, fset, filename)
	for _, v := range violations {
		if !isAPIBinding(v.Name) {
			continue
		}
		if seen[v.Name] {
			continue
		}
		// Mark as violation - this binding uses the wrong pattern
		bindings = append(bindings, PlanBinding{
			Name:       v.Name,
			Slots:      nil,
			Operations: nil,
			Output:     "VIOLATION: uses StringDict instead of Attr receiver",
			File:       filename,
			Line:       v.Line,
		})
		seen[v.Name] = true
	}

	return bindings, nil
}

// StringDictViolation represents a binding incorrectly registered via StringDict.
type StringDictViolation struct {
	Name string
	Line int
}

// findStringDictViolations finds plan.* bindings incorrectly registered via StringDict.
// These are CONTRACT VIOLATIONS - all plan bindings must use Attr receiver methods.
func findStringDictViolations(node *ast.File, fset *token.FileSet, filename string) []StringDictViolation {
	var violations []StringDictViolation

	ast.Inspect(node, func(n ast.Node) bool {
		// Look for composite literals (map/struct creation)
		comp, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// Check if this is a starlark.StringDict
		sel, ok := comp.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "StringDict" {
			return true
		}

		// Walk through the map entries
		for _, elt := range comp.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}

			// The value should be a NewBuiltin call
			call, ok := kv.Value.(*ast.CallExpr)
			if !ok {
				continue
			}

			// Check if this is a NewBuiltin call
			callSel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || callSel.Sel.Name != "NewBuiltin" {
				continue
			}

			// Extract binding name (first argument)
			if len(call.Args) < 2 {
				continue
			}

			bindingLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || bindingLit.Kind != token.STRING {
				continue
			}
			bindingName := strings.Trim(bindingLit.Value, `"`)

			violations = append(violations, StringDictViolation{
				Name: bindingName,
				Line: fset.Position(call.Pos()).Line,
			})
		}

		return true
	})

	return violations
}

// findAttrBindings finds all NewBuiltin calls in Attr methods and returns a map
// of binding name -> handler method name.
func findAttrBindings(node *ast.File, fset *token.FileSet) map[string]string {
	bindings := make(map[string]string)

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name == nil || fn.Name.Name != "Attr" {
			return true
		}

		// Walk the Attr method body looking for NewBuiltin calls
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check if this is a NewBuiltin call
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "NewBuiltin" {
				return true
			}

			// Extract binding name (first argument)
			if len(call.Args) < 2 {
				return true
			}

			bindingLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || bindingLit.Kind != token.STRING {
				return true
			}
			bindingName := strings.Trim(bindingLit.Value, `"`)

			// Extract handler method name (second argument)
			// Pattern: f.methodName or s.methodName
			handlerSel, ok := call.Args[1].(*ast.SelectorExpr)
			if !ok {
				return true
			}
			methodName := handlerSel.Sel.Name

			bindings[bindingName] = methodName
			return true
		})

		return true
	})

	return bindings
}

// extractBindingFromMethod extracts binding info from a handler method using AST.
func extractBindingFromMethod(node *ast.File, fset *token.FileSet, methodName, bindingName, filename string) *PlanBinding {
	var binding *PlanBinding

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name == nil || fn.Name.Name != methodName {
			return true
		}

		// Found the method - extract info
		slots := extractSlotsFromAST(fn.Body)
		operations := extractOperationsFromAST(fn.Body)
		output := extractOutputFromAST(fn.Body)

		// Extract documentation from doc comment
		doc, usage, slotDocs, returns := parseDocComment(fn.Doc)

		binding = &PlanBinding{
			Name:       bindingName,
			Slots:      slots,
			SlotDocs:   slotDocs,
			Operations: operations,
			Output:     output,
			Doc:        doc,
			Usage:      usage,
			Returns:    returns,
			File:       filename,
			Line:       fset.Position(fn.Pos()).Line,
		}

		return false // Found it, stop searching
	})

	return binding
}

// parseDocComment parses a Go doc comment and extracts structured documentation.
// Expected format:
//
//	// description line(s)
//	// Usage: plan.namespace.method(args)
//	//
//	// Slots:
//	//   - slot_name: description
//	//
//	// Returns: description
func parseDocComment(doc *ast.CommentGroup) (description, usage string, slotDocs map[string]string, returns string) {
	slotDocs = make(map[string]string)

	if doc == nil {
		return
	}

	// Get the full doc text
	text := doc.Text()
	lines := strings.Split(text, "\n")

	var descLines []string
	inSlots := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for Usage:
		if strings.HasPrefix(line, "Usage:") {
			usage = strings.TrimSpace(strings.TrimPrefix(line, "Usage:"))
			inSlots = false
			continue
		}

		// Check for Slots:
		if strings.HasPrefix(line, "Slots:") {
			inSlots = true
			continue
		}

		// Check for Returns:
		if strings.HasPrefix(line, "Returns:") {
			returns = strings.TrimSpace(strings.TrimPrefix(line, "Returns:"))
			inSlots = false
			continue
		}

		// Parse slot documentation
		if inSlots && strings.HasPrefix(line, "- ") {
			// Format: "- slot_name: description"
			slotLine := strings.TrimPrefix(line, "- ")
			if colonIdx := strings.Index(slotLine, ":"); colonIdx > 0 {
				slotName := strings.TrimSpace(slotLine[:colonIdx])
				slotDesc := strings.TrimSpace(slotLine[colonIdx+1:])
				slotDocs[slotName] = slotDesc
			}
			continue
		}

		// Empty line ends slots section
		if inSlots && line == "" {
			inSlots = false
			continue
		}

		// Accumulate description lines (before any section)
		if usage == "" && !inSlots && returns == "" && line != "" {
			descLines = append(descLines, line)
		}
	}

	description = strings.Join(descLines, " ")
	return
}

// extractSlotsFromAST extracts slot names from FillSlot calls using AST.
func extractSlotsFromAST(body *ast.BlockStmt) []string {
	var slots []string
	seen := make(map[string]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if this is a FillSlot call
		ident, ok := call.Fun.(*ast.Ident)
		if !ok || ident.Name != "FillSlot" {
			return true
		}

		// FillSlot(node, graph, "slotName", value) - third argument is slot name
		if len(call.Args) < 4 {
			return true
		}

		slotLit, ok := call.Args[2].(*ast.BasicLit)
		if !ok || slotLit.Kind != token.STRING {
			return true
		}

		slotName := strings.Trim(slotLit.Value, `"`)
		if !seen[slotName] {
			seen[slotName] = true
			slots = append(slots, slotName)
		}

		return true
	})

	return slots
}

// extractOperationsFromAST extracts operations from execution.Node literals using AST.
func extractOperationsFromAST(body *ast.BlockStmt) []string {
	var operations []string

	ast.Inspect(body, func(n ast.Node) bool {
		// Look for composite literals (struct creation)
		comp, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// Check if this is an execution.Node or &execution.Node
		isExecutionNode := false
		switch t := comp.Type.(type) {
		case *ast.SelectorExpr:
			// execution.Node
			if ident, ok := t.X.(*ast.Ident); ok && ident.Name == "execution" && t.Sel.Name == "Node" {
				isExecutionNode = true
			}
		}

		if !isExecutionNode {
			return true
		}

		// Find the Operations field
		for _, elt := range comp.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}

			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Operations" {
				continue
			}

			// Extract string values from the slice literal
			compLit, ok := kv.Value.(*ast.CompositeLit)
			if !ok {
				continue
			}

			for _, elem := range compLit.Elts {
				lit, ok := elem.(*ast.BasicLit)
				if ok && lit.Kind == token.STRING {
					operations = append(operations, strings.Trim(lit.Value, `"`))
				}
			}
		}

		return true
	})

	return operations
}

// extractOutputFromAST detects if the method returns a promise (NewOutput).
func extractOutputFromAST(body *ast.BlockStmt) string {
	output := "none"

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if this is a NewOutput call
		ident, ok := call.Fun.(*ast.Ident)
		if ok && ident.Name == "NewOutput" {
			output = "promise"
			return false // Found it
		}

		return true
	})

	return output
}

// isAPIBinding returns true if the binding name is part of the devlore API.
func isAPIBinding(name string) bool {
	return strings.HasPrefix(name, "plan.") || strings.HasPrefix(name, "system.")
}

// sortPlanBindings sorts bindings alphabetically by name.
func sortPlanBindings(bindings []PlanBinding) {
	for i := 0; i < len(bindings)-1; i++ {
		for j := i + 1; j < len(bindings); j++ {
			if bindings[i].Name > bindings[j].Name {
				bindings[i], bindings[j] = bindings[j], bindings[i]
			}
		}
	}
}

// =============================================================================
// EXECUTION SCHEMA PARSING
// =============================================================================

// StructField represents a field extracted from a Go struct.
type StructField struct {
	Name        string // Go field name
	JSONName    string // JSON field name from tag
	Type        string // Go type
	Required    bool   // Not omitempty
	Description string // From comment
}

// StructDef represents a Go struct definition.
type StructDef struct {
	Name   string        // Struct name
	Fields []StructField // Fields
}

// ConstValue represents a const value extracted from Go.
type ConstValue struct {
	Name  string
	Value string
}

// goParseExecutionSchema parses Go source files in the execution package
// and extracts struct definitions for JSON schema generation.
//
// Args:
//   - path: Path to the execution directory (e.g., "internal/execution")
//
// Returns:
//   - A struct with:
//   - node: Node struct definition
//   - slot_value: SlotValue struct definition
//   - edge: Edge struct definition
//   - graph_states: GraphState enum values
//   - node_statuses: NodeStatus enum values
//   - operations: List of operation names (from ops.go)
func goParseExecutionSchema(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.parse_execution_schema", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	// Parse graph.go for struct definitions
	graphPath := filepath.Join(path, "graph.go")
	structs, consts, err := parseStructDefs(graphPath)
	if err != nil {
		return nil, fmt.Errorf("go.parse_execution_schema: parsing graph.go: %w", err)
	}

	// Parse operations from ops*.go
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

	// Convert to Starlark values
	result := starlark.StringDict{}

	// Add struct definitions
	for name, def := range structs {
		result[toSnakeCase(name)] = structDefToStarlark(def)
	}

	// Add const enums
	for typeName, values := range consts {
		var enumList []starlark.Value
		for _, v := range values {
			enumList = append(enumList, starlark.String(v.Value))
		}
		result[toSnakeCase(typeName)+"s"] = starlark.NewList(enumList)
	}

	// Add operations
	var opsList []starlark.Value
	for _, op := range ops {
		opsList = append(opsList, starlark.String(op.Name))
	}
	result["operations"] = starlark.NewList(opsList)

	return starlarkstruct.FromStringDict(starlarkstruct.Default, result), nil
}

// parseStructDefs parses a Go file and extracts struct and const definitions.
func parseStructDefs(path string) (map[string]StructDef, map[string][]ConstValue, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	structs := make(map[string]StructDef)
	consts := make(map[string][]ConstValue)

	// Track current const type for iota-style declarations
	var currentConstType string

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				// Type declaration
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
							continue // Embedded field
						}

						sf := StructField{
							Name: field.Names[0].Name,
							Type: typeToString(field.Type),
						}

						// Parse struct tag for JSON info
						if field.Tag != nil {
							tag := strings.Trim(field.Tag.Value, "`")
							sf.JSONName, sf.Required = parseJSONTag(tag)
						}

						// Get description from comment
						if field.Comment != nil {
							sf.Description = strings.TrimSpace(field.Comment.Text())
						} else if field.Doc != nil {
							sf.Description = strings.TrimSpace(field.Doc.Text())
						}

						// Skip fields with json:"-"
						if sf.JSONName == "-" {
							continue
						}

						def.Fields = append(def.Fields, sf)
					}

					structs[def.Name] = def
				}
			} else if x.Tok == token.CONST {
				// Const declaration
				for _, spec := range x.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					// Check if this declares a type
					if valueSpec.Type != nil {
						if ident, ok := valueSpec.Type.(*ast.Ident); ok {
							currentConstType = ident.Name
						}
					}

					// Extract const values
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

// typeToString converts an AST type to a string representation.
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

// parseJSONTag extracts the JSON field name and whether it's required.
func parseJSONTag(tag string) (name string, required bool) {
	// Find json:"..." in the tag
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

// toSnakeCase converts CamelCase to snake_case.
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

// structDefToStarlark converts a StructDef to a Starlark struct.
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
