// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// =============================================================================
// DESCRIPTOR TYPES
// =============================================================================

// generateDescriptor holds the complete input for code generation.
type generateDescriptor struct {
	Template   string       // "plan_receiver", "graph_ops", "realtime_receiver"
	Package    string       // Go package name for generated file
	Category   string       // snake_case category (e.g., "file")
	StructName string       // Go struct name (e.g., "File")
	Namespace  string       // dotted namespace (e.g., "plan.file")
	Methods    []methodInfo // analyzed methods
}

// methodInfo holds analyzed information about a single method.
type methodInfo struct {
	GoName     string     // original Go name (e.g., "Copy")
	SnakeName  string     // snake_case name (e.g., "copy")
	Params     []paramInfo
	ReturnType string // value portion of (T, error)
	Doc        string
}

// paramInfo holds information about a single parameter.
type paramInfo struct {
	GoName    string // original param name
	SnakeName string // snake_case name
	GoType    string // Go type string
	Variadic  bool
}

// =============================================================================
// TYPE MAPPING
// =============================================================================

// typeMapping maps a Go type to its Starlark representations.
type typeMapping struct {
	unpackType string // Go type for starlark.UnpackArgs (e.g., "string")
	slotReader string // fmt pattern for reading from node slot
}

var typeMappings = map[string]typeMapping{
	"string":   {unpackType: "string", slotReader: `node.GetSlot("%s")`},
	"bool":     {unpackType: "bool", slotReader: `node.GetSlot("%s") == "true"`},
	"int":      {unpackType: "int", slotReader: `strconv.Atoi(node.GetSlot("%s"))`},
	"int64":    {unpackType: "int64", slotReader: `strconv.ParseInt(node.GetSlot("%s"), 10, 64)`},
	"[]string": {unpackType: "*starlark.List", slotReader: `strings.Split(node.GetSlot("%s"), ",")`},
}

// =============================================================================
// NAME NORMALIZATION
// =============================================================================

// camelToSnake converts CamelCase Go names to snake_case.
func camelToSnake(s string) string {
	runes := []rune(s)
	var result []rune
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) {
					result = append(result, '_')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					result = append(result, '_')
				}
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// =============================================================================
// GATE VALIDATION
// =============================================================================

// validateReturnSignature checks that a return type string matches (T, error).
func validateReturnSignature(returns string) (string, error) {
	if returns == "" {
		return "", fmt.Errorf("must return (T, error), got empty return")
	}
	if !strings.HasPrefix(returns, "(") || !strings.HasSuffix(returns, ")") {
		return "", fmt.Errorf("must return (T, error), got %s", returns)
	}
	inner := returns[1 : len(returns)-1]
	if !strings.HasSuffix(inner, ", error") {
		return "", fmt.Errorf("must return (T, error), got %s", returns)
	}
	valueType := strings.TrimSuffix(inner, ", error")
	if valueType == "" || strings.Contains(valueType, ", ") {
		return "", fmt.Errorf("must return (T, error), got %s", returns)
	}
	return valueType, nil
}

// validateParamTypes checks that all parameter types have Starlark mappings.
func validateParamTypes(params []paramInfo) error {
	var unmapped []string
	for _, p := range params {
		if _, ok := typeMappings[p.GoType]; !ok {
			unmapped = append(unmapped, fmt.Sprintf("%s (%s)", p.GoName, p.GoType))
		}
	}
	if len(unmapped) > 0 {
		return fmt.Errorf("unmapped parameter types: %s", strings.Join(unmapped, ", "))
	}
	return nil
}

// =============================================================================
// TEMPLATE FUNCTIONS
// =============================================================================

var genTemplateFuncs = template.FuncMap{
	"attrNamesList":      tplAttrNamesList,
	"planUnpackArgs":     tplPlanUnpackArgs,
	"realtimeUnpackArgs": tplRealtimeUnpackArgs,
	"slotReaders":        tplSlotReaders,
	"dryRunFmt":          tplDryRunFmt,
	"dryRunVars":         tplDryRunVars,
}

func tplAttrNamesList(methods []methodInfo) string {
	names := make([]string, len(methods))
	for i, m := range methods {
		names[i] = m.SnakeName
	}
	sort.Strings(names)
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = `"` + n + `"`
	}
	return strings.Join(quoted, ", ")
}

func tplPlanUnpackArgs(m methodInfo) string {
	if len(m.Params) == 0 {
		return ""
	}
	var names []string
	var pairs []string
	for _, p := range m.Params {
		names = append(names, p.GoName)
		pairs = append(pairs, fmt.Sprintf(`"%s", &%s`, p.SnakeName, p.GoName))
	}
	var buf strings.Builder
	buf.WriteString("var " + strings.Join(names, ", ") + " starlark.Value\n")
	buf.WriteString(fmt.Sprintf("if err := starlark.UnpackArgs(%q, args, kwargs, %s); err != nil {\nreturn nil, err\n}", m.SnakeName, strings.Join(pairs, ", ")))
	return buf.String()
}

