// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"

	"github.com/NobleFactor/devlore-cli/pkg/op"
)

// StarlarkParseReceiver provides Starlark source parsing operations.
// Implements starlark.Value and starlark.HasAttrs.
// Note: Named starlark_parse to avoid conflict with the starlark package name.
type StarlarkParseReceiver struct {
	op.Receiver
}

// NewStarlarkParseReceiver creates a new StarlarkParseReceiver.
func NewStarlarkParseReceiver() *StarlarkParseReceiver {
	return &StarlarkParseReceiver{Receiver: op.NewReceiver("starlark_parse")}
}

// Attr implements starlark.HasAttrs.
func (r *StarlarkParseReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "parse":
		return op.MakeAttr("starlark_parse.parse", r.parse), nil
	case "complexity":
		return op.MakeAttr("starlark_parse.complexity", r.complexity), nil
	case "metrics":
		return op.MakeAttr("starlark_parse.metrics", r.metrics), nil
	default:
		return nil, op.NoSuchAttrError("starlark_parse", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *StarlarkParseReceiver) AttrNames() []string {
	return []string{"complexity", "metrics", "parse"}
}

// =============================================================================
// STARLARK PARSING
// =============================================================================

// StarlarkFunction represents a function defined in a Starlark file.
type StarlarkFunction struct {
	Name       string
	Line       int
	EndLine    int
	Params     []string
	HasVarargs bool
	HasKwargs  bool
	DocString  string
	Decorators []string
}

// StarlarkLoad represents a load() statement.
type StarlarkLoad struct {
	Module  string
	Symbols []string
	Line    int
}

// StarlarkGlobal represents a top-level assignment.
type StarlarkGlobal struct {
	Name  string
	Line  int
	Value string
}

// StarlarkParseResult holds the parsed structure of a Starlark file.
type StarlarkParseResult struct {
	Path      string
	Functions []StarlarkFunction
	Loads     []StarlarkLoad
	Globals   []StarlarkGlobal
	LOC       int
	SLOC      int
	Comments  int
	Blanks    int
}

func (r *StarlarkParseReceiver) parse(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("starlark_parse.parse", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectStarlarkFiles(path)
	if err != nil {
		return nil, fmt.Errorf("starlark_parse.parse: %w", err)
	}

	var allFiles []starlark.Value
	var totalFunctions, totalLoads, totalGlobals, totalLOC, totalSLOC int

	for _, file := range files {
		result, err := parseStarlarkFile(file)
		if err != nil {
			continue
		}

		totalFunctions += len(result.Functions)
		totalLoads += len(result.Loads)
		totalGlobals += len(result.Globals)
		totalLOC += result.LOC
		totalSLOC += result.SLOC

		allFiles = append(allFiles, starlarkParseResultToValue(result))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"files":           starlark.NewList(allFiles),
		"total_functions": starlark.MakeInt(totalFunctions),
		"total_loads":     starlark.MakeInt(totalLoads),
		"total_globals":   starlark.MakeInt(totalGlobals),
		"total_loc":       starlark.MakeInt(totalLOC),
		"total_sloc":      starlark.MakeInt(totalSLOC),
	}), nil
}

func parseStarlarkFile(path string) (StarlarkParseResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return StarlarkParseResult{}, err
	}

	opts := syntax.FileOptions{}
	f, err := opts.Parse(path, content, 0)
	if err != nil {
		return StarlarkParseResult{}, err
	}

	result := StarlarkParseResult{Path: path}

	lines := strings.Split(string(content), "\n")
	result.LOC = len(lines)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result.Blanks++
		} else if strings.HasPrefix(trimmed, "#") {
			result.Comments++
		}
	}
	result.SLOC = result.LOC - result.Blanks - result.Comments

	for _, stmt := range f.Stmts {
		switch s := stmt.(type) {
		case *syntax.DefStmt:
			fn := extractStarlarkFunction(s)
			result.Functions = append(result.Functions, fn)

		case *syntax.LoadStmt:
			moduleStr, ok := s.Module.Value.(string)
			if !ok {
				continue
			}
			load := StarlarkLoad{
				Module: moduleStr,
				Line:   int(s.Module.TokenPos.Line),
			}
			for _, binding := range s.From {
				load.Symbols = append(load.Symbols, binding.Name)
			}
			result.Loads = append(result.Loads, load)

		case *syntax.AssignStmt:
			if ident, ok := s.LHS.(*syntax.Ident); ok {
				global := StarlarkGlobal{
					Name: ident.Name,
					Line: int(ident.NamePos.Line),
				}
				global.Value = exprToString(s.RHS)
				result.Globals = append(result.Globals, global)
			}
		}
	}

	return result, nil
}

