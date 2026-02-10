# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint.star - Static code analysis commands
#
# Provides unified linting commands for Go, shell, markdown, and copyright.
# Configuration is loaded from star.yaml (see star config show).
# On first run, creates default config files and checks for required tools.
#
# Usage:
#   star lint go [--path=./...]       # Run golangci-lint
#   star lint shell [--path=.]        # Run shellcheck + shfmt
#   star lint markdown [--path=.]     # Run markdownlint + frontmatter check
#   star lint copyright [--fix]       # Check/fix SPDX copyright headers
#   star lint tools                   # Check/show required tool status
#   star lint all [--fix]             # Run all configured linters

def check_tool(name):
    """Check if a tool is installed and return its status."""
    result = lint.ensure_tools()
    for tool in result.tools:
        if tool.name == name:
            return tool
    return None

def ensure_tool_installed(name):
    """Ensure a tool is installed, fail with install instructions if not."""
    tool = check_tool(name)
    if tool and not tool.installed:
        fail(name + " is not installed\n  Install: " + tool.install_cmd)
    return tool

def run_go(ctx):
    """Run golangci-lint on Go code."""
    path = ctx.args.get("path", "./...")
    config = ctx.args.get("config", "")
    skip_mod_tidy = ctx.args.get("skip_mod_tidy", "false") == "true"

    # Check tool is installed
    ensure_tool_installed("golangci-lint")

    note("Running Go lint checks on " + path)

    # Run go mod tidy check first (unless skipped)
    if not skip_mod_tidy:
        note("Checking go.mod tidy...")

    result = lint.go(path=path, config=config, skip_mod_tidy=skip_mod_tidy)

    # Report mod tidy status
    if not skip_mod_tidy:
        if result.mod_tidy_passed:
            success("go.mod is tidy")
        else:
            error("go.mod is not tidy")
            if result.mod_tidy_details:
                for line in result.mod_tidy_details.split("\n"):
                    if line:
                        note("  " + line)

    # Note if config was created
    if result.config_created:
        success("Created .golangci.yaml with NobleFactor defaults")

    # Report golangci-lint issues
    note("Running golangci-lint...")
    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + ":" + str(issue.column)
        msg = msg + " " + issue.linter + ": " + issue.message
        if issue.severity == "error":
            error(msg)
        else:
            warn(msg)

    # Summary
    if result.lint_passed:
        success("No golangci-lint issues found")
    else:
        warn("Found " + str(result.total_count) + " lint issues (" + str(result.error_count) + " errors, " + str(result.warning_count) + " warnings)")

    # Final pass/fail
    if result.passed:
        success("All Go lint checks passed")
    else:
        msgs = []
        if not result.mod_tidy_passed:
            msgs.append("go.mod not tidy")
        if not result.lint_passed:
            msgs.append(str(result.total_count) + " lint issues")
        fail("Go lint failed: " + ", ".join(msgs))

def run_shell(ctx):
    """Run shellcheck and shfmt on shell scripts."""
    path = ctx.args.get("path", ".")
    severity = ctx.args.get("severity", "warning")
    indent = int(ctx.args.get("indent", "4"))

    # Check tools are installed
    ensure_tool_installed("shellcheck")
    ensure_tool_installed("shfmt")

    note("Running shell lint on " + path)

    # Run combined shell lint (shellcheck + shfmt)
    result = lint.shell(path=path, severity=severity, indent=indent)

    # Report shellcheck issues
    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + ":" + str(issue.column)
        msg = msg + " SC" + str(issue.code) + ": " + issue.message
        if issue.level == "error":
            error(msg)
        elif issue.level == "warning":
            warn(msg)
        else:
            note(msg)

    # Report formatting issues
    for file_info in result.format_issues:
        warn(file_info.file + " needs formatting")
        if file_info.diff:
            lines = file_info.diff.split("\n")
            for line in lines[:10]:
                note("  " + line)
            if len(lines) > 10:
                note("  ... (" + str(len(lines) - 10) + " more lines)")

    # Summary
    if result.passed:
        success("Shell lint passed (" + str(result.files_checked) + " files)")
    else:
        msg = "Shell lint failed:"
        if not result.lint_passed:
            msg = msg + " " + str(result.error_count) + " errors, " + str(result.warning_count) + " warnings"
        if not result.format_passed:
            msg = msg + " " + str(len(result.format_issues)) + " files need formatting"
        fail(msg)

