// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package config provides unified configuration for star commands.
// Configuration is loaded from a hierarchy of config.yaml files:
//  1. ./star/config.yaml (project - highest priority)
//  2. ${XDG_CONFIG_HOME}/star/config.yaml (user defaults)
//  3. Built-in defaults (hardcoded fallback)
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// builtinConfig is the top-level configuration structure for star commands.
// This is private - consumers should use the unified Config type.
type builtinConfig struct {
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
	Go        GoLintConfig        `yaml:"go"`
	Shell     ShellLintConfig     `yaml:"shell"`
	Markdown  MarkdownLintConfig  `yaml:"markdown"`
	Copyright CopyrightLintConfig `yaml:"copyright"`
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

// CopyrightLintConfig configures star lint copyright.
type CopyrightLintConfig struct {
	Enabled  bool                        `yaml:"enabled"`
	License  string                      `yaml:"license"`  // SPDX identifier or "auto"
	Holder   string                      `yaml:"holder"`   // Copyright holder name
	Patterns map[string]CopyrightPattern `yaml:"patterns"` // lang -> pattern config
	Exclude  []string                    `yaml:"exclude"`  // Glob patterns to exclude
}

// CopyrightPattern defines match and replace patterns for copyright headers.
type CopyrightPattern struct {
	Match   string `yaml:"match"`   // Regex to match existing headers (permissive)
	Replace string `yaml:"replace"` // Canonical form to use
}

// defaultBuiltinConfig returns the built-in default configuration.
func defaultBuiltinConfig() *builtinConfig {
	return &builtinConfig{
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
			Copyright: CopyrightLintConfig{
				Enabled: false, // Disabled by default until configured
				License: "auto",
				Holder:  "",
				Patterns: map[string]CopyrightPattern{
					"go": {
						Match:   `// SPDX-License-Identifier: \S+\s*\n// Copyright.*All rights reserved\.`,
						Replace: "// SPDX-License-Identifier: {license}\n// Copyright {holder}. All rights reserved.",
					},
					"star": {
						Match:   `# SPDX-License-Identifier: \S+\s*\n# Copyright.*All rights reserved\.`,
						Replace: "# SPDX-License-Identifier: {license}\n# Copyright {holder}. All rights reserved.",
					},
					"shell": {
						Match:   `# SPDX-License-Identifier: \S+\s*\n# Copyright.*All rights reserved\.`,
						Replace: "# SPDX-License-Identifier: {license}\n# Copyright {holder}. All rights reserved.",
					},
				},
				Exclude: []string{"**/testdata/**", "**/vendor/**"},
			},
		},
	}
}

// loadBuiltin loads builtin configuration from the hierarchy of star.yaml files.
// Project config overrides user config, which overrides defaults.
func loadBuiltin() (*builtinConfig, error) {
	cfg := defaultBuiltinConfig()

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

// loadBuiltinWithSources loads builtin configuration and returns the source of each file.
func loadBuiltinWithSources() (*builtinConfig, []ConfigSource, error) {
	cfg := defaultBuiltinConfig()
	var sources []ConfigSource

	sources = append(sources, ConfigSource{
		Path:   "<builtin>",
		Exists: true,
	})

	// Load user config
	userPath := userConfigPath()
	if userPath != "" {
		userCfg, err := loadBuiltinFile(userPath)
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
	projectCfg, err := loadBuiltinFile(projectPath)
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

// userConfigPath returns the path to the user's config.yaml.
func userConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "star", "config.yaml")
}

// projectConfigPath returns the path to the project's config.yaml.
func projectConfigPath() string {
	return filepath.Join("star", "config.yaml")
}

// loadUserConfig loads the user's star.yaml if it exists.
func loadUserConfig() (*builtinConfig, error) {
	path := userConfigPath()
	if path == "" {
		return nil, nil
	}
	return loadBuiltinFile(path)
}

// loadProjectConfig loads the project's star.yaml if it exists.
func loadProjectConfig() (*builtinConfig, error) {
	return loadBuiltinFile(projectConfigPath())
}

// loadBuiltinFile loads a builtin config file, returning nil if it doesn't exist.
func loadBuiltinFile(path string) (*builtinConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg builtinConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
