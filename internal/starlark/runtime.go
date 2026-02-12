// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package starlark provides a Starlark runtime for nf-ops commands.
package starlark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/NobleFactor/noblefactor-ops/internal/cli"
	"github.com/NobleFactor/noblefactor-ops/internal/config"
	"github.com/NobleFactor/noblefactor-ops/internal/extension"
	"github.com/NobleFactor/noblefactor-ops/internal/wasm"
)

// DryRun is set by the --dry-run global flag. When true, side-effect
// bindings (fs.write, fs.mkdir, fs.remove, etc.) log what they would
// do instead of executing.
var DryRun bool

// Runtime manages Starlark script execution.
type Runtime struct {
	commands map[string]*Command
	config   *config.Config // Unified config for builtin and extension config

	// wasmHosts maps extension name to its WASM host.
	// Each extension gets its own host with its receiver's capabilities.
	wasmHosts map[string]*wasm.WasmHost

	// wasmReceivers maps extension name to its loaded WASM receivers.
	// Key format: "extensionName:receiverName"
	wasmReceivers map[string]*WasmReceiver
}

// NewRuntime creates a new Starlark runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		commands:      make(map[string]*Command),
		wasmHosts:     make(map[string]*wasm.WasmHost),
		wasmReceivers: make(map[string]*WasmReceiver),
	}
}

// Config returns the unified config, initializing if needed.
func (r *Runtime) Config() *config.Config {
	if r.config == nil {
		r.config = config.New()
	}
	return r.config
}

// LoadAll loads extensions from all default search paths.
func (r *Runtime) LoadAll() error {
	return r.LoadExtensions()
}

// LoadExtensions discovers and loads extensions from all default search paths.
func (r *Runtime) LoadExtensions() error {
	return r.loadExtensionsFromPaths(extension.DefaultSearchPaths()...)
}

// LoadExtensionsFrom loads extensions from a specific directory.
func (r *Runtime) LoadExtensionsFrom(dir string) error {
	return r.loadExtensionsFromPaths(dir)
}

// loadExtensionsFromPaths discovers and loads extensions from the given paths.
func (r *Runtime) loadExtensionsFromPaths(paths ...string) error {
	// Discover extensions from all search paths
	var specs []*extension.ExtensionSpec
	for _, dir := range paths {
		// Skip non-existent directories
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		dirSpecs, err := extension.Discover(dir)
		if err != nil {
			return fmt.Errorf("discover extensions in %s: %w", dir, err)
		}
		specs = append(specs, dirSpecs...)
	}

	// Register extensions and load their commands
	for _, spec := range specs {
		// Register with global extension registry (ignore duplicates)
		_ = extension.Register(spec)

		// Register config if the extension has one
		if spec.HasConfig() {
			if err := r.Config().RegisterExtension(spec.ConfigPath(), spec.ToConfigSpec()); err != nil {
				return fmt.Errorf("register config for %s: %w", spec.Extension, err)
			}
		}

		// Load WASM receivers for this extension
		if spec.HasWasmReceivers() {
			if err := r.loadWasmReceivers(spec); err != nil {
				return fmt.Errorf("load WASM receivers for %s: %w", spec.Extension, err)
			}
		}

		// Load the extension's Starlark commands
		if spec.HasCommands() {
			if err := r.loadExtensionCommands(spec); err != nil {
				return fmt.Errorf("load extension %s: %w", spec.Extension, err)
			}
		}
	}

	// Load config values from user/project YAML files now that all
	// extensions have registered their config specs.
	if err := r.Config().LoadFromFiles(); err != nil {
		return fmt.Errorf("load config files: %w", err)
	}

	// Wire the loaded config into the ConfigReceiver singleton so
	// config.get()/show()/sync() use the fully-populated config.
	Config.SetConfig(r.Config())

	return nil
}

// loadWasmReceivers loads WASM receivers for an extension.
// Creates a WASM host for each receiver with its declared capabilities.
func (r *Runtime) loadWasmReceivers(spec *extension.ExtensionSpec) error {
	if !spec.HasWasmReceivers() {
		return nil
	}

	extDir := filepath.Dir(spec.SourcePath)

	for _, recv := range spec.Receivers {
		if recv.Builtin || recv.Wasm == "" {
			continue
		}

		// Build absolute path to WASM file
		wasmPath := filepath.Join(extDir, recv.Wasm)

		// Check if WASM file exists
		if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
			return fmt.Errorf("WASM file not found: %s", wasmPath)
		}

		// Create WASM host with receiver's capabilities
		caps := extension.Capabilities{}
		if recv.Capabilities != nil {
			caps = *recv.Capabilities
		}

		host, err := wasm.NewHost(context.Background(), caps)
		if err != nil {
			return fmt.Errorf("create WASM host for %s: %w", recv.Name, err)
		}

		// Store host for cleanup later
		hostKey := fmt.Sprintf("%s:%s", spec.Extension, recv.Name)
		r.wasmHosts[hostKey] = host

		// Load the WASM module
		module, err := host.LoadModule(wasmPath)
		if err != nil {
			return fmt.Errorf("load WASM module %s: %w", recv.Name, err)
		}

		// Auto-discover functions from WASM exports
		functions := module.Functions()

		// Create WasmReceiver
		receiver := NewWasmReceiver(recv.Name, module, functions)
		r.wasmReceivers[hostKey] = receiver

	}

	return nil
}

