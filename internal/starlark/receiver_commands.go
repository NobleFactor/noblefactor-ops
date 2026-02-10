// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"fmt"
	"sort"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// CommandsReceiver provides command tree query and execution operations.
// Implements starlark.Value and starlark.HasAttrs.
type CommandsReceiver struct {
	BaseReceiver
	runtime        *Runtime
	currentCommand string // dot-separated name of currently executing command
}

// NewCommandsReceiver creates a new CommandsReceiver.
func NewCommandsReceiver(runtime *Runtime) *CommandsReceiver {
	return &CommandsReceiver{
		BaseReceiver: NewBaseReceiver("commands"),
		runtime:      runtime,
	}
}

// SetCurrentCommand sets the currently executing command name.
func (r *CommandsReceiver) SetCurrentCommand(name string) {
	r.currentCommand = name
}

// Attr implements starlark.HasAttrs.
func (r *CommandsReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "parent":
		return MakeAttr("commands.parent", r.parent), nil
	case "siblings":
		return MakeAttr("commands.siblings", r.siblings), nil
	case "children":
		return MakeAttr("commands.children", r.children), nil
	case "query":
		return MakeAttr("commands.query", r.query), nil
	case "get":
		return MakeAttr("commands.get", r.get), nil
	case "run":
		return MakeAttr("commands.run", r.run), nil
	case "current":
		return MakeAttr("commands.current", r.current), nil
	default:
		return nil, NoSuchAttrError("commands", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *CommandsReceiver) AttrNames() []string {
	return []string{"children", "current", "get", "parent", "query", "run", "siblings"}
}

// parent returns the parent command path of the current command.
// commands.parent() -> string
// Example: "lint.all" -> "lint"
func (r *CommandsReceiver) parent(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("commands.parent", args, kwargs); err != nil {
		return nil, err
	}

	parent := getParentPath(r.currentCommand)
	return starlark.String(parent), nil
}

// current returns the name of the currently executing command.
// commands.current() -> string
func (r *CommandsReceiver) current(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("commands.current", args, kwargs); err != nil {
		return nil, err
	}

	return starlark.String(r.currentCommand), nil
}

// siblings returns all sibling commands (same parent, excluding self).
// commands.siblings() -> list[CommandRef]
func (r *CommandsReceiver) siblings(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("commands.siblings", args, kwargs); err != nil {
		return nil, err
	}

	parent := getParentPath(r.currentCommand)
	var siblings []starlark.Value

	for name, cmd := range r.runtime.Commands() {
		// Convert space-separated to dot-separated for comparison
		dotName := strings.ReplaceAll(name, " ", ".")

		// Check if same parent and not self
		if getParentPath(dotName) == parent && dotName != r.currentCommand {
			siblings = append(siblings, NewCommandRef(dotName, cmd, r.runtime))
		}
	}

	// Sort for consistent ordering
	sort.Slice(siblings, func(i, j int) bool {
		return siblings[i].(*CommandRef).name < siblings[j].(*CommandRef).name
	})

	return starlark.NewList(siblings), nil
}

// children returns all child commands of the given parent path.
// commands.children(parent?) -> list[CommandRef]
// If parent is not specified, uses current command's parent.
func (r *CommandsReceiver) children(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var parent string
	if err := starlark.UnpackArgs("commands.children", args, kwargs, "parent?", &parent); err != nil {
		return nil, err
	}

	if parent == "" {
		parent = getParentPath(r.currentCommand)
	}

	var children []starlark.Value

	for name, cmd := range r.runtime.Commands() {
		dotName := strings.ReplaceAll(name, " ", ".")

		// Check if direct child of parent
		if getParentPath(dotName) == parent {
			children = append(children, NewCommandRef(dotName, cmd, r.runtime))
		}
	}

	// Sort for consistent ordering
	sort.Slice(children, func(i, j int) bool {
		return children[i].(*CommandRef).name < children[j].(*CommandRef).name
	})

	return starlark.NewList(children), nil
}

// query returns commands matching a pattern.
// commands.query(pattern) -> list[CommandRef]
// Pattern supports * wildcard: "lint.*" matches all lint commands.
func (r *CommandsReceiver) query(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern string
	if err := starlark.UnpackArgs("commands.query", args, kwargs, "pattern", &pattern); err != nil {
		return nil, err
	}

	var matches []starlark.Value

	for name, cmd := range r.runtime.Commands() {
		dotName := strings.ReplaceAll(name, " ", ".")

		if matchPattern(dotName, pattern) {
			matches = append(matches, NewCommandRef(dotName, cmd, r.runtime))
		}
	}

	// Sort for consistent ordering
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].(*CommandRef).name < matches[j].(*CommandRef).name
	})

	return starlark.NewList(matches), nil
}

