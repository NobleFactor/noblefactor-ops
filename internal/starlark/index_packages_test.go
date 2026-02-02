// SPDX-License-Identifier: MIT
// Copyright (c) 2025 Noble Factor. All rights reserved.

package starlark

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestIndexPackages tests the index-packages.star script.
func TestIndexPackages(t *testing.T) {
	// Create temp directory for test registry
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("create packages dir: %v", err)
	}

	// Create test fixtures
	fixtures := []struct {
		name      string
		lifecycle string
		variants  []string
		hasReadme bool
	}{
		{
			name: "git",
			lifecycle: `name: git
version: "2.43"
description: Distributed version control system
homepage: https://git-scm.com
license: GPL-2.0
maintainer: git@noblefactor.com
platforms:
  - Darwin
  - Linux
  - Windows
provides:
  - git
aliases:
  - version-control
tags:
  - vcs
  - development
signatures:
  homebrew:
    - git
  apt:
    - git
  dnf:
    - git
  pacman:
    - git
  winget:
    - Git.Git
`,
			variants:  []string{"Darwin", "Linux", "Windows"},
			hasReadme: true,
		},
		{
			name: "neovim",
			lifecycle: `name: neovim
version: "0.10"
description: Hyperextensible Vim-based text editor
homepage: https://neovim.io
license: Apache-2.0
maintainer: editors@noblefactor.com
platforms:
  - Darwin
  - Linux
provides:
  - nvim
aliases:
  - vim
  - editor
tags:
  - editor
  - development
signatures:
  homebrew:
    - neovim
  apt:
    - neovim
  dnf:
    - neovim
  pacman:
    - neovim
`,
			variants:  []string{"Darwin", "Linux"},
			hasReadme: false,
		},
		{
			name: "ripgrep",
			lifecycle: `name: ripgrep
version: "14.1"
description: Fast recursive grep alternative
homepage: https://github.com/BurntSushi/ripgrep
license: MIT
maintainer: tools@noblefactor.com
platforms:
  - Darwin
  - Linux
  - Windows
provides:
  - rg
aliases: []
tags:
  - search
  - cli
signatures:
  homebrew:
    - ripgrep
  apt:
    - ripgrep
  cargo:
    - ripgrep
`,
			variants:  []string{"Darwin", "Linux", "Windows"},
			hasReadme: true,
		},
	}

	// Create fixture directories and files
	for _, f := range fixtures {
		pkgDir := filepath.Join(packagesDir, f.name)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatalf("create package dir %s: %v", f.name, err)
		}

		// Write lifecycle.yaml
		lifecyclePath := filepath.Join(pkgDir, "lifecycle.yaml")
		if err := os.WriteFile(lifecyclePath, []byte(f.lifecycle), 0644); err != nil {
			t.Fatalf("write lifecycle.yaml for %s: %v", f.name, err)
		}

		// Create variant directories
		for _, variant := range f.variants {
			variantDir := filepath.Join(pkgDir, variant)
			if err := os.MkdirAll(variantDir, 0755); err != nil {
				t.Fatalf("create variant dir %s/%s: %v", f.name, variant, err)
			}
		}

		// Create README if needed
		if f.hasReadme {
			readmePath := filepath.Join(pkgDir, "README.md")
			if err := os.WriteFile(readmePath, []byte("# "+f.name), 0644); err != nil {
				t.Fatalf("write README.md for %s: %v", f.name, err)
			}
		}
	}

	// Load and run the index-packages script
	opsDir := filepath.Join(getRepoRoot(t), "ops")
	runtime := NewRuntime(opsDir)
	scriptPath := filepath.Join(opsDir, "devlore-registry", "index-packages.star")
	if err := runtime.Load(scriptPath); err != nil {
		t.Fatalf("load script: %v", err)
	}

	cmd, ok := runtime.Commands()["devlore-registry.index.packages"]
	if !ok {
		t.Fatal("command devlore-registry.index.packages not found")
	}

	// Run the command
	if err := cmd.Run(map[string]string{"path": tmpDir}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	// Verify index.yaml was created
	indexPath := filepath.Join(packagesDir, "index.yaml")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index.yaml: %v", err)
	}

	var index struct {
		Version   string `yaml:"version"`
		Generated string `yaml:"generated"`
		Count     int    `yaml:"count"`
		Packages  []struct {
			Name        string   `yaml:"name"`
			Version     string   `yaml:"version"`
			Description string   `yaml:"description"`
			Dir         string   `yaml:"dir"`
			HasReadme   bool     `yaml:"has_readme"`
			Variants    []string `yaml:"variants"`
			Platforms   []string `yaml:"platforms"`
			Provides    []string `yaml:"provides"`
			Aliases     []string `yaml:"aliases"`
			Tags        []string `yaml:"tags"`
		} `yaml:"packages"`
	}
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		t.Fatalf("unmarshal index.yaml: %v", err)
	}

	// Verify index properties
	t.Run("index_count", func(t *testing.T) {
		if index.Count != 3 {
			t.Errorf("expected count 3, got %d", index.Count)
		}
	})

	t.Run("index_packages_sorted", func(t *testing.T) {
		// Should be sorted alphabetically: git, neovim, ripgrep
		expected := []string{"git", "neovim", "ripgrep"}
		for i, exp := range expected {
			if index.Packages[i].Name != exp {
				t.Errorf("expected package[%d]=%s, got %s", i, exp, index.Packages[i].Name)
			}
		}
	})

	t.Run("index_has_readme_correct", func(t *testing.T) {
		// git=true, neovim=false, ripgrep=true
		expectedReadme := map[string]bool{"git": true, "neovim": false, "ripgrep": true}
		for _, pkg := range index.Packages {
			if pkg.HasReadme != expectedReadme[pkg.Name] {
				t.Errorf("package %s has_readme: expected %v, got %v", pkg.Name, expectedReadme[pkg.Name], pkg.HasReadme)
			}
		}
	})

	t.Run("index_variants_correct", func(t *testing.T) {
		expectedVariants := map[string]int{"git": 3, "neovim": 2, "ripgrep": 3}
		for _, pkg := range index.Packages {
			if len(pkg.Variants) != expectedVariants[pkg.Name] {
				t.Errorf("package %s variants: expected %d, got %d", pkg.Name, expectedVariants[pkg.Name], len(pkg.Variants))
			}
		}
	})

	t.Run("index_no_signatures", func(t *testing.T) {
		// Index should not contain signatures (they go in cross-reference.yaml)
		// We verify this by checking the raw YAML doesn't contain "signatures"
		if containsString(string(indexData), "signatures:") {
			t.Error("index.yaml should not contain signatures")
		}
	})

	// Verify cross-reference.yaml was created
	xrefPath := filepath.Join(packagesDir, "cross-reference.yaml")
	xrefData, err := os.ReadFile(xrefPath)
	if err != nil {
		t.Fatalf("read cross-reference.yaml: %v", err)
	}

	var xref map[string]map[string]string
	if err := yaml.Unmarshal(xrefData, &xref); err != nil {
		t.Fatalf("unmarshal cross-reference.yaml: %v", err)
	}

	t.Run("xref_managers", func(t *testing.T) {
		// Should have: apt, cargo, dnf, homebrew, pacman, winget
		expectedManagers := []string{"apt", "cargo", "dnf", "homebrew", "pacman", "winget"}
		for _, mgr := range expectedManagers {
			if _, ok := xref[mgr]; !ok {
				t.Errorf("missing manager %s in cross-reference", mgr)
			}
		}
	})

	t.Run("xref_homebrew_mappings", func(t *testing.T) {
		// homebrew: git→git, neovim→neovim, ripgrep→ripgrep
		homebrew := xref["homebrew"]
		expected := map[string]string{"git": "git", "neovim": "neovim", "ripgrep": "ripgrep"}
		for native, lore := range expected {
			if homebrew[native] != lore {
				t.Errorf("homebrew[%s]: expected %s, got %s", native, lore, homebrew[native])
			}
		}
	})

	t.Run("xref_winget_mapping", func(t *testing.T) {
		// winget: Git.Git→git
		if xref["winget"]["Git.Git"] != "git" {
			t.Errorf("winget[Git.Git]: expected git, got %s", xref["winget"]["Git.Git"])
		}
	})

	t.Run("xref_cargo_mapping", func(t *testing.T) {
		// cargo: ripgrep→ripgrep
		if xref["cargo"]["ripgrep"] != "ripgrep" {
			t.Errorf("cargo[ripgrep]: expected ripgrep, got %s", xref["cargo"]["ripgrep"])
		}
	})
}

