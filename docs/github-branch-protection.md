---
title: "GitHub Branch Protection Rules"
description: "Branch protection and required-check policy for all NobleFactor repositories"
---

# GitHub Branch Protection Rules

This document defines the branch protection policy for all NobleFactor repositories, and records
what is actually enforced today.

## Protected Repositories

| Repository | Default Branch |
| --- | --- |
| devlore.noblefactor.com | develop |
| devlore-cli | develop |
| devlore-registry | develop |
| noblefactor | main |
| noblefactor-ops | develop |

## Protected Branches

- `~DEFAULT_BRANCH`
- `main`
- `develop`
- `release/*`

Branch *names* are constrained everywhere, on every ref — see ruleset 12515090 below.

## Implementation: three organization rulesets

Protection is implemented with **organization-level** GitHub rulesets, not per-repository ones.
One definition governs every repository in the org; a repository cannot weaken it, and a
repository-level ruleset can only add to it.

That distinction matters for required checks and is the reason for the two-tier policy below: an
org ruleset can only require a check that **every** repository produces.

### 12426847 — "Protection for default, main, develop, and release/\*"

Applies to `~DEFAULT_BRANCH`, `refs/heads/main`, `refs/heads/develop`, `refs/heads/release/*`.

| Rule | Setting |
| --- | --- |
| `deletion` | Protected branches cannot be deleted |
| `non_fast_forward` | No force-pushes |
| `required_linear_history` | No merge commits |
| `pull_request` | Squash only; **1** approving review; dismiss stale reviews on push; **require code-owner review**; require extra approval for unattributed changes |
| `code_quality` | severity `all` |
| `required_status_checks` | `quality-gate`; `strict_required_status_checks_policy` **false** |
| `code_scanning` | CodeQL — alerts threshold `errors`, security alerts `all` |

**Bypass actors:**

| Actor | Mode |
| --- | --- |
| `OrganizationAdmin` | `pull_request` |
| `Integration` id `2791147` (the automation app) | `pull_request` |

`pull_request` mode is an override at merge and nothing else. **Direct pushes to `main`, `develop`,
and `release/*` are refused for everyone**, organization admins and the automation app included.
Both can still merge a pull request whose required checks are failing or whose review is missing,
which is what keeps `gh pr merge --admin` working in the PR scripts and lets the automation app
merge its own cross-repository pull requests in devlore-cli's `knowledge-extract`, `docs-publish`,
and `release` workflows.

Until 2026-08-26 both actors held `always`, which exempted them from **every** rule in every
circumstance — including the `pull_request` rule requiring a pull request at all. A direct push to a
protected branch was therefore possible for an admin and for the app, contrary to the policy stated
below and to every previous revision of this document. Narrowed deliberately: you must at least look
at the CI results.

One workflow relied on the removed behavior. `devlore-registry`'s `update-indexes.yaml` triggers on
push to `develop`/`main`/`release/*`, then commits and runs a bare `git push` back into the branch
that triggered it. That push is now refused, and the workflow must be reworked to open a pull
request as devlore-cli's three already do.

`strict` is deliberately **false**. Enabling it would require every pull request to be current with
its base before merging, which on a five-leg matrix means re-running ten checks each time the base
advances.

### 12515090 — "Restrict branch names"

Every ref, every repository, **no bypass actors at all** — organization admins included.

```
^(chore|doc|feature|fix|hotfix|refactor|release)/.+$
```

A branch outside that set cannot be pushed by anyone. Worth knowing before naming one.

### 12427647 — "Signed commits for main"

`refs/heads/main` only, **no bypass actors**. Commits reaching `main` must be signed.

## Required checks

**Every required check must pass.** There is no partial credit and no leg that reports for
information only. The sole exception is a bypass actor — an organization admin or a repository
admin — who may override at merge, having looked at the results.

`quality-gate` is the **minimum**, and the only check an *organization* ruleset can require, because
it is the only one every repository produces. That is a statement about where a requirement can be
declared, not about how much is required: a repository carrying a tier-2 ruleset requires everything
in it, and a red leg blocks the merge for anyone who is not a bypass actor.

Cross-platform repositories require more. Building for a platform proves the code compiles there;
it does not prove it runs. Every supported platform must therefore report **both**:

| Tier | Scope | Required checks |
| --- | --- | --- |
| 1 — baseline | every repository, org ruleset 12426847 | `quality-gate` |
| 2 — cross-platform | repositories that ship platform-specific binaries, repository ruleset | `test (<goos>-<goarch>)` and `scenario (<goos>-<goarch>)` for **each** supported platform |

**`test (…)`** is the functional suite — unit tests, the whole package set.
**`scenario (…)`** is end-to-end — the real binaries driven in a sandbox as an operator would.

Neither substitutes for the other, and neither substitutes for a cross-compile. A green
cross-compile says the target builds. A green `test` says its units behave. Only `scenario` says
the shipped artifact works on the machine it was built for.

### Context naming

The check context is the **platform descriptor** — `<goos>-<goarch>` — matching `ALL_PLATFORMS` and
the `build/<goos>-<goarch>` directories. Never the runner label.