func tplRealtimeUnpackArgs(m methodInfo) string {
	if len(m.Params) == 0 {
		return ""
	}
	var decls []string
	var pairs []string
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		decls = append(decls, fmt.Sprintf("var %s %s", p.GoName, tm.unpackType))
		pairs = append(pairs, fmt.Sprintf(`"%s", &%s`, p.SnakeName, p.GoName))
	}
	var buf strings.Builder
	for _, d := range decls {
		buf.WriteString(d + "\n")
	}
	buf.WriteString(fmt.Sprintf("if err := starlark.UnpackArgs(%q, args, kwargs, %s); err != nil {\nreturn nil, err\n}", m.SnakeName, strings.Join(pairs, ", ")))
	return buf.String()
}

func tplSlotReaders(params []paramInfo) string {
	if len(params) == 0 {
		return ""
	}
	var lines []string
	for _, p := range params {
		lines = append(lines, fmt.Sprintf("%s := node.GetSlot(%q)", p.GoName, p.SnakeName))
	}
	return strings.Join(lines, "\n")
}

func tplDryRunFmt(params []paramInfo) string {
	parts := make([]string, len(params))
	for i := range params {
		parts[i] = "%s"
	}
	return strings.Join(parts, " ")
}

func tplDryRunVars(params []paramInfo) string {
	if len(params) == 0 {
		return ""
	}
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.GoName
	}
	return ", " + strings.Join(names, ", ")
}

// =============================================================================
// TEMPLATES
// =============================================================================

var planReceiverTemplate = template.Must(
	template.New("plan_receiver").Funcs(genTemplateFuncs).Parse(`// Code generated by go.generate; DO NOT EDIT.

package {{.Package}}

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/NobleFactor/devlore-cli/internal/execution"
	"github.com/NobleFactor/devlore-cli/internal/host"
)

type {{.StructName}}Plan struct {
	graph   *execution.Graph
	host    host.Host
	project string
}

func New{{.StructName}}Plan(graph *execution.Graph, h host.Host, project string) *{{.StructName}}Plan {
	return &{{.StructName}}Plan{graph: graph, host: h, project: project}
}

func (p *{{.StructName}}Plan) String() string        { return "{{.Namespace}}" }
func (p *{{.StructName}}Plan) Type() string          { return "{{.Namespace}}" }
func (p *{{.StructName}}Plan) Freeze()               {}
func (p *{{.StructName}}Plan) Truth() starlark.Bool  { return true }
func (p *{{.StructName}}Plan) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: {{.Namespace}}") }

func (p *{{.StructName}}Plan) Attr(name string) (starlark.Value, error) {
	switch name {
{{- range .Methods}}
	case "{{.SnakeName}}":
		return starlark.NewBuiltin("{{$.Namespace}}.{{.SnakeName}}", p.{{.SnakeName}}), nil
{{- end}}
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("{{.Namespace}} has no attribute %q", name))
	}
}

func (p *{{.StructName}}Plan) AttrNames() []string {
	return []string{{"{"}}{{attrNamesList .Methods}}{{"}"}}
}
{{range .Methods}}
func (p *{{$.StructName}}Plan) {{.SnakeName}}(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
{{planUnpackArgs .}}

	node := &execution.Node{
		ID:        generateNodeID("{{.SnakeName}}"),
		Operation: "{{$.Category}}.{{.SnakeName}}",
		Project:   p.project,
	}
{{range .Params}}
	if err := FillSlot(node, p.graph, "{{.SnakeName}}", {{.GoName}}); err != nil {
		return nil, fmt.Errorf("{{.SnakeName}}: %w", err)
	}
{{- end}}

	p.graph.Nodes = append(p.graph.Nodes, node)
	return NewOutput(node, p.graph, ""), nil
}
{{end}}`))

var graphOpsTemplate = template.Must(
	template.New("graph_ops").Funcs(genTemplateFuncs).Parse(`// Code generated by go.generate; DO NOT EDIT.

package {{.Package}}

import "fmt"
{{range .Methods}}
type {{$.StructName}}{{.GoName}}Op struct{}

func (o *{{$.StructName}}{{.GoName}}Op) Name() string         { return "{{$.Category}}.{{.SnakeName}}" }
func (o *{{$.StructName}}{{.GoName}}Op) Category() OpCategory { return OpDirect }

func (o *{{$.StructName}}{{.GoName}}Op) Execute(ctx *Context, node Executable) error {
{{slotReaders .Params}}

	if ctx.DryRun {
		_, _ = fmt.Fprintf(ctx.Logger, "[dry-run] {{$.Category}}.{{.SnakeName}} {{dryRunFmt .Params}}\n"{{dryRunVars .Params}})
		return nil
	}

	_, _ = fmt.Fprintf(ctx.Logger, "[{{$.Category}}] {{.SnakeName}} {{dryRunFmt .Params}}\n"{{dryRunVars .Params}})
	// TODO: call backing implementation
	return nil
}
{{end}}
func {{.StructName}}Ops() []Operation {
	return []Operation{
{{- range .Methods}}
		&{{$.StructName}}{{.GoName}}Op{},
{{- end}}
	}
}
`))

