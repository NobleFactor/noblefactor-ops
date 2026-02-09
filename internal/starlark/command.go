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
