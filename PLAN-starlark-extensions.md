# Plan: Starlark Extension Model for nf-ops

## Vision

Transform `nf-ops` into a git-style extensible operations tool where all
commands are implemented as Starlark scripts discovered at runtime. A single
Go driver program provides the runtime, discovery, and built-in modules.

```
nf-ops update-site-deploy-token
       └─ finds nf-ops-update-site-deploy-token.star on the extension path
       └─ loads it into the Starlark runtime
       └─ calls main() with context
```

Like git's extension model: if `git-foo` is on PATH, `git foo` runs it.
Here, if `nf-ops-foo.star` is on the extension path, `nf-ops foo` runs it.

## Architecture

```
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

```
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
|----------|-------------|
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
|----------|-------------|
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
|----------|-------------|
| `ctx.http.get(url, headers=)` | GET request, return `Response` |
| `ctx.http.post(url, body=, headers=)` | POST request |
| `ctx.http.status(url, headers=)` | GET and return status code only |
| `ctx.http.download(url, path)` | Download file to path |

### `ctx.crypto` — Cryptographic Operations

| Function | Description |
|----------|-------------|
| `ctx.crypto.ssh_sign(path, key=)` | Sign file with SSH key |
| `ctx.crypto.ssh_verify(path, sig, key=)` | Verify SSH signature |
| `ctx.crypto.age_encrypt(data, recipients)` | Encrypt with age |
| `ctx.crypto.age_decrypt(data, identity=)` | Decrypt with age |
| `ctx.crypto.sha256(data)` | SHA-256 hash |
| `ctx.crypto.sha256_file(path)` | SHA-256 hash of file |

### `ctx.yaml` — YAML Operations

| Function | Description |
|----------|-------------|
| `ctx.yaml.load(text)` | Parse YAML string to dict/list |
| `ctx.yaml.dump(obj)` | Serialize dict/list to YAML string |
| `ctx.yaml.load_file(path)` | Parse YAML file |
| `ctx.yaml.dump_file(path, obj)` | Write YAML file |

### `ctx.ui` — User Interface

| Function | Description |
|----------|-------------|
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
|----------|-------------|
| `ctx.git.run(*args)` | Run git command |
| `ctx.git.branch()` | Current branch name |
| `ctx.git.status()` | Working tree status |
| `ctx.git.describe()` | Current version tag |
| `ctx.git.rev_parse(ref)` | Resolve ref to SHA |

### `ctx` — Context Properties

| Property | Description |
|----------|-------------|
| `ctx.args` | Positional arguments after flags |
| `ctx.flags` | Parsed flag values (from metadata) |
| `ctx.version` | nf-ops version string |
| `ctx.script_path` | Path to the running .star file |
| `ctx.verbose` | Whether --verbose was passed |
| `ctx.dry_run` | Whether --dry-run was passed |

### `ctx` — Context Methods

| Method | Description |
|--------|-------------|
| `ctx.require(*names)` | Assert commands exist on PATH |
| `ctx.error(msg)` | Print error and exit (alias for ctx.ui.error) |

## Go Package Structure

```
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
|------|-------------|
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

```
go.starlark.net v0.0.0-...    # Starlark interpreter (already used in devlore-cli)
```

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
