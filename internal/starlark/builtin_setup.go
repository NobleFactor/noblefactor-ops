// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package starlark

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/NobleFactor/noblefactor-ops/internal/cli"
	"github.com/NobleFactor/noblefactor-ops/internal/config"
)

// setupModule returns the setup module with repository setup operations.
func setupModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "setup",
		Members: starlark.StringDict{
			"tools":             starlark.NewBuiltin("setup.tools", setupTools),
			"precommit_install": starlark.NewBuiltin("setup.precommit_install", setupPrecommitInstall),
			"precommit_check":   starlark.NewBuiltin("setup.precommit_check", setupPrecommitCheck),
			"init_config":       starlark.NewBuiltin("setup.init_config", setupInitConfig),
		},
	}
}

// =============================================================================
// TOOL INFORMATION
// =============================================================================

// DevTool describes a development tool with platform-specific install info.
type DevTool struct {
	Name        string
	Binary      string
	Description string
	DocsURL     string
	Install     map[string]string // platform -> install command
}

// devTools lists all tools needed for NobleFactor development.
var devTools = []DevTool{
	{
		Name:        "golangci-lint",
		Binary:      "golangci-lint",
		Description: "Go linter aggregator",
		DocsURL:     "https://golangci-lint.run/usage/install/",
		Install: map[string]string{
			"darwin": "brew install golangci-lint",
			"linux":  "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin",
		},
	},
	{
		Name:        "shellcheck",
		Binary:      "shellcheck",
		Description: "Shell script static analysis",
		DocsURL:     "https://github.com/koalaman/shellcheck#installing",
		Install: map[string]string{
			"darwin": "brew install shellcheck",
			"linux":  "sudo apt-get install shellcheck",
		},
	},
	{
		Name:        "shfmt",
		Binary:      "shfmt",
		Description: "Shell script formatter",
		DocsURL:     "https://github.com/mvdan/sh",
		Install: map[string]string{
			"darwin": "brew install shfmt",
			"linux":  "go install mvdan.cc/sh/v3/cmd/shfmt@latest",
		},
	},
	{
		Name:        "markdownlint-cli2",
		Binary:      "markdownlint-cli2",
		Description: "Markdown linter",
		DocsURL:     "https://github.com/DavidAnson/markdownlint-cli2",
		Install: map[string]string{
			"darwin": "brew install markdownlint-cli2",
			"linux":  "npm install -g markdownlint-cli2",
		},
	},
	{
		Name:        "pre-commit",
		Binary:      "pre-commit",
		Description: "Git pre-commit hook manager",
		DocsURL:     "https://pre-commit.com/#install",
		Install: map[string]string{
			"darwin": "brew install pre-commit",
			"linux":  "pip install pre-commit",
		},
	},
}

// setupTools returns information about all required development tools.
// Output is structured for consumption by lore or other tools.
//
// Returns a struct with:
//   - tools: List of tool info structs
//   - all_installed: True if all tools are installed
//   - missing_count: Number of missing tools
func setupTools(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("setup.tools", args, kwargs); err != nil {
		return nil, err
	}

	var toolList []starlark.Value
	allInstalled := true
	missingCount := 0
	platform := runtime.GOOS

	for _, tool := range devTools {
		path, _ := exec.LookPath(tool.Binary)
		installed := path != ""
		if !installed {
			allInstalled = false
			missingCount++
		}

		// Build platform-specific install commands dict
		installDict := starlark.NewDict(len(tool.Install))
		for plat, cmd := range tool.Install {
			_ = installDict.SetKey(starlark.String(plat), starlark.String(cmd))
		}

		// Get install command for current platform
		installCmd := tool.Install[platform]
		if installCmd == "" {
			installCmd = fmt.Sprintf("See %s", tool.DocsURL)
		}

		toolList = append(toolList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":        starlark.String(tool.Name),
			"binary":      starlark.String(tool.Binary),
			"description": starlark.String(tool.Description),
			"docs_url":    starlark.String(tool.DocsURL),
			"installed":   starlark.Bool(installed),
			"path":        starlark.String(path),
			"install":     installDict,
			"install_cmd": starlark.String(installCmd), // Current platform
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"tools":         starlark.NewList(toolList),
		"all_installed": starlark.Bool(allInstalled),
		"missing_count": starlark.MakeInt(missingCount),
		"platform":      starlark.String(platform),
	}), nil
}

// =============================================================================
// PRE-COMMIT HOOKS
// =============================================================================

