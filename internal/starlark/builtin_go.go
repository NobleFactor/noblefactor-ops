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
			"complexity":              starlark.NewBuiltin("go.complexity", goComplexity),
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

	var bindings []Binding
	var namespaces []Namespace

	// Regex patterns for extracting names from string literals
	// Pattern 1: NewBuiltin("name", handlerFunc) - named handler
	// Pattern 2: NewBuiltin("name", func(...) - inline anonymous function
	newBuiltinRe := regexp.MustCompile(`NewBuiltin\s*\(\s*"([^"]+)"\s*,\s*(?:(\w+\.)?(\w+)\s*\)|func\s*\()`)
	fromStringDictRe := regexp.MustCompile(`FromStringDict\s*\(\s*starlark\.String\s*\(\s*"([^"]+)"`)

	// Read file content for regex matching (AST doesn't preserve raw string positions well)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	// First pass: find all functions that mutate (create execution.Node or modify graph)
	mutatingFuncs := findMutatingFunctions(node, fset, contentStr)

	// Track current context for nested namespaces
	currentNamespace := ""

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Track function context for namespace resolution
			// Functions like platformStruct(), packageStruct() create sub-namespaces
			if x.Name != nil {
				name := x.Name.Name
				if strings.HasSuffix(name, "Struct") || strings.HasSuffix(name, "Module") {
					// Extract base namespace from function name
					baseName := strings.TrimSuffix(strings.TrimSuffix(name, "Struct"), "Module")
					baseName = camelToSnake(baseName)
					currentNamespace = baseName
				}
			}

		case *ast.CallExpr:
			line := fset.Position(x.Pos()).Line
			if line > 0 && line <= len(lines) {
				lineContent := lines[line-1]

				// Check for NewBuiltin calls - capture handler function name
				if matches := newBuiltinRe.FindStringSubmatch(lineContent); len(matches) > 1 {
					bindingName := matches[1]
					// Handler name is in matches[3] if it's a named function
					// If it's an inline func, matches[3] will be empty
					handlerName := ""
					if len(matches) > 3 && matches[3] != "" {
						handlerName = matches[3]
					}

					// Check if it's an inline function (no handler name)
					isInlineFunc := strings.Contains(lineContent, ", func(")

					binding := Binding{
						Name:     extractMethodName(bindingName),
						FullName: bindingName,
						Line:     line,
						Handler:  handlerName,
						// Inline functions are assumed non-mutating (system.* queries)
						// Named handlers check the mutatingFuncs map
						Mutates: !isInlineFunc && mutatingFuncs[handlerName],
					}
					// Determine namespace from full name
					if idx := strings.LastIndex(bindingName, "."); idx > 0 {
						binding.Namespace = bindingName[:idx]
					}
					bindings = append(bindings, binding)
				}

				// Check for FromStringDict calls (namespace definitions)
				if matches := fromStringDictRe.FindStringSubmatch(lineContent); len(matches) > 1 {
					nsName := matches[1]
					ns := Namespace{
						Name: nsName,
						Line: line,
					}
					// Determine parent namespace
					if currentNamespace != "" && currentNamespace != nsName {
						ns.Parent = currentNamespace
						ns.Name = currentNamespace + "." + nsName
					}
					namespaces = append(namespaces, ns)
				}
			}
		}
		return true
	})

	// Second pass: infer namespaces from binding names
	inferredNS := make(map[string]bool)
	for _, b := range bindings {
		if b.Namespace != "" {
			parts := strings.Split(b.Namespace, ".")
			for i := range parts {
				ns := strings.Join(parts[:i+1], ".")
				if !inferredNS[ns] {
					inferredNS[ns] = true
					// Check if we already have this namespace
					found := false
					for _, existingNS := range namespaces {
						if existingNS.Name == ns {
							found = true
							break
						}
					}
					if !found {
						namespaces = append(namespaces, Namespace{Name: ns, Line: 0})
					}
				}
			}
		}
	}

	return bindings, namespaces, nil
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
// CODE COMPLEXITY ANALYSIS
// =============================================================================

