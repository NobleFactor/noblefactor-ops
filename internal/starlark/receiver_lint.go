// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/NobleFactor/noblefactor-ops/internal/cli"
)

// LintReceiver provides static analysis operations.
// Implements starlark.Value and starlark.HasAttrs.
type LintReceiver struct {
	BaseReceiver
}

// NewLintReceiver creates a new LintReceiver.
func NewLintReceiver() *LintReceiver {
	return &LintReceiver{BaseReceiver: NewBaseReceiver("lint")}
}

// Attr implements starlark.HasAttrs.
func (r *LintReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "go":
		return MakeAttr("lint.go", r.lintGo), nil
	case "shell":
		return MakeAttr("lint.shell", r.lintShell), nil
	case "markdown":
		return MakeAttr("lint.markdown", r.lintMarkdown), nil
	case "ensure_tools":
		return MakeAttr("lint.ensure_tools", r.ensureTools), nil
	default:
		return nil, NoSuchAttrError("lint", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *LintReceiver) AttrNames() []string {
	return []string{"ensure_tools", "go", "markdown", "shell"}
}

// =============================================================================
// DEFAULT CONFIGURATION
// =============================================================================

const defaultGolangciConfig = `# SPDX-License-Identifier: MIT
# golangci-lint v2 configuration for NobleFactor Go projects

version: "2"

run:
  timeout: 5m
  modules-download-mode: readonly

formatters:
  enable:
    - gofmt
    - goimports

linters:
  default: none
  enable:
    - errcheck
    - govet
    - staticcheck
    - ineffassign
    - unused
    - gocyclo
    - gocognit
    - unparam
    - unconvert
    - gocritic
    - gosec
    - misspell
    - nilerr
    - bodyclose
    - durationcheck
    - errorlint
    - noctx
    - revive
    - whitespace

  settings:
    gocyclo:
      min-complexity: 15
    gocognit:
      min-complexity: 20
    gocritic:
      enabled-tags:
        - diagnostic
        - performance
        - style
      disabled-checks:
        - whyNoLint
        - hugeParam
    revive:
      rules:
        - name: blank-imports
        - name: context-as-argument
        - name: context-keys-type
        - name: dot-imports
        - name: error-return
        - name: error-strings
        - name: error-naming
        - name: exported
        - name: increment-decrement
        - name: indent-error-flow
        - name: package-comments
        - name: range
        - name: receiver-naming
        - name: time-naming
        - name: unexported-return
        - name: var-declaration
        - name: var-naming
    gosec:
      excludes:
        - G104
        - G304
    misspell:
      locale: US
    errcheck:
      check-type-assertions: true
      check-blank: true
      exclude-functions:
        - io.Copy
        - (*os.File).Close
        - (io.Closer).Close

  exclusions:
    generated: lax
    rules:
      - path: _test\.go
        linters:
          - gocyclo
          - gocognit
          - errcheck
          - gosec
      - path: \.pb\.go$
        linters:
          - all
    paths:
      - vendor
      - testdata
      - .git

output:
  formats:
    text:
      path: stdout
  print-issued-lines: true
  print-linter-name: true
  sort-results: true
`

// =============================================================================
// TOOL MANAGEMENT
// =============================================================================

type ToolInfo struct {
	Name        string
	Binary      string
	InstallCmd  string
	InstallNote string
}

var requiredTools = []ToolInfo{
	{
		Name:       "golangci-lint",
		Binary:     "golangci-lint",
		InstallCmd: "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin",
	},
	{
		Name:       "shellcheck",
		Binary:     "shellcheck",
		InstallCmd: getShellcheckInstallCmd(),
	},
	{
		Name:       "shfmt",
		Binary:     "shfmt",
		InstallCmd: "go install mvdan.cc/sh/v3/cmd/shfmt@latest",
	},
	{
		Name:       "markdownlint-cli2",
		Binary:     "markdownlint-cli2",
		InstallCmd: getMarkdownlintInstallCmd(),
	},
}

func getShellcheckInstallCmd() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install shellcheck"
	case "linux":
		return "sudo apt-get install shellcheck"
	default:
		return "See https://github.com/koalaman/shellcheck#installing"
	}
}

