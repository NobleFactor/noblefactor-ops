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
	"gopkg.in/yaml.v3"
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
	ImplType   string       // implementation struct name for delegation (e.g., "fileOps")
	Methods    []methodInfo // analyzed methods
}

// methodInfo holds analyzed information about a single method.
type methodInfo struct {
	GoName       string      // original Go name (e.g., "Copy")
	SnakeName    string      // snake_case name (e.g., "copy")
	Params       []paramInfo
	ReturnType   string // value type from (T, error), empty for error-only
	ContentModel string // "none", "consumer", "transformer"
	Doc          string
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
	unpackType     string // Go type for starlark.UnpackArgs (e.g., "string")
	slotReader     string // fmt pattern for reading from node slot
	starlarkFacing bool   // include in plan receiver UnpackArgs/FillSlot
	contextReader  string // if set, read from this expr instead of a slot
}

var typeMappings = map[string]typeMapping{
	// Starlark-facing: in plan UnpackArgs + graph ops slot readers
	"string":         {unpackType: "string", slotReader: `node.GetSlot("%s").(string)`, starlarkFacing: true},
	"bool":           {unpackType: "bool", slotReader: `node.GetSlot("%s").(bool)`, starlarkFacing: true},
	"int":            {unpackType: "int", slotReader: `node.GetSlot("%s").(int)`, starlarkFacing: true},
	"int64":          {unpackType: "int64", slotReader: `node.GetSlot("%s").(int64)`, starlarkFacing: true},
	"[]string":       {unpackType: "*starlark.List", slotReader: `node.GetSlot("%s").([]string)`, starlarkFacing: true},
	"os.FileMode":    {unpackType: "int", slotReader: `node.GetSlot("%s").(os.FileMode)`, starlarkFacing: true},
	"map[string]any": {unpackType: "*starlark.Dict", slotReader: `node.GetSlot("%s").(map[string]any)`, starlarkFacing: true},
	// Engine-injected: graph ops slot readers only (filled by engine from ctx.Data)
	"func(string, []byte) ([]byte, error)": {slotReader: `node.GetSlot("%s").(func(string, []byte) ([]byte, error))`},
	"func(string, string) error":           {slotReader: `node.GetSlot("%s").(func(string, string) error)`},
	// Context-provided: read from context expression, not slots
	"io.Writer": {contextReader: "ctx.Logger"},
	// Content pipeline: handled by content model inference, not mappings
	"[]byte": {},
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
		return "", fmt.Errorf("must return error or (T, error), got empty return")
	}
	// Plain error return — no value type
	if returns == "error" {
		return "", nil
	}
	if !strings.HasPrefix(returns, "(") || !strings.HasSuffix(returns, ")") {
		return "", fmt.Errorf("must return error or (T, error), got %s", returns)
	}
	inner := returns[1 : len(returns)-1]
	if !strings.HasSuffix(inner, ", error") {
		return "", fmt.Errorf("must return error or (T, error), got %s", returns)
	}
	valueType := strings.TrimSuffix(inner, ", error")
	if valueType == "" || strings.Contains(valueType, ", ") {
		return "", fmt.Errorf("must return error or (T, error), got %s", returns)
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
// CONTENT MODEL INFERENCE
// =============================================================================

// inferContentModel determines how a method participates in the content pipeline.
func inferContentModel(valueType string, params []paramInfo) string {
	if valueType == "" {
		return "none"
	}
	if len(params) == 0 {
		return "none"
	}
	lastParam := params[len(params)-1]
	if lastParam.GoType != "[]byte" {
		return "none"
	}
	switch valueType {
	case "string":
		return "consumer"
	case "[]byte":
		return "transformer"
	default:
		return "none"
	}
}

// isContentParam returns true if p is the content pipeline parameter for m.
func isContentParam(p paramInfo, m methodInfo) bool {
	if m.ContentModel == "none" {
		return false
	}
	// The content param is the last []byte param.
	for i := len(m.Params) - 1; i >= 0; i-- {
		if m.Params[i].GoType == "[]byte" {
			return m.Params[i].GoName == p.GoName
		}
	}
	return false
}

// =============================================================================
// TEMPLATE FUNCTIONS
// =============================================================================

var genTemplateFuncs = template.FuncMap{
	"attrNamesList":      tplAttrNamesList,
	"planUnpackArgs":     tplPlanUnpackArgs,
	"planFillSlots":      tplPlanFillSlots,
	"realtimeUnpackArgs": tplRealtimeUnpackArgs,
	"graphReaders":       tplGraphReaders,
	"dryRunFmt":          tplDryRunFmt,
	"dryRunVars":         tplDryRunVars,
	"dryRunChecksum":     tplDryRunChecksum,
	"implArgs":           tplImplArgs,
	"graphReturn":        tplGraphReturn,
	"methodSignature":    tplMethodSignature,
	"methodReturn":       tplMethodReturn,
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
	var starlarkParams []paramInfo
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if !tm.starlarkFacing {
			continue
		}
		if isContentParam(p, m) {
			continue
		}
		starlarkParams = append(starlarkParams, p)
	}
	if len(starlarkParams) == 0 {
		return ""
	}
	var names []string
	var pairs []string
	for _, p := range starlarkParams {
		names = append(names, p.GoName)
		pairs = append(pairs, fmt.Sprintf(`"%s", &%s`, p.SnakeName, p.GoName))
	}
	var buf strings.Builder
	buf.WriteString("var " + strings.Join(names, ", ") + " starlark.Value\n")
	buf.WriteString(fmt.Sprintf("if err := starlark.UnpackArgs(%q, args, kwargs, %s); err != nil {\nreturn nil, err\n}", m.SnakeName, strings.Join(pairs, ", ")))
	return buf.String()
}

