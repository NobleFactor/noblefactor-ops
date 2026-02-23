# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint-shell.star - Shell script lint checks
#
# Run shellcheck and shfmt on shell scripts.

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
        ui.fail(name + " is not installed\n  Install: " + tool.install_cmd)
    return tool

def run(ctx):
    """Run shellcheck and shfmt on shell scripts."""
    path = ctx.args.get("path", ".")
    severity = ctx.args.get("severity", "warning")
    indent = int(ctx.args.get("indent", "4"))

    # Check tools are installed
    ensure_tool_installed("shellcheck")
    ensure_tool_installed("shfmt")

    ui.note("Running shell lint on " + path)

    # Run combined shell lint (shellcheck + shfmt)
    result = lint.shell(path=path, severity=severity, indent=indent)

    # Report shellcheck issues
    for issue in result.issues:
        msg = issue.file + ":" + str(issue.line) + ":" + str(issue.column)
        msg = msg + " SC" + str(issue.code) + ": " + issue.message
        if issue.level == "error":
            ui.error(msg)
        elif issue.level == "warning":
            ui.warn(msg)
        else:
            ui.note(msg)

    # Report formatting issues
    for file_info in result.format_issues:
        ui.warn(file_info.file + " needs formatting")
        if file_info.diff:
            lines = file_info.diff.split("\n")
            for line in lines[:10]:
                ui.note("  " + line)
            if len(lines) > 10:
                ui.note("  ... (" + str(len(lines) - 10) + " more lines)")

    # Summary
    if result.passed:
        ui.success("Shell lint passed (" + str(result.files_checked) + " files)")
    else:
        msg = "Shell lint failed:"
        if not result.lint_passed:
            msg = msg + " " + str(result.error_count) + " errors, " + str(result.warning_count) + " warnings"
        if not result.format_passed:
            msg = msg + " " + str(len(result.format_issues)) + " files need formatting"
        ui.fail(msg)
