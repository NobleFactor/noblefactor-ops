// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package copyright

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetector_DetectByExtension(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	detector := NewDetector(cfg)

	tests := []struct {
		path     string
		wantLang string
		wantMeth string
	}{
		{"main.go", "go", "extension"},
		{"script.py", "python", "extension"},
		{"config.yaml", "yaml", "extension"},
		{"app.ts", "typescript", "extension"},
		{"lib.rs", "rust", "extension"},
		{"main.c", "c", "extension"},
		{"style.css", "css", "extension"},
		{"query.sql", "sql", "extension"},
		{"init.lua", "lua", "extension"},
		{"module.hs", "haskell", "extension"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := detector.Detect(tt.path)
			if result.Language != tt.wantLang {
				t.Errorf("Detect(%q) language = %q, want %q", tt.path, result.Language, tt.wantLang)
			}
			if result.Method != tt.wantMeth {
				t.Errorf("Detect(%q) method = %q, want %q", tt.path, result.Method, tt.wantMeth)
			}
		})
	}
}

func TestDetector_DetectByFilename(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	detector := NewDetector(cfg)

	tests := []struct {
		path     string
		wantLang string
	}{
		{"Makefile", "shell"},
		{"Dockerfile", "shell"},
		{"Jenkinsfile", "groovy"},
		{"Rakefile", "ruby"},
		{"Gemfile", "ruby"},
		{".emacs", "lisp"},
		{".vimrc", "vim"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := detector.Detect(tt.path)
			if result.Language != tt.wantLang {
				t.Errorf("Detect(%q) language = %q, want %q", tt.path, result.Language, tt.wantLang)
			}
			if result.Method != "filename" {
				t.Errorf("Detect(%q) method = %q, want 'filename'", tt.path, result.Method)
			}
		})
	}
}

func TestDetector_DetectByShebang(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	detector := NewDetector(cfg)

	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		wantLang string
	}{
		{
			name:     "bash script",
			content:  "#!/bin/bash\necho hello",
			wantLang: "shell",
		},
		{
			name:     "env bash",
			content:  "#!/usr/bin/env bash\necho hello",
			wantLang: "shell",
		},
		{
			name:     "python script",
			content:  "#!/usr/bin/python3\nprint('hello')",
			wantLang: "python",
		},
		{
			name:     "env python",
			content:  "#!/usr/bin/env python\nprint('hello')",
			wantLang: "python",
		},
		{
			name:     "ruby script",
			content:  "#!/usr/bin/ruby\nputs 'hello'",
			wantLang: "ruby",
		},
		{
			name:     "node script",
			content:  "#!/usr/bin/env node\nconsole.log('hello')",
			wantLang: "javascript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write file without extension
			path := filepath.Join(tmpDir, "script_"+tt.name)
			if err := os.WriteFile(path, []byte(tt.content), 0755); err != nil {
				t.Fatal(err)
			}

			result := detector.Detect(path)
			if result.Language != tt.wantLang {
				t.Errorf("Detect shebang %q = %q, want %q", tt.name, result.Language, tt.wantLang)
			}
			if result.Method != "shebang" {
				t.Errorf("Detect shebang %q method = %q, want 'shebang'", tt.name, result.Method)
			}
		})
	}
}

func TestDetector_DetectUnknown(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	detector := NewDetector(cfg)

	result := detector.Detect("unknown.xyz")
	if result.Language != "" {
		t.Errorf("expected empty language for unknown file, got %q", result.Language)
	}
	if result.Method != "" {
		t.Errorf("expected empty method for unknown file, got %q", result.Method)
	}
}

func TestDetector_SupportedLanguages(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	detector := NewDetector(cfg)

	langs := detector.SupportedLanguages()
	if len(langs) < 40 {
		t.Errorf("expected at least 40 supported languages, got %d", len(langs))
	}

	// Check some expected languages
	langMap := make(map[string]bool)
	for _, l := range langs {
		langMap[l] = true
	}

	expected := []string{"go", "python", "javascript", "rust", "java", "c", "cpp", "shell", "ruby"}
	for _, e := range expected {
		if !langMap[e] {
			t.Errorf("expected language %q to be supported", e)
		}
	}
}
