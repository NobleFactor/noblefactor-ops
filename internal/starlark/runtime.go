// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package starlark provides a Starlark runtime for nf-ops commands.
package starlark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"

	"github.com/NobleFactor/noblefactor-ops/internal/cli"
	"github.com/NobleFactor/noblefactor-ops/internal/config"
	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// DryRun is set by the --dry-run global flag. When true, side-effect
// bindings (fs.write, fs.mkdir, fs.remove, etc.) log what they would
// do instead of executing.
var DryRun bool

// Runtime manages Starlark script execution.
type Runtime struct {
	commands      map[string]*Command
	opsDir        string
	extensionsDir string         // Path to extensions directory
	config        *config.Config // Unified config for builtin and extension config
}

// NewRuntime creates a new Starlark runtime.
func NewRuntime(opsDir string) *Runtime {
	return &Runtime{
		commands:      make(map[string]*Command),
		opsDir:        opsDir,
		extensionsDir: "extensions",
	}
}

// SetExtensionsDir sets the extensions directory path.
func (r *Runtime) SetExtensionsDir(dir string) {
	r.extensionsDir = dir
}

// Config returns the unified config, initializing if needed.
func (r *Runtime) Config() *config.Config {
	if r.config == nil {
		cfg, err := config.Load()
		if err != nil {
			// Fall back to empty config on error
			cfg, _ = config.Load()
		}
		r.config = cfg
	}
	return r.config
}

// LoadAll loads extensions first, then all .star files from the ops directory.
func (r *Runtime) LoadAll() error {
	// Load extensions first (before ops/*.star files)
	if err := r.LoadExtensions(); err != nil {
		return fmt.Errorf("loading extensions: %w", err)
	}

	// Then walk ops directory for traditional star files
	return filepath.WalkDir(r.opsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // No ops directory is fine
			}
			return err
		}

		// Skip directories and non-.star files
		if d.IsDir() || filepath.Ext(d.Name()) != ".star" {
			return nil
		}

		// Load the star file
		relPath, relErr := filepath.Rel(r.opsDir, path)
		if relErr != nil {
			relPath = path // Fall back to absolute path
		}
		if err := r.Load(path); err != nil {
			return fmt.Errorf("loading %s: %w", relPath, err)
		}
		return nil
	})
}

// LoadExtensions discovers and loads extensions from the extensions directory.
// This should be called before LoadAll() to ensure extension commands
// are available when ops/*.star files execute.
func (r *Runtime) LoadExtensions() error {
	// Skip if extensions directory doesn't exist
	if _, err := os.Stat(r.extensionsDir); os.IsNotExist(err) {
		return nil // No extensions directory is fine
	}

	// Discover extensions
	specs, err := extension.Discover(r.extensionsDir)
	if err != nil {
		return fmt.Errorf("discover extensions: %w", err)
	}

	// Register extensions and load their commands
	for _, spec := range specs {
		// Register with global extension registry (ignore duplicates)
		_ = extension.Register(spec)

		// Register config if the extension has one
		if spec.HasConfig() {
			if err := r.Config().RegisterExtension(spec.Extension, spec.ToConfigSpec()); err != nil {
				return fmt.Errorf("register config for %s: %w", spec.Extension, err)
			}
		}

		// Load the extension's Starlark implementation if it has a command
		if spec.HasCommand() {
			if err := r.loadExtensionCommand(spec); err != nil {
				return fmt.Errorf("load extension %s: %w", spec.Extension, err)
			}
		}
	}

	return nil
}

