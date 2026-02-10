// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package ignore provides gitignore-aware file filtering.
//
// This is a stub implementation. The full implementation will use
// the gitignore WASM extension which wraps BurntSushi's ignore crate.
package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Matcher checks if paths should be ignored based on .gitignore rules.
type Matcher struct {
	patterns []pattern
	base     string
}

type pattern struct {
	regex  *regexp.Regexp
	negate bool
}

// New creates a new Matcher for the given directory.
// It walks up the directory tree to find .gitignore files.
func New(dir string) (*Matcher, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	m := &Matcher{base: absDir}

	// Walk up to find .gitignore files
	current := absDir
	var gitignorePaths []string

	for {
		gi := filepath.Join(current, ".gitignore")
		if _, err := os.Stat(gi); err == nil {
			gitignorePaths = append(gitignorePaths, gi)
		}

		// Stop at .git directory or filesystem root
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			break
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Load gitignore files (root first)
	for i := len(gitignorePaths) - 1; i >= 0; i-- {
		if err := m.loadGitignore(gitignorePaths[i]); err != nil {
			// Ignore errors loading individual files
			continue
		}
	}

	return m, nil
}

// loadGitignore parses a .gitignore file and adds patterns.
func (m *Matcher) loadGitignore(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if p := parsePattern(line); p != nil {
			m.patterns = append(m.patterns, *p)
		}
	}

	return scanner.Err()
}

// parsePattern converts a gitignore line to a regex pattern.
// Returns nil for comments and blank lines.
func parsePattern(line string) *pattern {
	// Trim trailing spaces
	line = strings.TrimRight(line, " \t\r")

	// Skip empty lines and comments
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	negate := false
	if strings.HasPrefix(line, "!") {
		negate = true
		line = line[1:]
	}

	// Convert gitignore pattern to regex
	regex := gitignoreToRegex(line)
	if regex == "" {
		return nil
	}

	re, err := regexp.Compile(regex)
	if err != nil {
		return nil
	}

	return &pattern{regex: re, negate: negate}
}

// gitignoreToRegex converts a gitignore pattern to a regex.
func gitignoreToRegex(pattern string) string {
	// Handle directory-only patterns
	dirOnly := strings.HasSuffix(pattern, "/")
	if dirOnly {
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Escape regex special chars except * and ?
	var b strings.Builder
	b.WriteString("(?:^|/)")

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// ** matches any path
				b.WriteString(".*")
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++ // skip the slash after **
				}
			} else {
				// * matches anything except /
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}

	if dirOnly {
		b.WriteString("/")
	} else {
		b.WriteString("(?:/|$)")
	}

	return b.String()
}

// Match checks if the given path should be ignored.
func (m *Matcher) Match(path string) bool {
	if len(m.patterns) == 0 {
		return false
	}

	// Normalize path
	path = filepath.ToSlash(path)

	ignored := false
	for _, p := range m.patterns {
		if p.regex.MatchString(path) {
			ignored = !p.negate
		}
	}

	return ignored
}

// Filter returns paths that are NOT ignored.
func (m *Matcher) Filter(paths []string) []string {
	if len(m.patterns) == 0 {
		return paths
	}

	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if !m.Match(p) {
			result = append(result, p)
		}
	}
	return result
}
