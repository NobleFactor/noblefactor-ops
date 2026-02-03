// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package starlark

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// lintModule returns the lint module with static analysis operations.
func lintModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "lint",
		Members: starlark.StringDict{
			"go": starlark.NewBuiltin("lint.go", lintGo),
		},
	}
}

// =============================================================================
// GO LINTING (golangci-lint)
// =============================================================================

// GolangCILintOutput represents the JSON output from golangci-lint.
type GolangCILintOutput struct {
	Issues []GolangCILintIssue `json:"Issues"`
}

// GolangCILintIssue represents a single golangci-lint finding.
type GolangCILintIssue struct {
	FromLinter  string                `json:"FromLinter"`
	Text        string                `json:"Text"`
	Severity    string                `json:"Severity"`
	SourceLines []string              `json:"SourceLines"`
	Pos         GolangCILintPosition  `json:"Pos"`
}

// GolangCILintPosition represents the position of an issue.
type GolangCILintPosition struct {
	Filename string `json:"Filename"`
	Offset   int    `json:"Offset"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

// lintGo runs golangci-lint on Go code and returns structured issues.
//
// Args:
//   - path: Path to lint (default: "./...")
//   - config: Path to config file (optional, uses .golangci.yaml by default)
//
// Returns:
//   - A struct with:
//   - issues: List of issues found
//   - error_count, warning_count
//   - passed: True if no errors/warnings
//   - total_count: Total number of issues
func lintGo(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, config string
	if err := starlark.UnpackArgs("lint.go", args, kwargs, "path?", &path, "config?", &config); err != nil {
		return nil, err
	}

	if path == "" {
		path = "./..."
	}

	// Check if golangci-lint is available
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return nil, fmt.Errorf("lint.go: golangci-lint not installed (go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
	}

	// Build command arguments
	// golangci-lint v2 uses --output.json.path for JSON output
	cmdArgs := []string{"run", "--output.json.path", "stdout"}
	if config != "" {
		cmdArgs = append(cmdArgs, "--config="+config)
	}
	cmdArgs = append(cmdArgs, path)

	// Run golangci-lint
	cmd := exec.CommandContext(context.Background(), "golangci-lint", cmdArgs...)
	output, err := cmd.Output()

	// golangci-lint exits non-zero when issues are found, so we need to handle that
	// We only fail on actual execution errors (not finding issues)
	var issues []GolangCILintIssue
	if len(output) > 0 {
		var lintOutput GolangCILintOutput
		if jsonErr := json.Unmarshal(output, &lintOutput); jsonErr == nil {
			issues = lintOutput.Issues
		}
	}

	// If there was an error but no output, check if it's a real error
	if err != nil && len(output) == 0 {
		// Try to get stderr for better error message
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if stderr != "" {
				return nil, fmt.Errorf("lint.go: golangci-lint failed: %s", strings.TrimSpace(stderr))
			}
		}
		// Could be that the tool just found issues and exited non-zero
		// but produced no JSON (shouldn't happen with --out-format=json)
	}

	// Count by severity
	var errors, warnings int
	var issueList []starlark.Value
	for _, issue := range issues {
		severity := issue.Severity
		if severity == "" {
			severity = "warning" // Default severity
		}

		switch severity {
		case "error":
			errors++
		default:
			warnings++
		}

		// Convert source lines to Starlark list
		var sourceLines []starlark.Value
		for _, line := range issue.SourceLines {
			sourceLines = append(sourceLines, starlark.String(line))
		}

		issueList = append(issueList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"file":         starlark.String(issue.Pos.Filename),
			"line":         starlark.MakeInt(issue.Pos.Line),
			"column":       starlark.MakeInt(issue.Pos.Column),
			"message":      starlark.String(issue.Text),
			"linter":       starlark.String(issue.FromLinter),
			"severity":     starlark.String(severity),
			"source_lines": starlark.NewList(sourceLines),
		}))
	}

	passed := errors == 0 && warnings == 0

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"issues":        starlark.NewList(issueList),
		"error_count":   starlark.MakeInt(errors),
		"warning_count": starlark.MakeInt(warnings),
		"total_count":   starlark.MakeInt(len(issues)),
		"passed":        starlark.Bool(passed),
	}), nil
}

