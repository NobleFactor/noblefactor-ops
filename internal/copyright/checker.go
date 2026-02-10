// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package copyright

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// Checker checks and fixes copyright headers in files.
type Checker struct {
	config   *Config
	detector *Detector

	// Compiled patterns per language
	spdxPatterns      map[string]*regexp.Regexp
	copyrightPatterns map[string]*regexp.Regexp

	// Template cache
	headerTemplates map[string]*template.Template
}

// NewChecker creates a new copyright header checker.
func NewChecker(cfg *Config) (*Checker, error) {
	c := &Checker{
		config:            cfg,
		detector:          NewDetector(cfg),
		spdxPatterns:      make(map[string]*regexp.Regexp),
		copyrightPatterns: make(map[string]*regexp.Regexp),
		headerTemplates:   make(map[string]*template.Template),
	}

	// Pre-compile patterns and templates
	for name, lang := range cfg.Languages {
		if lang.Match.SPDX != "" {
			re, err := regexp.Compile(lang.Match.SPDX)
			if err != nil {
				return nil, fmt.Errorf("invalid SPDX pattern for %s: %w", name, err)
			}
			c.spdxPatterns[name] = re
		}

		if lang.Match.Copyright != "" {
			re, err := regexp.Compile(lang.Match.Copyright)
			if err != nil {
				return nil, fmt.Errorf("invalid copyright pattern for %s: %w", name, err)
			}
			c.copyrightPatterns[name] = re
		}

		if lang.Header != "" {
			tmpl, err := template.New(name).Parse(lang.Header)
			if err != nil {
				return nil, fmt.Errorf("invalid header template for %s: %w", name, err)
			}
			c.headerTemplates[name] = tmpl
		}
	}

	return c, nil
}

// HeaderData holds data for template rendering.
type HeaderData struct {
	License string
	Holder  string
	Year    string
}

// CheckResult holds the result of checking a single file.
type CheckResult struct {
	Path     string
	Language string
	Method   string // Detection method: filename, extension, shebang

	HasHeader bool
	License   string // Detected license from SPDX
	Holder    string // Detected holder from copyright line

	LicenseMatch bool // True if license matches expected
	HolderMatch  bool // True if holder matches expected
	Valid        bool // True if header is completely valid

	Error error
}

// CheckFile checks a single file for copyright header.
func (c *Checker) CheckFile(path string, expected HeaderData) CheckResult {
	result := CheckResult{Path: path}

	// Detect language
	detect := c.detector.Detect(path)
	if detect.Language == "" {
		result.Error = fmt.Errorf("unknown file type")
		return result
	}

	result.Language = detect.Language
	result.Method = detect.Method

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		result.Error = err
		return result
	}

	// Get patterns for this language
	spdxRe := c.spdxPatterns[detect.Language]
	copyrightRe := c.copyrightPatterns[detect.Language]

	if spdxRe == nil || copyrightRe == nil {
		result.Error = fmt.Errorf("no patterns defined for language %s", detect.Language)
		return result
	}

	// Check for header in first N lines
	lines := strings.SplitN(string(content), "\n", 20)
	startLine := 0

	// Skip shebang line if present
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		startLine = 1
	}

	// Search for SPDX and copyright lines
	for i := startLine; i < len(lines) && i < startLine+10; i++ {
		line := lines[i]

		// Check SPDX
		if result.License == "" {
			if matches := spdxRe.FindStringSubmatch(line); len(matches) > 1 {
				result.License = matches[1]
				result.HasHeader = true
			}
		}

		// Check copyright
		if result.Holder == "" {
			if matches := copyrightRe.FindStringSubmatch(line); len(matches) > 1 {
				result.Holder = strings.TrimSpace(matches[1])
				result.HasHeader = true
			}
		}

		// Both found, can stop
		if result.License != "" && result.Holder != "" {
			break
		}
	}

	// Validate against expected values
	if expected.License != "" && expected.License != "auto" {
		result.LicenseMatch = result.License == expected.License
	} else {
		result.LicenseMatch = result.License != ""
	}

	if expected.Holder != "" {
		result.HolderMatch = result.Holder == expected.Holder
	} else {
		result.HolderMatch = result.Holder != ""
	}

	result.Valid = result.HasHeader && result.LicenseMatch && result.HolderMatch

	return result
}

