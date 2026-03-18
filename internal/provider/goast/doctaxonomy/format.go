// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

import (
	"go/doc/comment"
	"strings"
)

// Format takes normalized text (from Normalize()) and produces the final
// formatted comment with // prefix, line wrapping, list indentation, and
// code block pass-through.
//
// Parameters:
//   - normalized: The normalized text from a Normalize() call.
//   - width: The target total line width (e.g., 120) including the "// " prefix.
func Format(normalized string, width int) string {
	var p comment.Parser
	doc := p.Parse(normalized)

	var pr comment.Printer
	pr.TextPrefix = "// "
	pr.TextCodePrefix = "//\t"
	pr.TextWidth = width - 3 // Exclude "// " prefix; TextWidth is content width only.

	out := string(pr.Text(doc))

	// Trim trailing "// " on blank separator lines to just "//".
	out = strings.ReplaceAll(out, "// \n", "//\n")

	return out
}