// tplPlanFillSlots generates FillSlot calls for starlark-facing params only.
func tplPlanFillSlots(m methodInfo) string {
	var lines []string
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if !tm.starlarkFacing {
			continue
		}
		if isContentParam(p, m) {
			continue
		}
		lines = append(lines, fmt.Sprintf("if err := FillSlot(node, p.graph, %q, %s); err != nil {\nreturn nil, fmt.Errorf(%q, err)\n}", p.SnakeName, p.GoName, p.SnakeName+": %w"))
	}
	return strings.Join(lines, "\n")
}

func tplRealtimeUnpackArgs(m methodInfo) string {
	if len(m.Params) == 0 {
		return ""
	}
	var decls []string
	var pairs []string
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if !tm.starlarkFacing {
			continue
		}
		decls = append(decls, fmt.Sprintf("var %s %s", p.GoName, tm.unpackType))
		pairs = append(pairs, fmt.Sprintf(`"%s", &%s`, p.SnakeName, p.GoName))
	}
	if len(decls) == 0 {
		return ""
	}
	var buf strings.Builder
	for _, d := range decls {
		buf.WriteString(d + "\n")
	}
	buf.WriteString(fmt.Sprintf("if err := starlark.UnpackArgs(%q, args, kwargs, %s); err != nil {\nreturn nil, err\n}", m.SnakeName, strings.Join(pairs, ", ")))
	return buf.String()
}

// tplGraphReaders generates variable declarations for Execute: slot reads,
// context reads, engine-injected reads, and content pipeline reads.
func tplGraphReaders(m methodInfo) string {
	var slotLines, contextLines, engineLines []string

	for _, p := range m.Params {
		if isContentParam(p, m) {
			continue
		}
		tm := typeMappings[p.GoType]
		if tm.contextReader != "" {
			contextLines = append(contextLines, fmt.Sprintf("%s := %s", p.GoName, tm.contextReader))
		} else if tm.starlarkFacing && tm.slotReader != "" {
			slotLines = append(slotLines, fmt.Sprintf("%s := "+tm.slotReader, p.GoName, p.SnakeName))
		} else if tm.slotReader != "" {
			engineLines = append(engineLines, fmt.Sprintf("%s := "+tm.slotReader, p.GoName, p.SnakeName))
		}
	}

	var lines []string
	lines = append(lines, slotLines...)
	lines = append(lines, contextLines...)
	lines = append(lines, engineLines...)

	if m.ContentModel != "none" {
		lines = append(lines, "content, err := ContentFor(ctx, node)")
		lines = append(lines, "if err != nil {\nreturn err\n}")
	}

	return strings.Join(lines, "\n")
}

// tplDryRunFmt generates format verbs for starlark-facing params only.
func tplDryRunFmt(m methodInfo) string {
	var parts []string
	for _, p := range m.Params {
		if isContentParam(p, m) {
			continue
		}
		tm := typeMappings[p.GoType]
		if !tm.starlarkFacing {
			continue
		}
		parts = append(parts, "%v")
	}
	return strings.Join(parts, " ")
}

