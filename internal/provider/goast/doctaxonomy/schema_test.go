// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const testSchemaYAML = `schemas:
  func_doc:
    format: go
    node_type: FuncDecl
    elements:
      - name: summary
        type: paragraph
        required: "true"
        order: 1
      - name: body
        type: block
        cardinality: "*"
        order: 2

  gen_decl:
    format: go
    node_type: GenDecl
    elements:
      - name: summary
        type: paragraph
        required: "true"
        order: 1
      - name: body
        type: block
        cardinality: "*"
        order: 2

  copyright:
    format: go
    node_type: File
    elements:
      - name: spdx
        type: verbatim
        required: "true"
        order: 1
      - name: copyright
        type: verbatim
        required: "true"
        order: 2
`

// TestParseSchemas verifies that YAML is correctly deserialized.
func TestParseSchemas(t *testing.T) {
	schemas, err := ParseSchemas([]byte(testSchemaYAML))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(schemas) != 3 {
		t.Fatalf("expected 3 schemas, got %d", len(schemas))
	}

	// Sorted by name: copyright, func_doc, gen_decl.
	if schemas[0].Name != "copyright" {
		t.Errorf("schema 0 name = %q, want 'copyright'", schemas[0].Name)
	}
	if schemas[1].Name != "func_doc" {
		t.Errorf("schema 1 name = %q, want 'func_doc'", schemas[1].Name)
	}
	if schemas[2].Name != "gen_decl" {
		t.Errorf("schema 2 name = %q, want 'gen_decl'", schemas[2].Name)
	}

	funcDoc := schemas[1]
	if funcDoc.Format != "go" {
		t.Errorf("func_doc format = %q, want 'go'", funcDoc.Format)
	}
	if funcDoc.NodeType != "FuncDecl" {
		t.Errorf("func_doc node_type = %q, want 'FuncDecl'", funcDoc.NodeType)
	}
	if len(funcDoc.Elements) != 2 {
		t.Fatalf("func_doc elements = %d, want 2", len(funcDoc.Elements))
	}
	if funcDoc.Elements[0].Name != "summary" {
		t.Errorf("element 0 name = %q, want 'summary'", funcDoc.Elements[0].Name)
	}
	if funcDoc.Elements[1].Name != "body" {
		t.Errorf("element 1 name = %q, want 'body'", funcDoc.Elements[1].Name)
	}
}

// TestSchemaRoundTrip verifies load → marshal → load produces identical schemas.
func TestSchemaRoundTrip(t *testing.T) {
	schemas1, err := ParseSchemas([]byte(testSchemaYAML))
	if err != nil {
		t.Fatalf("first parse error: %v", err)
	}

	// Marshal back to YAML.
	out := struct {
		Schemas map[string]CommentSchema `yaml:"schemas"`
	}{Schemas: make(map[string]CommentSchema)}
	for _, s := range schemas1 {
		out.Schemas[s.Name] = s
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Parse again.
	schemas2, err := ParseSchemas(data)
	if err != nil {
		t.Fatalf("second parse error: %v", err)
	}

	if len(schemas1) != len(schemas2) {
		t.Fatalf("schema count mismatch: %d vs %d", len(schemas1), len(schemas2))
	}

	for i := range schemas1 {
		if schemas1[i].Name != schemas2[i].Name {
			t.Errorf("schema %d name mismatch: %q vs %q", i, schemas1[i].Name, schemas2[i].Name)
		}
		if schemas1[i].Format != schemas2[i].Format {
			t.Errorf("schema %d format mismatch: %q vs %q", i, schemas1[i].Format, schemas2[i].Format)
		}
		if len(schemas1[i].Elements) != len(schemas2[i].Elements) {
			t.Errorf("schema %d element count mismatch: %d vs %d", i, len(schemas1[i].Elements), len(schemas2[i].Elements))
		}
	}
}

// TestSchemaRegistry verifies Register and Lookup.
func TestSchemaRegistry(t *testing.T) {
	reg := NewSchemaRegistry()

	schemas, err := ParseSchemas([]byte(testSchemaYAML))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	for _, s := range schemas {
		reg.Register(s)
	}

	funcDoc := reg.Lookup("FuncDecl", "go")
	if funcDoc == nil {
		t.Fatal("Lookup(FuncDecl, go) returned nil")
	}
	if funcDoc.Name != "func_doc" {
		t.Errorf("name = %q, want 'func_doc'", funcDoc.Name)
	}

	typeDoc := reg.Lookup("GenDecl", "go")
	if typeDoc == nil {
		t.Fatal("Lookup(GenDecl, go) returned nil")
	}

	missing := reg.Lookup("InterfaceDecl", "go")
	if missing != nil {
		t.Error("Lookup(InterfaceDecl, go) should return nil")
	}
}

// TestLoadSchemas verifies loading from the actual go.yaml file.
func TestLoadSchemas(t *testing.T) {
	schemas, err := LoadSchemas("schemas/go.yaml")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if len(schemas) != 3 {
		t.Fatalf("expected 3 schemas, got %d", len(schemas))
	}
}

// TestValidate_MissingSummary checks that a missing summary produces a diagnostic.
func TestValidate_MissingSummary(t *testing.T) {
	schemas, _ := ParseSchemas([]byte(testSchemaYAML))
	funcSchema := findSchema(schemas, "func_doc")

	doc := &FuncDoc{Elements: nil} // empty doc
	diags := Validate(doc, funcSchema, nil, nil)

	found := false
	for _, d := range diags {
		if d.Element == "summary" && d.Message == "missing required summary" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'missing required summary' diagnostic, got %v", diags)
	}
}

// TestValidate_UndocumentedParam checks that an undocumented parameter is flagged.
func TestValidate_UndocumentedParam(t *testing.T) {
	funcSchema := projectFuncDocSchema()

	parser := NewFuncParser([]string{"resource", "opts"}, nil)
	doc, _ := parser.ParseString("", `Summary line.

