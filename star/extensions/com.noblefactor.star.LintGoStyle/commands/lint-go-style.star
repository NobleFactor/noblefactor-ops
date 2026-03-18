# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint-go-style.star — Go style guidelines enforcement
#
# Orchestrator for the pluggable Go style linter. Discovers rules, collects
# Go source files, and dispatches check/fix calls to each rule.

load("rules/line-width.star", lw_check = "check", lw_fix = "fix")
load("rules/formatting.star", fmt_check = "check", fmt_fix = "fix")
load("rules/receivers.star", recv_check = "check", recv_fix = "fix")
load("rules/file-layout.star", fl_check = "check", fl_fix = "fix")
load("rules/regions.star", reg_check = "check", reg_fix = "fix")
load("rules/method-order.star", mo_check = "check", mo_fix = "fix")
load("rules/doc-comments.star", dc_check = "check", dc_fix = "fix")

# =============================================================================
# Rule Registry
# =============================================================================

# Rules in execution order. Each entry: (name, check_fn, fix_fn).
ALL_RULES = [
    ("line-width", lw_check, lw_fix),
    ("formatting", fmt_check, fmt_fix),
    ("receivers", recv_check, recv_fix),
    ("file-layout", fl_check, fl_fix),
    ("regions", reg_check, reg_fix),
    ("method-order", mo_check, mo_fix),
    ("doc-comments", dc_check, dc_fix),
]

def active_rules(rule_filter, disabled_rules):
    """Return the list of active rules after applying filters.

    Parameters:
        rule_filter: if non-empty, only include this single rule name.
        disabled_rules: list of rule names to skip.

    Returns:
        list of tuples: (name, check_fn, fix_fn).
    """
    result = []
    for name, check_fn, fix_fn in ALL_RULES:
        if rule_filter and name != rule_filter:
            continue
        if name in disabled_rules:
            continue
        result.append((name, check_fn, fix_fn))
    return result

# =============================================================================
# File Collection
# =============================================================================

def collect_go_files(path, exclude_patterns, include_generated, include_tests):
    """Collect Go source files from path, respecting filters.

    Parameters:
        path: root directory to scan.
        exclude_patterns: list of glob patterns to exclude.
        include_generated: if False, skip files containing "DO NOT EDIT".
        include_tests: if False, skip _test.go files.

    Returns:
        list of string: file paths.
    """
    pattern = path + "/**/*.go"
    all_files = file.find(pattern)

    result = []
    for f in all_files:
        if not include_tests and f.endswith("_test.go"):
            continue
        if is_excluded(f, exclude_patterns):
            continue
        if not include_generated and is_generated(f):
            continue
        result.append(f)

    return sorted(result)

def is_excluded(path, patterns):
    """Check if a path matches any exclude pattern."""
    for pattern in patterns:
        if regexp.match(glob_to_regex(pattern), path):
            return True
    return False

def glob_to_regex(pattern):
    """Convert a glob pattern to a regex."""
    result = ""
    i = 0
    while i < len(pattern):
        c = pattern[i]
        if c == "*" and i + 1 < len(pattern) and pattern[i + 1] == "*":
            result += ".*"
            i += 2
            if i < len(pattern) and pattern[i] == "/":
                i += 1
            continue
        elif c == "*":
            result += "[^/]*"
        elif c == "?":
            result += "[^/]"
        elif c in ".+^${}()|[]":
            result += "\\" + c
        else:
            result += c
        i += 1
    return result

def is_generated(path):
    """Check if a Go file is generated."""
    content = file.read_text(path)
    for line in content.split("\n")[:10]:
        if "DO NOT EDIT" in line or "Code generated" in line:
            return True
    return False

# =============================================================================
# Rule Context and Dispatch
# =============================================================================

def build_context(path, line_width):
    """Build the rule context for a single file.

    Parameters:
        path: the Go source file path.
        line_width: the configured line width.

    Returns:
        dict: the rule context.
    """
    content = file.read_text(path)
    return {
        "path": path,
        "content": content,
        "lines": content.split("\n"),
        "config": {"line_width": line_width},
    }

def run_check(rules, files, verbose, line_width):
    """Run check mode on all files.

    Returns:
        list of violation dicts.
    """
    violations = []
    for path in files:
        if verbose:
            ui.note("Checking " + path)
        ctx = build_context(path, line_width)
        for name, check_fn, _ in rules:
            for v in check_fn(ctx):
                violations.append({
                    "file": path,
                    "line": v.get("line", 0),
                    "message": v.get("message", ""),
                    "rule": name,
                })
    return violations

def run_fix(rules, files, verbose, line_width):
    """Run fix mode on all files.

    Returns:
        int: number of files modified.
    """
    fixed_count = 0
    for path in files:
        if verbose:
            ui.note("Fixing " + path)
        original = file.read_text(path)
        for _, _, fix_fn in rules:
            # Rebuild context from disk before each rule so goast sees current content.
            ctx = build_context(path, line_width)
            new_content = fix_fn(ctx)
            if new_content and new_content != ctx["content"]:
                file.write_text(path, new_content)
        # Check if the file changed overall.
        final = file.read_text(path)
        if final != original:
            fixed_count += 1
    return fixed_count

# =============================================================================
# Command Entry Point
# =============================================================================

def run(ctx):
    """Enforce Go style guidelines on Go source files."""
    fix_mode = ctx.args.get("fix", "false") == "true"
    path = ctx.args.get("path", ".")
    exclude_str = ctx.args.get("exclude", "")
    include_generated = ctx.args.get("generated", "true") == "true"
    include_tests = ctx.args.get("tests", "true") == "true"
    rule_filter = ctx.args.get("rule", "")
    verbose = ctx.args.get("verbose", "false") == "true"

    # Load config.
    cfg = config.get()
    go_style_cfg = cfg.lint.go_style
    line_width = go_style_cfg.line_width
    disabled_rules = list(go_style_cfg.disabled_rules)

    # Merge exclude patterns.
    exclude_patterns = []
    if exclude_str:
        exclude_patterns = exclude_str.split(",")
    for p in go_style_cfg.exclude:
        if p not in exclude_patterns:
            exclude_patterns.append(p)

    # Select active rules.
    rules = active_rules(rule_filter, disabled_rules)
    if verbose:
        if rules:
            ui.note("Rules: " + ", ".join([r[0] for r in rules]))
        else:
            ui.note("No rules active")

    # Collect files.
    files = collect_go_files(path, exclude_patterns, include_generated, include_tests)
    if not files:
        ui.success("No Go files found to check")
        return
    if verbose:
        ui.note("Found " + str(len(files)) + " Go file(s)")

    # Dispatch.
    if fix_mode:
        fixed_count = run_fix(rules, files, verbose, line_width)
        if fixed_count > 0:
            ui.success("Fixed " + str(fixed_count) + " file(s)")
        else:
            ui.success("All " + str(len(files)) + " file(s) compliant")
    else:
        violations = run_check(rules, files, verbose, line_width)
        if violations:
            for v in violations:
                if v["line"] > 0:
                    ui.warn(v["file"] + ":" + str(v["line"]) + " [" + v["rule"] + "] " + v["message"])
                else:
                    ui.warn(v["file"] + " [" + v["rule"] + "] " + v["message"])
            ui.fail("Found " + str(len(violations)) + " violation(s) in " + str(len(files)) + " file(s)")
        else:
            ui.success("All " + str(len(files)) + " file(s) compliant")
