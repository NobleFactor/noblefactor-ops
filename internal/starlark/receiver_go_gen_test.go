// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"go/format"
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"gopkg.in/yaml.v3"
)

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Copy", "copy"},
		{"WalkTree", "walk_tree"},
		{"ConfigGet", "config_get"},
		{"HTMLParser", "html_parser"},
		{"ID", "id"},
		{"HTTPSProxy", "https_proxy"},
		{"A", "a"},
		{"ABC", "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := camelToSnake(tc.input)
			if got != tc.want {
				t.Errorf("camelToSnake(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestValidateReturnSignature(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"(string, error)", "string", false},
		{"(bool, error)", "bool", false},
		{"(int, error)", "int", false},
		{"([]string, error)", "[]string", false},
		{"([]byte, error)", "[]byte", false},
		{"error", "", false}, // plain error return — no value type
		{"string", "", true},
		{"(string, int, error)", "", true},
		{"", "", true},
		{"(error)", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := validateReturnSignature(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got value %q", tc.input, got)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for %q: %v", tc.input, err)
				}
				if got != tc.want {
					t.Errorf("validateReturnSignature(%q) = %q, want %q", tc.input, got, tc.want)
				}
			}
		})
	}
}

func TestValidateParamTypes(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		params := []paramInfo{
			{GoName: "source", GoType: "string"},
			{GoName: "count", GoType: "int"},
			{GoName: "verbose", GoType: "bool"},
			{GoName: "items", GoType: "[]string"},
		}
		if err := validateParamTypes(params); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("framework types", func(t *testing.T) {
		params := []paramInfo{
			{GoName: "content", GoType: "[]byte"},
			{GoName: "output", GoType: "io.Writer"},
			{GoName: "decryptor", GoType: "func(string, []byte) ([]byte, error)"},
			{GoName: "gitMv", GoType: "func(string, string) error"},
			{GoName: "mode", GoType: "os.FileMode"},
			{GoName: "data", GoType: "map[string]any"},
		}
		if err := validateParamTypes(params); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		params := []paramInfo{
			{GoName: "ch", GoType: "chan string"},
			{GoName: "fn", GoType: "func()"},
			{GoName: "node", GoType: "*Node"},
		}
		err := validateParamTypes(params)
		if err == nil {
			t.Fatal("expected error for unmapped types")
		}
		if !strings.Contains(err.Error(), "chan string") {
			t.Errorf("error should mention 'chan string': %v", err)
		}
		if !strings.Contains(err.Error(), "func()") {
			t.Errorf("error should mention 'func()': %v", err)
		}
		if !strings.Contains(err.Error(), "*Node") {
			t.Errorf("error should mention '*Node': %v", err)
		}
	})

	t.Run("mixed", func(t *testing.T) {
		params := []paramInfo{
			{GoName: "source", GoType: "string"},
			{GoName: "ch", GoType: "chan string"},
			{GoName: "count", GoType: "int"},
		}
		err := validateParamTypes(params)
		if err == nil {
			t.Fatal("expected error for mixed valid/invalid")
		}
		if !strings.Contains(err.Error(), "chan string") {
			t.Errorf("error should mention 'chan string': %v", err)
		}
	})
}

