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
	// Extension is the reverse domain name identifier.
	// Example: "com.noblefactor.star.CopyrightChecker"
	Extension string `yaml:"extension"`

	// Description is a brief summary of what the extension does.
	Description string `yaml:"description"`

	// Receivers are binding functions provided by this extension.
	// Can be built-in (compiled into binary) or Wasm (loaded at runtime).
	Receivers []ReceiverSpec `yaml:"receivers"`

	// Commands defines CLI subcommands provided by this extension.
	// Each command has its own implementation file in commands/ subdirectory.
	Commands []CommandSpec `yaml:"commands"`

	// Config defines the configuration schema for this extension.
	// Optional - not all extensions need configuration.
	Config *ConfigDef `yaml:"config"`

	// SourcePath is the path to the YAML file this was loaded from.
	// Set by ParseSpec, not part of YAML.
	SourcePath string `yaml:"-"`

	// ExtensionDir is the directory containing the extension.
	// Set by ParseSpec, not part of YAML.
	ExtensionDir string `yaml:"-"`
}

// ReceiverSpec describes binding functions provided by an extension.
type ReceiverSpec struct {
	// Name is the module name exposed to Starlark (e.g., "gitignore").
	Name string `yaml:"name"`

	// Type is the Go type name for built-in receivers (e.g., "FileReceiver").
	// Only used when Builtin is true.
	Type string `yaml:"type"`

	// Wasm is the path to the .wasm file for external receivers.
	// Relative to the extension directory, in receivers/ subdirectory.
	// Example: "receivers/gitignore.wasm"
	Wasm string `yaml:"wasm"`

	// Builtin indicates the receiver is compiled into the star binary.
	// When true, Type must be set. When false, Wasm must be set.
	Builtin bool `yaml:"builtin"`

	// Description is a brief summary of what this receiver provides.
	Description string `yaml:"description"`

	// Functions is deprecated and must not be set.
	// Functions are auto-discovered from WASM exports.
	// Kept in struct for YAML detection — Validate() rejects non-empty values.
	Functions map[string]string `yaml:"functions"`

	// Capabilities defines sandboxing rules for this WASM receiver.
	// Required when Wasm is set; ignored for builtin receivers.
	Capabilities *Capabilities `yaml:"capabilities"`
}

// CommandSpec describes a CLI subcommand.
type CommandSpec struct {
	// Name is the dotted command path (e.g., "lint.copyright").
	// Determines the CLI hierarchy: star lint copyright
	Name string `yaml:"name"`

	// Help is the help text shown for --help.
	Help string `yaml:"help"`

	// Implementation is the path to the Starlark file implementing the command.
	// Relative to the extension directory, in commands/ subdirectory.
	// Example: "commands/lint-copyright.star"
	Implementation string `yaml:"implementation"`

	// Flags are command-line flags for this command.
	Flags []FlagSpec `yaml:"flags"`
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
	// Path is the dotted config path where this extension registers its config.
	// Example: "lint.go" registers under config.lint.go in star/config.yaml.
	// If empty, ConfigPath() derives a path from the command name or extension name.
	Path string `yaml:"path"`

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

	// Validate extension name format (reverse domain name)
	// Example: com.noblefactor.star.CopyrightChecker
	if err := validateReverseDomainName(s.Extension); err != nil {
		return fmt.Errorf("extension name %q: %w", s.Extension, err)
	}

	// Must have receivers OR commands (or both)
	if len(s.Receivers) == 0 && len(s.Commands) == 0 {
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
		// Reject functions field — functions are auto-discovered from WASM exports
		if len(r.Functions) > 0 {
			return fmt.Errorf("receiver %q: 'functions' field is no longer supported; functions are auto-discovered from WASM exports", r.Name)
		}
		// WASM receivers must have capabilities
		if !r.Builtin && r.Wasm != "" && r.Capabilities == nil {
			return fmt.Errorf("receiver %q: WASM receiver requires capabilities", r.Name)
		}
		// WASM path must be in receivers/ subdirectory
		if r.Wasm != "" && !strings.HasPrefix(r.Wasm, "receivers/") {
			return fmt.Errorf("receiver %q: wasm path must be in receivers/ subdirectory", r.Name)
		}
	}

	// Validate commands
	for i, c := range s.Commands {
		if c.Name == "" {
			return fmt.Errorf("command[%d] name is required", i)
		}
		if c.Implementation == "" {
			return fmt.Errorf("command %q: implementation is required", c.Name)
		}
		// Implementation must be in commands/ subdirectory
		if !strings.HasPrefix(c.Implementation, "commands/") {
			return fmt.Errorf("command %q: implementation must be in commands/ subdirectory", c.Name)
		}
		// Validate command flags
		for j, f := range c.Flags {
			if f.Name == "" {
				return fmt.Errorf("command %q flag[%d]: name is required", c.Name, j)
			}
			if f.Type == "" {
				return fmt.Errorf("command %q flag %q: type is required", c.Name, f.Name)
			}
			switch f.Type {
			case "bool", "string", "int", "glob":
				// valid
			default:
				return fmt.Errorf("command %q flag %q: unknown type %q", c.Name, f.Name, f.Type)
			}
		}
	}

	return nil
}

