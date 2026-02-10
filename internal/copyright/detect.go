// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package copyright

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Detector detects programming languages from files.
type Detector struct {
	config *Config

	// Cached lookups for fast detection
	extToLang      map[string]string
	filenameToLang map[string]string
	shebangPatterns []shebangPattern

	mu sync.RWMutex
}

type shebangPattern struct {
	pattern *regexp.Regexp
	lang    string
}

// NewDetector creates a new language detector from config.
func NewDetector(cfg *Config) *Detector {
	d := &Detector{
		config:         cfg,
		extToLang:      make(map[string]string),
		filenameToLang: make(map[string]string),
	}
	d.buildLookups()
	return d
}

// buildLookups builds the fast lookup tables from config.
func (d *Detector) buildLookups() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for langName, lang := range d.config.Languages {
		// Extensions
		for _, ext := range lang.Extensions {
			d.extToLang[ext] = langName
		}

		// Filenames
		for _, filename := range lang.Filenames {
			d.filenameToLang[filename] = langName
		}

		// Shebangs
		for _, pattern := range lang.Shebangs {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue // Skip invalid patterns
			}
			d.shebangPatterns = append(d.shebangPatterns, shebangPattern{
				pattern: re,
				lang:    langName,
			})
		}
	}
}

// DetectResult holds the result of language detection.
type DetectResult struct {
	Language string // Language name, or empty if not detected
	Method   string // "filename", "extension", "shebang", or ""
}

// Detect detects the language of a file using the priority:
// 1. Exact filename match
// 2. File extension
// 3. Shebang line (first line starting with #!)
func (d *Detector) Detect(path string) DetectResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	filename := filepath.Base(path)

	// 1. Exact filename match
	if lang, ok := d.filenameToLang[filename]; ok {
		return DetectResult{Language: lang, Method: "filename"}
	}

	// 2. Extension match
	ext := filepath.Ext(filename)
	if ext != "" {
		if lang, ok := d.extToLang[ext]; ok {
			return DetectResult{Language: lang, Method: "extension"}
		}
	}

	// 3. Shebang detection
	if shebang := d.readShebang(path); shebang != "" {
		for _, sp := range d.shebangPatterns {
			if sp.pattern.MatchString(shebang) {
				return DetectResult{Language: sp.lang, Method: "shebang"}
			}
		}
	}

	return DetectResult{}
}

// DetectContent detects language from content shebang only.
// Useful when you have content but not a file path.
func (d *Detector) DetectContent(content string) DetectResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Check shebang
	if strings.HasPrefix(content, "#!") {
		firstLine := strings.SplitN(content, "\n", 2)[0]
		for _, sp := range d.shebangPatterns {
			if sp.pattern.MatchString(firstLine) {
				return DetectResult{Language: sp.lang, Method: "shebang"}
			}
		}
	}

	return DetectResult{}
}

// readShebang reads the first line if it's a shebang.
func (d *Detector) readShebang(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#!") {
			return line
		}
	}

	return ""
}

// GetLanguageConfig returns the Language config for a detected language.
func (d *Detector) GetLanguageConfig(result DetectResult) *Language {
	if result.Language == "" {
		return nil
	}
	return d.config.GetLanguage(result.Language)
}

// SupportedExtensions returns all recognized file extensions.
func (d *Detector) SupportedExtensions() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	exts := make([]string, 0, len(d.extToLang))
	for ext := range d.extToLang {
		exts = append(exts, ext)
	}
	return exts
}

// SupportedFilenames returns all recognized exact filenames.
func (d *Detector) SupportedFilenames() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	names := make([]string, 0, len(d.filenameToLang))
	for name := range d.filenameToLang {
		names = append(names, name)
	}
	return names
}

// SupportedLanguages returns all configured language names.
func (d *Detector) SupportedLanguages() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	langs := make([]string, 0, len(d.config.Languages))
	for name := range d.config.Languages {
		langs = append(langs, name)
	}
	return langs
}
