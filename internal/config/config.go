// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package config provides unified configuration for star commands.
// Configuration is loaded from a hierarchy of config.yaml files:
//  1. ${GIT_TOPLEVEL}/star/config.yaml (project - highest priority)
//  2. ${XDG_CONFIG_HOME}/star/config.yaml (user defaults)
//  3. Built-in defaults (hardcoded fallback)
package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
)

// gitWorkspaceRoot caches the git repository root path.
// Empty string means not in a git repo (or git not available).
var (
	gitWorkspaceRoot     string
	gitWorkspaceRootOnce sync.Once
	gitWorkspaceRootSet  bool // true if explicitly set (for testing)
)

// initGitWorkspaceRoot finds the git repository root once using go-git.
// Returns empty string if not in a git repo.
func initGitWorkspaceRoot() string {
	gitWorkspaceRootOnce.Do(func() {
		if gitWorkspaceRootSet {
			return // Already set by SetGitWorkspaceRoot
		}

		// Start from current directory and search up for .git
		cwd, err := os.Getwd()
		if err != nil {
			gitWorkspaceRoot = ""
			return
		}

		// PlainOpenWithOptions with DetectDotGit walks up the directory tree
		repo, err := git.PlainOpenWithOptions(cwd, &git.PlainOpenOptions{
			DetectDotGit: true,
		})
		if err != nil {
			gitWorkspaceRoot = ""
			return
		}

		// Get the worktree to find the root path
		wt, err := repo.Worktree()
		if err != nil {
			gitWorkspaceRoot = ""
			return
		}

		gitWorkspaceRoot = wt.Filesystem.Root()
	})
	return gitWorkspaceRoot
}

// GitWorkspaceRoot returns the cached git repository root.
// Returns empty string if not in a git repo.
func GitWorkspaceRoot() string {
	return initGitWorkspaceRoot()
}

// SetGitWorkspaceRoot sets the git workspace root for testing.
// Call ResetGitWorkspaceRoot to restore normal behavior.
func SetGitWorkspaceRoot(path string) {
	gitWorkspaceRoot = path
	gitWorkspaceRootSet = true
	gitWorkspaceRootOnce.Do(func() {}) // Mark as done
}

// ResetGitWorkspaceRoot resets the git workspace root cache.
// The next call to GitWorkspaceRoot will re-detect from git.
func ResetGitWorkspaceRoot() {
	gitWorkspaceRoot = ""
	gitWorkspaceRootSet = false
	gitWorkspaceRootOnce = sync.Once{}
}

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

// loadBuiltin loads builtin configuration from the hierarchy of config.yaml files.
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

	// Load project config (only if in a git repo)
	projectPath := projectConfigPath()
	if projectPath != "" {
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
// Returns empty string if not in a git repository.
func projectConfigPath() string {
	root := GitWorkspaceRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "star", "config.yaml")
}

// loadUserConfig loads the user's config.yaml if it exists.
func loadUserConfig() (*builtinConfig, error) {
	path := userConfigPath()
	if path == "" {
		return nil, nil
	}
	return loadBuiltinFile(path)
}

// loadProjectConfig loads the project's config.yaml if it exists.
// Returns nil if not in a git repository.
func loadProjectConfig() (*builtinConfig, error) {
	path := projectConfigPath()
	if path == "" {
		return nil, nil
	}
	return loadBuiltinFile(path)
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
