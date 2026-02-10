// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestCommandsReceiver_Attr(t *testing.T) {
	r := NewRuntime("ops")
	cmds := NewCommandsReceiver(r)

	tests := []string{"parent", "siblings", "children", "query", "get", "run", "current"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			attr, err := cmds.Attr(name)
			if err != nil {
				t.Errorf("Attr(%q) error = %v", name, err)
			}
			if attr == nil {
				t.Errorf("Attr(%q) = nil", name)
			}
		})
	}

	// Test unknown attribute
	t.Run("unknown", func(t *testing.T) {
		_, err := cmds.Attr("unknown")
		if err == nil {
			t.Error("expected error for unknown attribute")
		}
	})
}

func TestCommandsReceiver_AttrNames(t *testing.T) {
	r := NewRuntime("ops")
	cmds := NewCommandsReceiver(r)

	names := cmds.AttrNames()
	if len(names) != 7 {
		t.Errorf("AttrNames() len = %d, want 7", len(names))
	}
}

func TestGetParentPath(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{"single", "lint", ""},
		{"two parts", "lint.go", "lint"},
		{"three parts", "lint.go.check", "lint.go"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getParentPath(tt.input)
			if got != tt.output {
				t.Errorf("getParentPath(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		pattern string
		want    bool
	}{
		{"exact match", "lint.go", "lint.go", true},
		{"exact no match", "lint.go", "lint.shell", false},
		{"prefix wildcard match", "lint.go", "lint.*", true},
		{"prefix wildcard no match", "setup.tools", "lint.*", false},
		{"suffix wildcard match", "lint.go", "*.go", true},
		{"suffix wildcard no match", "lint.shell", "*.go", false},
		{"star all", "anything", "*", true},
		{"parent matches prefix", "lint", "lint.*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.cmdName, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.cmdName, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestStarlarkValueToString(t *testing.T) {
	tests := []struct {
		name  string
		value starlark.Value
		want  string
	}{
		{"string", starlark.String("hello"), "hello"},
		{"bool true", starlark.Bool(true), "true"},
		{"bool false", starlark.Bool(false), "false"},
		{"int", starlark.MakeInt(42), "42"},
		{"none", starlark.None, "None"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := starlarkValueToString(tt.value)
			if got != tt.want {
				t.Errorf("starlarkValueToString(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestCommandRef_Attrs(t *testing.T) {
	cmd := &Command{
		Name: "test.cmd",
		Help: "Test command",
		Flags: []Flag{
			{Name: "verbose", Help: "Enable verbose", Default: "false"},
		},
	}

	ref := NewCommandRef("test.cmd", cmd, nil)

	// Test name
	name, err := ref.Attr("name")
	if err != nil {
		t.Errorf("Attr(name) error = %v", err)
	}
	if string(name.(starlark.String)) != "test.cmd" {
		t.Errorf("name = %v, want test.cmd", name)
	}

	// Test help
	help, err := ref.Attr("help")
	if err != nil {
		t.Errorf("Attr(help) error = %v", err)
	}
	if string(help.(starlark.String)) != "Test command" {
		t.Errorf("help = %v, want Test command", help)
	}

	// Test flags
	flags, err := ref.Attr("flags")
	if err != nil {
		t.Errorf("Attr(flags) error = %v", err)
	}
	flagList := flags.(*starlark.List)
	if flagList.Len() != 1 {
		t.Errorf("flags len = %d, want 1", flagList.Len())
	}

	// Test run
	run, err := ref.Attr("run")
	if err != nil {
		t.Errorf("Attr(run) error = %v", err)
	}
	if run == nil {
		t.Error("run = nil")
	}

	// Test unknown
	_, err = ref.Attr("unknown")
	if err == nil {
		t.Error("expected error for unknown attribute")
	}
}

func TestCommandRef_String(t *testing.T) {
	ref := NewCommandRef("lint.go", nil, nil)
	if ref.String() != "<command lint.go>" {
		t.Errorf("String() = %q, want <command lint.go>", ref.String())
	}
}

func TestCommandRef_Type(t *testing.T) {
	ref := NewCommandRef("lint.go", nil, nil)
	if ref.Type() != "command" {
		t.Errorf("Type() = %q, want command", ref.Type())
	}
}

func TestCommandsReceiver_Current(t *testing.T) {
	r := NewRuntime("ops")
	cmds := NewCommandsReceiver(r)
	cmds.SetCurrentCommand("lint.all")

	// Get the current method
	currentAttr, _ := cmds.Attr("current")
	builtin := currentAttr.(*starlark.Builtin)

	// Call it
	result, err := builtin.CallInternal(nil, nil, nil)
	if err != nil {
		t.Fatalf("current() error = %v", err)
	}

	if string(result.(starlark.String)) != "lint.all" {
		t.Errorf("current() = %q, want lint.all", result)
	}
}

func TestCommandsReceiver_Parent(t *testing.T) {
	r := NewRuntime("ops")
	cmds := NewCommandsReceiver(r)
	cmds.SetCurrentCommand("lint.all")

	// Get the parent method
	parentAttr, _ := cmds.Attr("parent")
	builtin := parentAttr.(*starlark.Builtin)

	// Call it
	result, err := builtin.CallInternal(nil, nil, nil)
	if err != nil {
		t.Fatalf("parent() error = %v", err)
	}

	if string(result.(starlark.String)) != "lint" {
		t.Errorf("parent() = %q, want lint", result)
	}
}

func TestCommandsReceiver_Siblings(t *testing.T) {
	r := NewRuntime("ops")

	// Register some commands
	r.commands["lint go"] = &Command{Name: "lint.go", Help: "Go linter"}
	r.commands["lint shell"] = &Command{Name: "lint.shell", Help: "Shell linter"}
	r.commands["lint all"] = &Command{Name: "lint.all", Help: "All linters"}
	r.commands["setup tools"] = &Command{Name: "setup.tools", Help: "Setup tools"}

	cmds := NewCommandsReceiver(r)
	cmds.SetCurrentCommand("lint.all")

	// Get the siblings method
	siblingsAttr, _ := cmds.Attr("siblings")
	builtin := siblingsAttr.(*starlark.Builtin)

	// Call it
	result, err := builtin.CallInternal(nil, nil, nil)
	if err != nil {
		t.Fatalf("siblings() error = %v", err)
	}

	siblings := result.(*starlark.List)
	if siblings.Len() != 2 {
		t.Errorf("siblings() len = %d, want 2 (lint.go, lint.shell)", siblings.Len())
	}

	// Verify the siblings are lint.go and lint.shell (not lint.all or setup.tools)
	for i := 0; i < siblings.Len(); i++ {
		ref := siblings.Index(i).(*CommandRef)
		if ref.name != "lint.go" && ref.name != "lint.shell" {
			t.Errorf("unexpected sibling: %s", ref.name)
		}
	}
}

func TestCommandsReceiver_Query(t *testing.T) {
	r := NewRuntime("ops")

	// Register some commands
	r.commands["lint go"] = &Command{Name: "lint.go", Help: "Go linter"}
	r.commands["lint shell"] = &Command{Name: "lint.shell", Help: "Shell linter"}
	r.commands["setup tools"] = &Command{Name: "setup.tools", Help: "Setup tools"}

	cmds := NewCommandsReceiver(r)

	// Get the query method
	queryAttr, _ := cmds.Attr("query")
	builtin := queryAttr.(*starlark.Builtin)

	// Query for lint.*
	result, err := builtin.CallInternal(nil, starlark.Tuple{starlark.String("lint.*")}, nil)
	if err != nil {
		t.Fatalf("query(lint.*) error = %v", err)
	}

	matches := result.(*starlark.List)
	if matches.Len() != 2 {
		t.Errorf("query(lint.*) len = %d, want 2", matches.Len())
	}
}

func TestCommandsReceiver_Get(t *testing.T) {
	r := NewRuntime("ops")
	r.commands["lint go"] = &Command{Name: "lint.go", Help: "Go linter"}

	cmds := NewCommandsReceiver(r)

	// Get the get method
	getAttr, _ := cmds.Attr("get")
	builtin := getAttr.(*starlark.Builtin)

	// Get existing command
	result, err := builtin.CallInternal(nil, starlark.Tuple{starlark.String("lint.go")}, nil)
	if err != nil {
		t.Fatalf("get(lint.go) error = %v", err)
	}
	if result == starlark.None {
		t.Error("get(lint.go) = None, want CommandRef")
	}

	// Get non-existent command
	result, err = builtin.CallInternal(nil, starlark.Tuple{starlark.String("nonexistent")}, nil)
	if err != nil {
		t.Fatalf("get(nonexistent) error = %v", err)
	}
	if result != starlark.None {
		t.Errorf("get(nonexistent) = %v, want None", result)
	}
}
