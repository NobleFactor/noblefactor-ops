# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# regions.star — Method region hierarchy markers
#
# Checks every struct with methods for required region markers:
# - // region EXPORTED METHODS / // endregion
# - // region UNEXPORTED METHODS / // endregion
# - // region State management / // endregion (if getters/setters exist)
# - // region Behaviors / // endregion (if non-getter methods exist)
# Empty sub-regions are omitted (not flagged as missing).

def check(ctx):
    """Check for required region markers around method groups."""
    lines = ctx["lines"]
    path = ctx["path"]
    violations = []

    structs = goast.structs(path=path)
    methods = goast.methods(path=path)

    # Group methods by base receiver type.
    by_type = {}
    for m in methods:
        base = m.receiver_type
        if base.startswith("*"):
            base = base[1:]
        if base not in by_type:
            by_type[base] = []
        by_type[base].append(m)

    # Find existing region markers.
    regions = _find_regions(lines)

    for s in structs:
        type_name = s.name
        type_methods = by_type.get(type_name, [])
        if not type_methods:
            continue

        has_exported = False
        has_unexported = False
        for m in type_methods:
            if m.name[0].isupper():
                has_exported = True
            else:
                has_unexported = True

        # Check for top-level region markers.
        if has_exported and "EXPORTED METHODS" not in regions:
            violations.append({
                "line": type_methods[0].line,
                "message": type_name + ": missing '// region EXPORTED METHODS' marker",
            })

        if has_unexported and "UNEXPORTED METHODS" not in regions:
            violations.append({
                "line": type_methods[0].line,
                "message": type_name + ": missing '// region UNEXPORTED METHODS' marker",
            })

    return violations

def _find_regions(lines):
    """Find all region names present in the file.

    Returns:
        set of region name strings.
    """
    regions = {}
    for i, line in enumerate(lines):
        stripped = line.strip()
        if stripped.startswith("// region "):
            name = stripped[len("// region "):]
            regions[name] = i + 1
    return regions

def fix(ctx):
    """Insert missing region markers around method groups.

    This is a best-effort insertion — it adds markers before the first and after
    the last method in each group. Does not reorder methods.
    """
    lines = list(ctx["lines"])
    path = ctx["path"]
    modified = False

    structs = goast.structs(path=path)
    methods = goast.methods(path=path)

    by_type = {}
    for m in methods:
        base = m.receiver_type
        if base.startswith("*"):
            base = base[1:]
        if base not in by_type:
            by_type[base] = []
        by_type[base].append(m)

    regions = _find_regions(lines)

    # Process in reverse so line numbers stay valid.
    for s in reversed(list(structs)):
        type_methods = by_type.get(s.name, [])
        if not type_methods:
            continue

        exported = [m for m in type_methods if m.name[0].isupper()]
        unexported = [m for m in type_methods if not m.name[0].isupper()]

        # Insert UNEXPORTED METHODS region if missing.
        if unexported and "UNEXPORTED METHODS" not in regions:
            last = unexported[-1]
            first = unexported[0]
            # Find the line after the last unexported method's closing brace.
            end_line = _find_func_end(lines, last.line - 1)
            if end_line < len(lines):
                lines.insert(end_line + 1, "")
                lines.insert(end_line + 2, "// endregion")
                modified = True
            # Insert region marker before the first unexported method's doc comment.
            insert_at = _find_doc_start(lines, first.line - 1)
            lines.insert(insert_at, "// region UNEXPORTED METHODS")
            lines.insert(insert_at + 1, "")
            modified = True

        # Insert EXPORTED METHODS region if missing.
        if exported and "EXPORTED METHODS" not in regions:
            last = exported[-1]
            first = exported[0]
            end_line = _find_func_end(lines, last.line - 1)
            if end_line < len(lines):
                lines.insert(end_line + 1, "")
                lines.insert(end_line + 2, "// endregion")
                modified = True
            insert_at = _find_doc_start(lines, first.line - 1)
            lines.insert(insert_at, "// region EXPORTED METHODS")
            lines.insert(insert_at + 1, "")
            modified = True

    if not modified:
        return None
    return "\n".join(lines)

def _find_func_end(lines, func_line):
    """Find the closing brace line of a function starting at func_line (0-indexed)."""
    depth = 0
    for i in range(func_line, len(lines)):
        for c in lines[i].elems():
            if c == "{":
                depth += 1
            elif c == "}":
                depth -= 1
        if depth == 0 and i > func_line:
            return i
    return func_line

def _find_doc_start(lines, func_line):
    """Find the first line of the doc comment above a function (0-indexed)."""
    i = func_line - 1
    while i >= 0 and lines[i].strip().startswith("//"):
        i -= 1
    return i + 1