var realtimeReceiverTemplate = template.Must(
	template.New("realtime_receiver").Funcs(genTemplateFuncs).Parse(`// Code generated by go.generate; DO NOT EDIT.

package {{.Package}}

import (
	"go.starlark.net/starlark"
)

type {{.StructName}}Receiver struct {
	Receiver
}

func New{{.StructName}}Receiver() *{{.StructName}}Receiver {
	return &{{.StructName}}Receiver{Receiver: NewReceiver("{{.Category}}")}
}

func (r *{{.StructName}}Receiver) Attr(name string) (starlark.Value, error) {
	switch name {
{{- range .Methods}}
	case "{{.SnakeName}}":
		return MakeAttr("{{$.Category}}.{{.SnakeName}}", r.{{.SnakeName}}), nil
{{- end}}
	default:
		return nil, NoSuchAttrError("{{.Category}}", name)
	}
}

func (r *{{.StructName}}Receiver) AttrNames() []string {
	return []string{{"{"}}{{attrNamesList .Methods}}{{"}"}}
}
{{range .Methods}}
func (r *{{$.StructName}}Receiver) {{.SnakeName}}(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
{{realtimeUnpackArgs .}}
	// TODO: call backing implementation, convert result
	return starlark.None, nil
}
{{end}}`))

// =============================================================================
// go.generate() METHOD
// =============================================================================

func (r *GoReceiver) goGenerate(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var templateName string
	var descriptorVal starlark.Value
	if err := starlark.UnpackArgs("go.generate", args, kwargs, "template", &templateName, "descriptor", &descriptorVal); err != nil {
		return nil, err
	}

	// Validate template name
	var tmpl *template.Template
	switch templateName {
	case "plan_receiver":
		tmpl = planReceiverTemplate
	case "graph_ops":
		tmpl = graphOpsTemplate
	case "realtime_receiver":
		tmpl = realtimeReceiverTemplate
	default:
		return nil, fmt.Errorf("go.generate: unknown template %q (valid: plan_receiver, graph_ops, realtime_receiver)", templateName)
	}

	// Convert descriptor
	desc, err := descriptorFromValue(templateName, descriptorVal)
	if err != nil {
		return nil, fmt.Errorf("go.generate: %w", err)
	}

	// Gate 1: validate param types
	for _, m := range desc.Methods {
		if err := validateParamTypes(m.Params); err != nil {
			return nil, fmt.Errorf("go.generate: method %s: %w", m.GoName, err)
		}
	}

	// Gate 2: validate return signatures
	for i, m := range desc.Methods {
		valueType, err := validateReturnSignature(m.ReturnType)
		if err != nil {
			return nil, fmt.Errorf("go.generate: method %s: %w", m.GoName, err)
		}
		desc.Methods[i].ReturnType = valueType
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, desc); err != nil {
		return nil, fmt.Errorf("go.generate: template execution: %w", err)
	}

	// Format output
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("go.generate: format error: %w\nraw output:\n%s", err, buf.String())
	}

	return starlark.String(string(formatted)), nil
}

// =============================================================================
// DESCRIPTOR CONVERSION
// =============================================================================

// descriptorFromValue converts a Starlark dict or struct to a generateDescriptor.
func descriptorFromValue(templateName string, v starlark.Value) (*generateDescriptor, error) {
	desc := &generateDescriptor{Template: templateName}

	pkg, err := valueGetString(v, "package")
	if err != nil {
		return nil, fmt.Errorf("descriptor.package: %w", err)
	}
	desc.Package = pkg

	category, err := valueGetString(v, "category")
	if err != nil {
		return nil, fmt.Errorf("descriptor.category: %w", err)
	}
	desc.Category = category

	structName, err := valueGetString(v, "struct_name")
	if err != nil {
		return nil, fmt.Errorf("descriptor.struct_name: %w", err)
	}
	desc.StructName = structName

	namespace, err := valueGetString(v, "namespace")
	if err != nil {
		return nil, fmt.Errorf("descriptor.namespace: %w", err)
	}
	desc.Namespace = namespace

	methodsVal, err := valueGetList(v, "methods")
	if err != nil {
		return nil, fmt.Errorf("descriptor.methods: %w", err)
	}

	for i := 0; i < methodsVal.Len(); i++ {
		m, err := methodInfoFromValue(methodsVal.Index(i))
		if err != nil {
			return nil, fmt.Errorf("descriptor.methods[%d]: %w", i, err)
		}
		desc.Methods = append(desc.Methods, m)
	}

	return desc, nil
}

