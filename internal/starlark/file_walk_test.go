// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"os"
	"path/filepath"
	"testing"

	starlarklib "go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/NobleFactor/devlore-cli/pkg/op/provider/ui"
)

// =============================================================================
// Test: file.walk_tree basic
// =============================================================================

func TestFileWalkTree_Basic(t *testing.T) {
	root := t.TempDir()
	mkdirFile(t, root, "sub")
	writeTestFile(t, root, "file.txt", "hello")
	writeTestFile(t, root, "sub/nested.txt", "world")

	script := `
results = []
def collect(entry):
    results.append({"path": entry.path, "name": entry.name, "is_dir": entry.is_dir})

file.walk_tree(root=ROOT, callback=collect, gitignore=False)
`

	globals := execStarlark(t, script, root)

	resultsList := globals["results"].(*starlarklib.List)
	if resultsList.Len() < 3 {
		t.Fatalf("expected at least 3 entries (sub, file.txt, sub/nested.txt), got %d", resultsList.Len())
	}

	// Verify entries have the expected fields
	for i := 0; i < resultsList.Len(); i++ {
		entry := resultsList.Index(i).(*starlarklib.Dict)
		if _, found, _ := entry.Get(starlarklib.String("path")); !found {
			t.Error("entry missing 'path' field")
		}
		if _, found, _ := entry.Get(starlarklib.String("name")); !found {
			t.Error("entry missing 'name' field")
		}
		if _, found, _ := entry.Get(starlarklib.String("is_dir")); !found {
			t.Error("entry missing 'is_dir' field")
		}
	}
}

// =============================================================================
// Test: file.walk_tree respects gitignore
// =============================================================================

func TestFileWalkTree_GitignoreRespected(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitignore", "*.log\nbuild/\n")
	writeTestFile(t, root, "main.go", "package main")
	writeTestFile(t, root, "debug.log", "log data")
	mkdirFile(t, root, "build")
	writeTestFile(t, root, "build/output.bin", "binary")

	script := `
paths = []
def collect(entry):
    paths.append(entry.path)

file.walk_tree(root=ROOT, callback=collect)
`

	globals := execStarlark(t, script, root)

	pathsList := globals["paths"].(*starlarklib.List)
	var paths []string
	for i := 0; i < pathsList.Len(); i++ {
		s, _ := starlarklib.AsString(pathsList.Index(i))
		paths = append(paths, s)
	}

	assertStrContains(t, paths, "main.go")
	assertStrNotContains(t, paths, "debug.log")
	assertStrNotContains(t, paths, "build")
	assertStrNotContains(t, paths, filepath.Join("build", "output.bin"))
}

// =============================================================================
// Test: file.walk_tree control signals
// =============================================================================

func TestFileWalkTree_ControlSignals(t *testing.T) {
	root := t.TempDir()
	mkdirFile(t, root, "a")
	writeTestFile(t, root, "a/file.txt", "a")
	mkdirFile(t, root, "b")
	writeTestFile(t, root, "b/file.txt", "b")
	mkdirFile(t, root, "c")
	writeTestFile(t, root, "c/file.txt", "c")

	t.Run("skip directory", func(t *testing.T) {
		script := `
paths = []
def collect(entry):
    paths.append(entry.path)
    if entry.is_dir and entry.name == "b":
        return "skip"

file.walk_tree(root=ROOT, callback=collect, gitignore=False)
`
		globals := execStarlark(t, script, root)

		pathsList := globals["paths"].(*starlarklib.List)
		var paths []string
		for i := 0; i < pathsList.Len(); i++ {
			s, _ := starlarklib.AsString(pathsList.Index(i))
			paths = append(paths, s)
		}

		assertStrContains(t, paths, "b")                             // dir itself is yielded
		assertStrNotContains(t, paths, filepath.Join("b", "file.txt")) // children skipped
		assertStrContains(t, paths, filepath.Join("a", "file.txt"))
		assertStrContains(t, paths, filepath.Join("c", "file.txt"))
	})

	t.Run("stop walk", func(t *testing.T) {
		script := `
paths = []
def collect(entry):
    paths.append(entry.path)
    if entry.is_dir and entry.name == "b":
        return "stop"

file.walk_tree(root=ROOT, callback=collect, gitignore=False)
`
		globals := execStarlark(t, script, root)

		pathsList := globals["paths"].(*starlarklib.List)
		var paths []string
		for i := 0; i < pathsList.Len(); i++ {
			s, _ := starlarklib.AsString(pathsList.Index(i))
			paths = append(paths, s)
		}

		// Walk should have stopped at "b"
		assertStrContains(t, paths, "a")
		assertStrContains(t, paths, "b")
		assertStrNotContains(t, paths, "c")
	})
}

// =============================================================================
// Test: file.list with gitignore
// =============================================================================

func TestFileList_Gitignore(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitignore", "*.log\n")
	writeTestFile(t, root, "main.go", "package main")
	writeTestFile(t, root, "debug.log", "log")

	script := `
entries = file.list(ROOT)
names = [e.name for e in entries]
`
	globals := execStarlark(t, script, root)

	namesList := globals["names"].(*starlarklib.List)
	var names []string
	for i := 0; i < namesList.Len(); i++ {
		s, _ := starlarklib.AsString(namesList.Index(i))
		names = append(names, s)
	}

	assertStrContains(t, names, "main.go")
	assertStrNotContains(t, names, "debug.log")
}

// =============================================================================
// Helpers
// =============================================================================

func execStarlark(t *testing.T, script string, root string) starlarklib.StringDict {
	t.Helper()

	thread := &starlarklib.Thread{Name: "test"}

	predeclared := starlarklib.StringDict{
		"file": NewFileReceiver(&ui.Provider{Writer: os.Stderr}),
		"ROOT": starlarklib.String(root),
	}

	fileOpts := syntax.FileOptions{
		Set:             true,
		While:           true,
		TopLevelControl: true,
		GlobalReassign:  true,
		Recursion:       true,
	}

	globals, err := starlarklib.ExecFileOptions(&fileOpts, thread, "test.star", script, predeclared)
	if err != nil {
		t.Fatalf("Starlark exec error: %v", err)
	}

	return globals
}

func writeTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdirFile(t *testing.T, root, relPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, relPath), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertStrContains(t *testing.T, items []string, want string) {
	t.Helper()
	for _, item := range items {
		if item == want {
			return
		}
	}
	t.Errorf("expected %v to contain %q", items, want)
}

func assertStrNotContains(t *testing.T, items []string, notWant string) {
	t.Helper()
	for _, item := range items {
		if item == notWant {
			t.Errorf("expected %v to NOT contain %q", items, notWant)
			return
		}
	}
}