// TestIndexPackagesEmptySignatures tests that packages without signatures don't cause errors.
func TestIndexPackagesEmptySignatures(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("create packages dir: %v", err)
	}

	// Package with no signatures
	pkgDir := filepath.Join(packagesDir, "custom-tool")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("create package dir: %v", err)
	}

	lifecycle := `name: custom-tool
version: "1.0"
description: A custom tool with no package manager signatures
platforms:
  - Darwin
`
	if err := os.WriteFile(filepath.Join(pkgDir, "lifecycle.yaml"), []byte(lifecycle), 0644); err != nil {
		t.Fatalf("write lifecycle.yaml: %v", err)
	}

	// Load and run
	opsDir := filepath.Join(getRepoRoot(t), "ops")
	runtime := NewRuntime(opsDir)
	if err := runtime.Load(filepath.Join(opsDir, "devlore-registry", "index-packages.star")); err != nil {
		t.Fatalf("load script: %v", err)
	}

	cmd := runtime.Commands()["devlore-registry.index.packages"]
	if err := cmd.Run(map[string]string{"path": tmpDir}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	// index.yaml should exist with 1 package
	indexData, err := os.ReadFile(filepath.Join(packagesDir, "index.yaml"))
	if err != nil {
		t.Fatalf("read index.yaml: %v", err)
	}

	var index struct {
		Count int `yaml:"count"`
	}
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if index.Count != 1 {
		t.Errorf("expected count 1, got %d", index.Count)
	}

	// cross-reference.yaml should NOT exist (no mappings)
	xrefPath := filepath.Join(packagesDir, "cross-reference.yaml")
	if _, err := os.Stat(xrefPath); !os.IsNotExist(err) {
		t.Error("cross-reference.yaml should not exist when there are no signatures")
	}
}

