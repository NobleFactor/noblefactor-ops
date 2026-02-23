// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/NobleFactor/devlore-cli/pkg/op"
	"github.com/NobleFactor/noblefactor-ops/internal/cli"
	"github.com/NobleFactor/noblefactor-ops/internal/config"
)

// SetupReceiver provides repository setup operations.
// Implements starlark.Value and starlark.HasAttrs.
type SetupReceiver struct {
	op.Receiver
}

// NewSetupReceiver creates a new SetupReceiver.
func NewSetupReceiver() *SetupReceiver {
	return &SetupReceiver{Receiver: op.NewReceiver("setup")}
}

// Attr implements starlark.HasAttrs.
func (r *SetupReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "tools":
		return op.MakeAttr("setup.tools", r.tools), nil
	case "precommit_install":
		return op.MakeAttr("setup.precommit_install", r.precommitInstall), nil
	case "precommit_check":
		return op.MakeAttr("setup.precommit_check", r.precommitCheck), nil
	case "init_config":
		return op.MakeAttr("setup.init_config", r.initConfig), nil
	case "install_hook":
		return op.MakeAttr("setup.install_hook", r.installHook), nil
	case "uninstall_hook":
		return op.MakeAttr("setup.uninstall_hook", r.uninstallHook), nil
	case "check_hook":
		return op.MakeAttr("setup.check_hook", r.checkHook), nil
	default:
		return nil, op.NoSuchAttrError("setup", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *SetupReceiver) AttrNames() []string {
	return []string{"check_hook", "init_config", "install_hook", "precommit_check", "precommit_install", "tools", "uninstall_hook"}
}

// =============================================================================
// TOOL INFORMATION
// =============================================================================

type DevTool struct {
	Name        string
	Binary      string
	Description string
	DocsURL     string
	Install     map[string]string
}

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
}

func (r *SetupReceiver) tools(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

		installDict := starlark.NewDict(len(tool.Install))
		for plat, cmd := range tool.Install {
			_ = installDict.SetKey(starlark.String(plat), starlark.String(cmd))
		}

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
			"install_cmd": starlark.String(installCmd),
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

func (r *SetupReceiver) precommitCheck(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("setup.precommit_check", args, kwargs); err != nil {
		return nil, err
	}

	precommitPath, _ := exec.LookPath("pre-commit")
	precommitAvailable := precommitPath != ""

	// Use git workspace root for all paths
	root := config.GitWorkspaceRoot()
	if root == "" {
		root = "."
	}

	configExists := false
	if _, err := os.Stat(filepath.Join(root, ".pre-commit-config.yaml")); err == nil {
		configExists = true
	}

	hooksInstalled := false
	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	if info, err := os.Stat(hookPath); err == nil && info.Size() > 0 {
		content, err := os.ReadFile(hookPath)
		if err == nil && len(content) > 0 {
			hooksInstalled = filepath.Ext(hookPath) == "" && len(content) > 100
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"installed":           starlark.Bool(hooksInstalled),
		"config_exists":       starlark.Bool(configExists),
		"precommit_available": starlark.Bool(precommitAvailable),
		"precommit_path":      starlark.String(precommitPath),
	}), nil
}

func (r *SetupReceiver) precommitInstall(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("setup.precommit_install", args, kwargs); err != nil {
		return nil, err
	}

	// Use git workspace root for all paths
	root := config.GitWorkspaceRoot()
	if root == "" {
		root = "."
	}

	if _, err := os.Stat(filepath.Join(root, ".pre-commit-config.yaml")); os.IsNotExist(err) {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(false),
			"message":           starlark.String("No .pre-commit-config.yaml found"),
			"already_installed": starlark.Bool(false),
		}), nil
	}

	precommitPath, err := exec.LookPath("pre-commit")
	if err != nil {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(false),
			"message":           starlark.String("pre-commit not installed. Run: star setup tools"),
			"already_installed": starlark.Bool(false),
		}), nil
	}

	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); err == nil {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(true),
			"message":           starlark.String("Pre-commit hooks already installed"),
			"already_installed": starlark.Bool(true),
		}), nil
	}

	if DryRun {
		cli.Note("[dry-run] would run: pre-commit install")
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(true),
			"message":           starlark.String("[dry-run] would install pre-commit hooks"),
			"already_installed": starlark.Bool(false),
		}), nil
	}

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

