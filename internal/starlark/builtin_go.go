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
