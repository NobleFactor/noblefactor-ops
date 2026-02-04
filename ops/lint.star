# lint.star - Static code analysis commands
#
# Provides unified linting commands for Go, shell, and markdown.
# Configuration is loaded from star.yaml (see star config show).
# On first run, creates default config files and checks for required tools.
#
# Usage:
#   star lint go [--path=./...]       # Run golangci-lint
#   star lint shell [--path=.]        # Run shellcheck + shfmt
#   star lint markdown [--path=.]     # Run markdownlint + frontmatter check
#   star lint sync                    # Sync tool configs from star.yaml
#   star lint tools                   # Check/show required tool status

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

    # Check tool is installed
    ensure_tool_installed("golangci-lint")

    note("Running golangci-lint on " + path)

    result = lint.go(path=path, config=config)

    # Note if config was created
    if result.config_created:
        success("Created .golangci.yaml with NobleFactor defaults")

    # Report issues
    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + ":" + str(issue.column)
        msg = msg + " " + issue.linter + ": " + issue.message
        if issue.severity == "error":
            error(msg)
        else:
            warn(msg)

    # Summary
    if result.passed:
        success("No lint issues found")
    else:
        fail("Found " + str(result.total_count) + " lint issues (" + str(result.error_count) + " errors, " + str(result.warning_count) + " warnings)")

def run_shell(ctx):
    """Run shellcheck and shfmt on shell scripts."""
    path = ctx.args.get("path", ".")
    severity = ctx.args.get("severity", "warning")
    indent = int(ctx.args.get("indent", "4"))

    # Check tools are installed
    ensure_tool_installed("shellcheck")
    ensure_tool_installed("shfmt")

    note("Running shell lint on " + path)

    # Run shellcheck
    note("Running shellcheck...")
    lint_result = shell.lint(path=path, severity=severity)

    # Report shellcheck issues
    for issue in lint_result.issues:
        msg = issue.file + ":" + str(issue.line) + ":" + str(issue.column)
        msg = msg + " SC" + str(issue.code) + ": " + issue.message
        if issue.level == "error":
            error(msg)
        elif issue.level == "warning":
            warn(msg)
        else:
            note(msg)

    # Run shfmt
    note("Running shfmt format check...")
    fmt_result = shell.format_check(path=path, indent=indent)

    # Report formatting issues
    for file_info in fmt_result.files_failed:
        warn(file_info.file + " needs formatting")
        if file_info.diff:
            # Show first few lines of diff
            lines = file_info.diff.split("\n")
            for line in lines[:10]:
                note("  " + line)
            if len(lines) > 10:
                note("  ... (" + str(len(lines) - 10) + " more lines)")

    # Summary
    shell_passed = lint_result.passed
    fmt_passed = fmt_result.passed

    if shell_passed and fmt_passed:
        success("Shell lint passed (" + str(lint_result.total_count) + " issues, " + str(fmt_result.files_checked) + " files formatted)")
    else:
        msg = "Shell lint failed:"
        if not shell_passed:
            msg = msg + " " + str(lint_result.error_count) + " errors, " + str(lint_result.warning_count) + " warnings"
        if not fmt_passed:
            msg = msg + " " + str(len(fmt_result.files_failed)) + " files need formatting"
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

def run_sync(ctx):
    """Sync tool-specific config files from star.yaml."""
    result = config.sync()

    if result.files_generated == 0:
        note("No tool configs to sync (no config sections in star.yaml)")
        return

    if result.golangci_lint:
        success("Generated " + result.golangci_lint)
    if result.markdown_lint:
        success("Generated " + result.markdown_lint)

    success("Synced " + str(result.files_generated) + " config file(s)")

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
    help = "Run golangci-lint on Go code",
    flags = [
        {"name": "path", "help": "Path to lint (default: ./...)", "default": "./..."},
        {"name": "config", "help": "Path to golangci-lint config file", "default": ""},
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
    name = "lint.sync",
    help = "Sync tool configs (.golangci.yaml, etc.) from star.yaml",
    flags = [],
    run = run_sync,
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