// FunctionComplexity holds complexity metrics for a single function.
type FunctionComplexity struct {
	Name       string
	File       string
	Line       int
	Cyclomatic int // McCabe cyclomatic complexity
	Cognitive  int // Cognitive complexity (SonarSource algorithm)
	LOC        int // Lines of code in function
	Params     int // Number of parameters
}

// FileComplexity holds complexity metrics for a file.
type FileComplexity struct {
	Path          string
	Functions     []FunctionComplexity
	TotalCyclo    int
	TotalCognit   int
	TotalLOC      int
	AvgCyclo      float64
	AvgCognit     float64
	MaxCyclo      int
	MaxCognit     int
	MaxCycloFunc  string
	MaxCognitFunc string
}

// goComplexity calculates cyclomatic and cognitive complexity for Go source.
//
// Args:
//   - path: Path to a Go file or directory
//
// Returns:
//   - A struct with:
//   - files: List of file complexity structs
//   - total_cyclomatic: Sum of all cyclomatic complexity
//   - total_cognitive: Sum of all cognitive complexity
//   - total_loc: Total lines of code
//   - avg_cyclomatic: Average cyclomatic per function
//   - avg_cognitive: Average cognitive per function
//   - max_cyclomatic: Highest cyclomatic complexity found
//   - max_cognitive: Highest cognitive complexity found
//   - hotspots: Functions with cyclomatic > 10 or cognitive > 15
func goComplexity(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("go.complexity", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectGoFiles(path)
	if err != nil {
		return nil, fmt.Errorf("go.complexity: %w", err)
	}

	var allFiles []FileComplexity
	var totalFuncs int
	var totalCyclo, totalCognit, totalLOC int
	var maxCyclo, maxCognit int
	var maxCycloFunc, maxCognitFunc string
	var hotspots []starlark.Value

	for _, file := range files {
		fc, err := analyzeFileComplexity(file)
		if err != nil {
			continue // Skip files that fail to parse
		}

		allFiles = append(allFiles, fc)
		totalFuncs += len(fc.Functions)
		totalCyclo += fc.TotalCyclo
		totalCognit += fc.TotalCognit
		totalLOC += fc.TotalLOC

		if fc.MaxCyclo > maxCyclo {
			maxCyclo = fc.MaxCyclo
			maxCycloFunc = fc.MaxCycloFunc
		}
		if fc.MaxCognit > maxCognit {
			maxCognit = fc.MaxCognit
			maxCognitFunc = fc.MaxCognitFunc
		}

		// Identify hotspots
		for _, fn := range fc.Functions {
			if fn.Cyclomatic > 10 || fn.Cognitive > 15 {
				hotspots = append(hotspots, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
					"name":       starlark.String(fn.Name),
					"file":       starlark.String(fn.File),
					"line":       starlark.MakeInt(fn.Line),
					"cyclomatic": starlark.MakeInt(fn.Cyclomatic),
					"cognitive":  starlark.MakeInt(fn.Cognitive),
					"loc":        starlark.MakeInt(fn.LOC),
				}))
			}
		}
	}

	// Calculate averages
	var avgCyclo, avgCognit float64
	if totalFuncs > 0 {
		avgCyclo = float64(totalCyclo) / float64(totalFuncs)
		avgCognit = float64(totalCognit) / float64(totalFuncs)
	}

	// Convert files to Starlark
	var filesList []starlark.Value
	for _, fc := range allFiles {
		filesList = append(filesList, fileComplexityToStarlark(fc))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"files":            starlark.NewList(filesList),
		"total_cyclomatic": starlark.MakeInt(totalCyclo),
		"total_cognitive":  starlark.MakeInt(totalCognit),
		"total_loc":        starlark.MakeInt(totalLOC),
		"total_functions":  starlark.MakeInt(totalFuncs),
		"avg_cyclomatic":   starlark.Float(avgCyclo),
		"avg_cognitive":    starlark.Float(avgCognit),
		"max_cyclomatic":   starlark.MakeInt(maxCyclo),
		"max_cognitive":    starlark.MakeInt(maxCognit),
		"max_cyclo_func":   starlark.String(maxCycloFunc),
		"max_cognit_func":  starlark.String(maxCognitFunc),
		"hotspots":         starlark.NewList(hotspots),
	}), nil
}

