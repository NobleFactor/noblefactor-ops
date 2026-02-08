---
title: "GitHub Branch Protection Rules"
description: "Branch protection policy for all NobleFactor repositories"
---

# GitHub Branch Protection Rules

This document defines the branch protection policy for all NobleFactor repositories.

## Protected Repositories

| Repository | Default Branch |
| --- | --- |
| devlore.noblefactor.com | develop |
| devlore-cli | develop |
| devlore-registry | develop |
| noblefactor | main |
| noblefactor-ops | develop |

## Protected Branches

The following branch patterns are protected in all repositories:

- `main`
- `develop`
- `release/*`

## Rules

| Rule | Setting |
| --- | --- |
| Direct commits | **Blocked for everyone** |
| Pull requests | **Required** |
| Merge method | **Squash only** |
| Required reviews | **1** (for contributors) |
| Dismiss stale reviews | **Yes** |
| Admin bypass | **PR merge only** (admins can merge without review, but cannot push directly) |

## Implementation

Branch protection is implemented using GitHub Repository Rulesets (the newer fine-grained permissions system).

### Ruleset Configuration

Each repository has a ruleset named "Protect main branches" with the following configuration:

```json
{
  "name": "Protect main branches",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": {
      "include": ["refs/heads/main", "refs/heads/develop", "refs/heads/release/*"],
      "exclude": []
    }
  },
  "rules": [
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 1,
        "dismiss_stale_reviews_on_push": true,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false,
        "allowed_merge_methods": ["squash"]
      }
    }
  ],
  "bypass_actors": [
    {
      "actor_id": 5,
      "actor_type": "RepositoryRole",
      "bypass_mode": "pull_request"
    }
  ]
}
```

### Bypass Actor Explanation

- `actor_id: 5` = Repository Admin role
- `actor_type: RepositoryRole` = Uses repository-level role permissions
- `bypass_mode: pull_request` = Admins can only bypass review requirements when merging PRs; direct pushes are still blocked

## Managing Rulesets

### View Current Rulesets

```bash
gh api repos/NobleFactor/<repo>/rulesets --jq '.[] | {name, id, enforcement}'
```

### Update a Ruleset

```bash
gh api repos/NobleFactor/<repo>/rulesets/<ruleset_id> --method PUT --input ruleset.json
```

### View Ruleset Details

```bash
gh api repos/NobleFactor/<repo>/rulesets/<ruleset_id>
```

## Rationale

1. **Squash-only merges**: Keep commit history clean and linear on protected branches
2. **Required reviews**: Ensure code quality through peer review for contributors
3. **Admin bypass for PRs**: Allow repository owners to merge urgent fixes without waiting for reviews, while still requiring the PR workflow for traceability
4. **No direct pushes**: All changes must go through pull requests for audit trail and CI validation

## Changelog

- 2026-01-27: Initial policy created and applied to all 5 NobleFactor repositories
