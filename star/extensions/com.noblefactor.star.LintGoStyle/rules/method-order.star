# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# method-order.star — Method sort order within regions
#
# Checks within each region/delineator group:
# 1. State management before Behaviors
# 2. Within Behaviors: Compensable → Fallible → Actions
# 3. Within each group: alphabetical (CompensateX immediately follows X)

def check(ctx):
    """Check method ordering within regions."""
    lines = ctx["lines"]
    path = ctx["path"]
    violations = []

    methods = goast.methods(path=path)
    if len(methods) < 2:
        return violations

    # Group methods by their region/delineator context.
    groups = _group_methods_by_region(lines, methods)

    for group in groups:
        if len(group) < 2:
            continue
        # Check alphabetical order within the group.
        for i in range(1, len(group)):
            prev = group[i - 1]
            curr = group[i]
            # CompensateX must immediately follow X — skip alpha check for these pairs.
            if curr.name.startswith("Compensate") and curr.name[len("Compensate"):] == prev.name:
                continue
            if prev.name.startswith("Compensate"):
                base = prev.name[len("Compensate"):]
                if i >= 2 and group[i - 2].name == base:
                    continue
            if curr.name < prev.name:
                violations.append({
                    "line": curr.line,
                    "message": curr.name + " should come before " + prev.name + " (alphabetical within group)",
                })

    return violations

def _group_methods_by_region(lines, methods):
    """Group methods by the region/delineator they appear in.

    Methods between two delineator comments form a group. Methods before the first
    delineator or after the last form their own groups.

    Parameters:
        lines: all file lines.
        methods: list of MethodResult from goast.

    Returns:
        list of lists of MethodResult.
    """
    # Find delineator lines (// Compensable actions, // Fallible actions, // Actions, etc.)
    delineators = []
    for i, line in enumerate(lines):
        stripped = line.strip()
        if stripped in [
            "// Compensable actions",
            "// Fallible actions",
            "// Actions",
            "// region State management",
            "// region Behaviors",
            "// region EXPORTED METHODS",
            "// region UNEXPORTED METHODS",
            "// endregion",
        ]:
            delineators.append(i + 1)  # 1-indexed

    if not delineators:
        # No delineators — all methods in one group.
        return [list(methods)]

    # Split methods into groups by delineator boundaries.
    groups = []
    current_group = []
    delineator_idx = 0

    for m in methods:
        # Advance past any delineators before this method.
        while delineator_idx < len(delineators) and delineators[delineator_idx] < m.line:
            if current_group:
                groups.append(current_group)
                current_group = []
            delineator_idx += 1
        current_group.append(m)

    if current_group:
        groups.append(current_group)

    return groups

def fix(ctx):
    """Fix method ordering by calling goast.sort_declarations on each disordered group."""
    lines = ctx["lines"]
    path = ctx["path"]
    methods = goast.methods(path=path)

    if len(methods) < 2:
        return None

    groups = _group_methods_by_region(lines, methods)
    content = ctx["content"]
    modified = False

    # Process groups in reverse order so line numbers stay valid.
    for group in reversed(groups):
        if len(group) < 2:
            continue

        # Check if this group is already sorted.
        sorted_ok = True
        for i in range(1, len(group)):
            prev = group[i - 1]
            curr = group[i]
            if curr.name.startswith("Compensate") and curr.name[len("Compensate"):] == prev.name:
                continue
            if prev.name.startswith("Compensate"):
                base = prev.name[len("Compensate"):]
                if i >= 2 and group[i - 2].name == base:
                    continue
            if curr.name < prev.name:
                sorted_ok = False
                break

        if sorted_ok:
            continue

        # Sort the group's line range.
        first_line = group[0].line
        last_line = group[-1].line
        # Extend last_line to the end of the function body.
        last_end = _find_func_end_line(content.split("\n"), last_line - 1)
        scope = "lines:" + str(first_line) + "-" + str(last_end + 1)

        new_content = goast.sort_declarations(path=path, scope=scope, order="alphabetical")
        if new_content != content:
            content = new_content
            modified = True

    if not modified:
        return None
    return content

def _find_func_end_line(lines, start):
    """Find the closing brace line of a function (0-indexed)."""
    depth = 0
    for i in range(start, len(lines)):
        for c in lines[i].elems():
            if c == "{":
                depth += 1
            elif c == "}":
                depth -= 1
        if depth == 0 and i > start:
            return i
    return start
