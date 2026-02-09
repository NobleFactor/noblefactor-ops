// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"fmt"
	"testing"

	"go.starlark.net/starlark"
)

func TestCommand_Run(t *testing.T) {
	tests := []struct {
		name    string
		runFunc func(ctx starlark.Value) (starlark.Value, error)
		args    map[string]string
		wantErr bool
	}{
		{
			name: "successful run with no args",
			runFunc: func(_ starlark.Value) (starlark.Value, error) {
				return starlark.None, nil
			},
			args:    map[string]string{},
			wantErr: false,
		},
		{
			name: "successful run with args",
			runFunc: func(ctx starlark.Value) (starlark.Value, error) {
				// Verify args are accessible
				argsAttr, err := ctx.(starlark.HasAttrs).Attr("args")
				if err != nil {
					return nil, err
				}
				argsDict := argsAttr.(*starlark.Dict)
				val, _, _ := argsDict.Get(starlark.String("name"))
				if val.(starlark.String) != "test" {
					t.Errorf("expected arg 'name' to be 'test', got %v", val)
				}
				return starlark.None, nil
			},
			args:    map[string]string{"name": "test"},
			wantErr: false,
		},
		{
			name: "run with dry_run context",
			runFunc: func(ctx starlark.Value) (starlark.Value, error) {
				dryRunAttr, err := ctx.(starlark.HasAttrs).Attr("dry_run")
				if err != nil {
					return nil, err
				}
				// DryRun should be false by default in tests
				if dryRunAttr != starlark.False {
					t.Errorf("expected dry_run to be False, got %v", dryRunAttr)
				}
				return starlark.None, nil
			},
			args:    map[string]string{},
			wantErr: false,
		},
		{
			name: "run function returns error",
			runFunc: func(_ starlark.Value) (starlark.Value, error) {
				return nil, fmt.Errorf("intentional error")
			},
			args:    map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a wrapper callable for the test function
			runnable := &testCallable{fn: tt.runFunc}

			cmd := &Command{
				Name:    "test-cmd",
				Help:    "Test command",
				RunFunc: runnable,
			}

			err := cmd.Run(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Command.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommand_RunWithDryRun(t *testing.T) {
	// Save and restore DryRun state
	originalDryRun := DryRun
	defer func() { DryRun = originalDryRun }()

	DryRun = true

	runnable := &testCallable{fn: func(ctx starlark.Value) (starlark.Value, error) {
		dryRunAttr, err := ctx.(starlark.HasAttrs).Attr("dry_run")
		if err != nil {
			return nil, err
		}
		if dryRunAttr != starlark.True {
			t.Errorf("expected dry_run to be True when DryRun=true, got %v", dryRunAttr)
		}
		return starlark.None, nil
	}}

	cmd := &Command{
		Name:    "test-cmd",
		Help:    "Test command",
		RunFunc: runnable,
	}

	if err := cmd.Run(map[string]string{}); err != nil {
		t.Errorf("Command.Run() unexpected error: %v", err)
	}
}

func TestParseFlag(t *testing.T) {
	tests := []struct {
		name     string
		dict     map[string]starlark.Value
		wantFlag Flag
		wantErr  bool
	}{
		{
			name: "complete flag",
			dict: map[string]starlark.Value{
				"name":     starlark.String("verbose"),
				"help":     starlark.String("Enable verbose output"),
				"default":  starlark.String("false"),
				"required": starlark.Bool(false),
			},
			wantFlag: Flag{
				Name:     "verbose",
				Help:     "Enable verbose output",
				Default:  "false",
				Required: false,
			},
			wantErr: false,
		},
		{
			name: "required flag",
			dict: map[string]starlark.Value{
				"name":     starlark.String("input"),
				"help":     starlark.String("Input file path"),
				"required": starlark.Bool(true),
			},
			wantFlag: Flag{
				Name:     "input",
				Help:     "Input file path",
				Required: true,
			},
			wantErr: false,
		},
		{
			name: "minimal flag - name only",
			dict: map[string]starlark.Value{
				"name": starlark.String("flag"),
			},
			wantFlag: Flag{
				Name: "flag",
			},
			wantErr: false,
		},
		{
			name:     "missing name - error",
			dict:     map[string]starlark.Value{},
			wantFlag: Flag{},
			wantErr:  true,
		},
		{
			name: "empty name - error",
			dict: map[string]starlark.Value{
				"name": starlark.String(""),
			},
			wantFlag: Flag{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build starlark.Dict from map
			d := starlark.NewDict(len(tt.dict))
			for k, v := range tt.dict {
				if err := d.SetKey(starlark.String(k), v); err != nil {
					t.Fatalf("failed to set dict key: %v", err)
				}
			}

			got, err := parseFlag(d)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Name != tt.wantFlag.Name {
					t.Errorf("parseFlag() Name = %v, want %v", got.Name, tt.wantFlag.Name)
				}
				if got.Help != tt.wantFlag.Help {
					t.Errorf("parseFlag() Help = %v, want %v", got.Help, tt.wantFlag.Help)
				}
				if got.Default != tt.wantFlag.Default {
					t.Errorf("parseFlag() Default = %v, want %v", got.Default, tt.wantFlag.Default)
				}
				if got.Required != tt.wantFlag.Required {
					t.Errorf("parseFlag() Required = %v, want %v", got.Required, tt.wantFlag.Required)
				}
			}
		})
	}
}

func TestCommandCollector_CommandBuiltin(t *testing.T) {
	tests := []struct {
		name       string
		args       starlark.Tuple
		kwargs     []starlark.Tuple
		wantCmdCnt int
		wantErr    bool
	}{
		{
			name: "valid command with flags",
			args: starlark.Tuple{},
			kwargs: []starlark.Tuple{
				{starlark.String("name"), starlark.String("test.cmd")},
				{starlark.String("help"), starlark.String("A test command")},
				{starlark.String("flags"), buildFlagsList(t, []map[string]starlark.Value{
					{"name": starlark.String("verbose"), "help": starlark.String("Be verbose")},
				})},
				{starlark.String("run"), &testCallable{}},
			},
			wantCmdCnt: 1,
			wantErr:    false,
		},
		{
			name: "valid command without flags",
			args: starlark.Tuple{},
			kwargs: []starlark.Tuple{
				{starlark.String("name"), starlark.String("simple")},
				{starlark.String("help"), starlark.String("A simple command")},
				{starlark.String("run"), &testCallable{}},
			},
			wantCmdCnt: 1,
			wantErr:    false,
		},
		{
			name: "missing required arg - name",
			args: starlark.Tuple{},
			kwargs: []starlark.Tuple{
				{starlark.String("help"), starlark.String("Missing name")},
				{starlark.String("run"), &testCallable{}},
			},
			wantCmdCnt: 0,
			wantErr:    true,
		},
		{
			name: "command without run - allowed but RunFunc will be nil",
			args: starlark.Tuple{},
			kwargs: []starlark.Tuple{
				{starlark.String("name"), starlark.String("no-run")},
				{starlark.String("help"), starlark.String("Missing run")},
			},
			wantCmdCnt: 1, // Registered but with nil RunFunc
			wantErr:    false,
		},
		{
			name: "invalid flag type in list",
			args: starlark.Tuple{},
			kwargs: []starlark.Tuple{
				{starlark.String("name"), starlark.String("bad-flags")},
				{starlark.String("help"), starlark.String("Bad flags")},
				{starlark.String("flags"), starlark.NewList([]starlark.Value{starlark.String("not-a-dict")})},
				{starlark.String("run"), &testCallable{}},
			},
			wantCmdCnt: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &commandCollector{commands: make(map[string]*Command)}
			thread := &starlark.Thread{Name: "test"}

			_, err := collector.commandBuiltin(thread, nil, tt.args, tt.kwargs)
			if (err != nil) != tt.wantErr {
				t.Errorf("commandBuiltin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(collector.commands) != tt.wantCmdCnt {
				t.Errorf("commandBuiltin() registered %d commands, want %d", len(collector.commands), tt.wantCmdCnt)
			}
		})
	}
}

func TestCommandCollector_MultipleCommands(t *testing.T) {
	collector := &commandCollector{commands: make(map[string]*Command)}
	thread := &starlark.Thread{Name: "test"}

	// Register first command
	_, err := collector.commandBuiltin(thread, nil, starlark.Tuple{}, []starlark.Tuple{
		{starlark.String("name"), starlark.String("cmd1")},
		{starlark.String("help"), starlark.String("First command")},
		{starlark.String("run"), &testCallable{}},
	})
	if err != nil {
		t.Fatalf("first command failed: %v", err)
	}

	// Register second command
	_, err = collector.commandBuiltin(thread, nil, starlark.Tuple{}, []starlark.Tuple{
		{starlark.String("name"), starlark.String("cmd2")},
		{starlark.String("help"), starlark.String("Second command")},
		{starlark.String("run"), &testCallable{}},
	})
	if err != nil {
		t.Fatalf("second command failed: %v", err)
	}

	if len(collector.commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(collector.commands))
	}
	if _, ok := collector.commands["cmd1"]; !ok {
		t.Error("expected 'cmd1' to be registered")
	}
	if _, ok := collector.commands["cmd2"]; !ok {
		t.Error("expected 'cmd2' to be registered")
	}
}

func TestCommandCollector_CommandWithMultipleFlags(t *testing.T) {
	collector := &commandCollector{commands: make(map[string]*Command)}
	thread := &starlark.Thread{Name: "test"}

	flags := buildFlagsList(t, []map[string]starlark.Value{
		{"name": starlark.String("verbose"), "help": starlark.String("Be verbose"), "default": starlark.String("false")},
		{"name": starlark.String("output"), "help": starlark.String("Output file"), "required": starlark.Bool(true)},
		{"name": starlark.String("format"), "help": starlark.String("Output format"), "default": starlark.String("json")},
	})

	_, err := collector.commandBuiltin(thread, nil, starlark.Tuple{}, []starlark.Tuple{
		{starlark.String("name"), starlark.String("multi-flag")},
		{starlark.String("help"), starlark.String("Command with multiple flags")},
		{starlark.String("flags"), flags},
		{starlark.String("run"), &testCallable{}},
	})
	if err != nil {
		t.Fatalf("commandBuiltin failed: %v", err)
	}

	cmd := collector.commands["multi-flag"]
	if cmd == nil {
		t.Fatal("command not registered")
	}
	if len(cmd.Flags) != 3 {
		t.Errorf("expected 3 flags, got %d", len(cmd.Flags))
	}

	// Verify flag details
	flagsByName := make(map[string]Flag)
	for _, f := range cmd.Flags {
		flagsByName[f.Name] = f
	}

	if f, ok := flagsByName["verbose"]; !ok {
		t.Error("missing 'verbose' flag")
	} else if f.Default != "false" {
		t.Errorf("verbose default = %q, want %q", f.Default, "false")
	}

	if f, ok := flagsByName["output"]; !ok {
		t.Error("missing 'output' flag")
	} else if !f.Required {
		t.Error("output flag should be required")
	}

	if f, ok := flagsByName["format"]; !ok {
		t.Error("missing 'format' flag")
	} else if f.Default != "json" {
		t.Errorf("format default = %q, want %q", f.Default, "json")
	}
}

// testCallable is a minimal starlark.Callable implementation for testing.
type testCallable struct {
	fn func(ctx starlark.Value) (starlark.Value, error)
}

func (c *testCallable) Name() string { return "test_func" }
func (c *testCallable) String() string { return "test_func" }
func (c *testCallable) Type() string { return "function" }
func (c *testCallable) Freeze() {}
func (c *testCallable) Truth() starlark.Bool { return starlark.True }
func (c *testCallable) Hash() (uint32, error) { return 0, nil }

func (c *testCallable) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if c.fn == nil {
		return starlark.None, nil
	}
	if len(args) > 0 {
		return c.fn(args[0])
	}
	return c.fn(nil)
}

// buildFlagsList creates a starlark.List of flag dicts for testing.
func buildFlagsList(t *testing.T, flags []map[string]starlark.Value) *starlark.List {
	t.Helper()
	var values []starlark.Value
	for _, flagMap := range flags {
		d := starlark.NewDict(len(flagMap))
		for k, v := range flagMap {
			if err := d.SetKey(starlark.String(k), v); err != nil {
				t.Fatalf("failed to set dict key: %v", err)
			}
		}
		values = append(values, d)
	}
	return starlark.NewList(values)
}