func extractStarlarkFunction(def *syntax.DefStmt) StarlarkFunction {
	_, end := def.Span()
	fn := StarlarkFunction{
		Name:    def.Name.Name,
		Line:    int(def.Def.Line),
		EndLine: int(end.Line),
	}

	for _, param := range def.Params {
		switch p := param.(type) {
		case *syntax.Ident:
			fn.Params = append(fn.Params, p.Name)
		case *syntax.BinaryExpr:
			if ident, ok := p.X.(*syntax.Ident); ok {
				fn.Params = append(fn.Params, ident.Name)
			}
		case *syntax.UnaryExpr:
			if ident, ok := p.X.(*syntax.Ident); ok {
				switch p.Op {
				case syntax.STAR:
					fn.HasVarargs = true
					fn.Params = append(fn.Params, "*"+ident.Name)
				case syntax.STARSTAR:
					fn.HasKwargs = true
					fn.Params = append(fn.Params, "**"+ident.Name)
				}
			}
		}
	}

	if len(def.Body) > 0 {
		if expr, ok := def.Body[0].(*syntax.ExprStmt); ok {
			if lit, ok := expr.X.(*syntax.Literal); ok && lit.Token == syntax.STRING {
				if s, ok := lit.Value.(string); ok {
					fn.DocString = s
				}
			}
		}
	}

	return fn
}

func exprToString(expr syntax.Expr) string {
	switch e := expr.(type) {
	case *syntax.Literal:
		switch e.Token {
		case syntax.STRING:
			if s, ok := e.Value.(string); ok {
				return fmt.Sprintf("%q", s)
			}
		case syntax.INT:
			return fmt.Sprintf("%v", e.Value)
		case syntax.FLOAT:
			return fmt.Sprintf("%v", e.Value)
		}
	case *syntax.Ident:
		return e.Name
	case *syntax.ListExpr:
		return "[...]"
	case *syntax.DictExpr:
		return "{...}"
	case *syntax.CallExpr:
		if ident, ok := e.Fn.(*syntax.Ident); ok {
			return ident.Name + "(...)"
		}
		return "call(...)"
	}
	return ""
}

func starlarkParseResultToValue(r StarlarkParseResult) starlark.Value {
	var functions []starlark.Value
	for _, fn := range r.Functions {
		var params []starlark.Value
		for _, p := range fn.Params {
			params = append(params, starlark.String(p))
		}
		var decorators []starlark.Value
		for _, d := range fn.Decorators {
			decorators = append(decorators, starlark.String(d))
		}

		functions = append(functions, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":        starlark.String(fn.Name),
			"line":        starlark.MakeInt(fn.Line),
			"end_line":    starlark.MakeInt(fn.EndLine),
			"params":      starlark.NewList(params),
			"has_varargs": starlark.Bool(fn.HasVarargs),
			"has_kwargs":  starlark.Bool(fn.HasKwargs),
			"docstring":   starlark.String(fn.DocString),
			"decorators":  starlark.NewList(decorators),
		}))
	}

	var loads []starlark.Value
	for _, l := range r.Loads {
		var symbols []starlark.Value
		for _, s := range l.Symbols {
			symbols = append(symbols, starlark.String(s))
		}
		loads = append(loads, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"module":  starlark.String(l.Module),
			"symbols": starlark.NewList(symbols),
			"line":    starlark.MakeInt(l.Line),
		}))
	}

	var globals []starlark.Value
	for _, g := range r.Globals {
		globals = append(globals, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":  starlark.String(g.Name),
			"line":  starlark.MakeInt(g.Line),
			"value": starlark.String(g.Value),
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":      starlark.String(r.Path),
		"functions": starlark.NewList(functions),
		"loads":     starlark.NewList(loads),
		"globals":   starlark.NewList(globals),
		"loc":       starlark.MakeInt(r.LOC),
		"sloc":      starlark.MakeInt(r.SLOC),
		"comments":  starlark.MakeInt(r.Comments),
		"blanks":    starlark.MakeInt(r.Blanks),
	})
}

