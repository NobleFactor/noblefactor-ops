# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint-markdown.star - Markdown lint checks
#
# Run markdownlint and frontmatter validation on markdown files.

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

def run(ctx):
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