// buildTestDescriptor creates a Starlark dict descriptor for testing.
func buildTestDescriptor(t *testing.T, methods []map[string]any) *starlark.Dict {
	t.Helper()
	desc := starlark.NewDict(5)
	must(t, desc.SetKey(starlark.String("package"), starlark.String("starlark")))
	must(t, desc.SetKey(starlark.String("category"), starlark.String("file")))
	must(t, desc.SetKey(starlark.String("struct_name"), starlark.String("File")))
	must(t, desc.SetKey(starlark.String("namespace"), starlark.String("plan.file")))

	var methodsList []starlark.Value
	for _, m := range methods {
		md := starlark.NewDict(4)
		must(t, md.SetKey(starlark.String("name"), starlark.String(m["name"].(string))))
		must(t, md.SetKey(starlark.String("returns"), starlark.String(m["returns"].(string))))
		must(t, md.SetKey(starlark.String("doc"), starlark.String("")))

		var paramsList []starlark.Value
		if params, ok := m["params"].([]map[string]any); ok {
			for _, p := range params {
				pd := starlark.NewDict(3)
				must(t, pd.SetKey(starlark.String("name"), starlark.String(p["name"].(string))))
				must(t, pd.SetKey(starlark.String("type"), starlark.String(p["type"].(string))))
				variadic := false
				if v, ok := p["variadic"].(bool); ok {
					variadic = v
				}
				must(t, pd.SetKey(starlark.String("variadic"), starlark.Bool(variadic)))
				paramsList = append(paramsList, pd)
			}
		}
		must(t, md.SetKey(starlark.String("params"), starlark.NewList(paramsList)))
		methodsList = append(methodsList, md)
	}
	must(t, desc.SetKey(starlark.String("methods"), starlark.NewList(methodsList)))
	return desc
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func fileMethodsFixture() []map[string]any {
	return []map[string]any{
		{
			"name":    "Copy",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "source", "type": "string"},
				{"name": "path", "type": "string"},
			},
		},
		{
			"name":    "Remove",
			"returns": "(bool, error)",
			"params": []map[string]any{
				{"name": "path", "type": "string"},
			},
		},
	}
}

func TestGeneratePlanReceiver(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, fileMethodsFixture())

	result := callMethod(t, r, "generate",
		starlark.Tuple{starlark.String("plan_receiver"), desc}, nil)

	code, ok := starlark.AsString(result)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	// Valid Go syntax
	if _, err := format.Source([]byte(code)); err != nil {
		t.Fatalf("generated code is not valid Go:\n%s\nerror: %v", code, err)
	}

	// Struct definition with Receiver embedding
	if !strings.Contains(code, "type FilePlan struct") {
		t.Error("missing FilePlan struct definition")
	}
	if !strings.Contains(code, "Receiver\n") {
		t.Error("FilePlan should embed Receiver")
	}

	// Constructor initializes Receiver
	if !strings.Contains(code, `NewReceiver("plan.file")`) {
		t.Error("constructor should call NewReceiver")
	}

	// No inline starlark.Value methods (provided by Receiver)
	if strings.Contains(code, "func (p *FilePlan) String()") {
		t.Error("should not have inline String() — provided by Receiver")
	}
	if strings.Contains(code, "func (p *FilePlan) Hash()") {
		t.Error("should not have inline Hash() — provided by Receiver")
	}

	// Attr switch uses MakeAttr
	if !strings.Contains(code, `case "copy":`) {
		t.Error("missing case for copy in Attr switch")
	}
	if !strings.Contains(code, `case "remove":`) {
		t.Error("missing case for remove in Attr switch")
	}
	if !strings.Contains(code, `MakeAttr("plan.file.copy"`) {
		t.Error("Attr should use MakeAttr")
	}
	if strings.Contains(code, "starlark.NewBuiltin(") {
		t.Error("should not use starlark.NewBuiltin — use MakeAttr")
	}

	// NoSuchAttrError
	if !strings.Contains(code, `NoSuchAttrError("plan.file"`) {
		t.Error("should use NoSuchAttrError with namespace")
	}
	if strings.Contains(code, "starlark.NoSuchAttrError") {
		t.Error("should not use starlark.NoSuchAttrError")
	}

	// Method definitions
	if !strings.Contains(code, "func (p *FilePlan) copy(") {
		t.Error("missing copy method")
	}
	if !strings.Contains(code, "func (p *FilePlan) remove(") {
		t.Error("missing remove method")
	}

	// AttrNames sorted
	if !strings.Contains(code, `"copy", "remove"`) {
		t.Error("AttrNames should be sorted: copy, remove")
	}

	// Namespace
	if !strings.Contains(code, `"plan.file"`) {
		t.Error("missing namespace plan.file")
	}

	// Node ID includes category
	if !strings.Contains(code, `generateNodeID("file.copy")`) {
		t.Error("node ID should include category: file.copy")
	}
	if !strings.Contains(code, `generateNodeID("file.remove")`) {
		t.Error("node ID should include category: file.remove")
	}

	// FillSlot calls
	if !strings.Contains(code, `FillSlot(node, p.graph, "source"`) {
		t.Error("missing FillSlot for source")
	}
	if !strings.Contains(code, `FillSlot(node, p.graph, "path"`) {
		t.Error("missing FillSlot for path")
	}
}

