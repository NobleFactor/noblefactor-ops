// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package config

// merge combines two configs, with overlay taking precedence over base.
// Only non-zero values in overlay override base values.
func merge(base, overlay *Config) *Config {
	result := *base

	// Merge lint config
	result.Lint = mergeLint(base.Lint, overlay.Lint)

	// Merge precommit config
	result.Precommit = mergePrecommit(base.Precommit, overlay.Precommit)

	return &result
}

func mergePrecommit(base, overlay PrecommitConfig) PrecommitConfig {
	result := base

	// Hooks are replaced entirely if overlay has any
	if len(overlay.Hooks) > 0 {
		result.Hooks = overlay.Hooks
	}

	return result
}

func mergeLint(base, overlay LintConfig) LintConfig {
	result := base

	// Merge Go config
	result.Go = mergeGo(base.Go, overlay.Go)

	// Merge Shell config
	result.Shell = mergeShell(base.Shell, overlay.Shell)

	// Merge Markdown config
	result.Markdown = mergeMarkdown(base.Markdown, overlay.Markdown)

	// Merge Copyright config
	result.Copyright = mergeCopyright(base.Copyright, overlay.Copyright)

	return result
}

func mergeGo(base, overlay GoLintConfig) GoLintConfig {
	result := base

	if overlay.Path != "" {
		result.Path = overlay.Path
	}
	// SkipModTidy is a bool, so we always take overlay if set explicitly
	// Since we can't distinguish "not set" from "false", we always merge
	result.SkipModTidy = overlay.SkipModTidy || base.SkipModTidy

	if overlay.Config != nil {
		result.Config = mergeMaps(base.Config, overlay.Config)
	}

	return result
}

func mergeShell(base, overlay ShellLintConfig) ShellLintConfig {
	result := base

	if overlay.Path != "" {
		result.Path = overlay.Path
	}
	if overlay.Severity != "" {
		result.Severity = overlay.Severity
	}
	if overlay.Indent != 0 {
		result.Indent = overlay.Indent
	}

	return result
}

func mergeMarkdown(base, overlay MarkdownLintConfig) MarkdownLintConfig {
	result := base

	if overlay.Path != "" {
		result.Path = overlay.Path
	}
	if len(overlay.Exclude) > 0 {
		result.Exclude = overlay.Exclude
	}
	if overlay.Config != nil {
		result.Config = mergeMaps(base.Config, overlay.Config)
	}

	// Merge frontmatter config
	result.Frontmatter = mergeFrontmatter(base.Frontmatter, overlay.Frontmatter)

	return result
}

func mergeFrontmatter(base, overlay FrontmatterConfig) FrontmatterConfig {
	result := base

	if len(overlay.Required) > 0 {
		result.Required = overlay.Required
	}
	if len(overlay.Optional) > 0 {
		result.Optional = overlay.Optional
	}

	return result
}

func mergeCopyright(base, overlay CopyrightLintConfig) CopyrightLintConfig {
	result := base

	// Enabled is explicitly set if the overlay has it true
	// (since we can't distinguish "not set" from "false", we OR them)
	if overlay.Enabled {
		result.Enabled = true
	}
	if overlay.License != "" {
		result.License = overlay.License
	}
	if overlay.Holder != "" {
		result.Holder = overlay.Holder
	}
	if overlay.Patterns != nil {
		// Merge patterns map
		if result.Patterns == nil {
			result.Patterns = make(map[string]CopyrightPattern)
		}
		for k, v := range overlay.Patterns {
			result.Patterns[k] = v
		}
	}
	if len(overlay.Exclude) > 0 {
		result.Exclude = overlay.Exclude
	}

	return result
}

// mergeMaps deep merges two maps, with overlay taking precedence.
func mergeMaps(base, overlay map[string]interface{}) map[string]interface{} {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	result := make(map[string]interface{})

	// Copy base values
	for k, v := range base {
		result[k] = v
	}

	// Override with overlay values
	for k, v := range overlay {
		if baseMap, ok := result[k].(map[string]interface{}); ok {
			if overlayMap, ok := v.(map[string]interface{}); ok {
				result[k] = mergeMaps(baseMap, overlayMap)
				continue
			}
		}
		result[k] = v
	}

	return result
}
