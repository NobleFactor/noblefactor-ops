---
title: "Starlark Extension Model for nf-ops"
description: "Design plan for transforming nf-ops into a git-style extensible operations tool with Starlark scripting"
---

# Plan: Starlark Extension Model for nf-ops

## Vision

Transform `nf-ops` into a git-style extensible operations tool where all
commands are implemented as Starlark scripts discovered at runtime. A single
Go driver program provides the runtime, discovery, and built-in modules.

```text
nf-ops update-site-deploy-token
       └─ finds nf-ops-update-site-deploy-token.star on the extension path
       └─ loads it into the Starlark runtime
       └─ calls main() with context
```

Like git's extension model: if `git-foo` is on PATH, `git foo` runs it.
Here, if `nf-ops-foo.star` is on the extension path, `nf-ops foo` runs it.

## Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                        nf-ops (Go binary)                       │
├─────────────┬──────────────┬────────────────┬───────────────────┤
│  Discovery  │   Starlark   │   Built-in     │    Cobra CLI      │
│   Engine    │   Runtime    │   Modules      │   (driver only)   │
└──────┬──────┴──────┬───────┴───────┬────────┴───────────────────┘
       │             │               │
       ▼             ▼               ▼
┌─────────────┐ ┌──────────┐ ┌──────────────────────────────────┐
│ Extension   │ │  Script  │ │  os, gh, crypto, http, yaml, ui  │
│   Path      │ │  Loader  │ │  (Go implementations exposed     │
│             │ │          │ │   as Starlark built-in modules)   │
└─────────────┘ └──────────┘ └──────────────────────────────────┘
```

## Extension Discovery

### Search Path (in priority order)

1. `$NF_OPS_PATH` — Colon-separated list of directories (user override)
2. `./ops/` — Repo-local extensions (for project-specific ops)
3. `~/.config/nf-ops/extensions/` — User extensions
4. `<binary-dir>/extensions/` — Bundled extensions (shipped with the binary)

### File Naming Convention

Extensions follow PowerShell Verb-Noun naming, lowercased and hyphenated
for the filename:

```text
Starlark file:              nf-ops-update-site-deploy-token.star
Invocation:                 nf-ops update-site-deploy-token
```

### Subcommand Routing

Cobra handles top-level dispatch. When no built-in command matches, the
driver searches the extension path:

```go
rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
    return runExtension(args[0], args[1:])
}
```

Nested commands use hyphens: `nf-ops sign-pmm` or `nf-ops registry-verify`.

### Listing Available Extensions

```bash
nf-ops list              # Show all discovered extensions
nf-ops list --path       # Show extension search path
nf-ops which <name>      # Show which file would be executed
```

## Starlark Runtime

### Script Entry Point

Every extension defines a `main(ctx)` function:

```python
def main(ctx):
    """Update the SITE_DEPLOY_TOKEN repository secret."""

    ctx.require("gh", "curl")

    token = ctx.ui.secret("Paste token")
    if not token:
        ctx.error("No token provided")

    code = ctx.http.status("https://api.github.com/repos/" + SITE_REPO,
                           headers={"Authorization": "Bearer " + token})
    if code != 200:
        ctx.error("Token cannot access repo (HTTP %d)" % code)

    ctx.ui.success("Token validated")
    ctx.gh.secret_set("SITE_DEPLOY_TOKEN", token, repo=CLI_REPO)
    ctx.ui.success("Secret set on " + CLI_REPO)
