// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSpec_FullExtension(t *testing.T) {
	yaml := `
extension: lint.copyright
description: "Check or fix copyright headers"

receivers:
  - name: copyright
    type: CopyrightChecker
    builtin: true
    description: "Copyright header primitives"
    functions:
      check: "Verify files have correct headers"
      fix: "Add or update headers"

command:
  help: "Check or fix copyright headers"
  implementation: lint-copyright.star

flags:
  - name: fix
    type: bool
    default: "false"
    help: Add missing headers

config:
  type: CopyrightConfig
  fields:
    enabled: bool
    license: string
  defaults:
    enabled: false
    license: "auto"
`
	spec, err := ParseSpecFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSpecFromBytes failed: %v", err)
	}

	if spec.Extension != "lint.copyright" {
		t.Errorf("Extension = %q, want %q", spec.Extension, "lint.copyright")
	}

	if spec.Description != "Check or fix copyright headers" {
		t.Errorf("Description = %q, want %q", spec.Description, "Check or fix copyright headers")
	}

	if len(spec.Receivers) != 1 {
		t.Fatalf("len(Receivers) = %d, want 1", len(spec.Receivers))
	}

	r := spec.Receivers[0]
	if r.Name != "copyright" {
		t.Errorf("Receiver.Name = %q, want %q", r.Name, "copyright")
	}
	if !r.Builtin {
		t.Error("Receiver.Builtin = false, want true")
	}
	if r.Type != "CopyrightChecker" {
		t.Errorf("Receiver.Type = %q, want %q", r.Type, "CopyrightChecker")
	}

	if spec.Command == nil {
		t.Fatal("Command is nil")
	}
	if spec.Command.Implementation != "lint-copyright.star" {
		t.Errorf("Command.Implementation = %q, want %q", spec.Command.Implementation, "lint-copyright.star")
	}

	if len(spec.Flags) != 1 {
		t.Fatalf("len(Flags) = %d, want 1", len(spec.Flags))
	}
	if spec.Flags[0].Name != "fix" {
		t.Errorf("Flag.Name = %q, want %q", spec.Flags[0].Name, "fix")
	}

	if spec.Config == nil {
		t.Fatal("Config is nil")
	}
	if spec.Config.Fields["enabled"] != "bool" {
		t.Errorf("Config.Fields[enabled] = %q, want %q", spec.Config.Fields["enabled"], "bool")
	}
}

func TestParseSpec_BindingOnly(t *testing.T) {
	yaml := `
extension: copyright
description: "Copyright header primitives"

receivers:
  - name: copyright
    type: CopyrightChecker
    builtin: true
    functions:
      check: "Verify headers"
      fix: "Fix headers"
`
	spec, err := ParseSpecFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSpecFromBytes failed: %v", err)
	}

	if !spec.HasReceivers() {
		t.Error("HasReceivers() = false, want true")
	}
	if spec.HasCommand() {
		t.Error("HasCommand() = true, want false")
	}
	if spec.HasConfig() {
		t.Error("HasConfig() = true, want false")
	}
}

func TestParseSpec_CommandOnly(t *testing.T) {
	yaml := `
extension: lint.all
description: "Run all linters"

command:
  help: "Run all configured linters"
  implementation: lint-all.star
`
	spec, err := ParseSpecFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSpecFromBytes failed: %v", err)
	}

	if spec.HasReceivers() {
		t.Error("HasReceivers() = true, want false")
	}
	if !spec.HasCommand() {
		t.Error("HasCommand() = false, want true")
	}
}

func TestParseSpec_WasmExtension(t *testing.T) {
	yaml := `
extension: custom.linter
description: "Custom analysis"

receivers:
  - name: customlint
    wasm: customlint.wasm
    functions:
      analyze: "Run analysis"

capabilities:
  fs:
    read: ["/workspace"]
    write: []
  host_calls:
    - shell.run
`
	spec, err := ParseSpecFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSpecFromBytes failed: %v", err)
	}

	if spec.IsBuiltin() {
		t.Error("IsBuiltin() = true, want false")
	}
	if !spec.HasWasmReceivers() {
		t.Error("HasWasmReceivers() = false, want true")
	}
	if spec.Capabilities == nil {
		t.Fatal("Capabilities is nil")
	}
	if len(spec.Capabilities.FS.Read) != 1 {
		t.Errorf("len(Capabilities.FS.Read) = %d, want 1", len(spec.Capabilities.FS.Read))
	}
}

func TestValidate_EmptyExtension(t *testing.T) {
	yaml := `
extension: empty
description: "No receivers or command"
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for extension with no receivers or command")
	}
}

func TestValidate_EmptyExtensionName(t *testing.T) {
	yaml := `
extension: ""
receivers:
  - name: test
    builtin: true
    type: TestReceiver
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for empty extension name")
	}
}

