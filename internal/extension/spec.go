// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension

import (
	"fmt"
	"os"
	"strings"

	"github.com/NobleFactor/noblefactor-ops/internal/config"
	"gopkg.in/yaml.v3"
)

// ExtensionSpec describes an extension parsed from YAML.
type ExtensionSpec struct {
	// Extension is the dotted name (e.g., "lint.copyright").
	// Determines command path and config path.
	Extension string `yaml:"extension"`

	// Description is a brief summary of what the extension does.
	Description string `yaml:"description"`

	// Receivers are binding functions provided by this extension.
	// Can be built-in (compiled into binary) or Wasm (loaded at runtime).
	Receivers []ReceiverSpec `yaml:"receivers"`

	// Capabilities defines sandboxing rules for Wasm extensions.
	// Required if any receiver uses Wasm; nil for built-in only.
	Capabilities *Capabilities `yaml:"capabilities"`

	// Command defines a CLI subcommand if this extension provides one.
	// Optional - binding-only extensions have no command.
	Command *CommandSpec `yaml:"command"`

	// Flags are command-line flags for the command.
	Flags []FlagSpec `yaml:"flags"`

	// Config defines the configuration schema for this extension.
	// Optional - not all extensions need configuration.
	Config *ConfigDef `yaml:"config"`

	// SourcePath is the path to the YAML file this was loaded from.
	// Set by ParseSpec, not part of YAML.
	SourcePath string `yaml:"-"`
}

// ReceiverSpec describes binding functions provided by an extension.
type ReceiverSpec struct {
	// Name is the module name exposed to Starlark (e.g., "copyright").
	Name string `yaml:"name"`

	// Type is the Go type name for built-in receivers (e.g., "CopyrightChecker").
	// Only used when Builtin is true.
	Type string `yaml:"type"`

	// Wasm is the path to the .wasm file for external receivers.
	// Relative to the extension directory.
	Wasm string `yaml:"wasm"`

	// Builtin indicates the receiver is compiled into the star binary.
	// When true, Type must be set. When false, Wasm must be set.
	Builtin bool `yaml:"builtin"`

	// Description is a brief summary of what this receiver provides.
	Description string `yaml:"description"`

	// Functions maps function names to descriptions.
	// Keys are snake_case names as exposed to Starlark.
	Functions map[string]string `yaml:"functions"`
}

// CommandSpec describes a CLI subcommand.
type CommandSpec struct {
	// Help is the help text shown for --help.
	Help string `yaml:"help"`

	// Implementation is the path to the Starlark file implementing the command.
	// Relative to the extension directory.
	Implementation string `yaml:"implementation"`
}

// FlagSpec describes a command flag.
type FlagSpec struct {
	// Name is the flag name without dashes (e.g., "fix" for --fix).
	Name string `yaml:"name"`

	// Type is the flag type: bool, string, int, glob.
	Type string `yaml:"type"`

	// Default is the default value as a string.
	Default string `yaml:"default"`

	// Help is the help text for this flag.
	Help string `yaml:"help"`
}

// ConfigDef describes the configuration schema for an extension.
// Used to generate config.ConfigSpec for registration.
type ConfigDef struct {
	// Type is the Go type name (informational).
	Type string `yaml:"type"`

	// Fields maps field names to type strings.
	// Supported: bool, string, int, []string, map[K]V, and nested types.
	Fields map[string]string `yaml:"fields"`

	// Defaults provides default values for fields.
	Defaults map[string]interface{} `yaml:"defaults"`
}

// Capabilities describes sandboxing rules for Wasm extensions.
type Capabilities struct {
	// FS defines filesystem access permissions.
	FS FSCapabilities `yaml:"fs"`

	// HostCalls lists host callback functions the extension can invoke.
	// Examples: "shell.run", "http.get"
	HostCalls []string `yaml:"host_calls"`
}

// FSCapabilities describes filesystem access permissions.
type FSCapabilities struct {
	// Read lists directories the extension can read from.
	Read []string `yaml:"read"`

	// Write lists directories the extension can write to.
	Write []string `yaml:"write"`
}

// ParseSpec parses an extension specification from a YAML file.
func ParseSpec(path string) (*ExtensionSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read extension spec: %w", err)
	}

	var spec ExtensionSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse extension spec: %w", err)
	}

	spec.SourcePath = path

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid extension spec %s: %w", path, err)
	}

	return &spec, nil
}

// ParseSpecFromBytes parses an extension specification from YAML bytes.
func ParseSpecFromBytes(data []byte) (*ExtensionSpec, error) {
	var spec ExtensionSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse extension spec: %w", err)
	}

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid extension spec: %w", err)
	}

	return &spec, nil
}

