// SPDX-License-Identifier: MIT
// Copyright (c) 2025 Noble Factor. All rights reserved.

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"gopkg.in/yaml.v3"
)

// yamlModule returns the yaml module for encoding/decoding YAML.
func yamlModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "yaml",
		Members: starlark.StringDict{
			"encode": starlark.NewBuiltin("yaml.encode", yamlEncode),
			"decode": starlark.NewBuiltin("yaml.decode", yamlDecode),
		},
	}
}

func yamlEncode(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v starlark.Value
	if err := starlark.UnpackArgs("yaml.encode", args, kwargs, "value", &v); err != nil {
		return nil, err
	}

	goVal := starlarkToGo(v)
	data, err := yaml.Marshal(goVal)
	if err != nil {
		return nil, fmt.Errorf("yaml.encode: %w", err)
	}
	return starlark.String(data), nil
}

func yamlDecode(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s string
	if err := starlark.UnpackArgs("yaml.decode", args, kwargs, "data", &s); err != nil {
		return nil, err
	}

	var v interface{}
	if err := yaml.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("yaml.decode: %w", err)
	}
	return goToStarlark(v), nil
}

// starlarkToGo converts a Starlark value to a Go value.
func starlarkToGo(v starlark.Value) interface{} {
	switch x := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(x)
	case starlark.Int:
		i, _ := x.Int64()
		return i
	case starlark.Float:
		return float64(x)
	case starlark.String:
		return string(x)
	case *starlark.List:
		var items []interface{}
		iter := x.Iterate()
		defer iter.Done()
		var item starlark.Value
		for iter.Next(&item) {
			items = append(items, starlarkToGo(item))
		}
		return items
	case *starlark.Dict:
		m := make(map[string]interface{})
		for _, kv := range x.Items() {
			k, _ := starlark.AsString(kv[0])
			m[k] = starlarkToGo(kv[1])
		}
		return m
	case *starlarkstruct.Struct:
		m := make(map[string]interface{})
		for _, name := range x.AttrNames() {
			val, err := x.Attr(name)
			if err == nil {
				m[name] = starlarkToGo(val)
			}
		}
		return m
	default:
		return v.String()
	}
}

// goToStarlark converts a Go value to a Starlark value.
func goToStarlark(v interface{}) starlark.Value {
	switch x := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(x)
	case int:
		return starlark.MakeInt(x)
	case int64:
		return starlark.MakeInt64(x)
	case float64:
		return starlark.Float(x)
	case string:
		return starlark.String(x)
	case []interface{}:
		var items []starlark.Value
		for _, item := range x {
			items = append(items, goToStarlark(item))
		}
		return starlark.NewList(items)
	case map[string]interface{}:
		d := starlark.NewDict(len(x))
		for k, v := range x {
			if err := d.SetKey(starlark.String(k), goToStarlark(v)); err != nil {
				continue // Skip keys that fail to set
			}
		}
		return d
	case map[interface{}]interface{}:
		d := starlark.NewDict(len(x))
		for k, v := range x {
			kStr := fmt.Sprintf("%v", k)
			if err := d.SetKey(starlark.String(kStr), goToStarlark(v)); err != nil {
				continue // Skip keys that fail to set
			}
		}
		return d
	default:
		return starlark.String(fmt.Sprintf("%v", x))
	}
}