func TestValidate_BuiltinWithoutType(t *testing.T) {
	yaml := `
extension: test
receivers:
  - name: test
    builtin: true
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for builtin receiver without type")
	}
}

func TestValidate_WasmWithoutPath(t *testing.T) {
	yaml := `
extension: test
receivers:
  - name: test
    builtin: false
capabilities:
  fs:
    read: []
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for non-builtin receiver without wasm path")
	}
}

func TestValidate_WasmWithoutCapabilities(t *testing.T) {
	yaml := `
extension: test
receivers:
  - name: test
    wasm: test.wasm
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for wasm receiver without capabilities")
	}
}

func TestValidate_InvalidFlagType(t *testing.T) {
	yaml := `
extension: test
command:
  help: "Test"
flags:
  - name: test
    type: invalid
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid flag type")
	}
}

func TestCommandPath(t *testing.T) {
	spec := &ExtensionSpec{Extension: "lint.copyright"}
	path := spec.CommandPath()

	if len(path) != 2 {
		t.Fatalf("len(CommandPath()) = %d, want 2", len(path))
	}
	if path[0] != "lint" || path[1] != "copyright" {
		t.Errorf("CommandPath() = %v, want [lint, copyright]", path)
	}
}

func TestToConfigSpec(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "test",
		Command:   &CommandSpec{Help: "Test"},
		Config: &ConfigDef{
			Type: "TestConfig",
			Fields: map[string]string{
				"enabled": "bool",
				"name":    "string",
			},
			Defaults: map[string]interface{}{
				"enabled": false,
				"name":    "default",
			},
		},
	}

	configSpec := spec.ToConfigSpec()

	if configSpec.Type != "TestConfig" {
		t.Errorf("Type = %q, want %q", configSpec.Type, "TestConfig")
	}
	if configSpec.Fields["enabled"] != "bool" {
		t.Errorf("Fields[enabled] = %q, want %q", configSpec.Fields["enabled"], "bool")
	}
	if configSpec.Defaults["enabled"] != false {
		t.Errorf("Defaults[enabled] = %v, want false", configSpec.Defaults["enabled"])
	}
}

func TestToConfigSpec_NoConfig(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "test",
		Command:   &CommandSpec{Help: "Test"},
	}

	configSpec := spec.ToConfigSpec()

	if configSpec.Type != "" {
		t.Errorf("Type = %q, want empty", configSpec.Type)
	}
}

func TestParseSpec_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extension.yaml")

	yaml := `
extension: test.example
description: "Test extension"
command:
  help: "Test command"
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	spec, err := ParseSpec(path)
	if err != nil {
		t.Fatalf("ParseSpec failed: %v", err)
	}

	if spec.Extension != "test.example" {
		t.Errorf("Extension = %q, want %q", spec.Extension, "test.example")
	}
	if spec.SourcePath != path {
		t.Errorf("SourcePath = %q, want %q", spec.SourcePath, path)
	}
}

func TestGetFlag(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "test",
		Command:   &CommandSpec{Help: "Test"},
		Flags: []FlagSpec{
			{Name: "fix", Type: "bool"},
			{Name: "output", Type: "string"},
		},
	}

	flag := spec.GetFlag("fix")
	if flag == nil {
		t.Fatal("GetFlag(fix) returned nil")
	}
	if flag.Type != "bool" {
		t.Errorf("flag.Type = %q, want %q", flag.Type, "bool")
	}

	if spec.GetFlag("nonexistent") != nil {
		t.Error("GetFlag(nonexistent) should return nil")
	}
}

func TestGetReceiver(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "test",
		Receivers: []ReceiverSpec{
			{Name: "copyright", Type: "CopyrightChecker", Builtin: true},
			{Name: "shell", Type: "ShellRunner", Builtin: true},
		},
	}

	r := spec.GetReceiver("copyright")
	if r == nil {
		t.Fatal("GetReceiver(copyright) returned nil")
	}
	if r.Type != "CopyrightChecker" {
		t.Errorf("receiver.Type = %q, want %q", r.Type, "CopyrightChecker")
	}

	if spec.GetReceiver("nonexistent") != nil {
		t.Error("GetReceiver(nonexistent) should return nil")
	}
}

func TestToConfigSpec_NestedDefaults(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "test",
		Command:   &CommandSpec{Help: "Test"},
		Config: &ConfigDef{
			Type: "TestConfig",
			Fields: map[string]string{
				"items": "[]string",
			},
			Defaults: map[string]interface{}{
				"items": []interface{}{"a", "b", "c"},
				"nested": map[string]interface{}{
					"deep": []interface{}{
						map[string]interface{}{"key": "value"},
					},
				},
			},
		},
	}

	configSpec := spec.ToConfigSpec()

	// Verify nested defaults are copied
	items := configSpec.Defaults["items"].([]interface{})
	if len(items) != 3 {
		t.Errorf("len(items) = %d, want 3", len(items))
	}
}

func TestIsBuiltin_NoReceivers(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "test",
		Command:   &CommandSpec{Help: "Test"},
		Receivers: []ReceiverSpec{},
	}

	// Empty receivers should return true (vacuously true)
	if !spec.IsBuiltin() {
		t.Error("IsBuiltin() with no receivers should return true")
	}
}

func TestParseSpec_ReadError(t *testing.T) {
	_, err := ParseSpec("/nonexistent/path/extension.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestValidate_EmptyReceiverName(t *testing.T) {
	yaml := `
