// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package copyright

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatalf("loadDefaults() error = %v", err)
	}

	// Verify defaults
	if cfg.Defaults.License != "auto" {
		t.Errorf("expected license 'auto', got %q", cfg.Defaults.License)
	}

	// Verify some languages are loaded
	if len(cfg.Languages) == 0 {
		t.Error("expected languages to be loaded")
	}

	// Check Go language definition
	goLang := cfg.GetLanguage("go")
	if goLang == nil {
		t.Fatal("expected 'go' language to be defined")
	}

	if len(goLang.Extensions) == 0 {
		t.Error("expected go language to have extensions")
	}

	if goLang.Comment.Line == nil || *goLang.Comment.Line != "//" {
		t.Error("expected go language to have // line comment")
	}
}

func TestLoadConfig_WithProjectOverride(t *testing.T) {
	// Create temp directory with project config
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Write project config
	projectConfig := `
copyright:
  defaults:
    license: "MIT"
    holder: "Test Corp"
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".star.yaml"), []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify overrides are applied
	if cfg.Defaults.License != "MIT" {
		t.Errorf("expected license 'MIT', got %q", cfg.Defaults.License)
	}
	if cfg.Defaults.Holder != "Test Corp" {
		t.Errorf("expected holder 'Test Corp', got %q", cfg.Defaults.Holder)
	}

	// Verify defaults are still present (not overwritten)
	if len(cfg.Languages) == 0 {
		t.Error("expected default languages to still be present")
	}
}

func TestMergeConfig(t *testing.T) {
	base := &Config{
		Defaults: Defaults{
			License: "auto",
			Holder:  "Base Holder",
			Include: []string{"**/*"},
			Exclude: []string{".git/**"},
		},
		Languages: map[string]Language{
			"go": {Extensions: []string{".go"}},
		},
	}

	overlay := &Config{
		Defaults: Defaults{
			License: "MIT",
			Exclude: []string{"vendor/**"},
		},
		Languages: map[string]Language{
			"custom": {Extensions: []string{".custom"}},
		},
	}

	result := mergeConfig(base, overlay)

	// License should be overwritten
	if result.Defaults.License != "MIT" {
		t.Errorf("expected license 'MIT', got %q", result.Defaults.License)
	}

	// Holder should be preserved from base
	if result.Defaults.Holder != "Base Holder" {
		t.Errorf("expected holder 'Base Holder', got %q", result.Defaults.Holder)
	}

	// Exclude should be merged
	if len(result.Defaults.Exclude) != 2 {
		t.Errorf("expected 2 exclude patterns, got %d", len(result.Defaults.Exclude))
	}

	// Base language should be preserved
	if _, ok := result.Languages["go"]; !ok {
		t.Error("expected 'go' language to be preserved")
	}

	// Overlay language should be added
	if _, ok := result.Languages["custom"]; !ok {
		t.Error("expected 'custom' language to be added")
	}
}
