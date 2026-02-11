// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package config

import (
	"go.starlark.net/starlark"
)

// Config provides a unified view of all configuration.
// It combines builtin config (lint, precommit) with extension-registered config.
// Consumers should use this type - the underlying implementation is private.
type Config struct {
	builtin    *builtinConfig
	extensions *extensionsConfig
}

// Load loads the unified configuration from star/config.yaml files.
// This loads both builtin config and prepares for extension registration.
func Load() (*Config, error) {
	builtin, err := loadBuiltin()
	if err != nil {
		return nil, err
	}

	return &Config{
		builtin:    builtin,
		extensions: newExtensionsConfig("star/config.yaml"),
	}, nil
}

// LoadWithSources loads configuration and returns the source of each file.
func LoadWithSources() (*Config, []ConfigSource, error) {
	builtin, sources, err := loadBuiltinWithSources()
	if err != nil {
		return nil, nil, err
	}

	return &Config{
		builtin:    builtin,
		extensions: newExtensionsConfig("star/config.yaml"),
	}, sources, nil
}

// RegisterExtension registers an extension's config at a dotted path.
// Creates intermediate elements as needed.
func (c *Config) RegisterExtension(path string, spec ConfigSpec) error {
	return c.extensions.registerExtension(path, spec)
}

// GetSpec returns the ConfigSpec for an extension path.
func (c *Config) GetSpec(path string) (ConfigSpec, bool) {
	return c.extensions.getSpec(path)
}

// Sync generates tool-specific config files from star/config.yaml.
func (c *Config) Sync() (*SyncResult, error) {
	return c.builtin.Sync()
}

// ToStarlark returns the config wrapped for Starlark access.
// This provides a unified view of both builtin and extension config.
func (c *Config) ToStarlark() starlark.Value {
	return &unifiedConfigValue{config: c}
}

// Builtin returns the builtin config for direct typed access.
// This is primarily for internal use by builtin modules.
func (c *Config) Builtin() *builtinConfig {
	return c.builtin
}

// unifiedConfigValue wraps Config for Starlark attribute access.
// It checks extensions first, then falls back to builtin config.
type unifiedConfigValue struct {
	config *Config
}

// Ensure unifiedConfigValue implements the required interfaces.
var (
	_ starlark.Value    = (*unifiedConfigValue)(nil)
	_ starlark.HasAttrs = (*unifiedConfigValue)(nil)
)

// String returns a string representation.
func (v *unifiedConfigValue) String() string {
	return "config"
}

// Type returns the Starlark type name.
func (v *unifiedConfigValue) Type() string {
	return "config"
}

// Freeze makes the value immutable.
func (v *unifiedConfigValue) Freeze() {}

// Truth returns the Starlark truth value.
func (v *unifiedConfigValue) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns a hash for the value.
func (v *unifiedConfigValue) Hash() (uint32, error) {
	return 0, nil
}

// Attr returns the value of the named attribute.
// Checks extensions first, then builtin config.
func (v *unifiedConfigValue) Attr(name string) (starlark.Value, error) {
	// Check extensions first
	if v.config.extensions != nil {
		if child := v.config.extensions.Get(name); child != nil {
			return goToStarlarkReflect(child)
		}
	}

	// Fall back to builtin config
	if v.config.builtin != nil {
		return v.attrFromBuiltin(name)
	}

	return nil, starlark.NoSuchAttrError(name)
}

// attrFromBuiltin gets an attribute from the builtin config.
func (v *unifiedConfigValue) attrFromBuiltin(name string) (starlark.Value, error) {
	switch name {
	case "lint":
		return v.config.builtin.Lint.ToStarlark(), nil
	case "precommit":
		return v.config.builtin.Precommit.ToStarlark(), nil
	default:
		return nil, starlark.NoSuchAttrError(name)
	}
}

// AttrNames returns the names of all available attributes.
func (v *unifiedConfigValue) AttrNames() []string {
	names := []string{"lint", "precommit"}

	// Add extension config names
	if v.config.extensions != nil {
		for name := range v.config.extensions.Children() {
			// Avoid duplicates
			found := false
			for _, n := range names {
				if n == name {
					found = true
					break
				}
			}
			if !found {
				names = append(names, name)
			}
		}
	}

	return names
}
