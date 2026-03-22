// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"errors"
	"fmt"
	"strconv"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Command represents a Starlark-defined command.
type Command struct {
	Name          string
	Help          string
	Args          []Arg
	Flags         []Flag
	RunFunc       starlark.Callable
	ExtensionDir  string // Absolute path to the extension's directory
	ExtensionName string // Extension identifier (e.g., "com.noblefactor.devlore.Ops")
	globals       starlark.StringDict
	predeclared   starlark.StringDict
	runtime       *Runtime // Reference to runtime for command tree access
}

// Arg represents a positional argument.
type Arg struct {
	Name     string
	Help     string
	Default  string
	Variadic bool
}

// Flag represents a command flag.
type Flag struct {
	Name     string
	Type     string // "bool", "int", "string", "glob"
	Help     string
	Default  string
	Required bool
}

// Run executes the command with the given flag values and optional positional arguments.
func (c *Command) Run(flags map[string]string, positional ...string) error {
	thread := &starlark.Thread{
		Name:  c.Name,
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	// Build flag type lookup for native starlark types.
	flagTypes := make(map[string]string, len(c.Flags))
	for _, f := range c.Flags {
		flagTypes[f.Name] = f.Type
	}

	// Build context dict with native types based on flag spec.
	argsDict := starlark.NewDict(len(flags) + len(c.Args))
	for k, v := range flags {
		sv := flagToStarlark(flagTypes[k], v)
		if err := argsDict.SetKey(starlark.String(k), sv); err != nil {
			return fmt.Errorf("setting flag %q: %w", k, err)
		}
	}

	// Map positional args to named entries using the arg spec.
	for _, arg := range c.Args {
		if arg.Variadic {
			vals := make([]starlark.Value, len(positional))
			for i, v := range positional {
				vals[i] = starlark.String(v)
			}
			if len(vals) == 0 && arg.Default != "" {
				vals = []starlark.Value{starlark.String(arg.Default)}
			}
			if err := argsDict.SetKey(starlark.String(arg.Name), starlark.NewList(vals)); err != nil {
				return fmt.Errorf("setting arg %q: %w", arg.Name, err)
			}
		} else if len(positional) > 0 {
			if err := argsDict.SetKey(starlark.String(arg.Name), starlark.String(positional[0])); err != nil {
				return fmt.Errorf("setting arg %q: %w", arg.Name, err)
			}
			positional = positional[1:]
		} else if arg.Default != "" {
			if err := argsDict.SetKey(starlark.String(arg.Name), starlark.String(arg.Default)); err != nil {
				return fmt.Errorf("setting arg %q: %w", arg.Name, err)
			}
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

	// Set current command name in context data for the commands provider.
	if c.runtime != nil && c.runtime.data != nil {
		c.runtime.data["current_command"] = c.Name
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

// flagToStarlark converts a string value to the appropriate starlark type based on
// the flag type from the extension spec.
func flagToStarlark(flagType, value string) starlark.Value {
	switch flagType {
	case "bool":
		return starlark.Bool(value == "true")
	case "int":
		n, _ := strconv.Atoi(value)
		return starlark.MakeInt(n)
	default:
		return starlark.String(value)
	}
}
