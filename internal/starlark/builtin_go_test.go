// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package starlark

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoFile_BasicBindings(t *testing.T) {
	// Pattern derived from devlore-cli/internal/starlark/bindings.go
	// Uses method receivers (b.method) like the real code
	content := `package starlark

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type Bindings struct{}

func (b *Bindings) fsStruct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String("fs"), starlark.StringDict{
		"exists": starlark.NewBuiltin("fs.exists", b.fsExists),
		"read":   starlark.NewBuiltin("fs.read", b.fsRead),
	})
}

func (b *Bindings) fsExists(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.Bool(true), nil
}

func (b *Bindings) fsRead(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String("content"), nil
}
`
	tmpFile := createTempGoFile(t, content)
	defer os.Remove(tmpFile)

	bindings, namespaces, err := parseGoFile(tmpFile)
	if err != nil {
		t.Fatalf("parseGoFile failed: %v", err)
	}

	// Should find 2 bindings
	if len(bindings) != 2 {
		t.Errorf("expected 2 bindings, got %d", len(bindings))
	}

	// Check first binding - note handler is "fsExists" not "b.fsExists"
	found := findBinding(bindings, "fs.exists")
	if found == nil {
		t.Error("expected to find binding 'fs.exists'")
	} else {
		if found.Name != "exists" {
			t.Errorf("expected Name 'exists', got %q", found.Name)
		}
		if found.Namespace != "fs" {
			t.Errorf("expected Namespace 'fs', got %q", found.Namespace)
		}
		if found.Handler != "fsExists" {
			t.Errorf("expected Handler 'fsExists', got %q", found.Handler)
		}
	}

	// Check second binding
	found = findBinding(bindings, "fs.read")
	if found == nil {
		t.Error("expected to find binding 'fs.read'")
	}

	// Should have namespace "fs" from FromStringDict
	nsFound := false
	for _, ns := range namespaces {
		if ns.Name == "fs" {
			nsFound = true
			break
		}
	}
	if !nsFound {
		t.Error("expected to find namespace 'fs'")
	}
}

func TestParseGoFile_NestedNamespaces(t *testing.T) {
	// Pattern derived from devlore-cli nested namespace structure
	content := `package starlark

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type Bindings struct{}

func (b *Bindings) planStruct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String("plan"), starlark.StringDict{
		"package": b.packageStruct(),
	})
}

func (b *Bindings) packageStruct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String("package"), starlark.StringDict{
		"install": starlark.NewBuiltin("plan.package.install", b.packageInstall),
		"remove":  starlark.NewBuiltin("plan.package.remove", b.packageRemove),
	})
}

func (b *Bindings) packageInstall(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, nil
}

func (b *Bindings) packageRemove(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, nil
}
`
	tmpFile := createTempGoFile(t, content)
	defer os.Remove(tmpFile)

	bindings, namespaces, err := parseGoFile(tmpFile)
	if err != nil {
		t.Fatalf("parseGoFile failed: %v", err)
	}

	// Should find 2 bindings with nested namespace
	if len(bindings) != 2 {
		t.Errorf("expected 2 bindings, got %d", len(bindings))
	}

	found := findBinding(bindings, "plan.package.install")
	if found == nil {
		t.Error("expected to find binding 'plan.package.install'")
	} else {
		if found.Namespace != "plan.package" {
			t.Errorf("expected Namespace 'plan.package', got %q", found.Namespace)
		}
	}

	// Should infer both "plan" and "plan.package" namespaces
	planFound := false
	planPackageFound := false
	for _, ns := range namespaces {
		if ns.Name == "plan" {
			planFound = true
		}
		if ns.Name == "plan.package" {
			planPackageFound = true
		}
	}
	if !planFound {
		t.Error("expected to find namespace 'plan'")
	}
	if !planPackageFound {
		t.Error("expected to find namespace 'plan.package'")
	}
}

