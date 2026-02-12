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
extension: com.noblefactor.star.CopyrightChecker
description: "Check or fix copyright headers"

receivers:
  - name: copyright
    type: CopyrightChecker
    builtin: true
    description: "Copyright header primitives"

commands:
  - name: lint.copyright
    help: "Check or fix copyright headers"
    implementation: commands/lint-copyright.star
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

	if spec.Extension != "com.noblefactor.star.CopyrightChecker" {
		t.Errorf("Extension = %q, want %q", spec.Extension, "com.noblefactor.star.CopyrightChecker")
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

	if len(spec.Commands) != 1 {
		t.Fatalf("len(Commands) = %d, want 1", len(spec.Commands))
	}
	cmd := spec.Commands[0]
	if cmd.Name != "lint.copyright" {
		t.Errorf("Command.Name = %q, want %q", cmd.Name, "lint.copyright")
	}
	if cmd.Implementation != "commands/lint-copyright.star" {
		t.Errorf("Command.Implementation = %q, want %q", cmd.Implementation, "commands/lint-copyright.star")
	}

	if len(cmd.Flags) != 1 {
		t.Fatalf("len(Flags) = %d, want 1", len(cmd.Flags))
	}
	if cmd.Flags[0].Name != "fix" {
		t.Errorf("Flag.Name = %q, want %q", cmd.Flags[0].Name, "fix")
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
extension: com.example.Copyright
description: "Copyright header primitives"

receivers:
  - name: copyright
    type: CopyrightChecker
    builtin: true
`
	spec, err := ParseSpecFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSpecFromBytes failed: %v", err)
	}

	if !spec.HasReceivers() {
		t.Error("HasReceivers() = false, want true")
	}
	if spec.HasCommands() {
		t.Error("HasCommands() = true, want false")
	}
	if spec.HasConfig() {
		t.Error("HasConfig() = true, want false")
	}
}

func TestParseSpec_CommandOnly(t *testing.T) {
	yaml := `
extension: com.example.LintAll
description: "Run all linters"

commands:
  - name: lint.all
    help: "Run all configured linters"
    implementation: commands/lint-all.star
`
	spec, err := ParseSpecFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSpecFromBytes failed: %v", err)
	}

	if spec.HasReceivers() {
		t.Error("HasReceivers() = true, want false")
	}
	if !spec.HasCommands() {
		t.Error("HasCommands() = false, want true")
	}
}

func TestParseSpec_WasmExtension(t *testing.T) {
	yaml := `
extension: com.example.CustomLinter
description: "Custom analysis"

receivers:
  - name: customlint
    wasm: receivers/customlint.wasm
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

	r := spec.GetReceiver("customlint")
	if r == nil {
		t.Fatal("GetReceiver(customlint) returned nil")
	}
	if r.Capabilities == nil {
		t.Fatal("Receiver.Capabilities is nil")
	}
	if len(r.Capabilities.FS.Read) != 1 {
		t.Errorf("len(Capabilities.FS.Read) = %d, want 1", len(r.Capabilities.FS.Read))
	}
}

func TestValidate_EmptyExtension(t *testing.T) {
	yaml := `
extension: com.example.Empty
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

func TestValidate_InvalidReverseDomainName(t *testing.T) {
	yaml := `
extension: "just.two"
receivers:
  - name: test
    builtin: true
    type: TestReceiver
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid reverse domain name (too few segments)")
	}
}

func TestValidate_BuiltinWithoutType(t *testing.T) {
	yaml := `
extension: com.example.Test
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
extension: com.example.Test
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
extension: com.example.Test
receivers:
  - name: test
    wasm: receivers/test.wasm
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for wasm receiver without capabilities")
	}
}

func TestValidate_WasmNotInReceiversDir(t *testing.T) {
	yaml := `
extension: com.example.Test
receivers:
  - name: test
    wasm: test.wasm
    capabilities:
      fs:
        read: []
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for wasm not in receivers/ subdirectory")
	}
}

func TestValidate_CommandNotInCommandsDir(t *testing.T) {
	yaml := `
extension: com.example.Test
commands:
  - name: test
    help: "Test"
    implementation: test.star
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for implementation not in commands/ subdirectory")
	}
}

func TestValidate_InvalidFlagType(t *testing.T) {
	yaml := `
extension: com.example.Test
commands:
  - name: test
    help: "Test"
    implementation: commands/test.star
    flags:
      - name: test
        type: invalid
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid flag type")
	}
}

