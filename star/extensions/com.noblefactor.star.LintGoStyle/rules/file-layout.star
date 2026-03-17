# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# file-layout.star — File element ordering
#
# Checks the order of top-level declarations:
# copyright → package → imports → init → guards → vars → main struct → types → methods
#
# Fix: check-only. Reordering top-level declarations is too risky for auto-fix.

# Expected order of top-level elements. Each element maps to a numeric rank.
ELEMENT_ORDER = {
    "copyright": 0,
    "package": 1,
    "import": 2,
    "init": 3,
    "guard": 4,
    "var": 5,
    "const": 6,
    "type": 7,
    "func": 8,
}

def check(ctx):
    """Check that top-level declarations follow the expected order."""
    lines = ctx["lines"]
    violations = []

    elements = _classify_elements(lines)
    if len(elements) < 2:
        return violations

    # Check that elements are in non-decreasing order.
    prev_rank = -1
    prev_name = ""
    for elem in elements:
        rank = ELEMENT_ORDER.get(elem["kind"], 99)
        if rank < prev_rank:
            violations.append({
                "line": elem["line"],
                "message": elem["kind"] + " declaration should come before " + prev_name + " (file layout order)",
            })
        prev_rank = rank
        prev_name = elem["kind"]

    return violations

def _classify_elements(lines):
    """Classify top-level elements by kind and line number.

    Parameters:
        lines: all file lines.

    Returns:
        list of dicts with "kind" and "line" keys.
    """
    elements = []
    i = 0

    while i < len(lines):
        stripped = lines[i].strip()

        # Copyright header (SPDX line).
        if stripped.startswith("// SPDX-License-Identifier:"):
            elements.append({"kind": "copyright", "line": i + 1})
            i += 1
            continue

        # Package declaration.
        if stripped.startswith("package "):
            elements.append({"kind": "package", "line": i + 1})
            i += 1
            continue

        # Import block.
        if stripped.startswith("import "):
            elements.append({"kind": "import", "line": i + 1})
            # Skip past the import block.
            if "(" in stripped:
                while i < len(lines) and not lines[i].strip().startswith(")"):
                    i += 1
            i += 1
            continue

        # Init function.
        if stripped.startswith("func init()"):
            elements.append({"kind": "init", "line": i + 1})
            i = _skip_func_body(lines, i)
            continue

        # Interface guard: var _ Interface = (*Type)(nil).
        if stripped.startswith("var _"):
            elements.append({"kind": "guard", "line": i + 1})
            i += 1
            continue

        # Var block.
        if stripped.startswith("var ") or stripped == "var (":
            elements.append({"kind": "var", "line": i + 1})
            if "(" in stripped:
                while i < len(lines) and not lines[i].strip().startswith(")"):
                    i += 1
            i += 1
            continue

        # Const block.
        if stripped.startswith("const ") or stripped == "const (":
            elements.append({"kind": "const", "line": i + 1})
            if "(" in stripped:
                while i < len(lines) and not lines[i].strip().startswith(")"):
                    i += 1
            i += 1
            continue

        # Type declaration.
        if stripped.startswith("type "):
            elements.append({"kind": "type", "line": i + 1})
            # Skip struct/interface body if present.
            if stripped.endswith("{"):
                i = _skip_brace_block(lines, i)
            else:
                i += 1
            continue

        # Function/method.
        if stripped.startswith("func "):
            elements.append({"kind": "func", "line": i + 1})
            i = _skip_func_body(lines, i)
            continue

        i += 1

    return elements

def _skip_func_body(lines, start):
    """Skip past a function body starting at the signature line.

    Returns:
        int: the index after the closing brace.
    """
    # Find opening brace.
    i = start
    while i < len(lines) and "{" not in lines[i]:
        i += 1
    return _skip_brace_block(lines, i)

def _skip_brace_block(lines, start):
    """Skip past a brace-delimited block.

    Returns:
        int: the index after the closing brace.
    """
    depth = 0
    i = start
    while i < len(lines):
        for c in lines[i].elems():
            if c == "{":
                depth += 1
            elif c == "}":
                depth -= 1
        if depth == 0:
            return i + 1
        i += 1
    return i

def fix(ctx):
    """No auto-fix — reordering top-level declarations is too risky."""
    return None
