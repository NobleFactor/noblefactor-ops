// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// stubMatcher is the fallback implementation using regex patterns.
type stubMatcher struct {
	patterns []stubPattern
}

type stubPattern struct {
	regex  *regexp.Regexp
	negate bool
}

// matchStub checks if a path is ignored using the stub implementation.
func matchStub(path, base string) bool {
	m := loadStubMatcher(base)
	if m == nil {
		return false
	}
	return m.match(path)
}

// filterStub filters paths using the stub implementation.
func filterStub(paths []string, base string) []string {
	m := loadStubMatcher(base)
	if m == nil {
		return paths
	}

	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if !m.match(p) {
			result = append(result, p)
		}
	}
	return result
}

// loadStubMatcher creates a matcher from .gitignore files.
func loadStubMatcher(base string) *stubMatcher {
	m := &stubMatcher{}

	// Walk up to find .gitignore files
	current := base
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
		m.loadGitignore(gitignorePaths[i])
	}

	if len(m.patterns) == 0 {
		return nil
	}
	return m
}

// loadGitignore parses a .gitignore file and adds patterns.
func (m *stubMatcher) loadGitignore(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if p := parseStubPattern(line); p != nil {
			m.patterns = append(m.patterns, *p)
		}
	}
}

// parseStubPattern converts a gitignore line to a regex pattern.
func parseStubPattern(line string) *stubPattern {
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

	return &stubPattern{regex: re, negate: negate}
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

// match checks if the given path should be ignored.
func (m *stubMatcher) match(path string) bool {
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