func TestGenerateGraphOps(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, fileMethodsFixture())
	// Graph ops use package "execution"
	must(t, desc.SetKey(starlark.String("package"), starlark.String("execution")))

	result := callMethod(t, r, "generate",
		starlark.Tuple{starlark.String("graph_ops"), desc}, nil)

	code, ok := starlark.AsString(result)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	// Valid Go syntax
	if _, err := format.Source([]byte(code)); err != nil {
		t.Fatalf("generated code is not valid Go:\n%s\nerror: %v", code, err)
	}

	// Op structs
	if !strings.Contains(code, "type FileCopyOp struct{}") {
		t.Error("missing FileCopyOp struct")
	}
	if !strings.Contains(code, "type FileRemoveOp struct{}") {
		t.Error("missing FileRemoveOp struct")
	}

	// Op names
	if !strings.Contains(code, `return "file.copy"`) {
		t.Error("missing file.copy op name")
	}
	if !strings.Contains(code, `return "file.remove"`) {
		t.Error("missing file.remove op name")
	}

	// Unified Execute method
	if !strings.Contains(code, "Execute(ctx *Context, node *Node) error") {
		t.Error("missing unified Execute method signature")
	}

	// No legacy dispatch interfaces
	if strings.Contains(code, "Category()") {
		t.Error("should not have Category method")
	}
	if strings.Contains(code, "Write(ctx") {
		t.Error("should not have Write method")
	}
	if strings.Contains(code, "Transform(ctx") {
		t.Error("should not have Transform method")
	}

	// Registration function
	if !strings.Contains(code, "func FileOps() []Operation") {
		t.Error("missing FileOps registration function")
	}

	// Slot readers with type assertions (typed slots)
	if !strings.Contains(code, `node.GetSlot("source").(string)`) {
		t.Error("missing typed slot reader for source")
	}
}

func TestGenerateRealtimeReceiver(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, fileMethodsFixture())

	result := callMethod(t, r, "generate",
		starlark.Tuple{starlark.String("realtime_receiver"), desc}, nil)

	code, ok := starlark.AsString(result)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	// Valid Go syntax
	if _, err := format.Source([]byte(code)); err != nil {
		t.Fatalf("generated code is not valid Go:\n%s\nerror: %v", code, err)
	}

	// Struct definition
	if !strings.Contains(code, "type FileReceiver struct") {
		t.Error("missing FileReceiver struct")
	}

	// Constructor
	if !strings.Contains(code, `NewReceiver("file")`) {
		t.Error("missing NewReceiver call")
	}

	// Attr dispatch
	if !strings.Contains(code, `MakeAttr("file.copy"`) {
		t.Error("missing MakeAttr for file.copy")
	}
	if !strings.Contains(code, `MakeAttr("file.remove"`) {
		t.Error("missing MakeAttr for file.remove")
	}

	// NoSuchAttrError
	if !strings.Contains(code, `NoSuchAttrError("file"`) {
		t.Error("missing NoSuchAttrError")
	}
}

func TestGenerateGateRejectsUnmappedType(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, []map[string]any{
		{
			"name":    "Send",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "ch", "type": "chan string"},
			},
		},
	})

	thread := &starlark.Thread{Name: "test"}
	attr, _ := r.Attr("generate")
	fn := attr.(*starlark.Builtin)
	_, err := fn.CallInternal(thread, starlark.Tuple{starlark.String("plan_receiver"), desc}, nil)
	if err == nil {
		t.Fatal("expected error for unmapped type")
	}
	if !strings.Contains(err.Error(), "chan string") {
		t.Errorf("error should mention unmapped type: %v", err)
	}
}