func TestCommandPaths(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "com.example.Test",
		Commands: []CommandSpec{
			{Name: "lint.copyright", Implementation: "commands/lint-copyright.star"},
			{Name: "lint.go", Implementation: "commands/lint-go.star"},
		},
	}
	paths := spec.CommandPaths()

	if len(paths) != 2 {
		t.Fatalf("len(CommandPaths()) = %d, want 2", len(paths))
	}
	if paths[0][0] != "lint" || paths[0][1] != "copyright" {
		t.Errorf("CommandPaths()[0] = %v, want [lint, copyright]", paths[0])
	}
}

func TestToConfigSpec(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "com.example.Test",
		Commands: []CommandSpec{
			{Name: "test", Help: "Test", Implementation: "commands/test.star"},
		},
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
		Extension: "com.example.Test",
		Commands: []CommandSpec{
			{Name: "test", Help: "Test", Implementation: "commands/test.star"},
		},
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
extension: com.example.Test
description: "Test extension"
commands:
  - name: test
    help: "Test command"
    implementation: commands/test.star
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	spec, err := ParseSpec(path)
	if err != nil {
		t.Fatalf("ParseSpec failed: %v", err)
	}

	if spec.Extension != "com.example.Test" {
		t.Errorf("Extension = %q, want %q", spec.Extension, "com.example.Test")
	}
	if spec.SourcePath != path {
		t.Errorf("SourcePath = %q, want %q", spec.SourcePath, path)
	}
}

func TestGetFlag(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "com.example.Test",
		Commands: []CommandSpec{
			{
				Name:           "test",
				Help:           "Test",
				Implementation: "commands/test.star",
				Flags: []FlagSpec{
					{Name: "fix", Type: "bool"},
					{Name: "output", Type: "string"},
				},
			},
		},
	}

	flag := spec.GetFlag("test", "fix")
	if flag == nil {
		t.Fatal("GetFlag(test, fix) returned nil")
	}
	if flag.Type != "bool" {
		t.Errorf("flag.Type = %q, want %q", flag.Type, "bool")
	}

	if spec.GetFlag("test", "nonexistent") != nil {
		t.Error("GetFlag(test, nonexistent) should return nil")
	}

	if spec.GetFlag("nonexistent", "fix") != nil {
		t.Error("GetFlag(nonexistent, fix) should return nil")
	}
}

func TestGetReceiver(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "com.example.Test",
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

func TestIsBuiltin_NoReceivers(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "com.example.Test",
		Commands: []CommandSpec{
			{Name: "test", Help: "Test", Implementation: "commands/test.star"},
		},
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
extension: com.example.Test
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

func TestValidate_CommandWithoutName(t *testing.T) {
	yaml := `
extension: com.example.Test
commands:
  - name: ""
    help: "Test"
    implementation: commands/test.star
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for command without name")
	}
}

func TestValidate_FlagWithoutName(t *testing.T) {
	yaml := `
extension: com.example.Test
commands:
  - name: test
    help: "Test"
    implementation: commands/test.star
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
extension: com.example.Test
commands:
  - name: test
    help: "Test"
    implementation: commands/test.star
    flags:
      - name: "myflag"
        type: ""
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for flag without type")
	}
}

func TestValidate_AllFlagTypes(t *testing.T) {
	// Test all valid flag types
	for _, flagType := range []string{"bool", "string", "int", "glob"} {
		yaml := `
extension: com.example.Test
commands:
  - name: test
    help: "Test"
    implementation: commands/test.star
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

func TestParseSpecFromBytes_InvalidYAML(t *testing.T) {
	yaml := `
this is not valid yaml: [
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidate_FunctionsFieldRejected(t *testing.T) {
	yaml := `
extension: com.example.Test
receivers:
  - name: test
    wasm: receivers/test.wasm
    functions:
      analyze: "Run analysis"
    capabilities:
      fs:
        read: ["/workspace"]
`
	_, err := ParseSpecFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for receiver with functions field")
	}
}

func TestGetCommand(t *testing.T) {
	spec := &ExtensionSpec{
		Extension: "com.example.Test",
		Commands: []CommandSpec{
			{Name: "lint.go", Help: "Go linting", Implementation: "commands/lint-go.star"},
			{Name: "lint.shell", Help: "Shell linting", Implementation: "commands/lint-shell.star"},
		},
	}

	cmd := spec.GetCommand("lint.go")
	if cmd == nil {
		t.Fatal("GetCommand(lint.go) returned nil")
	}
	if cmd.Help != "Go linting" {
		t.Errorf("cmd.Help = %q, want %q", cmd.Help, "Go linting")
	}

	if spec.GetCommand("nonexistent") != nil {
		t.Error("GetCommand(nonexistent) should return nil")
	}
}
