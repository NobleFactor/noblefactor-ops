// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

// Package config provides unified configuration for star commands.
// Configuration is loaded from a hierarchy of star.yaml files:
//  1. ./star.yaml (project - highest priority)
//  2. ${XDG_CONFIG_HOME}/star/star.yaml (user defaults)
//  3. Built-in defaults (hardcoded fallback)
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure for star commands.
type Config struct {
	Lint      LintConfig      `yaml:"lint"`
	Precommit PrecommitConfig `yaml:"precommit"`
}

// PrecommitConfig configures .pre-commit-config.yaml generation.
type PrecommitConfig struct {
	Hooks []PrecommitHook `yaml:"hooks"`
}

// PrecommitHook defines a single pre-commit hook.
type PrecommitHook struct {
	ID            string   `yaml:"id"`
	Name          string   `yaml:"name"`
	Entry         string   `yaml:"entry"`
	Language      string   `yaml:"language"`
	PassFilenames bool     `yaml:"pass_filenames"`
	Types         []string `yaml:"types"`
	Stages        []string `yaml:"stages"`
}

// LintConfig contains configuration for all lint commands.
type LintConfig struct {
	Go       GoLintConfig       `yaml:"go"`
	Shell    ShellLintConfig    `yaml:"shell"`
	Markdown MarkdownLintConfig `yaml:"markdown"`
}

// GoLintConfig configures star lint go.
type GoLintConfig struct {
	Path        string                 `yaml:"path"`
	SkipModTidy bool                   `yaml:"skip_mod_tidy"`
	Config      map[string]interface{} `yaml:"config"` // golangci-lint config
}

// ShellLintConfig configures star lint shell.
type ShellLintConfig struct {
	Path     string `yaml:"path"`
	Severity string `yaml:"severity"` // error, warning, info, style
	Indent   int    `yaml:"indent"`
}

// MarkdownLintConfig configures star lint markdown.
type MarkdownLintConfig struct {
	Path        string                 `yaml:"path"`
	Exclude     []string               `yaml:"exclude"`
	Config      map[string]interface{} `yaml:"config"` // markdownlint rules
	Frontmatter FrontmatterConfig      `yaml:"frontmatter"`
}

// FrontmatterConfig defines required/optional frontmatter fields.
type FrontmatterConfig struct {
	Required []string `yaml:"required"`
	Optional []string `yaml:"optional"`
}

// DefaultConfig returns the built-in default configuration.
func DefaultConfig() *Config {
	return &Config{
		Lint: LintConfig{
			Go: GoLintConfig{
				Path:        "./...",
				SkipModTidy: false,
			},
			Shell: ShellLintConfig{
				Path:     ".",
				Severity: "warning",
				Indent:   4,
			},
			Markdown: MarkdownLintConfig{
				Path:    ".",
				Exclude: []string{"node_modules", "vendor", ".git"},
				Frontmatter: FrontmatterConfig{
					Required: []string{"title", "description"},
					Optional: []string{},
				},
			},
		},
	}
}

// Load loads configuration from the hierarchy of star.yaml files.
// Project config overrides user config, which overrides defaults.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load user config from XDG_CONFIG_HOME
	userCfg, err := loadUserConfig()
	if err != nil {
		return nil, err
	}
	if userCfg != nil {
		cfg = merge(cfg, userCfg)
	}

	// Load project config from current directory
	projectCfg, err := loadProjectConfig()
	if err != nil {
		return nil, err
	}
	if projectCfg != nil {
		cfg = merge(cfg, projectCfg)
	}

	return cfg, nil
}

// LoadWithSources loads configuration and returns the source of each file.
func LoadWithSources() (*Config, []ConfigSource, error) {
	cfg := DefaultConfig()
	var sources []ConfigSource

	sources = append(sources, ConfigSource{
		Path:   "<builtin>",
		Exists: true,
	})

	// Load user config
	userPath := userConfigPath()
	if userPath != "" {
		userCfg, err := loadFile(userPath)
		if err != nil {
			return nil, nil, err
		}
		sources = append(sources, ConfigSource{
			Path:   userPath,
			Exists: userCfg != nil,
		})
		if userCfg != nil {
			cfg = merge(cfg, userCfg)
		}
	}

	// Load project config
	projectPath := projectConfigPath()
	projectCfg, err := loadFile(projectPath)
	if err != nil {
		return nil, nil, err
	}
	sources = append(sources, ConfigSource{
		Path:   projectPath,
		Exists: projectCfg != nil,
	})
	if projectCfg != nil {
		cfg = merge(cfg, projectCfg)
	}

	return cfg, sources, nil
}

// ConfigSource describes a configuration file location and whether it exists.
type ConfigSource struct {
	Path   string
	Exists bool
}

// userConfigPath returns the path to the user's star.yaml.
func userConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "star", "star.yaml")
}

// projectConfigPath returns the path to the project's star.yaml.
func projectConfigPath() string {
	return "star.yaml"
}

// loadUserConfig loads the user's star.yaml if it exists.
func loadUserConfig() (*Config, error) {
	path := userConfigPath()
	if path == "" {
		return nil, nil
	}
	return loadFile(path)
}

// loadProjectConfig loads the project's star.yaml if it exists.
func loadProjectConfig() (*Config, error) {
	return loadFile(projectConfigPath())
}

// loadFile loads a config file, returning nil if it doesn't exist.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
