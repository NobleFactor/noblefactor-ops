// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/NobleFactor/devlore-cli/pkg/op"
	"github.com/NobleFactor/devlore-cli/pkg/op/provider/ui"

	commandsprov "github.com/NobleFactor/noblefactor-ops/internal/starlark/provider/commands"
	_ "github.com/NobleFactor/noblefactor-ops/internal/starlark/provider"

	"github.com/NobleFactor/noblefactor-ops/internal/config"
	"github.com/NobleFactor/noblefactor-ops/internal/extension"
	starint "github.com/NobleFactor/noblefactor-ops/internal/starlark"
	"github.com/NobleFactor/noblefactor-ops/internal/wasm"
)

// dryRun is set by the --dry-run global flag. When true, side-effect
// bindings (fs.write, fs.mkdir, fs.remove, etc.) log what they would
// do instead of executing.
var dryRun bool

// Application manages Starlark script execution.
type Application struct {
	commands map[string]*Command
	config   *config.Config // Unified config for builtin and extension config
	star     *op.StarlarkRuntime
	data     map[string]any // Shared context data (dry_run, config, etc.)

	// wasmHosts maps extension name to its WASM host.
	// Each extension gets its own host with its receiver's capabilities.
	wasmHosts map[string]*wasm.WasmHost

	// wasmReceivers maps extension name to its loaded WASM receivers.
	// Key format: "extensionName:receiverName"
	wasmReceivers map[string]*starint.WasmReceiver

	// uiProvider is the canonical UI output provider.
	// Wire --silent to uiProvider.Silent in main.
	uiProvider *ui.Provider
}

// NewApplication creates a new star application.
func NewApplication() *Application {

	cfg := op.NewBindingConfig("star").
		WithReceivers(op.Receivers()...).
		WithColor()
	star := op.NewStarlarkRuntime(cfg)

	// Initialize the framework runtime so BuildReceivers can construct providers.
	// Root is set to the current working directory so file.Provider can perform I/O.
	// RecoverySite is auto-created by Initialize when Root is non-nil.
	wd, _ := os.Getwd()
	data := map[string]any{"dry_run": dryRun}
	star.Initialize(op.NewActionRegistry(), op.ContextBase{
		Context: context.Background(),
		Writer:  os.Stderr,
		Root:    op.NewRootReaderWriter(wd),
		Data:    data,
	})

	// uiProvider is exposed for --silent flag wiring in main.
	uip := &ui.Provider{
		Writer:      os.Stderr,
		ProgramName: "star",
		Color:       true,
	}

	app := &Application{
		commands:      make(map[string]*Command),
		wasmHosts:     make(map[string]*wasm.WasmHost),
		wasmReceivers: make(map[string]*starint.WasmReceiver),
		star:          star,
		data:          data,
		uiProvider:    uip,
	}

	// Wire command tree into context data (shared map — providers read it lazily).
	data["command_tree"] = app

	return app
}

// Config returns the unified config, initializing if needed.
func (a *Application) Config() *config.Config {
	if a.config == nil {
		a.config = config.New()
	}
	return a.config
}

// LoadAll loads extensions from all default search paths.
func (a *Application) LoadAll() error {
	return a.LoadExtensions()
}

// LoadExtensions discovers and loads extensions from all default search paths.
func (a *Application) LoadExtensions() error {
	return a.loadExtensionsFromPaths(extension.DefaultSearchPaths()...)
}

// LoadExtensionsFrom loads extensions from a specific directory.
func (a *Application) LoadExtensionsFrom(dir string) error {
	return a.loadExtensionsFromPaths(dir)
}

// loadExtensionsFromPaths discovers and loads extensions from the given paths.
func (a *Application) loadExtensionsFromPaths(paths ...string) error {
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
			if err := a.Config().RegisterExtension(spec.ConfigPath(), spec.ToConfigSpec()); err != nil {
				return fmt.Errorf("register config for %s: %w", spec.Extension, err)
			}
		}

		// Load WASM receivers for this extension
		if spec.HasWasmReceivers() {
			if err := a.loadWasmReceivers(spec); err != nil {
				return fmt.Errorf("load WASM receivers for %s: %w", spec.Extension, err)
			}
		}

		// Load the extension's Starlark commands
		if spec.HasCommands() {
			if err := a.loadExtensionCommands(spec); err != nil {
				return fmt.Errorf("load extension %s: %w", spec.Extension, err)
			}
		}
	}

	// Load config values from user/project YAML files now that all
	// extensions have registered their config specs.
	if err := a.Config().LoadFromFiles(); err != nil {
		return fmt.Errorf("load config files: %w", err)
	}

	// Wire the loaded config into context data so config.get()/show()/sync()
	// and setup.init_config() use the populated config.
	a.data["config"] = a.Config()

	return nil
}