```

### Script Metadata

Extensions declare metadata via a top-level `metadata` dict:

```python
metadata = {
    "name": "Update-SiteDeployToken",
    "description": "Create or rotate the SITE_DEPLOY_TOKEN repository secret",
    "args": "[--no-browser]",
    "flags": {
        "no-browser": {"type": "bool", "description": "Skip browser prompt"},
    },
}
```

The driver uses this for `nf-ops list`, `nf-ops help <name>`, and flag parsing
before calling `main(ctx)`.

## Built-in Modules

### `ctx.os` — System Operations

| Function | Description |
| --- | --- |
| `ctx.os.run(cmd, *args)` | Run command, return `Result(code, stdout, stderr)` |
| `ctx.os.run_ok(cmd, *args)` | Run command, error if non-zero exit |
| `ctx.os.env(name, default="")` | Get environment variable |
| `ctx.os.which(name)` | Find executable on PATH, return path or None |
| `ctx.os.platform()` | Return `"darwin"`, `"linux"`, or `"windows"` |
| `ctx.os.home()` | Home directory path |
| `ctx.os.read_file(path)` | Read file contents as string |
| `ctx.os.write_file(path, content)` | Write string to file |
| `ctx.os.file_exists(path)` | Check if file exists |
| `ctx.os.mkdir(path)` | Create directory (parents included) |

### `ctx.gh` — GitHub Operations

| Function | Description |
| --- | --- |
| `ctx.gh.secret_set(name, value, repo=)` | Set a repository secret |
| `ctx.gh.secret_list(repo=)` | List secret names |
| `ctx.gh.pr_create(title, body, base=, repo=)` | Create a pull request |
| `ctx.gh.pr_merge(number, method=, repo=)` | Merge a pull request |
| `ctx.gh.pr_list(repo=, state=)` | List pull requests |
| `ctx.gh.api(method, path, body=)` | Raw GitHub API call |
| `ctx.gh.repo_clone(repo, path)` | Clone a repository |
| `ctx.gh.auth_status()` | Check gh authentication |

### `ctx.http` — HTTP Client

| Function | Description |
| --- | --- |
| `ctx.http.get(url, headers=)` | GET request, return `Response` |
| `ctx.http.post(url, body=, headers=)` | POST request |
| `ctx.http.status(url, headers=)` | GET and return status code only |
| `ctx.http.download(url, path)` | Download file to path |

### `ctx.crypto` — Cryptographic Operations

| Function | Description |
| --- | --- |
| `ctx.crypto.ssh_sign(path, key=)` | Sign file with SSH key |
| `ctx.crypto.ssh_verify(path, sig, key=)` | Verify SSH signature |
| `ctx.crypto.age_encrypt(data, recipients)` | Encrypt with age |
| `ctx.crypto.age_decrypt(data, identity=)` | Decrypt with age |
| `ctx.crypto.sha256(data)` | SHA-256 hash |
| `ctx.crypto.sha256_file(path)` | SHA-256 hash of file |

### `ctx.yaml` — YAML Operations

| Function | Description |
| --- | --- |
| `ctx.yaml.load(text)` | Parse YAML string to dict/list |
| `ctx.yaml.dump(obj)` | Serialize dict/list to YAML string |
| `ctx.yaml.load_file(path)` | Parse YAML file |
| `ctx.yaml.dump_file(path, obj)` | Write YAML file |

### `ctx.ui` — User Interface

| Function | Description |
| --- | --- |
| `ctx.ui.note(msg)` | Print informational message |
| `ctx.ui.success(msg)` | Print success message (✔) |
| `ctx.ui.warn(msg)` | Print warning message |
| `ctx.ui.error(msg)` | Print error and exit |
| `ctx.ui.prompt(msg, default=)` | Prompt for input |
| `ctx.ui.secret(msg)` | Prompt for hidden input |
| `ctx.ui.confirm(msg, default=)` | Yes/no confirmation |
| `ctx.ui.open_url(url)` | Open URL in browser |

### `ctx.git` — Git Operations

| Function | Description |
| --- | --- |
| `ctx.git.run(*args)` | Run git command |
| `ctx.git.branch()` | Current branch name |
| `ctx.git.status()` | Working tree status |
| `ctx.git.describe()` | Current version tag |
| `ctx.git.rev_parse(ref)` | Resolve ref to SHA |

### `ctx` — Context Properties

| Property | Description |
| --- | --- |
| `ctx.args` | Positional arguments after flags |
| `ctx.flags` | Parsed flag values (from metadata) |
| `ctx.version` | nf-ops version string |
| `ctx.script_path` | Path to the running .star file |
| `ctx.verbose` | Whether --verbose was passed |
| `ctx.dry_run` | Whether --dry-run was passed |

### `ctx` — Context Methods

| Method | Description |
| --- | --- |
| `ctx.require(*names)` | Assert commands exist on PATH |
| `ctx.error(msg)` | Print error and exit (alias for ctx.ui.error) |

## Go Package Structure

```text
noblefactor-ops/
├── cmd/
│   └── nf-ops/
│       └── main.go              # Cobra root, discovery hook, version
├── internal/
│   ├── discovery/
│   │   └── discovery.go         # Extension path resolution and search
│   ├── runtime/
│   │   ├── runtime.go           # Starlark thread setup, module loading
│   │   ├── context.go           # ctx object implementation
│   │   └── metadata.go          # Script metadata extraction
│   └── modules/
│       ├── os.go                # ctx.os implementation
│       ├── gh.go                # ctx.gh implementation
│       ├── http.go              # ctx.http implementation
│       ├── crypto.go            # ctx.crypto implementation
│       ├── yaml.go              # ctx.yaml implementation
│       ├── ui.go                # ctx.ui implementation
│       └── git.go               # ctx.git implementation
├── extensions/                  # Bundled extensions (shipped with binary)
│   ├── nf-ops-key-generate.star
│   ├── nf-ops-key-list.star
│   ├── nf-ops-key-rotate.star
│   ├── nf-ops-sign-pmm.star
│   ├── nf-ops-sign-index.star
│   ├── nf-ops-sign-binary.star
│   ├── nf-ops-registry-reindex.star
│   ├── nf-ops-registry-verify.star
│   ├── nf-ops-registry-audit.star
│   └── nf-ops-update-site-deploy-token.star
├── Makefile
├── go.mod
└── go.sum
```

## Migration Plan

### Phase 1: Runtime Foundation

1. Add `go.starlark.net` dependency
2. Implement `internal/discovery/` — extension path search
3. Implement `internal/runtime/` — Starlark thread, ctx object
4. Implement `internal/modules/os.go` and `internal/modules/ui.go` (minimum viable)
5. Hook Cobra's unknown-command handler to the discovery engine
6. Add `list` and `which` built-in commands

### Phase 2: Core Modules

7. Implement `internal/modules/gh.go`
8. Implement `internal/modules/http.go`
9. Implement `internal/modules/yaml.go`
10. Implement `internal/modules/git.go`
11. Implement `internal/modules/crypto.go`

### Phase 3: Migrate Existing Commands

12. Write `nf-ops-key-generate.star` (replacing Go stub)
13. Write `nf-ops-key-list.star`
14. Write `nf-ops-key-rotate.star`
15. Write `nf-ops-sign-pmm.star`
16. Write `nf-ops-sign-index.star`
17. Write `nf-ops-sign-binary.star`
18. Write `nf-ops-registry-reindex.star`
19. Write `nf-ops-registry-verify.star`
20. Write `nf-ops-registry-audit.star`
21. Remove stub commands from main.go

### Phase 4: Cross-Repo Operations

22. Write `nf-ops-update-site-deploy-token.star` (replaces shell script in devlore-cli)
23. Write additional ops scripts as needed
24. Remove `scripts/Update-SiteDeployToken` from devlore-cli (replaced by nf-ops extension)

## Global Flags

The driver handles these before passing to extensions:

| Flag | Description |
| --- | --- |
| `--verbose, -v` | Enable verbose output (passed as `ctx.verbose`) |
| `--dry-run` | Dry-run mode (passed as `ctx.dry_run`) |
| `--no-color` | Disable colored output |
| `--extension-path` | Override extension search path |

## Security Considerations

- **Audit logging**: Every extension invocation is logged with timestamp, script path, and user
- **Signature verification**: Bundled extensions can be signed; user extensions are trusted
- **No network by default**: Extensions must explicitly use `ctx.http` or `ctx.gh` — no ambient network access from Starlark
- **No arbitrary exec**: `ctx.os.run()` logs commands; `--dry-run` prevents execution
- **Ceremony mode**: Extensions can declare `metadata.ceremony = True` to require interactive confirmation before destructive operations

## Example: Update-SiteDeployToken as Starlark

```python
metadata = {
    "name": "Update-SiteDeployToken",
    "description": "Create or rotate the SITE_DEPLOY_TOKEN repository secret",
    "args": "",
    "flags": {
        "no-browser": {"type": "bool", "description": "Skip browser prompt"},
    },
}

