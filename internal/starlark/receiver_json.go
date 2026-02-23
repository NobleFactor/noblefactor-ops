// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"encoding/json"
	"fmt"

	"go.starlark.net/starlark"

	"github.com/NobleFactor/devlore-cli/pkg/op"
)

// JSONReceiver provides JSON encoding/decoding operations.
// Implements starlark.Value and starlark.HasAttrs.
type JSONReceiver struct {
	op.Receiver
}

// NewJSONReceiver creates a new JSONReceiver.
func NewJSONReceiver() *JSONReceiver {
	return &JSONReceiver{Receiver: op.NewReceiver("json")}
}

// Attr implements starlark.HasAttrs.
func (r *JSONReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "encode":
		return op.MakeAttr("json.encode", r.encode), nil
	case "encode_indent":
		return op.MakeAttr("json.encode_indent", r.encodeIndent), nil
	case "decode":
		return op.MakeAttr("json.decode", r.decode), nil
	default:
		return nil, op.NoSuchAttrError("json", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *JSONReceiver) AttrNames() []string {
	return []string{"decode", "encode", "encode_indent"}
}

// encode encodes a Starlark value as JSON.
func (r *JSONReceiver) encode(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

// encodeIndent encodes a Starlark value as indented JSON.
func (r *JSONReceiver) encodeIndent(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

// decode decodes JSON into a Starlark value.
func (r *JSONReceiver) decode(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
