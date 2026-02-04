// SPDX-License-Identifier: MIT
// Copyright (c) 2025 Noble Factor. All rights reserved.

// Package starlark provides a Starlark runtime for nf-ops commands.
package starlark

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"

	"github.com/NobleFactor/noblefactor-ops/internal/cli"
)

// DryRun is set by the --dry-run global flag. When true, side-effect
// bindings (fs.write, fs.mkdir, fs.remove, etc.) log what they would
// do instead of executing.
var DryRun bool

// Runtime manages Starlark script execution.
type Runtime struct {
	commands map[string]*Command
	opsDir   string
}

// NewRuntime creates a new Starlark runtime.
func NewRuntime(opsDir string) *Runtime {
	return &Runtime{
		commands: make(map[string]*Command),
		opsDir:   opsDir,
	}
}

// LoadAll loads all .star files from the ops directory and subdirectories.
func (r *Runtime) LoadAll() error {
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

// Load loads a single .star file and registers its commands.
func (r *Runtime) Load(path string) error {
	// Create thread with print function
	thread := &starlark.Thread{
		Name:  filepath.Base(path),
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	// Create collector for command() calls
	collector := &commandCollector{commands: make(map[string]*Command)}

	// Build predeclared environment
	predeclared := starlark.StringDict{
		"fs":             fsModule(),
		"json":           jsonModule(),
		"yaml":           yamlModule(),
		"schema":         schemaModule(),
		"go":             goModule(),
		"shell":          shellModule(),
		"lint":           lintModule(),
		"config":         configModule(),
		"starlark_parse": starlarkParseModule(),
		"command":        starlark.NewBuiltin("command", collector.commandBuiltin),
		// Output functions in global namespace
		"note":    starlark.NewBuiltin("note", noteBuiltin),
		"warn":    starlark.NewBuiltin("warn", warnBuiltin),
		"error":   starlark.NewBuiltin("error", errorBuiltin),
		"success": starlark.NewBuiltin("success", successBuiltin),
		"fail":    starlark.NewBuiltin("fail", failBuiltin),
	}

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

// Commands returns all registered commands.
func (r *Runtime) Commands() map[string]*Command {
	return r.commands
}

// Command represents a Starlark-defined command.
type Command struct {
	Name        string
	Help        string
	Flags       []Flag
	RunFunc     starlark.Callable
	globals     starlark.StringDict
	predeclared starlark.StringDict
}

// Flag represents a command flag.
type Flag struct {
	Name     string
	Help     string
	Default  string
	Required bool
}

// Run executes the command with the given arguments.
func (c *Command) Run(args map[string]string) error {
	thread := &starlark.Thread{
		Name:  c.Name,
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	// Build context dict
	argsDict := starlark.NewDict(len(args))
	for k, v := range args {
		if err := argsDict.SetKey(starlark.String(k), starlark.String(v)); err != nil {
			return fmt.Errorf("setting arg %q: %w", k, err)
		}
	}

	ctx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"args":    argsDict,
		"dry_run": starlark.Bool(DryRun),
	})

	// Call the run function
	_, err := starlark.Call(thread, c.RunFunc, starlark.Tuple{ctx}, nil)
	if err != nil {
		var evalErr *starlark.EvalError
		if errors.As(err, &evalErr) {
			return fmt.Errorf("%s", evalErr.Backtrace())
		}
		return err
	}
	return nil
}

// commandCollector collects command() calls during script execution.
type commandCollector struct {
	commands map[string]*Command
}

func (c *commandCollector) commandBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, help string
	var flags *starlark.List
	var runFunc starlark.Callable

	if err := starlark.UnpackArgs("command", args, kwargs,
		"name", &name,
		"help", &help,
		"flags?", &flags,
		"run", &runFunc,
	); err != nil {
		return nil, err
	}

	cmd := &Command{
		Name:    name,
		Help:    help,
		RunFunc: runFunc,
	}

	// Parse flags if provided
	if flags != nil {
		iter := flags.Iterate()
		defer iter.Done()
		var v starlark.Value
		for iter.Next(&v) {
			flagDict, ok := v.(*starlark.Dict)
			if !ok {
				return nil, fmt.Errorf("flag must be a dict, got %s", v.Type())
			}
			flag, err := parseFlag(flagDict)
			if err != nil {
				return nil, err
			}
			cmd.Flags = append(cmd.Flags, flag)
		}
	}

	c.commands[name] = cmd
	return starlark.None, nil
}

func parseFlag(d *starlark.Dict) (Flag, error) {
	var flag Flag

	if v, found, err := d.Get(starlark.String("name")); err == nil && found {
		if s, ok := v.(starlark.String); ok {
			flag.Name = string(s)
		}
	}
	if v, found, err := d.Get(starlark.String("help")); err == nil && found {
		if s, ok := v.(starlark.String); ok {
			flag.Help = string(s)
		}
	}
	if v, found, err := d.Get(starlark.String("default")); err == nil && found {
		if s, ok := v.(starlark.String); ok {
			flag.Default = string(s)
		}
	}
	if v, found, err := d.Get(starlark.String("required")); err == nil && found {
		if b, ok := v.(starlark.Bool); ok {
			flag.Required = bool(b)
		}
	}

	if flag.Name == "" {
		return flag, fmt.Errorf("flag missing 'name' field")
	}
	return flag, nil
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
