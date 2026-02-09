// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package config

import (
	"os"
	"strings"
	"testing"
)

func TestSyncPrecommitConfig(t *testing.T) {
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

	cfg := &builtinConfig{
		Precommit: PrecommitConfig{
			Hooks: []PrecommitHook{
				{
					ID:            "star-lint-all",
					Name:          "Star quality gate",
					Entry:         "star lint all --",
					Language:      "system",
					PassFilenames: true,
					Types:         []string{"file"},
				},
			},
		},
	}

	result, err := cfg.Sync()
	if err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	if result.PrecommitConfig != ".pre-commit-config.yaml" {
		t.Errorf("PrecommitConfig = %q, want %q", result.PrecommitConfig, ".pre-commit-config.yaml")
	}
	if result.FilesGenerated != 1 {
		t.Errorf("FilesGenerated = %d, want 1", result.FilesGenerated)
	}

	// Verify file content
	data, err := os.ReadFile(".pre-commit-config.yaml")
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	content := string(data)

	// Check header
	if !strings.HasPrefix(content, generatedHeader) {
		t.Error("file should start with generated header")
	}

	// Check structure
	if !strings.Contains(content, "repo: local") {
		t.Error("file should contain 'repo: local'")
	}
	if !strings.Contains(content, "id: star-lint-all") {
		t.Error("file should contain hook id")
	}
	if !strings.Contains(content, "entry: star lint all --") {
		t.Error("file should contain hook entry")
	}
}

func TestSyncPrecommitConfigDefaultLanguage(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Config with empty language - should default to "system"
	cfg := &builtinConfig{
		Precommit: PrecommitConfig{
			Hooks: []PrecommitHook{
				{
					ID:    "test-hook",
					Name:  "Test hook",
					Entry: "test",
					// Language deliberately empty
				},
			},
		},
	}

	_, err = cfg.Sync()
	if err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	data, err := os.ReadFile(".pre-commit-config.yaml")
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if !strings.Contains(string(data), "language: system") {
		t.Error("empty language should default to 'system'")
	}
}

func TestClean(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create a generated file
	generated := generatedHeader + "repos: []\n"
	if err := os.WriteFile(".pre-commit-config.yaml", []byte(generated), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a non-generated file (should not be deleted)
	manual := "# My custom config\nrepos: []\n"
	if err := os.WriteFile(".golangci.yaml", []byte(manual), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Clean(); err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	// Generated file should be removed
	if _, err := os.Stat(".pre-commit-config.yaml"); !os.IsNotExist(err) {
		t.Error(".pre-commit-config.yaml should have been removed")
	}

	// Non-generated file should remain
	if _, err := os.Stat(".golangci.yaml"); err != nil {
		t.Error(".golangci.yaml should not have been removed")
	}
}

func TestEnsureGitignore(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// No existing .gitignore
	if err := EnsureGitignore(); err != nil {
		t.Fatalf("EnsureGitignore() error: %v", err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	content := string(data)

	// Should contain all three entries
	expected := []string{".golangci.yaml", ".markdownlint-cli2.yaml", ".pre-commit-config.yaml"}
	for _, entry := range expected {
		if !strings.Contains(content, entry) {
			t.Errorf(".gitignore should contain %q", entry)
		}
	}
}

func TestEnsureGitignoreExisting(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Existing .gitignore with one of the entries
	existing := "node_modules/\n.golangci.yaml\n"
	if err := os.WriteFile(".gitignore", []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignore(); err != nil {
		t.Fatalf("EnsureGitignore() error: %v", err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	content := string(data)

	// Original content should be preserved
	if !strings.Contains(content, "node_modules/") {
		t.Error(".gitignore should preserve existing entries")
	}

	// Should not duplicate .golangci.yaml
	if strings.Count(content, ".golangci.yaml") > 1 {
		t.Error(".gitignore should not duplicate existing entries")
	}

	// Should add missing entries
	if !strings.Contains(content, ".markdownlint-cli2.yaml") {
		t.Error(".gitignore should add .markdownlint-cli2.yaml")
	}
	if !strings.Contains(content, ".pre-commit-config.yaml") {
		t.Error(".gitignore should add .pre-commit-config.yaml")
	}
}

func TestSyncNoHooks(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Empty config - should not generate anything
	cfg := &builtinConfig{}

	result, err := cfg.Sync()
	if err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	if result.FilesGenerated != 0 {
		t.Errorf("FilesGenerated = %d, want 0", result.FilesGenerated)
	}
	if result.PrecommitConfig != "" {
		t.Errorf("PrecommitConfig = %q, want empty", result.PrecommitConfig)
	}

	// No file should be created
	if _, err := os.Stat(".pre-commit-config.yaml"); !os.IsNotExist(err) {
		t.Error(".pre-commit-config.yaml should not exist")
	}
}

func TestContainsLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		entry   string
		want    bool
	}{
		{"exact match only", ".golangci.yaml", ".golangci.yaml", true},
		{"first line", ".golangci.yaml\nother", ".golangci.yaml", true},
		{"middle line", "first\n.golangci.yaml\nlast", ".golangci.yaml", true},
		{"last line", "first\n.golangci.yaml", ".golangci.yaml", true},
		{"not found", "other\nstuff", ".golangci.yaml", false},
		{"partial match is not found", ".golangci.yaml.bak", ".golangci.yaml", false},
		{"empty content", "", ".golangci.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsLine(tt.content, tt.entry)
			if got != tt.want {
				t.Errorf("containsLine(%q, %q) = %v, want %v", tt.content, tt.entry, got, tt.want)
			}
		})
	}
}
