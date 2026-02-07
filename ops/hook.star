# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# hook.star - Git hook execution commands
#
# These commands are called by git hooks installed via `star setup hooks`.
# They run the configured quality checks for each hook type.
#
# Usage:
#   star hook pre-commit    # Run pre-commit checks (called by git)
#   star hook pre-push      # Run pre-push checks (called by git)

def run_pre_commit(ctx):
    """Run pre-commit checks."""
    # Run all linters
    failures = []

    # Go lint
    go_result = run_linter("go")
    if not go_result:
        failures.append("go")

    # Shell lint
    shell_result = run_linter("shell")
    if not shell_result:
        failures.append("shell")

    # Markdown lint
    md_result = run_linter("markdown")
    if not md_result:
        failures.append("markdown")

    # Summary
    if len(failures) == 0:
        success("All pre-commit checks passed")
    else:
        fail("Pre-commit checks failed: " + ", ".join(failures))

def run_linter(name):
    """Run a single linter, return True if passed."""
    if name == "go":
        return run_go_check()
    elif name == "shell":
        return run_shell_check()
    elif name == "markdown":
        return run_markdown_check()
    return True

def run_go_check():
    """Run Go lint checks, return True if passed."""
    tool = lint.ensure_tools()
    for t in tool.tools:
        if t.name == "golangci-lint" and not t.installed:
            error("golangci-lint not installed")
            return False

    result = lint.go(path="./...", config="", skip_mod_tidy=False)

    if not result.mod_tidy_passed:
        error("go.mod is not tidy - run 'go mod tidy'")

    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + " " + issue.linter + ": " + issue.message
        if issue.severity == "error":
            error(msg)
        else:
            warn(msg)

    return result.passed

def run_shell_check():
    """Run shell lint checks, return True if passed."""
    tool = lint.ensure_tools()
    has_shellcheck = False
    has_shfmt = False
    for t in tool.tools:
        if t.name == "shellcheck":
            has_shellcheck = t.installed
        if t.name == "shfmt":
            has_shfmt = t.installed

    if not has_shellcheck or not has_shfmt:
        # Skip silently if tools not installed
        return True

    result = lint.shell(path=".", severity="warning", indent=4)

    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + " SC" + str(issue.code) + ": " + issue.message
        if issue.level == "error":
            error(msg)
        else:
            warn(msg)

    for file_info in result.format_issues:
        warn(file_info.file + " needs formatting - run 'shfmt -w -i 4'")

    return result.passed

def run_markdown_check():
    """Run markdown lint checks, return True if passed."""
    tool = lint.ensure_tools()
    for t in tool.tools:
        if t.name == "markdownlint-cli2" and not t.installed:
            # Skip silently if not installed
            return True

    result = lint.markdown(path=".", fix=False)

    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + " " + issue.rule + ": " + issue.message
        if issue.severity == "error":
            error(msg)
        else:
            warn(msg)

    for issue in result.frontmatter_issues:
        error(issue.file + ": " + issue.message)

    return result.lint_passed and result.frontmatter_passed

def run_pre_push(ctx):
    """Run pre-push checks."""
    # For now, pre-push runs the same checks as pre-commit
    # Can be extended to run tests, etc.
    run_pre_commit(ctx)

# Register commands
command(
    name = "hook.pre-commit",
    help = "Run pre-commit checks (called by git pre-commit hook)",
    flags = [],
    run = run_pre_commit,
)

command(
    name = "hook.pre-push",
    help = "Run pre-push checks (called by git pre-push hook)",
    flags = [],
    run = run_pre_push,
)
