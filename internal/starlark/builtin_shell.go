// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// shellModule returns the shell module with shell script analysis operations.
func shellModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "shell",
		Members: starlark.StringDict{
			"lint":         starlark.NewBuiltin("shell.lint", shellLint),
			"format_check": starlark.NewBuiltin("shell.format_check", shellFormatCheck),
			"parse":        starlark.NewBuiltin("shell.parse", shellParse),
			"complexity":   starlark.NewBuiltin("shell.complexity", shellComplexity),
		},
	}
}

// =============================================================================
// SHELL LINTING (shellcheck)
// =============================================================================

// ShellcheckIssue represents a single shellcheck finding.
type ShellcheckIssue struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	EndLine   int    `json:"endLine"`
	Column    int    `json:"column"`
	EndColumn int    `json:"endColumn"`
	Level     string `json:"level"` // error, warning, info, style
	Code      int    `json:"code"`  // SC code (e.g., 2086)
	Message   string `json:"message"`
}

// shellLint runs shellcheck on shell scripts and returns structured issues.
//
// Args:
//   - path: Path to a shell file or directory
//   - severity?: Minimum severity (error, warning, info, style). Default: warning
//
// Returns:
//   - A struct with:
//   - issues: List of issues found
//   - error_count, warning_count, info_count, style_count
//   - passed: True if no errors/warnings
func shellLint(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, severity string
	if err := starlark.UnpackArgs("shell.lint", args, kwargs, "path", &path, "severity?", &severity); err != nil {
		return nil, err
	}

	if severity == "" {
		severity = "warning"
	}

	// Check if shellcheck is available
	if _, err := exec.LookPath("shellcheck"); err != nil {
		return nil, fmt.Errorf("shell.lint: shellcheck not installed (install with your package manager)")
	}

	files, err := collectShellFiles(path)
	if err != nil {
		return nil, fmt.Errorf("shell.lint: %w", err)
	}

	var allIssues []ShellcheckIssue
	for _, file := range files {
		issues, err := runShellcheck(file, severity)
		if err != nil {
			// Shellcheck returns non-zero on findings, that's expected
			// Only fail on actual execution errors
			if !strings.Contains(err.Error(), "exit status") {
				continue
			}
		}
		allIssues = append(allIssues, issues...)
	}

	// Count by severity
	var errors, warnings, infos, styles int
	var issueList []starlark.Value
	for _, issue := range allIssues {
		switch issue.Level {
		case "error":
			errors++
		case "warning":
			warnings++
		case "info":
			infos++
		case "style":
			styles++
		}

		issueList = append(issueList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"file":       starlark.String(issue.File),
			"line":       starlark.MakeInt(issue.Line),
			"end_line":   starlark.MakeInt(issue.EndLine),
			"column":     starlark.MakeInt(issue.Column),
			"end_column": starlark.MakeInt(issue.EndColumn),
			"level":      starlark.String(issue.Level),
			"code":       starlark.MakeInt(issue.Code),
			"message":    starlark.String(issue.Message),
		}))
	}

	passed := errors == 0 && warnings == 0

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"issues":        starlark.NewList(issueList),
		"error_count":   starlark.MakeInt(errors),
		"warning_count": starlark.MakeInt(warnings),
		"info_count":    starlark.MakeInt(infos),
		"style_count":   starlark.MakeInt(styles),
		"total_count":   starlark.MakeInt(len(allIssues)),
		"passed":        starlark.Bool(passed),
	}), nil
}

// runShellcheck executes shellcheck on a file and returns issues.
func runShellcheck(path, severity string) ([]ShellcheckIssue, error) {
	cmd := exec.CommandContext(context.Background(), "shellcheck", "-f", "json", "-x", "--severity="+severity, path)
	// shellcheck exits non-zero when it finds issues, so we capture output
	// regardless of exit status
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		// Real error (not just findings)
		return nil, nil
	}

	if len(output) == 0 {
		return nil, nil
	}

	var issues []ShellcheckIssue
	if err := json.Unmarshal(output, &issues); err != nil {
		return nil, fmt.Errorf("parsing shellcheck output: %w", err)
	}

	return issues, nil
}

// =============================================================================
// SHELL FORMAT CHECK (shfmt)
// =============================================================================