```
test (darwin-arm64)      scenario (darwin-arm64)
test (linux-amd64)       scenario (linux-amd64)
test (linux-arm64)       scenario (linux-arm64)
test (windows-amd64)     scenario (windows-amd64)
test (windows-arm64)     scenario (windows-arm64)
```

Two reasons the runner label is wrong here. It leaks an implementation detail a required-check name
then pins — whether linux/arm64 comes from `ubuntu-24.04-arm` or a successor is not the ruleset's
business. And GitHub composes a matrix job's default name from **every** matrix value, so a
`matrix.include` carrying any extra key yields `test (windows-11-arm, false)`. Set the name
explicitly:

```yaml
jobs:
  test:
    name: test (${{ matrix.platform }})
    strategy:
      matrix:
        include:
          - { platform: windows-arm64, os: windows-11-arm, race: false }
```

Without that, adding one matrix key silently renames every required context, and a required context
whose name never arrives blocks **every** merge rather than none.

### Only require checks that run on pull requests

A workflow triggered solely by `push` never reports on a pull request. Requiring one blocks every
merge permanently. In devlore-cli, `build-and-release`, `generate-and-pr`, and `sync-install-script`
are push-only and must not appear in any required list, even though they show green on `develop`.

Confirm every context against a real run before writing it into a ruleset:

```bash
sha=$(git rev-parse origin/develop)
gh api "repos/NobleFactor/<repo>/commits/$sha/check-runs" --jq '.check_runs[] | .name' | sort
```

### Where tier 2 lives

In a **repository** ruleset, not the organization one.

The first instance is **21539972 — "Cross-platform required checks"** on devlore-cli, created
2026-08-26. It requires all eleven contexts — `quality-gate` plus the ten platform legs, so a
repository's full requirement is legible in one place rather than split across two rulesets — over
the same refs as the organization ruleset, with `strict` false and both bypass actors at
`pull_request`.
 `test (darwin-arm64)` exists only in
devlore-cli; requiring it org-wide would leave devlore-registry and noblefactor-ops waiting forever
for a check that never arrives, blocking every merge in both.

Repository rulesets compose with organization rulesets: the org keeps enforcing squash-only, one
review, linear history, CodeQL and `quality-gate` everywhere, and the repository adds its own matrix
on top.

Note that the organization's `OrganizationAdmin` bypass does **not** extend to a repository ruleset.
A tier-2 ruleset therefore binds admins too, unless it declares its own bypass. That is a deliberate
choice either way, and should be made deliberately: without a bypass, `gh pr merge --admin` starts
being refused whenever a platform leg is red.

## Managing rulesets

These are organization rulesets. The repository endpoints below are **read-only views** of them;
edits must go through the organization endpoints, which need the `admin:org` scope.

```bash
# View — works with the repo scope
gh api repos/NobleFactor/<repo>/rulesets --jq '.[] | {id, name, enforcement, source_type}'
gh api repos/NobleFactor/<repo>/rulesets/<id>

# Edit — needs admin:org
gh auth refresh -h github.com -s admin:org
gh api orgs/NobleFactor/rulesets/<id> --method PUT --input ruleset.json

# Repository-level ruleset, for tier-2 checks — the repo scope suffices
gh api repos/NobleFactor/<repo>/rulesets --method POST --input repo-ruleset.json
```

## Rationale

1. **Squash-only merges** — clean, linear history on protected branches.
2. **Required reviews** — peer review for contributors; admins override review at merge, never the
   pull request itself.
3. **No direct pushes, by anyone** — every change carries an audit trail and CI validation, and an
   override is a decision made after looking at the results, not a way to avoid producing them.
4. **`quality-gate` everywhere** — one check every repository can honor.
5. **Per-platform `test` and `scenario` where platforms are shipped** — a cross-compile proves the
   target builds, not that it runs. Both suites, on the platform itself, or the claim is untested.
6. **Descriptor-named contexts** — required-check names outlive runner images and matrix keys.

## Changelog

- 2026-01-27: Initial policy created and applied to all 5 NobleFactor repositories
- 2026-08-26: Both bypass actors on ruleset 12426847 narrowed from `always` to `pull_request`, so
  direct pushes to protected branches are refused for everyone including organization admins and the
  automation app. Recorded the first tier-2 repository ruleset, 21539972 on devlore-cli.
- 2026-08-25: Rewritten against the live configuration. The previous revision described a
  per-repository ruleset named "Protect main branches" carrying a single `pull_request` rule and a
  `RepositoryRole` bypass limited to `pull_request` mode. None of that matches what is enforced:
  protection is three **organization** rulesets carrying seven rules, and the bypass actors are
  `OrganizationAdmin` and the automation app at mode **`always`** — every rule, not merely review.
  Also previously undocumented: `required_status_checks`, `code_scanning`, `code_quality`,
  `deletion`, `non_fast_forward`, `required_linear_history`, the `~DEFAULT_BRANCH` condition, the
  org-wide branch-name pattern, and the signed-commits requirement on `main`. Added the two-tier
  required-check policy and the context-naming rule.