// =============================================================================
// STARLARK COMPLEXITY
// =============================================================================

// StarlarkFunctionComplexity holds complexity metrics for a Starlark function.
type StarlarkFunctionComplexity struct {
	Name         string
	Line         int
	Cyclomatic   int
	Cognitive    int
	NestingDepth int
	LOC          int
	Params       int
}

func (r *StarlarkParseReceiver) complexity(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("starlark_parse.complexity", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectStarlarkFiles(path)
	if err != nil {
		return nil, fmt.Errorf("starlark_parse.complexity: %w", err)
	}

	var allFiles []starlark.Value
	var totalCyclo, totalCognit, totalFuncs int
	var maxCyclo, maxCognit int
	var maxCycloFunc, maxCognitFunc string
	var hotspots []starlark.Value

	for _, file := range files {
		fileResult, funcs := analyzeStarlarkComplexity(file)
		allFiles = append(allFiles, fileResult)

		for _, fn := range funcs {
			totalFuncs++
			totalCyclo += fn.Cyclomatic
			totalCognit += fn.Cognitive

			if fn.Cyclomatic > maxCyclo {
				maxCyclo = fn.Cyclomatic
				maxCycloFunc = fn.Name
			}
			if fn.Cognitive > maxCognit {
				maxCognit = fn.Cognitive
				maxCognitFunc = fn.Name
			}

			if fn.Cyclomatic > 10 || fn.Cognitive > 15 {
				hotspots = append(hotspots, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
					"name":       starlark.String(fn.Name),
					"file":       starlark.String(filepath.Base(file)),
					"line":       starlark.MakeInt(fn.Line),
					"cyclomatic": starlark.MakeInt(fn.Cyclomatic),
					"cognitive":  starlark.MakeInt(fn.Cognitive),
					"loc":        starlark.MakeInt(fn.LOC),
				}))
			}
		}
	}

	var avgCyclo, avgCognit float64
	if totalFuncs > 0 {
		avgCyclo = float64(totalCyclo) / float64(totalFuncs)
		avgCognit = float64(totalCognit) / float64(totalFuncs)
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"files":            starlark.NewList(allFiles),
		"total_cyclomatic": starlark.MakeInt(totalCyclo),
		"total_cognitive":  starlark.MakeInt(totalCognit),
		"total_functions":  starlark.MakeInt(totalFuncs),
		"avg_cyclomatic":   starlark.Float(avgCyclo),
		"avg_cognitive":    starlark.Float(avgCognit),
		"max_cyclomatic":   starlark.MakeInt(maxCyclo),
		"max_cognitive":    starlark.MakeInt(maxCognit),
		"max_cyclo_func":   starlark.String(maxCycloFunc),
		"max_cognit_func":  starlark.String(maxCognitFunc),
		"hotspots":         starlark.NewList(hotspots),
	}), nil
}

