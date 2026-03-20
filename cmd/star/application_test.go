// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// =============================================================================
// Test: LoadExtensions
// =============================================================================

func TestApplication_LoadExtensions(t *testing.T) {
	t.Run("missing extensions directory is not an error", func(t *testing.T) {
		r := NewApplication()

		err := r.LoadExtensionsFrom("/nonexistent/path/to/extensions")
		if err != nil {
			t.Errorf("LoadExtensionsFrom() error = %v, want nil for missing dir", err)
		}
	})

	t.Run("empty extensions directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "extensions")
		if err := os.MkdirAll(extDir, 0755); err != nil {
			t.Fatal(err)
		}

		r := NewApplication()

		err := r.LoadExtensionsFrom(extDir)
		if err != nil {
			t.Errorf("LoadExtensionsFrom() error = %v, want nil", err)
		}
	})

	t.Run("discovers and registers extensions", func(t *testing.T) {
		// Clear global registry first
		extension.Clear()
		defer extension.Clear()

		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "extensions")

		// Create test extension with new directory structure
		testExtDir := filepath.Join(extDir, "com.example.TestCmd")
		cmdDir := filepath.Join(testExtDir, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write extension.yaml with new schema
		extYAML := `extension: com.example.TestCmd
description: Test command extension
receivers:
  - name: test
    builtin: true
    type: TestReceiver
commands:
  - name: test.cmd
    help: A test command
    implementation: commands/test-cmd.star
    flags:
      - name: verbose
        type: bool
        default: "false"
        help: Enable verbose output
`
		if err := os.WriteFile(filepath.Join(testExtDir, "extension.yaml"), []byte(extYAML), 0644); err != nil {
			t.Fatal(err)
		}

		// Write minimal .star file in commands/ subdirectory
		starContent := `def run(ctx):
    pass
`
		if err := os.WriteFile(filepath.Join(cmdDir, "test-cmd.star"), []byte(starContent), 0644); err != nil {
			t.Fatal(err)
		}

		r := NewApplication()

		err := r.LoadExtensionsFrom(extDir)
		if err != nil {
			t.Fatalf("LoadExtensionsFrom() error = %v", err)
		}

		// Verify extension was registered
		spec := extension.Get("com.example.TestCmd")
		if spec == nil {
			t.Error("expected extension 'com.example.TestCmd' to be registered")
		}

		// Verify command was loaded
		cmds := r.Commands()
		if _, ok := cmds["test cmd"]; !ok {
			t.Errorf("expected command 'test cmd' to be registered, got keys: %v", keys(cmds))
		}
	})

	t.Run("registers extension config", func(t *testing.T) {
		extension.Clear()
		defer extension.Clear()

		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "extensions")
		testExtDir := filepath.Join(extDir, "com.example.WithConfig")
		cmdDir := filepath.Join(testExtDir, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Extension with config (requires at least one command for validation)
		extYAML := `extension: com.example.WithConfig
description: Extension with config
commands:
  - name: cfg.test
    help: Config test command
    implementation: commands/cfg-test.star
config:
  type: TestConfig
  fields:
    enabled: bool
    path: string
  defaults:
    enabled: true
    path: "."
`
		if err := os.WriteFile(filepath.Join(testExtDir, "extension.yaml"), []byte(extYAML), 0644); err != nil {
			t.Fatal(err)
		}

		// Write minimal .star file
		starContent := `def run(ctx):
    pass
`
		if err := os.WriteFile(filepath.Join(cmdDir, "cfg-test.star"), []byte(starContent), 0644); err != nil {
			t.Fatal(err)
		}

		r := NewApplication()

		err := r.LoadExtensionsFrom(extDir)
		if err != nil {
			t.Fatalf("LoadExtensionsFrom() error = %v", err)
		}

		// Verify config was registered at the config path (derived from command name)
		cfg := r.Config()
		if cfg == nil {
			t.Fatal("expected config to be initialized")
		}

		spec, ok := cfg.GetSpec("cfg.test")
		if !ok {
			t.Error("expected config spec 'cfg.test' to be registered (derived from single command name)")
		}
		if spec.Fields["enabled"] != "bool" {
			t.Errorf("expected field 'enabled' to be 'bool', got %q", spec.Fields["enabled"])
		}
	})
}

// =============================================================================
// Test: loadExtensionCommands
// =============================================================================

