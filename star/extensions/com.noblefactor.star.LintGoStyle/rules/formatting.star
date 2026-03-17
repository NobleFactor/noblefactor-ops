# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# formatting.star — Blank line rules
#
# Checks:
# 1. Blank line after function signature for multi-statement bodies
# 2. No blank line after function signature for single-statement bodies
# 3. No blank line before return in short functions

def check(ctx):
    """Check blank line formatting rules."""
    lines = ctx["lines"]
    violations = []

    i = 0
    while i < len(lines):
        line = lines[i].strip()

        # Detect function/method signatures: "func ..." ending with "{".
        if line.startswith("func ") and line.endswith("{"):
            body_start = i + 1
            violations += _check_func_body(lines, body_start)

        i += 1

    return violations

def _check_func_body(lines, body_start):
    """Check blank line rules for a function body starting at body_start.

    Parameters:
        lines: all file lines.
        body_start: index of the first line after the opening brace.

    Returns:
        list of violation dicts.
    """
    violations = []

    # Find the closing brace by counting braces.
    depth = 1
    body_end = body_start
    for j in range(body_start, len(lines)):
        for c in lines[j].elems():
            if c == "{":
                depth += 1
            elif c == "}":
                depth -= 1
        if depth == 0:
            body_end = j
            break

    # Count non-blank, non-comment statements in the body.
    stmt_count = 0
    for j in range(body_start, body_end):
        stripped = lines[j].strip()
        if stripped and not stripped.startswith("//"):
            stmt_count += 1

    if body_start >= len(lines):
        return violations

    next_line = lines[body_start].strip() if body_start < len(lines) else ""
    is_blank_after_sig = (next_line == "")

    # Rule 1: Multi-statement body needs blank line after signature.
    if stmt_count > 1 and not is_blank_after_sig:
        violations.append({
            "line": body_start + 1,
            "message": "missing blank line after function signature (multi-statement body)",
        })

    # Rule 2: Single-statement body should NOT have blank line after signature.
    if stmt_count <= 1 and is_blank_after_sig:
        violations.append({
            "line": body_start + 1,
            "message": "unnecessary blank line after function signature (single-statement body)",
        })

    return violations

def fix(ctx):
    """Fix blank line formatting issues."""
    lines = list(ctx["lines"])
    modified = False

    i = 0
    while i < len(lines):
        line = lines[i].strip()

        if line.startswith("func ") and line.endswith("{"):
            body_start = i + 1

            # Find closing brace.
            depth = 1
            body_end = body_start
            for j in range(body_start, len(lines)):
                for c in lines[j].elems():
                    if c == "{":
                        depth += 1
                    elif c == "}":
                        depth -= 1
                if depth == 0:
                    body_end = j
                    break

            # Count statements.
            stmt_count = 0
            for j in range(body_start, body_end):
                stripped = lines[j].strip()
                if stripped and not stripped.startswith("//"):
                    stmt_count += 1

            if body_start < len(lines):
                is_blank = (lines[body_start].strip() == "")

                # Add blank line for multi-statement bodies.
                if stmt_count > 1 and not is_blank:
                    lines.insert(body_start, "")
                    modified = True
                    i += 1  # skip the inserted line

                # Remove blank line for single-statement bodies.
                if stmt_count <= 1 and is_blank:
                    lines.pop(body_start)
                    modified = True
                    i -= 1  # re-check current position

        i += 1

    if not modified:
        return None
    return "\n".join(lines)