func getMarkdownlintInstallCmd() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install markdownlint-cli2"
	default:
		return "npm install -g markdownlint-cli2"
	}
}

func checkTool(binary string) string {
	path, err := exec.LookPath(binary)
	if err != nil {
		return ""
	}
	return path
}

func (r *LintReceiver) ensureTools(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("lint.ensure_tools", args, kwargs); err != nil {
		return nil, err
	}

	var toolStatus []starlark.Value
	allInstalled := true
	var missingTools []ToolInfo

	for _, tool := range requiredTools {
		path := checkTool(tool.Binary)
		installed := path != ""
		if !installed {
			allInstalled = false
			missingTools = append(missingTools, tool)
		}

		toolStatus = append(toolStatus, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":        starlark.String(tool.Name),
			"installed":   starlark.Bool(installed),
			"path":        starlark.String(path),
			"install_cmd": starlark.String(tool.InstallCmd),
		}))
	}

	var instructions []starlark.Value
	for _, tool := range missingTools {
		instructions = append(instructions, starlark.String(tool.InstallCmd))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"all_installed": starlark.Bool(allInstalled),
		"tools":         starlark.NewList(toolStatus),
		"install_cmds":  starlark.NewList(instructions),
	}), nil
}

func ensureGolangciConfig() (string, bool, error) {
	configPath := ".golangci.yaml"

	if _, err := os.Stat(configPath); err == nil {
		return configPath, false, nil
	}
	if _, err := os.Stat(".golangci.yml"); err == nil {
		return ".golangci.yml", false, nil
	}

	if err := os.WriteFile(configPath, []byte(defaultGolangciConfig), 0o644); err != nil {
		return "", false, fmt.Errorf("creating %s: %w", configPath, err)
	}

	return configPath, true, nil
}

// =============================================================================
// GO LINTING
// =============================================================================

type GolangCILintOutput struct {
	Issues []GolangCILintIssue `json:"Issues"`
}

type GolangCILintIssue struct {
	FromLinter  string               `json:"FromLinter"`
	Text        string               `json:"Text"`
	Severity    string               `json:"Severity"`
	SourceLines []string             `json:"SourceLines"`
	Pos         GolangCILintPosition `json:"Pos"`
}