// tplDryRunVars generates variable names for starlark-facing params only.
func tplDryRunVars(m methodInfo) string {
	var names []string
	for _, p := range m.Params {
		if isContentParam(p, m) {
			continue
		}
		tm := typeMappings[p.GoType]
		if !tm.starlarkFacing {
			continue
		}
		names = append(names, p.GoName)
	}
	if len(names) == 0 {
		return ""
	}
	return ", " + strings.Join(names, ", ")
}

// tplDryRunChecksum generates the checksum line for consumer content model.
func tplDryRunChecksum(m methodInfo) string {
	if m.ContentModel != "consumer" {
		return ""
	}
	return "\nctx.TargetChecksum = ChecksumBytes(content)"
}

// tplImplArgs generates all param names in order for the delegation call.
func tplImplArgs(m methodInfo) string {
	names := make([]string, len(m.Params))
	for i, p := range m.Params {
		names[i] = p.GoName
	}
	return strings.Join(names, ", ")
}

// tplGraphReturn generates the delegation call and return handling per content model.
func tplGraphReturn(m methodInfo, implType string) string {
	if implType == "" {
		return "\n// TODO: call backing implementation\nreturn nil"
	}

	argStr := tplImplArgs(m)
	call := fmt.Sprintf("o.impl.%s(%s)", m.GoName, argStr)

	switch m.ContentModel {
	case "consumer":
		// err already declared by ContentFor
		return fmt.Sprintf("\n_, err = %s\nreturn err", call)
	case "transformer":
		// err already declared by ContentFor; result is new so := works
		return fmt.Sprintf("\nresult, err := %s\nif err != nil {\nreturn err\n}\nStoreContent(ctx, node, result)\nreturn nil", call)
	default: // "none"
		if m.ReturnType == "" {
			return "\nreturn " + call
		}
		return fmt.Sprintf("\n_, err := %s\nreturn err", call)
	}
}

// tplMethodSignature generates the parameter list for the ops interface.
func tplMethodSignature(m methodInfo) string {
	parts := make([]string, len(m.Params))
	for i, p := range m.Params {
		parts[i] = p.GoName + " " + p.GoType
	}
	return strings.Join(parts, ", ")
}

// tplMethodReturn generates the return type for the ops interface.
func tplMethodReturn(m methodInfo) string {
	if m.ReturnType == "" {
		return "error"
	}
	return "(" + m.ReturnType + ", error)"
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
	Receiver
	graph   *execution.Graph
	host    host.Host
	project string
}

func New{{.StructName}}Plan(graph *execution.Graph, h host.Host, project string) *{{.StructName}}Plan {
	return &{{.StructName}}Plan{
		Receiver: NewReceiver("{{.Namespace}}"),
		graph:    graph,
		host:     h,
		project:  project,
	}
}

func (p *{{.StructName}}Plan) Attr(name string) (starlark.Value, error) {
	switch name {
{{- range .Methods}}
	case "{{.SnakeName}}":
		return MakeAttr("{{$.Namespace}}.{{.SnakeName}}", p.{{.SnakeName}}), nil
{{- end}}
	default:
		return nil, NoSuchAttrError("{{.Namespace}}", name)
	}
}

func (p *{{.StructName}}Plan) AttrNames() []string {
	return []string{{"{"}}{{attrNamesList .Methods}}{{"}"}}
}
{{range .Methods}}
func (p *{{$.StructName}}Plan) {{.SnakeName}}(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
{{planUnpackArgs .}}

	node := &execution.Node{
		ID:        generateNodeID("{{$.Category}}.{{.SnakeName}}"),
		Operation: "{{$.Category}}.{{.SnakeName}}",
		Project:   p.project,
	}
{{planFillSlots .}}

	p.graph.Nodes = append(p.graph.Nodes, node)
	return NewOutput(node, p.graph, ""), nil
}
{{end}}`))

var graphOpsTemplate = template.Must(
	template.New("graph_ops").Funcs(genTemplateFuncs).Parse(`// Code generated by go.generate; DO NOT EDIT.

package {{.Package}}

import "fmt"
{{- if .ImplType}}

type {{.ImplType}} interface {
{{range .Methods}}	{{.GoName}}({{methodSignature .}}) {{methodReturn .}}
{{end}}}
{{- end}}
{{range .Methods}}
{{- if $.ImplType}}
type {{$.StructName}}{{.GoName}}Op struct{ impl {{$.ImplType}} }
{{- else}}
type {{$.StructName}}{{.GoName}}Op struct{}
{{- end}}

