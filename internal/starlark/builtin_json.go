// SPDX-License-Identifier: MIT
// Copyright (c) 2025 Noble Factor. All rights reserved.

package starlark

import (
	"encoding/json"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// jsonModule returns the json module for encoding/decoding JSON.
func jsonModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "json",
		Members: starlark.StringDict{
			"encode":        starlark.NewBuiltin("json.encode", jsonEncode),
			"encode_indent": starlark.NewBuiltin("json.encode_indent", jsonEncodeIndent),
			"decode":        starlark.NewBuiltin("json.decode", jsonDecode),
		},
	}
}

func jsonEncode(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v starlark.Value
	if err := starlark.UnpackArgs("json.encode", args, kwargs, "value", &v); err != nil {
		return nil, err
	}

	goVal := starlarkToGo(v)
	data, err := json.Marshal(goVal)
	if err != nil {
		return nil, fmt.Errorf("json.encode: %w", err)
	}
	return starlark.String(data), nil
}

func jsonEncodeIndent(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v starlark.Value
	var indent string
	if err := starlark.UnpackArgs("json.encode_indent", args, kwargs, "value", &v, "indent", &indent); err != nil {
		return nil, err
	}

	goVal := starlarkToGo(v)
	data, err := json.MarshalIndent(goVal, "", indent)
	if err != nil {
		return nil, fmt.Errorf("json.encode_indent: %w", err)
	}
	// Add trailing newline for file writing
	return starlark.String(string(data) + "\n"), nil
}

func jsonDecode(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s string
	if err := starlark.UnpackArgs("json.decode", args, kwargs, "data", &s); err != nil {
		return nil, err
	}

	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("json.decode: %w", err)
	}
	return goToStarlark(v), nil
}
