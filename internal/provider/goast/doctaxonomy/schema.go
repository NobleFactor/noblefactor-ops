// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// DefaultRegistry returns a SchemaRegistry with the standard Go comment
// schemas. These match the defaults in the LintGoStyle extension config.
func DefaultRegistry() *SchemaRegistry {
	reg := NewSchemaRegistry()
	reg.Register(CommentSchema{
		Name: "copyright", Format: "go", NodeType: "File",
		Elements: []SchemaElement{
			{Name: "spdx", Type: "verbatim", Required: "true", Order: 1},
			{Name: "copyright", Type: "verbatim", Required: "true", Order: 2},
		},
	})
	reg.Register(CommentSchema{
		Name: "gen_decl", Format: "go", NodeType: "GenDecl",
		Elements: []SchemaElement{
			{Name: "summary", Type: "paragraph", Required: "true", Order: 1},
			{Name: "body", Type: "block", Cardinality: "*", Order: 2},
		},
	})
	reg.Register(CommentSchema{
		Name: "func_doc", Format: "go", NodeType: "FuncDecl",
		Elements: []SchemaElement{
			{Name: "summary", Type: "paragraph", Required: "true", Order: 1},
			{Name: "body", Type: "block", Cardinality: "*", Order: 2},
		},
	})
	return reg
}

// SchemaElement defines one element slot in a comment schema.
type SchemaElement struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Required    string `yaml:"required,omitempty"`
	Cardinality string `yaml:"cardinality,omitempty"`
	Order       int    `yaml:"order"`
	Header      string `yaml:"header,omitempty"`
	ItemTokens  string `yaml:"item_tokens,omitempty"`
}

// CommentSchema defines the structure of a doc comment for a given node type
// and format.
type CommentSchema struct {
	Name     string          `yaml:"-"`
	Format   string          `yaml:"format"`
	NodeType string          `yaml:"node_type"`
	Elements []SchemaElement `yaml:"elements"`
}

// schemaFile is the top-level YAML structure.
type schemaFile struct {
	Schemas map[string]CommentSchema `yaml:"schemas"`
}

// LoadSchemas deserializes a YAML file into a slice of CommentSchema.
func LoadSchemas(path string) ([]CommentSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema file: %w", err)
	}
	return ParseSchemas(data)
}

// ParseSchemas deserializes YAML bytes into a slice of CommentSchema.
func ParseSchemas(data []byte) ([]CommentSchema, error) {
	var f schemaFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("unmarshal schemas: %w", err)
	}

	schemas := make([]CommentSchema, 0, len(f.Schemas))
	for name, s := range f.Schemas {
		s.Name = name
		schemas = append(schemas, s)
	}

	// Sort by name for deterministic ordering.
	sort.Slice(schemas, func(i, j int) bool {
		return schemas[i].Name < schemas[j].Name
	})

	return schemas, nil
}

// SchemaRegistry holds loaded schemas keyed by (nodeType, format).
type SchemaRegistry struct {
	mu      sync.RWMutex
	schemas map[string]*CommentSchema // key: "nodeType:format"
}

// NewSchemaRegistry creates an empty registry.
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		schemas: make(map[string]*CommentSchema),
	}
}

// Register adds a schema to the registry.
func (r *SchemaRegistry) Register(schema CommentSchema) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := schema.NodeType + ":" + schema.Format
	s := schema // copy
	r.schemas[key] = &s
}

// Lookup finds a schema by node type and format. Returns nil if not found.
func (r *SchemaRegistry) Lookup(nodeType, format string) *CommentSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.schemas[nodeType+":"+format]
}

// Diagnostic represents a validation issue found in a parsed doc comment.
type Diagnostic struct {
	Element string // schema element name (e.g., "summary", "parameters")
	Message string // human-readable description
}

// Validate checks a parsed FuncDoc against a schema and the actual function
// signature. Returns diagnostics for missing required elements, cardinality
// violations, and parameter/return sync issues.
func Validate(doc *FuncDoc, schema *CommentSchema, paramNames, returnTypes []string) []Diagnostic {
	var diags []Diagnostic

	// Classify parsed elements.
	var paragraphs []*Paragraph
	var directives []*Directive
	var paramSections []*ParamSection
	var returnSections []*ReturnSection
	var headings []*Heading
	var codeBlocks []*CodeBlock

	for _, el := range doc.Elements {
		switch {
		case el.Paragraph != nil:
			paragraphs = append(paragraphs, el.Paragraph)
		case el.Directive != nil:
			directives = append(directives, el.Directive)
		case el.ParamSection != nil:
			paramSections = append(paramSections, el.ParamSection)
		case el.ReturnSection != nil:
			returnSections = append(returnSections, el.ReturnSection)
		case el.Heading != nil:
			headings = append(headings, el.Heading)
		case el.CodeBlock != nil:
			codeBlocks = append(codeBlocks, el.CodeBlock)
		}
	}

	for _, se := range schema.Elements {
		switch se.Type {
		case "paragraph":
			diags = validateParagraph(diags, se, paragraphs)
		case "directive":
			diags = validateCardinality(diags, se, len(directives))
		case "block":
			bodyCount := len(headings) + len(codeBlocks)
			if len(paragraphs) > 1 {
				bodyCount += len(paragraphs) - 1
			}
			diags = validateCardinality(diags, se, bodyCount)
		case "param_section":
			diags = validateParamSection(diags, se, paramSections, paramNames)
		case "return_section":
			diags = validateReturnSection(diags, se, returnSections, returnTypes)
		}
	}

	return diags
}

