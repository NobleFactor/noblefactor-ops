// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// star is the Starlark-powered operations tool for NobleFactor projects.
// Operations are defined as .star scripts in the ops/ directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/NobleFactor/noblefactor-ops/internal/cli"
	starruntime "github.com/NobleFactor/noblefactor-ops/internal/starlark"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

const starlarkDocs = `WRITING STARLARK OPERATIONS

Operations are defined in .star files in the ops/ directory (relative to the
star binary or current working directory). Each file can register one or
more commands.

BASIC STRUCTURE

    # ops/my-operation.star

    def run(ctx):
        """Main entry point for the operation."""
        path = ctx.args.get("path", ".")

        note("Processing: " + path)
        # fs.write is dry-run safe - automatically skips when --dry-run is set
        fs.write(fs.join(path, "output.txt"), "Hello!")
        success("Wrote output.txt")

    command(
        name = "mygroup.my-operation",
        help = "Description shown in help output",
        flags = [
            {"name": "path", "help": "Path to process", "default": "."},
        ],
        run = run,
    )

COMMAND NAMING

The command name uses dots to create subcommand hierarchy:
  - "foo"           -> star foo
  - "foo.bar"       -> star foo bar
  - "foo.bar.baz"   -> star foo bar baz

AVAILABLE MODULES

fs - File system operations:
  fs.read(path)              Read file contents as string
  fs.write(path, content)    Write string to file [dry-run safe]
  fs.exists(path)            Check if path exists
  fs.is_dir(path)            Check if path is directory
  fs.is_file(path)           Check if path is file
  fs.list_dir(path)          List directory entries
                             Returns list of {name, path, is_dir}
  fs.join(a, b, ...)         Join path components
  fs.basename(path)          Get filename from path
  fs.dirname(path)           Get directory from path
  fs.glob(pattern)           Find files matching pattern
  fs.mkdir(path)             Create directory (with parents) [dry-run safe]
  fs.remove(path)            Remove file [dry-run safe]
  fs.remove_all(path)        Remove file or directory recursively [dry-run safe]

  Functions marked [dry-run safe] log their intent and skip execution
  when --dry-run is set.

yaml - YAML encoding/decoding:
  yaml.encode(value)         Convert dict/list to YAML string
  yaml.decode(string)        Parse YAML string to dict/list

GLOBAL OUTPUT FUNCTIONS

These functions produce formatted output to stderr:
  note(msg)                  Informational message (gray +)
  warn(msg)                  Warning message (yellow △)
  error(msg)                 Error message (red ✖)
  success(msg)               Success message (green ✔)
  fail(msg)                  Error message + abort execution

Use print(msg) for raw stdout output (e.g., YAML content in dry-run mode).

CONTEXT OBJECT

The run function receives a context object with:
  ctx.args                   Dict of flag values (all strings)
  ctx.dry_run                Bool: true if --dry-run flag is set

FLAG DEFINITION

Each flag is a dict with:
  name      (required)       Flag name (becomes --name)
  help      (optional)       Help text
  default   (optional)       Default value (string)
  required  (optional)       If true, flag must be provided

EXAMPLES

List files in a directory:

    def run(ctx):
        for entry in fs.list_dir(ctx.args.get("path", ".")):
            if entry.is_dir:
                print("[DIR]  " + entry.name)
            else:
                print("[FILE] " + entry.name)

    command(name="list", help="List directory contents", run=run)

Generate YAML index:

    def run(ctx):
        items = []
        for entry in fs.list_dir(ctx.args.get("path", ".")):
            if entry.name.endswith(".md"):
                items.append({"name": entry.name, "path": entry.path})
        index = {"version": "1", "items": items}
        print(yaml.encode(index))

    command(name="index", help="Generate index", run=run)
`

