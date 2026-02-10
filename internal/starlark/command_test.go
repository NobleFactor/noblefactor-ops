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