CLI_REPO = "NobleFactor/devlore-cli"
SITE_REPO = "NobleFactor/devlore.noblefactor.com"
SECRET_NAME = "SITE_DEPLOY_TOKEN"
PAT_URL = "https://github.com/settings/personal-access-tokens/new"

def main(ctx):
    ctx.require("gh", "curl")

    # Check gh authentication
    if not ctx.gh.auth_status():
        ctx.error("Not authenticated with gh. Run: gh auth login")

    # Check existing secret
    secrets = ctx.gh.secret_list(repo=CLI_REPO)
    if SECRET_NAME in secrets:
        ctx.ui.note("Existing %s secret found — this will replace it." % SECRET_NAME)

    # Instructions
    ctx.ui.note("Create a fine-grained PAT with these settings:")
    ctx.ui.note("  Repository: %s" % SITE_REPO)
    ctx.ui.note("  Permissions: Contents (R/W), Pull requests (R/W)")
    ctx.ui.note("  Expiration: 90 days recommended")

    # Open browser
    if not ctx.flags.get("no-browser"):
        if ctx.ui.confirm("Open GitHub in browser?", default=True):
            ctx.ui.open_url(PAT_URL)

    # Read and validate token
    token = ctx.ui.secret("Paste token")
    if not token:
        ctx.error("No token provided.")

    ctx.ui.note("Validating token...")
    code = ctx.http.status("https://api.github.com/repos/" + SITE_REPO,
                           headers={"Authorization": "Bearer " + token,
                                    "Accept": "application/vnd.github+json"})
    if code == 200:
        ctx.ui.success("Token can access %s" % SITE_REPO)
    elif code == 401:
        ctx.error("Token is invalid (401 Unauthorized)")
    elif code == 403:
        ctx.error("Token lacks access to %s (403 Forbidden)" % SITE_REPO)
    elif code == 404:
        ctx.error("Token cannot see %s (404)" % SITE_REPO)
    else:
        ctx.error("Unexpected response: HTTP %d" % code)

    # Set secret
    ctx.ui.note("Setting repository secret...")
    ctx.gh.secret_set(SECRET_NAME, token, repo=CLI_REPO)
    ctx.ui.success("%s set on %s" % (SECRET_NAME, CLI_REPO))

    ctx.ui.note("The docs-publish workflow will use this token on next push.")
    ctx.ui.note("Run this extension again before the token expires.")
