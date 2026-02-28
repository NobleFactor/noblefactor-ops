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
	Template       string       // "planned_receiver", "graph_actions", "immediate_receiver"
	Package        string       // Go package name for generated file
	Provider       string       // snake_case provider (e.g., "file")
	StructName     string       // Go struct name (e.g., "File")
	WrapperSuffix  string       // suffix for the wrapper type: "Receiver" (default) or "Value"
	Namespace      string       // dotted namespace (e.g., "plan.file")
	ImplType       string       // implementation struct name for delegation (e.g., "fileOps")
	Methods        []methodInfo // analyzed methods
	ExtraAttrs     []string     // additional attr names from companion files (e.g., query methods)
	AllMethodNames []string     // all method names on the provider (for compensate validation)
	Access         string       // access level: "immediate", "planned", "both"
	AccessTitle    string       // title-case access for Go constants: "Immediate", "Planned", "Both"
	Lifetime       string       // lifetime level: "stateless", "phase", "session"
	LifetimeTitle  string       // title-case lifetime for Go constants: "Stateless", "Phase", "Session"
	Registered     bool             // if true, emit init()/RegisterBinding; false for dependent types
	ProviderFields []providerField  // dynamic field init for ImmediateFactory (replaces hardcoded Writer/ProgramName/Color)
	ProviderImport string           // if non-empty, import path for parent package (gen/ subpackage mode)
	Converters          []converterInfo      // struct-to-Starlark converter functions (for struct_converter template)
	CrossPackageImports []crossPackageImport // cross-package gen imports (e.g., starstatsgen → .../starstats/gen)
}

// crossPackageImport describes a gen import from a sibling provider package.
type crossPackageImport struct {
	Alias string // import alias, e.g., "starstatsgen"
	Path  string // full import path, e.g., "github.com/NobleFactor/devlore-cli/pkg/op/provider/starstats/gen"
}

// providerField describes a Provider struct field and how it maps to BindingConfig.
type providerField struct {
	GoName    string // field name on Provider struct (e.g., "Root")
	CfgField  string // BindingConfig field (e.g., "WorkDir")
	Default   string // Go default expression (e.g., `"."`)
	ZeroValue string // Go zero-value expression for comparison (e.g., `""`)
}

// converterInfo describes a struct-to-Starlark converter function.
type converterInfo struct {
	FuncName     string               // function name, e.g., "indexToStarlark"
	GoType       string               // Go type name, e.g., "Index"
	IsPointer    bool                 // true for *T parameter
	StarlarkName string               // starlark struct type name, e.g., "index"
	Fields       []converterFieldInfo // struct fields to convert
}

// converterFieldInfo describes a single field in a struct converter.
type converterFieldInfo struct {
	GoName       string               // Go field name, e.g., "Files"
	SnakeName    string               // Starlark attr name, e.g., "files"
	Kind         string               // "string", "int", "int64", "bool", "string_slice", "struct_slice", "struct_ptr", "inline_struct", "conditional"
	Converter    string               // named converter function for struct_slice/struct_ptr
	InlineName   string               // starlark name for inline_struct/struct_slice+inline
	InlineFields []converterFieldInfo // nested fields for inline_struct/struct_slice+inline
	Nullable     bool                 // emit nil check (struct_ptr, struct_slice)
	NilExpr      string               // expression when nil (default: "starlark.None")
	Guard        string               // for conditional: bool field name
	GuardField   string               // for conditional: field to access when guard is true
	GuardKind    string               // for conditional: conversion kind for true case
}

// methodInfo holds analyzed information about a single method.
type methodInfo struct {
	GoName       string      // original Go name (e.g., "Copy")
	SnakeName    string      // snake_case name (e.g., "copy")
	Params       []paramInfo
	ReturnType   string // value type from (T, error), empty for error-only
	ResultExpr   string // override for result expression (e.g., "indexToStarlark(result)")
	HasError     bool   // true if the method returns an error (standard/compensable)
	ContentModel string // "none", "consumer", "transformer"
	Compensable    bool   // inferred from return signature: (T, U, error) = true
	CompStateType  string // Go type U from (T, U, error) — used for typed assertion in Undo
	Property     bool   // if true, exposed as read-only property (direct Attr value, no callable)
	Doc          string
	File         string // source file basename (e.g., "provider.go")
	Line         int    // source line number
}

// paramInfo holds information about a single parameter.
type paramInfo struct {
	GoName     string // original param name
	SnakeName  string // snake_case name
	GoType     string // Go type string
	Variadic   bool
	Doc        string // parameter description from Parameters: section
	Optional   bool   // if true, param is keyword-optional (? suffix in UnpackArgs)
	Default    string // Go default expression (e.g., "true", "10"); empty = zero value
	StructType string // if set, param was expanded from this struct type (fields already validated)
	Callable   *callableInfo // if set, this param is a callable (function type) with classified params
}

// callableInfo describes a function-type parameter with classified params.
type callableInfo struct {
	TypeName    string          // Go type name (e.g., "Visitor")
	Params      []callableParam // classified parameters
	Returns     string          // return type string (e.g., "(any, error)")
	HandleTypes []handleType    // handle specs from the directive
}

// callableParam describes a single classified parameter of a callable.
type callableParam struct {
	GoName string // Go parameter name (e.g., "path")
	GoType string // Go type string (e.g., "string")
	Role   string // "projected", "pass_through", "swallowed", "handle"
}

// handleType describes a Go type to be wrapped as a Starlark HasAttrs handle.
type handleType struct {
	GoType     string         // Go type (e.g., "os.DirEntry")
	HandleName string         // generated handle name (e.g., "DirEntryHandle")
	Methods    []handleMethod // methods to expose as attributes
}

// handleMethod describes a single method exposed on a handle type.
type handleMethod struct {
	GoName     string // Go method name (e.g., "Name")
	SnakeName  string // snake_case attr name (e.g., "name")
	ReturnType string // Go return type (e.g., "string")
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
	needsConstruct bool   // emit op.Construct[provider.GoType] in action Do()
	constructType  string // Go type name for Construct (e.g., "Blob")
}

var typeMappings = map[string]typeMapping{
	// Starlark-facing: in plan UnpackArgs + graph actions slot readers
	"string":         {unpackType: "string", slotReader: `slots["%s"].(string)`, starlarkFacing: true},
	"bool":           {unpackType: "bool", slotReader: `slots["%s"].(bool)`, starlarkFacing: true},
	"int":            {unpackType: "int", slotReader: `slots["%s"].(int)`, starlarkFacing: true},
	"int64":          {unpackType: "int64", slotReader: `slots["%s"].(int64)`, starlarkFacing: true},
	"[]string":       {unpackType: "*starlark.List", slotReader: `slots["%s"].([]string)`, starlarkFacing: true},
	"os.FileMode":    {unpackType: "int", slotReader: `slots["%s"].(os.FileMode)`, starlarkFacing: true},
	"Blob":           {unpackType: "string", slotReader: `slots["%s"].(string)`, starlarkFacing: true, needsConstruct: true, constructType: "Blob"},
	"map[string]any": {unpackType: "*starlark.Dict", slotReader: `slots["%s"].(map[string]any)`, starlarkFacing: true},
	// Engine-injected: graph actions slot readers only (filled by engine from ctx.Data)
	"func(string, []byte) ([]byte, error)": {slotReader: `slots["%s"].(func(string, []byte) ([]byte, error))`},
	"func(string, string) error":           {slotReader: `slots["%s"].(func(string, string) error)`},
	// Context-provided: read from context expression, not slots
	"io.Writer": {contextReader: "ctx.Writer"},
	// Content: read from slot with optional assertion (may come via promise)
	"[]byte": {slotReader: `slots["%s"].([]byte)`},
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

// methodLocation formats a method's source location for error messages.
// Returns "File:Line: Method" if location is available, otherwise just "Method".
func methodLocation(m methodInfo) string {
	if m.File != "" && m.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", m.File, m.Line, m.GoName)
	}
	return m.GoName
}

// validateReturnSignature checks that a non-compensable method returns (T, error).
// Every provider method has a Result — plain error returns are not valid.
func validateReturnSignature(returns string) (string, error) {
	if returns == "" {
		return "", fmt.Errorf("expected (T, error), got no return value")
	}
	if returns == "error" {
		return "", fmt.Errorf("expected (T, error), got error — every method must return a Result")
	}
	if !strings.HasPrefix(returns, "(") || !strings.HasSuffix(returns, ")") {
		return "", fmt.Errorf("expected (T, error), got %s", returns)
	}
	inner := returns[1 : len(returns)-1]
	if !strings.HasSuffix(inner, ", error") {
		return "", fmt.Errorf("expected (T, error), got %s", returns)
	}
	valueType := strings.TrimSuffix(inner, ", error")
	if valueType == "" {
		return "", fmt.Errorf("expected (T, error), got %s — missing Result type", returns)
	}
	if strings.Contains(valueType, ", ") {
		return "", fmt.Errorf("expected (T, error), got %s — too many return values", returns)
	}
	return valueType, nil
}

// validateCompensableReturn checks that a compensable method returns (T, U, error).
// T is the Result type, U is the compensation state type. Returns both so the
// generator can produce typed assertions in Undo methods.
func validateCompensableReturn(returns string) (valueType string, compStateType string, err error) {
	if returns == "" {
		return "", "", fmt.Errorf("expected (T, U, error), got no return value")
	}
	if !strings.HasPrefix(returns, "(") || !strings.HasSuffix(returns, ")") {
		return "", "", fmt.Errorf("expected (T, U, error), got %s", returns)
	}
	inner := returns[1 : len(returns)-1]
	if !strings.HasSuffix(inner, ", error") {
		return "", "", fmt.Errorf("expected (T, U, error), got %s — must end with error", returns)
	}
	withoutError := strings.TrimSuffix(inner, ", error")
	if withoutError == "" {
		return "", "", fmt.Errorf("expected (T, U, error), got %s — missing Result and UndoState", returns)
	}

	// Count top-level commas (not inside brackets) to find T and U.
	var topLevelCommas []int
	depth := 0
	for i, ch := range withoutError {
		switch ch {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				topLevelCommas = append(topLevelCommas, i)
			}
		}
	}

	switch len(topLevelCommas) {
	case 0:
		// Only one type before error — missing either Result or UndoState.
		return "", "", fmt.Errorf("expected (T, U, error), got (%s, error) — missing Result or UndoState", withoutError)
	case 1:
		// (T, U, error) — Result + UndoState
		vt := strings.TrimSpace(withoutError[:topLevelCommas[0]])
		ut := strings.TrimSpace(withoutError[topLevelCommas[0]+1:])
		if vt == "" {
			return "", "", fmt.Errorf("expected (T, U, error), got %s — empty Result type", returns)
		}
		return vt, ut, nil
	default:
		return "", "", fmt.Errorf("expected (T, U, error), got %s — too many return values", returns)
	}
}