func TestGenerateGateRejectsBadReturn(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, []map[string]any{
		{
			"name":    "Check",
			"returns": "string",
			"params":  []map[string]any{},
		},
	})

	thread := &starlark.Thread{Name: "test"}
	attr, _ := r.Attr("generate")
	fn := attr.(*starlark.Builtin)
	_, err := fn.CallInternal(thread, starlark.Tuple{starlark.String("plan_receiver"), desc}, nil)
	if err == nil {
		t.Fatal("expected error for bad return signature")
	}
	if !strings.Contains(err.Error(), "must return error or (T, error)") {
		t.Errorf("error should mention return format: %v", err)
	}
}

func TestGenerateVariadicParam(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, []map[string]any{
		{
			"name":    "Install",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "packages", "type": "string", "variadic": true},
			},
		},
	})

	result := callMethod(t, r, "generate",
		starlark.Tuple{starlark.String("plan_receiver"), desc}, nil)

	code, ok := starlark.AsString(result)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	// Valid Go syntax
	if _, err := format.Source([]byte(code)); err != nil {
		t.Fatalf("generated code is not valid Go:\n%s\nerror: %v", code, err)
	}

	// Should have the install method
	if !strings.Contains(code, "func (p *FilePlan) install(") {
		t.Error("missing install method")
	}

	// Should unpack the packages param
	if !strings.Contains(code, `"packages"`) {
		t.Error("missing packages param in unpack")
	}
}

func TestGoGenerateAttr(t *testing.T) {
	r := NewGoReceiver()

	// Check AttrNames includes "generate"
	names := r.AttrNames()
	found := false
	for _, n := range names {
		if n == "generate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AttrNames() should include 'generate'")
	}

	// Check Attr("generate") returns a builtin
	attr, err := r.Attr("generate")
	if err != nil {
		t.Fatalf("Attr(generate): %v", err)
	}
	if _, ok := attr.(*starlark.Builtin); !ok {
		t.Fatalf("Attr(generate) returned %T, want *starlark.Builtin", attr)
	}
}

func TestGenerateGraphOpsDelegation(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, []map[string]any{
		{
			"name":    "Link",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "source", "type": "string"},
				{"name": "path", "type": "string"},
			},
		},
		{
			"name":    "Copy",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "path", "type": "string"},
			},
		},
		{
			"name":    "Render",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "source", "type": "string"},
			},
		},
	})
	must(t, desc.SetKey(starlark.String("package"), starlark.String("execution")))
	must(t, desc.SetKey(starlark.String("namespace"), starlark.String("file")))
	must(t, desc.SetKey(starlark.String("impl_type"), starlark.String("fileOps")))

	result := callMethod(t, r, "generate",
		starlark.Tuple{starlark.String("graph_ops"), desc}, nil)

	code, ok := starlark.AsString(result)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	// Valid Go syntax
	if _, err := format.Source([]byte(code)); err != nil {
		t.Fatalf("generated code is not valid Go:\n%s\nerror: %v", code, err)
	}

	// Op structs have impl field
	if !strings.Contains(code, "type FileLinkOp struct{ impl *fileOps }") {
		t.Error("FileLinkOp should have impl *fileOps field")
	}
	if !strings.Contains(code, "type FileCopyOp struct{ impl *fileOps }") {
		t.Error("FileCopyOp should have impl *fileOps field")
	}
	if !strings.Contains(code, "type FileRenderOp struct{ impl *fileOps }") {
		t.Error("FileRenderOp should have impl *fileOps field")
	}

	// All ops delegate via unified Execute (slot params only)
	if !strings.Contains(code, "o.impl.Link(ctx, source, path)") {
		t.Error("Link op should delegate to impl.Link")
	}
	if !strings.Contains(code, "o.impl.Copy(ctx, path)") {
		t.Error("Copy op should delegate to impl.Copy")
	}
	if !strings.Contains(code, "o.impl.Render(ctx, source)") {
		t.Error("Render op should delegate to impl.Render")
	}

	// Slot readers use type assertions
	if !strings.Contains(code, `node.GetSlot("source").(string)`) {
		t.Error("slot readers should use type assertions")
	}

	// No legacy dispatch interfaces
	if strings.Contains(code, "Category()") {
		t.Error("should not have Category method")
	}
	if strings.Contains(code, "Write(ctx") {
		t.Error("should not have Write method")
	}
	if strings.Contains(code, "Transform(ctx") {
		t.Error("should not have Transform method")
	}

	// No TODO stubs when impl_type is set
	if strings.Contains(code, "// TODO") {
		t.Error("should not have TODO stubs when impl_type is set")
	}

	// Registration function creates impl
	if !strings.Contains(code, "impl := &fileOps{}") {
		t.Error("registration function should create impl")
	}
	if !strings.Contains(code, "&FileLinkOp{impl: impl}") {
		t.Error("registration should pass impl to ops")
	}
}

