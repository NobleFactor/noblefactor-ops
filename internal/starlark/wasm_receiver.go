// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"encoding/json"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/NobleFactor/devlore-cli/pkg/op"
	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// WasmReceiver wraps a WASM module as a Starlark HasAttrs value.
// Each attribute access returns a builtin that invokes the corresponding
// WASM function via shared memory.
type WasmReceiver struct {
	op.Receiver
	module    extension.WasmModule
	functions map[string]bool // Available functions from extension.yaml
}

// NewWasmReceiver creates a receiver wrapping a WASM module.
// The functions parameter specifies which function names are valid attributes.
func NewWasmReceiver(name string, module extension.WasmModule, functions []string) *WasmReceiver {
	funcMap := make(map[string]bool, len(functions))
	for _, fn := range functions {
		funcMap[fn] = true
	}
	return &WasmReceiver{
		Receiver: op.NewReceiver(name),
		module:       module,
		functions:    funcMap,
	}
}

// Attr implements starlark.HasAttrs.
func (r *WasmReceiver) Attr(name string) (starlark.Value, error) {
	if !r.functions[name] {
		return nil, op.NoSuchAttrError(r.String(), name)
	}
	return starlark.NewBuiltin(r.String()+"."+name, r.makeCall(name)), nil
}

// AttrNames implements starlark.HasAttrs.
func (r *WasmReceiver) AttrNames() []string {
	names := make([]string, 0, len(r.functions))
	for fn := range r.functions {
		names = append(names, fn)
	}
	return names
}

// makeCall returns a builtin function that invokes the WASM method.
func (r *WasmReceiver) makeCall(method string) op.BuiltinFunc {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Convert Starlark args to JSON
		params, err := wasmArgsToJSON(args, kwargs)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", r.String(), method, err)
		}

		// Call WASM module
		resultBytes, err := r.module.Call(method, params)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", r.String(), method, err)
		}

		// Handle nil/empty result
		if len(resultBytes) == 0 {
			return starlark.None, nil
		}

		// Convert JSON result back to Starlark
		return wasmJSONToStarlark(resultBytes)
	}
}

// wasmArgsToJSON converts Starlark function arguments to JSON bytes.
// Positional args become an array, kwargs become object properties.
func wasmArgsToJSON(args starlark.Tuple, kwargs []starlark.Tuple) ([]byte, error) {
	// If we have kwargs, build an object
	if len(kwargs) > 0 {
		obj := make(map[string]any)
		for _, kv := range kwargs {
			if len(kv) != 2 {
				continue
			}
			key, ok := starlark.AsString(kv[0])
			if !ok {
				continue
			}
			// Use existing starlarkToGo (returns interface{}, no error)
			obj[key] = starlarkToGo(kv[1])
		}
		// Also include positional args if any
		if len(args) > 0 {
			for i, arg := range args {
				// Use numeric keys for positional args
				obj[fmt.Sprintf("arg%d", i)] = starlarkToGo(arg)
			}
		}
		return json.Marshal(obj)
	}

	// Positional only - if single arg, marshal directly; if multiple, as array
	if len(args) == 0 {
		return []byte("{}"), nil
	}
	if len(args) == 1 {
		val := starlarkToGo(args[0])
		// If it's already a map, marshal directly
		if _, ok := val.(map[string]interface{}); ok {
			return json.Marshal(val)
		}
		// Wrap single value in object with "path" key (common pattern)
		return json.Marshal(map[string]any{"path": val})
	}

	// Multiple positional args - convert to array
	arr := make([]any, len(args))
	for i, arg := range args {
		arr[i] = starlarkToGo(arg)
	}
	return json.Marshal(arr)
}

// wasmJSONToStarlark converts JSON bytes to a Starlark value.
// Unlike goToStarlark which returns dicts for JSON objects, this function
// returns starlarkstruct.Struct values so that Starlark scripts can use
// attribute access (result.field) rather than dict access (result["field"]).
// This ensures WASM receivers behave identically to builtin receivers.
func wasmJSONToStarlark(data []byte) (starlark.Value, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return jsonToStarlarkStruct(v), nil
}

// jsonToStarlarkStruct recursively converts a Go value (from JSON) to a
// Starlark value, using starlarkstruct.Struct for maps instead of Dict.
func jsonToStarlarkStruct(v any) starlark.Value {
	switch x := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(x)
	case float64:
		return starlark.Float(x)
	case string:
		return starlark.String(x)
	case []any:
		items := make([]starlark.Value, len(x))
		for i, item := range x {
			items[i] = jsonToStarlarkStruct(item)
		}
		return starlark.NewList(items)
	case map[string]any:
		dict := make(starlark.StringDict, len(x))
		for k, v := range x {
			dict[k] = jsonToStarlarkStruct(v)
		}
		return starlarkstruct.FromStringDict(starlarkstruct.Default, dict)
	default:
		return starlark.String(fmt.Sprintf("%v", x))
	}
}