// analyzeFileComplexity calculates complexity metrics for a single file.
func analyzeFileComplexity(path string) (FileComplexity, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return FileComplexity{}, err
	}

	fc := FileComplexity{Path: path}

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}

		funcName := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			// Method: include receiver type in name
			if t, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
				if ident, ok := t.X.(*ast.Ident); ok {
					funcName = ident.Name + "." + funcName
				}
			} else if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
				funcName = ident.Name + "." + funcName
			}
		}

		startLine := fset.Position(fn.Pos()).Line
		endLine := fset.Position(fn.End()).Line
		loc := endLine - startLine + 1

		// Count parameters
		paramCount := 0
		if fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				if len(field.Names) == 0 {
					paramCount++ // Unnamed parameter
				} else {
					paramCount += len(field.Names)
				}
			}
		}

		cyclo := calculateCyclomatic(fn)
		cognit := calculateCognitive(fn)

		fnComplexity := FunctionComplexity{
			Name:       funcName,
			File:       filepath.Base(path),
			Line:       startLine,
			Cyclomatic: cyclo,
			Cognitive:  cognit,
			LOC:        loc,
			Params:     paramCount,
		}

		fc.Functions = append(fc.Functions, fnComplexity)
		fc.TotalCyclo += cyclo
		fc.TotalCognit += cognit
		fc.TotalLOC += loc

		if cyclo > fc.MaxCyclo {
			fc.MaxCyclo = cyclo
			fc.MaxCycloFunc = funcName
		}
		if cognit > fc.MaxCognit {
			fc.MaxCognit = cognit
			fc.MaxCognitFunc = funcName
		}

		return true
	})

	if len(fc.Functions) > 0 {
		fc.AvgCyclo = float64(fc.TotalCyclo) / float64(len(fc.Functions))
		fc.AvgCognit = float64(fc.TotalCognit) / float64(len(fc.Functions))
	}

	return fc, nil
}

// calculateCyclomatic calculates McCabe cyclomatic complexity.
// Formula: 1 + number of decision points
// Decision points: if, for, switch case, select case, &&, ||
func calculateCyclomatic(fn *ast.FuncDecl) int {
	complexity := 1 // Base complexity

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			if x.List != nil { // Not default case
				complexity++
			}
		case *ast.CommClause:
			if x.Comm != nil { // Not default case
				complexity++
			}
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				complexity++
			}
		}
		return true
	})

	return complexity
}