func main() {
	rootCmd := &cobra.Command{
		Use:   "star",
		Short: "Starlark-powered operations tool",
		Long: `star is the Starlark-powered operations tool for NobleFactor projects.

Operations are defined as .star scripts in the ops/ directory.
Each script can register commands using the command() function.
Run 'star docs starlark' for details on writing operations.

SHELL COMPLETION

Generate shell completions with:
  star completion bash > /etc/bash_completion.d/star
  star completion zsh > "${fpath[1]}/_star"
  star completion fish > ~/.config/fish/completions/star.fish`,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&starruntime.DryRun, "dry-run", false, "Preview changes without executing side effects")

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("star %s (%s) built %s\n", version, commit, buildDate)
		},
	})

	// Key management commands
	keyCmd := &cobra.Command{
		Use:   "key",
		Short: "Key management operations",
	}
	keyCmd.AddCommand(&cobra.Command{
		Use:   "generate",
		Short: "Generate a new signing key",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Key generation not yet implemented")
			fmt.Println("See ADR-040 for the key ceremony protocol")
		},
	})
	keyCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List managed signing keys",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Key listing not yet implemented")
		},
	})
	keyCmd.AddCommand(&cobra.Command{
		Use:   "rotate",
		Short: "Rotate a signing key with ceremony",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Key rotation not yet implemented")
			fmt.Println("This operation requires hardware key presence")
		},
	})
	rootCmd.AddCommand(keyCmd)

	// Documentation commands
	docsCmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate documentation",
	}
	docsCmd.AddCommand(&cobra.Command{
		Use:   "man <output-dir>",
		Short: "Generate man pages",
		Long: `Generate man pages for star and all subcommands.

The man pages are written to the specified output directory.
Install them to your man path (e.g., /usr/local/share/man/man1/).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outDir := args[0]
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
			header := &doc.GenManHeader{
				Title:   "STAR",
				Section: "1",
				Source:  "Noble Factor",
				Manual:  "Star Operations Manual",
			}
			if err := doc.GenManTree(rootCmd, header, outDir); err != nil {
				return fmt.Errorf("generating man pages: %w", err)
			}
			fmt.Printf("Man pages written to %s\n", outDir)
			return nil
		},
	})
	docsCmd.AddCommand(&cobra.Command{
		Use:   "markdown <output-dir>",
		Short: "Generate markdown documentation",
		Long:  `Generate markdown documentation for star and all subcommands.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outDir := args[0]
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
			if err := doc.GenMarkdownTree(rootCmd, outDir); err != nil {
				return fmt.Errorf("generating markdown: %w", err)
			}
			fmt.Printf("Markdown docs written to %s\n", outDir)
			return nil
		},
	})
	docsCmd.AddCommand(&cobra.Command{
		Use:   "starlark",
		Short: "Show how to write Starlark operations",
		Long:  `Show documentation for writing Starlark operations.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(starlarkDocs)
		},
	})
	rootCmd.AddCommand(docsCmd)

	// Self-install command
	rootCmd.AddCommand(cli.NewSelfInstallCmd(rootCmd, cli.SelfInstallInfo{
		Name: "star",
		ManHeader: cli.ManHeader{
			Title:   "STAR",
			Section: "1",
			Source:  "Noble Factor",
			Manual:  "Star Operations Manual",
		},
	}))

	// Load Starlark commands from ops/ directory
	if err := loadStarlarkCommands(rootCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load Starlark commands: %v\n", err)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// loadStarlarkCommands loads .star files from the ops/ directory and registers them as commands.
func loadStarlarkCommands(rootCmd *cobra.Command) error {
	// Find ops directory relative to executable or current directory
	opsDir := findOpsDir()
	if opsDir == "" {
		return nil // No ops directory found, not an error
	}

	runtime := starruntime.NewRuntime(opsDir)
	if err := runtime.LoadAll(); err != nil {
		return err
	}

	// Register each Starlark command
	for _, cmd := range runtime.Commands() {
		registerStarlarkCommand(rootCmd, cmd)
	}

	return nil
}

// findOpsDir looks for the ops/ directory in standard locations.
func findOpsDir() string {
	// Try relative to current directory (development)
	if info, err := os.Stat("ops"); err == nil && info.IsDir() {
		return "ops"
	}

	// Try relative to executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)

		// Try ops/ next to binary (portable installation)
		dir := filepath.Join(exeDir, "ops")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}

		// Try ../share/star/ops (FHS-compliant installation)
		// e.g., /usr/local/bin/star -> /usr/local/share/star/ops
		shareOps := filepath.Join(filepath.Dir(exeDir), "share", "star", "ops")
		if info, err := os.Stat(shareOps); err == nil && info.IsDir() {
			return shareOps
		}
	}

	return ""
}

// registerStarlarkCommand creates a cobra command from a Starlark command.
func registerStarlarkCommand(rootCmd *cobra.Command, cmd *starruntime.Command) {
	// Parse command name (e.g., "registry.index-knowledge" -> registry subcommand with index-knowledge)
	parts := strings.Split(cmd.Name, ".")

	// Build the command path
	parent := rootCmd
	for i := 0; i < len(parts)-1; i++ {
		// Find or create parent command
		found := false
		for _, child := range parent.Commands() {
			if child.Use == parts[i] || strings.HasPrefix(child.Use, parts[i]+" ") {
				parent = child
				found = true
				break
			}
		}
		if !found {
			// Create intermediate command
			newCmd := &cobra.Command{
				Use:   parts[i],
				Short: fmt.Sprintf("%s commands", parts[i]),
			}
			parent.AddCommand(newCmd)
			parent = newCmd
		}
	}

	// Create the leaf command
	leafName := parts[len(parts)-1]
	cobraCmd := &cobra.Command{
		Use:   leafName,
		Short: cmd.Help,
		RunE: func(c *cobra.Command, args []string) error {
			// Collect flag values
			flagValues := make(map[string]string)
			for _, flag := range cmd.Flags {
				val, err := c.Flags().GetString(flag.Name)
				if err == nil {
					flagValues[flag.Name] = val
				}
			}
			return cmd.Run(flagValues)
		},
	}

	// Add flags
	for _, flag := range cmd.Flags {
		cobraCmd.Flags().String(flag.Name, flag.Default, flag.Help)
		if flag.Required {
			if err := cobraCmd.MarkFlagRequired(flag.Name); err != nil {
				// Flag was just added, this can't fail
				panic(fmt.Sprintf("failed to mark flag %q as required: %v", flag.Name, err))
			}
		}
	}

	parent.AddCommand(cobraCmd)
}