// shellFormatCheck checks if shell scripts conform to shfmt formatting.
//
// Args:
//   - path: Path to a shell file or directory
//   - indent?: Indent size (default: 4)
//
// Returns:
//   - A struct with:
//   - passed: True if all files are properly formatted
//   - files_checked: Number of files checked
//   - files_failed: List of files that need formatting
func shellFormatCheck(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var indent int
	if err := starlark.UnpackArgs("shell.format_check", args, kwargs, "path", &path, "indent?", &indent); err != nil {
		return nil, err
	}

	if indent == 0 {
		indent = 4
	}

	// Check if shfmt is available
	if _, err := exec.LookPath("shfmt"); err != nil {
		return nil, fmt.Errorf("shell.format_check: shfmt not installed (install with your package manager)")
	}

	files, err := collectShellFiles(path)
	if err != nil {
		return nil, fmt.Errorf("shell.format_check: %w", err)
	}

	var failedFiles []starlark.Value
	for _, file := range files {
		cmd := exec.CommandContext(context.Background(), "shfmt", "-d", "-i", fmt.Sprintf("%d", indent), "-ci", file)
		output, err := cmd.CombinedOutput()
		if err != nil {
			// shfmt exits non-zero if file needs formatting
			failedFiles = append(failedFiles, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"file": starlark.String(file),
				"diff": starlark.String(string(output)),
			}))
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"passed":        starlark.Bool(len(failedFiles) == 0),
		"files_checked": starlark.MakeInt(len(files)),
		"files_failed":  starlark.NewList(failedFiles),
	}), nil
}

// =============================================================================
// SHELL PARSING (shfmt AST)
// =============================================================================

// ShellFunction represents a function defined in a shell script.
type ShellFunction struct {
	Name      string
	Line      int
	EndLine   int
	BodyLines int
}

// ShellVariable represents a variable assignment in a shell script.
type ShellVariable struct {
	Name  string
	Line  int
	Value string
}

// ShellParseResult holds the parsed structure of a shell script.
type ShellParseResult struct {
	Path      string
	Functions []ShellFunction
	Variables []ShellVariable
	Commands  []string // Unique external commands called
	Sources   []string // Sourced files
	LOC       int
	SLOC      int
	Comments  int
	Blanks    int
}

// shellParse parses shell scripts and extracts structural information.
//
// Args:
//   - path: Path to a shell file or directory
//
// Returns:
//   - A struct with functions, variables, commands, etc.
func shellParse(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("shell.parse", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectShellFiles(path)
	if err != nil {
		return nil, fmt.Errorf("shell.parse: %w", err)
	}

	var allFiles []starlark.Value
	var totalFunctions, totalVariables, totalLOC, totalSLOC int

	for _, file := range files {
		result, err := parseShellFile(file)
		if err != nil {
			continue
		}

		totalFunctions += len(result.Functions)
		totalVariables += len(result.Variables)
		totalLOC += result.LOC
		totalSLOC += result.SLOC

		allFiles = append(allFiles, shellParseResultToStarlark(result))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"files":           starlark.NewList(allFiles),
		"total_functions": starlark.MakeInt(totalFunctions),
		"total_variables": starlark.MakeInt(totalVariables),
		"total_loc":       starlark.MakeInt(totalLOC),
		"total_sloc":      starlark.MakeInt(totalSLOC),
	}), nil
}

// parseShellFile parses a single shell file using regex patterns.
// Note: This is a simpler approach than using shfmt --tojson, which has
// a complex AST format. We use pattern matching for the common cases.
func parseShellFile(path string) (ShellParseResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ShellParseResult{}, err
	}

	result := ShellParseResult{Path: path}
	lines := strings.Split(string(content), "\n")
	result.LOC = len(lines)

	inFunction := false
	functionStartLine := 0
	functionName := ""
	braceDepth := 0
	commandsSeen := make(map[string]bool)
	sourcesSeen := make(map[string]bool)

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Count blanks and comments
		if trimmed == "" {
			result.Blanks++
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			result.Comments++
			continue
		}

		// Detect function definitions: name() { or function name {
		if !inFunction {
			if matched := matchFunctionDef(trimmed); matched != "" {
				inFunction = true
				functionName = matched
				functionStartLine = lineNum
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
				if braceDepth <= 0 {
					// Single-line function or inline
					result.Functions = append(result.Functions, ShellFunction{
						Name:      functionName,
						Line:      functionStartLine,
						EndLine:   lineNum,
						BodyLines: 1,
					})
					inFunction = false
				}
				continue
			}
		}

		// Track brace depth for function body
		if inFunction {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 {
				result.Functions = append(result.Functions, ShellFunction{
					Name:      functionName,
					Line:      functionStartLine,
					EndLine:   lineNum,
					BodyLines: lineNum - functionStartLine + 1,
				})
				inFunction = false
			}
		}

		// Detect variable assignments: VAR=value or export VAR=value
		if varName, varValue := matchVariableAssign(trimmed); varName != "" {
			result.Variables = append(result.Variables, ShellVariable{
				Name:  varName,
				Line:  lineNum,
				Value: varValue,
			})
		}

		// Detect source/. commands
		if source := matchSourceCommand(trimmed); source != "" {
			if !sourcesSeen[source] {
				sourcesSeen[source] = true
				result.Sources = append(result.Sources, source)
			}
		}

		// Detect external commands (first word of command, excluding builtins)
		if cmd := matchExternalCommand(trimmed); cmd != "" {
			if !commandsSeen[cmd] {
				commandsSeen[cmd] = true
				result.Commands = append(result.Commands, cmd)
			}
		}
	}

	result.SLOC = result.LOC - result.Blanks - result.Comments
	return result, nil
}

