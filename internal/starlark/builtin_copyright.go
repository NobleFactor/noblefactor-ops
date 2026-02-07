// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// copyrightModule returns the copyright module for header checking/fixing.
func copyrightModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "copyright",
		Members: starlark.StringDict{
			"check":          starlark.NewBuiltin("copyright.check", copyrightCheck),
			"fix":            starlark.NewBuiltin("copyright.fix", copyrightFix),
			"detect_license": starlark.NewBuiltin("copyright.detect_license", copyrightDetectLicense),
		},
	}
}

// CopyrightPattern holds the match and replace patterns for a language.
type CopyrightPattern struct {
	Match   string // Regex to match existing headers
	Replace string // Canonical form to use
}

// langFromExt returns the language key based on file extension.
func langFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".star":
		return "star"
	case ".sh", ".bash":
		return "shell"
	default:
		return ""
	}
}

// buildCanonicalHeader substitutes placeholders in the replace pattern.
func buildCanonicalHeader(pattern, license, holder string) string {
	result := pattern
	result = strings.ReplaceAll(result, "{license}", license)
	result = strings.ReplaceAll(result, "{holder}", holder)
	return strings.TrimSpace(result)
}

// extractHeader extracts the copyright header from file content.
// Returns the header lines and whether a header was found.
func extractHeader(content, lang string) (string, bool) {
	lines := strings.Split(content, "\n")
	var headerLines []string

	commentPrefix := "//"
	if lang == "star" || lang == "shell" {
		commentPrefix = "#"
	}

	// Skip shebang for shell scripts
	startIdx := 0
	if lang == "shell" && len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		startIdx = 1
		// Skip blank line after shebang
		if len(lines) > 1 && strings.TrimSpace(lines[1]) == "" {
			startIdx = 2
		}
	}

	// Collect comment lines at the start
	for i := startIdx; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue // Skip blank lines within header block
		}
		if strings.HasPrefix(line, commentPrefix) {
			headerLines = append(headerLines, line)
		} else {
			break // Non-comment line ends the header
		}
		// Stop after we have the SPDX + Copyright lines
		if len(headerLines) >= 2 {
			break
		}
	}

	if len(headerLines) == 0 {
		return "", false
	}

	return strings.Join(headerLines, "\n"), true
}

// copyrightCheck checks files for correct copyright headers.
//
// Args:
//   - paths: List of file paths to check
//   - license: SPDX license identifier
//   - holder: Copyright holder name
//   - patterns: Dict of lang -> struct{match, replace}
//
// Returns a struct with:
//   - issues: List of files with problems
//   - passed: True if no issues found
func copyrightCheck(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pathsList *starlark.List
	var license, holder string
	var patternsDict *starlark.Dict

	if err := starlark.UnpackArgs("copyright.check", args, kwargs,
		"paths", &pathsList,
		"license", &license,
		"holder", &holder,
		"patterns", &patternsDict,
	); err != nil {
		return nil, err
	}

	// Convert patterns dict to Go map
	patterns := make(map[string]CopyrightPattern)
	for _, item := range patternsDict.Items() {
		key, _ := starlark.AsString(item[0])
		patternStruct := item[1].(*starlarkstruct.Struct)
		matchVal, _ := patternStruct.Attr("match")
		replaceVal, _ := patternStruct.Attr("replace")
		match, _ := starlark.AsString(matchVal)
		replace, _ := starlark.AsString(replaceVal)
		patterns[key] = CopyrightPattern{Match: match, Replace: replace}
	}

	var issues []starlark.Value

	iter := pathsList.Iterate()
	defer iter.Done()
	var pathVal starlark.Value
	for iter.Next(&pathVal) {
		path, _ := starlark.AsString(pathVal)

		lang := langFromExt(path)
		if lang == "" {
			continue // Skip unsupported file types
		}

		pattern, ok := patterns[lang]
		if !ok {
			continue // No pattern for this language
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		canonical := buildCanonicalHeader(pattern.Replace, license, holder)
		existing, found := extractHeader(string(content), lang)

		// Check: header must match canonical form exactly
		if !found {
			issues = append(issues, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"file":    starlark.String(path),
				"type":    starlark.String("missing"),
				"message": starlark.String("missing copyright header"),
			}))
		} else if strings.TrimSpace(existing) != canonical {
			issues = append(issues, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"file":    starlark.String(path),
				"type":    starlark.String("mismatch"),
				"message": starlark.String("copyright header does not match canonical form"),
			}))
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"issues": starlark.NewList(issues),
		"passed": starlark.Bool(len(issues) == 0),
		"count":  starlark.MakeInt(len(issues)),
	}), nil
}

