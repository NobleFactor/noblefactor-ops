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
	Template   string       // "planned_receiver", "graph_actions", "immediate_receiver"
	Package    string       // Go package name for generated file
	Provider   string       // snake_case provider (e.g., "file")
	StructName string       // Go struct name (e.g., "File")
	Namespace  string       // dotted namespace (e.g., "plan.file")
	ImplType   string       // implementation struct name for delegation (e.g., "fileOps")
	Methods    []methodInfo // analyzed methods
	ExtraAttrs []string     // additional attr names from companion files (e.g., query methods)
}

// methodInfo holds analyzed information about a single method.
type methodInfo struct {
	GoName       string      // original Go name (e.g., "Copy")
	SnakeName    string      // snake_case name (e.g., "copy")
	Params       []paramInfo
	ReturnType   string // value type from (T, error), empty for error-only
	ContentModel string // "none", "consumer", "transformer"
	Compensable  bool   // has a Compensate<GoName> pair on the provider
	Doc          string
	File         string // source file basename (e.g., "provider.go")
	Line         int    // source line number
}

// paramInfo holds information about a single parameter.
type paramInfo struct {
	GoName    string // original param name
	SnakeName string // snake_case name
	GoType    string // Go type string
	Variadic  bool
	Doc       string // parameter description from Parameters: section
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
	// Starlark-facing: in plan UnpackArgs + graph actions slot readers
	"string":         {unpackType: "string", slotReader: `slots["%s"].(string)`, starlarkFacing: true},
	"bool":           {unpackType: "bool", slotReader: `slots["%s"].(bool)`, starlarkFacing: true},
	"int":            {unpackType: "int", slotReader: `slots["%s"].(int)`, starlarkFacing: true},
	"int64":          {unpackType: "int64", slotReader: `slots["%s"].(int64)`, starlarkFacing: true},
	"[]string":       {unpackType: "*starlark.List", slotReader: `slots["%s"].([]string)`, starlarkFacing: true},
	"os.FileMode":    {unpackType: "int", slotReader: `slots["%s"].(os.FileMode)`, starlarkFacing: true},
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
		return "", fmt.Errorf("expected (T, error), got %s — use CompensateMethod for (T, U, error)", returns)
	}
	return valueType, nil
}