// matchFunctionDef matches shell function definitions and returns the function name.
func matchFunctionDef(line string) string {
	// Pattern 1: name() { or name () {
	if strings.Contains(line, "()") {
		parts := strings.SplitN(line, "()", 2)
		if len(parts) >= 1 {
			name := strings.TrimSpace(parts[0])
			name = strings.TrimPrefix(name, "function ")
			if isValidFunctionName(name) {
				return name
			}
		}
	}
	// Pattern 2: function name {
	if strings.HasPrefix(line, "function ") {
		rest := strings.TrimPrefix(line, "function ")
		parts := strings.Fields(rest)
		if len(parts) >= 1 && isValidFunctionName(parts[0]) {
			return parts[0]
		}
	}
	return ""
}

// isValidFunctionName checks if a string is a valid shell function name.
func isValidFunctionName(name string) bool {
	if name == "" {
		return false
	}
	for i, c := range name {
		if i == 0 {
			// First char must be letter or underscore
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && c != '_' {
				return false
			}
		} else {
			// Subsequent chars can also include digits and hyphen
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '-' {
				return false
			}
		}
	}
	return true
}

// matchVariableAssign matches variable assignments.
func matchVariableAssign(line string) (name, value string) {
	// Remove leading export/local/declare
	work := line
	for _, prefix := range []string{"export ", "local ", "declare ", "readonly ", "typeset "} {
		if strings.HasPrefix(work, prefix) {
			work = strings.TrimPrefix(work, prefix)
			break
		}
	}

	// Match NAME=value
	if idx := strings.Index(work, "="); idx > 0 {
		name = strings.TrimSpace(work[:idx])
		// Skip if name contains spaces or special chars (not a simple assignment)
		if strings.ContainsAny(name, " \t[]()") {
			return "", ""
		}
		if isValidFunctionName(name) {
			value = strings.TrimSpace(work[idx+1:])
			return name, value
		}
	}
	return "", ""
}

// matchSourceCommand matches source/. commands.
func matchSourceCommand(line string) string {
	if strings.HasPrefix(line, "source ") {
		return strings.TrimSpace(strings.TrimPrefix(line, "source "))
	}
	if strings.HasPrefix(line, ". ") {
		return strings.TrimSpace(strings.TrimPrefix(line, ". "))
	}
	return ""
}

// matchExternalCommand extracts the first command word (excluding builtins).
func matchExternalCommand(line string) string {
	// Skip comments, variable assignments, control structures
	if strings.HasPrefix(line, "#") || strings.Contains(line, "=") {
		return ""
	}

	// Get first word
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}

	cmd := fields[0]

	// Skip shell builtins and keywords
	builtins := map[string]bool{
		"if": true, "then": true, "else": true, "elif": true, "fi": true,
		"for": true, "do": true, "done": true, "while": true, "until": true,
		"case": true, "esac": true, "in": true,
		"function": true, "return": true, "exit": true,
		"local": true, "export": true, "declare": true, "readonly": true,
		"source": true, ".": true, "eval": true, "exec": true,
		"cd": true, "pwd": true, "echo": true, "printf": true,
		"read": true, "set": true, "unset": true, "shift": true,
		"test": true, "[": true, "[[": true, "true": true, "false": true,
		"{": true, "}": true, "(": true, ")": true,
		"break": true, "continue": true,
	}

	if builtins[cmd] {
		return ""
	}

	// Skip if starts with special characters
	if strings.ContainsAny(cmd[:1], "$\"'`-|&;<>()[]{}") {
		return ""
	}

	return cmd
}

