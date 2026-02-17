# Unified Go lint check registry

## Context

`star lint go` should be an orchestrator that runs a configurable set of Go
static analysis checks. The user doesn't care which tool implements a check —
all checks live in a flat namespace. `--skip errcheck` disables errcheck
inside golangci-lint, `--skip deadcode` skips the standalone deadcode binary,
`--skip mod_tidy` skips the tidy check. Same mechanism, same namespace.

Adding a new check means registering it in the registry, not modifying the
orchestrator.

The immediate motivation is adding `deadcode` (from
`golang.org/x/tools/cmd/deadcode`), but the design applies to the full set
of Go static analysis tools.

## Design

### Flat check namespace

Every individual analysis pass is a named check. Some are standalone tools,
some are linters inside golangci-lint, some are built-in Go commands. The
user sees one flat list:

| Check          | Implementation   | Default | Settings                       |
|----------------|------------------|---------|--------------------------------|
| `mod_tidy`     | `go mod tidy`    | enabled | —                              |
| `errcheck`     | golangci-lint    | enabled | `check-type-assertions`, etc.  |
| `govet`        | golangci-lint    | enabled | —                              |
| `staticcheck`  | golangci-lint    | enabled | —                              |
| `ineffassign`  | golangci-lint    | enabled | —                              |
| `unused`       | golangci-lint    | enabled | —                              |
| `gocyclo`      | golangci-lint    | enabled | `min-complexity`               |
| `gocognit`     | golangci-lint    | enabled | `min-complexity`               |
| `unparam`      | golangci-lint    | enabled | —                              |
| `unconvert`    | golangci-lint    | enabled | —                              |
| `gocritic`     | golangci-lint    | enabled | `enabled-tags`, etc.           |
| `gosec`        | golangci-lint    | enabled | `excludes`                     |
| `misspell`     | golangci-lint    | enabled | `locale`                       |
| `nilerr`       | golangci-lint    | enabled | —                              |
| `bodyclose`    | golangci-lint    | enabled | —                              |
| `durationcheck`| golangci-lint    | enabled | —                              |
| `errorlint`    | golangci-lint    | enabled | —                              |
| `noctx`        | golangci-lint    | enabled | —                              |
| `revive`       | golangci-lint    | enabled | `rules`                        |
| `whitespace`   | golangci-lint    | enabled | —                              |
| `gofmt`        | golangci-lint    | enabled | — (formatter)                  |
| `goimports`    | golangci-lint    | enabled | — (formatter)                  |
| `deadcode`     | standalone       | enabled | `test`                         |

### How --skip and --only work

When the user says `--skip errcheck --skip deadcode`:

1. The orchestrator partitions the skip list by implementation.
2. For golangci-lint checks: generate a modified golangci-lint config with
   those linters removed from the `enable` list.
3. For standalone checks (deadcode, mod_tidy): skip the tool invocation
   entirely.

When the user says `--only errcheck --only deadcode`:

1. Only the named checks run.
2. golangci-lint runs with only `errcheck` enabled.
3. deadcode runs standalone.
4. mod_tidy and all other golangci-lint linters are skipped.

### Command-line interface

```
star lint go [flags]
```

Flags:

- `--path` — package pattern (default `./...`)
- `--skip <check>` — skip a named check (repeatable)
- `--only <check>` — run only named checks (repeatable, mutually exclusive
  with --skip)
- `--checks` — list available checks and exit
- `--deadcode.test` — include test binaries in deadcode analysis

### Configuration

The config declares which checks are enabled and their settings. The
orchestrator merges config with CLI overrides (CLI wins).

```yaml
config:
  path: lint.go
  type: GoLintConfig
  fields:
    path: string
    checks: "map[string]interface{}"
  defaults:
    path: "./..."
    checks:
      mod_tidy:
        enabled: true
      errcheck:
        enabled: true
        check-type-assertions: true
        check-blank: true
      govet:
        enabled: true
      staticcheck:
        enabled: true
      ineffassign:
        enabled: true
      unused:
        enabled: true
      gocyclo:
        enabled: true
        min-complexity: 15
      gocognit:
        enabled: true
        min-complexity: 20
      unparam:
        enabled: true
      unconvert:
        enabled: true
      gocritic:
        enabled: true
      gosec:
        enabled: true
      misspell:
        enabled: true
      nilerr:
        enabled: true
      bodyclose:
        enabled: true
      durationcheck:
        enabled: true
      errorlint:
        enabled: true
      noctx:
        enabled: true
      revive:
        enabled: true
      whitespace:
        enabled: true
      gofmt:
        enabled: true
      goimports:
        enabled: true
      deadcode:
        enabled: true
        test: false
```

### Receiver layer (receiver_lint.go)

The receiver provides individual callable methods:

- `lint.golangci(path, checks, config)` — runs golangci-lint with a
  dynamically generated config based on which checks are enabled
- `lint.deadcode(path, test)` — runs standalone deadcode binary
- `lint.mod_tidy()` — runs `go mod tidy` + `git diff`

**Key change for golangci-lint:** the receiver accepts a list of enabled
linters rather than a static config file. It generates the golangci-lint
YAML config at runtime from the check registry, honoring per-linter settings
from the config.

**New `lintDeadcode` method:**

- Args: `path` (string, default `./...`), `test` (bool, default false)
- Absolute path handling: set `cmd.Dir` when path starts with `/`
- Parse JSON output (array of package objects with nested `Funcs`)
- Returns: `functions` (list of structs), `count` (int), `passed` (bool)

**Tool registration — add to `requiredTools`:**

```go
{
    Name:       "deadcode",
    Binary:     "deadcode",
    InstallCmd: "go install golang.org/x/tools/cmd/deadcode@latest",
},
```

### Orchestrator (lint-go.star)

```
def run(ctx):
    path = ctx.args.get("path", "./...")
    skip = parse_repeatable(ctx.args, "skip")
    only = parse_repeatable(ctx.args, "only")

    all_checks = load_check_registry()
    active = resolve_active(all_checks, skip, only)

    results = {}

    # Partition by implementation
    golangci_checks = [c for c in active if c.impl == "golangci-lint"]
    standalone_checks = [c for c in active if c.impl != "golangci-lint"]

    # Run golangci-lint once with only the active linters
    if golangci_checks:
        results["golangci-lint"] = lint.golangci(
            path=path,
            checks=[c.name for c in golangci_checks],
        )

    # Run each standalone check
    for check in standalone_checks:
        results[check.name] = run_standalone(check, ctx)

    report_summary(results)
```

### CI changes (devlore-cli)

**`.github/workflows/ci.yaml`**:

- Add `go install golang.org/x/tools/cmd/deadcode@latest` to "Install lint
  tools" step
- No separate CI step needed — `star lint go` runs all checks
- Can override per-repo: `star lint go --skip deadcode` if needed

## deadcode JSON output format

Output is an array of package objects with nested function arrays:

```json
[
  {
    "Name": "pkgname",
    "Path": "full/import/path",
    "Funcs": [
      {
        "Name": "FuncName",
        "Position": { "File": "path/to/file.go", "Line": 43, "Col": 6 },
        "Generated": false
      }
    ]
  }
]
```

## Open questions

1. Should `lint.shell` and `lint.markdown` follow the same registry pattern?
   If so, this becomes a general lint framework, not just Go-specific.
2. Should check ordering be configurable, or is declaration order sufficient?
3. Should checks run in parallel or sequentially?
4. golangci-lint supports hundreds of linters. Should the registry only
   include currently-enabled ones, or should it enumerate all available
   linters with `enabled: false` defaults?
