#!/usr/bin/env bash

# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

#
# Codegen Freshness Check
#
# Verifies that generated provider files match what the LKG (last-known-good)
# star binary produces. The LKG binary is built from a baseline branch
# (develop, release/*, or main).
#
# Usage:
#   scripts/check-codegen.sh --branch=develop
#   scripts/check-codegen.sh --branch=release/1.0
#   scripts/check-codegen.sh --branch=main
#
# Environment:
#   DEVLORE_CLI   Path to devlore-cli repo (default: ../devlore-cli)
#
# Exit codes:
#   0  All generated files match LKG codegen output
#   1  Generated files differ from LKG codegen output
#   2  Setup error (missing dependency, build failure, etc.)
#

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEVLORE_CLI="${DEVLORE_CLI:-$(cd "$REPO_ROOT/../devlore-cli" 2>/dev/null && pwd)}"
BRANCH=""

# =============================================================================
# Argument parsing
# =============================================================================

for arg in "$@"; do
    case "$arg" in
    --branch=*)
        BRANCH="${arg#--branch=}"
        ;;
    --help | -h)
        sed -n '7,27p' "$0" | sed 's/^# \?//'
        exit 0
        ;;
    *)
        echo "error: unknown argument: $arg" >&2
        echo "usage: scripts/check-codegen.sh --branch=develop" >&2
        exit 2
        ;;
    esac
done

if [[ -z "$BRANCH" ]]; then
    echo "error: --branch is required" >&2
    echo "usage: scripts/check-codegen.sh --branch=develop" >&2
    exit 2
fi

# =============================================================================
# Precondition checks
# =============================================================================

if [[ ! -d "$DEVLORE_CLI" ]]; then
    echo "error: devlore-cli not found at $DEVLORE_CLI" >&2
    echo "set DEVLORE_CLI to the correct path" >&2
    exit 2
fi

if ! git rev-parse --verify "$BRANCH" >/dev/null 2>&1; then
    # Try with origin/ prefix for remote-only branches.
    if git rev-parse --verify "origin/$BRANCH" >/dev/null 2>&1; then
        BRANCH="origin/$BRANCH"
    else
        echo "error: branch '$BRANCH' not found (local or remote)" >&2
        exit 2
    fi
fi

# =============================================================================
# Discover providers
# =============================================================================

mapfile -t PROVIDERS < <(find "$REPO_ROOT/internal/provider" -name gen -type d 2>/dev/null | while read -r gendir; do dirname "$gendir"; done)

if [[ ${#PROVIDERS[@]} -eq 0 ]]; then
    echo "No providers with gen/ directories found."
    exit 0
fi

echo "Providers to check:"
for p in "${PROVIDERS[@]}"; do
    echo "  ${p#"$REPO_ROOT/"}"
done
echo ""

# =============================================================================
# Build LKG binary
# =============================================================================

WORK_DIR="$(mktemp -d)"
LKG_BINARY="$WORK_DIR/star"
LKG_WORKTREE="$WORK_DIR/worktree"

cleanup() {
    if [[ -d "$LKG_WORKTREE" ]]; then
        git -C "$REPO_ROOT" worktree remove --force "$LKG_WORKTREE" 2>/dev/null || true
    fi
    rm -rf "$WORK_DIR"
}
trap cleanup EXIT

echo "Building LKG star binary from: $BRANCH"
git -C "$REPO_ROOT" worktree add --detach --quiet "$LKG_WORKTREE" "$BRANCH"

if ! (cd "$LKG_WORKTREE" && go build -o "$LKG_BINARY" ./cmd/star) 2>&1; then
    echo "error: failed to build LKG binary from $BRANCH" >&2
    exit 2
fi

echo "LKG binary built successfully."
echo ""

# =============================================================================
# Run codegen and compare
# =============================================================================

FAILED=0
CHECKED=0

for provider in "${PROVIDERS[@]}"; do
    rel_provider="${provider#"$REPO_ROOT/"}"
    echo "--- Checking $rel_provider ---"

    TEMP_OUT="$(mktemp -d)"

    # Run LKG codegen against current provider source, output to temp.
    if ! (cd "$DEVLORE_CLI" && "$LKG_BINARY" devlore actions generate \
        --source="$provider" \
        --gen=true \
        --write=true \
        --output="$TEMP_OUT") 2>&1; then
        echo "FAIL: codegen failed for $rel_provider" >&2
        FAILED=$((FAILED + 1))
        rm -rf "$TEMP_OUT"
        continue
    fi

    # Compare generated files.
    DIFF_OUTPUT=""
    HAS_DIFF=0

    for genfile in "$TEMP_OUT/gen/"*.go; do
        [[ -f "$genfile" ]] || continue
        basename="$(basename "$genfile")"
        committed="$provider/gen/$basename"

        if [[ ! -f "$committed" ]]; then
            echo "  MISSING: $rel_provider/gen/$basename (codegen produced it, but it's not committed)"
            HAS_DIFF=1
            continue
        fi

        if ! diff -u "$committed" "$genfile" >/dev/null 2>&1; then
            HAS_DIFF=1
            DIFF_OUTPUT+="$(diff -u \
                --label "committed: $rel_provider/gen/$basename" "$committed" \
                --label "codegen:   $rel_provider/gen/$basename" "$genfile" 2>&1 || true)"
            DIFF_OUTPUT+=$'\n'
        fi
    done

    # Check for committed files that codegen didn't produce.
    for committed in "$provider/gen/"*.go; do
        [[ -f "$committed" ]] || continue
        basename="$(basename "$committed")"
        if [[ ! -f "$TEMP_OUT/gen/$basename" ]]; then
            echo "  EXTRA: $rel_provider/gen/$basename (committed but codegen didn't produce it)"
            HAS_DIFF=1
        fi
    done

    if [[ $HAS_DIFF -eq 1 ]]; then
        echo "FAIL: $rel_provider/gen/ differs from LKG codegen output"
        if [[ -n "$DIFF_OUTPUT" ]]; then
            echo "$DIFF_OUTPUT"
        fi
        FAILED=$((FAILED + 1))
    else
        echo "PASS: $rel_provider"
    fi

    CHECKED=$((CHECKED + 1))
    rm -rf "$TEMP_OUT"
    echo ""
done

# =============================================================================
# Summary
# =============================================================================

echo "========================================="
echo "Codegen check: $CHECKED provider(s) checked"
if [[ $FAILED -gt 0 ]]; then
    echo "FAILED: $FAILED provider(s) have stale or incorrect gen/ files"
    echo ""
    echo "To fix: rebuild star (make build) and re-run codegen:"
    echo "  cd $DEVLORE_CLI && $REPO_ROOT/bin/star devlore actions generate \\"
    echo "    --source=\$REPO/<provider> --gen=true --write=true --output=\$REPO/<provider>"
    exit 1
else
    echo "PASSED: all gen/ files match LKG codegen output"
    exit 0
fi