// validateCompensableReturn checks that a compensable method returns (T, U, error).
// T is the Result, U is the UndoState. Every method has a Result.
func validateCompensableReturn(returns string) (string, error) {
	if returns == "" {
		return "", fmt.Errorf("expected (T, U, error), got no return value")
	}
	if !strings.HasPrefix(returns, "(") || !strings.HasSuffix(returns, ")") {
		return "", fmt.Errorf("expected (T, U, error), got %s", returns)
	}
	inner := returns[1 : len(returns)-1]
	if !strings.HasSuffix(inner, ", error") {
		return "", fmt.Errorf("expected (T, U, error), got %s — must end with error", returns)
	}
	withoutError := strings.TrimSuffix(inner, ", error")
	if withoutError == "" {
		return "", fmt.Errorf("expected (T, U, error), got %s — missing Result and UndoState", returns)
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
		return "", fmt.Errorf("expected (T, U, error), got (%s, error) — missing Result or UndoState", withoutError)
	case 1:
		// (T, U, error) — Result + UndoState
		valueType := strings.TrimSpace(withoutError[:topLevelCommas[0]])
		if valueType == "" {
			return "", fmt.Errorf("expected (T, U, error), got %s — empty Result type", returns)
		}
		return valueType, nil
	default:
		return "", fmt.Errorf("expected (T, U, error), got %s — too many return values", returns)
	}
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
	"attrNamesList":        tplAttrNamesList,
	"allAttrNames":         tplAllAttrNames,
	"hasExtraAttrs":        tplHasExtraAttrs,
	"planUnpackArgs":       tplPlanUnpackArgs,
	"planFillSlots":        tplPlanFillSlots,
	"immediateUnpackArgs":   tplImmediateUnpackArgs,
	"immediateProviderBody": tplImmediateProviderBody,
	"needsImport":          tplNeedsImport,
	"graphReaders":         tplGraphReaders,
	"dryRunFmt":            tplDryRunFmt,
	"dryRunVars":           tplDryRunVars,
	"dryRunChecksum":       tplDryRunChecksum,
	"implArgs":             tplImplArgs,
	"graphReturn":          tplGraphReturn,
	"graphUndo":            tplGraphUndo,
	"docComment":           tplDocComment,
	"docSummary":           tplDocSummary,
	"hasSlotDocs":          tplHasSlotDocs,
	"slotDocs":             tplSlotDocs,
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

// tplAllAttrNames returns all attribute names (generated methods + extra attrs)
// as a sorted, quoted, comma-separated string. Used by receivers with companion
// query files that contribute additional attributes.
func tplAllAttrNames(d *generateDescriptor) string {
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

// tplHasExtraAttrs returns true if the descriptor has extra attribute names.
func tplHasExtraAttrs(d *generateDescriptor) bool {
	return len(d.ExtraAttrs) > 0
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

func tplImmediateUnpackArgs(m methodInfo) string {
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

// tplImmediateProviderBody generates the Provider delegation call body for an
// immediate receiver method. It maps parameters from their Starlark-unpacked
// types to Provider method arguments, calls r.provider.GoName(...), and converts
// the return value to a Starlark value. Compensation state is ignored —
// immediate receivers discard undo state.
func tplImmediateProviderBody(m methodInfo) string {
	// Build conversion declarations and call args.
	// Some types require multi-return conversion (e.g., op.StarlarkDictToMap)
	// which must be pre-computed as variable declarations.
	var convDecls []string
	var callArgs []string
	for _, p := range m.Params {
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

	call := fmt.Sprintf("r.provider.%s(%s)", m.GoName, strings.Join(callArgs, ", "))

	if m.Compensable {
		if m.ReturnType == "" {
			buf.WriteString(fmt.Sprintf("\t_, err := %s\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn starlark.None, nil", call))
		} else {
			buf.WriteString(fmt.Sprintf("\tresult, _, err := %s\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn %s, nil", call, immediateResultExpr(m.ReturnType, "result")))
		}
		return buf.String()
	}

	if m.ReturnType == "" {
		buf.WriteString(fmt.Sprintf("\tif err := %s; err != nil {\n\t\treturn nil, err\n\t}\n\treturn starlark.None, nil", call))
	} else {
		buf.WriteString(fmt.Sprintf("\tresult, err := %s\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn %s, nil", call, immediateResultExpr(m.ReturnType, "result")))
	}
	return buf.String()
}

// immediateArgExpr returns a conversion declaration (if needed) and the Go
// expression for passing a parameter to a Provider method from an immediate
// receiver. Multi-return conversions (like op.StarlarkDictToMap) produce a
// declaration string; single-value conversions return only the inline expression.
func immediateArgExpr(p paramInfo) (decl, arg string) {
	tm := typeMappings[p.GoType]
	if tm.contextReader != "" {
		return "", "r.output"
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

// immediateResultExpr returns the Go expression for converting an immediate
// receiver's Provider return value to a Starlark value.
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
	default:
		return "starlark.None"
	}
}

// tplNeedsImport checks whether any method parameter uses the given Go type.
// Used in templates for conditional imports (e.g., "os" for os.FileMode).
func tplNeedsImport(methods []methodInfo, goType string) bool {
	for _, m := range methods {
		for _, p := range m.Params {
			if p.GoType == goType {
				return true
			}
		}
	}
	return false
}

// tplGraphReaders generates variable declarations for Do: slot reads,
// context reads, and engine-injected reads. Content params use optional
// assertion (_, ok pattern) since they may arrive via promise slots.
func tplGraphReaders(m methodInfo) string {
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

// tplDryRunChecksum generates additional dry-run output for consumer content model.
// Previously emitted ctx.TargetChecksum; now a no-op (checksums removed from Context).
func tplDryRunChecksum(m methodInfo) string {
	return ""
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
// Returns (Result, UndoState, error) — three values.
func tplGraphReturn(m methodInfo, implType string) string {
	if implType == "" {
		return "\nreturn nil, nil, nil"
	}

	argStr := tplImplArgs(m)
	call := fmt.Sprintf("o.Impl.%s(%s)", m.GoName, argStr)

	if m.Compensable {
		return tplGraphReturnCompensable(m, call)
	}

	switch m.ContentModel {
	case "consumer":
		// Consumer returns result (e.g., checksum), no undo state
		return fmt.Sprintf("\nresult, err := %s\nreturn result, nil, err", call)
	case "transformer":
		// Transformer returns transformed content as Result
		return fmt.Sprintf("\nresult, err := %s\nif err != nil {\nreturn nil, nil, err\n}\nreturn result, nil, nil", call)
	default: // "none"
		if m.ReturnType == "" {
			// Error-only return
			return fmt.Sprintf("\nreturn nil, nil, %s", call)
		}
		// Has value return — pass through as Result
		return fmt.Sprintf("\nresult, err := %s\nreturn result, nil, err", call)
	}
}

// tplGraphReturnCompensable generates the delegation call for compensable methods.
// Compensable methods return (U, error) or (T, U, error) where U becomes UndoState.
func tplGraphReturnCompensable(m methodInfo, call string) string {
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


// tplGraphUndo generates the Undo method for an action. Compensable actions
// delegate to Impl.Compensate<GoName>(state). Non-compensable actions return nil.
func tplGraphUndo(m methodInfo) string {
	if !m.Compensable {
		return "" // No Undo method — struct implements Action only, not Undoable.
	}
	return fmt.Sprintf("func (o *%s) Undo(state execution.UndoState) error {\n\tif state == nil {\n\t\treturn nil\n\t}\n\treturn o.Impl.Compensate%s(state)\n}", m.GoName, m.GoName)
}

// tplDocComment renders a multi-line Go doc comment. The first line is prefixed
// with "// snakeName ", subsequent lines get "// " (or "//" for blank lines).
func tplDocComment(snakeName, doc string) string {
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

// tplDocSummary returns the description portion of a doc string — text before
// the first blank line or structured section (Slots:, Parameters:, Usage:, Returns:).
func tplDocSummary(doc string) string {
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

// tplHasSlotDocs returns true if any starlark-facing parameter has documentation.
func tplHasSlotDocs(m methodInfo) bool {
	for _, p := range m.Params {
		tm := typeMappings[p.GoType]
		if tm.starlarkFacing && p.Doc != "" && !isContentParam(p, m) {
			return true
		}
	}
	return false
}

// tplSlotDocs generates a "// Slots:" comment block from structured parameter docs.
// Returns empty string if no starlark-facing params have docs.
func tplSlotDocs(m methodInfo) string {
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

// =============================================================================
// TEMPLATES
// =============================================================================

// ImmediateReceiverTemplate is the builtin template for immediate receivers.
// Generated code imports pkg/op for Receiver, MakeAttr, NoSuchAttrError.
const ImmediateReceiverTemplate = `// Code generated by go.generate; DO NOT EDIT.

package {{.Package}}

import (
	"go.starlark.net/starlark"

	"github.com/NobleFactor/devlore-cli/pkg/op"
)

type {{.StructName}}Receiver struct {
	op.Receiver
}

func New{{.StructName}}Receiver() *{{.StructName}}Receiver {
	return &{{.StructName}}Receiver{Receiver: op.NewReceiver("{{.Provider}}")}
}

func (r *{{.StructName}}Receiver) Attr(name string) (starlark.Value, error) {
	switch name {
{{- range .Methods}}
	case "{{.SnakeName}}":
		return op.MakeAttr("{{$.Provider}}.{{.SnakeName}}", r.{{.SnakeName}}), nil
{{- end}}
	default:
		return nil, op.NoSuchAttrError("{{.Provider}}", name)
	}
}

func (r *{{.StructName}}Receiver) AttrNames() []string {
	return []string{{"{"}}{{attrNamesList .Methods}}{{"}"}}
}
{{range .Methods}}
func (r *{{$.StructName}}Receiver) {{.SnakeName}}(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
{{immediateUnpackArgs .}}
	// TODO: call backing implementation, convert result
	return starlark.None, nil
}
{{end}}`

// builtinTemplates maps names to content for builtin templates.
var builtinTemplates = map[string]string{
	"immediate_receiver": ImmediateReceiverTemplate,
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
	if content, ok := builtinTemplates[templateContent]; ok {
		templateContent = content
	}

	// Parse template content
	tmpl, err := template.New("gen").Funcs(genTemplateFuncs).Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("go.generate: template parse: %w", err)
	}

	// Convert descriptor
	desc, err := descriptorFromValue("gen", descriptorVal)
	if err != nil {
		return nil, fmt.Errorf("go.generate: %w", err)
	}

	// Gate 1: validate param types
	for _, m := range desc.Methods {
		if err := validateParamTypes(m.Params); err != nil {
			return nil, fmt.Errorf("go.generate: %s: %w", methodLocation(m), err)
		}
	}

	// Gate 2: validate return signatures and infer content models
	for i, m := range desc.Methods {
		rawReturn := m.ReturnType

		var valueType string
		var err error
		if m.Compensable {
			valueType, err = validateCompensableReturn(rawReturn)
		} else {
			valueType, err = validateReturnSignature(rawReturn)
		}
		if err != nil {
			return nil, fmt.Errorf("go.generate: %s: %w", methodLocation(m), err)
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

	// Gate 1: validate param types
	for _, m := range desc.Methods {
		if err := validateParamTypes(m.Params); err != nil {
			return nil, fmt.Errorf("go.mapping: %s: %w", methodLocation(m), err)
		}
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

	compensable, _ := valueGetBool(v, "compensable")
	file, _ := valueGetString(v, "file")
	line, _ := valueGetInt(v, "line")

	return methodInfo{
		GoName:      name,
		SnakeName:   camelToSnake(name),
		Params:      params,
		ReturnType:  returns,
		Compensable: compensable,
		Doc:         doc,
		File:        file,
		Line:        line,
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

	doc, _ := valueGetString(v, "doc") // optional

	return paramInfo{
		GoName:    name,
		SnakeName: camelToSnake(name),
		GoType:    goType,
		Variadic:  variadic,
		Doc:       doc,
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
