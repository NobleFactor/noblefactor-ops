// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/NobleFactor/noblefactor-ops/internal/config"
)

// ConfigReceiver provides configuration operations.
// Implements starlark.Value and starlark.HasAttrs.
type ConfigReceiver struct {
	Receiver
	cfg *config.Config
}

// NewConfigReceiver creates a new ConfigReceiver.
func NewConfigReceiver() *ConfigReceiver {
	return &ConfigReceiver{Receiver: NewReceiver("config")}
}

// SetConfig sets the fully-populated config on the receiver.
// Called by the runtime after extensions are loaded and config files are merged.
func (r *ConfigReceiver) SetConfig(cfg *config.Config) {
	r.cfg = cfg
}

// Attr implements starlark.HasAttrs.
func (r *ConfigReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "get":
		return MakeAttr("config.get", r.get), nil
	case "show":
		return MakeAttr("config.show", r.show), nil
	case "sync":
		return MakeAttr("config.sync", r.sync), nil
	default:
		return nil, NoSuchAttrError("config", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *ConfigReceiver) AttrNames() []string {
	return []string{"get", "show", "sync"}
}

// getConfig returns the receiver's config, falling back to Load() if not set.
func (r *ConfigReceiver) getConfig() (*config.Config, error) {
	if r.cfg != nil {
		return r.cfg, nil
	}
	return config.Load()
}

// get loads the merged configuration from the hierarchy.
// Returns a Starlark struct with attribute access (cfg.lint.copyright.enabled).
func (r *ConfigReceiver) get(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("config.get", args, kwargs); err != nil {
		return nil, err
	}

	cfg, err := r.getConfig()
	if err != nil {
		return nil, err
	}

	return cfg.ToStarlark(), nil
}

// show loads config with source information.
func (r *ConfigReceiver) show(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("config.show", args, kwargs); err != nil {
		return nil, err
	}

	cfg, err := r.getConfig()
	if err != nil {
		return nil, err
	}

	// Build sources list from known config file locations
	_, sources, err := config.LoadWithSources()
	if err != nil {
		return nil, err
	}

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

// sync writes tool-specific config files from star/config.yaml.
func (r *ConfigReceiver) sync(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("config.sync", args, kwargs); err != nil {
		return nil, err
	}

	cfg, err := r.getConfig()
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
