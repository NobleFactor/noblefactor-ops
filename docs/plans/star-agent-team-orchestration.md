---
title: "Star Agent Team Orchestration"
description: "Instructions for executing the star extension model refactoring plan"
status: draft
created: 2025-02-08
updated: 2025-02-08
---

# Star Agent Team Orchestration

This document provides instructions for executing the star extension model refactoring plan.

**Plan document:** [star-agent-team-refactor.md](./star-agent-team-refactor.md)

---

## Option A: Lead-Driven Orchestration (Recommended)

Start one Lead session that orchestrates everything:

```
Read docs/plans/star-agent-team-refactor.md as Lead.

Execute the full refactoring by spawning workers via the Task tool:
1. Spawn Worker 1 for Phase 1, wait for completion
2. Spawn Workers 3 and 5 in parallel for Phase 2, wait for completion
3. Continue through all phases per the merge order
4. Run integration tests after each phase

Manage branch merges between phases.
```

The Lead session will:
- Use the Task tool to spawn workers as background agents
- Wait for each phase to complete before starting the next
- Handle the merge order
- Run verification tests

**Trade-off:** Higher token burn (all context in one session), but simpler orchestration.

---

## Option B: Manual Session Orchestration

Start each worker session manually, following the phase order below.

### Phase 1: Core Configuration Infrastructure

**Worker 1 only** (no dependencies)

```
Read docs/plans/star-agent-team-refactor.md and execute Phase 1 as Worker 1.
```

Wait for completion. Merge PR to develop.

---

### Phase 2: Extension Registration + Wasm Runtime

**Workers 3 and 5** (can run in parallel)

Session 1:
```
Read docs/plans/star-agent-team-refactor.md and execute Phase 2 as Worker 3.
```

Session 2:
```
Read docs/plans/star-agent-team-refactor.md and execute Phase 2 as Worker 5.
```

Wait for both. Merge Worker 3 PR first, then Worker 5 PR.

---

### Phase 3: Migrate Commands to Extensions

**Workers 4 and 2**

Session 1:
```
Read docs/plans/star-agent-team-refactor.md and execute Phase 3 as Worker 4.
```

Session 2:
```
Read docs/plans/star-agent-team-refactor.md and execute Phase 3 as Worker 2.
```

Wait for both. Merge Worker 4 PR first, then Worker 2 PR.

---

### Phase 4: Update Runtime + Wasm Integration

**Workers 2, 5, and Lead**

Session 1:
```
Read docs/plans/star-agent-team-refactor.md and execute Phase 4 as Worker 5.
```

Session 2:
```
Read docs/plans/star-agent-team-refactor.md and execute Phase 4 as Worker 2.
```

Session 3 (after Workers 2 and 5 complete):
```
Read docs/plans/star-agent-team-refactor.md and execute Phase 4 as Lead.
Run integration tests.
```

Merge order: Worker 5 PR, Worker 2 PR, Lead PR.

---

### Phase 5: Remove Legacy Code

**Worker 1 only**

```
Read docs/plans/star-agent-team-refactor.md and execute Phase 5 as Worker 1.
```

Wait for completion. Lead verifies no regressions. Merge PR.

---

### Phase 6: Documentation and Testing

**Worker 4 and all agents for final testing**

```
Read docs/plans/star-agent-team-refactor.md and execute Phase 6 as Worker 4.
```

Final verification:
```
Read docs/plans/star-agent-team-refactor.md as Lead.
Run final integration tests and verify all acceptance criteria.
```

---

## Merge Order Summary

| Phase | Order | Branch | Agent |
|-------|-------|--------|-------|
| 1 | 1 | `feat/ext-config-phase-1` | Worker 1 |
| 2 | 1 | `feat/ext-extension-phase-2` | Worker 3 |
| 2 | 2 | `feat/ext-wasm-phase-2` | Worker 5 |
| 3 | 1 | `feat/ext-commands-phase-3` | Worker 4 |
| 3 | 2 | `feat/ext-starlark-phase-3` | Worker 2 |
| 4 | 1 | `feat/ext-wasm-phase-4` | Worker 5 |
| 4 | 2 | `feat/ext-starlark-phase-4` | Worker 2 |
| 4 | 3 | `feat/ext-lead-integration` | Lead |
| 5 | 1 | `feat/ext-config-phase-5` | Worker 1 |
| 6 | 1 | `feat/ext-commands-phase-6` | Worker 4 |

---

## Acceptance Criteria

All existing commands must work after refactoring:

- `star lint go`
- `star lint shell`
- `star lint markdown`
- `star lint copyright`
- `star lint copyright --fix`
- `star lint all`
- `star setup config`
- `star hook pre-commit`
