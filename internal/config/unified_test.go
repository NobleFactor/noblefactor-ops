// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package config

import (
	"os"
	"path/filepath"
	"testing"

	"go.starlark.net/starlark"
)

func TestLoad(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}
	if cfg.builtin == nil {
		t.Error("Load() builtin should not be nil")
	}
	if cfg.extensions == nil {
		t.Error("Load() extensions should not be nil")
	}
}

func TestLoadWithSources(t *testing.T) {
	cfg, sources, err := LoadWithSources()
	if err != nil {
		t.Fatalf("LoadWithSources() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadWithSources() returned nil config")
	}
	if len(sources) == 0 {
		t.Error("LoadWithSources() should return at least one source")
	}

	// Should have builtin source
	hasBuiltin := false
	for _, s := range sources {
		if s.Path == "<builtin>" {
			hasBuiltin = true
			break
		}
	}
	if !hasBuiltin {
		t.Error("LoadWithSources() should include <builtin> source")
	}
}

func TestConfig_RegisterExtension(t *testing.T) {
	ClearTypeCache()
	defer ClearTypeCache()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	spec := ConfigSpec{
		Fields: map[string]string{
			"enabled": "bool",
			"name":    "string",
		},
		Defaults: map[string]interface{}{
			"enabled": true,
			"name":    "test",
		},
	}

	err = cfg.RegisterExtension("test.extension", spec)
	if err != nil {
		t.Fatalf("RegisterExtension() error = %v", err)
	}

	// Verify it was registered
	gotSpec, ok := cfg.GetSpec("test.extension")
	if !ok {
		t.Error("GetSpec() should return true after registration")
	}
	if gotSpec.Fields["enabled"] != "bool" {
		t.Errorf("GetSpec() fields mismatch, got %v", gotSpec.Fields)
	}
}

func TestConfig_GetSpec_NotFound(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	_, ok := cfg.GetSpec("nonexistent.path")
	if ok {
		t.Error("GetSpec() should return false for nonexistent path")
	}
}

func TestConfig_Sync(t *testing.T) {
	// Change to temp dir for test
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create a config with precommit hooks
	cfg := &Config{
		builtin: &builtinConfig{
			Precommit: PrecommitConfig{
				Hooks: []PrecommitHook{
					{
						ID:    "test-hook",
						Name:  "Test",
						Entry: "echo test",
					},
				},
			},
		},
		extensions: newExtensionsConfig("star.yaml"),
	}

	result, err := cfg.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if result.FilesGenerated != 1 {
		t.Errorf("Sync() FilesGenerated = %d, want 1", result.FilesGenerated)
	}
	if result.PrecommitConfig == "" {
		t.Error("Sync() should have generated pre-commit config")
	}
}

func TestConfig_ToStarlark(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	val := cfg.ToStarlark()
	if val == nil {
		t.Fatal("ToStarlark() returned nil")
	}

	// Should be a unifiedConfigValue
	ucv, ok := val.(*unifiedConfigValue)
	if !ok {
		t.Fatalf("ToStarlark() type = %T, want *unifiedConfigValue", val)
	}
	if ucv.config != cfg {
		t.Error("ToStarlark() should wrap the same config")
	}
}

func TestConfig_Builtin(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	builtin := cfg.Builtin()
	if builtin == nil {
		t.Fatal("Builtin() returned nil")
	}
	if builtin != cfg.builtin {
		t.Error("Builtin() should return the same builtin config")
	}
}

// =============================================================================
// unifiedConfigValue tests
// =============================================================================

func TestUnifiedConfigValue_String(t *testing.T) {
	cfg, _ := Load()
	v := cfg.ToStarlark().(*unifiedConfigValue)

	if got := v.String(); got != "config" {
		t.Errorf("String() = %q, want %q", got, "config")
	}
}

func TestUnifiedConfigValue_Type(t *testing.T) {
	cfg, _ := Load()
	v := cfg.ToStarlark().(*unifiedConfigValue)

	if got := v.Type(); got != "config" {
		t.Errorf("Type() = %q, want %q", got, "config")
	}
}

func TestUnifiedConfigValue_Freeze(t *testing.T) {
	cfg, _ := Load()
	v := cfg.ToStarlark().(*unifiedConfigValue)

	// Freeze should not panic
	v.Freeze()
}

func TestUnifiedConfigValue_Truth(t *testing.T) {
	cfg, _ := Load()
	v := cfg.ToStarlark().(*unifiedConfigValue)

	if got := v.Truth(); got != starlark.True {
		t.Errorf("Truth() = %v, want True", got)
	}
}

func TestUnifiedConfigValue_Hash(t *testing.T) {
	cfg, _ := Load()
	v := cfg.ToStarlark().(*unifiedConfigValue)

	hash, err := v.Hash()
	if err != nil {
		t.Errorf("Hash() error = %v", err)
	}
	if hash != 0 {
		t.Errorf("Hash() = %d, want 0", hash)
	}
}

func TestUnifiedConfigValue_Attr_Builtin(t *testing.T) {
	cfg, _ := Load()
	v := cfg.ToStarlark().(*unifiedConfigValue)

	// Access builtin "lint" attribute
	lintVal, err := v.Attr("lint")
	if err != nil {
		t.Fatalf("Attr('lint') error = %v", err)
	}
	if lintVal == nil {
		t.Error("Attr('lint') returned nil")
	}

	// Access builtin "precommit" attribute
	precommitVal, err := v.Attr("precommit")
	if err != nil {
		t.Fatalf("Attr('precommit') error = %v", err)
	}
	if precommitVal == nil {
		t.Error("Attr('precommit') returned nil")
	}
}