// calculateCognitive calculates cognitive complexity (SonarSource algorithm).
// Increments for: control structures, nesting, recursion, breaks in flow.
// Nesting adds extra penalty for nested control structures.
func calculateCognitive(fn *ast.FuncDecl) int {
	complexity := 0
	nesting := 0

	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}

		switch x := n.(type) {
		case *ast.IfStmt:
			complexity += 1 + nesting // Base + nesting penalty
			nesting++
			walk(x.Cond)
			walk(x.Body)
			if x.Else != nil {
				// else if doesn't add to nesting, else does
				if _, isIf := x.Else.(*ast.IfStmt); !isIf {
					complexity++ // else keyword
				}
				walk(x.Else)
			}
			nesting--
			return

		case *ast.ForStmt:
			complexity += 1 + nesting
			nesting++
			if x.Init != nil {
				walk(x.Init)
			}
			if x.Cond != nil {
				walk(x.Cond)
			}
			if x.Post != nil {
				walk(x.Post)
			}
			walk(x.Body)
			nesting--
			return

		case *ast.RangeStmt:
			complexity += 1 + nesting
			nesting++
			walk(x.Key)
			walk(x.Value)
			walk(x.X)
			walk(x.Body)
			nesting--
			return

		case *ast.SwitchStmt:
			complexity += 1 + nesting
			nesting++
			if x.Init != nil {
				walk(x.Init)
			}
			if x.Tag != nil {
				walk(x.Tag)
			}
			walk(x.Body)
			nesting--
			return

		case *ast.TypeSwitchStmt:
			complexity += 1 + nesting
			nesting++
			if x.Init != nil {
				walk(x.Init)
			}
			walk(x.Assign)
			walk(x.Body)
			nesting--
			return

		case *ast.SelectStmt:
			complexity += 1 + nesting
			nesting++
			walk(x.Body)
			nesting--
			return

		case *ast.BinaryExpr:
			// Sequences of && and || add complexity
			if x.Op == token.LAND || x.Op == token.LOR {
				complexity++
			}
			walk(x.X)
			walk(x.Y)
			return

		case *ast.BranchStmt:
			// break, continue, goto with labels add complexity
			if x.Label != nil {
				complexity++
			}

		case *ast.FuncLit:
			// Nested functions increase nesting
			nesting++
			walk(x.Body)
			nesting--
			return
		}

		// Default: walk children
		switch x := n.(type) {
		case *ast.BlockStmt:
			for _, stmt := range x.List {
				walk(stmt)
			}
		case *ast.ExprStmt:
			walk(x.X)
		case *ast.AssignStmt:
			for _, expr := range x.Lhs {
				walk(expr)
			}
			for _, expr := range x.Rhs {
				walk(expr)
			}
		case *ast.ReturnStmt:
			for _, expr := range x.Results {
				walk(expr)
			}
		case *ast.DeclStmt:
			walk(x.Decl)
		case *ast.CallExpr:
			walk(x.Fun)
			for _, arg := range x.Args {
				walk(arg)
			}
		case *ast.CaseClause:
			for _, expr := range x.List {
				walk(expr)
			}
			for _, stmt := range x.Body {
				walk(stmt)
			}
		case *ast.CommClause:
			walk(x.Comm)
			for _, stmt := range x.Body {
				walk(stmt)
			}
		}
	}

	walk(fn.Body)
	return complexity
}

// fileComplexityToStarlark converts FileComplexity to a Starlark struct.
func fileComplexityToStarlark(fc FileComplexity) starlark.Value {
	var functions []starlark.Value
	for _, fn := range fc.Functions {
		functions = append(functions, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":       starlark.String(fn.Name),
			"file":       starlark.String(fn.File),
			"line":       starlark.MakeInt(fn.Line),
			"cyclomatic": starlark.MakeInt(fn.Cyclomatic),
			"cognitive":  starlark.MakeInt(fn.Cognitive),
			"loc":        starlark.MakeInt(fn.LOC),
			"params":     starlark.MakeInt(fn.Params),
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":            starlark.String(fc.Path),
		"functions":       starlark.NewList(functions),
		"total_cyclo":     starlark.MakeInt(fc.TotalCyclo),
		"total_cognit":    starlark.MakeInt(fc.TotalCognit),
		"total_loc":       starlark.MakeInt(fc.TotalLOC),
		"avg_cyclo":       starlark.Float(fc.AvgCyclo),
		"avg_cognit":      starlark.Float(fc.AvgCognit),
		"max_cyclo":       starlark.MakeInt(fc.MaxCyclo),
		"max_cognit":      starlark.MakeInt(fc.MaxCognit),
		"max_cyclo_func":  starlark.String(fc.MaxCycloFunc),
		"max_cognit_func": starlark.String(fc.MaxCognitFunc),
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
