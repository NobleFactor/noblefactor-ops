// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// =============================================================================
// Test: applyFlagDefaults
// =============================================================================

func TestApplyFlagDefaults(t *testing.T) {
	tests := []struct {
		name         string
		cmdFlags     []Flag
		specFlags    []extension.FlagSpec
		wantFlags    []Flag
		wantFlagsCnt int
	}{
		{
			name:     "empty spec flags - no changes",
			cmdFlags: []Flag{{Name: "verbose", Default: "true"}},
			specFlags: []extension.FlagSpec{},
			wantFlags: []Flag{{Name: "verbose", Default: "true"}},
			wantFlagsCnt: 1,
		},
		{
			name:     "merge default into empty command flag",
			cmdFlags: []Flag{{Name: "fix", Default: ""}},
			specFlags: []extension.FlagSpec{
				{Name: "fix", Default: "false", Help: "Fix issues"},
			},
			wantFlags: []Flag{{Name: "fix", Default: "false", Help: "Fix issues"}},
			wantFlagsCnt: 1,
		},
		{
			name:     "preserve existing command flag default",
			cmdFlags: []Flag{{Name: "fix", Default: "true", Help: "Already set"}},
			specFlags: []extension.FlagSpec{
				{Name: "fix", Default: "false", Help: "Spec help"},
			},
			wantFlags: []Flag{{Name: "fix", Default: "true", Help: "Already set"}},
			wantFlagsCnt: 1,
		},
		{
			name:     "add new flags from spec",
			cmdFlags: []Flag{{Name: "existing"}},
			specFlags: []extension.FlagSpec{
				{Name: "new-flag", Default: "value", Help: "New flag from spec"},
			},
			wantFlags: []Flag{
				{Name: "existing"},
				{Name: "new-flag", Default: "value", Help: "New flag from spec"},
			},
			wantFlagsCnt: 2,
		},
		{
			name:     "merge and add flags",
			cmdFlags: []Flag{{Name: "fix", Default: ""}},
			specFlags: []extension.FlagSpec{
				{Name: "fix", Default: "false", Help: "Fix issues"},
				{Name: "path", Default: ".", Help: "Path to check"},
			},
			wantFlags: []Flag{
				{Name: "fix", Default: "false", Help: "Fix issues"},
				{Name: "path", Default: ".", Help: "Path to check"},
			},
			wantFlagsCnt: 2,
		},
		{
			name:     "only update help when default already set",
			cmdFlags: []Flag{{Name: "verbose", Default: "true", Help: ""}},
			specFlags: []extension.FlagSpec{
				{Name: "verbose", Default: "false", Help: "Enable verbose output"},
			},
			wantFlags: []Flag{{Name: "verbose", Default: "true", Help: "Enable verbose output"}},
			wantFlagsCnt: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRuntime("ops")
			cmd := &Command{Flags: tt.cmdFlags}
			spec := &extension.ExtensionSpec{Flags: tt.specFlags}

			r.applyFlagDefaults(cmd, spec)

			if len(cmd.Flags) != tt.wantFlagsCnt {
				t.Errorf("applyFlagDefaults() flag count = %d, want %d", len(cmd.Flags), tt.wantFlagsCnt)
			}

			// Build map for easier comparison
			flagsByName := make(map[string]Flag)
			for _, f := range cmd.Flags {
				flagsByName[f.Name] = f
			}

			for _, want := range tt.wantFlags {
				got, ok := flagsByName[want.Name]
				if !ok {
					t.Errorf("applyFlagDefaults() missing flag %q", want.Name)
					continue
				}
				if got.Default != want.Default {
					t.Errorf("flag %q Default = %q, want %q", want.Name, got.Default, want.Default)
				}
				if got.Help != want.Help {
					t.Errorf("flag %q Help = %q, want %q", want.Name, got.Help, want.Help)
				}
			}
		})
	}
}

// =============================================================================
// Test: LoadExtensions
// =============================================================================

