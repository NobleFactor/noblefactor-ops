// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package starlark

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"gopkg.in/yaml.v3"

	"github.com/NobleFactor/noblefactor-ops/internal/config"
)

// configModule returns the config module for star.yaml operations.
func configModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "config",
		Members: starlark.StringDict{
			"load": starlark.NewBuiltin("config.load", configLoad),
			"show": starlark.NewBuiltin("config.show", configShow),
			"sync": starlark.NewBuiltin("config.sync", configSync),
		},
	}
}

// configLoad loads the merged configuration from the hierarchy.
func configLoad(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("config.load", args, kwargs); err != nil {
		return nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	return configToStarlark(cfg)
}

// configShow loads config with source information.
func configShow(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("config.show", args, kwargs); err != nil {
		return nil, err
	}

	cfg, sources, err := config.LoadWithSources()
	if err != nil {
		return nil, err
	}

	// Convert sources to Starlark
	var sourceList []starlark.Value
	for _, s := range sources {
		sourceList = append(sourceList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"path":   starlark.String(s.Path),
			"exists": starlark.Bool(s.Exists),
		}))
	}

	cfgValue, err := configToStarlark(cfg)
	if err != nil {
		return nil, err
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"config":  cfgValue,
		"sources": starlark.NewList(sourceList),
	}), nil
}

// configSync writes tool-specific config files from star.yaml.
func configSync(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("config.sync", args, kwargs); err != nil {
		return nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	result, err := cfg.Sync()
	if err != nil {
		return nil, err
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"golangci_lint":   starlark.String(result.GolangciLint),
		"markdown_lint":   starlark.String(result.MarkdownLint),
		"files_generated": starlark.MakeInt(result.FilesGenerated),
	}), nil
}

// configToStarlark converts a Config to a Starlark value.
func configToStarlark(cfg *config.Config) (starlark.Value, error) {
	// Marshal to YAML then to a generic map for Starlark conversion
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return mapToStarlark(m), nil
}

// mapToStarlark converts a Go map to a Starlark dict.
func mapToStarlark(m map[string]interface{}) starlark.Value {
	if m == nil {
		return starlark.None
	}

	dict := starlark.NewDict(len(m))
	for k, v := range m {
		_ = dict.SetKey(starlark.String(k), valueToStarlark(v))
	}
	return dict
}

// valueToStarlark converts a Go value to a Starlark value.
func valueToStarlark(v interface{}) starlark.Value {
	switch val := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(val)
	case int:
		return starlark.MakeInt(val)
	case int64:
		return starlark.MakeInt64(val)
	case float64:
		return starlark.Float(val)
	case string:
		return starlark.String(val)
	case []interface{}:
		var items []starlark.Value
		for _, item := range val {
			items = append(items, valueToStarlark(item))
		}
		return starlark.NewList(items)
	case map[string]interface{}:
		return mapToStarlark(val)
	default:
		return starlark.String(starlark.String(v.(string)))
	}
}