// validateParagraph checks summary (first paragraph) presence.
func validateParagraph(diags []Diagnostic, se SchemaElement, paragraphs []*Paragraph) []Diagnostic {
	if se.Required == "true" && len(paragraphs) == 0 {
		diags = append(diags, Diagnostic{
			Element: se.Name,
			Message: "missing required summary",
		})
	}
	return diags
}

// validateCardinality checks that element count is within bounds.
func validateCardinality(diags []Diagnostic, se SchemaElement, count int) []Diagnostic {
	if se.Required == "true" && count == 0 {
		diags = append(diags, Diagnostic{
			Element: se.Name,
			Message: fmt.Sprintf("missing required %s", se.Name),
		})
	}
	if se.Cardinality == "" && count > 1 {
		diags = append(diags, Diagnostic{
			Element: se.Name,
			Message: fmt.Sprintf("expected at most 1 %s, found %d", se.Name, count),
		})
	}
	return diags
}

// validateParamSection checks parameter sync: documented vs. actual.
func validateParamSection(diags []Diagnostic, se SchemaElement, sections []*ParamSection, paramNames []string) []Diagnostic {
	// Cardinality: at most one ParamSection.
	if len(sections) > 1 {
		diags = append(diags, Diagnostic{
			Element: se.Name,
			Message: fmt.Sprintf("expected at most 1 Parameters section, found %d", len(sections)),
		})
	}

	// Required: if_params means required when params exist.
	if se.Required == "if_params" && len(paramNames) > 0 && len(sections) == 0 {
		diags = append(diags, Diagnostic{
			Element: se.Name,
			Message: "missing Parameters section (function has parameters)",
		})
		return diags
	}

	if len(sections) == 0 {
		return diags
	}

	section := sections[0]
	documented := make(map[string]bool)
	for _, item := range section.Items {
		documented[item.Name] = true
	}

	actual := make(map[string]bool)
	for _, name := range paramNames {
		actual[name] = true
	}

	// Undocumented parameters.
	for _, name := range paramNames {
		if !documented[name] {
			diags = append(diags, Diagnostic{
				Element: se.Name,
				Message: fmt.Sprintf("parameter '%s' not documented", name),
			})
		}
	}

	// Stale documented parameters.
	for _, item := range section.Items {
		if !actual[item.Name] {
			diags = append(diags, Diagnostic{
				Element: se.Name,
				Message: fmt.Sprintf("documented parameter '%s' not in signature", item.Name),
			})
		}
	}

	return diags
}

// validateReturnSection checks return sync: documented vs. actual.
func validateReturnSection(diags []Diagnostic, se SchemaElement, sections []*ReturnSection, returnTypes []string) []Diagnostic {
	// Cardinality: at most one ReturnSection.
	if len(sections) > 1 {
		diags = append(diags, Diagnostic{
			Element: se.Name,
			Message: fmt.Sprintf("expected at most 1 Returns section, found %d", len(sections)),
		})
	}

	// Required: if_returns means required when returns exist.
	if se.Required == "if_returns" && len(returnTypes) > 0 && len(sections) == 0 {
		diags = append(diags, Diagnostic{
			Element: se.Name,
			Message: "missing Returns section (function has return values)",
		})
		return diags
	}

	if len(sections) == 0 {
		return diags
	}

	section := sections[0]
	documented := make(map[string]bool)
	for _, item := range section.Items {
		documented[item.Type] = true
	}

	actual := make(map[string]bool)
	for _, t := range returnTypes {
		actual[t] = true
	}

	// Undocumented return values.
	for _, t := range returnTypes {
		if !documented[t] {
			diags = append(diags, Diagnostic{
				Element: se.Name,
				Message: fmt.Sprintf("return value '%s' not documented", t),
			})
		}
	}

	// Stale documented return values.
	for _, item := range section.Items {
		if !actual[item.Type] {
			diags = append(diags, Diagnostic{
				Element: se.Name,
				Message: fmt.Sprintf("documented return value '%s' not in signature", item.Type),
			})
		}
	}

	return diags
}