def run_tools(ctx):
    """Check status of all required lint tools."""
    result = lint.ensure_tools()

    note("Checking lint tools...")
    for tool in result.tools:
        if tool.installed:
            success(tool.name + ": " + tool.path)
        else:
            error(tool.name + ": not installed")
            note("  Install: " + tool.install_cmd)

    if result.all_installed:
        success("All lint tools installed")
    else:
        print("")
        note("Install missing tools with:")
        for cmd in result.install_cmds:
            print("  " + cmd)
        fail("Missing required lint tools")

def run_markdown(ctx):
    """Run markdownlint and frontmatter check on markdown files."""
    path = ctx.args.get("path", ".")
    fix = ctx.args.get("fix", "false") == "true"

    # Check tool is installed
    ensure_tool_installed("markdownlint-cli2")

    note("Running markdown lint on " + path)

    # Sync config if needed
    config.sync()

    # Run markdownlint
    result = lint.markdown(path=path, fix=fix)

    # Report markdownlint issues
    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + " " + issue.rule + ": " + issue.message
        if issue.severity == "error":
            error(msg)
        else:
            warn(msg)

    # Report frontmatter issues
    for issue in result.frontmatter_issues:
        msg = issue.file + ": " + issue.message
        error(msg)

    # Summary
    lint_passed = result.lint_passed
    frontmatter_passed = result.frontmatter_passed

    if lint_passed and frontmatter_passed:
        success("Markdown lint passed (" + str(result.files_checked) + " files)")
    else:
        msg = "Markdown lint failed:"
        if not lint_passed:
            msg = msg + " " + str(result.issue_count) + " lint issues"
        if not frontmatter_passed:
            msg = msg + " " + str(len(result.frontmatter_issues)) + " frontmatter issues"
        fail(msg)

# Register commands
command(
    name = "lint.go",
    help = "Run Go lint checks (go mod tidy + golangci-lint)",
    flags = [
        {"name": "path", "help": "Path to lint (default: ./...)", "default": "./..."},
        {"name": "config", "help": "Path to golangci-lint config file", "default": ""},
        {"name": "skip_mod_tidy", "help": "Skip go mod tidy check", "default": "false"},
    ],
    run = run_go,
)

command(
    name = "lint.shell",
    help = "Run shellcheck and shfmt on shell scripts",
    flags = [
        {"name": "path", "help": "Path to lint (default: .)", "default": "."},
        {"name": "severity", "help": "Minimum shellcheck severity (error, warning, info, style)", "default": "warning"},
        {"name": "indent", "help": "Expected indent size for shfmt", "default": "4"},
    ],
    run = run_shell,
)

command(
    name = "lint.tools",
    help = "Check status of required lint tools",
    flags = [],
    run = run_tools,
)

command(
    name = "lint.markdown",
    help = "Run markdownlint and frontmatter check on markdown files",
    flags = [
        {"name": "path", "help": "Path to lint (default: .)", "default": "."},
        {"name": "fix", "help": "Auto-fix issues where possible", "default": "false"},
    ],
    run = run_markdown,
)

def run_all(ctx):
    """Run all configured linters."""
    fix = ctx.args.get("fix", "false") == "true"

    # Collect failures - run all linters even if some fail
    failures = []

    # Run Go lint
    note("=== Go ===")
    go_result = run_go_silent(fix)
    if not go_result:
        failures.append("go")

    # Run Shell lint
    note("=== Shell ===")
    shell_result = run_shell_silent()
    if not shell_result:
        failures.append("shell")

    # Run Markdown lint
    note("=== Markdown ===")
    md_result = run_markdown_silent(fix)
    if not md_result:
        failures.append("markdown")

    # Run Copyright lint (if enabled)
    cfg = config.get()
    if cfg.lint.copyright.enabled:
        note("=== Copyright ===")
        copyright_result = run_copyright_silent(fix)
        if not copyright_result:
            failures.append("copyright")

    # Summary
    if len(failures) == 0:
        success("All linters passed")
    else:
        fail("Linters failed: " + ", ".join(failures))