func TestGoMappingRoundTrip(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, []map[string]any{
		{
			"name":    "Link",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "source", "type": "string"},
				{"name": "path", "type": "string"},
			},
		},
		{
			"name":    "Copy",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "path", "type": "string"},
			},
		},
		{
			"name":    "Render",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "source", "type": "string"},
			},
		},
	})
	must(t, desc.SetKey(starlark.String("package"), starlark.String("execution")))
	must(t, desc.SetKey(starlark.String("namespace"), starlark.String("file")))
	must(t, desc.SetKey(starlark.String("impl_type"), starlark.String("fileOps")))

	result := callMethod(t, r, "mapping", starlark.Tuple{desc}, nil)

	yamlStr, ok := starlark.AsString(result)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	// Parse the YAML output
	var mapping mappingFile
	if err := yaml.Unmarshal([]byte(yamlStr), &mapping); err != nil {
		t.Fatalf("failed to parse mapping YAML: %v\n%s", err, yamlStr)
	}

	// Verify top-level fields
	if mapping.Version != "1.0" {
		t.Errorf("expected version 1.0, got %q", mapping.Version)
	}
	if mapping.Struct != "fileOps" {
		t.Errorf("expected struct fileOps, got %q", mapping.Struct)
	}
	if mapping.Package != "execution" {
		t.Errorf("expected package execution, got %q", mapping.Package)
	}
	if mapping.Category != "file" {
		t.Errorf("expected category file, got %q", mapping.Category)
	}

	// Verify operations
	if len(mapping.Operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(mapping.Operations))
	}

	// Link
	link := mapping.Operations[0]
	if link.Name != "file.link" {
		t.Errorf("expected file.link, got %q", link.Name)
	}
	if link.GoMethod != "Link" {
		t.Errorf("expected GoMethod Link, got %q", link.GoMethod)
	}
	if len(link.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(link.Params))
	}
	if link.Params[0].Name != "source" || link.Params[0].Type != "string" {
		t.Errorf("unexpected first param: %+v", link.Params[0])
	}

	// Copy
	copyOp := mapping.Operations[1]
	if copyOp.Name != "file.copy" {
		t.Errorf("expected file.copy, got %q", copyOp.Name)
	}
	if len(copyOp.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(copyOp.Params))
	}

	// Render
	render := mapping.Operations[2]
	if render.Name != "file.render" {
		t.Errorf("expected file.render, got %q", render.Name)
	}

	// Verify header comment
	if !strings.Contains(yamlStr, "# Generated by go.mapping") {
		t.Error("missing header comment")
	}
}

func TestGoMappingAttr(t *testing.T) {
	r := NewGoReceiver()

	// Check AttrNames includes "mapping"
	names := r.AttrNames()
	found := false
	for _, n := range names {
		if n == "mapping" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AttrNames() should include 'mapping'")
	}

	// Check Attr("mapping") returns a builtin
	attr, err := r.Attr("mapping")
	if err != nil {
		t.Fatalf("Attr(mapping): %v", err)
	}
	if _, ok := attr.(*starlark.Builtin); !ok {
		t.Fatalf("Attr(mapping) returned %T, want *starlark.Builtin", attr)
	}
}

