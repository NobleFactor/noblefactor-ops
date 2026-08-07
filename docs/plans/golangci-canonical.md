---
title: "Golangci Canonical"
status: complete
created: 2026-08-07
updated: 2026-08-07
---

# Plan: Golangci Canonical

## Summary

Replaces this repository's `.golangci.yaml` with the canonical NobleFactor template,
hardened by devlore-cli's lint ladder (2,486 findings to zero). The previous copy was older
and looser: it suppressed G204, G301, G302, and G306 wholesale — the policy the ladder
rejected in favor of narrow per-site suppressions with stated reasons. The canonical adds
`whitespace.multi-func`, the ladder's shared exclusions, and the golangci-lint v2-correct
`output` block. Repos adopting it may surface new findings; that is the point.

## Sync contract

The same content is embedded in star's lint provider (`defaultGolangciConfig`), which seeds
new repos, and devlore-cli's `TestDefaultGolangciConfigTracksRepoConfig` locks that copy to
devlore-cli's own config minus its declared repo-specific exclusions. Changes flow:
devlore-cli ladder → this canonical → star's seed template. Companion plan:
devlore-cli `docs/plans/golangci-template-sync.md`.
