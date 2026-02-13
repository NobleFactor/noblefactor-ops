// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"go/format"
	"strings"
	"testing"

	"go.starlark.net/starlark"
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
		{"error", "", true},
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

	// Struct definition
	if !strings.Contains(code, "type FilePlan struct") {
		t.Error("missing FilePlan struct definition")
	}

	// Attr switch cases
	if !strings.Contains(code, `case "copy":`) {
		t.Error("missing case for copy in Attr switch")
	}
	if !strings.Contains(code, `case "remove":`) {
		t.Error("missing case for remove in Attr switch")
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

	// Registration function
	if !strings.Contains(code, "func FileOps() []Operation") {
		t.Error("missing FileOps registration function")
	}

	// Slot readers
	if !strings.Contains(code, `node.GetSlot("source")`) {
		t.Error("missing slot reader for source")
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
			"returns": "error",
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
	if !strings.Contains(err.Error(), "must return (T, error)") {
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

// buildTestDescriptorWithOpCategory creates a descriptor with op_category set on each method.
func buildTestDescriptorWithOpCategory(t *testing.T, methods []map[string]any) *starlark.Dict {
	t.Helper()
	desc := starlark.NewDict(5)
	must(t, desc.SetKey(starlark.String("package"), starlark.String("execution")))
	must(t, desc.SetKey(starlark.String("category"), starlark.String("file")))
	must(t, desc.SetKey(starlark.String("struct_name"), starlark.String("File")))
	must(t, desc.SetKey(starlark.String("namespace"), starlark.String("file")))

	var methodsList []starlark.Value
	for _, m := range methods {
		md := starlark.NewDict(5)
		must(t, md.SetKey(starlark.String("name"), starlark.String(m["name"].(string))))
		must(t, md.SetKey(starlark.String("returns"), starlark.String(m["returns"].(string))))
		must(t, md.SetKey(starlark.String("doc"), starlark.String("")))
		if opCat, ok := m["op_category"].(string); ok {
			must(t, md.SetKey(starlark.String("op_category"), starlark.String(opCat)))
		}

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

func TestGenerateGraphOpsWriter(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptorWithOpCategory(t, []map[string]any{
		{
			"name":        "Copy",
			"returns":     "(string, error)",
			"op_category": "writer",
			"params": []map[string]any{
				{"name": "source", "type": "string"},
				{"name": "path", "type": "string"},
			},
		},
	})

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

	// OpWriter category
	if !strings.Contains(code, "Category() OpCategory { return OpWriter }") {
		t.Error("missing OpWriter category")
	}

	// Write method signature
	if !strings.Contains(code, "Write(ctx *Context, node Executable, content []byte) (string, error)") {
		t.Error("missing Write method signature")
	}

	// Should NOT have Execute method
	if strings.Contains(code, "Execute(ctx *Context") {
		t.Error("Writer op should not have Execute method")
	}
}

func TestGenerateGraphOpsTransform(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptorWithOpCategory(t, []map[string]any{
		{
			"name":        "Render",
			"returns":     "(string, error)",
			"op_category": "transform",
			"params": []map[string]any{
				{"name": "path", "type": "string"},
			},
		},
	})

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

	// OpTransform category
	if !strings.Contains(code, "Category() OpCategory { return OpTransform }") {
		t.Error("missing OpTransform category")
	}

	// Transform method signature
	if !strings.Contains(code, "Transform(ctx *Context, node Executable, content []byte) ([]byte, error)") {
		t.Error("missing Transform method signature")
	}

	// Should NOT have Execute method
	if strings.Contains(code, "Execute(ctx *Context") {
		t.Error("Transform op should not have Execute method")
	}
}

func TestGenerateGraphOpsInvalidOpCategory(t *testing.T) {
	r := NewGoReceiver()
	desc := buildTestDescriptorWithOpCategory(t, []map[string]any{
		{
			"name":        "Bad",
			"returns":     "(string, error)",
			"op_category": "invalid",
			"params":      []map[string]any{},
		},
	})

	thread := &starlark.Thread{Name: "test"}
	attr, _ := r.Attr("generate")
	fn := attr.(*starlark.Builtin)
	_, err := fn.CallInternal(thread, starlark.Tuple{starlark.String("graph_ops"), desc}, nil)
	if err == nil {
		t.Fatal("expected error for invalid op_category")
	}
	if !strings.Contains(err.Error(), "invalid op_category") {
		t.Errorf("error should mention invalid op_category: %v", err)
	}
}
