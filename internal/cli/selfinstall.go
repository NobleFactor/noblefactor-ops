// SPDX-License-Identifier: MIT
// Copyright (c) 2025 Noble Factor. All rights reserved.

package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// SelfInstallInfo contains metadata needed for self-installation.
type SelfInstallInfo struct {
	Name      string    // Tool name (e.g., "star")
	ManHeader ManHeader // Man page header metadata
}

// ManHeader contains man page generation metadata.
type ManHeader struct {
	Title   string
	Section string
	Source  string
	Manual  string
}

// NewSelfInstallCmd creates the self-install command.
func NewSelfInstallCmd(rootCmd *cobra.Command, info SelfInstallInfo) *cobra.Command {
	var shells []string

	cmd := &cobra.Command{
		Use:   "self-install <root-directory>",
		Short: "Install star and supporting files to specified directory",
		Long: `Install ` + info.Name + ` and all supporting files to the specified root directory.

This command:
  1. Copies the binary to <root>/bin/` + info.Name + `
  2. Installs man pages to <root>/share/man/man1/ (if man command exists)
  3. Installs shell completions (auto-detects bash, fish, powershell, zsh or use --shell)
  4. Copies ops/ directory to <root>/share/` + info.Name + `/ops/ (for Starlark commands)

Shell completions are auto-detected by default. Use --shell to override:
  ` + info.Name + ` self-install --shell bash --shell zsh ~/.local

Example:
  ` + info.Name + ` self-install ~/.local
  ` + info.Name + ` self-install /usr/local

After installation, ensure <root>/bin is in your PATH.
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := args[0]
			root = expandTilde(root)

			return runSelfInstall(rootCmd, root, info, installFlags{
				Shells: shells,
			})
		},
	}

	cmd.Flags().StringArrayVar(&shells, "shell", nil,
		"Shell to install completions for (repeatable, e.g., --shell bash --shell zsh)")

	return cmd
}

// installFlags holds the flag values for self-install.
type installFlags struct {
	Shells []string
}

// runSelfInstall performs the complete installation.
func runSelfInstall(rootCmd *cobra.Command, root string, info SelfInstallInfo, flags installFlags) error {
	var installed []string
	var installedShells []string

	// 1. Install binary
	binPath, err := installBinary(root, info.Name)
	if err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}
	installed = append(installed, fmt.Sprintf("Binary:      %s", binPath))

	// 2. Install man pages (if man command exists)
	if hasMan() {
		manPath := filepath.Join(root, "share", "man", "man1")
		manFiles, err := installManPagesTo(rootCmd, manPath, info.ManHeader)
		if err != nil {
			return fmt.Errorf("failed to install man pages: %w", err)
		}
		for _, f := range manFiles {
			installed = append(installed, fmt.Sprintf("Man page:    %s", f))
		}
	} else {
		Note("Skipping man pages (man command not found)")
	}

	// 3. Determine which shells to install completions for
	shells := flags.Shells
	if len(shells) == 0 {
		shells = detectShells()
		if len(shells) == 0 {
			Note("No shells detected for completions")
		}
	}

	// 4. Install completions for selected shells
	if len(shells) > 0 {
		completionPaths, err := installCompletionsForShells(rootCmd, root, shells)
		if err != nil {
			return fmt.Errorf("failed to install completions: %w", err)
		}
		for _, p := range completionPaths {
			installed = append(installed, fmt.Sprintf("Completion:  %s", p))
		}
		installedShells = shells
	}

	// 5. Install ops/ directory for Starlark commands
	opsInstalled, err := installOpsDir(root, info.Name)
	if err != nil {
		Warn("Failed to install ops directory: %v", err)
	} else if opsInstalled != "" {
		installed = append(installed, fmt.Sprintf("Ops dir:     %s", opsInstalled))
	}

	// Print summary
	Success("Installed %s to %s", info.Name, root)
	Note("")
	for _, line := range installed {
		Note("  %s", line)
	}

	// Print PATH reminder
	binDir := filepath.Join(root, "bin")
	Note("")
	Note("Add %s to your PATH if not already present.", binDir)

	// Print shell completion setup instructions
	printShellSetupInstructions(installedShells, info.Name)

	return nil
}

// installBinary copies the current executable to the target location.
func installBinary(root, name string) (string, error) {
	currentExe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	binDir := filepath.Join(root, "bin")
	targetPath := filepath.Join(binDir, name)

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", binDir, err)
	}

	if currentExe == targetPath {
		return targetPath, nil
	}

	if err := copyFile(currentExe, targetPath); err != nil {
		return "", err
	}

	if err := os.Chmod(targetPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make executable: %w", err)
	}

	return targetPath, nil
}

// installOpsDir copies the ops/ directory to the installation location.
func installOpsDir(root, name string) (string, error) {
	// Find source ops directory
	srcOpsDir := findOpsDir()
	if srcOpsDir == "" {
		return "", nil // No ops directory to install
	}

	// Target directory: <root>/share/<name>/ops/
	targetOpsDir := filepath.Join(root, "share", name, "ops")
	if err := os.MkdirAll(targetOpsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create ops directory: %w", err)
	}

	// Copy all .star files
	entries, err := os.ReadDir(srcOpsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read ops directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".star" {
			continue
		}
		src := filepath.Join(srcOpsDir, entry.Name())
		dst := filepath.Join(targetOpsDir, entry.Name())
		if err := copyFile(src, dst); err != nil {
			return "", fmt.Errorf("failed to copy %s: %w", entry.Name(), err)
		}
	}

	return targetOpsDir, nil
}

// findOpsDir looks for the ops/ directory relative to cwd or executable.
func findOpsDir() string {
	if info, err := os.Stat("ops"); err == nil && info.IsDir() {
		return "ops"
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "ops")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		// Also check share/<name>/ops relative to bin
		exeDir := filepath.Dir(exe)
		shareOps := filepath.Join(filepath.Dir(exeDir), "share", "star", "ops")
		if info, err := os.Stat(shareOps); err == nil && info.IsDir() {
			return shareOps
		}
	}

	return ""
}

// installManPagesTo generates and installs man pages.
func installManPagesTo(rootCmd *cobra.Command, path string, header ManHeader) ([]string, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	now := time.Now()
	h := &doc.GenManHeader{
		Title:   header.Title,
		Section: header.Section,
		Date:    &now,
		Source:  header.Source,
		Manual:  header.Manual,
	}

	if err := doc.GenManTree(rootCmd, h, path); err != nil {
		return nil, fmt.Errorf("failed to generate man pages: %w", err)
	}

	var files []string
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, filepath.Join(path, e.Name()))
		}
	}

	return files, nil
}

// shellCompletionPath returns the installation path and filename for a shell's completion file.
func shellCompletionPath(shell, cmdName string) (relPath, filename string) {
	switch shell {
	case "bash":
		return filepath.Join("share", "bash-completion", "completions"), cmdName
	case "fish":
		return filepath.Join("share", "fish", "vendor_completions.d"), cmdName + ".fish"
	case "powershell":
		return filepath.Join("share", "powershell", "completions"), cmdName + ".ps1"
	case "zsh":
		return filepath.Join("share", "zsh", "site-functions"), "_" + cmdName
	default:
		return "", ""
	}
}

// installCompletionsForShells installs completions for the specified shells.
func installCompletionsForShells(rootCmd *cobra.Command, root string, shells []string) ([]string, error) {
	var paths []string

	for _, shellName := range shells {
		relPath, filename := shellCompletionPath(shellName, rootCmd.Name())
		if relPath == "" {
			Warn("Unknown shell: %s (skipping)", shellName)
			continue
		}

		dir := filepath.Join(root, relPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return paths, fmt.Errorf("failed to create %s completion directory: %w", shellName, err)
		}

		fullPath := filepath.Join(dir, filename)
		f, err := os.Create(fullPath)
		if err != nil {
			return paths, fmt.Errorf("failed to create %s completion file: %w", shellName, err)
		}

		var genErr error
		switch shellName {
		case "bash":
			genErr = rootCmd.GenBashCompletionV2(f, true)
		case "fish":
			genErr = rootCmd.GenFishCompletion(f, true)
		case "powershell":
			genErr = rootCmd.GenPowerShellCompletionWithDesc(f)
		case "zsh":
			genErr = rootCmd.GenZshCompletion(f)
		default:
			_ = f.Close()
			continue
		}
		_ = f.Close()

		if genErr != nil {
			return paths, fmt.Errorf("failed to generate %s completion: %w", shellName, genErr)
		}

		paths = append(paths, fullPath)
	}

	return paths, nil
}

// printShellSetupInstructions prints setup instructions for installed shells.
func printShellSetupInstructions(shells []string, toolName string) {
	if len(shells) == 0 {
		return
	}

	Note("")
	Note("Shell completion setup:")

	for _, shell := range shells {
		switch shell {
		case "bash":
			Note("")
			Note("  For bash, ensure bash-completion is installed.")
		case "fish":
			Note("")
			Note("  For fish, completions work automatically.")
		case "powershell":
			Note("")
			Note("  For PowerShell, add to your $PROFILE:")
			Note("    . ~/.local/share/powershell/completions/%s.ps1", toolName)
		case "zsh":
			Note("")
			Note("  For zsh, add to ~/.zshrc:")
			Note("    fpath=(~/.local/share/zsh/site-functions $fpath)")
			Note("    autoload -Uz compinit && compinit")
		}
	}
}

// detectShells returns available shells on the system.
func detectShells() []string {
	var shells []string

	if _, err := exec.LookPath("bash"); err == nil {
		shells = append(shells, "bash")
	}
	if _, err := exec.LookPath("fish"); err == nil {
		shells = append(shells, "fish")
	}
	if _, err := exec.LookPath("pwsh"); err == nil {
		shells = append(shells, "powershell")
	} else if _, err := exec.LookPath("powershell"); err == nil {
		shells = append(shells, "powershell")
	}
	if _, err := exec.LookPath("zsh"); err == nil {
		shells = append(shells, "zsh")
	}

	return shells
}

// hasMan returns true if the man command is available.
func hasMan() bool {
	_, err := exec.LookPath("man")
	return err == nil
}

// expandTilde expands ~ to $HOME in a path.
func expandTilde(path string) string {
	if path == "" {
		return ""
	}
	if len(path) >= 2 && path[:2] == "~/" {
		return filepath.Join(os.Getenv("HOME"), path[2:])
	}
	if path == "~" {
		return os.Getenv("HOME")
	}
	return path
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer func() { _ = source.Close() }()

	dest, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer func() { _ = dest.Close() }()

	if _, err := io.Copy(dest, source); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	return nil
}