func analyzeStarlarkComplexity(path string) (starlark.Value, []StarlarkFunctionComplexity) {
	content, err := os.ReadFile(path)
	if err != nil {
		return starlark.None, nil
	}

	opts := syntax.FileOptions{}
	f, err := opts.Parse(path, content, 0)
	if err != nil {
		return starlark.None, nil
	}

	var functions []StarlarkFunctionComplexity

	for _, stmt := range f.Stmts {
		if def, ok := stmt.(*syntax.DefStmt); ok {
			fc := calculateStarlarkFunctionComplexity(def)
			functions = append(functions, fc)
		}
	}

	var funcList []starlark.Value
	var totalCyclo, totalCognit int
	for _, fn := range functions {
		totalCyclo += fn.Cyclomatic
		totalCognit += fn.Cognitive
		funcList = append(funcList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":          starlark.String(fn.Name),
			"line":          starlark.MakeInt(fn.Line),
			"cyclomatic":    starlark.MakeInt(fn.Cyclomatic),
			"cognitive":     starlark.MakeInt(fn.Cognitive),
			"nesting_depth": starlark.MakeInt(fn.NestingDepth),
			"loc":           starlark.MakeInt(fn.LOC),
			"params":        starlark.MakeInt(fn.Params),
		}))
	}

	fileResult := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":           starlark.String(path),
		"functions":      starlark.NewList(funcList),
		"total_cyclo":    starlark.MakeInt(totalCyclo),
		"total_cognit":   starlark.MakeInt(totalCognit),
		"function_count": starlark.MakeInt(len(functions)),
	})

	return fileResult, functions
}

func calculateStarlarkFunctionComplexity(def *syntax.DefStmt) StarlarkFunctionComplexity {
	_, end := def.Span()
	fc := StarlarkFunctionComplexity{
		Name:   def.Name.Name,
		Line:   int(def.Def.Line),
		LOC:    int(end.Line) - int(def.Def.Line) + 1,
		Params: len(def.Params),
	}

	fc.Cyclomatic = 1

	maxNesting := 0
	walkStmts(def.Body, 0, &fc.Cyclomatic, &fc.Cognitive, &maxNesting)
	fc.NestingDepth = maxNesting

	return fc
}

func walkStmts(stmts []syntax.Stmt, nesting int, cyclo, cognit, maxNesting *int) {
	if nesting > *maxNesting {
		*maxNesting = nesting
	}

	for _, stmt := range stmts {
		walkStmt(stmt, nesting, cyclo, cognit, maxNesting)
	}
}

func walkStmt(stmt syntax.Stmt, nesting int, cyclo, cognit, maxNesting *int) {
	switch s := stmt.(type) {
	case *syntax.IfStmt:
		*cyclo++
		*cognit += 1 + nesting

		walkExpr(s.Cond, cyclo, cognit)
		walkStmts(s.True, nesting+1, cyclo, cognit, maxNesting)

		if len(s.False) > 0 {
			if _, isElif := s.False[0].(*syntax.IfStmt); !isElif {
				*cognit++
			}
			walkStmts(s.False, nesting+1, cyclo, cognit, maxNesting)
		}

	case *syntax.ForStmt:
		*cyclo++
		*cognit += 1 + nesting
		walkStmts(s.Body, nesting+1, cyclo, cognit, maxNesting)

	case *syntax.WhileStmt:
		*cyclo++
		*cognit += 1 + nesting
		walkExpr(s.Cond, cyclo, cognit)
		walkStmts(s.Body, nesting+1, cyclo, cognit, maxNesting)

	case *syntax.DefStmt:
		walkStmts(s.Body, nesting+1, cyclo, cognit, maxNesting)

	case *syntax.ExprStmt:
		walkExpr(s.X, cyclo, cognit)

	case *syntax.AssignStmt:
		walkExpr(s.RHS, cyclo, cognit)

	case *syntax.ReturnStmt:
		if s.Result != nil {
			walkExpr(s.Result, cyclo, cognit)
		}
	}
}

