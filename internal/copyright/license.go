// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package copyright

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LicensePatterns maps SPDX identifiers to regex patterns for LICENSE file detection.
var LicensePatterns = map[string]*regexp.Regexp{
	"MIT":        regexp.MustCompile(`(?i)MIT License|Permission is hereby granted, free of charge`),
	"Apache-2.0": regexp.MustCompile(`(?i)Apache License.*Version 2\.0|Licensed under the Apache License`),
	"GPL-3.0":    regexp.MustCompile(`(?i)GNU GENERAL PUBLIC LICENSE.*Version 3|GPL.?3`),
	"GPL-2.0":    regexp.MustCompile(`(?i)GNU GENERAL PUBLIC LICENSE.*Version 2|GPL.?2`),
	"LGPL-3.0":   regexp.MustCompile(`(?i)GNU LESSER GENERAL PUBLIC LICENSE.*Version 3|LGPL.?3`),
	"LGPL-2.1":   regexp.MustCompile(`(?i)GNU LESSER GENERAL PUBLIC LICENSE.*Version 2\.1|LGPL.?2\.1`),
	"BSD-3-Clause": regexp.MustCompile(`(?i)BSD 3-Clause|Redistribution and use in source and binary forms.*provided that the following conditions`),
	"BSD-2-Clause": regexp.MustCompile(`(?i)BSD 2-Clause|Simplified BSD License`),
	"ISC":        regexp.MustCompile(`(?i)ISC License|Permission to use, copy, modify, and/or distribute`),
	"MPL-2.0":    regexp.MustCompile(`(?i)Mozilla Public License.*2\.0`),
	"Unlicense":  regexp.MustCompile(`(?i)This is free and unencumbered software released into the public domain`),
	"WTFPL":      regexp.MustCompile(`(?i)DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE`),
	"CC0-1.0":    regexp.MustCompile(`(?i)CC0 1\.0|Creative Commons.*Zero`),
	"Zlib":       regexp.MustCompile(`(?i)zlib License|Permission is granted to anyone to use this software`),
	"BSL-1.0":    regexp.MustCompile(`(?i)Boost Software License|BSL.?1\.0`),
	"EPL-2.0":    regexp.MustCompile(`(?i)Eclipse Public License.*2\.0`),
	"AGPL-3.0":   regexp.MustCompile(`(?i)GNU AFFERO GENERAL PUBLIC LICENSE.*Version 3`),
}

// LicenseResult holds the result of license detection.
type LicenseResult struct {
	Detected bool
	SPDX     string
	FilePath string
}

// DetectLicense detects the license from a LICENSE file in the given directory.
// It searches for common license file names.
func DetectLicense(dir string) LicenseResult {
	// Common license file names
	candidates := []string{
		"LICENSE",
		"LICENSE.txt",
		"LICENSE.md",
		"LICENCE",
		"LICENCE.txt",
		"LICENCE.md",
		"LICENSE-MIT",
		"LICENSE-APACHE",
		"COPYING",
		"COPYING.txt",
	}

	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if content, err := os.ReadFile(path); err == nil {
			if spdx := matchLicense(string(content)); spdx != "" {
				return LicenseResult{
					Detected: true,
					SPDX:     spdx,
					FilePath: path,
				}
			}
		}
	}

	return LicenseResult{}
}

// matchLicense matches license content against known patterns.
func matchLicense(content string) string {
	// Normalize content for matching
	content = strings.ToLower(content)

	for spdx, pattern := range LicensePatterns {
		if pattern.MatchString(content) {
			return spdx
		}
	}

	return ""
}

// DetectLicenseFile detects license from a specific file path.
func DetectLicenseFile(path string) LicenseResult {
	content, err := os.ReadFile(path)
	if err != nil {
		return LicenseResult{}
	}

	spdx := matchLicense(string(content))
	if spdx != "" {
		return LicenseResult{
			Detected: true,
			SPDX:     spdx,
			FilePath: path,
		}
	}

	return LicenseResult{}
}

// FindLicenseFile finds a license file walking up from the given directory.
func FindLicenseFile(startDir string) string {
	candidates := []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "LICENCE", "COPYING"}

	dir := startDir
	for {
		for _, name := range candidates {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return ""
		}
		dir = parent
	}
}