// FixFile adds or fixes the copyright header in a file.
func (c *Checker) FixFile(path string, data HeaderData) error {
	// Detect language
	detect := c.detector.Detect(path)
	if detect.Language == "" {
		return fmt.Errorf("unknown file type: %s", path)
	}

	// Get template
	tmpl := c.headerTemplates[detect.Language]
	if tmpl == nil {
		return fmt.Errorf("no header template for language %s", detect.Language)
	}

	// Render header
	var headerBuf bytes.Buffer
	if err := tmpl.Execute(&headerBuf, data); err != nil {
		return fmt.Errorf("rendering header: %w", err)
	}
	header := headerBuf.String()

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	startLine := 0

	// Preserve shebang
	var shebang string
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		shebang = lines[0] + "\n"
		startLine = 1
	}

	// Check if header already exists and remove it
	spdxRe := c.spdxPatterns[detect.Language]
	copyrightRe := c.copyrightPatterns[detect.Language]

	endOfHeader := startLine
	foundSPDX := false
	foundCopyright := false

	for i := startLine; i < len(lines) && i < startLine+10; i++ {
		line := lines[i]

		if spdxRe != nil && spdxRe.MatchString(line) {
			foundSPDX = true
			endOfHeader = i + 1
		}
		if copyrightRe != nil && copyrightRe.MatchString(line) {
			foundCopyright = true
			endOfHeader = i + 1
		}

		// Stop at first non-comment, non-empty line after finding header
		if (foundSPDX || foundCopyright) && !isCommentLine(line, detect.Language, c.config) {
			break
		}
	}

	// Build new content
	var newContent strings.Builder

	// Shebang first
	if shebang != "" {
		newContent.WriteString(shebang)
	}

	// New header
	newContent.WriteString(strings.TrimRight(header, "\n"))
	newContent.WriteString("\n")

	// Original content after old header (or after shebang if no header found)
	if foundSPDX || foundCopyright {
		// Skip blank lines immediately after old header
		for endOfHeader < len(lines) && strings.TrimSpace(lines[endOfHeader]) == "" {
			endOfHeader++
		}
		if endOfHeader < len(lines) {
			newContent.WriteString("\n")
			newContent.WriteString(strings.Join(lines[endOfHeader:], "\n"))
		}
	} else {
		// No old header, add blank line and rest of content
		newContent.WriteString("\n")
		restStart := startLine
		for restStart < len(lines) && strings.TrimSpace(lines[restStart]) == "" {
			restStart++
		}
		if restStart < len(lines) {
			newContent.WriteString(strings.Join(lines[restStart:], "\n"))
		}
	}

	// Ensure trailing newline
	result := newContent.String()
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	// Write back
	return os.WriteFile(path, []byte(result), 0644)
}

// isCommentLine checks if a line is a comment for the given language.
func isCommentLine(line string, lang string, cfg *Config) bool {
	langCfg := cfg.GetLanguage(lang)
	if langCfg == nil {
		return false
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true // Blank lines are part of header block
	}

	// Check line comment
	if langCfg.Comment.Line != nil {
		if strings.HasPrefix(trimmed, *langCfg.Comment.Line) {
			return true
		}
	}

	// Check block comment markers
	if len(langCfg.Comment.Block) >= 3 {
		start := langCfg.Comment.Block[0]
		prefix := langCfg.Comment.Block[1]
		end := langCfg.Comment.Block[2]

		if strings.HasPrefix(trimmed, start) ||
			strings.HasPrefix(trimmed, end) ||
			(prefix != "" && strings.HasPrefix(trimmed, prefix)) {
			return true
		}
	}

	return false
}

// CheckDir checks all files in a directory matching include/exclude patterns.
func (c *Checker) CheckDir(dir string, expected HeaderData) ([]CheckResult, error) {
	var results []CheckResult

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Get relative path for pattern matching
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			relPath = path
		}

		// Check include patterns
		included := false
		for _, pattern := range c.config.Defaults.Include {
			if match, _ := doublestar.Match(pattern, relPath); match {
				included = true
				break
			}
		}
		if !included {
			return nil
		}

		// Check exclude patterns
		for _, pattern := range c.config.Defaults.Exclude {
			if match, _ := doublestar.Match(pattern, relPath); match {
				return nil
			}
		}

		// Check file
		result := c.CheckFile(path, expected)
		results = append(results, result)

		return nil
	})

	return results, err
}

// GetYearString returns the year string for headers.
func GetYearString(year string) string {
	if year == "auto" || year == "" {
		return fmt.Sprintf("%d", time.Now().Year())
	}
	return year
}