// loadExtensionCommands loads all commands from an extension.
func (r *Runtime) loadExtensionCommands(spec *extension.ExtensionSpec) error {
	// Get extension directory from the spec's source path
	extDir := filepath.Dir(spec.SourcePath)

	// Load each command
	for _, cmdSpec := range spec.Commands {
		if cmdSpec.Implementation == "" {
			continue
		}

		if err := r.loadExtensionCommand(spec, &cmdSpec, extDir); err != nil {
			return fmt.Errorf("load command %s: %w", cmdSpec.Name, err)
		}
	}

	return nil
}

// loadExtensionCommand loads a single command from an extension.
// The .star file must define a `run` function. All metadata comes from extension.yaml.
func (r *Runtime) loadExtensionCommand(spec *extension.ExtensionSpec, cmdSpec *extension.CommandSpec, extDir string) error {
	// Build path to implementation file
	implPath := filepath.Join(extDir, cmdSpec.Implementation)

	// Create thread with print function
	thread := &starlark.Thread{
		Name:  cmdSpec.Name,
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	// Build predeclared environment with extension-specific context
	predeclared := r.buildPredeclared(spec)

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

	// Look for the run function
	runVal, ok := globals["run"]
	if !ok {
		return fmt.Errorf("%s: missing 'run' function", implPath)
	}
	runFunc, ok := runVal.(starlark.Callable)
	if !ok {
		return fmt.Errorf("%s: 'run' is not callable", implPath)
	}

	// Build command from extension.yaml metadata
	cmd := &Command{
		Name:        cmdSpec.Name,
		Help:        cmdSpec.Help,
		RunFunc:     runFunc,
		globals:     globals,
		predeclared: predeclared,
		runtime:     r,
	}

	// Build flags from command spec
	for _, flagSpec := range cmdSpec.Flags {
		cmd.Flags = append(cmd.Flags, Flag{
			Name:    flagSpec.Name,
			Help:    flagSpec.Help,
			Default: flagSpec.Default,
		})
	}

	// Register with space-separated name (e.g., "lint.go" -> "lint go")
	cmdName := strings.ReplaceAll(cmdSpec.Name, ".", " ")
	r.commands[cmdName] = cmd

	return nil
}

// buildPredeclared constructs the predeclared environment for Starlark execution.
// If spec is non-nil, it may provide extension-specific bindings (WASM receivers).
func (r *Runtime) buildPredeclared(spec *extension.ExtensionSpec) starlark.StringDict {
	predeclared := starlark.StringDict{
		// Receiver-pattern bindings (all modules use HasAttrs pattern)
		"file":           File,
		"json":           JSON,
		"yaml":           YAML,
		"schema":         Schema,
		"shell":          Shell,
		"regexp":         Regexp,
		"go":             Go,
		"lint":           Lint,
		"setup":          Setup,
		"config":         Config,
		"starlark_parse": StarlarkParse,

		// Command tree navigation (current command set at runtime)
		"commands": NewCommandsReceiver(r),

		// Output functions in global namespace
		"note":    starlark.NewBuiltin("note", noteBuiltin),
		"warn":    starlark.NewBuiltin("warn", warnBuiltin),
		"error":   starlark.NewBuiltin("error", errorBuiltin),
		"success": starlark.NewBuiltin("success", successBuiltin),
		"fail":    starlark.NewBuiltin("fail", failBuiltin),
	}

	// Add WASM receivers from extension spec
	if spec != nil {
		for _, recv := range spec.Receivers {
			if recv.Builtin || recv.Wasm == "" {
				continue
			}
			key := fmt.Sprintf("%s:%s", spec.Extension, recv.Name)
			if wasmRecv, ok := r.wasmReceivers[key]; ok {
				predeclared[recv.Name] = wasmRecv
			}
		}
	}

	return predeclared
}

// Commands returns all registered commands.
func (r *Runtime) Commands() map[string]*Command {
	return r.commands
}

// Close releases all resources held by the runtime.
// This includes closing all WASM hosts.
func (r *Runtime) Close() error {
	var errs []error
	for name, host := range r.wasmHosts {
		if err := host.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close WASM host %s: %w", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
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
