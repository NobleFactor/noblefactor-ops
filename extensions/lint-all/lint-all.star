# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint-all.star - Run all configured linters
#
# Orchestrates running Go, shell, markdown, and copyright linters.

def check_tool(name):
    """Check if a tool is installed and return its status."""
    result = lint.ensure_tools()
    for tool in result.tools:
        if tool.name == name:
            return tool
    return None

def run(ctx):
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
        for f in fs.glob(ext):
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
