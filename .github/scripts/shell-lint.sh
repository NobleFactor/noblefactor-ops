#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2025-2026 Noble Factor. All rights reserved.

# shell-lint.sh - Lint shell scripts with shfmt and shellcheck
#
# Finds all files with shell shebangs or shellcheck directives,
# then runs shfmt and shellcheck on each file.
#
# A consumer's `source=Declare-BashScript` directive resolves through -P. Here the file is in-tree;
# a consumer repository points DECLARE_BASHSCRIPT_DIR at a checkout of this one, or at its own
# deployed ~/.local/bin. (A comment must not begin with the word shellcheck: it is read as a directive.)

failed=0
files=$(
    find . -path ./.git -prune -o -type f -print 2>/dev/null |
        while read -r file; do
            head -n1 "$file" 2>/dev/null |
                grep -qE "^#!/(usr/bin/env[[:space:]]+)?(sh|bash)\b|^# shellcheck shell=" &&
                echo "$file"
        done |
        sort
)

for f in $files; do
    shfmt_ok=true
    shellcheck_ok=true
    shfmt -d -i 4 -ci "$f" >/dev/null 2>&1 || shfmt_ok=false
    shellcheck -x --severity=warning -P "${DECLARE_BASHSCRIPT_DIR:-Home/common/.local/bin}" "$f" >/dev/null 2>&1 || shellcheck_ok=false
    if $shfmt_ok && $shellcheck_ok; then
        echo "✓ $f"
    else
        echo "✗ $f"
        $shfmt_ok || echo "  shfmt: FAIL"
        $shellcheck_ok || echo "  shellcheck: FAIL"
        failed=1
    fi
done

exit $failed