```

## Dependencies to Add

```text
go.starlark.net v0.0.0-...    # Starlark interpreter (already used in devlore-cli)
```

## Binding Implementation Strategy

### Context Object Structure

The `ctx` object passed to `main(ctx)` implements `starlark.HasAttrs`. Each
sub-module (`ctx.os`, `ctx.gh`, etc.) is also a `starlark.HasAttrs` value.
This gives precise control over attribute resolution and clear error messages
for typos.

```go
// context.go
type Context struct {
    os      *OSModule
    gh      *GHModule
    http    *HTTPModule
    crypto  *CryptoModule
    yaml    *YAMLModule
    ui      *UIModule
    git     *GitModule
    args    *starlark.List
    flags   *starlark.Dict
    verbose starlark.Bool
    dryRun  starlark.Bool
    // ...
}

func (c *Context) String() string        { return "<ctx>" }
func (c *Context) Type() string          { return "context" }
func (c *Context) Freeze()               { /* freeze sub-modules */ }
func (c *Context) Truth() starlark.Bool   { return starlark.True }
func (c *Context) Hash() (uint32, error)  { return 0, fmt.Errorf("unhashable: context") }

func (c *Context) Attr(name string) (starlark.Value, error) {
    switch name {
    case "os":
        return c.os, nil
    case "gh":
        return c.gh, nil
    case "args":
        return c.args, nil
    case "flags":
        return c.flags, nil
    case "verbose":
        return c.verbose, nil
    case "dry_run":
        return c.dryRun, nil
    case "require":
        return c.builtinRequire(), nil
    case "error":
        return c.builtinError(), nil
    // ...
    }
    return nil, starlark.NoSuchAttrError(fmt.Sprintf("context has no .%s attribute", name))
}

