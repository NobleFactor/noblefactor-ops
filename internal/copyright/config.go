// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package copyright provides configurable copyright header checking and fixing.
package copyright

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed defaults.yaml
var defaultsFS embed.FS

// Config holds the merged copyright configuration.
type Config struct {
	Defaults  Defaults            `yaml:"defaults"`
	Languages map[string]Language `yaml:"languages"`
}

// Defaults holds default settings applied to all files.
type Defaults struct {
	License string   `yaml:"license"` // "auto" to detect from LICENSE file
	Holder  string   `yaml:"holder"`
	Year    string   `yaml:"year"` // "auto" to use current year
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// Language defines language-specific detection and comment styles.
type Language struct {
	Extensions []string `yaml:"extensions"`
	Filenames  []string `yaml:"filenames"`
	Shebangs   []string `yaml:"shebangs"`
	Comment    Comment  `yaml:"comment"`
	Header     string   `yaml:"header"` // Go template
	Match      Match    `yaml:"match"`
}

// Comment defines comment styles for a language.
type Comment struct {
	Line  *string  `yaml:"line"`  // nil if no line comments
	Block []string `yaml:"block"` // [start, prefix, end] or nil
}

// Match defines regex patterns for detecting existing headers.
type Match struct {
	SPDX      string `yaml:"spdx"`
	Copyright string `yaml:"copyright"`
}

// LoadConfig loads and merges configuration from all sources.
// Priority: project (.star.yaml) > user (~/.config/star/config.yaml) > defaults
func LoadConfig() (*Config, error) {
	// Start with embedded defaults
	cfg, err := loadDefaults()
	if err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// Merge user config
	userCfg, err := loadUserConfig()
	if err == nil && userCfg != nil {
		cfg = mergeConfig(cfg, userCfg)
	}

	// Merge project config
	projectCfg, err := loadProjectConfig()
	if err == nil && projectCfg != nil {
		cfg = mergeConfig(cfg, projectCfg)
	}

	return cfg, nil
}

// loadDefaults loads the embedded defaults.yaml.
func loadDefaults() (*Config, error) {
	data, err := defaultsFS.ReadFile("defaults.yaml")
	if err != nil {
		return nil, fmt.Errorf("reading embedded defaults: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing defaults: %w", err)
	}

	return &cfg, nil
}

// loadUserConfig loads user config from ~/.config/star/config.yaml.
func loadUserConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(home, ".config", "star", "config.yaml")
	return loadConfigFile(configPath)
}

// loadProjectConfig loads project config from .star.yaml in current directory.
func loadProjectConfig() (*Config, error) {
	// Try current directory first
	configPath := ".star.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try looking up the directory tree
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		for {
			candidate := filepath.Join(dir, ".star.yaml")
			if _, err := os.Stat(candidate); err == nil {
				configPath = candidate
				break
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				// Reached root, no config found
				return nil, nil
			}
			dir = parent
		}
	}

	return loadConfigFile(configPath)
}

// loadConfigFile loads a config file if it exists.
func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Parse the full file looking for copyright section
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	// Extract copyright section
	copyrightSection, ok := raw["copyright"]
	if !ok {
		return nil, nil
	}

	// Re-marshal and unmarshal to get typed config
	copyrightData, err := yaml.Marshal(copyrightSection)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(copyrightData, &cfg); err != nil {
		return nil, fmt.Errorf("parsing copyright section in %s: %w", path, err)
	}

	return &cfg, nil
}

// mergeConfig merges overlay onto base, returning the result.
// overlay values take precedence when set.
func mergeConfig(base, overlay *Config) *Config {
	if overlay == nil {
		return base
	}
	if base == nil {
		return overlay
	}

	result := &Config{
		Defaults:  base.Defaults,
		Languages: make(map[string]Language),
	}

	// Copy base languages
	for name, lang := range base.Languages {
		result.Languages[name] = lang
	}

	// Merge defaults
	if overlay.Defaults.License != "" {
		result.Defaults.License = overlay.Defaults.License
	}
	if overlay.Defaults.Holder != "" {
		result.Defaults.Holder = overlay.Defaults.Holder
	}
	if overlay.Defaults.Year != "" {
		result.Defaults.Year = overlay.Defaults.Year
	}
	if len(overlay.Defaults.Include) > 0 {
		result.Defaults.Include = overlay.Defaults.Include
	}
	if len(overlay.Defaults.Exclude) > 0 {
		result.Defaults.Exclude = append(base.Defaults.Exclude, overlay.Defaults.Exclude...)
	}

	// Merge languages (overlay replaces or adds)
	for name, lang := range overlay.Languages {
		result.Languages[name] = lang
	}

	return result
}

// GetLanguage returns the language config by name, or nil if not found.
func (c *Config) GetLanguage(name string) *Language {
	if lang, ok := c.Languages[name]; ok {
		return &lang
	}
	return nil
}