// TestIndexPackagesSkipInvalid tests that invalid packages are skipped gracefully.
func TestIndexPackagesSkipInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("create packages dir: %v", err)
	}

	// Valid package
	validDir := filepath.Join(packagesDir, "valid-pkg")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("create valid dir: %v", err)
	}
	validLifecycle := `name: valid-pkg
version: "1.0"
description: A valid package
platforms:
  - Darwin
`
	if err := os.WriteFile(filepath.Join(validDir, "lifecycle.yaml"), []byte(validLifecycle), 0644); err != nil {
		t.Fatalf("write valid lifecycle: %v", err)
	}

	// Directory without lifecycle.yaml (should be skipped)
	noLifecycleDir := filepath.Join(packagesDir, "no-lifecycle")
	if err := os.MkdirAll(noLifecycleDir, 0755); err != nil {
		t.Fatalf("create no-lifecycle dir: %v", err)
	}

	// Run indexer
	opsDir := filepath.Join(getRepoRoot(t), "ops")
	runtime := NewRuntime(opsDir)
	if err := runtime.Load(filepath.Join(opsDir, "devlore-registry", "index-packages.star")); err != nil {
		t.Fatalf("load script: %v", err)
	}

	cmd := runtime.Commands()["devlore-registry.index.packages"]
	if err := cmd.Run(map[string]string{"path": tmpDir}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	// Should have indexed only the valid package
	indexData, err := os.ReadFile(filepath.Join(packagesDir, "index.yaml"))
	if err != nil {
		t.Fatalf("read index.yaml: %v", err)
	}

	var index struct {
		Count    int `yaml:"count"`
		Packages []struct {
			Name string `yaml:"name"`
		} `yaml:"packages"`
	}
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if index.Count != 1 {
		t.Errorf("expected count 1, got %d", index.Count)
	}
	if len(index.Packages) != 1 || index.Packages[0].Name != "valid-pkg" {
		t.Errorf("expected only valid-pkg, got %v", index.Packages)
	}
}

// TestIndexPackagesConflictingSignatures tests warning when two packages map to same native name.
func TestIndexPackagesConflictingSignatures(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("create packages dir: %v", err)
	}

	// Two packages that both claim "python" in apt
	pkg1Dir := filepath.Join(packagesDir, "python3")
	if err := os.MkdirAll(pkg1Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg1Dir, "lifecycle.yaml"), []byte(`name: python3
version: "3.12"
description: Python 3
platforms: [Linux]
signatures:
  apt:
    - python3
`), 0644); err != nil {
		t.Fatal(err)
	}

	pkg2Dir := filepath.Join(packagesDir, "python-legacy")
	if err := os.MkdirAll(pkg2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg2Dir, "lifecycle.yaml"), []byte(`name: python-legacy
version: "2.7"
description: Python 2 (legacy)
platforms: [Linux]
signatures:
  apt:
    - python3
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Run indexer - should not fail, but one will "win"
	opsDir := filepath.Join(getRepoRoot(t), "ops")
	runtime := NewRuntime(opsDir)
	if err := runtime.Load(filepath.Join(opsDir, "devlore-registry", "index-packages.star")); err != nil {
		t.Fatalf("load script: %v", err)
	}

	cmd := runtime.Commands()["devlore-registry.index.packages"]
	// Command should complete without error (warning is logged but not fatal)
	if err := cmd.Run(map[string]string{"path": tmpDir}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	// cross-reference.yaml should exist with the mapping (last one wins due to iteration order)
	xrefData, err := os.ReadFile(filepath.Join(packagesDir, "cross-reference.yaml"))
	if err != nil {
		t.Fatalf("read cross-reference.yaml: %v", err)
	}

	var xref map[string]map[string]string
	if err := yaml.Unmarshal(xrefData, &xref); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// python3 should map to one of them (we don't care which, just that it's deterministic)
	if xref["apt"]["python3"] == "" {
		t.Error("apt[python3] mapping should exist")
	}
}

// getRepoRoot returns the repository root path for test fixtures.
func getRepoRoot(t *testing.T) string {
	t.Helper()
	// Find repo root by looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