func (r *SetupReceiver) initConfig(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("setup.init_config", args, kwargs); err != nil {
		return nil, err
	}

	// Use git workspace root for all paths
	root := config.GitWorkspaceRoot()
	if root == "" {
		root = "."
	}

	starConfigCreated := false
	starConfigPath := filepath.Join(root, "star", "config.yaml")
	var configsSynced []starlark.Value

	if _, err := os.Stat(starConfigPath); os.IsNotExist(err) {
		if DryRun {
			cli.Note("[dry-run] would create %s", starConfigPath)
			starConfigCreated = true
		} else {
			// Ensure star/ directory exists
			starDir := filepath.Join(root, "star")
			if err := os.MkdirAll(starDir, 0o755); err != nil {
				return nil, fmt.Errorf("creating star directory: %w", err)
			}
			defaultConfig := `# star/config.yaml - NobleFactor project configuration
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
			if err := os.WriteFile(starConfigPath, []byte(defaultConfig), 0o644); err != nil {
				return nil, fmt.Errorf("creating star/config.yaml: %w", err)
			}
			starConfigCreated = true
			cli.Success("Created %s", starConfigPath)
		}
	}

	cfg, err := Config.getConfig()
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
		"config_created": starlark.Bool(starConfigCreated),
		"config_path":    starlark.String(starConfigPath),
		"configs_synced": starlark.NewList(configsSynced),
	}), nil
}

// =============================================================================
// NATIVE GIT HOOKS
// =============================================================================

func nativeHookScript(hookName string) string {
	return fmt.Sprintf(`#!/bin/sh
# Installed by star - run 'star setup hooks' to reinstall
exec star hook %s "$@"
`, hookName)
}

func (r *SetupReceiver) installHook(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs("setup.install_hook", args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	validHooks := map[string]bool{
		"pre-commit":  true,
		"pre-push":    true,
		"commit-msg":  true,
		"post-commit": true,
	}
	if !validHooks[name] {
		return nil, fmt.Errorf("invalid hook name: %s (valid: pre-commit, pre-push, commit-msg, post-commit)", name)
	}

	// Use git workspace root for all paths
	root := config.GitWorkspaceRoot()
	if root == "" {
		root = "."
	}

	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(false),
			"message":           starlark.String("Not a git repository (no .git directory)"),
			"already_installed": starlark.Bool(false),
		}), nil
	}

	hookPath := filepath.Join(root, ".git", "hooks", name)
	hookContent := nativeHookScript(name)

	if data, err := os.ReadFile(hookPath); err == nil {
		if string(data) == hookContent {
			return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"success":           starlark.Bool(true),
				"message":           starlark.String("Star hook already installed"),
				"already_installed": starlark.Bool(true),
			}), nil
		}
		if !containsStr(string(data), "Installed by star") {
			return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"success":           starlark.Bool(false),
				"message":           starlark.String("Existing hook found (not managed by star). Remove it first or use --force"),
				"already_installed": starlark.Bool(false),
			}), nil
		}
	}

	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating hooks directory: %w", err)
	}

	if DryRun {
		cli.Note("[dry-run] would install %s hook", name)
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success":           starlark.Bool(true),
			"message":           starlark.String(fmt.Sprintf("[dry-run] would install %s hook", name)),
			"already_installed": starlark.Bool(false),
		}), nil
	}

	if err := os.WriteFile(hookPath, []byte(hookContent), 0o755); err != nil {
		return nil, fmt.Errorf("writing hook: %w", err)
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"success":           starlark.Bool(true),
		"message":           starlark.String(fmt.Sprintf("Installed %s hook", name)),
		"already_installed": starlark.Bool(false),
	}), nil
}

func (r *SetupReceiver) uninstallHook(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs("setup.uninstall_hook", args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	// Use git workspace root for all paths
	root := config.GitWorkspaceRoot()
	if root == "" {
		root = "."
	}

	hookPath := filepath.Join(root, ".git", "hooks", name)

	data, err := os.ReadFile(hookPath)
	if os.IsNotExist(err) {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success": starlark.Bool(true),
			"message": starlark.String("Hook not installed"),
		}), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading hook: %w", err)
	}

	if !containsStr(string(data), "Installed by star") {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success": starlark.Bool(false),
			"message": starlark.String("Hook exists but is not managed by star"),
		}), nil
	}

	if DryRun {
		cli.Note("[dry-run] would remove %s hook", name)
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"success": starlark.Bool(true),
			"message": starlark.String(fmt.Sprintf("[dry-run] would remove %s hook", name)),
		}), nil
	}

	if err := os.Remove(hookPath); err != nil {
		return nil, fmt.Errorf("removing hook: %w", err)
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"success": starlark.Bool(true),
		"message": starlark.String(fmt.Sprintf("Removed %s hook", name)),
	}), nil
}

func (r *SetupReceiver) checkHook(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs("setup.check_hook", args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	// Use git workspace root for all paths
	root := config.GitWorkspaceRoot()
	if root == "" {
		root = "."
	}

	hookPath := filepath.Join(root, ".git", "hooks", name)

	data, err := os.ReadFile(hookPath)
	if os.IsNotExist(err) {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"installed":       starlark.Bool(false),
			"exists":          starlark.Bool(false),
			"managed_by_star": starlark.Bool(false),
		}), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading hook: %w", err)
	}

	managedByStar := containsStr(string(data), "Installed by star")

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"installed":       starlark.Bool(managedByStar),
		"exists":          starlark.Bool(true),
		"managed_by_star": starlark.Bool(managedByStar),
	}), nil
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
