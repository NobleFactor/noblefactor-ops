// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"bytes"
	"fmt"
	"go/format"
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
	Template            string               // "planned_receiver", "graph_actions", "immediate_receiver"
	Package             string               // Go package name for generated file
	Provider            string               // snake_case provider (e.g., "file")
	StructName          string               // Go struct name (e.g., "File")
	WrapperSuffix       string               // suffix for the wrapper type: "Receiver" (default) or "Value"
	Namespace           string               // dotted namespace (e.g., "plan.file")
	ImplType            string               // implementation struct name for delegation (e.g., "fileOps")
	Methods             []methodInfo         // analyzed methods
	AllMethodNames      []string             // all method names on the provider (for compensate validation)
	Access              string               // access level: "immediate", "planned", "both"
	AccessTitle         string               // title-case access for Go constants: "Immediate", "Planned", "Both"
	Lifetime            string               // lifetime level: "stateless", "phase", "session"
	LifetimeTitle       string               // title-case lifetime for Go constants: "Stateless", "Phase", "Session"
	Registered          bool                 // if true, emit init()/RegisterBinding; false for dependent types
	ProviderFields      []providerField      // dynamic field init for ImmediateFactory (replaces hardcoded Writer/ProgramName/Color)
	ProviderImport      string               // if non-empty, import path for parent package (gen/ subpackage mode)
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
	GoType    string // Go type of the Provider field, set only when it differs from CfgType
	CfgType   string // Go type of the BindingConfig field, set only when GoType is set
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
	GoName        string // original Go name (e.g., "Copy")
	SnakeName     string // snake_case name (e.g., "copy")
	Params        []paramInfo
	ReturnType    string // value type from (T, error), empty for error-only
	ResultExpr    string // override for result expression (e.g., "indexToStarlark(result)")
	HasError      bool   // true if the method returns an error (standard/compensable)
	ContentModel  string // "none", "consumer", "transformer"
	Compensable   bool   // inferred from return signature: (T, U, error) = true
	CompStateType string // Go type U from (T, U, error) — used for typed assertion in Undo
	Property      bool   // if true, exposed as read-only property (direct Attr value, no callable)
	Doc           string
	File          string // source file basename (e.g., "provider.go")
	Line          int    // source line number
}

// paramInfo holds information about a single parameter.
type paramInfo struct {
	GoName     string // original param name
	SnakeName  string // snake_case name
	GoType     string // Go type string
	Variadic   bool
	Doc        string        // parameter description from Parameters: section
	Optional   bool          // if true, param is keyword-optional (? suffix in UnpackArgs)
	Default    string        // Go default expression (e.g., "true", "10"); empty = zero value
	StructType string        // if set, param was expanded from this struct type (fields already validated)
	Callable   *callableInfo // if set, this param is a callable (function type) with classified params
}

// callableInfo describes a function-type parameter with classified params.
type callableInfo struct {
	TypeName string          // Go type name (e.g., "Visitor")
	Params   []callableParam // classified parameters
	Returns  string          // return type string (e.g., "(any, error)")
}