// Validate checks that the extension spec is valid.
// An extension must have at least one receiver OR one command.
func (s *ExtensionSpec) Validate() error {
	if s.Extension == "" {
		return fmt.Errorf("extension name is required")
	}

	// Validate extension name format (dotted path)
	parts := strings.Split(s.Extension, ".")
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("extension name %q has empty path segment", s.Extension)
		}
	}

	// Must have receivers OR command (or both)
	if len(s.Receivers) == 0 && s.Command == nil {
		return fmt.Errorf("extension must define at least one receiver or command")
	}

	// Validate receivers
	for i, r := range s.Receivers {
		if r.Name == "" {
			return fmt.Errorf("receiver[%d] name is required", i)
		}
		if r.Builtin && r.Type == "" {
			return fmt.Errorf("receiver %q: builtin receiver requires type", r.Name)
		}
		if !r.Builtin && r.Wasm == "" {
			return fmt.Errorf("receiver %q: non-builtin receiver requires wasm path", r.Name)
		}
	}

	// Validate Wasm receivers have capabilities
	if s.HasWasmReceivers() && s.Capabilities == nil {
		return fmt.Errorf("extension with Wasm receivers must define capabilities")
	}

	// Validate flags
	for i, f := range s.Flags {
		if f.Name == "" {
			return fmt.Errorf("flag[%d] name is required", i)
		}
		if f.Type == "" {
			return fmt.Errorf("flag %q: type is required", f.Name)
		}
		switch f.Type {
		case "bool", "string", "int", "glob":
			// valid
		default:
			return fmt.Errorf("flag %q: unknown type %q (expected bool, string, int, or glob)", f.Name, f.Type)
		}
	}

	return nil
}

// CommandPath returns the extension name split into path segments.
// "lint.copyright" -> ["lint", "copyright"]
func (s *ExtensionSpec) CommandPath() []string {
	return strings.Split(s.Extension, ".")
}

// IsBuiltin returns true if all receivers are built-in (no Wasm).
func (s *ExtensionSpec) IsBuiltin() bool {
	for _, r := range s.Receivers {
		if !r.Builtin {
			return false
		}
	}
	return true
}

// HasWasmReceivers returns true if any receiver uses Wasm.
func (s *ExtensionSpec) HasWasmReceivers() bool {
	for _, r := range s.Receivers {
		if !r.Builtin && r.Wasm != "" {
			return true
		}
	}
	return false
}

// HasCommand returns true if this extension provides a CLI command.
func (s *ExtensionSpec) HasCommand() bool {
	return s.Command != nil
}

// HasReceivers returns true if this extension provides binding functions.
func (s *ExtensionSpec) HasReceivers() bool {
	return len(s.Receivers) > 0
}

// HasConfig returns true if this extension has a configuration schema.
func (s *ExtensionSpec) HasConfig() bool {
	return s.Config != nil
}

// ToConfigSpec converts the extension's ConfigDef to config.ConfigSpec.
// Returns empty ConfigSpec if no config is defined.
func (s *ExtensionSpec) ToConfigSpec() config.ConfigSpec {
	if s.Config == nil {
		return config.ConfigSpec{}
	}

	return config.ConfigSpec{
		Type:     s.Config.Type,
		Fields:   copyStringMap(s.Config.Fields),
		Defaults: copyDefaults(s.Config.Defaults),
	}
}

// GetFlag returns the FlagSpec for the given flag name, or nil if not found.
func (s *ExtensionSpec) GetFlag(name string) *FlagSpec {
	for i := range s.Flags {
		if s.Flags[i].Name == name {
			return &s.Flags[i]
		}
	}
	return nil
}

// GetReceiver returns the ReceiverSpec for the given name, or nil if not found.
func (s *ExtensionSpec) GetReceiver(name string) *ReceiverSpec {
	for i := range s.Receivers {
		if s.Receivers[i].Name == name {
			return &s.Receivers[i]
		}
	}
	return nil
}

// copyStringMap creates a copy of a string map.
func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// copyDefaults creates a deep copy of default values.
func copyDefaults(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = copyDefaults(val)
		case []interface{}:
			result[k] = copySlice(val)
		default:
			result[k] = v
		}
	}
	return result
}

// copySlice creates a copy of an interface slice.
func copySlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[string]interface{}:
			result[i] = copyDefaults(val)
		case []interface{}:
			result[i] = copySlice(val)
		default:
			result[i] = v
		}
	}
	return result
}