func (c *Context) AttrNames() []string {
    return []string{"os", "gh", "http", "crypto", "yaml", "ui", "git",
        "args", "flags", "verbose", "dry_run", "version", "script_path",
        "require", "error"}
}
```

### Sub-Module Pattern

Each module is a Go struct implementing `starlark.HasAttrs`. Methods are
returned as `starlark.Builtin` values from `Attr()`:

```go
// modules/os.go
type OSModule struct {
    dryRun bool
    logger *audit.Logger
}

func (m *OSModule) String() string        { return "<ctx.os>" }
func (m *OSModule) Type() string          { return "os_module" }
func (m *OSModule) Freeze()               {}
func (m *OSModule) Truth() starlark.Bool   { return starlark.True }
func (m *OSModule) Hash() (uint32, error)  { return 0, fmt.Errorf("unhashable: os_module") }

func (m *OSModule) Attr(name string) (starlark.Value, error) {
    switch name {
    case "run":
        return starlark.NewBuiltin("ctx.os.run", m.run), nil
    case "run_ok":
        return starlark.NewBuiltin("ctx.os.run_ok", m.runOK), nil
    case "env":
        return starlark.NewBuiltin("ctx.os.env", m.env), nil
    case "which":
        return starlark.NewBuiltin("ctx.os.which", m.which), nil
    // ...
    }
    return nil, starlark.NoSuchAttrError(fmt.Sprintf("ctx.os has no .%s attribute", name))
}