// methodInfoFromValue converts a Starlark dict or struct to a methodInfo.
func methodInfoFromValue(v starlark.Value) (methodInfo, error) {
	name, err := valueGetString(v, "name")
	if err != nil {
		return methodInfo{}, fmt.Errorf("name: %w", err)
	}

	returns, err := valueGetString(v, "returns")
	if err != nil {
		return methodInfo{}, fmt.Errorf("returns: %w", err)
	}

	doc, _ := valueGetString(v, "doc") // doc is optional

	paramsVal, err := valueGetList(v, "params")
	if err != nil {
		return methodInfo{}, fmt.Errorf("params: %w", err)
	}

	var params []paramInfo
	for i := 0; i < paramsVal.Len(); i++ {
		p, err := paramInfoFromValue(paramsVal.Index(i))
		if err != nil {
			return methodInfo{}, fmt.Errorf("params[%d]: %w", i, err)
		}
		params = append(params, p)
	}

	return methodInfo{
		GoName:     name,
		SnakeName:  camelToSnake(name),
		Params:     params,
		ReturnType: returns,
		Doc:        doc,
	}, nil
}

// paramInfoFromValue converts a Starlark dict or struct to a paramInfo.
func paramInfoFromValue(v starlark.Value) (paramInfo, error) {
	name, err := valueGetString(v, "name")
	if err != nil {
		return paramInfo{}, fmt.Errorf("name: %w", err)
	}

	goType, err := valueGetString(v, "type")
	if err != nil {
		return paramInfo{}, fmt.Errorf("type: %w", err)
	}

	variadic, err := valueGetBool(v, "variadic")
	if err != nil {
		return paramInfo{}, fmt.Errorf("variadic: %w", err)
	}

	return paramInfo{
		GoName:    name,
		SnakeName: camelToSnake(name),
		GoType:    goType,
		Variadic:  variadic,
	}, nil
}

// =============================================================================
// VALUE EXTRACTION HELPERS
// =============================================================================

// valueGetString extracts a string field from a dict or struct.
func valueGetString(v starlark.Value, key string) (string, error) {
	switch val := v.(type) {
	case *starlark.Dict:
		result, found, err := val.Get(starlark.String(key))
		if err != nil {
			return "", err
		}
		if !found {
			return "", nil
		}
		s, ok := starlark.AsString(result)
		if !ok {
			return "", fmt.Errorf("expected string, got %s", result.Type())
		}
		return s, nil
	case *starlarkstruct.Struct:
		attr, err := val.Attr(key)
		if err != nil {
			return "", nil
		}
		s, ok := starlark.AsString(attr)
		if !ok {
			return "", fmt.Errorf("expected string, got %s", attr.Type())
		}
		return s, nil
	default:
		return "", fmt.Errorf("expected dict or struct, got %s", v.Type())
	}
}

// valueGetBool extracts a bool field from a dict or struct.
func valueGetBool(v starlark.Value, key string) (bool, error) {
	switch val := v.(type) {
	case *starlark.Dict:
		result, found, err := val.Get(starlark.String(key))
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}
		b, ok := result.(starlark.Bool)
		if !ok {
			return false, fmt.Errorf("expected bool, got %s", result.Type())
		}
		return bool(b), nil
	case *starlarkstruct.Struct:
		attr, err := val.Attr(key)
		if err != nil {
			return false, nil
		}
		b, ok := attr.(starlark.Bool)
		if !ok {
			return false, fmt.Errorf("expected bool, got %s", attr.Type())
		}
		return bool(b), nil
	default:
		return false, fmt.Errorf("expected dict or struct, got %s", v.Type())
	}
}

// valueGetList extracts a list field from a dict or struct.
func valueGetList(v starlark.Value, key string) (*starlark.List, error) {
	switch val := v.(type) {
	case *starlark.Dict:
		result, found, err := val.Get(starlark.String(key))
		if err != nil {
			return nil, err
		}
		if !found {
			return starlark.NewList(nil), nil
		}
		list, ok := result.(*starlark.List)
		if !ok {
			return nil, fmt.Errorf("expected list, got %s", result.Type())
		}
		return list, nil
	case *starlarkstruct.Struct:
		attr, err := val.Attr(key)
		if err != nil {
			return starlark.NewList(nil), nil
		}
		list, ok := attr.(*starlark.List)
		if !ok {
			return nil, fmt.Errorf("expected list, got %s", attr.Type())
		}
		return list, nil
	default:
		return nil, fmt.Errorf("expected dict or struct, got %s", v.Type())
	}
}
