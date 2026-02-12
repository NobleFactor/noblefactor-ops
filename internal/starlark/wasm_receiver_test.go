// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"encoding/json"
	"fmt"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// mockWasmModule implements extension.WasmModule for testing.
type mockWasmModule struct {
	name      string
	functions []string
	callFn    func(function string, args []byte) ([]byte, error)
}

func (m *mockWasmModule) Name() string                                { return m.name }
func (m *mockWasmModule) Functions() []string                         { return m.functions }
func (m *mockWasmModule) Call(function string, args []byte) ([]byte, error) {
	if m.callFn != nil {
		return m.callFn(function, args)
	}
	return nil, fmt.Errorf("unexpected call to %s", function)
}

func TestJsonToStarlarkStruct_Nil(t *testing.T) {
	result := jsonToStarlarkStruct(nil)
	if result != starlark.None {
		t.Errorf("jsonToStarlarkStruct(nil) = %v, want None", result)
	}
}

func TestJsonToStarlarkStruct_Bool(t *testing.T) {
	if v := jsonToStarlarkStruct(true); v != starlark.True {
		t.Errorf("jsonToStarlarkStruct(true) = %v, want True", v)
	}
	if v := jsonToStarlarkStruct(false); v != starlark.False {
		t.Errorf("jsonToStarlarkStruct(false) = %v, want False", v)
	}
}

func TestJsonToStarlarkStruct_Float(t *testing.T) {
	v := jsonToStarlarkStruct(42.5)
	f, ok := v.(starlark.Float)
	if !ok {
		t.Fatalf("jsonToStarlarkStruct(42.5) type = %T, want starlark.Float", v)
	}
	if float64(f) != 42.5 {
		t.Errorf("value = %v, want 42.5", f)
	}
}

func TestJsonToStarlarkStruct_String(t *testing.T) {
	v := jsonToStarlarkStruct("hello")
	s, ok := v.(starlark.String)
	if !ok {
		t.Fatalf("type = %T, want starlark.String", v)
	}
	if string(s) != "hello" {
		t.Errorf("value = %q, want %q", s, "hello")
	}
}

func TestJsonToStarlarkStruct_Array(t *testing.T) {
	v := jsonToStarlarkStruct([]any{"a", "b", "c"})
	list, ok := v.(*starlark.List)
	if !ok {
		t.Fatalf("type = %T, want *starlark.List", v)
	}
	if list.Len() != 3 {
		t.Errorf("len = %d, want 3", list.Len())
	}
}

// TestJsonToStarlarkStruct_MapReturnsStruct verifies that JSON objects are
// converted to starlarkstruct.Struct, enabling attribute access (result.field)
// rather than dict access (result["field"]). This is critical for WASM
// receivers to behave identically to builtin receivers.
func TestJsonToStarlarkStruct_MapReturnsStruct(t *testing.T) {
	v := jsonToStarlarkStruct(map[string]any{
		"name":  "test",
		"valid": true,
		"count": float64(7),
	})

	// Must be a starlarkstruct.Struct, NOT a starlark.Dict
	s, ok := v.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("type = %T, want *starlarkstruct.Struct (not Dict)", v)
	}

	// Verify attribute access works
	name, err := s.Attr("name")
	if err != nil {
		t.Fatalf("Attr(name) error = %v", err)
	}
	if string(name.(starlark.String)) != "test" {
		t.Errorf("name = %v, want test", name)
	}

	valid, err := s.Attr("valid")
	if err != nil {
		t.Fatalf("Attr(valid) error = %v", err)
	}
	if valid != starlark.True {
		t.Errorf("valid = %v, want True", valid)
	}

	count, err := s.Attr("count")
	if err != nil {
		t.Fatalf("Attr(count) error = %v", err)
	}
	if float64(count.(starlark.Float)) != 7.0 {
		t.Errorf("count = %v, want 7", count)
	}
}

// TestJsonToStarlarkStruct_NestedMapReturnsNestedStruct verifies that
// nested JSON objects also become structs, not dicts.
func TestJsonToStarlarkStruct_NestedMapReturnsNestedStruct(t *testing.T) {
	v := jsonToStarlarkStruct(map[string]any{
		"plan": map[string]any{
			"methods": []any{"run", "validate"},
			"config": map[string]any{
				"timeout": float64(30),
			},
		},
	})

	outer, ok := v.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("outer type = %T, want *starlarkstruct.Struct", v)
	}

	planVal, err := outer.Attr("plan")
	if err != nil {
		t.Fatalf("Attr(plan) error = %v", err)
	}
	plan, ok := planVal.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("plan type = %T, want *starlarkstruct.Struct", planVal)
	}

	configVal, err := plan.Attr("config")
	if err != nil {
		t.Fatalf("Attr(config) error = %v", err)
	}
	config, ok := configVal.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("config type = %T, want *starlarkstruct.Struct", configVal)
	}

	timeout, err := config.Attr("timeout")
	if err != nil {
		t.Fatalf("Attr(timeout) error = %v", err)
	}
	if float64(timeout.(starlark.Float)) != 30.0 {
		t.Errorf("timeout = %v, want 30", timeout)
	}
}