func (m *OSModule) AttrNames() []string {
    return []string{"run", "run_ok", "env", "which", "platform", "home",
        "read_file", "write_file", "file_exists", "mkdir"}
}
```

### Builtin Function Signature

Every binding follows the same `starlark.Builtin` signature. Use
`starlark.UnpackArgs` or `starlark.UnpackPositionalArgs` for argument parsing:

```go
func (m *OSModule) run(thread *starlark.Thread, fn *starlark.Builtin,
    args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {

    var cmd string
    var cmdArgs *starlark.List
    if err := starlark.UnpackPositionalArgs("ctx.os.run", args, kwargs, 1, &cmd, &cmdArgs); err != nil {
        return nil, err
    }

    // Convert starlark.List to []string
    goArgs := []string{}
    if cmdArgs != nil {
        for i := 0; i < cmdArgs.Len(); i++ {
            s, ok := starlark.AsString(cmdArgs.Index(i))
            if !ok {
                return nil, fmt.Errorf("ctx.os.run: argument %d is not a string", i+1)
            }
            goArgs = append(goArgs, s)
        }
    }

    // Execute (respecting dry-run)
    if m.dryRun {
        m.logger.Log("dry-run: %s %v", cmd, goArgs)
        return newResult(0, "", ""), nil
    }

    result := exec.Command(cmd, goArgs...).CombinedOutput()
    // ...
    return newResult(code, stdout, stderr), nil
}
```

### Keyword Argument Handling

For functions with keyword arguments (e.g., `ctx.gh.secret_set(name, value, repo=)`),
use `starlark.UnpackArgs` which handles both positional and keyword args:

```go
func (m *GHModule) secretSet(thread *starlark.Thread, fn *starlark.Builtin,
    args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {

    var name, value, repo string
    if err := starlark.UnpackArgs("ctx.gh.secret_set", args, kwargs,
        "name", &name, "value", &value, "repo?", &repo); err != nil {
        return nil, err
    }

    // "repo?" means optional — if omitted, repo == ""
    if repo == "" {
        return nil, fmt.Errorf("ctx.gh.secret_set: repo is required")
    }

    // ...
}
```

The `?` suffix in the parameter name makes it optional. Required parameters
without `?` will cause `UnpackArgs` to return an error if missing.

### Type Marshalling

| Starlark Type | Go Type | Conversion |
| --- | --- | --- |
| `String` | `string` | `starlark.AsString(v)` or `string(v.(starlark.String))` |
| `Int` | `int` | `v.(starlark.Int).Int64()` |
| `Bool` | `bool` | `bool(v.(starlark.Bool))` |
| `List` | `[]starlark.Value` | iterate with `.Index(i)` and `.Len()` |
| `Dict` | `map[string]string` | iterate with `.Items()` |
| `None` | `nil` | `v == starlark.None` |

Return values use the inverse:

```go
// Go → Starlark
starlark.String("hello")
starlark.MakeInt(42)
starlark.Bool(true)
starlark.None
```

For structured return values (e.g., `ctx.os.run()` returning `Result`), use a
custom `starlark.HasAttrs`:

```go
type ExecResult struct {
    code   int
    stdout string
    stderr string
}

func (r *ExecResult) Attr(name string) (starlark.Value, error) {
    switch name {
    case "code":
        return starlark.MakeInt(r.code), nil
    case "stdout":
        return starlark.String(r.stdout), nil
    case "stderr":
        return starlark.String(r.stderr), nil
    }
    return nil, nil
}
```

### Error Propagation

Two distinct error paths:

1. **Go errors → Starlark exceptions**: Return `(nil, error)` from a builtin.
   The Starlark runtime surfaces this as a stack trace with source location.
   Use for programming errors (wrong argument types, missing required args).

2. **`ctx.error(msg)` → process exit**: The `error` builtin calls
   `os.Exit(1)` after printing the message. This is for operational errors
   (token invalid, command not found). Implemented by panicking with a
   sentinel type that the top-level runner catches:

```go
type exitError struct {
    message string
    code    int
}

func (c *Context) builtinError() *starlark.Builtin {
    return starlark.NewBuiltin("ctx.error", func(
        thread *starlark.Thread, fn *starlark.Builtin,
        args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {

        var msg string
        starlark.UnpackPositionalArgs("ctx.error", args, kwargs, 1, &msg)
        panic(exitError{message: msg, code: 1})
    })
}
```

The top-level runner in `cmd/nf-ops/main.go` uses `recover()` to catch this
and exit cleanly.

### Thread Setup and Script Loading

The runtime creates a `starlark.Thread` per extension invocation, loads the
script, extracts metadata, parses flags, then calls `main(ctx)`:

```go
func RunExtension(path string, rawArgs []string) error {
    // 1. Load and parse the script (first pass — extracts globals)
    thread := &starlark.Thread{Name: filepath.Base(path)}
    globals, err := starlark.ExecFile(thread, path, nil, builtins())

    // 2. Extract metadata dict
    meta := extractMetadata(globals)

    // 3. Parse flags from rawArgs using metadata.flags
    flags, positional := parseFlags(meta, rawArgs)

    // 4. Build context
    ctx := NewContext(positional, flags, path)

    // 5. Call main(ctx)
    mainFn := globals["main"]
    _, err = starlark.Call(thread, mainFn, starlark.Tuple{ctx}, nil)
    return err
}
```

### Adding a New Module — Checklist

To add a new built-in module (e.g., `ctx.docker`):

1. Create `internal/modules/docker.go` — struct with `HasAttrs`, methods as builtins
2. Add field to `Context` struct in `internal/runtime/context.go`
3. Add case to `Context.Attr()` and `Context.AttrNames()`
4. Add constructor call in `NewContext()`
5. Write unit tests in `internal/modules/docker_test.go`
6. Document the API in this plan file

### Adding a Method to an Existing Module — Checklist

To add a new method (e.g., `ctx.os.temp_dir()`):

1. Add the method function to the module struct in `internal/modules/os.go`
2. Add case to the module's `Attr()` switch
3. Add to the module's `AttrNames()` return slice
4. Write unit test
5. Document in the API table above

## Open Questions

1. **Embedding vs. filesystem**: Should bundled extensions be embedded in the binary
   (`embed.FS`) or shipped as files alongside it?
   — Recommend: embed for portability, with filesystem override for development.

2. **Extension versioning**: Should extensions declare a minimum nf-ops version
   for compatibility?
   — Recommend: yes, via `metadata.min_version`.

3. **Testing**: How to test Starlark extensions?
   — Recommend: `nf-ops test <script.star>` that runs the script with a mock ctx,
   plus Go unit tests for each module.

4. **Shared libraries**: Can extensions import other .star files?
   — Recommend: yes, via `load("nf-ops-lib-<name>.star", ...)` syntax, searched
   on the same extension path.

## Addendum: Why `HasAttrs` Over `starlarkstruct`

The `go.starlark.net` library offers two approaches for exposing Go functions
to Starlark scripts. This plan uses `starlark.HasAttrs`; here is the rationale.

### `starlarkstruct` (the simpler alternative)

Uses `starlarkstruct.FromStringDict` to build modules from a flat dictionary:

```go
osModule := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
    "run":     starlark.NewBuiltin("ctx.os.run", osRun),
    "env":     starlark.NewBuiltin("ctx.os.env", osEnv),
    "which":   starlark.NewBuiltin("ctx.os.which", osWhich),
})
```

- No custom types — assemble a dict and hand it off
- Methods are standalone functions; shared state (dry-run, logger) must be
  captured via closures
- Error messages on typos are generic: `struct has no .runn attribute`
- No control over `String()`, `Type()`, or freeze behavior

### `starlark.HasAttrs` (chosen approach)

Each module is a Go struct implementing the interface directly:

```go
type OSModule struct {
    dryRun bool
    logger *audit.Logger
}

func (m *OSModule) Attr(name string) (starlark.Value, error) { ... }
func (m *OSModule) AttrNames() []string { ... }
```

- Methods are real method calls — natural access to struct fields
- Custom error messages: `ctx.os has no .runn attribute`
- Control over `Type()` (shows `"os_module"` not `"struct"`) and `String()`
  (shows `"<ctx.os>"` not `struct(run = ..., env = ...)`)
- Explicit freeze semantics

### Comparison

| Concern | `starlarkstruct` | `HasAttrs` |
| --- | --- | --- |
| Lines per module | ~5 (just the dict) | ~20 (interface methods) |
| State access | Closures | Struct fields |
| Error messages | Generic | Custom, contextual |
| Type display | `struct` | `os_module`, `gh_module` |
| Repr in REPL | `struct(run = ..., ...)` | `<ctx.os>` |
| Testability | Mock the dict | Mock the struct |
| Freeze semantics | Freezes all values recursively | You decide |

### Decision

`HasAttrs` is chosen because nf-ops modules carry mutable state (dry-run mode,
audit logger, auth tokens) and because clear error messages matter for
operator-facing tooling. The boilerplate cost is ~15 lines per module, paid
once across ~7 modules, and is entirely mechanical.

## Addendum: Starlark as a Management Tool

### Consistency with Google's Starlark Usage

The binding strategy (`HasAttrs` in Go, `@StarlarkBuiltin`/`@StarlarkMethod`
in Java) is directly consistent with how Google implements Starlark bindings
across all their tools:

| | Bazel (Java) | nf-ops (Go) |
| --- | --- | --- |
| Type declaration | `@StarlarkBuiltin(name = "ctx")` | `func (c *Context) Type() string` |
| Method exposure | `@StarlarkMethod(name = "run")` | `case "run": return NewBuiltin(...)` |
| Attribute dispatch | Annotation-driven reflection | `Attr(name)` switch |
| Interface | `implements StarlarkValue` | `implements starlark.HasAttrs` |

Bazel's `repository_ctx`, `ctx`, `actions`, and `native` modules all follow
this same pattern. The Go `HasAttrs` interface is the direct equivalent of
Java's `@StarlarkBuiltin` annotation.

### Intentional Departure: Imperative Execution

Where nf-ops diverges from Bazel is the **execution model**, not the binding
model. Starlark was designed for hermetic, side-effect-free evaluation. Bazel
enforces this: rule implementations declare actions that form a build graph,
but nothing executes during Starlark evaluation itself.

nf-ops uses Starlark as an **operations management tool**. The entire purpose
is side effects — creating secrets, signing artifacts, calling APIs, rotating
keys. This is an intentional and appropriate departure:

| | Bazel (build system) | nf-ops (management tool) |
| --- | --- | --- |
| Purpose | Declare build graph | Execute operations |
| Side effects | Prohibited during eval | The whole point |
| Hermeticity | Core requirement | Not a goal |
| `ctx.os.run()` | Not available | Executes immediately |
| Network access | None during eval | Explicit via `ctx.http`, `ctx.gh` |
| Idempotence | Via action caching | Where idempotence is a value |

### Why Starlark Still Fits

Starlark's value for management tooling is not hermeticity — it is:

1. **Sandboxed by default** — Scripts cannot import arbitrary Python packages
   or access the filesystem outside the provided `ctx` modules. The runtime
   controls exactly what capabilities are available.

2. **Deterministic language semantics** — No threads, no global mutable state
   between scripts, no monkey-patching. Scripts are predictable even when
   their effects are not hermetic.

3. **Familiar syntax** — Python-like syntax means operators can read and write
   extensions without learning a new language.

4. **Embeddable** — A single Go binary contains the interpreter. No external
   runtime, no dependency management, no version conflicts.

5. **Auditable** — Every side-effecting operation goes through a `ctx` method
   that the runtime can log, gate behind `--dry-run`, or require confirmation
   for (`metadata.ceremony = True`).

### Idempotence Strategy

Where idempotence is a value, extensions should implement it explicitly:

```python
def main(ctx):
    # Idempotent: only sets if different
    secrets = ctx.gh.secret_list(repo=REPO)
    if SECRET_NAME not in secrets:
        ctx.gh.secret_set(SECRET_NAME, token, repo=REPO)
        ctx.ui.success("Secret created")
    else:
        ctx.ui.note("Secret already exists — replacing")
        ctx.gh.secret_set(SECRET_NAME, token, repo=REPO)
        ctx.ui.success("Secret rotated")
```

The runtime does not enforce idempotence — that is the extension author's
responsibility. The `--dry-run` flag and audit logging provide visibility
into what would happen vs. what did happen.

### Precedent

Google itself uses Starlark imperatively in several tools beyond Bazel:

- **Copybara** — Code transformation tool; Starlark scripts describe
  transformations that execute with side effects (file modifications,
  git operations)
- **Skycfg** — Starlark-based configuration tool for generating protobuf
  messages, used at Stripe and other companies
- **Tilt** — Development environment tool using Starlark (Tiltfiles) for
  imperative container orchestration
- **Isopod** — Kubernetes deployment tool using Starlark for imperative
  cluster configuration

### Sources

- [google/starlark-go implementation guide](https://github.com/google/starlark-go/blob/master/doc/impl.md)
- [starlarkstruct package](https://github.com/google/starlark-go/blob/master/starlarkstruct/struct.go)
- [Bazel StarlarkRepositoryContext](https://github.com/bazelbuild/bazel/blob/master/src/main/java/com/google/devtools/build/lib/bazel/repository/starlark/StarlarkRepositoryContext.java)
- [Bazel StarlarkOS](https://github.com/bazelbuild/bazel/blob/master/src/main/java/com/google/devtools/build/lib/bazel/repository/starlark/StarlarkOS.java)
- [Bazel Starlark language specification](https://bazel.build/rules/language)
- [google/copybara](https://github.com/google/copybara)
- [Starlark language spec](https://github.com/bazelbuild/starlark/blob/master/spec.md)