func TestParseGoFile_InlineFunctions(t *testing.T) {
	// Pattern derived from devlore-cli/internal/starlark/system.go
	// system.* bindings use inline functions for read-only queries
	content := `package starlark

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func systemPackageStruct(pm PackageManager) *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String("package"), starlark.StringDict{
		"installed": starlark.NewBuiltin("system.package.installed", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.Bool(true), nil
		}),
		"manager": starlark.NewBuiltin("system.package.manager", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
			return starlark.String(pm.Name()), nil
		}),
	})
}
`
	tmpFile := createTempGoFile(t, content)
	defer os.Remove(tmpFile)

	bindings, _, err := parseGoFile(tmpFile)
	if err != nil {
		t.Fatalf("parseGoFile failed: %v", err)
	}

	if len(bindings) != 2 {
		t.Errorf("expected 2 bindings, got %d", len(bindings))
	}

	found := findBinding(bindings, "system.package.installed")
	if found == nil {
		t.Error("expected to find binding 'system.package.installed'")
	} else {
		// Inline functions should have empty handler name
		if found.Handler != "" {
			t.Errorf("expected empty Handler for inline func, got %q", found.Handler)
		}
		// Inline functions should be marked as non-mutating
		if found.Mutates {
			t.Error("expected inline function to be non-mutating")
		}
	}
}

func TestParseGoFile_MutatingFunctions(t *testing.T) {
	// Pattern derived from devlore-cli/internal/starlark/plan.go
	// planBindings methods create execution.Node and append to graph
	content := `package starlark

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"mymodule/execution"
)

type planBindings struct {
	graph *execution.Graph
}

func (p *planBindings) planStruct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String("plan"), starlark.StringDict{
		"shell": starlark.NewBuiltin("plan.shell", p.shellBuiltin),
	})
}

func (p *planBindings) shellBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// This creates an execution node, so it mutates
	node := &execution.Node{
		ID: "shell-1",
	}
	p.graph.Nodes = append(p.graph.Nodes, node)
	return starlark.None, nil
}
`
	tmpFile := createTempGoFile(t, content)
	defer os.Remove(tmpFile)

	bindings, _, err := parseGoFile(tmpFile)
	if err != nil {
		t.Fatalf("parseGoFile failed: %v", err)
	}

	found := findBinding(bindings, "plan.shell")
	if found == nil {
		t.Error("expected to find binding 'plan.shell'")
	} else {
		if !found.Mutates {
			t.Error("expected shellBuiltin to be marked as mutating (creates execution.Node)")
		}
	}
}

func TestParseGoFile_InterfaceMutation(t *testing.T) {
	// Pattern derived from devlore-cli/internal/starlark/plan.go
	// StarlarkPlanBindings methods call interface methods like s.PackageInstall
	content := `package starlark

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type StarlarkPlanBindings struct {
	PlanBindings
}

func (s *StarlarkPlanBindings) planStruct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String("plan"), starlark.StringDict{
		"install": starlark.NewBuiltin("plan.install", s.packageInstallBuiltin),
	})
}

func (s *StarlarkPlanBindings) packageInstallBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	// This calls s.PackageInstall which is a mutating interface method
	node := s.PackageInstall("vim")
	return nodeToStarlark(node), nil
}
`
	tmpFile := createTempGoFile(t, content)
	defer os.Remove(tmpFile)

	bindings, _, err := parseGoFile(tmpFile)
	if err != nil {
		t.Fatalf("parseGoFile failed: %v", err)
	}

	found := findBinding(bindings, "plan.install")
	if found == nil {
		t.Error("expected to find binding 'plan.install'")
	} else {
		if !found.Mutates {
			t.Error("expected packageInstallBuiltin to be marked as mutating (calls s.PackageInstall)")
		}
	}
}

func TestParseGoFile_NoBindings(t *testing.T) {
	content := `package example

func helper() string {
	return "no bindings here"
}
`
	tmpFile := createTempGoFile(t, content)
	defer os.Remove(tmpFile)

	bindings, namespaces, err := parseGoFile(tmpFile)
	if err != nil {
		t.Fatalf("parseGoFile failed: %v", err)
	}

	if len(bindings) != 0 {
		t.Errorf("expected 0 bindings, got %d", len(bindings))
	}
	if len(namespaces) != 0 {
		t.Errorf("expected 0 namespaces, got %d", len(namespaces))
	}
}