// validateImmediateReturn is a relaxed return-signature validator for immediate receivers.
// Unlike planned/graph receivers (which always require error), immediate receivers support:
//   - ""           → void: valueType="", hasError=false, compensable=false
//   - "error"      → error-only: valueType="", hasError=true, compensable=false
//   - "string"     → bare type: valueType="string", hasError=false, compensable=false
//   - "(T, error)" → standard: hasError=true, compensable=false
//   - "(T, U, error)" → compensable: hasError=true, compensable=true
//
// Compensability is inferred from the return shape — no descriptor flag needed.
func validateImmediateReturn(returns string) (valueType string, hasError bool, compensable bool, err error) {
	switch {
	case returns == "":
		return "", false, false, nil
	case returns == "error":
		return "", true, false, nil
	case !strings.HasPrefix(returns, "("):
		return returns, false, false, nil
	default:
		// Try compensable (T, U, error) first.
		if vt, _, e := validateCompensableReturn(returns); e == nil {
			return vt, true, true, nil
		}
		// Fall back to non-compensable (T, error).
		vt, e := validateReturnSignature(returns)
		return vt, true, false, e
	}
}

// validateParamTypes checks that all parameter types have Starlark mappings.
// Parameters with StructType set are skipped — they were expanded from a struct
// and their individual fields have already been validated.
// Parameters with Callable set are skipped — they use a generated bridge.
func validateParamTypes(params []paramInfo) error {
	var unmapped []string
	for _, p := range params {
		if p.StructType != "" {
			continue
		}
		if p.Callable != nil {
			continue
		}
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
	"attrNamesList":           templateFuncAttrNamesList,
	"allAttrNames":            templateFuncAllAttrNames,
	"hasExtraAttrs":           templateFuncHasExtraAttrs,
	"planUnpackArgs":          templateFuncPlanUnpackArgs,
	"planFillSlots":           templateFuncPlanFillSlots,
	"immediateUnpackArgs":     templateFuncImmediateUnpackArgs,
	"immediateProviderBody":   templateFuncImmediateProviderBody,
	"needsImport":             templateFuncNeedsImport,
	"needsThread":             templateFuncNeedsThread,
	"needsFmt":                templateFuncNeedsFmt,
	"graphReaders":            templateFuncGraphReaders,
	"dryRunFmt":               templateFuncDryRunFmt,
	"dryRunVars":              templateFuncDryRunVars,
	"dryRunChecksum":          templateFuncDryRunChecksum,
	"implArgs":                templateFuncImplArgs,
	"graphReturn":             templateFuncGraphReturn,
	"graphUndo":               templateFuncGraphUndo,
	"docComment":              templateFuncDocComment,
	"docSummary":              templateFuncDocSummary,
	"hasSlotDocs":             templateFuncHasSlotDocs,
	"slotDocs":                templateFuncSlotDocs,
	"structReconstruct":       templateFuncStructReconstruct,
	"providerFieldInit":       templateFuncProviderFieldInit,
	"providerTypePrefix":      templateFuncProviderTypePrefix,
	"hasStructParam":          templateFuncHasStructParam,
	"immediateStructCallArgs": templateFuncImmediateStructCallArgs,
	"converterFunc":           templateFuncConverterFunc,
	"needsOpImport":           templateFuncNeedsOpImport,
	"propertyAttrExpr":        templateFuncPropertyAttrExpr,
	"handleTypes":             templateFuncHandleTypes,
	// Marshaler-based template functions
	"paramNamesList":  templateFuncParamNamesList,
	"needsOverride":   templateFuncNeedsOverride,
	"overrideClosure":  templateFuncOverrideClosure,
	"providerInit":     templateFuncProviderInit,
	"hasOverrides":     templateFuncHasOverrides,
	"needsReflect":     templateFuncNeedsReflect,
}

func templateFuncAttrNamesList(methods []methodInfo) string {
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

// templateFuncAllAttrNames returns all attribute names (generated methods + extra attrs)
// as a sorted, quoted, comma-separated string. Used by receivers with companion
// query files that contribute additional attributes.
func templateFuncAllAttrNames(d *generateDescriptor) string {
	names := make([]string, 0, len(d.Methods)+len(d.ExtraAttrs))
	for _, m := range d.Methods {
		names = append(names, m.SnakeName)
	}
	names = append(names, d.ExtraAttrs...)
	sort.Strings(names)
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = `"` + n + `"`
	}
	return strings.Join(quoted, ", ")
}

// templateFuncHasExtraAttrs returns true if the descriptor has extra attribute names.
func templateFuncHasExtraAttrs(d *generateDescriptor) bool {
	return len(d.ExtraAttrs) > 0
}

func templateFuncPlanUnpackArgs(m methodInfo) string {
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

// templateFuncPlanFillSlots generates FillSlot calls for starlark-facing params only.
func templateFuncPlanFillSlots(m methodInfo) string {
	var lines []string
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if !tm.starlarkFacing {
			continue
		}
		if isContentParam(p, m) {
			continue
		}
		lines = append(lines, fmt.Sprintf("if err := op.FillSlot(node, p.graph, %q, %s); err != nil {\nreturn nil, fmt.Errorf(%q, err)\n}", p.SnakeName, p.GoName, p.SnakeName+": %w"))
	}
	return strings.Join(lines, "\n")
}

func templateFuncImmediateUnpackArgs(m methodInfo) string {
	if len(m.Params) == 0 {
		return ""
	}

	// Separate starlark-facing params into non-variadic and variadic groups.
	var nonVariadic, variadic []paramInfo
	for _, p := range m.Params {
		if p.StructType != "" {
			// Struct-expanded params are treated as individual non-variadic params.
			nonVariadic = append(nonVariadic, p)
			continue
		}
		if p.Callable != nil {
			// Callable params are starlark-facing (unpacked as starlark.Callable).
			nonVariadic = append(nonVariadic, p)
			continue
		}
		tm := typeMappings[p.GoType]
		if !tm.starlarkFacing {
			continue
		}
		if p.Variadic {
			variadic = append(variadic, p)
		} else {
			nonVariadic = append(nonVariadic, p)
		}
	}

	if len(nonVariadic) == 0 && len(variadic) == 0 {
		return ""
	}

	// All starlark-facing params are variadic — collect from args directly.
	if len(nonVariadic) == 0 && len(variadic) > 0 {
		var buf strings.Builder
		for _, p := range variadic {
			buf.WriteString(fmt.Sprintf("var %s []string\nfor _, arg := range args {\ns, ok := starlark.AsString(arg)\nif !ok {\nreturn nil, fmt.Errorf(%q, arg.Type())\n}\n%s = append(%s, s)\n}",
				p.GoName, m.SnakeName+": expected string argument, got %s", p.GoName, p.GoName))
		}
		return buf.String()
	}

	// Standard UnpackArgs for non-variadic params.
	// Optional params get default-value declarations and "?" suffix in UnpackArgs.
	var decls []string
	var pairs []string
	for _, p := range nonVariadic {
		unpackType := ""
		if p.Callable != nil {
			unpackType = "starlark.Callable"
		} else if p.StructType != "" {
			// Struct-expanded params: infer type from GoType directly.
			if ut, ok := typeMappings[p.GoType]; ok {
				unpackType = ut.unpackType
			} else {
				unpackType = p.GoType
			}
		} else {
			unpackType = typeMappings[p.GoType].unpackType
		}

		if p.Optional && p.Default != "" {
			decls = append(decls, fmt.Sprintf("%s := %s", p.GoName, p.Default))
		} else if p.Optional {
			decls = append(decls, fmt.Sprintf("var %s %s", p.GoName, unpackType))
		} else {
			decls = append(decls, fmt.Sprintf("var %s %s", p.GoName, unpackType))
		}

		name := p.SnakeName
		if p.Optional {
			name += "?"
		}
		pairs = append(pairs, fmt.Sprintf(`"%s", &%s`, name, p.GoName))
	}
	var buf strings.Builder
	for _, d := range decls {
		buf.WriteString(d + "\n")
	}
	buf.WriteString(fmt.Sprintf("if err := starlark.UnpackArgs(%q, args, kwargs, %s); err != nil {\nreturn nil, err\n}", m.SnakeName, strings.Join(pairs, ", ")))
	return buf.String()
}

// templateFuncImmediateProviderBody generates the Provider delegation call body for an
// immediate receiver method. It maps parameters from their Starlark-unpacked
// types to Provider method arguments, calls r.provider.GoName(...), and converts
// the return value to a Starlark value. Compensation state is ignored —
// immediate receivers discard undo state.
func templateFuncImmediateProviderBody(m methodInfo) string {
	// Build conversion declarations and call args.
	// Some types require multi-return conversion (e.g., op.StarlarkDictToMap)
	// which must be pre-computed as variable declarations.
	hasStruct := templateFuncHasStructParam(m)

	var convDecls []string
	var callArgs []string
	seenStructs := map[string]bool{}
	for _, p := range m.Params {
		if hasStruct && p.StructType != "" {
			// Struct-expanded param: emit the struct variable once as the call arg.
			if !seenStructs[p.StructType] {
				seenStructs[p.StructType] = true
				baseName := p.StructType
				if idx := strings.LastIndex(baseName, "."); idx >= 0 {
					baseName = baseName[idx+1:]
				}
				varName := strings.ToLower(baseName[:1]) + baseName[1:]
				callArgs = append(callArgs, varName)
			}
			continue
		}
		decl, arg := immediateArgExpr(p)
		if decl != "" {
			convDecls = append(convDecls, decl)
		}
		callArgs = append(callArgs, arg)
	}

	var buf strings.Builder
	for _, d := range convDecls {
		buf.WriteString(d + "\n")
	}

	// Emit struct reconstruction for struct-expanded params.
	if hasStruct {
		buf.WriteString(templateFuncStructReconstruct(m))
	}

	call := fmt.Sprintf("p.%s(%s)", m.GoName, strings.Join(callArgs, ", "))

	// Determine the result conversion expression.
	resultConv := func(varName string) string {
		if m.ResultExpr != "" {
			return strings.ReplaceAll(m.ResultExpr, "%s", varName)
		}
		return immediateResultExpr(m.ReturnType, varName)
	}

	// Check whether the result conversion actually references the variable.
	// If not (e.g., starlark.None for unmapped types), use _ to avoid unused-variable errors.
	convExpr := resultConv("result")
	usesResult := strings.Contains(convExpr, "result")

	// Non-error returns (bare type or void) — immediate receivers only.
	if !m.HasError {
		if m.ReturnType == "" {
			buf.WriteString(fmt.Sprintf("\t%s\n\treturn starlark.None, nil", call))
		} else if usesResult {
			buf.WriteString(fmt.Sprintf("\tresult := %s\n\treturn %s, nil", call, convExpr))
		} else {
			buf.WriteString(fmt.Sprintf("\t_ = %s\n\treturn %s, nil", call, convExpr))
		}
		return buf.String()
	}

	if m.Compensable {
		if m.ReturnType == "" || !usesResult {
			buf.WriteString(fmt.Sprintf("\t_, _, err := %s\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn %s, nil", call, convExpr))
		} else {
			buf.WriteString(fmt.Sprintf("\tresult, _, err := %s\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn %s, nil", call, convExpr))
		}
		return buf.String()
	}

	if m.ReturnType == "" {
		buf.WriteString(fmt.Sprintf("\tif err := %s; err != nil {\n\t\treturn nil, err\n\t}\n\treturn starlark.None, nil", call))
	} else if usesResult {
		buf.WriteString(fmt.Sprintf("\tresult, err := %s\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn %s, nil", call, convExpr))
	} else {
		buf.WriteString(fmt.Sprintf("\t_, err := %s\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn %s, nil", call, convExpr))
	}
	return buf.String()
}

// immediateArgExpr returns a conversion declaration (if needed) and the Go
// expression for passing a parameter to a Provider method from an immediate
// receiver. Multi-return conversions (like op.StarlarkDictToMap) produce a
// declaration string; single-value conversions return only the inline expression.
func immediateArgExpr(p paramInfo) (decl, arg string) {
	if p.Callable != nil {
		return callableBridgeExpr(p)
	}
	tm := typeMappings[p.GoType]
	if tm.contextReader != "" {
		return "", "p.Writer"
	}
	if p.Variadic {
		return "", p.GoName + "..."
	}
	if !tm.starlarkFacing {
		// Engine-injected dependency (callbacks) — not available from Starlark.
		// Pass nil; wired via Provider struct fields when needed.
		return "", "nil"
	}
	switch p.GoType {
	case "os.FileMode":
		return "", fmt.Sprintf("os.FileMode(%s)", p.GoName)
	case "[]string":
		return "", fmt.Sprintf("op.ListToStringSlice(%s)", p.GoName)
	case "map[string]any":
		convVar := p.GoName + "Map"
		d := fmt.Sprintf("\t%s, err := op.StarlarkDictToMap(%s)\n\tif err != nil {\n\t\treturn nil, err\n\t}", convVar, p.GoName)
		return d, convVar
	default:
		return "", p.GoName
	}
}

// callableBridgeExpr generates a Go closure that bridges a starlark.Callable to the
// callable's Go function type. Each parameter is converted based on its role:
//   - projected: Go → Starlark (positional arg)
//   - handle: Go → generated handle wrapper (positional arg)
//   - pass_through: Go any → starlark.Value keyword arg (only when non-nil)
//   - swallowed: not passed to Starlark (captured from outer scope)
func callableBridgeExpr(p paramInfo) (decl, arg string) {
	c := p.Callable
	bridgeVar := p.GoName + "Go"

	// Build Go function signature params.
	var sigParams []string
	for _, cp := range c.Params {
		sigParams = append(sigParams, fmt.Sprintf("%s %s", cp.GoName, cp.GoType))
	}

	// Build positional and keyword arg conversions.
	var positionalArgs []string
	var kwargLines []string
	for _, cp := range c.Params {
		switch cp.Role {
		case "projected":
			positionalArgs = append(positionalArgs, goToStarlarkExpr(cp.GoName, cp.GoType))
		case "handle":
			for _, ht := range c.HandleTypes {
				if cp.GoType == ht.GoType {
					positionalArgs = append(positionalArgs, fmt.Sprintf("New%s(%s)", ht.HandleName, cp.GoName))
					break
				}
			}
		case "pass_through":
			kwargLines = append(kwargLines, fmt.Sprintf(
				"\t\tif %s != nil {\n\t\t\tkwargs = append(kwargs, starlark.Tuple{starlark.String(%q), %s.(starlark.Value)})\n\t\t}",
				cp.GoName, cp.GoName, cp.GoName))
		case "swallowed":
			// Not passed to Starlark.
		}
	}

	// Build the bridge function body.
	var body strings.Builder

	// Positional args.
	body.WriteString(fmt.Sprintf("\t\targs := starlark.Tuple{%s}\n", strings.Join(positionalArgs, ", ")))

	// Keyword args.
	kwargsArg := "nil"
	if len(kwargLines) > 0 {
		kwargsArg = "kwargs"
		body.WriteString("\t\tvar kwargs []starlark.Tuple\n")
		for _, kl := range kwargLines {
			body.WriteString(kl + "\n")
		}
	}

	// Call and return.
	if c.Returns == "error" {
		body.WriteString(fmt.Sprintf("\t\t_, err := starlark.Call(thread, %s, args, %s)\n\t\treturn err\n", p.GoName, kwargsArg))
	} else {
		body.WriteString(fmt.Sprintf("\t\tret, err := starlark.Call(thread, %s, args, %s)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\treturn ret, nil\n", p.GoName, kwargsArg))
	}

	d := fmt.Sprintf("\t%s := %s(func(%s) %s {\n%s\t})",
		bridgeVar, c.TypeName, strings.Join(sigParams, ", "), c.Returns, body.String())

	return d, bridgeVar
}

// goToStarlarkExpr returns the Go expression for converting a Go value to a Starlark value.
func goToStarlarkExpr(varName, goType string) string {
	switch goType {
	case "string":
		return fmt.Sprintf("starlark.String(%s)", varName)
	case "bool":
		return fmt.Sprintf("starlark.Bool(%s)", varName)
	case "int":
		return fmt.Sprintf("starlark.MakeInt(%s)", varName)
	case "int64":
		return fmt.Sprintf("starlark.MakeInt64(%s)", varName)
	default:
		return varName
	}
}

// immediateResultExpr returns the Go expression for converting an immediate
// receiver's Provider return value to a Starlark value.
//
// For pointer types (*T) not in the primitive mapping, returns New<T>Value(result),
// following the convention that custom return types have a corresponding HasAttrs
// wrapper constructor.
func immediateResultExpr(goType, varName string) string {
	switch goType {
	case "string":
		return fmt.Sprintf("starlark.String(%s)", varName)
	case "bool":
		return fmt.Sprintf("starlark.Bool(%s)", varName)
	case "int":
		return fmt.Sprintf("starlark.MakeInt(%s)", varName)
	case "int64":
		return fmt.Sprintf("starlark.MakeInt64(%s)", varName)
	case "[]byte":
		return fmt.Sprintf("starlark.Bytes(%s)", varName)
	case "[]string":
		return fmt.Sprintf("op.StringSliceToList(%s)", varName)
	case "any":
		// The any carries a starlark.Value through (e.g., accumulator pattern).
		// If nil, return None; otherwise assert to starlark.Value.
		return fmt.Sprintf("op.AnyToStarlarkValue(%s)", varName)
	default:
		// Custom pointer types: *Sources → NewSourcesValue(result)
		if strings.HasPrefix(goType, "*") {
			typeName := goType[1:]
			return fmt.Sprintf("New%sValue(%s)", typeName, varName)
		}
		return "starlark.None"
	}
}

// templateFuncNeedsImport checks whether any method parameter or callable handle
// type uses the given Go type. Used in templates for conditional imports.
func templateFuncNeedsImport(methods []methodInfo, goType string) bool {
	pkg := goType
	if idx := strings.Index(pkg, "."); idx >= 0 {
		pkg = pkg[:idx]
	}
	for _, m := range methods {
		for _, p := range m.Params {
			if p.GoType == goType {
				return true
			}
			// Check callable handle types for the same package.
			if p.Callable != nil {
				for _, ht := range p.Callable.HandleTypes {
					if strings.HasPrefix(ht.GoType, pkg+".") {
						return true
					}
				}
			}
		}
	}
	return false
}

// templateFuncNeedsThread returns true if any method parameter uses a callable bridge,
// which requires the *starlark.Thread for starlark.Call().
func templateFuncNeedsThread(m methodInfo) bool {
	for _, p := range m.Params {
		if p.Callable != nil {
			return true
		}
	}
	return false
}

// templateFuncNeedsFmt returns true if any method has variadic starlark-facing params
// or callable params with handle types (which generate fmt.Errorf in Hash()).
func templateFuncNeedsFmt(methods []methodInfo) bool {
	for _, m := range methods {
		for _, p := range m.Params {
			if p.Variadic && typeMappings[p.GoType].starlarkFacing {
				return true
			}
			if p.Callable != nil && len(p.Callable.HandleTypes) > 0 {
				return true
			}
		}
	}
	return false
}

// templateFuncGraphReaders generates variable declarations for Do: slot reads,
// context reads, and engine-injected reads. Content params use optional
// assertion (_, ok pattern) since they may arrive via promise slots.
// Params with needsConstruct are read as their raw slot type here (e.g., string);
// the actual construction (op.Construct) happens in graphReturn, after dry-run.
func templateFuncGraphReaders(m methodInfo) string {
	var slotLines, contextLines, engineLines []string

	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if tm.contextReader != "" {
			contextLines = append(contextLines, fmt.Sprintf("%s := %s", p.GoName, tm.contextReader))
		} else if isContentParam(p, m) {
			// Content params use optional assertion (may come via promise slot)
			slotLines = append(slotLines, fmt.Sprintf("%s, _ := slots[\"%s\"].([]byte)", p.GoName, p.SnakeName))
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

	return strings.Join(lines, "\n")
}

// templateFuncDryRunFmt generates format verbs for starlark-facing params only.
func templateFuncDryRunFmt(m methodInfo) string {
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

// templateFuncDryRunVars generates variable names for starlark-facing params only.
func templateFuncDryRunVars(m methodInfo) string {
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

// templateFuncDryRunChecksum generates additional dry-run output for consumer content model.
// Previously emitted ctx.TargetChecksum; now a no-op (checksums removed from Context).
func templateFuncDryRunChecksum(m methodInfo) string {
	return ""
}

// templateFuncImplArgs generates all param names in order for the delegation call.
// For params with needsConstruct, uses the constructed variable name (GoName + "Val").
func templateFuncImplArgs(m methodInfo) string {
	names := make([]string, len(m.Params))
	for i, p := range m.Params {
		tm := typeMappings[p.GoType]
		if tm.needsConstruct {
			names[i] = p.GoName + "Val"
		} else {
			names[i] = p.GoName
		}
	}
	return strings.Join(names, ", ")
}

// graphConstructPrefix generates op.Construct calls for params with needsConstruct.
// These are emitted between the dry-run check and the delegation call, so dry-run
// prints the raw slot values while the real path constructs the Go types.
func graphConstructPrefix(m methodInfo) string {
	var lines []string
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if tm.needsConstruct {
			lines = append(lines, fmt.Sprintf("%sVal, err := op.Construct[provider.%s](%s)", p.GoName, tm.constructType, p.GoName))
			lines = append(lines, "if err != nil {\nreturn nil, nil, err\n}")
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n" + strings.Join(lines, "\n")
}

// templateFuncGraphReturn generates the delegation call and return handling per content model.
// Returns (Result, UndoState, error) — three values.
func templateFuncGraphReturn(m methodInfo, implType string) string {
	if implType == "" {
		return "\nreturn nil, nil, nil"
	}

	prefix := graphConstructPrefix(m)
	argStr := templateFuncImplArgs(m)
	call := fmt.Sprintf("o.Impl.%s(%s)", m.GoName, argStr)

	if m.Compensable {
		return prefix + templateFuncGraphReturnCompensable(m, call)
	}

	switch m.ContentModel {
	case "consumer":
		// Consumer returns result (e.g., checksum), no undo state
		return prefix + fmt.Sprintf("\nresult, err := %s\nreturn result, nil, err", call)
	case "transformer":
		// Transformer returns transformed content as Result
		return prefix + fmt.Sprintf("\nresult, err := %s\nif err != nil {\nreturn nil, nil, err\n}\nreturn result, nil, nil", call)
	default: // "none"
		if m.ReturnType == "" {
			// Error-only return
			return prefix + fmt.Sprintf("\nreturn nil, nil, %s", call)
		}
		// Has value return — pass through as Result
		return prefix + fmt.Sprintf("\nresult, err := %s\nreturn result, nil, err", call)
	}
}

// templateFuncGraphReturnCompensable generates the delegation call for compensable methods.
// Compensable methods return (U, error) or (T, U, error) where U becomes UndoState.
func templateFuncGraphReturnCompensable(m methodInfo, call string) string {
	switch m.ContentModel {
	case "consumer":
		// (string, U, error) — result + state; result flows to downstream nodes
		return fmt.Sprintf("\nresult, state, err := %s\nif err != nil {\nreturn nil, nil, err\n}\nreturn result, state, nil", call)
	case "transformer":
		// ([]byte, U, error) — result + state
		return fmt.Sprintf("\nresult, state, err := %s\nif err != nil {\nreturn nil, nil, err\n}\nreturn result, state, nil", call)
	default: // "none"
		if m.ReturnType == "" {
			// (U, error) — state only
			return fmt.Sprintf("\nstate, err := %s\nreturn nil, state, err", call)
		}
		// (T, U, error) — value + state, return value as Result
		return fmt.Sprintf("\nresult, state, err := %s\nreturn result, state, err", call)
	}
}


// templateFuncGraphUndo generates the Undo method for an action. Compensable actions
// delegate to Impl.Compensate<GoName>(state). The state argument is typed according
// to U from the method's (T, U, error) return signature — if U is not "any", the
// generated code includes a type assertion.
func templateFuncGraphUndo(m methodInfo) string {
	if !m.Compensable {
		return "" // No Undo method — struct implements Action only, not Undoable.
	}
	stateExpr := "state"
	if m.CompStateType != "" && m.CompStateType != "any" {
		stateExpr = fmt.Sprintf("state.(%s)", m.CompStateType)
	}
	return fmt.Sprintf("func (o *%s) Undo(_ *op.Context, state op.UndoState) error {\n\tif state == nil {\n\t\treturn nil\n\t}\n\treturn o.Impl.Compensate%s(%s)\n}", m.GoName, m.GoName, stateExpr)
}

// templateFuncDocComment renders a multi-line Go doc comment. The first line is prefixed
// with "// snakeName ", subsequent lines get "// " (or "//" for blank lines).
func templateFuncDocComment(snakeName, doc string) string {
	if doc == "" {
		return "// " + snakeName
	}
	lines := strings.Split(strings.TrimRight(doc, "\n"), "\n")
	var result []string
	for i, line := range lines {
		if i == 0 {
			result = append(result, "// "+snakeName+" "+line)
		} else if line == "" {
			result = append(result, "//")
		} else {
			result = append(result, "// "+line)
		}
	}
	return strings.Join(result, "\n")
}

// templateFuncDocSummary returns the description portion of a doc string — text before
// the first blank line or structured section (Slots:, Parameters:, Usage:, Returns:).
func templateFuncDocSummary(doc string) string {
	if doc == "" {
		return ""
	}
	lines := strings.Split(doc, "\n")
	var descLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		if strings.HasPrefix(trimmed, "Slots:") ||
			strings.HasPrefix(trimmed, "Parameters:") ||
			strings.HasPrefix(trimmed, "Usage:") ||
			strings.HasPrefix(trimmed, "Returns:") {
			break
		}
		descLines = append(descLines, trimmed)
	}
	return strings.Join(descLines, " ")
}

// templateFuncHasSlotDocs returns true if any starlark-facing parameter has documentation.
func templateFuncHasSlotDocs(m methodInfo) bool {
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if tm.starlarkFacing && p.Doc != "" && !isContentParam(p, m) {
			return true
		}
	}
	return false
}

// templateFuncSlotDocs generates a "// Slots:" comment block from structured parameter docs.
// Returns empty string if no starlark-facing params have docs.
func templateFuncSlotDocs(m methodInfo) string {
	var entries []string
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if !tm.starlarkFacing || p.Doc == "" || isContentParam(p, m) {
			continue
		}
		entries = append(entries, fmt.Sprintf("//   - %s: %s", p.SnakeName, p.Doc))
	}
	if len(entries) == 0 {
		return ""
	}
	return "\n//\n// Slots:\n" + strings.Join(entries, "\n")
}

// templateFuncStructReconstruct generates a Go struct literal construction from
// individually-expanded kwargs parameters. When a method's original Go signature
// takes a struct param, the Starlark interface expands its fields to individual
// kwargs. This function reassembles the struct for the provider call.
func templateFuncStructReconstruct(m methodInfo) string {
	// Group params by StructType
	structGroups := map[string][]paramInfo{}
	var structOrder []string
	for _, p := range m.Params {
		if p.StructType == "" {
			continue
		}
		if _, seen := structGroups[p.StructType]; !seen {
			structOrder = append(structOrder, p.StructType)
		}
		structGroups[p.StructType] = append(structGroups[p.StructType], p)
	}
	if len(structGroups) == 0 {
		return ""
	}

	var buf strings.Builder
	for _, st := range structOrder {
		params := structGroups[st]
		// Strip package prefix (e.g., "provider.AnalysisConfig" → "AnalysisConfig") for variable name.
		baseName := st
		if idx := strings.LastIndex(st, "."); idx >= 0 {
			baseName = st[idx+1:]
		}
		varName := strings.ToLower(baseName[:1]) + baseName[1:]
		buf.WriteString(fmt.Sprintf("\t%s := %s{\n", varName, st))
		for _, p := range params {
			// Use the original Go field name (title case) for the struct literal.
			fieldName := strings.ToUpper(p.GoName[:1]) + p.GoName[1:]
			buf.WriteString(fmt.Sprintf("\t\t%s: %s,\n", fieldName, p.GoName))
		}
		buf.WriteString("\t}\n")
	}
	return buf.String()
}

// templateFuncProviderFieldInit generates the Provider struct construction for
// ImmediateFactory from ProviderFields. Replaces the hard-coded Writer/ProgramName/Color.
func templateFuncProviderFieldInit(d *generateDescriptor) string {
	if len(d.ProviderFields) == 0 {
		return ""
	}

	prefix := templateFuncProviderTypePrefix(d)
	var buf strings.Builder
	for _, pf := range d.ProviderFields {
		buf.WriteString(fmt.Sprintf("\t\t\t%s := cfg.%s\n", strings.ToLower(pf.GoName[:1])+pf.GoName[1:], pf.CfgField))
		if pf.Default != "" {
			buf.WriteString(fmt.Sprintf("\t\t\tif %s == %s {\n\t\t\t\t%s = %s\n\t\t\t}\n",
				strings.ToLower(pf.GoName[:1])+pf.GoName[1:], pf.ZeroValue,
				strings.ToLower(pf.GoName[:1])+pf.GoName[1:], pf.Default))
		}
	}
	buf.WriteString(fmt.Sprintf("\t\t\treturn New%s%s(&%sProvider{", d.StructName, d.WrapperSuffix, prefix))
	for i, pf := range d.ProviderFields {
		if i > 0 {
			buf.WriteString(", ")
		}
		localVar := strings.ToLower(pf.GoName[:1]) + pf.GoName[1:]
		buf.WriteString(fmt.Sprintf("%s: %s", pf.GoName, localVar))
	}
	buf.WriteString("})")
	return buf.String()
}

// templateFuncProviderTypePrefix returns "provider." when generating into a gen/ subpackage,
// or "" when generating in the same package as the provider.
func templateFuncProviderTypePrefix(d *generateDescriptor) string {
	if d.ProviderImport != "" {
		return "provider."
	}
	return ""
}

// templateFuncPropertyAttrExpr generates the Attr() case body for a property method.
// Property methods are exposed as direct values (not callable functions):
//
//	case "paths":
//	    return op.StringSliceToList(p.Paths()), nil
//
// The expression converts the Go return type to a Starlark value using the same
// type mappings as immediateResultExpr.
func templateFuncPropertyAttrExpr(m methodInfo) string {
	call := fmt.Sprintf("p.%s()", m.GoName)
	if m.ResultExpr != "" {
		return strings.ReplaceAll(m.ResultExpr, "%s", call)
	}
	return immediateResultExpr(m.ReturnType, call)
}

// templateFuncHasStructParam returns true if the method has any struct-expanded params.
func templateFuncHasStructParam(m methodInfo) bool {
	for _, p := range m.Params {
		if p.StructType != "" {
			return true
		}
	}
	return false
}

// templateFuncImmediateStructCallArgs generates the call arguments for a provider method
// that includes struct-expanded params. Non-struct params use their variable names directly;
// struct params use the reconstructed struct variable.
func templateFuncImmediateStructCallArgs(m methodInfo) string {
	// Find the original params (before expansion) — each unique StructType
	// becomes one call arg using the reconstructed local variable.
	var args []string
	seenStructs := map[string]bool{}
	for _, p := range m.Params {
		if p.StructType != "" {
			if !seenStructs[p.StructType] {
				seenStructs[p.StructType] = true
				baseName := p.StructType
				if idx := strings.LastIndex(p.StructType, "."); idx >= 0 {
					baseName = p.StructType[idx+1:]
				}
				varName := strings.ToLower(baseName[:1]) + baseName[1:]
				args = append(args, varName)
			}
		} else {
			decl, arg := immediateArgExpr(p)
			_ = decl // any needed decl is handled by immediateProviderBody
			args = append(args, arg)
		}
	}
	return strings.Join(args, ", ")
}

// =============================================================================
// HANDLE TYPE GENERATION
// =============================================================================

// templateFuncHandleTypes collects unique handle types across all methods and generates
// Starlark HasAttrs wrapper code for each. Handle types appear in callable parameters
// and expose a subset of the Go interface's methods as Starlark attributes.
func templateFuncHandleTypes(methods []methodInfo) string {
	// Collect unique handle types across all methods.
	seen := map[string]bool{}
	var handles []handleType
	for _, m := range methods {
		for _, p := range m.Params {
			if p.Callable == nil {
				continue
			}
			for _, ht := range p.Callable.HandleTypes {
				if seen[ht.HandleName] {
					continue
				}
				seen[ht.HandleName] = true
				handles = append(handles, ht)
			}
		}
	}
	if len(handles) == 0 {
		return ""
	}

	var buf strings.Builder
	for _, ht := range handles {
		fieldName := strings.ToLower(ht.HandleName[:1]) + ht.HandleName[1:]
		// Remove "Handle" suffix for the field name if present.
		if strings.HasSuffix(fieldName, "Handle") {
			fieldName = fieldName[:len(fieldName)-6]
		}
		snakeName := camelToSnake(strings.TrimSuffix(ht.HandleName, "Handle"))

		// Struct definition
		buf.WriteString(fmt.Sprintf("\n// %s wraps a %s for Starlark consumption.\ntype %s struct {\n\t%s %s\n}\n\n",
			ht.HandleName, ht.GoType, ht.HandleName, fieldName, ht.GoType))

		// Constructor
		buf.WriteString(fmt.Sprintf("// New%s creates a new handle wrapper.\nfunc New%s(v %s) *%s { return &%s{%s: v} }\n\n",
			ht.HandleName, ht.HandleName, ht.GoType, ht.HandleName, ht.HandleName, fieldName))

		// Starlark Value interface
		buf.WriteString(fmt.Sprintf("func (h *%s) String() string        { return h.%s.Name() }\n",
			ht.HandleName, fieldName))
		buf.WriteString(fmt.Sprintf("func (h *%s) Type() string          { return %q }\n",
			ht.HandleName, snakeName))
		buf.WriteString(fmt.Sprintf("func (h *%s) Freeze()               {}\n", ht.HandleName))
		buf.WriteString(fmt.Sprintf("func (h *%s) Truth() starlark.Bool  { return true }\n",
			ht.HandleName))
		buf.WriteString(fmt.Sprintf("func (h *%s) Hash() (uint32, error) { return 0, fmt.Errorf(\"unhashable: %s\") }\n\n",
			ht.HandleName, snakeName))

		// Attr method
		buf.WriteString(fmt.Sprintf("func (h *%s) Attr(name string) (starlark.Value, error) {\n\tswitch name {\n",
			ht.HandleName))
		for _, hm := range ht.Methods {
			conv := handleMethodReturnExpr(fmt.Sprintf("h.%s.%s()", fieldName, hm.GoName), hm.ReturnType)
			buf.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn %s, nil\n", hm.SnakeName, conv))
		}
		buf.WriteString(fmt.Sprintf("\tdefault:\n\t\treturn nil, op.NoSuchAttrError(%q, name)\n\t}\n}\n\n", snakeName))

		// AttrNames method
		var attrNames []string
		for _, hm := range ht.Methods {
			attrNames = append(attrNames, fmt.Sprintf("%q", hm.SnakeName))
		}
		sort.Strings(attrNames)
		buf.WriteString(fmt.Sprintf("func (h *%s) AttrNames() []string {\n\treturn []string{%s}\n}\n",
			ht.HandleName, strings.Join(attrNames, ", ")))
	}
	return buf.String()
}

// handleMethodReturnExpr converts a Go method call expression to its Starlark value.
func handleMethodReturnExpr(callExpr, returnType string) string {
	switch returnType {
	case "string":
		return fmt.Sprintf("starlark.String(%s)", callExpr)
	case "bool":
		return fmt.Sprintf("starlark.Bool(%s)", callExpr)
	case "int":
		return fmt.Sprintf("starlark.MakeInt(%s)", callExpr)
	case "int64":
		return fmt.Sprintf("starlark.MakeInt64(%s)", callExpr)
	default:
		return callExpr
	}
}

// =============================================================================
// MARSHALER-BASED TEMPLATE FUNCTIONS
// =============================================================================

// templateFuncParamNamesList generates the quoted, comma-separated parameter
// name list for a single method's MethodParams entry. Only starlark-facing
// params are included; engine-injected and callable params are excluded.
// Optional params (with Default or marked Optional) get a "?" suffix.
func templateFuncParamNamesList(m methodInfo) string {
	var names []string
	for _, p := range m.Params {
		if p.Callable != nil {
			continue
		}
		tm := typeMappings[p.GoType]
		if tm.contextReader != "" {
			continue
		}
		if !tm.starlarkFacing && p.GoType != "[]byte" {
			continue
		}
		name := `"` + p.SnakeName
		if p.Optional || p.Default != "" {
			name += "?"
		}
		name += `"`
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// templateFuncNeedsOverride returns true if a method needs an Override()
// call instead of WrapReceiver's auto-bridging. Override is required when
// the method has callable params, variadic params, engine-injected params,
// struct_param expansion, non-zero defaults, or a custom ResultExpr.
func templateFuncNeedsOverride(m methodInfo) bool {
	for _, p := range m.Params {
		if p.Callable != nil {
			return true
		}
		if p.Variadic {
			return true
		}
		tm := typeMappings[p.GoType]
		if tm.contextReader != "" {
			return true
		}
		if !tm.starlarkFacing && p.GoType != "[]byte" {
			return true
		}
		if p.StructType != "" {
			return true
		}
		if p.Default != "" {
			return true
		}
	}
	if m.ResultExpr != "" {
		return true
	}
	if m.Property {
		return true
	}
	return false
}

// templateFuncOverrideClosure generates the Override closure body for a method
// that needs custom bridging. Reuses immediateUnpackArgs and immediateProviderBody.
func templateFuncOverrideClosure(m methodInfo) string {
	var buf strings.Builder
	// Use named "thread" parameter when the method has callable params that need starlark.Call().
	threadParam := "_ *starlark.Thread"
	if templateFuncNeedsThread(m) {
		threadParam = "thread *starlark.Thread"
	}
	buf.WriteString(fmt.Sprintf("\tr.Override(%q, func(%s, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {\n", m.SnakeName, threadParam))

	unpack := templateFuncImmediateUnpackArgs(m)
	if unpack != "" {
		// Indent the unpack block for nesting inside the closure.
		for _, line := range strings.Split(unpack, "\n") {
			buf.WriteString("\t" + line + "\n")
		}
	}

	body := templateFuncImmediateProviderBody(m)
	if body != "" {
		for _, line := range strings.Split(body, "\n") {
			buf.WriteString("\t" + line + "\n")
		}
	}

	buf.WriteString("\t})")
	return buf.String()
}

// templateFuncProviderInit generates the ImmediateFactory body that constructs
// the provider and delegates to New<StructName>Receiver. For providers with
// ProviderFields (bind directives), fields are read from BindingConfig first.
func templateFuncProviderInit(d *generateDescriptor) string {
	prefix := templateFuncProviderTypePrefix(d)
	var buf strings.Builder

	if len(d.ProviderFields) > 0 {
		for _, pf := range d.ProviderFields {
			localVar := strings.ToLower(pf.GoName[:1]) + pf.GoName[1:]
			buf.WriteString(fmt.Sprintf("\t\t\t%s := cfg.%s\n", localVar, pf.CfgField))
			if pf.Default != "" {
				buf.WriteString(fmt.Sprintf("\t\t\tif %s == %s {\n\t\t\t\t%s = %s\n\t\t\t}\n",
					localVar, pf.ZeroValue, localVar, pf.Default))
			}
		}
		buf.WriteString(fmt.Sprintf("\t\t\treturn New%s%s(&%sProvider{", d.StructName, d.WrapperSuffix, prefix))
		for i, pf := range d.ProviderFields {
			if i > 0 {
				buf.WriteString(", ")
			}
			localVar := strings.ToLower(pf.GoName[:1]) + pf.GoName[1:]
			buf.WriteString(fmt.Sprintf("%s: %s", pf.GoName, localVar))
		}
		buf.WriteString("})")
	} else {
		buf.WriteString(fmt.Sprintf("\t\t\treturn New%s%s(&%sProvider{})", d.StructName, d.WrapperSuffix, prefix))
	}

	return buf.String()
}

// templateFuncHasOverrides returns true if any method in the descriptor needs Override.
func templateFuncHasOverrides(d *generateDescriptor) bool {
	for _, m := range d.Methods {
		if templateFuncNeedsOverride(m) {
			return true
		}
	}
	return false
}

// templateFuncNeedsReflect returns true if the descriptor uses WrapPlanned
// (which requires reflect.TypeOf).
func templateFuncNeedsReflect(_ *generateDescriptor) bool {
	// Planned templates always need reflect for reflect.TypeOf.
	return true
}

// =============================================================================
// STRUCT CONVERTER GENERATION
// =============================================================================

// templateFuncConverterFunc generates a complete struct-to-Starlark converter function.
func templateFuncConverterFunc(c converterInfo, prefix string) string {
	var b strings.Builder

	// Receiver type
	recvType := prefix + c.GoType
	if c.IsPointer {
		recvType = "*" + recvType
	}

	fmt.Fprintf(&b, "// %s converts a %s to a Starlark struct.\n", c.FuncName, c.GoType)
	fmt.Fprintf(&b, "func %s(v %s) *starlarkstruct.Struct {\n", c.FuncName, recvType)

	// Determine if we need a dynamic dict (for nullable/conditional fields)
	hasDynamic := converterHasDynamic(c.Fields)

	// Pre-loops for non-nullable struct_slice fields
	for _, f := range c.Fields {
		if f.Kind == "struct_slice" && !f.Nullable {
			converterPreLoop(&b, "v", f, 1)
		}
	}

	if hasDynamic {
		// Dynamic dict: static fields first, then conditional assignments
		fmt.Fprintf(&b, "\td := starlark.StringDict{\n")
		for _, f := range c.Fields {
			if f.Nullable || f.Kind == "conditional" {
				continue
			}
			fmt.Fprintf(&b, "\t\t\"%s\": %s,\n", f.SnakeName, converterFieldExpr("v", f))
		}
		fmt.Fprintf(&b, "\t}\n")

		for _, f := range c.Fields {
			if f.Nullable {
				converterNullableAssign(&b, "v", f, 1)
			} else if f.Kind == "conditional" {
				converterConditionalAssign(&b, "v", f, 1)
			}
		}

		fmt.Fprintf(&b, "\treturn starlarkstruct.FromStringDict(starlark.String(\"%s\"), d)\n", c.StarlarkName)
	} else {
		// All static: return directly
		fmt.Fprintf(&b, "\treturn starlarkstruct.FromStringDict(starlark.String(\"%s\"), starlark.StringDict{\n", c.StarlarkName)
		for _, f := range c.Fields {
			fmt.Fprintf(&b, "\t\t\"%s\": %s,\n", f.SnakeName, converterFieldExpr("v", f))
		}
		fmt.Fprintf(&b, "\t})\n")
	}

	fmt.Fprintf(&b, "}")
	return b.String()
}

// converterHasDynamic checks if any fields need dynamic dict construction.
func converterHasDynamic(fields []converterFieldInfo) bool {
	for _, f := range fields {
		if f.Nullable || f.Kind == "conditional" {
			return true
		}
	}
	return false
}

// converterFieldExpr returns the Go expression for converting a field to a Starlark value.
func converterFieldExpr(varName string, f converterFieldInfo) string {
	access := varName + "." + f.GoName
	switch f.Kind {
	case "string":
		return fmt.Sprintf("starlark.String(%s)", access)
	case "int":
		return fmt.Sprintf("starlark.MakeInt(%s)", access)
	case "int64":
		return fmt.Sprintf("starlark.MakeInt64(%s)", access)
	case "bool":
		return fmt.Sprintf("starlark.Bool(%s)", access)
	case "string_slice":
		return fmt.Sprintf("op.StringSliceToList(%s)", access)
	case "struct_slice":
		return fmt.Sprintf("starlark.NewList(%sList)", converterLCFirst(f.GoName))
	case "struct_ptr":
		if f.Converter != "" {
			return fmt.Sprintf("%s(%s)", f.Converter, access)
		}
		return "starlark.None"
	case "struct_value":
		if f.Converter != "" {
			return fmt.Sprintf("%s(%s)", f.Converter, access)
		}
		return "starlark.None"
	case "inline_struct":
		return converterInlineStruct(varName, f, 2)
	default:
		return "starlark.None"
	}
}

// converterPrimitiveExpr returns the Go expression for a primitive field conversion.
func converterPrimitiveExpr(access, kind string) string {
	switch kind {
	case "string":
		return fmt.Sprintf("starlark.String(%s)", access)
	case "int":
		return fmt.Sprintf("starlark.MakeInt(%s)", access)
	case "int64":
		return fmt.Sprintf("starlark.MakeInt64(%s)", access)
	case "bool":
		return fmt.Sprintf("starlark.Bool(%s)", access)
	case "string_slice":
		return fmt.Sprintf("op.StringSliceToList(%s)", access)
	default:
		return "starlark.None"
	}
}

// converterInlineStruct generates an inline starlarkstruct.FromStringDict expression.
// Inner fields are accessed via varName.GoName (e.g., v.Totals.FileCount).
func converterInlineStruct(varName string, f converterFieldInfo, indent int) string {
	var b strings.Builder
	tabs := strings.Repeat("\t", indent)
	innerVar := varName + "." + f.GoName
	fmt.Fprintf(&b, "starlarkstruct.FromStringDict(starlark.String(\"%s\"), starlark.StringDict{\n", f.InlineName)
	for _, sf := range f.InlineFields {
		fmt.Fprintf(&b, "%s\t\"%s\": %s,\n", tabs, sf.SnakeName, converterFieldExpr(innerVar, sf))
	}
	fmt.Fprintf(&b, "%s})", tabs)
	return b.String()
}

// converterPreLoop generates the loop code for struct_slice fields.
func converterPreLoop(b *strings.Builder, varName string, f converterFieldInfo, indent int) {
	tabs := strings.Repeat("\t", indent)
	listVar := converterLCFirst(f.GoName) + "List"
	itemVar := converterLCFirst(f.GoName) + "Item"

	fmt.Fprintf(b, "%s%s := make([]starlark.Value, len(%s.%s))\n", tabs, listVar, varName, f.GoName)
	fmt.Fprintf(b, "%sfor i, %s := range %s.%s {\n", tabs, itemVar, varName, f.GoName)

	if f.Converter != "" {
		// Named converter function
		fmt.Fprintf(b, "%s\t%s[i] = %s(%s)\n", tabs, listVar, f.Converter, itemVar)
	} else if len(f.InlineFields) > 0 {
		// Inline struct conversion — may have nested pre-loops and conditional fields
		for _, sf := range f.InlineFields {
			if sf.Kind == "struct_slice" && !sf.Nullable {
				converterPreLoop(b, itemVar, sf, indent+1)
			}
		}

		hasDyn := converterHasDynamic(f.InlineFields)
		if hasDyn {
			fmt.Fprintf(b, "%s\td := starlark.StringDict{\n", tabs)
			for _, sf := range f.InlineFields {
				if sf.Nullable || sf.Kind == "conditional" {
					continue
				}
				fmt.Fprintf(b, "%s\t\t\"%s\": %s,\n", tabs, sf.SnakeName, converterFieldExpr(itemVar, sf))
			}
			fmt.Fprintf(b, "%s\t}\n", tabs)
			for _, sf := range f.InlineFields {
				if sf.Kind == "conditional" {
					converterConditionalAssign(b, itemVar, sf, indent+1)
				} else if sf.Nullable {
					converterNullableAssign(b, itemVar, sf, indent+1)
				}
			}
			fmt.Fprintf(b, "%s\t%s[i] = starlarkstruct.FromStringDict(starlark.String(\"%s\"), d)\n", tabs, listVar, f.InlineName)
		} else {
			fmt.Fprintf(b, "%s\t%s[i] = starlarkstruct.FromStringDict(starlark.String(\"%s\"), starlark.StringDict{\n", tabs, listVar, f.InlineName)
			for _, sf := range f.InlineFields {
				fmt.Fprintf(b, "%s\t\t\"%s\": %s,\n", tabs, sf.SnakeName, converterFieldExpr(itemVar, sf))
			}
			fmt.Fprintf(b, "%s\t})\n", tabs)
		}
	}

	fmt.Fprintf(b, "%s}\n", tabs)
}

// converterNullableAssign generates a nil-checked field assignment.
func converterNullableAssign(b *strings.Builder, varName string, f converterFieldInfo, indent int) {
	tabs := strings.Repeat("\t", indent)
	access := varName + "." + f.GoName
	nilExpr := f.NilExpr
	if nilExpr == "" {
		nilExpr = "starlark.None"
	}

	fmt.Fprintf(b, "%sif %s != nil {\n", tabs, access)

	if f.Kind == "struct_ptr" && f.Converter != "" {
		fmt.Fprintf(b, "%s\td[\"%s\"] = %s(%s)\n", tabs, f.SnakeName, f.Converter, access)
	} else if f.Kind == "struct_slice" {
		// Nullable slice: build list if non-nil
		listVar := converterLCFirst(f.GoName)
		itemVar := converterLCFirst(f.GoName) + "Item"
		fmt.Fprintf(b, "%s\t%s := make([]starlark.Value, len(%s))\n", tabs, listVar, access)
		fmt.Fprintf(b, "%s\tfor i, %s := range %s {\n", tabs, itemVar, access)
		if f.Converter != "" {
			fmt.Fprintf(b, "%s\t\t%s[i] = %s(%s)\n", tabs, listVar, f.Converter, itemVar)
		} else if len(f.InlineFields) > 0 {
			fmt.Fprintf(b, "%s\t\t%s[i] = starlarkstruct.FromStringDict(starlark.String(\"%s\"), starlark.StringDict{\n", tabs, listVar, f.InlineName)
			for _, sf := range f.InlineFields {
				fmt.Fprintf(b, "%s\t\t\t\"%s\": %s,\n", tabs, sf.SnakeName, converterFieldExpr(itemVar, sf))
			}
			fmt.Fprintf(b, "%s\t\t})\n", tabs)
		}
		fmt.Fprintf(b, "%s\t}\n", tabs)
		fmt.Fprintf(b, "%s\td[\"%s\"] = starlark.NewList(%s)\n", tabs, f.SnakeName, listVar)
	}

	fmt.Fprintf(b, "%s} else {\n", tabs)
	fmt.Fprintf(b, "%s\td[\"%s\"] = %s\n", tabs, f.SnakeName, nilExpr)
	fmt.Fprintf(b, "%s}\n", tabs)
}

// converterConditionalAssign generates an if-else field assignment based on a guard field.
func converterConditionalAssign(b *strings.Builder, varName string, f converterFieldInfo, indent int) {
	tabs := strings.Repeat("\t", indent)
	guard := varName + "." + f.Guard
	trueAccess := varName + "." + f.GuardField
	trueExpr := converterPrimitiveExpr(trueAccess, f.GuardKind)

	fmt.Fprintf(b, "%sif %s {\n", tabs, guard)
	fmt.Fprintf(b, "%s\td[\"%s\"] = %s\n", tabs, f.SnakeName, trueExpr)
	fmt.Fprintf(b, "%s} else {\n", tabs)
	fmt.Fprintf(b, "%s\td[\"%s\"] = starlark.None\n", tabs, f.SnakeName)
	fmt.Fprintf(b, "%s}\n", tabs)
}

// templateFuncNeedsOpImport returns true if any converter field uses op.StringSliceToList.
func templateFuncNeedsOpImport(converters []converterInfo) bool {
	for _, c := range converters {
		if converterFieldsNeedOp(c.Fields) {
			return true
		}
	}
	return false
}

func converterFieldsNeedOp(fields []converterFieldInfo) bool {
	for _, f := range fields {
		if f.Kind == "string_slice" {
			return true
		}
		if converterFieldsNeedOp(f.InlineFields) {
			return true
		}
	}
	return false
}

// converterLCFirst returns a string with the first letter lowercased.
func converterLCFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// =============================================================================
// TEMPLATES
// =============================================================================

// ImmediateReceiverTemplate is the builtin template for immediate receivers.
// Supports both same-package and gen/ subpackage generation via conditional fields:
//   - Registered: controls init()/RegisterBinding emission
//   - ProviderImport: enables provider type prefix for gen/ subpackage
//   - ProviderFields: dynamic ImmediateFactory field init
//   - ImplType: Go type being wrapped (e.g., "Provider", "Sources")
const ImmediateReceiverTemplate = `// Code generated by go.generate; DO NOT EDIT.

package {{.Package}}

import (
{{- if needsFmt .Methods}}
	"fmt"
{{- end}}
{{- if needsImport .Methods "os.FileMode"}}
	"os"
{{- end}}

	"go.starlark.net/starlark"

	"github.com/NobleFactor/devlore-cli/pkg/op"
{{- if .ProviderImport}}
	provider "{{.ProviderImport}}"
{{- end}}
)
{{if .Registered}}
func init() {
	op.RegisterBinding(&op.ProviderBinding{
		Name:     "{{.Provider}}",
		Access:   op.Access{{.AccessTitle}},
		Lifetime: op.Lifetime{{.LifetimeTitle}},
		ImmediateFactory: func(cfg op.BindingConfig) starlark.Value {
{{- if .ProviderFields}}
{{providerFieldInit .}}
{{- else}}
			return New{{.StructName}}{{.WrapperSuffix}}(&{{providerTypePrefix .}}Provider{
				Writer:      cfg.Writer,
				ProgramName: cfg.ProgramName,
				Color:       cfg.Color,
			})
{{- end}}
		},
	})
}
{{end}}
type {{.StructName}}{{.WrapperSuffix}} struct {
	op.Receiver
	provider *{{providerTypePrefix .}}{{.ImplType}}
}

func New{{.StructName}}{{.WrapperSuffix}}(p *{{providerTypePrefix .}}{{.ImplType}}) *{{.StructName}}{{.WrapperSuffix}} {
	return &{{.StructName}}{{.WrapperSuffix}}{
		Receiver: op.NewReceiver("{{.Namespace}}"),
		provider: p,
	}
}

func (r *{{.StructName}}{{.WrapperSuffix}}) Attr(name string) (starlark.Value, error) {
	switch name {
{{- range .Methods}}
{{- if .Property}}
	case "{{.SnakeName}}":
		return {{propertyAttrExpr .}}, nil
{{- else}}
	case "{{.SnakeName}}":
		return op.MakeAttr("{{$.Namespace}}.{{.SnakeName}}", r.{{.SnakeName}}), nil
{{- end}}
{{- end}}
	default:
{{- if hasExtraAttrs .}}
		return r.queryAttr(name)
{{- else}}
		return nil, op.NoSuchAttrError("{{.Namespace}}", name)
{{- end}}
	}
}

func (r *{{.StructName}}{{.WrapperSuffix}}) AttrNames() []string {
	return []string{{"{"}}{{allAttrNames .}}{{"}"}}
}
{{range .Methods}}{{if not .Property}}
func (r *{{$.StructName}}{{$.WrapperSuffix}}) {{.SnakeName}}({{if needsThread .}}thread{{else}}_{{end}} *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
{{immediateUnpackArgs .}}
{{immediateProviderBody .}}
}
{{end}}{{end}}`

// builtinTemplates maps names to content for builtin templates.
// StructConverterTemplate is the builtin template for struct-to-Starlark converters.
const StructConverterTemplate = `// Code generated by go.generate; DO NOT EDIT.

package {{.Package}}

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
{{- if needsOpImport .Converters}}

	"github.com/NobleFactor/devlore-cli/pkg/op"
{{- end}}
{{- if .ProviderImport}}

	provider "{{.ProviderImport}}"
{{- end}}
)
{{range .Converters}}
{{converterFunc . (providerTypePrefix $)}}
{{end}}`

var builtinTemplates = map[string]string{
	"immediate_receiver": ImmediateReceiverTemplate,
	"struct_converter":   StructConverterTemplate,
}

// =============================================================================
// go.generate() METHOD
// =============================================================================

// goTemplate returns builtin template content by name.
func (r *GoReceiver) goTemplate(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs("go.template", args, kwargs, "name", &name); err != nil {
		return nil, err
	}
	content, ok := builtinTemplates[name]
	if !ok {
		names := make([]string, 0, len(builtinTemplates))
		for k := range builtinTemplates {
			names = append(names, k)
		}
		return nil, fmt.Errorf("go.template: unknown builtin %q (valid: %s)", name, strings.Join(names, ", "))
	}
	return starlark.String(content), nil
}

func (r *GoReceiver) goGenerate(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var templateContent string
	var descriptorVal starlark.Value
	if err := starlark.UnpackArgs("go.generate", args, kwargs, "template", &templateContent, "descriptor", &descriptorVal); err != nil {
		return nil, err
	}

	// If the template arg is a known builtin name, resolve to content.
	// Otherwise the caller passed raw template text; check the descriptor
	// for a "template" key that carries the logical template name.
	templateName := templateContent
	if content, ok := builtinTemplates[templateContent]; ok {
		templateContent = content
	} else if descName, err := valueGetString(descriptorVal, "template"); err == nil && descName != "" {
		templateName = descName
	}

	// Parse template content
	tmpl, err := template.New("gen").Funcs(genTemplateFuncs).Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("go.generate: template parse: %w", err)
	}

	// Convert descriptor
	desc, err := descriptorFromValue(templateName, descriptorVal)
	if err != nil {
		return nil, fmt.Errorf("go.generate: %w", err)
	}

	// Gate 1: validate param types — flag unprojectable methods instead of failing.
	var projectable []methodInfo
	var flaggedMethods []starlark.Value
	for _, m := range desc.Methods {
		if err := validateParamTypes(m.Params); err != nil {
			reason := fmt.Sprintf("%s: %s", methodLocation(m), err)
			flaggedMethods = append(flaggedMethods, starlark.String(reason))
		} else {
			projectable = append(projectable, m)
		}
	}

	// Gate 1b: callable params are only supported in immediate receivers.
	// For planned/graph templates, callable params can't be serialized to slots.
	if desc.Template != "immediate_receiver" {
		var callableValid []methodInfo
		for _, m := range projectable {
			hasCallable := false
			for _, p := range m.Params {
				if p.Callable != nil {
					hasCallable = true
					break
				}
			}
			if hasCallable {
				reason := fmt.Sprintf("%s: callable parameter not supported in %s template", methodLocation(m), desc.Template)
				flaggedMethods = append(flaggedMethods, starlark.String(reason))
			} else {
				callableValid = append(callableValid, m)
			}
		}
		projectable = callableValid
	}
	desc.Methods = projectable

	// Gate 2: validate return signatures, infer compensability, and infer content models.
	// Compensability is determined by the return signature shape:
	//   (T, U, error) → compensable     (T, error) → non-compensable
	var returnValid []methodInfo
	for i, m := range desc.Methods {
		rawReturn := m.ReturnType

		var valueType string
		var hasError, compensable bool
		var err error
		if desc.Template == "immediate_receiver" || desc.Template == "params" {
			valueType, hasError, compensable, err = validateImmediateReturn(rawReturn)
		} else {
			// Planned receivers and graph actions always have error returns.
			// Try compensable (T, U, error) first; fall back to (T, error).
			if vt, ut, e := validateCompensableReturn(rawReturn); e == nil {
				valueType, compensable = vt, true
				desc.Methods[i].CompStateType = ut
			} else {
				valueType, err = validateReturnSignature(rawReturn)
			}
			hasError = true
		}
		if err != nil {
			reason := fmt.Sprintf("%s: %s", methodLocation(m), err)
			flaggedMethods = append(flaggedMethods, starlark.String(reason))
			continue
		}
		desc.Methods[i].ReturnType = valueType
		desc.Methods[i].HasError = hasError
		desc.Methods[i].Compensable = compensable
		desc.Methods[i].ContentModel = inferContentModel(valueType, m.Params)
		returnValid = append(returnValid, desc.Methods[i])
	}
	desc.Methods = returnValid

	// Gate 3: verify compensable methods have matching Compensate<GoName> methods.
	// Requires all_methods on the descriptor — the Starlark layer must provide the
	// full provider method set from go.methods().
	hasCompensable := false
	for _, m := range desc.Methods {
		if m.Compensable {
			hasCompensable = true
			break
		}
	}
	if hasCompensable && len(desc.AllMethodNames) == 0 {
		return nil, fmt.Errorf("go.generate: descriptor has compensable methods but all_methods is empty — provide the full provider method set")
	}
	if hasCompensable {
		methodSet := make(map[string]bool, len(desc.AllMethodNames))
		for _, name := range desc.AllMethodNames {
			methodSet[name] = true
		}
		for _, m := range desc.Methods {
			if m.Compensable {
				compensateName := "Compensate" + m.GoName
				if !methodSet[compensateName] {
					return nil, fmt.Errorf("go.generate: %s: compensable return signature requires %s method on provider", methodLocation(m), compensateName)
				}
			}
		}
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

	return starlarkstruct.FromStringDict(starlark.String("generate_result"), starlark.StringDict{
		"code":    starlark.String(string(formatted)),
		"flagged": starlark.NewList(flaggedMethods),
	}), nil
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
	Provider   string             `yaml:"provider"`
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

	// Gate 1: validate param types — flag unprojectable methods.
	var mappingProjectable []methodInfo
	for _, m := range desc.Methods {
		if err := validateParamTypes(m.Params); err != nil {
			// Skip unprojectable methods in mapping output.
			continue
		}
		mappingProjectable = append(mappingProjectable, m)
	}
	desc.Methods = mappingProjectable

	// Gate 2: validate return signatures
	for _, m := range desc.Methods {
		if _, err := validateReturnSignature(m.ReturnType); err != nil {
			return nil, fmt.Errorf("go.mapping: %s: %w", methodLocation(m), err)
		}
	}

	// Build mapping structure
	mapping := mappingFile{
		Version:   "1.0",
		Struct:    desc.ImplType,
		Package:   desc.Package,
		Provider:  desc.Provider,
		Namespace: desc.Namespace,
	}

	for _, m := range desc.Methods {
		op := mappingOperation{
			Name:     desc.Provider + "." + m.SnakeName,
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

	provider, err := valueGetString(v, "provider")
	if err != nil {
		return nil, fmt.Errorf("descriptor.provider: %w", err)
	}
	desc.Provider = provider

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

	// Optional: extra attribute names from companion files
	extraVal, err := valueGetList(v, "extra_attrs")
	if err != nil {
		return nil, fmt.Errorf("descriptor.extra_attrs: %w", err)
	}
	for i := 0; i < extraVal.Len(); i++ {
		s, ok := starlark.AsString(extraVal.Index(i))
		if !ok {
			return nil, fmt.Errorf("descriptor.extra_attrs[%d]: expected string, got %s", i, extraVal.Index(i).Type())
		}
		desc.ExtraAttrs = append(desc.ExtraAttrs, s)
	}

	// Optional: all method names on the provider (for compensate validation)
	allMethodsVal, err := valueGetList(v, "all_methods")
	if err != nil {
		return nil, fmt.Errorf("descriptor.all_methods: %w", err)
	}
	for i := 0; i < allMethodsVal.Len(); i++ {
		s, ok := starlark.AsString(allMethodsVal.Index(i))
		if !ok {
			return nil, fmt.Errorf("descriptor.all_methods[%d]: expected string, got %s", i, allMethodsVal.Index(i).Type())
		}
		desc.AllMethodNames = append(desc.AllMethodNames, s)
	}

	// Optional: access level and title-case variant for RegisterBinding init()
	access, _ := valueGetString(v, "access")
	desc.Access = access
	accessTitle, _ := valueGetString(v, "access_title")
	desc.AccessTitle = accessTitle

	// Optional: lifetime level and title-case variant for RegisterBinding init()
	lifetime, _ := valueGetString(v, "lifetime")
	desc.Lifetime = lifetime
	lifetimeTitle, _ := valueGetString(v, "lifetime_title")
	desc.LifetimeTitle = lifetimeTitle

	// Optional: whether to emit init()/RegisterBinding (default true; dependent types set false)
	registered, _ := valueGetBool(v, "registered")
	desc.Registered = registered

	// Optional: wrapper suffix for generated type names (default: "Receiver")
	wrapperSuffix, _ := valueGetString(v, "wrapper_suffix")
	if wrapperSuffix == "" {
		wrapperSuffix = "Receiver"
	}
	desc.WrapperSuffix = wrapperSuffix

	// Optional: import path for parent package when generating into gen/ subpackage
	providerImport, _ := valueGetString(v, "provider_import")
	desc.ProviderImport = providerImport

	// Optional: provider field descriptors for ImmediateFactory construction
	providerFieldsVal, err := valueGetList(v, "provider_fields")
	if err != nil {
		return nil, fmt.Errorf("descriptor.provider_fields: %w", err)
	}
	for i := 0; i < providerFieldsVal.Len(); i++ {
		pf, err := providerFieldFromValue(providerFieldsVal.Index(i))
		if err != nil {
			return nil, fmt.Errorf("descriptor.provider_fields[%d]: %w", i, err)
		}
		desc.ProviderFields = append(desc.ProviderFields, pf)
	}

	// Optional: struct-to-Starlark converter functions (for struct_converter template)
	convertersVal, err := valueGetList(v, "converters")
	if err != nil {
		return nil, fmt.Errorf("descriptor.converters: %w", err)
	}
	for i := 0; i < convertersVal.Len(); i++ {
		ci, err := converterInfoFromValue(convertersVal.Index(i))
		if err != nil {
			return nil, fmt.Errorf("descriptor.converters[%d]: %w", i, err)
		}
		desc.Converters = append(desc.Converters, ci)
	}

	// Optional: cross-package gen imports for converter/receiver templates
	crossImportsVal, err := valueGetList(v, "cross_package_imports")
	if err != nil {
		return nil, fmt.Errorf("descriptor.cross_package_imports: %w", err)
	}
	for i := 0; i < crossImportsVal.Len(); i++ {
		alias, _ := valueGetString(crossImportsVal.Index(i), "alias")
		path, _ := valueGetString(crossImportsVal.Index(i), "path")
		if alias != "" && path != "" {
			desc.CrossPackageImports = append(desc.CrossPackageImports, crossPackageImport{Alias: alias, Path: path})
		}
	}

	return desc, nil
}

// converterInfoFromValue converts a Starlark dict to a converterInfo.
func converterInfoFromValue(v starlark.Value) (converterInfo, error) {
	funcName, err := valueGetString(v, "func_name")
	if err != nil {
		return converterInfo{}, fmt.Errorf("func_name: %w", err)
	}
	goType, err := valueGetString(v, "go_type")
	if err != nil {
		return converterInfo{}, fmt.Errorf("go_type: %w", err)
	}
	isPointer, _ := valueGetBool(v, "is_pointer")
	starlarkName, err := valueGetString(v, "starlark_name")
	if err != nil {
		return converterInfo{}, fmt.Errorf("starlark_name: %w", err)
	}

	fieldsVal, err := valueGetList(v, "fields")
	if err != nil {
		return converterInfo{}, fmt.Errorf("fields: %w", err)
	}
	var fields []converterFieldInfo
	for i := 0; i < fieldsVal.Len(); i++ {
		f, err := converterFieldInfoFromValue(fieldsVal.Index(i))
		if err != nil {
			return converterInfo{}, fmt.Errorf("fields[%d]: %w", i, err)
		}
		fields = append(fields, f)
	}

	return converterInfo{
		FuncName:     funcName,
		GoType:       goType,
		IsPointer:    isPointer,
		StarlarkName: starlarkName,
		Fields:       fields,
	}, nil
}

// converterFieldInfoFromValue converts a Starlark dict to a converterFieldInfo.
func converterFieldInfoFromValue(v starlark.Value) (converterFieldInfo, error) {
	goName, err := valueGetString(v, "go_name")
	if err != nil {
		return converterFieldInfo{}, fmt.Errorf("go_name: %w", err)
	}
	snakeName, err := valueGetString(v, "snake_name")
	if err != nil {
		return converterFieldInfo{}, fmt.Errorf("snake_name: %w", err)
	}
	kind, err := valueGetString(v, "kind")
	if err != nil {
		return converterFieldInfo{}, fmt.Errorf("kind: %w", err)
	}

	converter, _ := valueGetString(v, "converter")
	inlineName, _ := valueGetString(v, "inline_name")
	nullable, _ := valueGetBool(v, "nullable")
	nilExpr, _ := valueGetString(v, "nil_expr")
	guard, _ := valueGetString(v, "guard")
	guardField, _ := valueGetString(v, "guard_field")
	guardKind, _ := valueGetString(v, "guard_kind")

	// Parse inline fields recursively
	inlineFieldsVal, err := valueGetList(v, "inline_fields")
	if err != nil {
		return converterFieldInfo{}, fmt.Errorf("inline_fields: %w", err)
	}
	var inlineFields []converterFieldInfo
	for i := 0; i < inlineFieldsVal.Len(); i++ {
		f, err := converterFieldInfoFromValue(inlineFieldsVal.Index(i))
		if err != nil {
			return converterFieldInfo{}, fmt.Errorf("inline_fields[%d]: %w", i, err)
		}
		inlineFields = append(inlineFields, f)
	}

	return converterFieldInfo{
		GoName:       goName,
		SnakeName:    snakeName,
		Kind:         kind,
		Converter:    converter,
		InlineName:   inlineName,
		InlineFields: inlineFields,
		Nullable:     nullable,
		NilExpr:      nilExpr,
		Guard:        guard,
		GuardField:   guardField,
		GuardKind:    guardKind,
	}, nil
}

// providerFieldFromValue converts a Starlark dict to a providerField.
func providerFieldFromValue(v starlark.Value) (providerField, error) {
	goName, err := valueGetString(v, "go_name")
	if err != nil {
		return providerField{}, fmt.Errorf("go_name: %w", err)
	}
	cfgField, err := valueGetString(v, "cfg_field")
	if err != nil {
		return providerField{}, fmt.Errorf("cfg_field: %w", err)
	}
	dflt, _ := valueGetString(v, "default")
	zeroVal, _ := valueGetString(v, "zero_value")
	return providerField{
		GoName:    goName,
		CfgField:  cfgField,
		Default:   dflt,
		ZeroValue: zeroVal,
	}, nil
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

	file, _ := valueGetString(v, "file")
	line, _ := valueGetInt(v, "line")
	property, _ := valueGetBool(v, "property")      // optional
	resultExpr, _ := valueGetString(v, "result_expr") // optional

	return methodInfo{
		GoName:     name,
		SnakeName:  camelToSnake(name),
		Params:     params,
		ReturnType: returns,
		ResultExpr: resultExpr,
		Property:   property,
		Doc:        doc,
		File:       file,
		Line:       line,
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

	doc, _ := valueGetString(v, "doc")                // optional
	optional, _ := valueGetBool(v, "optional")         // optional
	dflt, _ := valueGetString(v, "default")            // optional
	structType, _ := valueGetString(v, "struct_type")  // optional

	pi := paramInfo{
		GoName:     name,
		SnakeName:  camelToSnake(name),
		GoType:     goType,
		Variadic:   variadic,
		Doc:        doc,
		Optional:   optional,
		Default:    dflt,
		StructType: structType,
	}

	// Optional: callable metadata for function-type params
	ci, err := callableInfoFromValue(v)
	if err != nil {
		return paramInfo{}, fmt.Errorf("callable: %w", err)
	}
	pi.Callable = ci

	return pi, nil
}

// callableInfoFromValue reads the optional "callable" dict from a param descriptor.
// Returns nil if no callable key is present.
func callableInfoFromValue(v starlark.Value) (*callableInfo, error) {
	// Try to get the "callable" key as a dict/struct
	var callableVal starlark.Value
	switch val := v.(type) {
	case *starlark.Dict:
		result, found, err := val.Get(starlark.String("callable"))
		if err != nil || !found {
			return nil, nil
		}
		callableVal = result
	case *starlarkstruct.Struct:
		attr, err := val.Attr("callable")
		if err != nil {
			return nil, nil
		}
		callableVal = attr
	default:
		return nil, nil
	}

	if callableVal == nil || callableVal == starlark.None {
		return nil, nil
	}

	typeName, err := valueGetString(callableVal, "type_name")
	if err != nil {
		return nil, fmt.Errorf("type_name: %w", err)
	}
	returns, _ := valueGetString(callableVal, "returns")

	// Parse params
	paramsVal, err := valueGetList(callableVal, "params")
	if err != nil {
		return nil, fmt.Errorf("params: %w", err)
	}
	var params []callableParam
	for i := 0; i < paramsVal.Len(); i++ {
		pv := paramsVal.Index(i)
		goName, _ := valueGetString(pv, "go_name")
		goType, _ := valueGetString(pv, "go_type")
		role, _ := valueGetString(pv, "role")
		params = append(params, callableParam{
			GoName: goName,
			GoType: goType,
			Role:   role,
		})
	}

	// Parse handle types
	handlesVal, err := valueGetList(callableVal, "handle_types")
	if err != nil {
		return nil, fmt.Errorf("handle_types: %w", err)
	}
	var handleTypes []handleType
	for i := 0; i < handlesVal.Len(); i++ {
		hv := handlesVal.Index(i)
		goType, _ := valueGetString(hv, "go_type")
		handleName, _ := valueGetString(hv, "handle_name")

		methodsVal, _ := valueGetList(hv, "methods")
		var methods []handleMethod
		for j := 0; j < methodsVal.Len(); j++ {
			mv := methodsVal.Index(j)
			mGoName, _ := valueGetString(mv, "go_name")
			mSnakeName, _ := valueGetString(mv, "snake_name")
			mRetType, _ := valueGetString(mv, "return_type")
			methods = append(methods, handleMethod{
				GoName:     mGoName,
				SnakeName:  mSnakeName,
				ReturnType: mRetType,
			})
		}

		handleTypes = append(handleTypes, handleType{
			GoType:     goType,
			HandleName: handleName,
			Methods:    methods,
		})
	}

	return &callableInfo{
		TypeName:    typeName,
		Params:      params,
		Returns:     returns,
		HandleTypes: handleTypes,
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

// valueGetInt extracts an int field from a dict or struct.
func valueGetInt(v starlark.Value, key string) (int, error) {
	switch val := v.(type) {
	case *starlark.Dict:
		result, found, err := val.Get(starlark.String(key))
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, nil
		}
		n, err := starlark.AsInt32(result)
		if err != nil {
			return 0, fmt.Errorf("expected int, got %s", result.Type())
		}
		return int(n), nil
	case *starlarkstruct.Struct:
		attr, err := val.Attr(key)
		if err != nil {
			return 0, nil
		}
		n, err := starlark.AsInt32(attr)
		if err != nil {
			return 0, fmt.Errorf("expected int, got %s", attr.Type())
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("expected dict or struct, got %s", v.Type())
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