extension: test
receivers:
  - name: ""
    builtin: true
    type: TestType
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for empty receiver name")
	}
}

func TestValidate_InvalidExtensionPath(t *testing.T) {
	yaml := `
extension: "lint..copyright"
command:
  help: "Test"
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid extension path with empty segment")
	}
}

func TestValidate_FlagWithoutName(t *testing.T) {
	yaml := `
extension: test
command:
  help: "Test"
flags:
  - name: ""
    type: bool
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for flag without name")
	}
}

func TestValidate_FlagWithoutType(t *testing.T) {
	yaml := `
extension: test
command:
  help: "Test"
flags:
  - name: "myflag"
    type: ""
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for flag without type")
	}
}

func TestToConfigSpec_NilFields(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "test",
		Command:   &CommandSpec{Help: "Test"},
		Config: &ConfigDef{
			Type:     "TestConfig",
			Fields:   nil,
			Defaults: nil,
		},
	}

	configSpec := spec.ToConfigSpec()

	if configSpec.Fields != nil {
		t.Error("Fields should be nil when source is nil")
	}
	if configSpec.Defaults != nil {
		t.Error("Defaults should be nil when source is nil")
	}
}

func TestToConfigSpec_DeepNestedSlicesAndMaps(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "test",
		Command:   &CommandSpec{Help: "Test"},
		Config: &ConfigDef{
			Type: "TestConfig",
			Fields: map[string]string{
				"complex": "any",
			},
			Defaults: map[string]interface{}{
				// Nested map inside slice
				"items": []interface{}{
					map[string]interface{}{
						"nested_map": map[string]interface{}{
							"deep": "value",
						},
						"nested_slice": []interface{}{"a", "b"},
					},
				},
				// Slice inside map
				"wrapper": map[string]interface{}{
					"list": []interface{}{
						map[string]interface{}{"key": "val"},
					},
				},
				// Simple values
				"simple_string": "hello",
				"simple_int":    42,
			},
		},
	}

	configSpec := spec.ToConfigSpec()

	// Verify deep copy worked
	items := configSpec.Defaults["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	item := items[0].(map[string]interface{})
	nestedMap := item["nested_map"].(map[string]interface{})
	if nestedMap["deep"] != "value" {
		t.Errorf("nestedMap[deep] = %v, want value", nestedMap["deep"])
	}

	nestedSlice := item["nested_slice"].([]interface{})
	if len(nestedSlice) != 2 {
		t.Errorf("len(nestedSlice) = %d, want 2", len(nestedSlice))
	}
}

func TestParseSpecFromBytes_InvalidYAML(t *testing.T) {
	yaml := `
this is not valid yaml: [
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidate_AllFlagTypes(t *testing.T) {
	// Test all valid flag types
	for _, flagType := range []string{"bool", "string", "int", "glob"} {
		yaml := `
extension: test
command:
  help: "Test"
flags:
  - name: "myflag"
    type: ` + flagType + `
`
		_, err := ParseSpecFromBytes([]byte(yaml))
		if err != nil {
			t.Errorf("unexpected error for flag type %q: %v", flagType, err)
		}
	}
}

func TestToConfigSpec_SliceOfSlices(t *testing.T) {
	// Test copySlice with nested slices (slice containing slices)
	spec := &ExtensionSpec{
		Extension: "test",
		Command:   &CommandSpec{Help: "Test"},
		Config: &ConfigDef{
			Type: "TestConfig",
			Fields: map[string]string{
				"matrix": "any",
			},
			Defaults: map[string]interface{}{
				// Matrix is a slice of slices
				"matrix": []interface{}{
					[]interface{}{"a", "b", "c"},
					[]interface{}{"d", "e", "f"},
				},
			},
		},
	}

	configSpec := spec.ToConfigSpec()

	matrix := configSpec.Defaults["matrix"].([]interface{})
	if len(matrix) != 2 {
		t.Fatalf("len(matrix) = %d, want 2", len(matrix))
	}

	row1 := matrix[0].([]interface{})
	if len(row1) != 3 {
		t.Errorf("len(row1) = %d, want 3", len(row1))
	}
	if row1[0] != "a" {
		t.Errorf("row1[0] = %v, want a", row1[0])
	}
}