Parameters:
  - resource: The file.`)

	diags := Validate(doc, funcSchema, []string{"resource", "opts"}, nil)

	found := false
	for _, d := range diags {
		if d.Element == "parameters" && d.Message == "parameter 'opts' not documented" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected undocumented parameter diagnostic for 'opts', got %v", diags)
	}
}

// TestValidate_StaleParam checks that a documented parameter not in the
// signature is flagged.
func TestValidate_StaleParam(t *testing.T) {
	funcSchema := projectFuncDocSchema()

	parser := NewFuncParser([]string{"resource"}, nil)
	doc, _ := parser.ParseString("", `Summary line.

Parameters:
  - resource: The file.
  - path: Old name.`)

	diags := Validate(doc, funcSchema, []string{"resource"}, nil)

	found := false
	for _, d := range diags {
		if d.Element == "parameters" && d.Message == "documented parameter 'path' not in signature" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected stale parameter diagnostic for 'path', got %v", diags)
	}
}

// TestValidate_MissingReturns checks that a missing Returns section is flagged
// when the function has return values.
func TestValidate_MissingReturns(t *testing.T) {
	funcSchema := projectFuncDocSchema()

	parser := NewFuncParser(nil, nil)
	doc, _ := parser.ParseString("", "Summary line.")

	diags := Validate(doc, funcSchema, nil, []string{"error"})

	found := false
	for _, d := range diags {
		if d.Element == "returns" && d.Message == "missing Returns section (function has return values)" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing Returns diagnostic, got %v", diags)
	}
}

// TestValidate_Clean checks that a fully documented function produces no diagnostics.
func TestValidate_Clean(t *testing.T) {
	funcSchema := projectFuncDocSchema()

	parser := NewFuncParser([]string{"resource", "opts"}, []string{"Resource", "error"})
	doc, err := parser.ParseString("", `Summary line.

Parameters:
  - resource: The file.
  - opts: Options.

Returns:
  - Resource: The result.
  - error: Any error.`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	diags := Validate(doc, funcSchema, []string{"resource", "opts"}, []string{"Resource", "error"})

	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %v", diags)
	}
}

// TestNormalizeWithSchema verifies that schema-driven ordering works.
func TestNormalizeWithSchema(t *testing.T) {
	parser := NewFuncParser([]string{"x"}, nil)
	doc, err := parser.ParseString("", `+devlore:test value

Summary line.

Parameters:
  - x: Input.`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Default schema (summary + body) emits only the summary.
	defaultOut := doc.Normalize()
	if idx := findIndex(defaultOut, "Summary line."); idx != 0 {
		t.Errorf("summary not at start of default output")
	}

	// Project schema with all elements emits everything in order.
	projectSchema := projectFuncDocSchema()
	fullOut := doc.NormalizeWithSchema(projectSchema.Elements)
	if idx := findIndex(fullOut, "Summary line."); idx != 0 {
		t.Errorf("summary not at start of schema output")
	}
	if !strings.Contains(fullOut, "Parameters:") {
		t.Error("missing Parameters in schema output")
	}
	if !strings.Contains(fullOut, "+devlore:test value") {
		t.Error("missing directive in schema output")
	}
}

// projectFuncDocSchema returns a schema with all elements including
// parameters, returns, and directives — for testing project-specific config.
func projectFuncDocSchema() *CommentSchema {
	return &CommentSchema{
		Name:     "func_doc",
		Format:   "go",
		NodeType: "FuncDecl",
		Elements: []SchemaElement{
			{Name: "summary", Type: "paragraph", Required: "true", Order: 1},
			{Name: "body", Type: "block", Cardinality: "*", Order: 2},
			{Name: "parameters", Type: "param_section", Required: "if_params", Order: 3, Header: "Parameters:", ItemTokens: "param_names"},
			{Name: "returns", Type: "return_section", Required: "if_returns", Order: 4, Header: "Returns:", ItemTokens: "return_types"},
			{Name: "directives", Type: "directive", Cardinality: "*", Order: 5},
		},
	}
}

// findSchema returns the schema with the given name from a slice.
func findSchema(schemas []CommentSchema, name string) *CommentSchema {
	for i := range schemas {
		if schemas[i].Name == name {
			return &schemas[i]
		}
	}
	return nil
}

// findIndex returns the byte offset of substr in s, or -1.
func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