type GolangCILintPosition struct {
	Filename string `json:"Filename"`
	Offset   int    `json:"Offset"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

func checkModTidy() (bool, string) {
	tidyCmd := exec.Command("go", "mod", "tidy")
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("go mod tidy failed: %s\n%s", err, string(output))
	}

	diffCmd := exec.Command("git", "diff", "--exit-code", "go.mod", "go.sum")
	if output, err := diffCmd.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("go.mod or go.sum not tidy:\n%s", string(output))
	}

	return true, ""
}

func (r *LintReceiver) lintGo(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, config string
	var skipModTidy bool
	if err := starlark.UnpackArgs("lint.go", args, kwargs, "path?", &path, "config?", &config, "skip_mod_tidy?", &skipModTidy); err != nil {
		return nil, err
	}

	if path == "" {
		path = "./..."
	}

	modTidyPassed := true
	modTidyDetails := ""
	if !skipModTidy {
		modTidyPassed, modTidyDetails = checkModTidy()
	}

	if checkTool("golangci-lint") == "" {
		return nil, fmt.Errorf("lint.go: golangci-lint not installed\n  Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
	}

	var configCreated bool
	if config == "" {
		var err error
		config, configCreated, err = ensureGolangciConfig()
		if err != nil {
			return nil, fmt.Errorf("lint.go: %w", err)
		}
		if configCreated {
			cli.Note("created %s with NobleFactor defaults", config)
		}
	}

	cmdArgs := []string{"run", "--output.json.path", "stdout"}
	if config != "" {
		if !filepath.IsAbs(config) {
			if absConfig, err := filepath.Abs(config); err == nil {
				config = absConfig
			}
		}
		cmdArgs = append(cmdArgs, "--config="+config)
	}
	cmdArgs = append(cmdArgs, path)

	cmd := exec.CommandContext(context.Background(), "golangci-lint", cmdArgs...)
	output, err := cmd.Output()

	var issues []GolangCILintIssue
	if len(output) > 0 {
		var lintOutput GolangCILintOutput
		if jsonErr := json.Unmarshal(output, &lintOutput); jsonErr == nil {
			issues = lintOutput.Issues
		}
	}

	if err != nil && len(output) == 0 {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if stderr != "" {
				return nil, fmt.Errorf("lint.go: golangci-lint failed: %s", strings.TrimSpace(stderr))
			}
		}
	}

	var errors, warnings int
	var issueList []starlark.Value
	for _, issue := range issues {
		severity := issue.Severity
		if severity == "" {
			severity = "warning"
		}

		switch severity {
		case "error":
			errors++
		default:
			warnings++
		}

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

	lintPassed := errors == 0 && warnings == 0
	passed := lintPassed && modTidyPassed

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"issues":           starlark.NewList(issueList),
		"error_count":      starlark.MakeInt(errors),
		"warning_count":    starlark.MakeInt(warnings),
		"total_count":      starlark.MakeInt(len(issues)),
		"passed":           starlark.Bool(passed),
		"lint_passed":      starlark.Bool(lintPassed),
		"config_created":   starlark.Bool(configCreated),
		"mod_tidy_passed":  starlark.Bool(modTidyPassed),
		"mod_tidy_details": starlark.String(modTidyDetails),
	}), nil
}

// =============================================================================
// SHELL LINTING
// =============================================================================

func (r *LintReceiver) lintShell(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, severity string
	var indent int
	if err := starlark.UnpackArgs("lint.shell", args, kwargs, "path?", &path, "severity?", &severity, "indent?", &indent); err != nil {
		return nil, err
	}

	if path == "" {
		path = "."
	}
	if severity == "" {
		severity = "warning"
	}
	if indent == 0 {
		indent = 4
	}

	shellcheckPath := checkTool("shellcheck")
	shfmtPath := checkTool("shfmt")

	if shellcheckPath == "" {
		return nil, fmt.Errorf("lint.shell: shellcheck not installed\n  Install: %s", getShellcheckInstallCmd())
	}
	if shfmtPath == "" {
		return nil, fmt.Errorf("lint.shell: shfmt not installed\n  Install: go install mvdan.cc/sh/v3/cmd/shfmt@latest")
	}

	files, err := collectShellFiles(path)
	if err != nil {
		return nil, fmt.Errorf("lint.shell: %w", err)
	}

	if len(files) == 0 {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"issues":        starlark.NewList(nil),
			"format_issues": starlark.NewList(nil),
			"error_count":   starlark.MakeInt(0),
			"warning_count": starlark.MakeInt(0),
			"files_checked": starlark.MakeInt(0),
			"lint_passed":   starlark.Bool(true),
			"format_passed": starlark.Bool(true),
			"passed":        starlark.Bool(true),
		}), nil
	}

	var allIssues []starlark.Value
	var errors, warnings int

	for _, file := range files {
		issues, err := runShellcheckForLint(file, severity)
		if err != nil {
			continue
		}
		for _, issue := range issues {
			switch issue.Level {
			case "error":
				errors++
			case "warning":
				warnings++
			}
			allIssues = append(allIssues, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"file":    starlark.String(issue.File),
				"line":    starlark.MakeInt(issue.Line),
				"column":  starlark.MakeInt(issue.Column),
				"level":   starlark.String(issue.Level),
				"code":    starlark.MakeInt(issue.Code),
				"message": starlark.String(issue.Message),
			}))
		}
	}

	var formatIssues []starlark.Value
	for _, file := range files {
		cmd := exec.CommandContext(context.Background(), "shfmt", "-d", "-i", fmt.Sprintf("%d", indent), "-ci", file)
		output, err := cmd.CombinedOutput()
		if err != nil {
			formatIssues = append(formatIssues, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"file": starlark.String(file),
				"diff": starlark.String(string(output)),
			}))
		}
	}

	lintPassed := errors == 0 && warnings == 0
	formatPassed := len(formatIssues) == 0

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"issues":        starlark.NewList(allIssues),
		"format_issues": starlark.NewList(formatIssues),
		"error_count":   starlark.MakeInt(errors),
		"warning_count": starlark.MakeInt(warnings),
		"files_checked": starlark.MakeInt(len(files)),
		"lint_passed":   starlark.Bool(lintPassed),
		"format_passed": starlark.Bool(formatPassed),
		"passed":        starlark.Bool(lintPassed && formatPassed),
	}), nil
}

func runShellcheckForLint(path, severity string) ([]ShellcheckIssue, error) {
	cmd := exec.CommandContext(context.Background(), "shellcheck", "-f", "json", "-x", "--severity="+severity, path)
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
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
// MARKDOWN LINTING
// =============================================================================

type MarkdownLintIssue struct {
	File     string
	Line     int
	Rule     string
	Message  string
	Severity string
}

type FrontmatterIssue struct {
	File    string
	Message string
}

func (r *LintReceiver) lintMarkdown(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var fix bool
	if err := starlark.UnpackArgs("lint.markdown", args, kwargs, "path?", &path, "fix?", &fix); err != nil {
		return nil, err
	}

	if path == "" {
		path = "."
	}

	if checkTool("markdownlint-cli2") == "" {
		return nil, fmt.Errorf("lint.markdown: markdownlint-cli2 not installed\n  Install: %s", getMarkdownlintInstallCmd())
	}

	mdFiles, err := findMarkdownFiles(path)
	if err != nil {
		return nil, fmt.Errorf("lint.markdown: finding files: %w", err)
	}

	if len(mdFiles) == 0 {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"issues":             starlark.NewList(nil),
			"frontmatter_issues": starlark.NewList(nil),
			"files_checked":      starlark.MakeInt(0),
			"issue_count":        starlark.MakeInt(0),
			"lint_passed":        starlark.Bool(true),
			"frontmatter_passed": starlark.Bool(true),
			"passed":             starlark.Bool(true),
		}), nil
	}

	lintIssues, err := runMarkdownLint(path, fix)
	if err != nil {
		return nil, fmt.Errorf("lint.markdown: %w", err)
	}

	frontmatterIssues, err := checkFrontmatter(mdFiles)
	if err != nil {
		return nil, fmt.Errorf("lint.markdown: checking frontmatter: %w", err)
	}

	var issueList []starlark.Value
	for _, issue := range lintIssues {
		issueList = append(issueList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"file":     starlark.String(issue.File),
			"line":     starlark.MakeInt(issue.Line),
			"rule":     starlark.String(issue.Rule),
			"message":  starlark.String(issue.Message),
			"severity": starlark.String(issue.Severity),
		}))
	}

	var fmIssueList []starlark.Value
	for _, issue := range frontmatterIssues {
		fmIssueList = append(fmIssueList, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"file":    starlark.String(issue.File),
			"message": starlark.String(issue.Message),
		}))
	}

	lintPassed := len(lintIssues) == 0
	fmPassed := len(frontmatterIssues) == 0

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"issues":             starlark.NewList(issueList),
		"frontmatter_issues": starlark.NewList(fmIssueList),
		"files_checked":      starlark.MakeInt(len(mdFiles)),
		"issue_count":        starlark.MakeInt(len(lintIssues)),
		"lint_passed":        starlark.Bool(lintPassed),
		"frontmatter_passed": starlark.Bool(fmPassed),
		"passed":             starlark.Bool(lintPassed && fmPassed),
	}), nil
}

func findMarkdownFiles(path string) ([]string, error) {
	var files []string
	excludeDirs := map[string]bool{
		"node_modules": true,
		"vendor":       true,
		".git":         true,
	}

	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && excludeDirs[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			files = append(files, p)
		}
		return nil
	})

	return files, err
}

func runMarkdownLint(path string, fix bool) ([]MarkdownLintIssue, error) {
	cmdArgs := []string{path}
	if fix {
		cmdArgs = append(cmdArgs, "--fix")
	}

	cmd := exec.CommandContext(context.Background(), "markdownlint-cli2", cmdArgs...)
	output, err := cmd.CombinedOutput()

	issues := parseMarkdownLintOutput(string(output))

	if err != nil && len(issues) == 0 && len(output) > 0 {
		return nil, fmt.Errorf("markdownlint-cli2 failed: %s", strings.TrimSpace(string(output)))
	}

	return issues, nil
}

func parseMarkdownLintOutput(output string) []MarkdownLintIssue {
	var issues []MarkdownLintIssue

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Finding:") ||
			strings.HasPrefix(line, "Linting:") ||
			strings.HasPrefix(line, "Summary:") ||
			strings.HasPrefix(line, "markdownlint-cli2") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		locParts := strings.SplitN(parts[0], ":", 3)
		if len(locParts) < 2 {
			continue
		}

		file := locParts[0]
		lineNum := 0
		n, _ := fmt.Sscanf(locParts[1], "%d", &lineNum)
		if n == 0 || lineNum == 0 {
			continue
		}

		rest := parts[1]
		severity := "warning"
		if strings.HasPrefix(rest, "error ") {
			severity = "error"
			rest = rest[6:]
		} else if strings.HasPrefix(rest, "warning ") {
			rest = rest[8:]
		}

		msgParts := strings.SplitN(rest, " ", 2)
		rule := msgParts[0]
		message := ""
		if len(msgParts) > 1 {
			message = msgParts[1]
		}

		issues = append(issues, MarkdownLintIssue{
			File:     file,
			Line:     lineNum,
			Rule:     rule,
			Message:  message,
			Severity: severity,
		})
	}

	return issues
}

func checkFrontmatter(files []string) ([]FrontmatterIssue, error) {
	var issues []FrontmatterIssue

	cfg, err := loadFrontmatterConfig()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		fileIssues, err := validateFileFrontmatter(file, cfg)
		if err != nil {
			return nil, fmt.Errorf("validating %s: %w", file, err)
		}
		issues = append(issues, fileIssues...)
	}

	return issues, nil
}

type frontmatterConfig struct {
	Required []string
	Optional []string
}

func loadFrontmatterConfig() (*frontmatterConfig, error) {
	return &frontmatterConfig{
		Required: []string{"title", "description"},
		Optional: []string{},
	}, nil
}

func validateFileFrontmatter(file string, cfg *frontmatterConfig) ([]FrontmatterIssue, error) {
	var issues []FrontmatterIssue

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	content := string(data)

	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		issues = append(issues, FrontmatterIssue{
			File:    file,
			Message: "missing frontmatter (must start with ---)",
		})
		return issues, nil
	}

	endIdx := strings.Index(content[4:], "\n---")
	if endIdx == -1 {
		issues = append(issues, FrontmatterIssue{
			File:    file,
			Message: "malformed frontmatter (missing closing ---)",
		})
		return issues, nil
	}

	frontmatter := content[4 : 4+endIdx]

	for _, field := range cfg.Required {
		if !containsFrontmatterField(frontmatter, field) {
			issues = append(issues, FrontmatterIssue{
				File:    file,
				Message: fmt.Sprintf("missing required frontmatter field: %s", field),
			})
		}
	}

	return issues, nil
}

func containsFrontmatterField(frontmatter, field string) bool {
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+":") {
			return true
		}
	}
	return false
}
