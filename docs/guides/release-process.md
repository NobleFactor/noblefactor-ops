---
title: Development and Release Process
description: Branching strategy, release workflow, and quality gates for Noble Factor projects
type: Process
audience: Engineers, DevOps
status: Approved
created: 2026-01-12
updated: 2026-02-11
---

# Development and Release Process

This document defines the branching strategy, release workflow, and quality gates for Noble Factor projects.

---

## Section Status

| Section | Status | Notes |
|---------|--------|-------|
| Branch Structure | Approved | Git Flow model adopted |
| Workflow Overview | Approved | Diagram and flow defined |
| Development Phase | Approved | Feature/bugfix workflow |
| Testing Phase | Approved | CI checks defined |
| Staging Phase | Approved | Release branch process |
| Release Phase | Approved | Merge and tag workflow |
| Hotfix Process | Approved | Emergency fix workflow |
| Rollback Procedures | Approved | Revert and reset options |
| Tagging and Versioning | Approved | Date-based (registry) + SemVer (tools) |
| Certificate Usage by Phase | Outstanding | Awaiting [#60](https://github.com/NobleFactor/noblefactor-ops/issues/60) (certificate procurement) |
| Common Workflows | Approved | Quick reference commands |
| Releasing CLI Utilities | Approved | goreleaser workflow documented |

**Status key:** Outstanding (needs work) · Approved (reviewed, accepted) · Rejected (not proceeding) · Completed (implemented)

---

## Branch Structure

```
main                    # Production releases; always deployable
  │
  ├── hotfix/YYYY-MM-DD # Emergency fixes branched from main
  │
develop                 # Integration branch; next release candidate
  │
  ├── feature/*         # Feature branches from develop
  └── bugfix/*          # Non-critical fixes from develop
```

## Workflow Overview

```text
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   Development    │     │     Release      │     │      Hotfix      │
├──────────────────┤     ├──────────────────┤     ├──────────────────┤
│                  │     │                  │     │                  │
│ feature/* ─────┐ │     │ develop ─────────┼──>  │ main ──────────┐ │
│                │ │     │        │         │     │       │        │ │
│ bugfix/*  ───┐ │ │     │        v         │     │       v        │ │
│              │ │ │     │ release/YYYY-MM-DD     │ hotfix/YYYY-MM-DD
│              v v │     │        │         │     │       │        │ │
│          develop │     │  quality gate    │     │ quality gate   │ │
│                  │     │        │         │     │       │        │ │
└──────────────────┘     │   ┌───┴───┐      │     │   ┌───┴───┐    │ │
                         │   │       │      │     │   │       │    │ │
                         │   v       v      │     │   v       v    │ │
                         │ main   develop   │     │ main   develop │ │
                         └──────────────────┘     └──────────────────┘

                         main ──────> tag: release-YYYY-MM-DD
```

## Development Phase

### Feature Development

1. **Create branch** from `develop`:
   ```bash
   git checkout develop
   git pull origin develop
   git checkout -b feature/short-description
   ```

2. **Develop** with regular commits (signed):
   ```bash
   git commit -S -m "Add feature X"
   ```

3. **Push and create PR** to `develop`:
   ```bash
   git push -u origin feature/short-description
   gh pr create --base develop
   ```

4. **Review and merge** after CI passes and approval received.

### Signing Commits

All commits must be signed. Development uses dev/test certificates.

```bash
# Configure signing (one-time)
git config --global commit.gpgsign true
git config --global user.signingkey <key-id>
```

See ODD-001 (Certificate Management) for certificate hierarchy details.

## Testing Phase

### Continuous Integration

Every push to `develop` triggers:

| Check | Tool | Requirement |
|-------|------|-------------|
| **Lint** | golangci-lint, shellcheck | Zero warnings |
| **Unit tests** | go test | 100% pass |
| **Integration tests** | Platform-specific | 100% pass |
| **Build** | go build | Success on all platforms |
| **Index generation** | CI script | Valid INDEX.yaml produced |
| **Signature** | Dev/test cert | Signed with dev certificate |

### Pre-release Validation

Before creating a release branch, ensure:

- [ ] All CI checks pass on `develop`
- [ ] No known critical bugs in milestone
- [ ] Documentation updated for new features
- [ ] CHANGELOG.md updated

## Staging Phase

### Create Release Branch

Release branches are keyed by target release date:

```bash
git checkout develop
git pull origin develop
git checkout -b release/2026-01-15
git push -u origin release/2026-01-15
```

### Release Branch Rules

- **Only bug fixes** — No new features
- **Must pass quality gate** — Same CI checks as develop
- **Production signing** — Commits signed with production certificate
- **Merge both ways** — To main (release) and develop (preserve fixes)

### Quality Gate

Release branches must pass before merge to main:

| Gate | Requirement |
|------|-------------|
| **CI green** | All checks pass |
| **Manual QA** | Release checklist completed |
| **Security review** | No new vulnerabilities |
| **Documentation** | README, CHANGELOG current |
| **Sign-off** | Release manager approval |

## Release Phase

### Merge to Main

```bash
# Fast-forward merge preferred
git checkout main
git pull origin main
git merge --ff-only release/2026-01-15

# If fast-forward not possible (rare), use merge commit
git merge --no-ff release/2026-01-15 -m "Release 2026-01-15"
```

### Tag the Release

```bash
git tag -s release-2026-01-15 -m "Release 2026-01-15"
git push origin main --tags
```

### Merge Back to Develop

```bash
git checkout develop
git pull origin develop
git merge main -m "Merge release-2026-01-15 to develop"
git push origin develop
```

### Delete Release Branch

```bash
git branch -d release/2026-01-15
git push origin --delete release/2026-01-15
```

## Hotfix Process

For critical issues in production that cannot wait for the next release.

### 1. Create Hotfix Branch

```bash
git checkout main
git pull origin main
git checkout -b hotfix/2026-01-16
```

### 2. Apply Fix

```bash
# Make minimal change to fix the issue
git commit -S -m "Fix critical issue X"
```

### 3. Test

- Run full CI pipeline on hotfix branch
- Manual verification of the fix
- Regression testing

### 4. Merge to Main

```bash
git checkout main
git merge --no-ff hotfix/2026-01-16 -m "Hotfix 2026-01-16: Fix issue X"
```

### 5. Tag the Hotfix

```bash
git tag -s release-2026-01-15.1 -m "Hotfix 2026-01-16 for release-2026-01-15"
git push origin main --tags
```

### 6. Merge to Develop

```bash
git checkout develop
git merge main -m "Merge hotfix-2026-01-16 to develop"
git push origin develop
```

### 7. Delete Hotfix Branch

```bash
git branch -d hotfix/2026-01-16
git push origin --delete hotfix/2026-01-16
```

## Rollback Procedures

When a release or hotfix causes production issues, use these procedures to return to last-known-good state.

### Rollback Decision Matrix

| Severity | Symptom | Action |
|----------|---------|--------|
| **Critical** | System down, data loss risk | Immediate rollback |
| **High** | Major functionality broken | Rollback within hours |
| **Medium** | Degraded functionality | Hotfix preferred; rollback if fix delayed |
| **Low** | Minor issues | Fix forward in next release |

### Identify Last-Known-Good Release

```bash
# List recent release tags
git tag -l 'release-*' --sort=-version:refname | head -10

# Show what changed between releases
git log --oneline release-2026-01-15..release-2026-01-20

# Verify the tag you want to roll back to
git show release-2026-01-15
```

### Option A: Revert Commit (Preferred)

Creates a new commit that undoes the bad changes. Preserves history.

```bash
# 1. Identify commits to revert
git log --oneline release-2026-01-15..main

# 2. Revert the merge commit(s) on main
git checkout main
git pull origin main

# For a single merge commit (most common)
git revert -m 1 <merge-commit-sha> -S

# For multiple commits, revert in reverse order
git revert -m 1 <newest-commit> -S
git revert -m 1 <older-commit> -S

# 3. Tag the rollback
git tag -s release-2026-01-20.rollback -m "Rollback to release-2026-01-15"

# 4. Push
git push origin main --tags

# 5. Regenerate index (CI will do this, or manually)
# INDEX.yaml now reflects rolled-back state
```

**Pros:** Clean history, easy to re-apply changes later.
**Cons:** Extra commits in history.

### Option B: Reset to Tag (Nuclear Option)

Resets main to a previous state. Use only when revert is impractical.

```bash
# DESTRUCTIVE - Requires force push
# Coordinate with all team members first

# 1. Ensure you have the right tag
git checkout main
git pull origin main
git log --oneline -5  # Confirm current state

# 2. Reset to last-known-good
git reset --hard release-2026-01-15

# 3. Tag the rollback point
git tag -s release-2026-01-20.rollback -m "Rollback: reset to release-2026-01-15"

# 4. Force push (requires branch protection override)
git push --force-with-lease origin main --tags

# 5. Notify team - their local main is now diverged
```

**Pros:** Clean state, as if bad release never happened.
**Cons:** Destroys history, requires force push, breaks collaborators' local repos.

**When to use:** Only when the bad release contains secrets, security vulnerabilities that must be scrubbed, or corrupted data that cannot be reverted cleanly.

### Rollback a Hotfix

If a hotfix made things worse:

```bash
# 1. Revert the hotfix merge on main
git checkout main
git pull origin main
git revert -m 1 <hotfix-merge-commit> -S

# 2. Tag appropriately
# If rolling back release-2026-01-15.1, tag as:
git tag -s release-2026-01-15.2 -m "Revert hotfix .1"

# 3. Push
git push origin main --tags
```

### Post-Rollback Checklist

After any rollback:

- [ ] **Verify production** — Confirm system is stable on rolled-back version
- [ ] **Regenerate index** — Ensure INDEX.yaml reflects current main
- [ ] **Notify stakeholders** — Communicate rollback and reason
- [ ] **Create incident ticket** — Document what went wrong
- [ ] **Preserve bad release** — Keep branch/commits for post-mortem (don't delete)
- [ ] **Update develop** — Merge rollback to develop to keep branches aligned

```bash
# Sync develop with rolled-back main
git checkout develop
git pull origin develop
git merge main -m "Sync rollback from main"
git push origin develop
```

### Re-applying a Rolled-Back Release

After fixing the issues, to re-apply a previously reverted release:

```bash
# 1. Revert the revert (yes, really)
git checkout main
git revert <revert-commit-sha> -S -m "Re-apply release-2026-01-20 with fixes"

# Or, if fixes are substantial, create new release branch
git checkout develop
git checkout -b release/2026-01-22
# ... apply fixes ...
# ... follow normal release process ...
```

### Rollback Diagram

```text
                    ┌───────────────────────────┐
                    │ Production Issue Detected │
                    └─────────────┬─────────────┘
                                  │
                                  v
                          ┌─────────────┐
                          │  Severity?  │
                          └──────┬──────┘
                        ╱        │        ╲
                       ╱         │         ╲
            Critical/High      Medium       Low
                    │            │           │
                    v            v           v
        ┌──────────────────┐  ┌─────────────┐  ┌─────────────┐
        │ Immediate        │  │ Fix in <4h? │  │ Fix Forward │
        │ Rollback         │  └──────┬──────┘  └─────────────┘
        └────────┬─────────┘    Yes  │  No           │
                 │               │   │   │           │
                 │               v   │   │           │
                 │       ┌───────────┴┐  │           │
                 │       │ Apply      │  │           │
                 │       │ Hotfix     ├──┘           │
                 │       └─────┬──────┘              │
                 │             │                     │
                 v             │                     │
        ┌──────────────────────┴────┐               │
        │ Identify Last-Known-Good  │               │
        └─────────────┬─────────────┘               │
                      │                             │
                      v                             │
              ┌─────────────┐                       │
              │ History     │                       │
              │ concerns?   │                       │
              └──────┬──────┘                       │
                 ╱       ╲                          │
               No      Secrets/                     │
               │       Security                     │
               v          v                         │
    ┌──────────────┐  ┌──────────────┐              │
    │ Option A:    │  │ Option B:    │              │
    │ Revert       │  │ Reset        │              │
    │ Commit       │  │ (Nuclear)    │              │
    └───────┬──────┘  └───────┬──────┘              │
             ╲               ╱                      │
              ╲             ╱                       │
               v           v                        v
            ┌───────────────────────────┐
            │    Verify Production      │<──────────┘
            └─────────────┬─────────────┘
                          │
                          v
            ┌───────────────────────────┐
            │       Post-Mortem         │
            └───────────────────────────┘
```

## Tagging and Versioning

### Registry Versioning (Date-Based)

The package registry uses date-based tags:

```
release-YYYY-MM-DD[.N]          # N = hotfix number (omit .0)

release-2026-01-15              # Initial release
release-2026-01-15.1            # First hotfix
release-2026-01-15.2            # Second hotfix
release-2026-01-20              # Next release
```

**Rationale:**
- **Self-documenting** — No lookup needed for release timing
- **Monotonically increasing** — Natural sort order
- **Low ceremony** — No debates about major vs. minor for package manifests
- **Hotfix-friendly** — `.1`, `.2` clearly indicate patches to a release

### Tool Versioning (SemVer + Metadata)

Lore, writ, and other tools use SemVer with build metadata:

```
<semver>[-<qualifier>-<date>]+<commit-hash>
```

| Build Type | Format | Example |
|------------|--------|---------|
| **Development** | `vX.Y.Z-dev-YYYYMMDD+hash` | `v1.2.3-dev-20260112+abc123f` |
| **Pre-release** | `vX.Y.Z-pre-YYYYMMDD+hash` | `v1.2.3-pre-20260115+def456a` |
| **Release** | `vX.Y.Z+hash` | `v1.2.3+789bcd0` |

**Components:**

| Part | Purpose | Affects Sort |
|------|---------|--------------|
| `X.Y.Z` | Semantic version (breaking.feature.patch) | Yes |
| `-dev` / `-pre` | Qualifier (dev < pre < release) | Yes |
| `-YYYYMMDD` | Build date (distinguishes builds within qualifier) | Yes |
| `+hash` | Git commit (traceability, verification) | No |

**Sort order (SemVer compliant):**

```
v1.2.3-dev-20260110+aaa   <   # Older dev build
v1.2.3-dev-20260112+bbb   <   # Newer dev build
v1.2.3-pre-20260101+ccc   <   # Pre-release (even older date, pre > dev)
v1.2.3-pre-20260115+ddd   <   # Newer pre-release
v1.2.3+eee                    # Release (no qualifier = highest)
```

**Why this format:**
- SemVer-compliant for future library publishing
- Date in sortable position distinguishes dev/pre-release builds
- Hash after `+` is metadata (exact commit for debugging)
- Clean release versions (`v1.2.3+hash`) — no dev noise

**Implementation:**

```bash
# Build script
VERSION="1.2.3"
QUALIFIER="dev"                           # or "pre", or empty for release
DATE=$(date +%Y%m%d)
HASH=$(git rev-parse --short HEAD)

if [ -n "$QUALIFIER" ]; then
    FULL_VERSION="v${VERSION}-${QUALIFIER}-${DATE}+${HASH}"
else
    FULL_VERSION="v${VERSION}+${HASH}"
fi

# Results:
# Dev:     v1.2.3-dev-20260112+abc123f
# Pre:     v1.2.3-pre-20260112+abc123f
# Release: v1.2.3+abc123f
```

**Git tags for releases:**

```bash
# Release tag matches version
git tag -s v1.2.3 -m "Release v1.2.3"
```

## Certificate Usage by Phase

| Phase | Branch | Signing Certificate | Index Signature |
|-------|--------|---------------------|-----------------|
| **Development** | feature/*, bugfix/* | Dev/test | None |
| **Integration** | develop | Dev/test | Dev/test |
| **Staging** | release/* | Production | Production |
| **Production** | main | Production | Production |
| **Hotfix** | hotfix/* | Production | Production |

See ODD-001 (Certificate Management) in `devlore/design/lore/05-lore-design-decisions.md` for certificate infrastructure design.

## Common Workflows

### Start a New Feature

```bash
git checkout develop && git pull
git checkout -b feature/my-feature
# ... develop ...
git push -u origin feature/my-feature
gh pr create --base develop --title "Add my feature"
```

### Prepare a Release

```bash
# 1. Create release branch
git checkout develop && git pull
git checkout -b release/2026-01-15

# 2. Final fixes and version bumps
# ... make changes ...
git commit -S -m "Prepare release 2026-01-15"

# 3. Push for CI
git push -u origin release/2026-01-15

# 4. After CI passes and approval, merge to main
gh pr create --base main --title "Release 2026-01-15"
```

### Emergency Hotfix

```bash
# 1. Branch from main
git checkout main && git pull
git checkout -b hotfix/2026-01-16

# 2. Fix the issue
# ... minimal change ...
git commit -S -m "Fix critical issue X"

# 3. Test
git push -u origin hotfix/2026-01-16
# Wait for CI

# 4. Merge to main, tag, merge to develop
gh pr create --base main --title "Hotfix: Fix critical issue X"
```

## Releasing CLI Utilities (goreleaser)

For Go CLI tools (lore, writ, and future utilities), use [goreleaser](https://goreleaser.com/) to automate cross-platform builds and distribution.

### One-Time Setup

```bash
# 1. Install goreleaser
go install github.com/goreleaser/goreleaser@latest

# 2. Create .goreleaser.yaml in repo root
# See devlore/design/02-devlore-rfc.md Section 4 for template

# 3. Create GitHub Action workflow
# .github/workflows/release.yaml — triggers on tag push
```

### Release Workflow

```bash
# 1. Ensure develop is ready
git checkout develop && git pull
# All CI checks pass, CHANGELOG updated

# 2. Create release branch (follow standard process)
git checkout -b release/2026-01-15
# ... final fixes, version bump ...
git push -u origin release/2026-01-15

# 3. Merge to main after approval
gh pr create --base main
# ... wait for approval and merge ...

# 4. Tag the release (triggers goreleaser)
git checkout main && git pull
git tag -s v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 5. goreleaser runs automatically via GitHub Action
# Produces:
#   - GitHub Release with binaries
#   - .tar.gz / .zip archives
#   - .deb and .rpm packages
#   - Homebrew formula (if configured)
#   - checksums.txt
```

### Post-Release Manual Steps

| Target | Action | Command |
|--------|--------|---------|
| **winget** | Update manifest | `wingetcreate update NobleFactor.DevLore -u <url> -v <version>` |
| **Homebrew tap** | Auto-updated by goreleaser | (none) |
| **apt PPA** | Upload .deb | Upload to PPA repository |
| **COPR** | Build RPM | `copr-cli build devlore <srpm>` |

### Testing Locally

Before tagging, test the release build:

```bash
# Build without publishing
goreleaser release --snapshot --clean

# Check artifacts
ls dist/
```

### Configuration Reference

See [Architecture Section 4: Distribution](devlore/design/02-devlore-rfc.md#4-distribution) for:
- Complete `.goreleaser.yaml` template
- GitHub Action workflow
- Per-package-manager requirements

---

## References

- [Git Flow](https://nvie.com/posts/a-successful-git-branching-model/) — Original branching model
- [GitHub Flow](https://docs.github.com/en/get-started/quickstart/github-flow) — Simplified continuous delivery
- [GitLab Flow](https://docs.gitlab.com/ee/topics/gitlab_flow.html) — Environment branches
- [Trunk Based Development](https://trunkbaseddevelopment.com/) — Alternative for high-velocity teams
- [CalVer](https://calver.org/) — Calendar versioning
- [SemVer](https://semver.org/) — Semantic versioning
- [goreleaser](https://goreleaser.com/) — Go release automation
