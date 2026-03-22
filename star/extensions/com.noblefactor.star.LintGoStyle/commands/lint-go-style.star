# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# lint-go-style.star — Go style guidelines enforcement
#
# Orchestrator for the Go style linter. Discovers Go source files
# via the file provider (respects .gitignore), loads each into a
# SourceFile semantic tree, and runs check or fix. Config and job
# control only — all logic is in the provider.

def run(ctx):
    """Enforce Go style guidelines on Go source files."""
    fix_mode = ctx.args.get("fix", "false") == "true"
    scan_path = ctx.args.get("path", ".")
    include_tests = ctx.args.get("tests", "true") == "true"
    verbose = ctx.args.get("verbose", "false") == "true"

    if file.is_file(resource=scan_path):
        files = [scan_path]
    elif file.is_dir(resource=scan_path):
        all_files = file.find(scan_path + "/**/*.go")
        files = []
        for f in sorted(all_files):
            if not include_tests and f.endswith("_test.go"):
                continue
            files.append(f)
        if not files:
            ui.success("No Go files found")
            return
    else:
        ui.fail(scan_path + " is not a file or directory")
        return

    if verbose:
        ui.note("Found " + str(len(files)) + " Go file(s)")

    if fix_mode:
        for f in files:
            if verbose:
                ui.note("Fixing " + f)
            ast = goast.load_source_file(f)
            ast.cleanup()
            ast.save()
        ui.success("Fixed " + str(len(files)) + " file(s)")
    else:
        total_violations = 0
        for f in files:
            if verbose:
                ui.note("Checking " + f)
            ast = goast.load_source_file(f)
            for v in ast.check_compliance:
                ui.warn(f + " [" + v.kind + "] " + v.message)
                total_violations += 1
        if total_violations > 0:
            ui.fail("Found " + str(total_violations) + " violation(s) in " + str(len(files)) + " file(s)")
        else:
            ui.success("All " + str(len(files)) + " file(s) compliant")