func walkExpr(expr syntax.Expr, cyclo, cognit *int) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *syntax.BinaryExpr:
		if e.Op == syntax.AND || e.Op == syntax.OR {
			*cyclo++
			*cognit++
		}
		walkExpr(e.X, cyclo, cognit)
		walkExpr(e.Y, cyclo, cognit)

	case *syntax.UnaryExpr:
		walkExpr(e.X, cyclo, cognit)

	case *syntax.CallExpr:
		walkExpr(e.Fn, cyclo, cognit)
		for _, arg := range e.Args {
			walkExpr(arg, cyclo, cognit)
		}

	case *syntax.ListExpr:
		for _, elem := range e.List {
			walkExpr(elem, cyclo, cognit)
		}

	case *syntax.DictExpr:
		for _, entry := range e.List {
			if de, ok := entry.(*syntax.DictEntry); ok {
				walkExpr(de.Key, cyclo, cognit)
				walkExpr(de.Value, cyclo, cognit)
			}
		}

	case *syntax.Comprehension:
		*cyclo++
		*cognit++
		walkExpr(e.Body, cyclo, cognit)
		for _, clause := range e.Clauses {
			switch c := clause.(type) {
			case *syntax.ForClause:
				walkExpr(c.X, cyclo, cognit)
			case *syntax.IfClause:
				*cyclo++
				*cognit++
				walkExpr(c.Cond, cyclo, cognit)
			}
		}

	case *syntax.CondExpr:
		*cyclo++
		*cognit++
		walkExpr(e.Cond, cyclo, cognit)
		walkExpr(e.True, cyclo, cognit)
		walkExpr(e.False, cyclo, cognit)

	case *syntax.IndexExpr:
		walkExpr(e.X, cyclo, cognit)
		walkExpr(e.Y, cyclo, cognit)

	case *syntax.SliceExpr:
		walkExpr(e.X, cyclo, cognit)
		walkExpr(e.Lo, cyclo, cognit)
		walkExpr(e.Hi, cyclo, cognit)
		walkExpr(e.Step, cyclo, cognit)

	case *syntax.DotExpr:
		walkExpr(e.X, cyclo, cognit)

	case *syntax.TupleExpr:
		for _, elem := range e.List {
			walkExpr(elem, cyclo, cognit)
		}

	case *syntax.ParenExpr:
		walkExpr(e.X, cyclo, cognit)

	case *syntax.LambdaExpr:
		*cognit++
		walkExpr(e.Body, cyclo, cognit)
	}
}

// =============================================================================
// STARLARK METRICS
// =============================================================================

func (r *StarlarkParseReceiver) metrics(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("starlark_parse.metrics", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectStarlarkFiles(path)
	if err != nil {
		return nil, fmt.Errorf("starlark_parse.metrics: %w", err)
	}

	var allFiles []starlark.Value
	var totalLOC, totalSLOC, totalComments, totalBlanks int
	var totalFunctions, totalLoads, totalGlobals int

	for _, file := range files {
		result, err := parseStarlarkFile(file)
		if err != nil {
			continue
		}

		totalLOC += result.LOC
		totalSLOC += result.SLOC
		totalComments += result.Comments
		totalBlanks += result.Blanks
		totalFunctions += len(result.Functions)
		totalLoads += len(result.Loads)
		totalGlobals += len(result.Globals)

		allFiles = append(allFiles, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"path":      starlark.String(result.Path),
			"loc":       starlark.MakeInt(result.LOC),
			"sloc":      starlark.MakeInt(result.SLOC),
			"comments":  starlark.MakeInt(result.Comments),
			"blanks":    starlark.MakeInt(result.Blanks),
			"functions": starlark.MakeInt(len(result.Functions)),
			"loads":     starlark.MakeInt(len(result.Loads)),
			"globals":   starlark.MakeInt(len(result.Globals)),
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"files":           starlark.NewList(allFiles),
		"file_count":      starlark.MakeInt(len(files)),
		"total_loc":       starlark.MakeInt(totalLOC),
		"total_sloc":      starlark.MakeInt(totalSLOC),
		"total_comments":  starlark.MakeInt(totalComments),
		"total_blanks":    starlark.MakeInt(totalBlanks),
		"total_functions": starlark.MakeInt(totalFunctions),
		"total_loads":     starlark.MakeInt(totalLoads),
		"total_globals":   starlark.MakeInt(totalGlobals),
	}), nil
}

// =============================================================================
// HELPERS
// =============================================================================

func collectStarlarkFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if strings.HasSuffix(path, ".star") || strings.HasSuffix(path, ".bzl") {
			return []string{path}, nil
		}
		return nil, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".star") || strings.HasSuffix(d.Name(), ".bzl")) {
			files = append(files, p)
		}
		return nil
	})

	return files, err
}