func TestApplication_loadExtensionCommands(t *testing.T) {
	t.Run("command name transformation", func(t *testing.T) {
		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "com.example.LintCopyright")
		cmdDir := filepath.Join(extDir, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write .star file in commands/ subdirectory
		starContent := `def run(ctx):
    pass
`
		starPath := filepath.Join(cmdDir, "lint-copyright.star")
		if err := os.WriteFile(starPath, []byte(starContent), 0644); err != nil {
			t.Fatal(err)
		}

		spec := &extension.ExtensionSpec{
			Extension:  "com.example.LintCopyright",
			SourcePath: filepath.Join(extDir, "extension.yaml"),
			Commands: []extension.CommandSpec{
				{
					Name:           "lint.copyright",
					Help:           "Check copyright headers",
					Implementation: "commands/lint-copyright.star",
				},
			},
		}

		r := NewApplication()
		err := r.loadExtensionCommands(spec)
		if err != nil {
			t.Fatalf("loadExtensionCommands() error = %v", err)
		}

		// Verify command registered with space-separated name
		cmds := r.Commands()
		if _, ok := cmds["lint copyright"]; !ok {
			t.Errorf("expected command 'lint copyright', got keys: %v", keys(cmds))
		}
	})

	t.Run("applies flag defaults from spec", func(t *testing.T) {
		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "com.example.TestExt")
		cmdDir := filepath.Join(extDir, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Command just needs a run function - flags come from extension.yaml
		starContent := `def run(ctx):
    pass
`
		if err := os.WriteFile(filepath.Join(cmdDir, "test.star"), []byte(starContent), 0644); err != nil {
			t.Fatal(err)
		}

		spec := &extension.ExtensionSpec{
			Extension:  "com.example.TestExt",
			SourcePath: filepath.Join(extDir, "extension.yaml"),
			Commands: []extension.CommandSpec{
				{
					Name:           "test.ext",
					Help:           "Test extension",
					Implementation: "commands/test.star",
					Flags: []extension.FlagSpec{
						{Name: "fix", Type: "bool", Default: "false", Help: "Fix issues"},
						{Name: "path", Type: "string", Default: ".", Help: "Path to check"},
					},
				},
			},
		}

		r := NewApplication()
		err := r.loadExtensionCommands(spec)
		if err != nil {
			t.Fatalf("loadExtensionCommands() error = %v", err)
		}

		cmd := r.Commands()["test ext"]
		if cmd == nil {
			t.Fatal("command not found")
		}

		// Verify flags were applied
		flagsByName := make(map[string]Flag)
		for _, f := range cmd.Flags {
			flagsByName[f.Name] = f
		}

		if f, ok := flagsByName["fix"]; !ok {
			t.Error("missing 'fix' flag")
		} else {
			if f.Default != "false" {
				t.Errorf("fix.Default = %q, want %q", f.Default, "false")
			}
			if f.Help != "Fix issues" {
				t.Errorf("fix.Help = %q, want %q", f.Help, "Fix issues")
			}
		}

		// path flag should be added from spec
		if f, ok := flagsByName["path"]; !ok {
			t.Error("missing 'path' flag from spec")
		} else if f.Default != "." {
			t.Errorf("path.Default = %q, want %q", f.Default, ".")
		}
	})

	t.Run("no commands is no-op", func(t *testing.T) {
		r := NewApplication()
		spec := &extension.ExtensionSpec{
			Extension: "com.example.NoCommand",
			Commands:  nil,
		}

		err := r.loadExtensionCommands(spec)
		if err != nil {
			t.Errorf("loadExtensionCommands() error = %v, want nil", err)
		}
	})

	t.Run("empty implementation is skipped", func(t *testing.T) {
		r := NewApplication()
		spec := &extension.ExtensionSpec{
			Extension: "com.example.EmptyImpl",
			Commands: []extension.CommandSpec{
				{
					Name:           "empty.impl",
					Help:           "Empty impl",
					Implementation: "",
				},
			},
		}

		err := r.loadExtensionCommands(spec)
		if err != nil {
			t.Errorf("loadExtensionCommands() error = %v, want nil", err)
		}
	})

	t.Run("multiple commands loaded", func(t *testing.T) {
		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "com.example.MultiCmd")
		cmdDir := filepath.Join(extDir, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write two .star files - each just needs a run function
		star1 := `def run(ctx):
    pass
`
		star2 := `def run(ctx):
    pass
`
		if err := os.WriteFile(filepath.Join(cmdDir, "one.star"), []byte(star1), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cmdDir, "two.star"), []byte(star2), 0644); err != nil {
			t.Fatal(err)
		}

		spec := &extension.ExtensionSpec{
			Extension:  "com.example.MultiCmd",
			SourcePath: filepath.Join(extDir, "extension.yaml"),
			Commands: []extension.CommandSpec{
				{Name: "multi.one", Help: "First", Implementation: "commands/one.star"},
				{Name: "multi.two", Help: "Second", Implementation: "commands/two.star"},
			},
		}

		r := NewApplication()
		err := r.loadExtensionCommands(spec)
		if err != nil {
			t.Fatalf("loadExtensionCommands() error = %v", err)
		}

		cmds := r.Commands()
		if _, ok := cmds["multi one"]; !ok {
			t.Errorf("expected command 'multi one', got keys: %v", keys(cmds))
		}
		if _, ok := cmds["multi two"]; !ok {
			t.Errorf("expected command 'multi two', got keys: %v", keys(cmds))
		}
	})
}

// =============================================================================
// Test: Config
// =============================================================================

func TestApplication_Config(t *testing.T) {
	r := NewApplication()

	// Initially nil
	if r.config != nil {
		t.Error("expected config to be nil initially")
	}

	// First call initializes
	cfg := r.Config()
	if cfg == nil {
		t.Fatal("Config() returned nil")
	}

	// Second call returns same instance
	cfg2 := r.Config()
	if cfg != cfg2 {
		t.Error("Config() should return same instance")
	}
}

// =============================================================================
// Test: buildPredeclared
// =============================================================================

func TestApplication_buildPredeclared(t *testing.T) {
	r := NewApplication()

	predeclared := r.buildPredeclared(nil)

	// Verify essential modules are present
	requiredModules := []string{
		"file", "json", "yaml", "goast", "shellcheck",
		"lint", "regexp", "config", "setup",
		"commands", "ui",
		"starindex", "starcomplexity", "starstats", "staranalysis",
	}

	for _, name := range requiredModules {
		if _, ok := predeclared[name]; !ok {
			t.Errorf("buildPredeclared() missing module %q", name)
		}
	}
}

// ConfigReceiver.get test removed — config is now a framework provider.

// =============================================================================
// Helpers
// =============================================================================

func keys[K comparable, V any](m map[K]V) []K {
	result := make([]K, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