func TestGoMappingFrameworkTypes(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptor(t, []map[string]any{
		{
			"name":    "Decrypt",
			"returns": "([]byte, error)",
			"params": []map[string]any{
				{"name": "decryptor", "type": "func(string, []byte) ([]byte, error)"},
				{"name": "source", "type": "string"},
				{"name": "content", "type": "[]byte"},
			},
		},
		{
			"name":    "Shell",
			"returns": "error",
			"params": []map[string]any{
				{"name": "command", "type": "string"},
				{"name": "output", "type": "io.Writer"},
			},
		},
		{
			"name":    "Copy",
			"returns": "(string, error)",
			"params": []map[string]any{
				{"name": "path", "type": "string"},
				{"name": "mode", "type": "os.FileMode"},
				{"name": "content", "type": "[]byte"},
			},
		},
	})
	must(t, desc.SetKey(starlark.String("package"), starlark.String("execution")))
	must(t, desc.SetKey(starlark.String("impl_type"), starlark.String("EncryptionService")))

	result := callMethod(t, r, "mapping", starlark.Tuple{desc}, nil)

	yamlStr, ok := starlark.AsString(result)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	// Parse the YAML output
	var mapping mappingFile
	if err := yaml.Unmarshal([]byte(yamlStr), &mapping); err != nil {
		t.Fatalf("failed to parse mapping YAML: %v\n%s", err, yamlStr)
	}

	if len(mapping.Operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(mapping.Operations))
	}

	// Decrypt has framework params recorded
	decrypt := mapping.Operations[0]
	if decrypt.Name != "file.decrypt" {
		t.Errorf("expected file.decrypt, got %q", decrypt.Name)
	}
	if len(decrypt.Params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(decrypt.Params))
	}
	if decrypt.Params[0].Type != "func(string, []byte) ([]byte, error)" {
		t.Errorf("expected func type, got %q", decrypt.Params[0].Type)
	}

	// Shell has error-only return and io.Writer param
	shell := mapping.Operations[1]
	if shell.Name != "file.shell" {
		t.Errorf("expected file.shell, got %q", shell.Name)
	}
	if len(shell.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(shell.Params))
	}
}

func TestGoMappingGateEnforcement(t *testing.T) {
	t.Run("unmapped type", func(t *testing.T) {
		r := NewGoReceiver()
		desc := buildTestDescriptor(t, []map[string]any{
			{
				"name":    "Send",
				"returns": "(string, error)",
				"params": []map[string]any{
					{"name": "ch", "type": "chan string"},
				},
			},
		})

		thread := &starlark.Thread{Name: "test"}
		attr, _ := r.Attr("mapping")
		fn := attr.(*starlark.Builtin)
		_, err := fn.CallInternal(thread, starlark.Tuple{desc}, nil)
		if err == nil {
			t.Fatal("expected error for unmapped type")
		}
		if !strings.Contains(err.Error(), "chan string") {
			t.Errorf("error should mention unmapped type: %v", err)
		}
	})

	t.Run("bad return", func(t *testing.T) {
		r := NewGoReceiver()
		desc := buildTestDescriptor(t, []map[string]any{
			{
				"name":    "Check",
				"returns": "string",
				"params":  []map[string]any{},
			},
		})

		thread := &starlark.Thread{Name: "test"}
		attr, _ := r.Attr("mapping")
		fn := attr.(*starlark.Builtin)
		_, err := fn.CallInternal(thread, starlark.Tuple{desc}, nil)
		if err == nil {
			t.Fatal("expected error for bad return signature")
		}
		if !strings.Contains(err.Error(), "must return error or (T, error)") {
			t.Errorf("error should mention return format: %v", err)
		}
	})
}