// callableParam describes a single classified parameter of a callable.
type callableParam struct {
	GoName string // Go parameter name (e.g., "path")
	GoType string // Go type string (e.g., "string")
	Role   string // "projected", "pass_through", "swallowed"
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

// =============================================================================
// TEMPLATE FUNCTIONS
// =============================================================================

var genTemplateFuncs = template.FuncMap{
	"converterFunc":      templateFuncConverterFunc,
	"needsOpImport":      templateFuncNeedsOpImport,
	"paramNamesList":     templateFuncParamNamesList,
	"providerFieldInit":  templateFuncProviderFieldInit,
	"providerInit":       templateFuncProviderInit,
	"providerTypePrefix": templateFuncProviderTypePrefix,
}

// templateFuncProviderFieldInit generates the Provider struct construction for
// ImmediateFactory from ProviderFields. Replaces the hard-coded Writer/ProgramName/Color.
//
// When a provider field's Go type differs from the BindingConfig field type
// (e.g., Root is file.Resource but WorkDir is string), generates an
// op.Construct call to marshal the value through the constructor registry.
func templateFuncProviderFieldInit(d *generateDescriptor) string {
	if len(d.ProviderFields) == 0 {
		return ""
	}

	prefix := templateFuncProviderTypePrefix(d)
	var buf strings.Builder

	// Track fields that need construction (type mismatch).
	constructedVars := make(map[string]string)

	for _, pf := range d.ProviderFields {
		localVar := strings.ToLower(pf.GoName[:1]) + pf.GoName[1:]
		buf.WriteString(fmt.Sprintf("\t\t\t%s := cfg.%s\n", localVar, pf.CfgField))
		if pf.Default != "" {
			buf.WriteString(fmt.Sprintf("\t\t\tif %s == %s {\n\t\t\t\t%s = %s\n\t\t\t}\n",
				localVar, pf.ZeroValue, localVar, pf.Default))
		}
		if pf.GoType != "" {
			convertedVar := localVar + "Val"
			qualifiedType := prefix + pf.GoType
			buf.WriteString(fmt.Sprintf("\t\t\t%s, err := op.Construct[%s](%s)\n",
				convertedVar, qualifiedType, localVar))
			buf.WriteString(fmt.Sprintf("\t\t\tif err != nil {\n\t\t\t\tpanic(\"%s: construct %s: \" + err.Error())\n\t\t\t}\n",
				d.Provider, pf.GoName))
			constructedVars[pf.GoName] = convertedVar
		}
	}

	buf.WriteString(fmt.Sprintf("\t\t\treturn New%s%s(&%sProvider{\n", d.StructName, d.WrapperSuffix, prefix))
	buf.WriteString("\t\t\t\tProviderBase: op.NewProviderBase(op.Context{\n")
	buf.WriteString("\t\t\t\t\tWriter:   cfg.Writer,\n")
	buf.WriteString("\t\t\t\t\tPlatform: cfg.Platform,\n")
	buf.WriteString("\t\t\t\t}),\n")
	for _, pf := range d.ProviderFields {
		localVar := strings.ToLower(pf.GoName[:1]) + pf.GoName[1:]
		if converted, ok := constructedVars[pf.GoName]; ok {
			localVar = converted
		}
		buf.WriteString(fmt.Sprintf("\t\t\t\t%s: %s,\n", pf.GoName, localVar))
	}
	buf.WriteString("\t\t\t})")
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

// =============================================================================
// MARSHALER-BASED TEMPLATE FUNCTIONS
// =============================================================================

// templateFuncParamNamesList generates the quoted, comma-separated parameter
// name list for a single method's MethodParams entry. All params are included
// except callables (which the bridge handles separately).
// Optional params (with Default or marked Optional) get a "?" suffix.
func templateFuncParamNamesList(m methodInfo) string {
	var names []string
	for _, p := range m.Params {
		if p.Callable != nil {
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

// templateFuncProviderInit generates the ImmediateFactory body that constructs
// the provider and delegates to New<StructName>Receiver. For providers with
// ProviderFields (bind directives), fields are read from BindingConfig first.
//
// When a provider field's Go type differs from the BindingConfig field type
// (e.g., Root is file.Resource but WorkDir is string), generates an
// op.Construct call to marshal the value through the constructor registry.
func templateFuncProviderInit(d *generateDescriptor) string {
	prefix := templateFuncProviderTypePrefix(d)
	var buf strings.Builder

	if len(d.ProviderFields) > 0 {
		constructedVars := make(map[string]string)
		for _, pf := range d.ProviderFields {
			localVar := strings.ToLower(pf.GoName[:1]) + pf.GoName[1:]
			buf.WriteString(fmt.Sprintf("\t\t\t%s := cfg.%s\n", localVar, pf.CfgField))
			if pf.Default != "" {
				buf.WriteString(fmt.Sprintf("\t\t\tif %s == %s {\n\t\t\t\t%s = %s\n\t\t\t}\n",
					localVar, pf.ZeroValue, localVar, pf.Default))
			}
			if pf.GoType != "" {
				convertedVar := localVar + "Val"
				qualifiedType := prefix + pf.GoType
				buf.WriteString(fmt.Sprintf("\t\t\t%s, err := op.Construct[%s](%s)\n",
					convertedVar, qualifiedType, localVar))
				buf.WriteString(fmt.Sprintf("\t\t\tif err != nil {\n\t\t\t\tpanic(\"%s: construct %s: \" + err.Error())\n\t\t\t}\n",
					d.Provider, pf.GoName))
				constructedVars[pf.GoName] = convertedVar
			}
		}
		buf.WriteString(fmt.Sprintf("\t\t\treturn New%s%s(&%sProvider{", d.StructName, d.WrapperSuffix, prefix))
		for i, pf := range d.ProviderFields {
			if i > 0 {
				buf.WriteString(", ")
			}
			localVar := strings.ToLower(pf.GoName[:1]) + pf.GoName[1:]
			if converted, ok := constructedVars[pf.GoName]; ok {
				localVar = converted
			}
			buf.WriteString(fmt.Sprintf("%s: %s", pf.GoName, localVar))
		}
		buf.WriteString("})")
	} else {
		buf.WriteString(fmt.Sprintf("\t\t\treturn New%s%s(&%sProvider{})", d.StructName, d.WrapperSuffix, prefix))
	}

	return buf.String()
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
				ProviderBase: op.NewProviderBase(op.Context{
					Writer:   cfg.Writer,
					Platform: cfg.Platform,
				}),
			})
{{- end}}
		},
	})
}
{{end}}
func New{{.StructName}}{{.WrapperSuffix}}(p *{{providerTypePrefix .}}{{.ImplType}}) *op.ReflectedReceiver {
	return op.WrapReceiver("{{.Namespace}}", p, Params)
}
`

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

	// Gate 1: callable params are only supported in immediate receivers.
	projectable := desc.Methods
	var flaggedMethods []starlark.Value

	//
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
	goType, _ := valueGetString(v, "go_type")
	cfgType, _ := valueGetString(v, "cfg_type")
	return providerField{
		GoName:    goName,
		CfgField:  cfgField,
		Default:   dflt,
		ZeroValue: zeroVal,
		GoType:    goType,
		CfgType:   cfgType,
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
	property, _ := valueGetBool(v, "property")        // optional
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
	optional, _ := valueGetBool(v, "optional")        // optional
	dflt, _ := valueGetString(v, "default")           // optional
	structType, _ := valueGetString(v, "struct_type") // optional

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

	return &callableInfo{
		TypeName: typeName,
		Params:   params,
		Returns:  returns,
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