func TestRuntime_LoadExtensions(t *testing.T) {
	t.Run("missing extensions directory is not an error", func(t *testing.T) {
		r := NewRuntime("ops")
		r.SetExtensionsDir("/nonexistent/path/to/extensions")

		err := r.LoadExtensions()
		if err != nil {
			t.Errorf("LoadExtensions() error = %v, want nil for missing dir", err)
		}
	})

	t.Run("empty extensions directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "extensions")
		if err := os.MkdirAll(extDir, 0755); err != nil {
			t.Fatal(err)
		}

		r := NewRuntime("ops")
		r.SetExtensionsDir(extDir)

		err := r.LoadExtensions()
		if err != nil {
			t.Errorf("LoadExtensions() error = %v, want nil", err)
		}
	})

	t.Run("discovers and registers extensions", func(t *testing.T) {
		// Clear global registry first
		extension.Clear()
		defer extension.Clear()

		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "extensions")

		// Create test extension
		testExtDir := filepath.Join(extDir, "test-cmd")
		if err := os.MkdirAll(testExtDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write extension.yaml
		extYAML := `extension: test.cmd
description: Test command extension
receivers:
  - name: test
    builtin: true
    type: TestReceiver
command:
  help: A test command
  implementation: test-cmd.star
flags:
  - name: verbose
    type: bool
    default: "false"
    help: Enable verbose output
`
		if err := os.WriteFile(filepath.Join(testExtDir, "extension.yaml"), []byte(extYAML), 0644); err != nil {
			t.Fatal(err)
		}

		// Write minimal .star file
		starContent := `command(
    name = "test.cmd",
    help = "Test command",
    run = lambda ctx: None,
)
`
		if err := os.WriteFile(filepath.Join(testExtDir, "test-cmd.star"), []byte(starContent), 0644); err != nil {
			t.Fatal(err)
		}

		r := NewRuntime("ops")
		r.SetExtensionsDir(extDir)

		err := r.LoadExtensions()
		if err != nil {
			t.Fatalf("LoadExtensions() error = %v", err)
		}

		// Verify extension was registered
		spec := extension.Get("test.cmd")
		if spec == nil {
			t.Error("expected extension 'test.cmd' to be registered")
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
		testExtDir := filepath.Join(extDir, "with-config")
		if err := os.MkdirAll(testExtDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Extension with config
		extYAML := `extension: with.config
description: Extension with config
receivers:
  - name: cfg
    builtin: true
    type: ConfigReceiver
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

		r := NewRuntime("ops")
		r.SetExtensionsDir(extDir)

		err := r.LoadExtensions()
		if err != nil {
			t.Fatalf("LoadExtensions() error = %v", err)
		}

		// Verify config was registered
		cfg := r.Config()
		if cfg == nil {
			t.Fatal("expected config to be initialized")
		}

		spec, ok := cfg.GetSpec("with.config")
		if !ok {
			t.Error("expected config spec 'with.config' to be registered")
		}
		if spec.Fields["enabled"] != "bool" {
			t.Errorf("expected field 'enabled' to be 'bool', got %q", spec.Fields["enabled"])
		}
	})
}

// =============================================================================
// Test: loadExtensionCommand
// =============================================================================

func TestRuntime_loadExtensionCommand(t *testing.T) {
	t.Run("command name transformation", func(t *testing.T) {
		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "lint-copyright")
		if err := os.MkdirAll(extDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write .star file
		starContent := `command(
    name = "lint.copyright",
    help = "Check copyright headers",
    run = lambda ctx: None,
)
`
		starPath := filepath.Join(extDir, "lint-copyright.star")
		if err := os.WriteFile(starPath, []byte(starContent), 0644); err != nil {
			t.Fatal(err)
		}

		spec := &extension.ExtensionSpec{
			Extension:  "lint.copyright",
			SourcePath: filepath.Join(extDir, "extension.yaml"),
			Command: &extension.CommandSpec{
				Help:           "Check copyright headers",
				Implementation: "lint-copyright.star",
			},
		}

		r := NewRuntime("ops")
		err := r.loadExtensionCommand(spec)
		if err != nil {
			t.Fatalf("loadExtensionCommand() error = %v", err)
		}

		// Verify command registered with space-separated name
		cmds := r.Commands()
		if _, ok := cmds["lint copyright"]; !ok {
			t.Errorf("expected command 'lint copyright', got keys: %v", keys(cmds))
		}
	})

	t.Run("applies flag defaults from spec", func(t *testing.T) {
		tmpDir := t.TempDir()
		extDir := filepath.Join(tmpDir, "test-ext")
		if err := os.MkdirAll(extDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Command that doesn't define default for 'fix' flag
		starContent := `command(
    name = "test.ext",
    help = "Test extension",
    flags = [
        {"name": "fix"},
    ],
    run = lambda ctx: None,
)
`
		if err := os.WriteFile(filepath.Join(extDir, "test.star"), []byte(starContent), 0644); err != nil {
			t.Fatal(err)
		}

		spec := &extension.ExtensionSpec{
			Extension:  "test.ext",
			SourcePath: filepath.Join(extDir, "extension.yaml"),
			Command: &extension.CommandSpec{
				Help:           "Test extension",
				Implementation: "test.star",
			},
			Flags: []extension.FlagSpec{
				{Name: "fix", Type: "bool", Default: "false", Help: "Fix issues"},
				{Name: "path", Type: "string", Default: ".", Help: "Path to check"},
			},
		}

		r := NewRuntime("ops")
		err := r.loadExtensionCommand(spec)
		if err != nil {
			t.Fatalf("loadExtensionCommand() error = %v", err)
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

	t.Run("nil command spec is no-op", func(t *testing.T) {
		r := NewRuntime("ops")
		spec := &extension.ExtensionSpec{
			Extension: "no.command",
			Command:   nil,
		}

		err := r.loadExtensionCommand(spec)
		if err != nil {
			t.Errorf("loadExtensionCommand() error = %v, want nil", err)
		}
	})

	t.Run("empty implementation is no-op", func(t *testing.T) {
		r := NewRuntime("ops")
		spec := &extension.ExtensionSpec{
			Extension: "empty.impl",
			Command: &extension.CommandSpec{
				Help:           "Empty impl",
				Implementation: "",
			},
		}

		err := r.loadExtensionCommand(spec)
		if err != nil {
			t.Errorf("loadExtensionCommand() error = %v, want nil", err)
		}
	})
}

// =============================================================================
// Test: SetExtensionsDir and ExtensionsConfig
// =============================================================================

func TestRuntime_SetExtensionsDir(t *testing.T) {
	r := NewRuntime("ops")

	// Default value
	if r.extensionsDir != "extensions" {
		t.Errorf("default extensionsDir = %q, want %q", r.extensionsDir, "extensions")
	}

	// Set custom value
	r.SetExtensionsDir("/custom/path")
	if r.extensionsDir != "/custom/path" {
		t.Errorf("extensionsDir = %q, want %q", r.extensionsDir, "/custom/path")
	}
}

func TestRuntime_Config(t *testing.T) {
	r := NewRuntime("ops")

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

func TestRuntime_buildPredeclared(t *testing.T) {
	r := NewRuntime("ops")
	collector := &commandCollector{commands: make(map[string]*Command)}

	predeclared := r.buildPredeclared(collector, nil)

	// Verify essential modules are present
	requiredModules := []string{
		"fs", "json", "yaml", "schema", "go", "shell",
		"lint", "copyright", "config", "setup", "starlark_parse",
		"command", "note", "warn", "error", "success", "fail",
	}

	for _, name := range requiredModules {
		if _, ok := predeclared[name]; !ok {
			t.Errorf("buildPredeclared() missing module %q", name)
		}
	}
}

// =============================================================================
// Test: configGet
// =============================================================================

func TestRuntime_configGet(t *testing.T) {
	t.Run("returns unified config", func(t *testing.T) {
		r := NewRuntime("ops")

		val, err := r.configGet(nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("configGet() error = %v", err)
		}

		// Should return a unified config value
		if val == nil {
			t.Error("configGet() returned nil")
		}
		if val.Type() != "config" {
			t.Errorf("configGet() type = %q, want %q", val.Type(), "config")
		}
	})
}

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