func TestWasmJSONToStarlark_ReturnsStruct(t *testing.T) {
	input := `{"ignored": false, "path": "/workspace/main.go"}`
	v, err := wasmJSONToStarlark([]byte(input))
	if err != nil {
		t.Fatalf("wasmJSONToStarlark() error = %v", err)
	}

	s, ok := v.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("type = %T, want *starlarkstruct.Struct", v)
	}

	ignored, err := s.Attr("ignored")
	if err != nil {
		t.Fatalf("Attr(ignored) error = %v", err)
	}
	if ignored != starlark.False {
		t.Errorf("ignored = %v, want False", ignored)
	}
}

func TestWasmJSONToStarlark_InvalidJSON(t *testing.T) {
	_, err := wasmJSONToStarlark([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestWasmReceiver_Attr(t *testing.T) {
	mod := &mockWasmModule{
		name:      "go",
		functions: []string{"parse_devlore_api", "parse_migrate_knowledge"},
	}
	r := NewWasmReceiver("go", mod, []string{"parse_devlore_api", "parse_migrate_knowledge"})

	// Known function returns a builtin
	attr, err := r.Attr("parse_devlore_api")
	if err != nil {
		t.Fatalf("Attr(parse_devlore_api) error = %v", err)
	}
	if attr == nil {
		t.Fatal("Attr(parse_devlore_api) = nil")
	}
	if _, ok := attr.(*starlark.Builtin); !ok {
		t.Errorf("Attr() type = %T, want *starlark.Builtin", attr)
	}

	// Unknown function returns error
	_, err = r.Attr("unknown_method")
	if err == nil {
		t.Error("expected error for unknown attribute")
	}
}

func TestWasmReceiver_AttrNames(t *testing.T) {
	mod := &mockWasmModule{
		name:      "go",
		functions: []string{"parse_devlore_api"},
	}
	r := NewWasmReceiver("go", mod, []string{"parse_devlore_api"})

	names := r.AttrNames()
	if len(names) != 1 || names[0] != "parse_devlore_api" {
		t.Errorf("AttrNames() = %v, want [parse_devlore_api]", names)
	}
}

func TestWasmReceiver_CallReturnsStruct(t *testing.T) {
	mod := &mockWasmModule{
		name:      "gitignore",
		functions: []string{"matches"},
		callFn: func(function string, args []byte) ([]byte, error) {
			if function != "matches" {
				return nil, fmt.Errorf("unexpected function: %s", function)
			}
			return json.Marshal(map[string]any{"ignored": true})
		},
	}
	r := NewWasmReceiver("gitignore", mod, []string{"matches"})

	attr, err := r.Attr("matches")
	if err != nil {
		t.Fatalf("Attr() error = %v", err)
	}
	builtin := attr.(*starlark.Builtin)

	// Call with a path argument
	result, err := builtin.CallInternal(
		nil,
		starlark.Tuple{starlark.String("vendor/foo.go")},
		nil,
	)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	// Result must be a struct with attribute access
	s, ok := result.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("result type = %T, want *starlarkstruct.Struct", result)
	}

	ignored, err := s.Attr("ignored")
	if err != nil {
		t.Fatalf("Attr(ignored) error = %v", err)
	}
	if ignored != starlark.True {
		t.Errorf("ignored = %v, want True", ignored)
	}
}

func TestWasmReceiver_CallWithNilResult(t *testing.T) {
	mod := &mockWasmModule{
		name:      "go",
		functions: []string{"noop"},
		callFn: func(function string, args []byte) ([]byte, error) {
			return nil, nil // No result
		},
	}
	r := NewWasmReceiver("go", mod, []string{"noop"})

	attr, _ := r.Attr("noop")
	result, err := attr.(*starlark.Builtin).CallInternal(nil, nil, nil)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result != starlark.None {
		t.Errorf("result = %v, want None", result)
	}
}

func TestWasmReceiver_CallError(t *testing.T) {
	mod := &mockWasmModule{
		name:      "go",
		functions: []string{"fail"},
		callFn: func(function string, args []byte) ([]byte, error) {
			return nil, fmt.Errorf("module error")
		},
	}
	r := NewWasmReceiver("go", mod, []string{"fail"})

	attr, _ := r.Attr("fail")
	_, err := attr.(*starlark.Builtin).CallInternal(nil, nil, nil)
	if err == nil {
		t.Error("expected error from WASM call")
	}
}