// setupPrecommitCheck checks if pre-commit hooks are installed.
// Returns a struct with:
//   - installed: True if hooks are installed
//   - config_exists: True if .pre-commit-config.yaml exists
//   - precommit_available: True if pre-commit command is available
func setupPrecommitCheck(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("setup.precommit_check", args, kwargs); err != nil {
		return nil, err
	}

	// Check if pre-commit is available
	precommitPath, _ := exec.LookPath("pre-commit")
	precommitAvailable := precommitPath != ""

	// Check if config exists
	configExists := false
	if _, err := os.Stat(".pre-commit-config.yaml"); err == nil {
		configExists = true
	}

	// Check if hooks are installed
	hooksInstalled := false
	hookPath := ".git/hooks/pre-commit"
	if info, err := os.Stat(hookPath); err == nil && info.Size() > 0 {
		// Read first line to check if it's a pre-commit hook
		content, err := os.ReadFile(hookPath)
		if err == nil && len(content) > 0 {
			// pre-commit hooks typically have "pre-commit" in them
			hooksInstalled = filepath.Ext(hookPath) == "" && len(content) > 100
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"installed":            starlark.Bool(hooksInstalled),
		"config_exists":        starlark.Bool(configExists),
		"precommit_available":  starlark.Bool(precommitAvailable),
		"precommit_path":       starlark.String(precommitPath),
	}), nil
}

// setupPrecommitInstall installs pre-commit hooks if config exists.
// Returns a struct with:
//   - success: True if hooks were installed
//   - message: Status message
//   - already_installed: True if hooks were already installed
func setupPrecommitInstall(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("setup.precommit_install", args, kwargs); err != nil {
		return nil, err
	}

	// Check if config exists
	if _, err := os.Stat(".pre-commit-config.yaml"); os.IsNotExist(err) {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(false),
			"message":           starlark.String("No .pre-commit-config.yaml found"),
			"already_installed": starlark.Bool(false),
		}), nil
	}

	// Check if pre-commit is available
	precommitPath, err := exec.LookPath("pre-commit")
	if err != nil {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(false),
			"message":           starlark.String("pre-commit not installed. Run: star setup tools"),
			"already_installed": starlark.Bool(false),
		}), nil
	}

	// Check if already installed
	hookPath := ".git/hooks/pre-commit"
	if _, err := os.Stat(hookPath); err == nil {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(true),
			"message":           starlark.String("Pre-commit hooks already installed"),
			"already_installed": starlark.Bool(true),
		}), nil
	}

	// Dry run check
	if DryRun {
		cli.Note("[dry-run] would run: pre-commit install")
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(true),
			"message":           starlark.String("[dry-run] would install pre-commit hooks"),
			"already_installed": starlark.Bool(false),
		}), nil
	}

	// Run pre-commit install
	cmd := exec.Command(precommitPath, "install")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(false),
			"message":           starlark.String(fmt.Sprintf("Failed to install hooks: %s", string(output))),
			"already_installed": starlark.Bool(false),
		}), nil
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"success":           starlark.Bool(true),
		"message":           starlark.String("Pre-commit hooks installed"),
		"already_installed": starlark.Bool(false),
	}), nil
}

// =============================================================================
// CONFIG INITIALIZATION
// =============================================================================

// setupInitConfig creates star.yaml if it doesn't exist and syncs tool configs.
// Returns a struct with:
//   - star_yaml_created: True if star.yaml was created
//   - star_yaml_path: Path to star.yaml
//   - configs_synced: List of config files that were synced
func setupInitConfig(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("setup.init_config", args, kwargs); err != nil {
		return nil, err
	}

	starYAMLCreated := false
	starYAMLPath := "star.yaml"
	var configsSynced []starlark.Value

	// Check if star.yaml exists
	if _, err := os.Stat(starYAMLPath); os.IsNotExist(err) {
		if DryRun {
			cli.Note("[dry-run] would create %s", starYAMLPath)
			starYAMLCreated = true
		} else {
			// Create default star.yaml
			defaultConfig := `# star.yaml - NobleFactor project configuration
# See: star config show --defaults

lint:
  go:
    path: "./..."
  shell:
    path: "."
    severity: warning
    indent: 4
  markdown:
    path: "."
    exclude:
      - "vendor/**"
      - "node_modules/**"
    frontmatter:
      required:
        - title
        - description
`
			if err := os.WriteFile(starYAMLPath, []byte(defaultConfig), 0o644); err != nil {
				return nil, fmt.Errorf("creating star.yaml: %w", err)
			}
			starYAMLCreated = true
			cli.Success("Created %s", starYAMLPath)
		}
	}

	// Load config and sync tool configs
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	if DryRun {
		cli.Note("[dry-run] would sync tool configs")
		configsSynced = append(configsSynced, starlark.String(".golangci.yaml"))
		configsSynced = append(configsSynced, starlark.String(".markdownlint-cli2.yaml"))
	} else {
		synced, err := cfg.Sync()
		if err != nil {
			return nil, fmt.Errorf("syncing configs: %w", err)
		}
		if synced.GolangciLint != "" {
			configsSynced = append(configsSynced, starlark.String(synced.GolangciLint))
			cli.Success("Synced %s", synced.GolangciLint)
		}
		if synced.MarkdownLint != "" {
			configsSynced = append(configsSynced, starlark.String(synced.MarkdownLint))
			cli.Success("Synced %s", synced.MarkdownLint)
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"star_yaml_created": starlark.Bool(starYAMLCreated),
		"star_yaml_path":    starlark.String(starYAMLPath),
		"configs_synced":    starlark.NewList(configsSynced),
	}), nil
}
