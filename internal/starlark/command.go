// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"errors"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Command represents a Starlark-defined command.
type Command struct {
	Name          string
	Help          string
	Flags         []Flag
	RunFunc       starlark.Callable
	ExtensionDir  string // Absolute path to the extension's directory
	ExtensionName string // Extension identifier (e.g., "com.noblefactor.devlore.Ops")
	globals       starlark.StringDict
	predeclared   starlark.StringDict
	runtime       *Runtime // Reference to runtime for command tree access
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

	ext := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"dir":  starlark.String(c.ExtensionDir),
		"name": starlark.String(c.ExtensionName),
	})

	ctx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"args":      argsDict,
		"dry_run":   starlark.Bool(DryRun),
		"extension": ext,
	})

	// Set current command context on the commands receiver
	if cmds, ok := c.predeclared["commands"].(*CommandsReceiver); ok {
		cmds.SetCurrentCommand(c.Name)
	}

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