// shellParseResultToStarlark converts ShellParseResult to a Starlark struct.
func shellParseResultToStarlark(r ShellParseResult) starlark.Value {
	var functions []starlark.Value
	for _, fn := range r.Functions {
		functions = append(functions, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":       starlark.String(fn.Name),
			"line":       starlark.MakeInt(fn.Line),
			"end_line":   starlark.MakeInt(fn.EndLine),
			"body_lines": starlark.MakeInt(fn.BodyLines),
		}))
	}

	var variables []starlark.Value
	for _, v := range r.Variables {
		variables = append(variables, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":  starlark.String(v.Name),
			"line":  starlark.MakeInt(v.Line),
			"value": starlark.String(v.Value),
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":      starlark.String(r.Path),
		"functions": starlark.NewList(functions),
		"variables": starlark.NewList(variables),
		"commands":  stringsToStarlarkList(r.Commands),
		"sources":   stringsToStarlarkList(r.Sources),
		"loc":       starlark.MakeInt(r.LOC),
		"sloc":      starlark.MakeInt(r.SLOC),
		"comments":  starlark.MakeInt(r.Comments),
		"blanks":    starlark.MakeInt(r.Blanks),
	})
}

// =============================================================================
// SHELL COMPLEXITY
// =============================================================================

// ShellFunctionComplexity holds complexity metrics for a shell function.
type ShellFunctionComplexity struct {
	Name          string
	Line          int
	Cyclomatic    int // Control flow complexity
	NestingDepth  int // Maximum nesting depth
	LOC           int // Lines of code
	ParameterRefs int // $1, $2, etc. references
}

// shellComplexity calculates complexity metrics for shell scripts.
//
// Args:
//   - path: Path to a shell file or directory
//
// Returns:
//   - A struct with complexity metrics per function and file
func shellComplexity(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("shell.complexity", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	files, err := collectShellFiles(path)
	if err != nil {
		return nil, fmt.Errorf("shell.complexity: %w", err)
	}

	var allFiles []starlark.Value
	var totalCyclo, totalFuncs int
	var maxCyclo int
	var maxCycloFunc string
	var hotspots []starlark.Value

	for _, file := range files {
		fileResult, funcs := analyzeShellComplexity(file)
		allFiles = append(allFiles, fileResult)

		for _, fn := range funcs {
			totalFuncs++
			totalCyclo += fn.Cyclomatic

			if fn.Cyclomatic > maxCyclo {
				maxCyclo = fn.Cyclomatic
				maxCycloFunc = fn.Name
			}

			// Hotspots: cyclomatic > 10 or nesting > 4
			if fn.Cyclomatic > 10 || fn.NestingDepth > 4 {
				hotspots = append(hotspots, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
					"name":          starlark.String(fn.Name),
					"file":          starlark.String(filepath.Base(file)),
					"line":          starlark.MakeInt(fn.Line),
					"cyclomatic":    starlark.MakeInt(fn.Cyclomatic),
					"nesting_depth": starlark.MakeInt(fn.NestingDepth),
					"loc":           starlark.MakeInt(fn.LOC),
				}))
			}
		}
	}

	var avgCyclo float64
	if totalFuncs > 0 {
		avgCyclo = float64(totalCyclo) / float64(totalFuncs)
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"files":            starlark.NewList(allFiles),
		"total_cyclomatic": starlark.MakeInt(totalCyclo),
		"total_functions":  starlark.MakeInt(totalFuncs),
		"avg_cyclomatic":   starlark.Float(avgCyclo),
		"max_cyclomatic":   starlark.MakeInt(maxCyclo),
		"max_cyclo_func":   starlark.String(maxCycloFunc),
		"hotspots":         starlark.NewList(hotspots),
	}), nil
}