// get returns a specific command by name.
// commands.get(name) -> CommandRef | None
func (r *CommandsReceiver) get(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs("commands.get", args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	// Try dot-separated name first, then space-separated
	spaceName := strings.ReplaceAll(name, ".", " ")

	if cmd, ok := r.runtime.Commands()[spaceName]; ok {
		return NewCommandRef(name, cmd, r.runtime), nil
	}

	return starlark.None, nil
}

// run executes a command by name with the given arguments.
// commands.run(name, **kwargs) -> struct
func (r *CommandsReceiver) run(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackPositionalArgs("commands.run", args, nil, 1, &name); err != nil {
		return nil, err
	}

	// Convert remaining kwargs to string map
	cmdArgs := make(map[string]string)
	for _, kv := range kwargs {
		key := string(kv[0].(starlark.String))
		val := kv[1]
		cmdArgs[key] = starlarkValueToString(val)
	}

	// Find the command
	spaceName := strings.ReplaceAll(name, ".", " ")
	cmd, ok := r.runtime.Commands()[spaceName]
	if !ok {
		return nil, fmt.Errorf("commands.run: command %q not found", name)
	}

	// Execute the command
	err := cmd.Run(cmdArgs)

	// Return result struct
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"passed": starlark.Bool(err == nil),
		"error":  starlark.String(errorToString(err)),
	}), nil
}

// =============================================================================
// CommandRef - Starlark value wrapping a Command
// =============================================================================

// CommandRef wraps a Command for use in Starlark.
type CommandRef struct {
	name    string
	cmd     *Command
	runtime *Runtime
}

// NewCommandRef creates a new CommandRef.
func NewCommandRef(name string, cmd *Command, runtime *Runtime) *CommandRef {
	return &CommandRef{
		name:    name,
		cmd:     cmd,
		runtime: runtime,
	}
}

// String implements starlark.Value.
func (r *CommandRef) String() string {
	return fmt.Sprintf("<command %s>", r.name)
}

// Type implements starlark.Value.
func (r *CommandRef) Type() string {
	return "command"
}

// Freeze implements starlark.Value.
func (r *CommandRef) Freeze() {}

// Truth implements starlark.Value.
func (r *CommandRef) Truth() starlark.Bool {
	return true
}

// Hash implements starlark.Value.
func (r *CommandRef) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: command")
}

// Attr implements starlark.HasAttrs.
func (r *CommandRef) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return starlark.String(r.name), nil
	case "help":
		return starlark.String(r.cmd.Help), nil
	case "flags":
		return r.flagsList(), nil
	case "run":
		return starlark.NewBuiltin("command.run", r.run), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("command has no .%s attribute", name))
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *CommandRef) AttrNames() []string {
	return []string{"flags", "help", "name", "run"}
}

// flagsList returns the command's flags as a Starlark list.
func (r *CommandRef) flagsList() starlark.Value {
	var flags []starlark.Value
	for _, f := range r.cmd.Flags {
		flags = append(flags, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":    starlark.String(f.Name),
			"help":    starlark.String(f.Help),
			"default": starlark.String(f.Default),
		}))
	}
	return starlark.NewList(flags)
}

// run executes the command with the given arguments.
func (r *CommandRef) run(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Convert kwargs to string map
	cmdArgs := make(map[string]string)
	for _, kv := range kwargs {
		key := string(kv[0].(starlark.String))
		val := kv[1]
		cmdArgs[key] = starlarkValueToString(val)
	}

	// Execute the command
	err := r.cmd.Run(cmdArgs)

	// Return result struct
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"passed": starlark.Bool(err == nil),
		"error":  starlark.String(errorToString(err)),
	}), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// getParentPath returns the parent path of a dot-separated command name.
// "lint.all" -> "lint"
// "lint" -> ""
func getParentPath(name string) string {
	lastDot := strings.LastIndex(name, ".")
	if lastDot == -1 {
		return ""
	}
	return name[:lastDot]
}

// matchPattern checks if a name matches a pattern with * wildcards.
// "lint.go" matches "lint.*"
// "lint.go" matches "*.go"
// "lint.go" matches "*"
func matchPattern(name, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Handle prefix wildcard: "lint.*"
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(name, prefix+".") || name == prefix
	}

	// Handle suffix wildcard: "*.go"
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(name, suffix)
	}

	// Exact match
	return name == pattern
}

// starlarkValueToString converts a Starlark value to a string for command args.
func starlarkValueToString(v starlark.Value) string {
	switch val := v.(type) {
	case starlark.String:
		return string(val)
	case starlark.Bool:
		if val {
			return "true"
		}
		return "false"
	case starlark.Int:
		return val.String()
	default:
		return v.String()
	}
}

// errorToString converts an error to a string, returning empty string for nil.
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