// copyrightFix adds or updates copyright headers in files.
//
// Args:
//   - paths: List of file paths to fix
//   - license: SPDX license identifier
//   - holder: Copyright holder name
//   - patterns: Dict of lang -> struct{match, replace}
//   - dry_run: If true, don't actually modify files
//
// Returns a struct with:
//   - fixed: List of files that were fixed
//   - count: Number of files fixed
func copyrightFix(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pathsList *starlark.List
	var license, holder string
	var patternsDict *starlark.Dict
	var dryRun bool

	if err := starlark.UnpackArgs("copyright.fix", args, kwargs,
		"paths", &pathsList,
		"license", &license,
		"holder", &holder,
		"patterns", &patternsDict,
		"dry_run?", &dryRun,
	); err != nil {
		return nil, err
	}

	// Convert patterns dict to Go map
	patterns := make(map[string]CopyrightPattern)
	for _, item := range patternsDict.Items() {
		key, _ := starlark.AsString(item[0])
		patternStruct := item[1].(*starlarkstruct.Struct)
		matchVal, _ := patternStruct.Attr("match")
		replaceVal, _ := patternStruct.Attr("replace")
		match, _ := starlark.AsString(matchVal)
		replace, _ := starlark.AsString(replaceVal)
		patterns[key] = CopyrightPattern{Match: match, Replace: replace}
	}

	var fixed []starlark.Value
	var errors []starlark.Value

	iter := pathsList.Iterate()
	defer iter.Done()
	var pathVal starlark.Value
	for iter.Next(&pathVal) {
		path, _ := starlark.AsString(pathVal)

		lang := langFromExt(path)
		if lang == "" {
			continue
		}

		pattern, ok := patterns[lang]
		if !ok {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		canonical := buildCanonicalHeader(pattern.Replace, license, holder)
		existing, found := extractHeader(string(content), lang)

		// 1. If header matches canonical exactly, skip
		if found && strings.TrimSpace(existing) == canonical {
			continue
		}

		// 2. If header matches the match regex, fix it
		if found && pattern.Match != "" {
			matchRe, err := regexp.Compile(pattern.Match)
			if err == nil && matchRe.MatchString(existing) {
				// Build new content
				newContent := fixFileContent(string(content), canonical, lang, true)

				if !dryRun {
					if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
						errors = append(errors, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
							"file":    starlark.String(path),
							"message": starlark.String("failed to write: " + err.Error()),
						}))
						continue
					}
				}

				fixed = append(fixed, starlark.String(path))
				continue
			}
		}

		// 3. Otherwise, cannot fix automatically - report error
		msg := "cannot fix automatically: header does not match expected pattern"
		if !found {
			msg = "cannot fix automatically: no header found"
		}
		errors = append(errors, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"file":    starlark.String(path),
			"message": starlark.String(msg),
		}))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"fixed":  starlark.NewList(fixed),
		"count":  starlark.MakeInt(len(fixed)),
		"errors": starlark.NewList(errors),
	}), nil
}

// fixFileContent adds or replaces the copyright header in file content.
func fixFileContent(content, header, lang string, hasExisting bool) string {
	lines := strings.Split(content, "\n")

	commentPrefix := "//"
	if lang == "star" || lang == "shell" {
		commentPrefix = "#"
	}

	// Handle shebang for shell scripts
	var shebang string
	startIdx := 0
	if lang == "shell" && len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		shebang = lines[0]
		startIdx = 1
		// Skip blank line after shebang
		if len(lines) > 1 && strings.TrimSpace(lines[1]) == "" {
			startIdx = 2
		}
	}

	// Remove existing header if present
	if hasExisting {
		headerLineCount := 0
		for i := startIdx; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, commentPrefix) {
				headerLineCount++
				if headerLineCount >= 2 {
					startIdx = i + 1
					break
				}
			} else {
				break
			}
		}
		// Skip blank line after old header
		if startIdx < len(lines) && strings.TrimSpace(lines[startIdx]) == "" {
			startIdx++
		}
	}

	// Build new content
	var result strings.Builder

	if shebang != "" {
		result.WriteString(shebang)
		result.WriteString("\n\n")
	}

	result.WriteString(header)
	result.WriteString("\n\n")

	// Add remaining content
	for i := startIdx; i < len(lines); i++ {
		result.WriteString(lines[i])
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// copyrightDetectLicense detects the SPDX license identifier from a LICENSE file.
func copyrightDetectLicense(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("copyright.detect_license", args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"license":  starlark.String(""),
			"detected": starlark.Bool(false),
		}), nil
	}
	defer file.Close()

	// Read first few lines to detect license type
	scanner := bufio.NewScanner(file)
	var firstLines []string
	for i := 0; i < 10 && scanner.Scan(); i++ {
		firstLines = append(firstLines, scanner.Text())
	}
	content := strings.Join(firstLines, "\n")
	contentLower := strings.ToLower(content)

	// Detect common licenses
	var license string
	switch {
	case strings.Contains(contentLower, "mit license"):
		license = "MIT"
	case strings.Contains(contentLower, "apache license") && strings.Contains(contentLower, "2.0"):
		license = "Apache-2.0"
	case strings.Contains(contentLower, "gnu general public license") && strings.Contains(contentLower, "version 3"):
		license = "GPL-3.0"
	case strings.Contains(contentLower, "gnu lesser general public license"):
		license = "LGPL-3.0"
	case strings.Contains(contentLower, "bsd 3-clause"):
		license = "BSD-3-Clause"
	case strings.Contains(contentLower, "bsd 2-clause"):
		license = "BSD-2-Clause"
	case strings.Contains(contentLower, "mozilla public license") && strings.Contains(contentLower, "2.0"):
		license = "MPL-2.0"
	case strings.Contains(contentLower, "server side public license"):
		license = "SSPL-1.0"
	case strings.Contains(contentLower, "unlicense"):
		license = "Unlicense"
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"license":  starlark.String(license),
		"detected": starlark.Bool(license != ""),
	}), nil
}
