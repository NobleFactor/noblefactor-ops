// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	// Create extension directories
	ext1Dir := filepath.Join(dir, "lint-go")
	ext2Dir := filepath.Join(dir, "lint-shell")

	if err := os.MkdirAll(ext1Dir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.MkdirAll(ext2Dir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Write extension specs
	ext1YAML := `
extension: lint.go
description: "Go linting"
command:
  help: "Run Go linters"
`
	ext2YAML := `
extension: lint.shell
description: "Shell linting"
command:
  help: "Run shell linters"
`
	if err := os.WriteFile(filepath.Join(ext1Dir, "extension.yaml"), []byte(ext1YAML), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ext2Dir, "extension.yaml"), []byte(ext2YAML), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	specs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(specs) != 2 {
		t.Errorf("len(specs) = %d, want 2", len(specs))
	}

	// Check both extensions were found
	names := make(map[string]bool)
	for _, spec := range specs {
		names[spec.Extension] = true
	}

	if !names["lint.go"] {
		t.Error("lint.go not found")
	}
	if !names["lint.shell"] {
		t.Error("lint.shell not found")
	}
}

func TestDiscover_YmlExtension(t *testing.T) {
	dir := t.TempDir()

	extDir := filepath.Join(dir, "test-ext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	yaml := `
extension: test
command:
  help: "Test"
`
	// Use .yml extension
	if err := os.WriteFile(filepath.Join(extDir, "extension.yml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	specs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(specs) != 1 {
		t.Errorf("len(specs) = %d, want 1", len(specs))
	}
}

func TestDiscover_SkipsInvalid(t *testing.T) {
	dir := t.TempDir()

	// Valid extension
	validDir := filepath.Join(dir, "valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	validYAML := `
extension: valid
command:
  help: "Valid"
`
	if err := os.WriteFile(filepath.Join(validDir, "extension.yaml"), []byte(validYAML), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Invalid extension (no command or receivers)
	invalidDir := filepath.Join(dir, "invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	invalidYAML := `
extension: invalid
description: "No command or receivers"
`
	if err := os.WriteFile(filepath.Join(invalidDir, "extension.yaml"), []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	specs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Should only find the valid one
	if len(specs) != 1 {
		t.Errorf("len(specs) = %d, want 1", len(specs))
	}
	if len(specs) > 0 && specs[0].Extension != "valid" {
		t.Errorf("Extension = %q, want %q", specs[0].Extension, "valid")
	}
}

func TestDiscover_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	specs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(specs) != 0 {
		t.Errorf("len(specs) = %d, want 0", len(specs))
	}
}

func TestDiscover_NonexistentDir(t *testing.T) {
	_, err := Discover("/nonexistent/path/12345")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestDiscoverOne(t *testing.T) {
	dir := t.TempDir()

	yaml := `
extension: test.example
description: "Test"
command:
  help: "Test command"
`
	if err := os.WriteFile(filepath.Join(dir, "extension.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	spec, err := DiscoverOne(dir)
	if err != nil {
		t.Fatalf("DiscoverOne failed: %v", err)
	}

	if spec.Extension != "test.example" {
		t.Errorf("Extension = %q, want %q", spec.Extension, "test.example")
	}
}

func TestDiscoverOne_NoSpec(t *testing.T) {
	dir := t.TempDir()

	_, err := DiscoverOne(dir)
	if err == nil {
		t.Error("expected error for directory without extension.yaml")
	}
}

func TestLoadAll(t *testing.T) {
	// Clear registry first
	Clear()

	dir := t.TempDir()

	extDir := filepath.Join(dir, "test-ext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	yaml := `
extension: loadall.test
command:
  help: "Test"
`
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	count, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Verify it was registered
	if Get("loadall.test") == nil {
		t.Error("extension not registered")
	}
}

func TestLoadAll_SkipsNonexistent(t *testing.T) {
	Clear()

	dir := t.TempDir()

	extDir := filepath.Join(dir, "ext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	yaml := `
extension: exists
command:
  help: "Test"
`
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Include a nonexistent directory - should be skipped
	count, err := LoadAll("/nonexistent/path/12345", dir)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestLoadAll_SkipsDuplicates(t *testing.T) {
	Clear()

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	yaml := `
extension: duplicate
command:
  help: "Test"
`
	// Same extension in both directories
	ext1 := filepath.Join(dir1, "dup")
	ext2 := filepath.Join(dir2, "dup")

	if err := os.MkdirAll(ext1, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.MkdirAll(ext2, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(ext1, "extension.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ext2, "extension.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	count, err := LoadAll(dir1, dir2)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	// Should only count the first one
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestDefaultSearchPaths(t *testing.T) {
	paths := DefaultSearchPaths()

	if len(paths) < 2 {
		t.Errorf("len(DefaultSearchPaths()) = %d, want >= 2", len(paths))
	}

	// First should be project-local
	if paths[0] != "extensions" {
		t.Errorf("paths[0] = %q, want %q", paths[0], "extensions")
	}
}

func TestFindExtensionDir(t *testing.T) {
	dir := t.TempDir()

	// Create extension directory with hyphenated name
	extDir := filepath.Join(dir, "lint-copyright")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	yaml := `
extension: lint.copyright
command:
  help: "Test"
`
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Temporarily override search paths by searching in our temp dir
	// This is a limitation - FindExtensionDir uses DefaultSearchPaths
	// For now, just test that it doesn't panic and returns expected error
	_, err := FindExtensionDir("nonexistent.extension")
	if err == nil {
		t.Error("expected error for nonexistent extension")
	}
}

func TestLoadDefaults(t *testing.T) {
	Clear()

	// LoadDefaults searches default paths which likely don't exist in test
	// Just verify it doesn't panic and returns without error
	count, err := LoadDefaults()
	if err != nil {
		t.Fatalf("LoadDefaults failed: %v", err)
	}

	// Count may be 0 if no extensions in default paths
	_ = count
}

func TestLoadAll_StatError(t *testing.T) {
	Clear()

	// Test with a file (not directory) - should skip
	tmpFile := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	count, err := LoadAll(tmpFile)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	// Should skip the file
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestFindExtensionDir_Found(t *testing.T) {
	// Create a temp directory structure that matches DefaultSearchPaths
	dir := t.TempDir()

	// Create extensions/lint-copyright/extension.yaml
	extDir := filepath.Join(dir, "extensions", "lint-copyright")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	yaml := `
extension: lint.copyright
command:
  help: "Test"
`
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Change to temp dir so "extensions" is found
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer os.Chdir(oldWd)

	foundDir, err := FindExtensionDir("lint.copyright")
	if err != nil {
		t.Fatalf("FindExtensionDir failed: %v", err)
	}

	if !strings.Contains(foundDir, "lint-copyright") {
		t.Errorf("foundDir = %q, want path containing lint-copyright", foundDir)
	}
}

func TestFindExtensionDir_FoundYml(t *testing.T) {
	dir := t.TempDir()

	// Create extensions/test-ext/extension.yml (note .yml not .yaml)
	extDir := filepath.Join(dir, "extensions", "test-ext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	yaml := `
extension: test.ext
command:
  help: "Test"
`
	if err := os.WriteFile(filepath.Join(extDir, "extension.yml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer os.Chdir(oldWd)

	foundDir, err := FindExtensionDir("test.ext")
	if err != nil {
		t.Fatalf("FindExtensionDir failed: %v", err)
	}

	if !strings.Contains(foundDir, "test-ext") {
		t.Errorf("foundDir = %q, want path containing test-ext", foundDir)
	}
}

func TestDiscover_WalkError(t *testing.T) {
	// Test when WalkDir encounters an error during traversal
	dir := t.TempDir()

	// Create a subdirectory that we can't read
	subDir := filepath.Join(dir, "unreadable")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Create extension in unreadable dir
	extDir := filepath.Join(subDir, "ext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	yaml := `
extension: test
command:
  help: "Test"
`
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Make parent unreadable (only on Unix-like systems)
	if err := os.Chmod(subDir, 0000); err != nil {
		t.Skip("cannot test permission error on this platform")
	}
	defer os.Chmod(subDir, 0755)

	_, err := Discover(dir)
	if err == nil {
		t.Error("expected error for unreadable directory")
	}
}

func TestLoadAll_DiscoverError(t *testing.T) {
	Clear()

	dir := t.TempDir()

	// Create directory then make it unreadable
	extDir := filepath.Join(dir, "ext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Make it unreadable
	if err := os.Chmod(extDir, 0000); err != nil {
		t.Skip("cannot test permission error on this platform")
	}
	defer os.Chmod(extDir, 0755)

	_, err := LoadAll(dir)
	if err == nil {
		t.Error("expected error when Discover fails")
	}
}
