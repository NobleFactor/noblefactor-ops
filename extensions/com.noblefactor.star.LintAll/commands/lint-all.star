# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint-all.star - Run all configured linters
#
# Uses the commands API to discover and run all sibling lint commands.

def run(ctx):
    """Run all configured linters."""
    fix = ctx.args.get("fix", "false") == "true"

    # Get all sibling lint commands (lint.go, lint.shell, etc.)
    siblings = commands.siblings()

    if len(siblings) == 0:
        warn("No lint commands found")
        return

    # Track results
    failures = []
    passed = []

    # Run each sibling command
    for cmd in siblings:
        # Extract short name (e.g., "go" from "lint.go")
        short_name = cmd.name.split(".")[-1]
        note("=== " + short_name.upper() + " ===")

        # Check if command should be skipped based on config
        if short_name == "copyright":
            cfg = config.get()
            if not cfg.lint.copyright.enabled:
                note("Skipped (disabled in star.yaml)")
                continue

        # Run the command with the same fix flag
        result = cmd.run(fix=fix)

        if result.passed:
            passed.append(cmd.name)
        else:
            failures.append(cmd.name)

    # Summary
    note("")
    note("=== SUMMARY ===")

    if len(passed) > 0:
        for name in passed:
            success(name.split(".")[-1] + ": passed")

    if len(failures) > 0:
        for name in failures:
            error(name.split(".")[-1] + ": failed")
        fail("Linters failed: " + ", ".join([n.split(".")[-1] for n in failures]))
    else:
        success("All " + str(len(passed)) + " linters passed")

command(
    name = "lint.all",
    help = "Run all configured linters",
    run = run,
)
