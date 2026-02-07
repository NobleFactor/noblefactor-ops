// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/NobleFactor/noblefactor-ops/internal/config"
)

// configModule returns the config module for star.yaml operations.
func configModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "config",
		Members: starlark.StringDict{
			// Note: "load" is a reserved keyword in Starlark, so we use "get"
			"get":  starlark.NewBuiltin("config.get", configLoad),
			"show": starlark.NewBuiltin("config.show", configShow),
			"sync": starlark.NewBuiltin("config.sync", configSync),
		},
	}
}

// configLoad loads the merged configuration from the hierarchy.
// Returns a Starlark struct with attribute access (cfg.lint.copyright.enabled).
func configLoad(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("config.load", args, kwargs); err != nil {
		return nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	return cfg.ToStarlark(), nil
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

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"config":  cfg.ToStarlark(),
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