// loadExtensionCommand loads a single extension's Starlark command.
func (r *Runtime) loadExtensionCommand(spec *extension.ExtensionSpec) error {
	if spec.Command == nil || spec.Command.Implementation == "" {
		return nil
	}

	// Get extension directory from the spec's source path
	extDir := filepath.Dir(spec.SourcePath)

	// Build path to implementation file
	implPath := filepath.Join(extDir, spec.Command.Implementation)

	// Create thread with print function
	thread := &starlark.Thread{
		Name:  spec.Extension,
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	// Create collector for command() calls
	collector := &commandCollector{commands: make(map[string]*Command)}

	// Build predeclared environment with extension-specific context
	predeclared := r.buildPredeclared(collector, spec)

	// Execute the script with extended dialect options
	fileOpts := syntax.FileOptions{
		Set:             true,
		While:           true,
		TopLevelControl: true,
		GlobalReassign:  true,
		Recursion:       true,
	}
	globals, err := starlark.ExecFileOptions(&fileOpts, thread, implPath, nil, predeclared)
	if err != nil {
		return fmt.Errorf("exec %s: %w", implPath, err)
	}

	// Register commands with proper names
	for name, cmd := range collector.commands {
		cmd.globals = globals
		cmd.predeclared = predeclared

		// Apply flag defaults from extension spec
		r.applyFlagDefaults(cmd, spec)

		// Use the extension name as command path if not specified
		cmdName := name
		if cmdName == "" || cmdName == spec.Extension {
			// Convert "lint.copyright" to "lint copyright"
			cmdName = strings.ReplaceAll(spec.Extension, ".", " ")
		}

		r.commands[cmdName] = cmd
	}

	return nil
}

// applyFlagDefaults merges flag defaults from extension spec into command flags.
func (r *Runtime) applyFlagDefaults(cmd *Command, spec *extension.ExtensionSpec) {
	if len(spec.Flags) == 0 {
		return
	}

	// Build a map of spec flags for quick lookup
	specFlags := make(map[string]*extension.FlagSpec)
	for i := range spec.Flags {
		specFlags[spec.Flags[i].Name] = &spec.Flags[i]
	}

	// Update existing command flags with spec defaults
	for i := range cmd.Flags {
		if specFlag, ok := specFlags[cmd.Flags[i].Name]; ok {
			// Only set default if command flag has no default
			if cmd.Flags[i].Default == "" && specFlag.Default != "" {
				cmd.Flags[i].Default = specFlag.Default
			}
			// Update help if not set
			if cmd.Flags[i].Help == "" && specFlag.Help != "" {
				cmd.Flags[i].Help = specFlag.Help
			}
		}
	}

	// Add flags from spec that aren't already defined in command
	existingFlags := make(map[string]bool)
	for _, f := range cmd.Flags {
		existingFlags[f.Name] = true
	}

	for _, specFlag := range spec.Flags {
		if !existingFlags[specFlag.Name] {
			cmd.Flags = append(cmd.Flags, Flag{
				Name:    specFlag.Name,
				Help:    specFlag.Help,
				Default: specFlag.Default,
			})
		}
	}
}

// Load loads a single .star file and registers its commands.
func (r *Runtime) Load(path string) error {
	// Create thread with print function
	thread := &starlark.Thread{
		Name:  filepath.Base(path),
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	// Create collector for command() calls
	collector := &commandCollector{commands: make(map[string]*Command)}

	// Build predeclared environment (no extension spec for legacy files)
	predeclared := r.buildPredeclared(collector, nil)

	// Execute the script with extended dialect options
	fileOpts := syntax.FileOptions{
		Set:             true, // Enable set() built-in
		While:           true, // Enable while loops
		TopLevelControl: true, // Enable top-level if/for/while
		GlobalReassign:  true, // Enable reassignment to top-level names
		Recursion:       true, // Enable recursive functions
	}
	globals, err := starlark.ExecFileOptions(&fileOpts, thread, path, nil, predeclared)
	if err != nil {
		return fmt.Errorf("exec %s: %w", path, err)
	}

	// Store globals for later use by run functions
	for name, cmd := range collector.commands {
		cmd.globals = globals
		cmd.predeclared = predeclared
		r.commands[name] = cmd
	}

	return nil
}

// buildPredeclared constructs the predeclared environment for Starlark execution.
// If spec is non-nil, it may provide extension-specific bindings.
func (r *Runtime) buildPredeclared(collector *commandCollector, spec *extension.ExtensionSpec) starlark.StringDict {
	return starlark.StringDict{
		"fs":             fsModule(),
		"json":           jsonModule(),
		"yaml":           yamlModule(),
		"schema":         schemaModule(),
		"go":             goModule(),
		"shell":          shellModule(),
		"lint":           lintModule(),
		"copyright":      copyrightModule(),
		"config":         r.configModuleWithExtension(spec),
		"setup":          setupModule(),
		"starlark_parse": starlarkParseModule(),
		"command":        starlark.NewBuiltin("command", collector.commandBuiltin),
		// Output functions in global namespace
		"note":    starlark.NewBuiltin("note", noteBuiltin),
		"warn":    starlark.NewBuiltin("warn", warnBuiltin),
		"error":   starlark.NewBuiltin("error", errorBuiltin),
		"success": starlark.NewBuiltin("success", successBuiltin),
		"fail":    starlark.NewBuiltin("fail", failBuiltin),
	}
}

// configModuleWithExtension returns a config module that provides unified
// access to both builtin and extension configuration.
func (r *Runtime) configModuleWithExtension(_ *extension.ExtensionSpec) *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "config",
		Members: starlark.StringDict{
			"get":  starlark.NewBuiltin("config.get", r.configGet),
			"show": starlark.NewBuiltin("config.show", configShow),
			"sync": starlark.NewBuiltin("config.sync", configSync),
		},
	}
}

// configGet returns the unified config as a Starlark value.
func (r *Runtime) configGet(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("config.get", args, kwargs); err != nil {
		return nil, err
	}

	return r.Config().ToStarlark(), nil
}

// Commands returns all registered commands.
func (r *Runtime) Commands() map[string]*Command {
	return r.commands
}

// =============================================================================
// Output Builtins
// =============================================================================

func noteBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	msg := extractMessage(args, kwargs)
	cli.Note("%s", msg)
	return starlark.None, nil
}

func warnBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	msg := extractMessage(args, kwargs)
	cli.Warn("%s", msg)
	return starlark.None, nil
}

func errorBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	msg := extractMessage(args, kwargs)
	cli.Error("%s", msg)
	return starlark.None, nil
}

func successBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	msg := extractMessage(args, kwargs)
	cli.Success("%s", msg)
	return starlark.None, nil
}

func failBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	msg := extractMessage(args, kwargs)
	cli.Error("%s", msg)
	return nil, fmt.Errorf("fail: %s", msg)
}

func extractMessage(args starlark.Tuple, kwargs []starlark.Tuple) string {
	var msg string
	if err := starlark.UnpackArgs("", args, kwargs, "msg", &msg); err != nil {
		// Try positional
		if len(args) >= 1 {
			if s, ok := starlark.AsString(args[0]); ok {
				msg = s
			}
		}
	}
	return msg
}