// validateReverseDomainName checks that a name follows reverse domain format.
// Examples: com.noblefactor.star.CopyrightChecker, org.example.MyExtension
func validateReverseDomainName(name string) error {
	parts := strings.Split(name, ".")
	if len(parts) < 3 {
		return fmt.Errorf("must have at least 3 segments (e.g., com.example.Name)")
	}

	// First segment should be a TLD (com, org, io, etc.)
	tld := parts[0]
	validTLDs := map[string]bool{"com": true, "org": true, "io": true, "net": true, "dev": true, "app": true}
	if !validTLDs[tld] {
		return fmt.Errorf("first segment %q should be a TLD (com, org, io, net, dev, app)", tld)
	}

	// All segments must be non-empty
	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("segment %d is empty", i)
		}
	}

	return nil
}

// CommandPaths returns the command paths for all commands in this extension.
// Each command name like "lint.copyright" becomes ["lint", "copyright"].
func (s *ExtensionSpec) CommandPaths() [][]string {
	var paths [][]string
	for _, cmd := range s.Commands {
		paths = append(paths, strings.Split(cmd.Name, "."))
	}
	return paths
}

// GetCommand returns the CommandSpec for the given command name, or nil if not found.
func (s *ExtensionSpec) GetCommand(name string) *CommandSpec {
	for i := range s.Commands {
		if s.Commands[i].Name == name {
			return &s.Commands[i]
		}
	}
	return nil
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

// HasCommands returns true if this extension provides CLI commands.
func (s *ExtensionSpec) HasCommands() bool {
	return len(s.Commands) > 0
}

// HasReceivers returns true if this extension provides binding functions.
func (s *ExtensionSpec) HasReceivers() bool {
	return len(s.Receivers) > 0
}

// HasConfig returns true if this extension has a configuration schema.
func (s *ExtensionSpec) HasConfig() bool {
	return s.Config != nil
}

// ConfigPath returns the dotted path where this extension's config is registered.
// Priority: explicit Config.Path > single command name > extension name.
func (s *ExtensionSpec) ConfigPath() string {
	if s.Config != nil && s.Config.Path != "" {
		return s.Config.Path
	}
	if len(s.Commands) == 1 {
		return s.Commands[0].Name
	}
	return s.Extension
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

// GetFlag returns the FlagSpec for the given command and flag name, or nil if not found.
func (s *ExtensionSpec) GetFlag(cmdName, flagName string) *FlagSpec {
	cmd := s.GetCommand(cmdName)
	if cmd == nil {
		return nil
	}
	for i := range cmd.Flags {
		if cmd.Flags[i].Name == flagName {
			return &cmd.Flags[i]
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
