# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint-copyright.star - Copyright header checking and fixing
#
# Ensures all source files have correct SPDX license headers.
# Configuration is loaded from star.yaml lint.copyright section.
#
# Usage:
#   star lint copyright           # Check headers (report issues)
#   star lint copyright --fix     # Add/update headers

def collect_source_files(path, exclude_patterns):
    """Collect source files (.go, .star, .sh) from path, excluding patterns."""
    files = []

    # Collect Go files
    go_files = file.glob(path + "/**/*.go")
    for f in go_files:
        files.append(f)

    # Collect Starlark files
    star_files = file.glob(path + "/**/*.star")
    for f in star_files:
        files.append(f)

    # Collect shell files
    sh_files = file.glob(path + "/**/*.sh")
    for f in sh_files:
        files.append(f)

    bash_files = file.glob(path + "/**/*.bash")
    for f in bash_files:
        files.append(f)

    # Filter out excluded patterns
    filtered = []
    for f in files:
        excluded = False
        for pattern in exclude_patterns:
            # Simple glob matching
            if pattern.endswith("/**"):
                prefix = pattern[:-3]
                if prefix in f:
                    excluded = True
                    break
            elif "*" in pattern:
                # Handle simple wildcards
                if pattern.replace("*", "") in f:
                    excluded = True
                    break
        if not excluded:
            filtered.append(f)

    return filtered

def run_copyright(ctx):
    """Check or fix copyright headers in source files."""
    fix = ctx.args.get("fix", "false") == "true"
    path = ctx.args.get("path", ".")

    # Load config
    cfg = config.get()
    copyright_cfg = cfg.lint.copyright

    if not copyright_cfg.enabled:
        warn("Copyright checking is disabled in star.yaml")
        warn("Add 'lint.copyright.enabled: true' to enable")
        return

    # Detect license if set to "auto"
    license = copyright_cfg.license
    if license == "auto":
        result = copyright.detect_license("LICENSE")
        if result.detected:
            license = result.license
            note("Detected license: " + license)
        else:
            fail("Could not detect license from LICENSE file. Set lint.copyright.license in star.yaml")

    holder = copyright_cfg.holder
    if not holder:
        fail("Copyright holder not configured. Set lint.copyright.holder in star.yaml")

    # Get patterns (patterns is a dict of structs with match/replace)
    patterns = {}
    for lang in ["go", "star", "shell"]:
        val = copyright_cfg.patterns.get(lang)
        if val:
            patterns[lang] = val

    # Get exclude patterns
    exclude = list(copyright_cfg.exclude)

    # Collect files
    files = collect_source_files(path, exclude)

    if len(files) == 0:
        note("No source files found")
        return

    note("Checking " + str(len(files)) + " source files...")

    if fix:
        # Fix mode
        result = copyright.fix(
            paths=files,
            license=license,
            holder=holder,
            patterns=patterns,
            dry_run=False,
        )

        if result.count > 0:
            success("Fixed " + str(result.count) + " files:")
            for f in result.fixed:
                note("  " + f)

        # Report errors (files that couldn't be fixed automatically)
        error_count = len(list(result.errors))
        if error_count > 0:
            for e in result.errors:
                error(e.file + ": " + e.message)
            fail("Could not fix " + str(error_count) + " files (must be fixed manually)")
        elif result.count == 0:
            success("All files have correct copyright headers")
    else:
        # Check mode
        result = copyright.check(
            paths=files,
            license=license,
            holder=holder,
            patterns=patterns,
        )

        if result.passed:
            success("All " + str(len(files)) + " files have correct copyright headers")
        else:
            for issue in result.issues:
                error(issue.file + ": " + issue.message)
            fail("Found " + str(result.count) + " files with copyright issues (run with --fix to repair)")

command(
    name = "lint.copyright",
    help = "Check or fix copyright headers in source files",
    flags = [
        {"name": "fix", "help": "Add missing headers and update old format", "default": "false"},
        {"name": "path", "help": "Path to check (default: .)", "default": "."},
    ],
    run = run_copyright,
)