func TestUnifiedConfigValue_Attr_NotFound(t *testing.T) {
	cfg, _ := Load()
	v := cfg.ToStarlark().(*unifiedConfigValue)

	_, err := v.Attr("nonexistent")
	if err == nil {
		t.Error("Attr('nonexistent') should return error")
	}
	if _, ok := err.(starlark.NoSuchAttrError); !ok {
		t.Errorf("Attr() error type = %T, want NoSuchAttrError", err)
	}
}

func TestUnifiedConfigValue_Attr_Extension(t *testing.T) {
	ClearTypeCache()
	defer ClearTypeCache()

	cfg, _ := Load()

	// Register an extension
	spec := ConfigSpec{
		Fields: map[string]string{
			"enabled": "bool",
		},
		Defaults: map[string]interface{}{
			"enabled": true,
		},
	}
	cfg.RegisterExtension("myext", spec)

	v := cfg.ToStarlark().(*unifiedConfigValue)

	// Access extension attribute
	extVal, err := v.Attr("myext")
	if err != nil {
		t.Fatalf("Attr('myext') error = %v", err)
	}
	if extVal == nil {
		t.Error("Attr('myext') returned nil")
	}
}

func TestUnifiedConfigValue_Attr_ExtensionOverridesBuiltin(t *testing.T) {
	ClearTypeCache()
	defer ClearTypeCache()

	cfg, _ := Load()

	// Register an extension with the same name as a builtin
	spec := ConfigSpec{
		Fields: map[string]string{
			"custom": "string",
		},
		Defaults: map[string]interface{}{
			"custom": "from-extension",
		},
	}
	cfg.RegisterExtension("lint", spec)

	v := cfg.ToStarlark().(*unifiedConfigValue)

	// Access should return extension, not builtin
	lintVal, err := v.Attr("lint")
	if err != nil {
		t.Fatalf("Attr('lint') error = %v", err)
	}

	// The value should be from the extension (ConfigValue), not builtin (struct)
	if _, ok := lintVal.(*ConfigValue); !ok {
		t.Errorf("Attr('lint') should return extension ConfigValue, got %T", lintVal)
	}
}

func TestUnifiedConfigValue_AttrNames(t *testing.T) {
	cfg, _ := Load()
	v := cfg.ToStarlark().(*unifiedConfigValue)

	names := v.AttrNames()

	// Should contain builtin names
	hasLint := false
	hasPrecommit := false
	for _, name := range names {
		if name == "lint" {
			hasLint = true
		}
		if name == "precommit" {
			hasPrecommit = true
		}
	}

	if !hasLint {
		t.Error("AttrNames() should contain 'lint'")
	}
	if !hasPrecommit {
		t.Error("AttrNames() should contain 'precommit'")
	}
}

func TestUnifiedConfigValue_AttrNames_WithExtensions(t *testing.T) {
	ClearTypeCache()
	defer ClearTypeCache()

	cfg, _ := Load()

	// Register extensions
	spec := ConfigSpec{
		Fields: map[string]string{"enabled": "bool"},
	}
	cfg.RegisterExtension("myext", spec)
	cfg.RegisterExtension("another", spec)

	v := cfg.ToStarlark().(*unifiedConfigValue)
	names := v.AttrNames()

	// Should contain extension names
	hasMyext := false
	hasAnother := false
	for _, name := range names {
		if name == "myext" {
			hasMyext = true
		}
		if name == "another" {
			hasAnother = true
		}
	}

	if !hasMyext {
		t.Error("AttrNames() should contain 'myext'")
	}
	if !hasAnother {
		t.Error("AttrNames() should contain 'another'")
	}
}

func TestUnifiedConfigValue_AttrNames_NoDuplicates(t *testing.T) {
	ClearTypeCache()
	defer ClearTypeCache()

	cfg, _ := Load()

	// Register extension with same name as builtin
	spec := ConfigSpec{
		Fields: map[string]string{"enabled": "bool"},
	}
	cfg.RegisterExtension("lint", spec)

	v := cfg.ToStarlark().(*unifiedConfigValue)
	names := v.AttrNames()

	// Count occurrences of "lint"
	count := 0
	for _, name := range names {
		if name == "lint" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("AttrNames() has %d occurrences of 'lint', want 1", count)
	}
}

func TestUnifiedConfigValue_Attr_NilExtensions(t *testing.T) {
	// Create config with nil extensions
	cfg := &Config{
		builtin:    defaultBuiltinConfig(),
		extensions: nil,
	}

	v := &unifiedConfigValue{config: cfg}

	// Should still be able to access builtin
	lintVal, err := v.Attr("lint")
	if err != nil {
		t.Fatalf("Attr('lint') error = %v", err)
	}
	if lintVal == nil {
		t.Error("Attr('lint') returned nil")
	}
}

func TestUnifiedConfigValue_Attr_NilBuiltin(t *testing.T) {
	// Create config with nil builtin
	cfg := &Config{
		builtin:    nil,
		extensions: newExtensionsConfig("star.yaml"),
	}

	v := &unifiedConfigValue{config: cfg}

	// Should return error for builtin attrs
	_, err := v.Attr("lint")
	if err == nil {
		t.Error("Attr('lint') should error with nil builtin")
	}
}

func TestLoad_WithProjectConfig(t *testing.T) {
	// Save current dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Create temp dir with star.yaml
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Write a project config
	yamlContent := `
lint:
  go:
    path: "./custom/..."
`
	if err := os.WriteFile(filepath.Join(tmpDir, "star.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have merged the custom path
	if cfg.builtin.Lint.Go.Path != "./custom/..." {
		t.Errorf("Load() did not merge project config, got path = %q", cfg.builtin.Lint.Go.Path)
	}
}