func TestParseGoFile_InvalidSyntax(t *testing.T) {
	content := `package example

func broken( {
	// invalid syntax
}
`
	tmpFile := createTempGoFile(t, content)
	defer os.Remove(tmpFile)

	_, _, err := parseGoFile(tmpFile)
	if err == nil {
		t.Error("expected error for invalid Go syntax")
	}
}

func TestParseGoFile_NonExistentFile(t *testing.T) {
	_, _, err := parseGoFile("/nonexistent/path/to/file.go")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestBindingVisitor_ExtractMethodName(t *testing.T) {
	tests := []struct {
		fullName string
		want     string
	}{
		{"example.hello", "hello"},
		{"plan.package.install", "install"},
		{"toplevel", "toplevel"},
		{"a.b.c.d.e", "e"},
	}

	for _, tt := range tests {
		got := extractMethodName(tt.fullName)
		if got != tt.want {
			t.Errorf("extractMethodName(%q) = %q, want %q", tt.fullName, got, tt.want)
		}
	}
}

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Platform", "platform"},
		{"PackageManager", "package_manager"},
		{"HTTPServer", "h_t_t_p_server"}, // Note: consecutive caps aren't handled specially
		{"simple", "simple"},
		{"ABC", "a_b_c"},
	}

	for _, tt := range tests {
		got := camelToSnake(tt.input)
		if got != tt.want {
			t.Errorf("camelToSnake(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFindMutatingFunctions(t *testing.T) {
	// Pattern derived from devlore-cli/internal/starlark/plan.go
	content := `package starlark

import "mymodule/execution"

type planBindings struct {
	graph *execution.Graph
}

// PackageInstall creates execution.Node and appends to graph - mutating
func (p *planBindings) PackageInstall(packages ...string) *execution.Node {
	node := &execution.Node{
		ID: "pkg-install",
	}
	p.graph.Nodes = append(p.graph.Nodes, node)
	return node
}

// fsExists is a pure query - non-mutating
func (b *Bindings) fsExists() bool {
	return true
}

// packageInstallBuiltin calls interface method - mutating
func (s *StarlarkPlanBindings) packageInstallBuiltin() {
	s.PackageInstall("vim")
}
`
	tmpFile := createTempGoFile(t, content)
	defer os.Remove(tmpFile)

	// Parse the file to get the AST
	fset, node, contentStr := parseForTesting(t, tmpFile)

	mutating := findMutatingFunctions(node, fset, contentStr)

	if !mutating["PackageInstall"] {
		t.Error("expected 'PackageInstall' to be identified as mutating (creates execution.Node)")
	}
	if mutating["fsExists"] {
		t.Error("expected 'fsExists' to NOT be identified as mutating")
	}
	if !mutating["packageInstallBuiltin"] {
		t.Error("expected 'packageInstallBuiltin' to be identified as mutating (calls s.PackageInstall)")
	}
}

func TestInferNamespaces(t *testing.T) {
	// Test that inferNamespaces correctly builds hierarchy
	v := &bindingVisitor{
		bindings: []Binding{
			{FullName: "a.b.c.method1", Namespace: "a.b.c"},
			{FullName: "a.b.method2", Namespace: "a.b"},
			{FullName: "x.y.method3", Namespace: "x.y"},
		},
		namespaces: []Namespace{}, // Start empty
	}

	v.inferNamespaces()

	// Should infer: a, a.b, a.b.c, x, x.y
	expected := map[string]bool{
		"a":     true,
		"a.b":   true,
		"a.b.c": true,
		"x":     true,
		"x.y":   true,
	}

	for _, ns := range v.namespaces {
		if !expected[ns.Name] {
			t.Errorf("unexpected namespace: %q", ns.Name)
		}
		delete(expected, ns.Name)
	}

	for ns := range expected {
		t.Errorf("missing expected namespace: %q", ns)
	}
}

// Helper functions

func createTempGoFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return tmpFile
}

func findBinding(bindings []Binding, fullName string) *Binding {
	for i := range bindings {
		if bindings[i].FullName == fullName {
			return &bindings[i]
		}
	}
	return nil
}

func parseForTesting(t *testing.T, path string) (*token.FileSet, *ast.File, string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	return fset, node, string(content)
}
