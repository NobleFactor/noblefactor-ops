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

def get_comment_style(filepath):
    """Return comment prefix for file type."""
    if filepath.endswith(".go") or filepath.endswith(".star"):
        return "//"
    elif filepath.endswith(".sh") or filepath.endswith(".bash"):
        return "#"
    return None

def build_expected_header(comment, license_id, holder):
    """Build the expected SPDX header lines."""
    return [
        comment + " SPDX-License-Identifier: " + license_id,
        comment + " Copyright " + holder,
    ]

def check_header(content, comment, license_id, holder):
    """Check if content has correct SPDX header. Returns (ok, message)."""
    lines = content.split("\n")
    expected = build_expected_header(comment, license_id, holder)

    # Handle shebang for shell scripts
    start = 0
    if len(lines) > 0 and lines[0].startswith("#!"):
        start = 1

    # Check we have enough lines
    if len(lines) < start + 2:
        return (False, "missing SPDX header")

    # Check SPDX line
    if not lines[start].startswith(comment + " SPDX-License-Identifier:"):
        return (False, "missing SPDX-License-Identifier")

    if lines[start] != expected[0]:
        return (False, "wrong license identifier (expected " + license_id + ")")

    # Check copyright line
    if not lines[start + 1].startswith(comment + " Copyright"):
        return (False, "missing copyright line")

    if holder not in lines[start + 1]:
        return (False, "wrong copyright holder (expected " + holder + ")")

    return (True, "")

def fix_header(content, comment, license_id, holder):
    """Fix the SPDX header in content. Returns fixed content."""
    lines = content.split("\n")
    expected = build_expected_header(comment, license_id, holder)

    # Handle shebang
    shebang = ""
    start = 0
    if len(lines) > 0 and lines[0].startswith("#!"):
        shebang = lines[0] + "\n"
        start = 1

    # Skip existing SPDX/copyright lines if present
    body_start = start
    for i in range(start, min(start + 5, len(lines))):
        line = lines[i]
        if line.startswith(comment + " SPDX-License-Identifier:"):
            body_start = i + 1
            continue
        if line.startswith(comment + " Copyright"):
            body_start = i + 1
            continue
        if line == "" or line == comment:
            body_start = i + 1
            continue
        break

    # Build fixed content
    body = "\n".join(lines[body_start:])
    header = expected[0] + "\n" + expected[1] + "\n"

    if shebang:
        return shebang + header + "\n" + body
    return header + "\n" + body

def collect_files(path, exclude):
    """Collect source files from path, excluding patterns."""
    files = []
    for ext in ["**/*.go", "**/*.star", "**/*.sh", "**/*.bash"]:
        for f in file.glob(path + "/" + ext):
            excluded = False
            for pattern in exclude:
                if pattern.endswith("/**"):
                    prefix = pattern[:-3]
                    if prefix in f:
                        excluded = True
                        break
            if not excluded:
                files.append(f)
    return files

def run_copyright(ctx):
    """Check or fix copyright headers in source files."""
    fix_mode = ctx.args.get("fix", "false") == "true"
    path = ctx.args.get("path", ".")

    # Load config
    cfg = config.get()
    copyright_cfg = cfg.lint.copyright

    if not copyright_cfg.enabled:
        warn("Copyright checking is disabled in star.yaml")
        warn("Add 'lint.copyright.enabled: true' to enable")
        return

    license_id = copyright_cfg.license
    if license_id == "auto":
        # Read LICENSE file to detect
        if file.exists("LICENSE"):
            content = file.read("LICENSE")
            if "MIT License" in content or "Permission is hereby granted" in content:
                license_id = "MIT"
            elif "Apache License" in content:
                license_id = "Apache-2.0"
            elif "GNU GENERAL PUBLIC LICENSE" in content:
                license_id = "GPL-3.0"
            else:
                fail("Could not detect license. Set lint.copyright.license in star.yaml")
        else:
            fail("No LICENSE file found. Set lint.copyright.license in star.yaml")

    holder = copyright_cfg.holder
    if not holder:
        fail("Copyright holder not configured. Set lint.copyright.holder in star.yaml")

    exclude = list(copyright_cfg.exclude)
    files = collect_files(path, exclude)

    if len(files) == 0:
        note("No source files found")
        return

    note("Checking " + str(len(files)) + " source files...")

    issues = []
    fixed = []

    for f in files:
        comment = get_comment_style(f)
        if not comment:
            continue

        content = file.read(f)
        ok, msg = check_header(content, comment, license_id, holder)

        if not ok:
            if fix_mode:
                new_content = fix_header(content, comment, license_id, holder)
                file.write(f, new_content)
                fixed.append(f)
            else:
                issues.append({"file": f, "message": msg})

    if fix_mode:
        if len(fixed) > 0:
            success("Fixed " + str(len(fixed)) + " files:")
            for f in fixed:
                note("  " + f)
        else:
            success("All files have correct copyright headers")
    else:
        if len(issues) == 0:
            success("All " + str(len(files)) + " files have correct copyright headers")
        else:
            for issue in issues:
                error(issue["file"] + ": " + issue["message"])
            fail("Found " + str(len(issues)) + " files with copyright issues (run with --fix to repair)")

command(
    name = "lint.copyright",
    help = "Check or fix copyright headers in source files",
    flags = [
        {"name": "fix", "help": "Add missing headers and update old format", "default": "false"},
        {"name": "path", "help": "Path to check (default: .)", "default": "."},
    ],
    run = run_copyright,
)