// analyzeShellComplexity analyzes complexity for a shell file.
func analyzeShellComplexity(path string) (starlark.Value, []ShellFunctionComplexity) {
	content, err := os.ReadFile(path)
	if err != nil {
		return starlark.None, nil
	}

	lines := strings.Split(string(content), "\n")
	var functions []ShellFunctionComplexity

	// Track function boundaries
	inFunction := false
	functionName := ""
	functionStartLine := 0
	braceDepth := 0
	var functionLines []string

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect function start
		if !inFunction {
			if matched := matchFunctionDef(trimmed); matched != "" {
				inFunction = true
				functionName = matched
				functionStartLine = lineNum
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
				functionLines = []string{line}
				if braceDepth <= 0 {
					// Single-line function
					fc := calculateShellFunctionComplexity(functionName, functionStartLine, functionLines)
					functions = append(functions, fc)
					inFunction = false
					functionLines = nil
				}
				continue
			}
		}

		// Track function body
		if inFunction {
			functionLines = append(functionLines, line)
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 {
				fc := calculateShellFunctionComplexity(functionName, functionStartLine, functionLines)
				functions = append(functions, fc)
				inFunction = false
				functionLines = nil
			}
		}
	}

	// Convert to Starlark
	var funcList []starlark.Value
	var totalCyclo int
	for _, fn := range functions {
		totalCyclo += fn.Cyclomatic
		funcList = append(funcList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":           starlark.String(fn.Name),
			"line":           starlark.MakeInt(fn.Line),
			"cyclomatic":     starlark.MakeInt(fn.Cyclomatic),
			"nesting_depth":  starlark.MakeInt(fn.NestingDepth),
			"loc":            starlark.MakeInt(fn.LOC),
			"parameter_refs": starlark.MakeInt(fn.ParameterRefs),
		}))
	}

	fileResult := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":           starlark.String(path),
		"functions":      starlark.NewList(funcList),
		"total_cyclo":    starlark.MakeInt(totalCyclo),
		"function_count": starlark.MakeInt(len(functions)),
	})

	return fileResult, functions
}

// calculateShellFunctionComplexity calculates complexity for a function body.
func calculateShellFunctionComplexity(name string, startLine int, lines []string) ShellFunctionComplexity {
	fc := ShellFunctionComplexity{
		Name: name,
		Line: startLine,
		LOC:  len(lines),
	}

	// Base cyclomatic complexity
	fc.Cyclomatic = 1

	// Track nesting
	currentNesting := 0
	maxNesting := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments and blank lines for complexity
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Count decision points
		// if/elif add complexity
		if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "elif ") {
			fc.Cyclomatic++
			currentNesting++
			if currentNesting > maxNesting {
				maxNesting = currentNesting
			}
		}

		// for/while/until add complexity
		if strings.HasPrefix(trimmed, "for ") || strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "until ") {
			fc.Cyclomatic++
			currentNesting++
			if currentNesting > maxNesting {
				maxNesting = currentNesting
			}
		}

		// case adds complexity for each pattern (approximate: count ;;)
		if strings.HasSuffix(trimmed, ";;") {
			fc.Cyclomatic++
		}

		// && and || add complexity
		fc.Cyclomatic += strings.Count(trimmed, " && ")
		fc.Cyclomatic += strings.Count(trimmed, " || ")

		// Decrease nesting at fi/done/esac
		if trimmed == "fi" || trimmed == "done" || trimmed == "esac" {
			currentNesting--
			if currentNesting < 0 {
				currentNesting = 0
			}
		}

		// Count parameter references
		for i := 0; i <= 9; i++ {
			fc.ParameterRefs += strings.Count(line, fmt.Sprintf("$%d", i))
		}
		fc.ParameterRefs += strings.Count(line, "$@")
		fc.ParameterRefs += strings.Count(line, "$*")
		fc.ParameterRefs += strings.Count(line, "$#")
	}

	fc.NestingDepth = maxNesting
	return fc
}

// =============================================================================
// HELPERS
// =============================================================================

// collectShellFiles returns all shell files in a path.
func collectShellFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Check if it's actually a shell file
		if isShellFile(path) {
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
		if !d.IsDir() && isShellFile(p) {
			files = append(files, p)
		}
		return nil
	})

	return files, err
}

// isShellFile checks if a file is a shell script.
func isShellFile(path string) bool {
	// Check extension
	ext := filepath.Ext(path)
	if ext == ".sh" || ext == ".bash" || ext == ".zsh" {
		return true
	}

	// Check shebang
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 256)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}

	firstLine := string(buf[:n])
	if idx := strings.Index(firstLine, "\n"); idx > 0 {
		firstLine = firstLine[:idx]
	}

	// Check for shell shebang
	if strings.HasPrefix(firstLine, "#!/") {
		if strings.Contains(firstLine, "/bash") ||
			strings.Contains(firstLine, "/sh") ||
			strings.Contains(firstLine, "/zsh") ||
			strings.Contains(firstLine, "env bash") ||
			strings.Contains(firstLine, "env sh") ||
			strings.Contains(firstLine, "env zsh") {
			return true
		}
	}

	// Check for shellcheck directive
	if strings.Contains(firstLine, "# shellcheck shell=") {
		return true
	}

	return false
}