// loadWasmReceivers loads WASM receivers for an extension.
// Creates a WASM host for each receiver with its declared capabilities.
func (a *Application) loadWasmReceivers(spec *extension.ExtensionSpec) error {
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
		a.wasmHosts[hostKey] = host

		// Load the WASM module
		module, err := host.LoadModule(wasmPath)
		if err != nil {
			return fmt.Errorf("load WASM module %s: %w", recv.Name, err)
		}

		// Auto-discover functions from WASM exports
		functions := module.Functions()

		// Create WasmReceiver
		receiver := starint.NewWasmReceiver(recv.Name, module, functions)
		a.wasmReceivers[hostKey] = receiver

	}

	return nil
}

// loadExtensionCommands loads all commands from an extension.
func (a *Application) loadExtensionCommands(spec *extension.ExtensionSpec) error {
	// Get extension directory from the spec's source path
	extDir := filepath.Dir(spec.SourcePath)

	// Load each command
	for _, cmdSpec := range spec.Commands {
		if cmdSpec.Implementation == "" {
			continue
		}

		if err := a.loadExtensionCommand(spec, &cmdSpec, extDir); err != nil {
			return fmt.Errorf("load command %s: %w", cmdSpec.Name, err)
		}
	}

	return nil
}

// loadExtensionCommand loads a single command from an extension.
// The .star file must define a `run` function. All metadata comes from extension.yaml.
func (a *Application) loadExtensionCommand(spec *extension.ExtensionSpec, cmdSpec *extension.CommandSpec, extDir string) error {
	// Build path to implementation file
	implPath := filepath.Join(extDir, cmdSpec.Implementation)

	// Build predeclared environment with extension-specific context
	predeclared := a.buildPredeclared(spec)

	// Dialect options shared by the main script and any load() targets.
	fileOpts := syntax.FileOptions{
		Set:             true,
		While:           true,
		TopLevelControl: true,
		GlobalReassign:  true,
		Recursion:       true,
	}

	// Module cache for load() — prevents re-executing the same file.
	moduleCache := map[string]starlark.StringDict{}

	// Create thread with print and load functions.
	thread := &starlark.Thread{
		Name:  cmdSpec.Name,
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
		Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			// Resolve relative to the extension directory.
			modulePath := filepath.Join(extDir, module)
			if cached, ok := moduleCache[modulePath]; ok {
				return cached, nil
			}

			globals, err := starlark.ExecFileOptions(&fileOpts, thread, modulePath, nil, predeclared)
			if err != nil {
				return nil, fmt.Errorf("load %s: %w", module, err)
			}
			moduleCache[modulePath] = globals

			return globals, nil
		},
	}

	// Execute the command script.
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
		Name:          cmdSpec.Name,
		Help:          cmdSpec.Help,
		RunFunc:       runFunc,
		ExtensionDir:  extDir,
		ExtensionName: spec.Extension,
		globals:       globals,
		predeclared:   predeclared,
		app:           a,
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
	a.commands[cmdName] = cmd

	return nil
}

// buildPredeclared constructs the predeclared environment for Starlark execution.
// If spec is non-nil, it may provide extension-specific bindings (WASM receivers).
func (a *Application) buildPredeclared(spec *extension.ExtensionSpec) starlark.StringDict {

	// Framework-managed receivers.
	predeclared := a.star.BuildReceivers()

	// Add WASM receivers from extension spec
	if spec != nil {
		for _, recv := range spec.Receivers {
			if recv.Builtin || recv.Wasm == "" {
				continue
			}
			key := fmt.Sprintf("%s:%s", spec.Extension, recv.Name)
			if wasmRecv, ok := a.wasmReceivers[key]; ok {
				predeclared[recv.Name] = wasmRecv
			}
		}
	}

	return predeclared
}

// Commands returns all registered commands.
func (a *Application) Commands() map[string]*Command {
	return a.commands
}

// CommandNames implements commands.CommandTree.
func (a *Application) CommandNames() []string {
	names := make([]string, 0, len(a.commands))
	for name := range a.commands {
		names = append(names, name)
	}
	return names
}

// RunCommand implements commands.CommandTree.
func (a *Application) RunCommand(name string, args map[string]string) error {
	cmd, ok := a.commands[name]
	if !ok {
		return fmt.Errorf("command %q not found", name)
	}
	return cmd.Run(args)
}

// CommandHelp implements commands.CommandTree.
func (a *Application) CommandHelp(name string) string {
	spaceName := strings.ReplaceAll(name, ".", " ")
	if cmd, ok := a.commands[spaceName]; ok {
		return cmd.Help
	}
	return ""
}

// CommandFlags implements commands.CommandTree.
func (a *Application) CommandFlags(name string) []commandsprov.CommandFlag {
	spaceName := strings.ReplaceAll(name, ".", " ")
	cmd, ok := a.commands[spaceName]
	if !ok {
		return nil
	}
	flags := make([]commandsprov.CommandFlag, len(cmd.Flags))
	for i, f := range cmd.Flags {
		flags[i] = commandsprov.CommandFlag{Name: f.Name, Help: f.Help, Default: f.Default}
	}
	return flags
}

// Close releases all resources held by the application.
// This includes closing all WASM hosts.
func (a *Application) Close() error {
	var errs []error
	for name, host := range a.wasmHosts {
		if err := host.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close WASM host %s: %w", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
