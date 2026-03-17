---
title: PR Script Template
description: Standard bash script structure for Claude Code to create, verify, merge, and clean up pull requests
type: Process
audience: Engineers, Claude Code
status: Approved
created: 2026-03-16
updated: 2026-03-16
---

# PR Script Template

Standard structure for the bash PR script that Claude Code generates to
`~/Workspace/NobleFactor/go`. Each script is self-contained: stage, commit, push,
create PR, verify CI, verify mergeability, squash merge, clean up.

---

## Template

```bash
#!/usr/bin/env bash
set -euo pipefail

cd ~/Workspace/NobleFactor/<repo>

# Stage specific files (never git add -A or git add .)
git add \
  path/to/file1 \
  path/to/file2

# Commit
git commit -m "$(cat <<'EOF'
<type>(<scope>): <summary>

<body>
EOF
)"

# Push branch
git push -u origin <branch>

# Create PR
gh pr create \
  --title "<type>(<scope>): <summary>" \
  --body "$(cat <<'EOF'
## Summary

- **Change 1** — description
- **Change 2** — description

## Test plan

- [x] Verified item
- [ ] TODO item
EOF
)"

# --- CI gate ---
sleep 5
pr_number=$(gh pr view --json number --jq '.number')

if ! gh pr checks "${pr_number}" --watch; then
    check_output=$(gh pr checks "${pr_number}" 2>&1 || true)
    if [[ "${check_output}" == *"no checks reported"* ]]; then
        echo "No CI checks configured, proceeding."
    else
        echo "CI checks failed:"
        echo "${check_output}"
        exit 1
    fi
fi

# --- Merge gate ---
merge_state=$(gh pr view "${pr_number}" --json mergeStateStatus --jq '.mergeStateStatus')
case "${merge_state}" in
    CLEAN|HAS_HOOKS|UNSTABLE)
        ;; # OK to merge
    DIRTY|BLOCKED|BEHIND)
        echo "PR is not mergeable (state: ${merge_state})."
        if [[ "${merge_state}" == "DIRTY" ]]; then
            echo "Merge conflicts detected — resolve before merging."
        fi
        exit 1
        ;;
    *)
        echo "Unknown merge state: ${merge_state} — check PR manually."
        exit 1
        ;;
esac

# Squash merge
gh pr merge "${pr_number}" --squash --admin

# Clean up (works from both worktrees and regular branches)
git close-branch
```

---

## Rules

1. **Stage by name.** Never `git add -A` or `git add .`.
2. **No Generated-by footer.** No `Co-Authored-By` lines in commit messages.
3. **HEREDOC for messages.** Both commit messages and PR bodies use `cat <<'EOF'` for safe formatting.
4. **CI gate is tolerant.** Repos with no required checks proceed; real failures abort.
5. **Merge gate before merge.** Check `mergeStateStatus` to catch conflicts, blocks, or staleness before attempting `gh pr merge`.
6. **`git close-branch` for cleanup.** Handles both worktree and regular-branch scenarios.
7. **One PR at a time.** Merge and clean up before starting the next branch.
