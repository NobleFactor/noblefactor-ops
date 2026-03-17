# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# line-width.star — Line width enforcement
#
# Checks two things:
# 1. No line exceeds the configured column width (default 120).
# 2. Comment text fills to the column width — a line that could absorb the next
#    line's first word without exceeding the width is under-filled.
#
# Fix: calls goast.rewrap_comments to reflow comment paragraphs to fill to
# the configured width. Code lines that exceed the width are check-only.

def check(ctx):
    """Check for lines that exceed the width or are under-filled."""
    width = ctx["config"]["line_width"]
    lines = ctx["lines"]
    violations = []

    # Check for over-long lines.
    for i, line in enumerate(lines):
        if len(line) > width:
            violations.append({
                "line": i + 1,
                "message": "line is " + str(len(line)) + " columns (max " + str(width) + ")",
            })

    # Check for under-filled comment lines.
    violations += _check_under_filled(lines, width)

    return violations

def _check_under_filled(lines, width):
    """Check for under-filled comment lines.

    A comment line is under-filled if the first word of the following comment line
    could be appended to it without exceeding the width. Blank separator lines,
    indented code blocks (4+ spaces after //), bullet items, section headers, and
    directive lines are excluded.

    Parameters:
        lines: all file lines.
        width: the configured line width.

    Returns:
        list of violation dicts.
    """
    violations = []
    i = 0

    while i < len(lines) - 1:
        curr = lines[i]
        next_line = lines[i + 1]

        # Both lines must be comment lines.
        if not _is_comment(curr) or not _is_comment(next_line):
            i += 1
            continue

        curr_body = _comment_body(curr)
        next_body = _comment_body(next_line)

        # Skip blank separator lines.
        if not curr_body.strip() or not next_body.strip():
            i += 1
            continue

        # Skip section delineators (=== lines, --- lines).
        if _is_delineator(curr_body) or _is_delineator(next_body):
            i += 1
            continue

        # Skip SPDX/copyright header lines.
        if curr_body.startswith("SPDX-") or curr_body.startswith("Copyright"):
            i += 1
            continue

        # Skip indented code blocks (4+ spaces after //).
        if curr_body.startswith("    ") or next_body.startswith("    "):
            i += 1
            continue

        # Skip bullet items.
        if curr_body.strip().startswith("- ") or next_body.strip().startswith("- "):
            i += 1
            continue

        # Skip section headers (end with colon) and directives (start with +).
        if curr_body.strip().endswith(":") or curr_body.strip().startswith("+"):
            i += 1
            continue

        # Skip if next line is a section header, directive, or bullet.
        if next_body.strip().endswith(":") or next_body.strip().startswith("+") or next_body.strip().startswith("- "):
            i += 1
            continue

        # Check if first word of next line could fit on current line.
        next_words = next_body.strip().split(" ")
        if next_words and next_words[0]:
            first_word = next_words[0]
            if len(curr) + 1 + len(first_word) <= width:
                violations.append({
                    "line": i + 1,
                    "message": "under-filled comment ('" + first_word + "' fits on previous line, " + str(width - len(curr)) + " columns available)",
                })

        i += 1

    return violations

def _is_delineator(body):
    """Check if a comment body is a section delineator (====, ----, etc.)."""
    stripped = body.strip()
    if len(stripped) < 3:
        return False
    # All same character (=, -, ~, *).
    first = stripped[0]
    if first in "=-~*":
        for c in stripped.elems():
            if c != first:
                return False
        return True
    return False

def _is_comment(line):
    """Check if a line is a // comment."""
    return line.strip().startswith("//")

def _comment_body(line):
    """Extract the body after // from a comment line.

    Returns the text after '// ' (or after '//' if no space).
    """
    trimmed = line.strip()
    if trimmed.startswith("// "):
        return trimmed[3:]
    if trimmed == "//":
        return ""
    if trimmed.startswith("//"):
        return trimmed[2:]
    return ""

def fix(ctx):
    """Fix line width issues by rewrapping comments.

    Rewraps comment paragraphs to fill to the configured width. This fixes both
    over-long comment lines (by wrapping) and under-filled comment lines (by
    absorbing words from the next line).

    Code lines that exceed the width cannot be auto-fixed.
    """
    width = ctx["config"]["line_width"]
    path = ctx["path"]
    return goast.rewrap_comments(path=path, width=width)
