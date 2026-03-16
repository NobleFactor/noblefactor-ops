# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint-go-style.star — Go style guidelines enforcement
#
# Orchestrator for the pluggable Go style linter. Discovers rules, collects
# Go source files, and dispatches check/fix calls to each rule.

# =============================================================================
# Constants
# =============================================================================

# Built-in rule names in execution order.
BUILTIN_RULES = [
    "doc-comments",
    "regions",
    "method-order",
    "file-layout",
    "receivers",
    "line-width",
    "formatting",
]

# =============================================================================
# Rule Discovery
# =============================================================================

def discover_rules(extension_dir, override_dir, rule_filter, disabled_rules):
    """Discover rules from built-in and override directories.

    Parameters:
        extension_dir: path to the extension's rules/ directory.
        override_dir: path to the project's .star/lint/go-style/ override directory.
        rule_filter: if non-empty, only load this single rule name.
        disabled_rules: list of rule names to skip.

    Returns:
        list of dicts: each with "name" and "path" keys.
    """
    rules = []

    for name in BUILTIN_RULES:
        if rule_filter and name != rule_filter:
            continue
        if name in disabled_rules:
            continue

        # Check for project override first.
        override_path = override_dir + "/" + name + ".star"
        builtin_path = extension_dir + "/rules/" + name + ".star"

        if file.exists(override_path):
            rules.append({"name": name, "path": override_path})
        elif file.exists(builtin_path):
            rules.append({"name": name, "path": builtin_path})
        # else: rule script not yet implemented — skip silently.

    return rules

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
        list of string: absolute file paths.
    """
    pattern = path + "/**/*.go"
    all_files = file.glob(pattern)

    result = []
    for f in all_files:
        # Skip test files if requested.
        if not include_tests and f.endswith("_test.go"):
            continue

        # Skip excluded patterns.
        if is_excluded(f, exclude_patterns):
            continue

        # Skip generated files if requested.
        if not include_generated and is_generated(f):
            continue

        result.append(f)

    return sorted(result)

def is_excluded(path, patterns):
    """Check if a path matches any exclude pattern.

    Parameters:
        path: the file path to check.
        patterns: list of glob patterns.

    Returns:
        bool: True if excluded.
    """
    for pattern in patterns:
        if regexp.match(glob_to_regex(pattern), path):
            return True
    return False

def glob_to_regex(pattern):
    """Convert a simple glob pattern to a regex.

    Parameters:
        pattern: glob pattern with * and ** wildcards.

    Returns:
        string: equivalent regex pattern.
    """
    # Escape regex special chars except * and /.
    result = ""
    i = 0
    while i < len(pattern):
        c = pattern[i]
        if c == "*" and i + 1 < len(pattern) and pattern[i + 1] == "*":
            result += ".*"
            i += 2
            # Skip trailing slash after **.
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
    """Check if a Go file is generated (contains DO NOT EDIT marker).

    Parameters:
        path: the file path.

    Returns:
        bool: True if the file is generated.
    """
    content = file.read(path)
    # Check first few lines for the standard generated marker.
    lines = content.split("\n")
    for line in lines[:10]:
        if "DO NOT EDIT" in line or "Code generated" in line:
            return True
    return False

# =============================================================================
# Rule Loading and Execution
# =============================================================================

def load_rule(rule_info):
    """Load a rule script and validate its contract.

    Parameters:
        rule_info: dict with "name" and "path" keys.

    Returns:
        dict with "name", "check", and "fix" functions, or None on error.
    """
    content = file.read(rule_info["path"])
    globals = {}

    # Execute the rule script to populate its globals.
    # Starlark exec is not available — rules are loaded by the framework.
    # For now, return None; rules will be loaded when implemented in Phase 3.
    return None

def build_rule_context(path, verbose):
    """Build the RuleContext for a single file.

    Parameters:
        path: the Go source file path.
        verbose: whether to log progress.

    Returns:
        dict: the rule context with file data and goast accessors.
    """
    content = file.read(path)
    lines = content.split("\n")
    funcs = goast.funcs(path=path)
    structs = goast.structs(path=path)

    return {
        "path": path,
        "content": content,
        "lines": lines,
        "funcs": funcs,
        "structs": structs,
    }

def run_check(rules, files, verbose, line_width):
    """Run check mode: collect violations from all rules on all files.

    Parameters:
        rules: list of loaded rule dicts.
        files: list of file paths.
        verbose: whether to log per-file progress.
        line_width: the configured line width.

    Returns:
        list of dicts: violations with "file", "line", "message", "rule" keys.
    """
    violations = []

    for path in files:
        if verbose:
            ui.note("Checking " + path)

        ctx = build_rule_context(path, verbose)
        ctx["config"] = {"line_width": line_width}

        for rule in rules:
            check_fn = rule.get("check")
            if check_fn:
                rule_violations = check_fn(ctx)
                if rule_violations:
                    for v in rule_violations:
                        violations.append({
                            "file": path,
                            "line": v.get("line", 0),
                            "message": v.get("message", ""),
                            "rule": rule["name"],
                        })

    return violations

def run_fix(rules, files, verbose, line_width):
    """Run fix mode: apply fixes from all rules on all files.

    Parameters:
        rules: list of loaded rule dicts.
        files: list of file paths.
        verbose: whether to log per-file progress.
        line_width: the configured line width.

    Returns:
        int: number of files modified.
    """
    fixed_count = 0

    for path in files:
        if verbose:
            ui.note("Fixing " + path)

        ctx = build_rule_context(path, verbose)
        ctx["config"] = {"line_width": line_width}
        modified = False

        for rule in rules:
            fix_fn = rule.get("fix")
            if fix_fn:
                new_content = fix_fn(ctx)
                if new_content and new_content != ctx["content"]:
                    ctx["content"] = new_content
                    ctx["lines"] = new_content.split("\n")
                    modified = True

        if modified:
            file.write(path, ctx["content"])
            fixed_count += 1

    return fixed_count

# =============================================================================
# Command Entry Point
# =============================================================================

def run(ctx):
    """Enforce Go style guidelines on Go source files."""

    # Parse arguments.
    fix_mode = ctx.args.get("fix", "false") == "true"
    path = ctx.args.get("path", ".")
    exclude_str = ctx.args.get("exclude", "")
    include_generated = ctx.args.get("generated", "true") == "true"
    include_tests = ctx.args.get("tests", "true") == "true"
    rule_filter = ctx.args.get("rule", "")
    verbose = ctx.args.get("verbose", "false") == "true"

    # Load config (attribute access — defaults from extension.yaml).
    cfg = config.get()
    go_style_cfg = cfg.lint.go_style
    line_width = go_style_cfg.line_width
    disabled_rules = list(go_style_cfg.disabled_rules)

    # Merge exclude patterns from CLI and config.
    exclude_patterns = []
    if exclude_str:
        exclude_patterns = exclude_str.split(",")
    for p in go_style_cfg.exclude:
        if p not in exclude_patterns:
            exclude_patterns.append(p)

    # Discover rules.
    extension_dir = ctx.extension_dir if hasattr(ctx, "extension_dir") else ""
    override_dir = ".star/lint/go-style"
    rules_info = discover_rules(extension_dir, override_dir, rule_filter, disabled_rules)

    if verbose:
        if rules_info:
            ui.note("Rules: " + ", ".join([r["name"] for r in rules_info]))
        else:
            ui.note("No rules found")

    # Load rule scripts.
    rules = []
    for info in rules_info:
        loaded = load_rule(info)
        if loaded:
            rules.append(loaded)

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
            # Report violations grouped by file.
            current_file = ""
            for v in violations:
                if v["file"] != current_file:
                    current_file = v["file"]
                if v["line"] > 0:
                    ui.warn(v["file"] + ":" + str(v["line"]) + " [" + v["rule"] + "] " + v["message"])
                else:
                    ui.warn(v["file"] + " [" + v["rule"] + "] " + v["message"])

            ui.fail("Found " + str(len(violations)) + " violation(s) in " + str(len(files)) + " file(s)")
        else:
            ui.success("All " + str(len(files)) + " file(s) compliant")