def run_go_silent(fix):
    """Run Go lint, return True if passed."""
    tool = check_tool("golangci-lint")
    if tool and not tool.installed:
        error("golangci-lint is not installed")
        note("  Install: " + tool.install_cmd)
        return False

    result = lint.go(path="./...", config="", skip_mod_tidy=False)

    # Report mod tidy status
    if result.mod_tidy_passed:
        success("go.mod is tidy")
    else:
        error("go.mod is not tidy")
        if result.mod_tidy_details:
            for line in result.mod_tidy_details.split("\n"):
                if line:
                    note("  " + line)

    # Report golangci-lint issues
    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + ":" + str(issue.column)
        msg = msg + " " + issue.linter + ": " + issue.message
        if issue.severity == "error":
            error(msg)
        else:
            warn(msg)

    if result.passed:
        success("Go lint passed")
        return True
    else:
        error("Go lint failed")
        return False

def run_shell_silent():
    """Run Shell lint, return True if passed."""
    sc_tool = check_tool("shellcheck")
    shfmt_tool = check_tool("shfmt")

    if sc_tool and not sc_tool.installed:
        error("shellcheck is not installed")
        note("  Install: " + sc_tool.install_cmd)
        return False
    if shfmt_tool and not shfmt_tool.installed:
        error("shfmt is not installed")
        note("  Install: " + shfmt_tool.install_cmd)
        return False

    # Run combined shell lint
    result = lint.shell(path=".", severity="warning", indent=4)

    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + ":" + str(issue.column)
        msg = msg + " SC" + str(issue.code) + ": " + issue.message
        if issue.level == "error":
            error(msg)
        elif issue.level == "warning":
            warn(msg)
        else:
            note(msg)

    for file_info in result.format_issues:
        warn(file_info.file + " needs formatting")

    if result.passed:
        success("Shell lint passed")
        return True
    else:
        error("Shell lint failed")
        return False

def run_markdown_silent(fix):
    """Run Markdown lint, return True if passed."""
    tool = check_tool("markdownlint-cli2")
    if tool and not tool.installed:
        error("markdownlint-cli2 is not installed")
        note("  Install: " + tool.install_cmd)
        return False

    # Sync config if needed
    config.sync()

    result = lint.markdown(path=".", fix=fix)

    # Report issues
    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + " " + issue.rule + ": " + issue.message
        if issue.severity == "error":
            error(msg)
        else:
            warn(msg)

    for issue in result.frontmatter_issues:
        error(issue.file + ": " + issue.message)

    if result.lint_passed and result.frontmatter_passed:
        success("Markdown lint passed")
        return True
    else:
        error("Markdown lint failed")
        return False

def run_copyright_silent(fix):
    """Run Copyright lint, return True if passed."""
    cfg = config.get()
    copyright_cfg = cfg.lint.copyright

    # Detect license if set to "auto"
    license = copyright_cfg.license
    if license == "auto":
        result = copyright.detect_license("LICENSE")
        if result.detected:
            license = result.license
        else:
            error("Could not detect license from LICENSE file")
            return False

    holder = copyright_cfg.holder
    if not holder:
        error("Copyright holder not configured in star.yaml")
        return False

    # Get patterns (patterns is a dict of structs with match/replace)
    patterns = {}
    for lang in ["go", "star", "shell"]:
        val = copyright_cfg.patterns.get(lang)
        if val:
            patterns[lang] = val

    # Get exclude patterns
    exclude = list(copyright_cfg.exclude)

    # Collect files
    files = []
    for ext in ["**/*.go", "**/*.star", "**/*.sh"]:
        for f in file.glob(ext):
            excluded = False
            for pattern in exclude:
                if pattern.endswith("/**"):
                    prefix = pattern[:-3]
                    if prefix in f:
                        excluded = True
                        break
            if not excluded:
                files.append(f)

    if len(files) == 0:
        success("No source files found")
        return True

    if fix:
        result = copyright.fix(
            paths=files,
            license=license,
            holder=holder,
            patterns=patterns,
            dry_run=False,
        )
        if result.count > 0:
            success("Fixed " + str(result.count) + " copyright headers")
        else:
            success("Copyright headers correct")
        return True
    else:
        result = copyright.check(
            paths=files,
            license=license,
            holder=holder,
            patterns=patterns,
        )
        if result.passed:
            success("Copyright headers correct")
            return True
        else:
            for issue in result.issues:
                error(issue.file + ": " + issue.message)
            error("Copyright lint failed (" + str(result.count) + " issues)")
            return False

command(
    name = "lint.all",
    help = "Run all configured linters",
    flags = [
        {"name": "fix", "help": "Auto-fix issues where possible", "default": "false"},
    ],
    run = run_all,
)