func (o *{{$.StructName}}{{.GoName}}Op) Name() string { return "{{$.Category}}.{{.SnakeName}}" }

func (o *{{$.StructName}}{{.GoName}}Op) Execute(ctx *Context, node *Node) error {
{{graphReaders .}}

	if ctx.DryRun {
		_, _ = fmt.Fprintf(ctx.Logger, "[dry-run] {{$.Category}}.{{.SnakeName}} {{dryRunFmt .}}\n"{{dryRunVars .}}){{dryRunChecksum .}}
		return nil
	}
{{graphReturn . $.ImplType}}
}
{{end}}
{{- if .ImplType}}
func {{.StructName}}Ops(impl {{.ImplType}}) []Operation {
	return []Operation{
{{- range .Methods}}
		&{{$.StructName}}{{.GoName}}Op{impl: impl},
{{- end}}
	}
}
{{- else}}
func {{.StructName}}Ops() []Operation {
	return []Operation{
{{- range .Methods}}
		&{{$.StructName}}{{.GoName}}Op{},
{{- end}}
	}
}
{{- end}}
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

	// Gate 2: validate return signatures and infer content models
	for i, m := range desc.Methods {
		valueType, err := validateReturnSignature(m.ReturnType)
		if err != nil {
			return nil, fmt.Errorf("go.generate: method %s: %w", m.GoName, err)
		}
		desc.Methods[i].ReturnType = valueType
		desc.Methods[i].ContentModel = inferContentModel(valueType, m.Params)
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
// go.mapping() METHOD
// =============================================================================

// mappingOperation holds one operation entry for the mapping YAML.
type mappingOperation struct {
	Name     string         `yaml:"name"`
	GoMethod string         `yaml:"go_method"`
	Params   []mappingParam `yaml:"params,omitempty"`
}

// mappingParam holds one parameter entry for the mapping YAML.
type mappingParam struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
}

// mappingFile is the top-level structure for the mapping YAML.
type mappingFile struct {
	Version    string             `yaml:"version"`
	Struct     string             `yaml:"struct"`
	Package    string             `yaml:"package"`
	Category   string             `yaml:"category"`
	Namespace  string             `yaml:"namespace"`
	Operations []mappingOperation `yaml:"operations"`
}

func (r *GoReceiver) goMapping(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var descriptorVal starlark.Value
	if err := starlark.UnpackArgs("go.mapping", args, kwargs, "descriptor", &descriptorVal); err != nil {
		return nil, err
	}

	// Convert descriptor (use "mapping" as template name for descriptorFromValue)
	desc, err := descriptorFromValue("mapping", descriptorVal)
	if err != nil {
		return nil, fmt.Errorf("go.mapping: %w", err)
	}

	// Gate 1: validate param types
	for _, m := range desc.Methods {
		if err := validateParamTypes(m.Params); err != nil {
			return nil, fmt.Errorf("go.mapping: method %s: %w", m.GoName, err)
		}
	}

	// Gate 2: validate return signatures
	for _, m := range desc.Methods {
		if _, err := validateReturnSignature(m.ReturnType); err != nil {
			return nil, fmt.Errorf("go.mapping: method %s: %w", m.GoName, err)
		}
	}

	// Build mapping structure
	mapping := mappingFile{
		Version:   "1.0",
		Struct:    desc.ImplType,
		Package:   desc.Package,
		Category:  desc.Category,
		Namespace: desc.Namespace,
	}

	for _, m := range desc.Methods {
		op := mappingOperation{
			Name:     desc.Category + "." + m.SnakeName,
			GoMethod: m.GoName,
		}

		for _, p := range m.Params {
			op.Params = append(op.Params, mappingParam{
				Name:     p.SnakeName,
				Type:     p.GoType,
				Required: true,
			})
		}

		mapping.Operations = append(mapping.Operations, op)
	}

	// Serialize to YAML
	var buf bytes.Buffer
	buf.WriteString("# Generated by go.mapping — DO NOT EDIT\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(mapping); err != nil {
		return nil, fmt.Errorf("go.mapping: yaml encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("go.mapping: yaml close: %w", err)
	}

	return starlark.String(buf.String()), nil
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

	implType, _ := valueGetString(v, "impl_type") // optional
	desc.ImplType = implType

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
