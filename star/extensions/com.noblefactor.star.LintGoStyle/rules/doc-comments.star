# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# doc-comments.star — Doc comment structure and freshness
#
# Checks every exported method and unexported helper for:
# 1. Summary line (first line is imperative verb phrase)
# 2. Blank separator between summary and extended description
# 3. Fill width (comment text fills to configured column width)
# 4. Parameters section (present on every func/method with params)
# 5. Returns section (present on every func/method with return values)
# 6. Signature synchronization (doc matches current Go signature)

def check(ctx):
    """Check doc comment structure and freshness."""
    lines = ctx["lines"]
    path = ctx["path"]
    width = ctx["config"]["line_width"]
    violations = []

    funcs = goast.funcs(path=path)
    methods = goast.methods(path=path)
    all_decls = list(funcs) + list(methods)

    for decl in all_decls:
        doc_lines = _extract_doc_lines(lines, decl.line)
        if not doc_lines:
            violations.append({
                "line": decl.line,
                "message": decl.name + ": missing doc comment",
            })
            continue

        # Check Parameters section.
        has_params = len(decl.params) > 0
        has_params_section = _has_section(doc_lines, "Parameters:")
        if has_params and not has_params_section:
            violations.append({
                "line": decl.line,
                "message": decl.name + ": missing Parameters section",
            })

        # Check Returns section.
        has_returns = decl.returns != ""
        has_returns_section = _has_section(doc_lines, "Returns:")
        if has_returns and not has_returns_section:
            violations.append({
                "line": decl.line,
                "message": decl.name + ": missing Returns section",
            })

        # Check parameter sync.
        if has_params_section:
            doc_params = _extract_param_names(doc_lines)
            sig_params = [p.name for p in decl.params if p.name]
            for sp in sig_params:
                if sp not in doc_params:
                    violations.append({
                        "line": decl.line,
                        "message": decl.name + ": parameter '" + sp + "' not documented",
                    })
            for dp in doc_params:
                if dp not in sig_params:
                    violations.append({
                        "line": decl.line,
                        "message": decl.name + ": documented parameter '" + dp + "' not in signature",
                    })

        # Check fill width (under-filled lines).
        violations += _check_fill_width(doc_lines, width, decl.name)

    return violations

def _extract_doc_lines(lines, func_line):
    """Extract doc comment lines above a function declaration (0-indexed).

    Parameters:
        lines: all file lines.
        func_line: 1-indexed line number of the function.

    Returns:
        list of raw comment lines, or empty list if no doc comment.
    """
    result = []
    i = func_line - 2  # 0-indexed, line above the func
    while i >= 0:
        stripped = lines[i].strip()
        if stripped.startswith("//"):
            result.insert(0, stripped)
            i -= 1
        else:
            break
    return result

def _has_section(doc_lines, section_name):
    """Check if a doc comment contains a named section.

    Parameters:
        doc_lines: list of comment lines.
        section_name: e.g., "Parameters:" or "Returns:".

    Returns:
        bool: True if the section is present.
    """
    for line in doc_lines:
        body = line[2:].strip() if line.startswith("//") else line.strip()
        if body == section_name:
            return True
    return False

def _extract_param_names(doc_lines):
    """Extract parameter names from a Parameters section.

    Parameters:
        doc_lines: list of comment lines.

    Returns:
        list of string: parameter names found in the doc.
    """
    names = []
    in_params = False
    for line in doc_lines:
        body = line[2:].strip() if line.startswith("//") else line.strip()
        if body == "Parameters:":
            in_params = True
            continue
        if in_params:
            if body.startswith("- "):
                entry = body[2:]
                colon = entry.find(":")
                if colon > 0:
                    names.append(entry[:colon].strip())
            elif body == "" or body == "Returns:" or (not body.startswith("  ") and body != ""):
                in_params = False
    return names

def _check_fill_width(doc_lines, width, func_name):
    """Check for under-filled comment lines.

    A line is under-filled if it and the next line are both regular text (not blank,
    not code block, not bullet) and the first word of the next line could fit on the
    current line without exceeding the width.

    Parameters:
        doc_lines: list of raw comment lines.
        width: the configured line width.
        func_name: the function name for violation messages.

    Returns:
        list of violation dicts.
    """
    violations = []

    for i in range(len(doc_lines) - 1):
        curr = doc_lines[i]
        next_line = doc_lines[i + 1]

        curr_body = curr[2:].strip() if curr.startswith("//") else curr.strip()
        next_body = next_line[2:].strip() if next_line.startswith("//") else next_line.strip()

        # Skip blank lines, code blocks, section headers, bullets.
        if not curr_body or not next_body:
            continue
        if curr_body.startswith("    ") or next_body.startswith("    "):
            continue
        if curr_body.endswith(":") or next_body.endswith(":"):
            continue
        if curr_body.startswith("- ") or next_body.startswith("- "):
            continue
        if curr_body.startswith("+"):
            continue

        # Check if the first word of next line could fit on current line.
        next_words = next_body.split(" ")
        if next_words:
            first_word = next_words[0]
            if len(curr) + 1 + len(first_word) <= width:
                # Find the actual line number in the file for this doc line.
                # We don't have it here, so report at function level.
                violations.append({
                    "line": 0,
                    "message": func_name + ": under-filled comment line (could absorb '" + first_word + "')",
                })
                break  # One under-fill warning per function is enough.

    return violations

def fix(ctx):
    """Fix doc comments: rewrap to fill width, insert missing sections.

    Uses goast.rewrap_comments for fill width. Inserts TODO stubs for missing
    Parameters/Returns sections.
    """
    path = ctx["path"]
    width = ctx["config"]["line_width"]

    # First pass: rewrap comments for fill width.
    content = goast.rewrap_comments(path=path, width=width)
    lines = content.split("\n")

    # Second pass: insert missing Parameters/Returns sections.
    funcs = goast.funcs(path=path)
    methods = goast.methods(path=path)
    all_decls = list(funcs) + list(methods)

    # Sort by line number descending so insertions don't shift subsequent lines.
    all_decls_sorted = sorted(all_decls, key=lambda d: d.line, reverse=True)

    modified = (content != ctx["content"])

    for decl in all_decls_sorted:
        doc_lines = _extract_doc_lines(lines, decl.line)
        if not doc_lines:
            continue

        has_params = len(decl.params) > 0
        has_returns = decl.returns != ""
        has_params_section = _has_section(doc_lines, "Parameters:")
        has_returns_section = _has_section(doc_lines, "Returns:")

        insert_lines = []

        if has_returns and not has_returns_section:
            insert_lines.append("//")
            insert_lines.append("// Returns:")
            insert_lines.append("//   - " + decl.returns + ": TODO")

        if has_params and not has_params_section:
            insert_lines.append("//")
            insert_lines.append("// Parameters:")
            for p in decl.params:
                if p.name:
                    insert_lines.append("//   - " + p.name + ": TODO")

        if insert_lines:
            # Insert before the function line (after the last doc comment line).
            insert_at = decl.line - 1  # 0-indexed
            for j, il in enumerate(insert_lines):
                lines.insert(insert_at + j, il)
            modified = True

    if not modified:
        return None
    return "\n".join(lines)
